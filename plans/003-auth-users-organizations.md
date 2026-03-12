# 003 — Auth, Users, Organizations & Invites

## Context

The project has infrastructure in place (Postgres, Kratos, Keto via Docker Compose, config loading, health endpoint) but no domain code yet. This plan implements:
- Authentication middleware using Ory Kratos session cookies
- Ory Keto client for permission checks
- `organization`, `role`, `user`, and `invite` domains with their full handler→service→repository stacks
- Registration flow: user signs up in Kratos → authenticates → lists pending invites → accepts one (joins org) OR creates own org (becomes `admin`)

## Decisions

- Auth: cookie only (`ory_kratos_session`), validated via Kratos Admin API `GET /sessions/whoami`
- All onboarding endpoints require auth — the user is already authenticated with Kratos when they go through the invite/org creation flow
- Kratos manages user deactivation/session revocation — no `active` field in local DB
- All tables: UUID PKs, `created_at` + `updated_at` (timestamptz), foreign key columns named `<entity>_id` (e.g. `organization_id`, not `org_id`)
- Roles: fixed — `admin`, `manager`, `human-resource`, `developer` — in a separate `role.go` file
- Keto relation tuples: `organizations:<org-id>#<role>@<user-id>`
- HR cannot invite as admin, cannot set anyone as admin — only admins can assign admin role
- HR restriction (can't change own role, can't change other HR/admin roles) → enforced in service layer
- 1 user = exactly 1 org, always
- Invite token: `crypto/rand` (32 bytes) encoded as base64url → raw returned in API response (email integration later), SHA-256 hash stored in DB — never store raw token in DB

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
organization_id uuid NOT NULL REFERENCES organizations(id)
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()

-- invites
id              uuid PRIMARY KEY DEFAULT gen_random_uuid()
organization_id uuid NOT NULL REFERENCES organizations(id)
email           varchar(255) NOT NULL
token_hash      bytea NOT NULL UNIQUE   -- SHA-256 of the raw token
role            varchar(50) NOT NULL
invited_by      uuid NOT NULL REFERENCES users(id)
accepted        boolean NOT NULL DEFAULT false
expires_at      timestamptz NOT NULL   -- created_at + 7 days
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()
```

### Invite token lifecycle

```
1. Generate: raw = base64url(crypto/rand(32 bytes))
2. Store:    token_hash = SHA-256(raw)  → written to DB
3. Deliver:  raw → returned in API response (email integration later)
4. Validate: receive raw from user → SHA-256(raw) → lookup by token_hash
```

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

### 4. Token helpers (`pkg/token/token.go`)

`Generate() (raw string, hash []byte, err error)` and `Hash(raw string) []byte`.

### 5. Keto client (`pkg/keto/keto.go`)

`HasRelation`, `AddRelation`, `DeleteRelation` — plain `net/http`, no SDK.

### 6. Auth middleware (`pkg/middleware/auth.go`)

Validates cookie via Kratos whoami. Sets `"user"` and `"identity_email"` in Gin context.

### 7. `internal/organization`

`organization.go` (struct) + `role.go` (Role type, consts, IsValid, IsAdminOrHR) + repository + service. No handler.

### 8. `internal/invite`

invite.go, repository, service (Create returns rawToken, GetPendingForEmail), handler (POST /invites, GET /invites/pending).

### 9. `internal/user`

user.go, repository, service (JoinViaInvite, CreateWithOrg, GetByIdentityID, ChangeRole), handler (all auth required).

Routes:
```
POST /users/join        → JoinViaInvite  (body: {token})
POST /users/create-org  → CreateWithOrg  (body: {org_name})
GET  /users/me          → Me
PUT  /users/:id/role    → ChangeRole     (body: {role})
```

### 10. Wire-up (`cmd/api/main.go`)

DB connection + instantiate all repos, services, handlers + register routes.

### 11. Tests

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
| `pkg/token/token.go` | create |
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

_None yet — to be filled during implementation._
