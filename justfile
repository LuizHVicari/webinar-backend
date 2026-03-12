set windows-shell := ["C:\\Program Files\\Git\\bin\\sh.exe", "-c"]

# Run the API with hot reload (watches for file changes) [default]
dev:
    go tool air

# Run the API without hot reload
run:
    go run ./cmd/api

# Compile the binary to bin/api
build:
    go build -o bin/api ./cmd/api

# Run tests; optionally pass a package path: just test ./internal/user/...
test *args:
    go test {{args}} ./...

# Run linter
lint:
    golangci-lint run ./...

# Apply pending database migrations
migrate:
    goose -dir migrations postgres "$DATABASE_URL" up

# Run sqlc code generation
generate:
    go generate ./...

# Start Docker Compose services for the given profile (default: infra)
# Profiles: infra (postgres, redis, kratos, keto), app (api), full (everything)
docker-up profile="infra":
    docker compose --profile {{profile}} up -d

# Stop Docker Compose services for the given profile (default: infra)
docker-down profile="infra":
    docker compose --profile {{profile}} down
