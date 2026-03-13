# 004 — Swaggo API Documentation

## Context

The API has 7 endpoints across two domains (user, organization) plus a health check, but no API documentation. This plan adds Swaggo via comment annotations so that a Swagger UI is served at `/swagger/index.html`. The plan also wires a `just docs` command and updates the README with installation/usage instructions.

## Decisions

- **Domain types vs response DTOs:** `user.User` and `organization.Invite` are returned directly from handlers. Adding `json` struct tags to them makes the serialization contract explicit. No new response DTOs needed — those types *are* the response shape. `organization.Organization` is never returned from handlers, so no change needed there.
- **Security definition:** cookie-based auth (`ory_kratos_session`). Use `@securityDefinitions.apikey` with `@in cookie`.
- **`ErrorResponse`:** add to `pkg/common/errors.go` as a named exported struct so swaggo can resolve cross-package references in `@Failure` annotations.
- **`docs/` directory:** committed to repo, same as `sqlc/generated/` — CI can validate without running `swag init`.
- **swag CLI:** installed globally via `go install github.com/swaggo/swag/cmd/swag@latest` (not a runtime dep, not in `go.mod`). `just docs` calls `swag init`.
- **Runtime deps:** `github.com/swaggo/gin-swagger` and `github.com/swaggo/files` are imported in `main.go` and go into `go.mod require`. The generated `docs/` package is also imported at runtime (blank import).

## Steps

### 1. Add runtime dependencies
```
go get github.com/swaggo/gin-swagger
go get github.com/swaggo/files
```

The `swag` CLI is a dev tool only — install separately:
```
go install github.com/swaggo/swag/cmd/swag@latest
```

### 2. Add `ErrorResponse` to `pkg/common/errors.go`
```go
// ErrorResponse is the standard error body returned on all failed requests.
type ErrorResponse struct {
    Error string `json:"error"`
}
```

### 3. Add JSON tags to domain types

**`internal/user/user.go`:**
```go
type User struct {
    ID             uuid.UUID  `json:"id"`
    IdentityID     uuid.UUID  `json:"identity_id"`
    OrganizationID *uuid.UUID `json:"organization_id"`
    CreatedAt      time.Time  `json:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at"`
}
```

**`internal/organization/invite.go`:**
```go
type Invite struct {
    ID             uuid.UUID `json:"id"`
    OrganizationID uuid.UUID `json:"organization_id"`
    Email          string    `json:"email"`
    Role           Role      `json:"role"`
    InvitedBy      uuid.UUID `json:"invited_by"`
    Accepted       bool      `json:"accepted"`
    ExpiresAt      time.Time `json:"expires_at"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

### 4. Swaggo annotations on handlers

**`internal/user/handler.go`** — 4 handlers:

`me`:
```go
// @Summary      Get current user
// @Tags         users
// @Produce      json
// @Success      200  {object}  User
// @Failure      401  {object}  common.ErrorResponse
// @Failure      404  {object}  common.ErrorResponse
// @Failure      500  {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/me [get]
```

`joinViaInvite`:
```go
// @Summary      Join an organization via invite
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      JoinViaInviteRequest  true  "Invite ID"
// @Success      200   {object}  User
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      409   {object}  common.ErrorResponse
// @Failure      422   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/join [post]
```

`createOrg`:
```go
// @Summary      Create an organization
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body      CreateOrgRequest  true  "Organization name"
// @Success      201   {object}  User
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      409   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/create-org [post]
```

`changeRole`:
```go
// @Summary      Change a user's role
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "Target user UUID"
// @Param        body  body      ChangeRoleRequest  true  "New role"
// @Success      204
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      403   {object}  common.ErrorResponse
// @Failure      422   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /users/{id}/role [put]
```

**`internal/organization/handler.go`** — 2 handlers:

`createInvite`:
```go
// @Summary      Create an invite
// @Tags         invites
// @Accept       json
// @Produce      json
// @Param        body  body      CreateInviteRequest  true  "Invite details"
// @Success      201   {object}  Invite
// @Failure      400   {object}  common.ErrorResponse
// @Failure      401   {object}  common.ErrorResponse
// @Failure      403   {object}  common.ErrorResponse
// @Failure      500   {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /invites [post]
```

`listPendingInvites`:
```go
// @Summary      List pending invites
// @Tags         invites
// @Produce      json
// @Success      200  {array}   Invite
// @Failure      401  {object}  common.ErrorResponse
// @Failure      500  {object}  common.ErrorResponse
// @Security     KratosSession
// @Router       /invites/pending [get]
```

### 5. Update `cmd/api/main.go`

Extract health handler to a named function `healthHandler` and add global annotation block above `package main`.

### 6. Generate `docs/`
```
swag init -g cmd/api/main.go -o docs
```

### 7. Add `just docs` to justfile

### 8. Update README.md

### 9. GitHub Actions — publish swagger artifact

Create `.github/workflows/publish-swagger.yml` as a reusable workflow. Call it in `ci-push.yml` after tests pass.

## Files Modified

| File | Change |
|------|--------|
| `go.mod` | Add swaggo runtime deps |
| `pkg/common/errors.go` | Add `ErrorResponse` struct |
| `internal/user/user.go` | Add JSON struct tags |
| `internal/organization/invite.go` | Add JSON struct tags |
| `internal/user/handler.go` | Add swaggo annotations to 4 handlers |
| `internal/organization/handler.go` | Add swaggo annotations to 2 handlers |
| `cmd/api/main.go` | Global annotation block, extract health handler, swagger route + imports |
| `docs/` (generated) | Created by `swag init` |
| `justfile` | Add `docs` recipe |
| `README.md` | Add API docs section |
| `.github/workflows/publish-swagger.yml` | New reusable workflow: generate + upload artifact |
| `.github/workflows/ci-push.yml` | Call `publish-swagger` job after tests |

## Deviations

| # | What changed | Why |
|---|-------------|-----|
| 1 | `github.com/swaggo/swag` upgraded from v1.8.12 to v1.16.4 | `go get` pulled v1.8.12 but the `swag` CLI installed via `go install @latest` was v1.16.4; the generated `docs.go` used `LeftDelim`/`RightDelim` fields not present in v1.8.12, causing build failure |
