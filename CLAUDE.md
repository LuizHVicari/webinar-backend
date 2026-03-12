# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Stack

- **Language:** Go
- **HTTP framework:** Gin
- **Database queries:** sqlc
- **Migrations:** Goose
- **Testing:** testcontainers-go
- **Auth/Permissions:** Ory Kratos (identity/authn) + Ory Keto (permissions/authz)
- **Task runner:** just
- **Infrastructure:** Docker + Docker Compose with profiles

## Commands

```sh
just dev          # run the API with hot reload (default)
just run          # run the API without hot reload
just build        # build the binary
just test         # run all tests
just test <pkg>   # run tests in a specific package, e.g.: just test ./internal/user/...
just lint         # run linter
just migrate      # apply database migrations
just generate     # run sqlc generate
```

## Project Structure

Follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout), with `internal` organized by domain/entity:

```
cmd/api/          # entrypoint — wires dependencies and starts the server
internal/
  user/
    handler.go    # HTTP handlers for this domain
    service.go    # business logic
    repository.go # data access interface + implementation
    user.go       # domain types for this context
  webinar/
    handler.go
    service.go
    repository.go
    webinar.go
  ...             # one package per domain/entity
pkg/
  config/         # env-based config loading (used by cmd/api and tests)
  ...             # other reusable packages (no domain logic)
migrations/       # Goose SQL migration files
sqlc/
  queries/        # .sql query files for sqlc
  generated/      # sqlc output — committed to repo for CI validation
  sqlc.yaml
config/
  kratos/         # Kratos config files (mounted by Docker Compose)
```

## Architecture Rules

**Layering within each package:** handler → service → repository. Each layer depends only on interfaces defined in the same package, never on concrete types from another package.

**Interfaces:** Keep interfaces small (1–3 methods). Define them at the consumer side, not the provider side.

**Handlers** only: bind/validate the request, call one service method, return the response. No business logic.

**Services** only: business rules. No SQL, no HTTP types.

**Repositories** only: data access. Return domain types, never raw DB rows.

## Code Style

- No unnecessary comments. Only add comments to: explain *why* something is done (non-obvious reasoning), generate code (`//go:generate`), or document exported symbols used as a public API.
- Never write comments that restate what the code does (e.g., `// checks if a equals b`).
- Keep functions short and focused on a single responsibility.
- Follow idiomatic Go conventions throughout.

## Testing

Every layer must be tested:
- **Service tests:** pure unit tests, no I/O.
- **Repository tests:** use testcontainers to spin up a real database.
- **Handler tests:** use `httptest.NewRequest` / `httptest.NewRecorder` + a real (or fake) service.

Example handler test shape:
```go
func TestGetProjects(t *testing.T) {
    req := httptest.NewRequest("GET", "/projects", nil)
    w := httptest.NewRecorder()

    handler := http.HandlerFunc(GetProjectsHandler)
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", w.Code)
    }
}
```

## Plans

Every approved implementation plan must be saved at `plans/NNN-short-description.md` (e.g. `plans/001-initial-infrastructure.md`) in the repo root. Use a zero-padded sequential number. Write the plan file before starting implementation.

After implementation, append a **Deviations** section to the plan file documenting every change that invalidates or modifies something explicitly stated in the plan. Each entry must include what changed and why. Format as a markdown table with columns `#`, `What changed`, `Why`.

## Docker Compose Profiles

Use profiles to start only what you need:

```sh
docker compose --profile infra up -d    # postgres, kratos, keto, etc.
docker compose --profile app up -d      # the API itself
docker compose --profile full up -d     # everything
```
