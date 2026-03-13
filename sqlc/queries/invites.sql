-- name: CreateInvite :one
INSERT INTO invites (id, organization_id, email, role, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInviteByID :one
SELECT * FROM invites WHERE id = $1;

-- name: GetPendingInvitesByEmail :many
SELECT * FROM invites
WHERE email     = $1
  AND accepted  = false
  AND expires_at > now();

-- name: AcceptInvite :one
UPDATE invites
SET accepted   = true,
    updated_at = now()
WHERE id = $1
RETURNING *;
