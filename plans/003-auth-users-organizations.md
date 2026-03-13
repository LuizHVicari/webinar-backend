# 003 — Auth, Users, Organizations & Invites

## Context

The project has infrastructure in place (Postgres, Kratos, Keto via Docker Compose, config loading, health endpoint) but no domain code yet. This plan implements:
- Authentication middleware using Ory Kratos session cookies
- Ory Keto client for permission checks
- `organization`, `role`, `user`, and `invite` domains with their full handler→service→repository stacks
- Registration flow: user signs up in Kratos → authenticates → lists pending invites (by email) → accepts one (joins org) OR creates own org (becomes `admin`)
- A user may exist without belonging to any organization; org-scoped actions require membership

## Decisions

- Auth: cookie only (`ory_kratos_session`), validated via Kratos Admin API `GET /sessions/whoami`
- All onboarding endpoints require auth — the user is already authenticated with Kratos when they go through the invite/org creation flow
- Kratos manages user deactivation/session revocation — no `active` field in local DB
- All tables: UUID PKs, `created_at` + `updated_at` (timestamptz), foreign key columns named `<entity>_id` (e.g. `organization_id`, not `org_id`)
- Roles: fixed — `admin`, `manager`, `human-resource`, `developer` — in a separate `role.go` file
- Keto relation tuples: `organizations:<org-id>#<role>@<user-id>`
- HR cannot invite as admin, cannot set anyone as admin — only admins can assign admin role
- HR restriction (can't change own role, can't change other HR/admin roles) → enforced in service layer
- 1 user = at most 1 org; joining or creating an org is required before accessing org-scoped resources
- Invite lookup: by the user's email (from Kratos identity) — no token needed; invite is accepted by its UUID
- No email sending; invites are surfaced via API for the UI to display to the authenticated user

## Domain Model

### Tables

```sql
-- organizations
id              uuid PRIMARY KEY DEFAULT gen_random_uuid()
name            varchar(255) NOT NULL
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()

-- users
id              uuid PRIMARY KEY DEFAULT gen_random_uuid()
identity_id     uuid NOT NULL UNIQUE   -- Kratos identity ID
organization_id uuid REFERENCES organizations(id)  -- nullable: user may not yet belong to an org
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()

-- invites
id              uuid PRIMARY KEY DEFAULT gen_random_uuid()
organization_id uuid NOT NULL REFERENCES organizations(id)
email           varchar(255) NOT NULL
role            varchar(50) NOT NULL
invited_by      uuid NOT NULL REFERENCES users(id)
accepted        boolean NOT NULL DEFAULT false
expires_at      timestamptz NOT NULL   -- created_at + 7 days
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()
```

### Invite lifecycle

```
1. Admin/HR creates invite: POST /invites  → stored with target email + role
2. Invitee logs in → GET /invites/pending  → lists their pending (non-accepted, non-expired) invites by email
3. Invitee accepts:  POST /users/join  { invite_id }  → user.organization_id set, Keto tuple written
```

No token generation or hashing involved — the invite UUID is the reference.

### Keto relation tuples

```
organizations:<org-id>#admin@<user-id>
organizations:<org-id>#manager@<user-id>
organizations:<org-id>#human-resource@<user-id>
organizations:<org-id>#developer@<user-id>
```

### Role permission matrix

| Action                          | admin | manager | human-resource | developer |
|---------------------------------|-------|---------|----------------|-----------|
| Everything                      | ✓     |         |                |           |
| CRUD tasks & stages             |       | ✓       |                |           |
| View tasks & stages             |       | ✓       |                | ✓         |
| Update task current stage       |       | ✓       |                | ✓         |
| Invite users (non-admin roles)  | ✓     |         | ✓              |           |
| Invite users as admin           | ✓     |         |                |           |
| Change user role (non-admin/HR) | ✓     |         | ✓ (see rules)  |           |
| Change admin/HR role            | ✓     |         |                |           |

HR role change rules (service layer):
- HR cannot invite with role `admin`
- HR cannot change their own role
- HR cannot change the role of another HR or admin
- Only admins can assign or change `admin` and `human-resource` roles

## Steps

### 1. Keto namespace config

Create `config/keto/namespaces/namespaces.ts`:

```ts
class Organization implements Namespace {
  related: {
    admin: User[]
    manager: User[]
    "human-resource": User[]
    developer: User[]
  }
}
```

Update `config/keto/keto.yml`: set `namespaces.location` to `/etc/config/keto/namespaces`.

### 2. Database migration

**File:** `migrations/001_create_users_organizations_invites.sql`

### 3. sqlc queries

`sqlc/queries/organizations.sql`, `sqlc/queries/users.sql`, `sqlc/queries/invites.sql` — then `just generate`.

### 4. Keto client (`pkg/keto/keto.go`)

`HasRelation`, `AddRelation`, `DeleteRelation` — plain `net/http`, no SDK.

### 5. Auth middleware (`pkg/middleware/auth.go`)

Validates cookie via Kratos whoami. Sets `"user"` and `"identity_email"` in Gin context.

### 6. `internal/organization`

`organization.go` (struct) + `role.go` (Role type, consts, IsValid, IsAdminOrHR) + repository + service. No handler.

### 7. `internal/invite`

`invite.go`, repository, service (`Create`, `GetPendingForEmail`), handler (`POST /invites`, `GET /invites/pending`).

### 8. `internal/user`

`user.go`, repository, service (`JoinViaInvite`, `CreateWithOrg`, `GetByIdentityID`, `ChangeRole`), handler (all auth required).

Routes:
```
POST /users/join        → JoinViaInvite  (body: {invite_id})
POST /users/create-org  → CreateWithOrg  (body: {org_name})
GET  /users/me          → Me
PUT  /users/:id/role    → ChangeRole     (body: {role})
```

### 9. Wire-up (`cmd/api/main.go`)

DB connection + instantiate all repos, services, handlers + register routes.

### 10. Tests

- Repository: testcontainers + real Postgres
- Service: mock repo + mock Keto
- Handler: httptest + fake service

## Files Created/Modified

| File | Action |
|------|--------|
| `config/keto/namespaces/namespaces.ts` | create |
| `config/keto/keto.yml` | update namespaces path |
| `migrations/001_create_users_organizations_invites.sql` | create |
| `sqlc/queries/organizations.sql` | create |
| `sqlc/queries/users.sql` | create |
| `sqlc/queries/invites.sql` | create |
| `sqlc/generated/` | regenerate |
| `pkg/keto/keto.go` | create |
| `pkg/middleware/auth.go` | create |
| `internal/organization/organization.go` | create |
| `internal/organization/role.go` | create |
| `internal/organization/repository.go` | create |
| `internal/organization/service.go` | create |
| `internal/invite/invite.go` | create |
| `internal/invite/repository.go` | create |
| `internal/invite/service.go` | create |
| `internal/invite/handler.go` | create |
| `internal/user/user.go` | create |
| `internal/user/repository.go` | create |
| `internal/user/service.go` | create |
| `internal/user/handler.go` | create |
| `cmd/api/main.go` | update |

## Deviations

| # | What changed | Why |
|---|---|---|
| 1 | `pkg/token/token.go` not created | Invite token/hash removed from design — invites identified by UUID only |
| 2 | `pkg/keto/keto.go` uses `github.com/ory/client-go` SDK instead of raw `net/http` | Ory provides an official Go SDK; safer and less boilerplate than hand-rolled HTTP |
| 3 | `pkg/middleware/auth.go` accepts a `UserResolver` interface and sets `user_id` + `org_id` in Gin context in addition to `identity_id`/`identity_email` | Handlers must not call user service to resolve DB user on every request; middleware resolves once |
| 4 | `internal/invite/` does not exist; all invite code lives in `internal/organization/` | Invite is a subdomain of Organization, not a separate bounded context |
| 5 | Structs named `OrganizationRepository`, `OrganizationService`, `InviteRepository`, `InviteService` (instead of plain `Repository`/`Service`) | Explicit naming requested since they share a package |
| 6 | `dtos.go` added to each domain package | Required for Swaggo compatibility — anonymous inline structs cannot be documented |
| 7 | UUID v7 (`uuid.NewV7()`) generated in application layer; INSERT queries accept `id` as parameter | User requirement: time-ordered UUIDs generated in Go, not by the DB |
| 8 | `sqlc/sqlc.yaml` updated with `sql_package: pgx/v5` and UUID type override | Required for pgx/v5 driver and `github.com/google/uuid` in generated code |
| 9 | `pkg/common/errors.go` added; `internal/organization/errors.go` owns all org/invite errors and the `HTTPStatus(err) int` mapping | Error-to-HTTP mapping belongs in `errors.go`, not handlers |
| 10 | No interfaces defined in provider packages (repositories/services expose concrete structs) | Go idiom: interfaces at the consumer side only |
| 11 | `TestMain` added to `internal/organization`, `internal/user`, and `pkg/middleware`; `testhelper` gains `StartPostgres`, `StartKeto`, `StartKratos` (return containers) and `TruncateTables`, `DeleteAllRelations`, `DeleteAllIdentities` (reset between tests); `keto.Client` gains `DeleteAllRelations`; per-test `NewPostgres`/`NewKeto`/`NewKratos` helpers kept for backwards compat | Each test was starting its own containers (330s + 391s + 154s total); shared containers + truncate/cleanup between tests reduced to ~12s + 11s + 10s |
