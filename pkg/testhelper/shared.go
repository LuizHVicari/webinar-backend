package testhelper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LuizHVicari/webinar-backend/pkg/keto"
)

// PostgresContainer holds a shared Postgres container and connection pool.
type PostgresContainer struct {
	Pool    *pgxpool.Pool
	cleanup func()
}

// Terminate stops the container and closes the pool.
func (p *PostgresContainer) Terminate() { p.cleanup() }

// StartPostgres starts a Postgres container, runs migrations, and returns a PostgresContainer.
// The caller is responsible for calling Terminate() when done (typically deferred in TestMain).
func StartPostgres() (*PostgresContainer, error) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("get connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("create pool: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	if err := runMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		pool.Close()
		_ = ctr.Terminate(ctx)
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	_ = sqlDB.Close()

	return &PostgresContainer{
		Pool: pool,
		cleanup: func() {
			pool.Close()
			_ = ctr.Terminate(ctx)
		},
	}, nil
}

// TruncateTables deletes all rows from domain tables in dependency order.
// Call at the start of each test that uses a shared Postgres pool.
func TruncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, "TRUNCATE invites, users, organizations RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// KetoContainer holds a shared Keto container and client.
type KetoContainer struct {
	Client  *keto.Client
	cleanup func()
}

// Terminate stops the containers.
func (k *KetoContainer) Terminate() { k.cleanup() }

// StartKeto starts a Postgres + Keto container pair and returns a KetoContainer.
// The caller is responsible for calling Terminate() when done (typically deferred in TestMain).
func StartKeto() (*KetoContainer, error) {
	ctx := context.Background()

	_, file, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(file), "..", "..")
	namespacesPath := filepath.Join(projectRoot, "config", "keto", "namespaces", "namespaces.ts")

	net, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create docker network for keto: %w", err)
	}

	const (
		pgAlias    = "keto-postgres"
		pgUser     = "keto"
		pgPassword = "keto"
		pgDB       = "ketodb"
	)

	pgCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:16-alpine",
			Env: map[string]string{
				"POSTGRES_USER":     pgUser,
				"POSTGRES_PASSWORD": pgPassword,
				"POSTGRES_DB":       pgDB,
			},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {pgAlias}},
			WaitingFor:     wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("start postgres for keto: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", pgUser, pgPassword, pgAlias, pgDB)
	ketoConfig := fmt.Sprintf(ketoTestConfigTemplate, dsn)

	tmpDir, err := os.MkdirTemp("", "keto-shared-*")
	if err != nil {
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	ketoConfigPath := filepath.Join(tmpDir, "keto.yml")
	if err := os.WriteFile(ketoConfigPath, []byte(ketoConfig), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("write keto config: %w", err)
	}

	ketoCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "oryd/keto:v0.14.0",
			ExposedPorts:   []string{"4466/tcp", "4467/tcp"},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"keto"}},
			Entrypoint:     []string{"/bin/sh", "-c"},
			Cmd: []string{
				"echo y | keto migrate up -c /tmp/keto.yml && keto serve --config /tmp/keto.yml",
			},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: ketoConfigPath, ContainerFilePath: "/tmp/keto.yml", FileMode: 0o644},
				{HostFilePath: namespacesPath, ContainerFilePath: "/tmp/namespaces.ts", FileMode: 0o644},
			},
			WaitingFor: wait.ForHTTP("/health/ready").
				WithPort("4466/tcp").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("start keto container: %w", err)
	}

	readURL, err := ketoCtr.PortEndpoint(ctx, "4466/tcp", "http")
	if err != nil {
		_ = ketoCtr.Terminate(ctx)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("get keto read endpoint: %w", err)
	}
	writeURL, err := ketoCtr.PortEndpoint(ctx, "4467/tcp", "http")
	if err != nil {
		_ = ketoCtr.Terminate(ctx)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("get keto write endpoint: %w", err)
	}

	os.RemoveAll(tmpDir)

	return &KetoContainer{
		Client: keto.New(readURL, writeURL),
		cleanup: func() {
			_ = ketoCtr.Terminate(ctx)
			_ = pgCtr.Terminate(ctx)
			_ = net.Remove(ctx)
		},
	}, nil
}

// DeleteAllRelations removes all relation tuples from the shared Keto instance.
// Call at the start of each test that writes Keto relations.
func DeleteAllRelations(t *testing.T, client *keto.Client) {
	t.Helper()
	ctx := context.Background()
	if err := client.DeleteAllRelations(ctx); err != nil {
		t.Fatalf("delete all keto relations: %v", err)
	}
}

// KratosContainer holds a shared Kratos container and its URLs.
type KratosContainer struct {
	URLs    KratosURLs
	cleanup func()
}

// Terminate stops the containers.
func (k *KratosContainer) Terminate() { k.cleanup() }

// StartKratos starts a Postgres + Kratos container pair and returns a KratosContainer.
// The caller is responsible for calling Terminate() when done (typically deferred in TestMain).
func StartKratos() (*KratosContainer, error) {
	ctx := context.Background()

	net, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create docker network for kratos: %w", err)
	}

	const (
		pgAlias    = "kratos-postgres"
		pgUser     = "kratos"
		pgPassword = "kratos"
		pgDB       = "kratosdb"
	)

	pgCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:16-alpine",
			Env: map[string]string{
				"POSTGRES_USER":     pgUser,
				"POSTGRES_PASSWORD": pgPassword,
				"POSTGRES_DB":       pgDB,
			},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {pgAlias}},
			WaitingFor:     wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("start postgres for kratos: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", pgUser, pgPassword, pgAlias, pgDB)
	kratosConfig := fmt.Sprintf(kratosTestConfigTemplate, dsn)

	tmpDir, err := os.MkdirTemp("", "kratos-shared-*")
	if err != nil {
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	kratosConfigPath := filepath.Join(tmpDir, "kratos.yml")
	if err := os.WriteFile(kratosConfigPath, []byte(kratosConfig), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("write kratos config: %w", err)
	}

	kratosCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "oryd/kratos:v0.13.0",
			ExposedPorts:   []string{"4433/tcp", "4434/tcp"},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"kratos"}},
			Entrypoint:     []string{"/bin/sh", "-c"},
			Cmd: []string{
				"kratos migrate sql -e --yes -c /tmp/kratos.yml && kratos serve --config /tmp/kratos.yml --dev",
			},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: kratosConfigPath, ContainerFilePath: "/tmp/kratos.yml", FileMode: 0o644},
			},
			WaitingFor: wait.ForHTTP("/health/ready").
				WithPort("4433/tcp").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("start kratos container: %w", err)
	}

	publicURL, err := kratosCtr.PortEndpoint(ctx, "4433/tcp", "http")
	if err != nil {
		_ = kratosCtr.Terminate(ctx)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("get kratos public endpoint: %w", err)
	}
	adminURL, err := kratosCtr.PortEndpoint(ctx, "4434/tcp", "http")
	if err != nil {
		_ = kratosCtr.Terminate(ctx)
		_ = pgCtr.Terminate(ctx)
		_ = net.Remove(ctx)
		return nil, fmt.Errorf("get kratos admin endpoint: %w", err)
	}

	os.RemoveAll(tmpDir)

	return &KratosContainer{
		URLs: KratosURLs{PublicURL: publicURL, AdminURL: adminURL},
		cleanup: func() {
			_ = kratosCtr.Terminate(ctx)
			_ = pgCtr.Terminate(ctx)
			_ = net.Remove(ctx)
		},
	}, nil
}

// DeleteAllIdentities removes all identities from the shared Kratos instance via the admin API.
// Call at the start of each test that creates Kratos identities.
func DeleteAllIdentities(t *testing.T, adminURL string) {
	t.Helper()

	ids, err := listKratosIdentityIDs(adminURL)
	if err != nil {
		t.Fatalf("list kratos identities: %v", err)
	}
	for _, id := range ids {
		req, _ := http.NewRequest(http.MethodDelete, adminURL+"/admin/identities/"+id, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete kratos identity %s: %v", id, err)
		}
		resp.Body.Close()
	}
}

func listKratosIdentityIDs(adminURL string) ([]string, error) {
	resp, err := http.Get(adminURL + "/admin/identities")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var identities []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		return nil, err
	}

	ids := make([]string, len(identities))
	for i, id := range identities {
		ids[i] = id.ID
	}
	return ids, nil
}
