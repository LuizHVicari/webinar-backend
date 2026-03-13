package testhelper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// KratosURLs holds the public and admin base URLs for a test Kratos container.
type KratosURLs struct {
	PublicURL string
	AdminURL  string
}

// kratosTestConfigTemplate is the Kratos config for tests.
// The DSN placeholder is replaced at runtime with the container Postgres address.
// The identity schema is the same email preset used in production (base64-encoded).
const kratosTestConfigTemplate = `
version: v0.13.0

dsn: %s

serve:
  public:
    host: 0.0.0.0
    port: 4433
    base_url: http://localhost:4433/
  admin:
    host: 0.0.0.0
    port: 4434
    base_url: http://localhost:4434/

selfservice:
  default_browser_return_url: http://localhost:3000/
  allowed_return_urls:
    - http://localhost:3000
  methods:
    password:
      enabled: true
  flows:
    registration:
      ui_url: http://localhost:3000/registration
    login:
      ui_url: http://localhost:3000/login
    error:
      ui_url: http://localhost:3000/error
    settings:
      ui_url: http://localhost:3000/settings

identity:
  default_schema_id: default
  schemas:
    - id: default
      url: base64://ewogICIkaWQiOiAiaHR0cHM6Ly9zY2hlbWFzLm9yeS5zaC9wcmVzZXRzL2t3YXJncy9pZGVudGl0eS5lbWFpbC5zY2hlbWEuanNvbiIsCiAgIiRzY2hlbWEiOiAiaHR0cDovL2pzb24tc2NoZW1hLm9yZy9kcmFmdC0wNy9zY2hlbWEjIiwKICAidGl0bGUiOiAiUGVyc29uIiwKICAidHlwZSI6ICJvYmplY3QiLAogICJwcm9wZXJ0aWVzIjogewogICAgInRyYWl0cyI6IHsKICAgICAgInR5cGUiOiAib2JqZWN0IiwKICAgICAgInByb3BlcnRpZXMiOiB7CiAgICAgICAgImVtYWlsIjogewogICAgICAgICAgInR5cGUiOiAic3RyaW5nIiwKICAgICAgICAgICJmb3JtYXQiOiAiZW1haWwiLAogICAgICAgICAgInRpdGxlIjogIkUtTWFpbCIsCiAgICAgICAgICAib3J5LnNoL2tyYXRvcyI6IHsKICAgICAgICAgICAgImNyZWRlbnRpYWxzIjogewogICAgICAgICAgICAgICJwYXNzd29yZCI6IHsKICAgICAgICAgICAgICAgICJpZGVudGlmaWVyIjogdHJ1ZQogICAgICAgICAgICAgIH0KICAgICAgICAgICAgfSwKICAgICAgICAgICAgInZlcmlmaWNhdGlvbiI6IHsKICAgICAgICAgICAgICAidmlhIjogImVtYWlsIgogICAgICAgICAgICB9LAogICAgICAgICAgICAicmVjb3ZlcnkiOiB7CiAgICAgICAgICAgICAgInZpYSI6ICJlbWFpbCIKICAgICAgICAgICAgfQogICAgICAgICAgfQogICAgICAgIH0KICAgICAgfSwKICAgICAgInJlcXVpcmVkIjogWyJlbWFpbCJdLAogICAgICAiYWRkaXRpb25hbFByb3BlcnRpZXMiOiBmYWxzZQogICAgfQogIH0KfQo=

courier:
  smtp:
    connection_uri: smtps://test:test@localhost:1025/?skip_ssl_verify=true

secrets:
  cookie:
    - changeme-32-bytes-long-secret-1!
  cipher:
    - changeme-32-bytes-long-secret-2!
`

// NewKratos starts a dedicated Postgres + Kratos container pair and returns the public/admin URLs.
// It registers t.Cleanup to terminate the containers and remove the shared network.
func NewKratos(t *testing.T) KratosURLs {
	t.Helper()

	ctx := context.Background()

	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create docker network: %v", err)
	}
	t.Cleanup(func() { _ = net.Remove(ctx) })

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
		t.Fatalf("start postgres container for kratos: %v", err)
	}
	t.Cleanup(func() { _ = pgCtr.Terminate(ctx) })

	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", pgUser, pgPassword, pgAlias, pgDB)
	kratosConfig := fmt.Sprintf(kratosTestConfigTemplate, dsn)

	tmpDir := t.TempDir()
	kratosConfigPath := filepath.Join(tmpDir, "kratos.yml")
	if err := os.WriteFile(kratosConfigPath, []byte(kratosConfig), 0o644); err != nil {
		t.Fatalf("write kratos config: %v", err)
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
		t.Fatalf("start kratos container: %v", err)
	}
	t.Cleanup(func() { _ = kratosCtr.Terminate(ctx) })

	publicURL, err := kratosCtr.PortEndpoint(ctx, "4433/tcp", "http")
	if err != nil {
		t.Fatalf("get kratos public endpoint: %v", err)
	}
	adminURL, err := kratosCtr.PortEndpoint(ctx, "4434/tcp", "http")
	if err != nil {
		t.Fatalf("get kratos admin endpoint: %v", err)
	}

	return KratosURLs{
		PublicURL: publicURL,
		AdminURL:  adminURL,
	}
}
