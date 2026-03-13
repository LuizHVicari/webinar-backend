package testhelper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/LuizHVicari/webinar-backend/pkg/keto"
)

// ketoTestConfigTemplate is the Keto config for tests.
// The DSN placeholder is replaced at runtime with the container Postgres address.
const ketoTestConfigTemplate = `
version: v0.14.0

dsn: %s

serve:
  read:
    host: 0.0.0.0
    port: 4466
  write:
    host: 0.0.0.0
    port: 4467

namespaces:
  location: file:///tmp/namespaces.ts
`

// NewKeto starts a dedicated Postgres + Keto container pair and returns a configured client.
// It registers t.Cleanup to terminate the containers and remove the shared network.
func NewKeto(t *testing.T) *keto.Client {
	t.Helper()

	ctx := context.Background()

	_, file, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(file), "..", "..")
	namespacesPath := filepath.Join(projectRoot, "config", "keto", "namespaces", "namespaces.ts")

	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create docker network: %v", err)
	}
	t.Cleanup(func() { _ = net.Remove(ctx) })

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
		t.Fatalf("start postgres container for keto: %v", err)
	}
	t.Cleanup(func() { _ = pgCtr.Terminate(ctx) })

	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", pgUser, pgPassword, pgAlias, pgDB)
	ketoConfig := fmt.Sprintf(ketoTestConfigTemplate, dsn)

	tmpDir := t.TempDir()
	ketoConfigPath := filepath.Join(tmpDir, "keto.yml")
	if err := os.WriteFile(ketoConfigPath, []byte(ketoConfig), 0o644); err != nil {
		t.Fatalf("write keto config: %v", err)
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
		t.Fatalf("start keto container: %v", err)
	}
	t.Cleanup(func() { _ = ketoCtr.Terminate(ctx) })

	readURL, err := ketoCtr.PortEndpoint(ctx, "4466/tcp", "http")
	if err != nil {
		t.Fatalf("get keto read endpoint: %v", err)
	}
	writeURL, err := ketoCtr.PortEndpoint(ctx, "4467/tcp", "http")
	if err != nil {
		t.Fatalf("get keto write endpoint: %v", err)
	}

	return keto.New(readURL, writeURL)
}
