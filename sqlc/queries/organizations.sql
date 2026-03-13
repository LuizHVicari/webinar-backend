-- name: CreateOrganization :one
INSERT INTO organizations (id, name)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;
