# webinar-backend

This project is being developed as part of a webinar demonstrating how to use [Claude](https://claude.ai/code) to build real-world applications — from initial scaffolding to a production-ready backend.

The backend is a Go REST API using Gin, backed by PostgreSQL, Redis, Ory Kratos (authentication) and Ory Keto (authorization).

## Requirements

| Tool | Purpose | Install |
|------|---------|---------|
| [Go 1.25+](https://go.dev/dl/) | Language runtime | https://go.dev/dl/ |
| [Docker](https://docs.docker.com/get-docker/) | Run infrastructure services | https://docs.docker.com/get-docker/ |
| [Docker Compose v2](https://docs.docker.com/compose/install/) | Orchestrate services | bundled with Docker Desktop |
| [just](https://github.com/casey/just#installation) | Task runner | https://github.com/casey/just#installation |
| [golangci-lint](https://golangci-lint.run/usage/install/) | Linter | https://golangci-lint.run/docs/welcome/install/ |
| [swag](https://github.com/swaggo/swag#getting-started) | Swagger doc generator | `go install github.com/swaggo/swag/cmd/swag@latest` |

## Running

**1. Copy the environment file and adjust if needed:**

```sh
cp .env.example .env
```

**2. Start infrastructure (PostgreSQL, Redis, Kratos, Keto):**

```sh
just docker-up infra
```

**3. Run the API with hot reload:**

```sh
just dev
```

The API will be available at `http://localhost:8080`. Verify with:

```sh
curl http://localhost:8080/health
# {"status":"ok"}
```

### Other commands

```sh
just run            # run without hot reload
just build          # compile binary to bin/api
just test           # run all tests
just test ./internal/user/...  # run tests for a specific package
just lint           # run linter
just migrate        # apply database migrations
just generate       # run sqlc code generation
just docs           # regenerate Swagger documentation
just docker-up app  # run the API in Docker instead of locally
just docker-up full # start everything (infra + app)
just docker-down    # stop infra services
```

## API Documentation

With the server running, the Swagger UI is available at:

```
http://localhost:8080/swagger/index.html
```

To regenerate the docs after changing handler annotations:

```sh
just docs
```
