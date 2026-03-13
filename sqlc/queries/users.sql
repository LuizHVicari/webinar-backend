-- name: CreateUser :one
INSERT INTO users (id, identity_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByIdentityID :one
SELECT * FROM users WHERE identity_id = $1;

-- name: UpdateUserOrganization :one
UPDATE users
SET organization_id = $2,
    updated_at      = now()
WHERE id = $1
RETURNING *;
