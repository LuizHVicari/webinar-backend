# 002 — GitHub Actions CI/CD Pipeline

## Context

The project had no CI/CD configuration. This plan establishes a reusable GitHub Actions pipeline that:
- Validates code quality (tests + lint) on every PR and push
- Builds and publishes Docker images to GHCR only on pushes to `dev` or `main`

---

## Workflow Structure

```
.github/
  workflows/
    run-tests.yml          # reusable: go test
    run-lint.yml           # reusable: golangci-lint
    build-and-publish.yml  # reusable: Docker build + push to GHCR
    ci-push.yml            # coordinator: push to dev or main
    ci-pr.yml              # coordinator: PR targeting dev or main
```

---

## Reusable Workflows

### `run-tests.yml`
- Triggered by `workflow_call`
- Sets up Go 1.25 with module + build cache
- Runs `go test -race -count=1 ./...`
- Commented-out postgres service block for future DB tests

### `run-lint.yml`
- Triggered by `workflow_call`
- Uses `golangci/golangci-lint-action@v6` pinned to `v2.1.6`
- `setup-go` has `cache: false` (golangci-lint-action manages its own cache)

### `build-and-publish.yml`
- Triggered by `workflow_call` with inputs:
  - `environment` (string): `dev` or `prod`
  - `tag_prefix` (string): e.g. `dev` or `v`
- Permissions: `contents: read`, `packages: write`
- Logs in to `ghcr.io` via `GITHUB_TOKEN`
- Derives image tags via shell script:
  - `prod` → `ghcr.io/luizhvicari/webinar-backend:v<sha7>` + `:latest`
  - `dev` → `ghcr.io/luizhvicari/webinar-backend:dev-<sha7>` + `:dev-latest`
- Builds with `docker/build-push-action@v6` using BuildKit GHA layer cache

---

## Coordinator Workflows

### `ci-push.yml`
- `on: push: branches: [dev, main]`
- `test` and `lint` run in parallel
- `publish-dev` runs after both, only if `refs/heads/dev`
- `publish-prod` runs after both, only if `refs/heads/main`
- Two separate jobs with mutually exclusive `if:` conditions (GitHub Actions limitation: no dynamic expressions in `with:` of reusable workflow calls)

### `ci-pr.yml`
- `on: pull_request: branches: [dev, main]`
- `test` and `lint` run in parallel
- No publish step

---

## Image Tagging

| Branch | environment | tag_prefix | Tags |
|--------|-------------|------------|------|
| `dev`  | `dev`       | `dev`      | `dev-<sha7>`, `dev-latest` |
| `main` | `prod`      | `v`        | `v<sha7>`, `latest`        |

---

## Repository Settings Required
- Settings → Actions → General → Workflow permissions: **Read and write permissions**
- After first push: configure package visibility in GitHub Packages if needed

---

## Verification
1. Push to `dev` → `ci-push.yml`: tests + lint pass, image published as `dev-<sha7>` and `dev-latest`
2. Open PR `dev` → `main` → `ci-pr.yml`: tests + lint only, no image published
3. Merge to `main` → `ci-push.yml`: image published as `v<sha7>` and `latest`
4. Confirm package appears under the repository in GitHub Packages

---

## Deviations

| # | What changed | Why |
|---|---|---|
| 1 | `golangci/golangci-lint-action@v6` → `@v7` in `run-lint.yml` | `golangci-lint-action@v6` does not support golangci-lint v2.x; `@v7` is required for v2 |
| 2 | golangci-lint version `v2.1.6` → `v2.10.1` in `run-lint.yml` | v2.1.6 is built with Go 1.24 and fails when `go.mod` targets Go 1.25; v2.10.1 is built with Go 1.25 and supports it |
| 3 | Go version reverted to `1.25` in `run-lint.yml` and `run-tests.yml` | local toolchain is Go 1.25.8 and `go mod tidy` always writes `go 1.25.0` to `go.mod`; downgrading was not viable |
| 4 | Removed manual `actions/cache@v4` step from `run-tests.yml` and `build-and-publish-binary.yml` | `setup-go@v5` manages Go module and build cache by default; the manual cache step conflicted with it, causing `tar: Cannot open: File exists` errors on read-only module cache files |
| 5 | Updated all actions to Node.js 24-compatible versions: `checkout@v6`, `setup-go@v6`, `golangci-lint-action@v9`, `docker/login-action@v4`, `docker/setup-buildx-action@v4`, `docker/build-push-action@v7` | Node.js 20 actions deprecated; forced to Node.js 24 from June 2026 |
| 6 | Added `cache: false` to `setup-go@v6` in `build-and-publish-binary.yml` | default cache restore was failing with `tar exit code 2` due to pre-existing module cache from checkout; no `go mod download` step in this workflow |
| 7 | Added `run-build.yml` reusable workflow; lint and build now run in parallel first, tests run after both succeed | tests use testcontainers and are slow (~5–7 min); lint and build catch most issues faster and gate the expensive test run |
| 8 | Added `run-vet.yml` reusable workflow (`go vet ./...`); vet runs in parallel with lint and build, gates tests | catches type-system and correctness issues that golangci-lint doesn't always surface; fast (no containers) |
