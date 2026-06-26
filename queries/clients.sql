-- name: CreateClient :one
INSERT INTO clients (name, access_token_hash, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetClient :one
SELECT * FROM clients WHERE id = $1;

-- name: GetClientByTokenHash :one
SELECT * FROM clients WHERE access_token_hash = $1;

-- name: ListClients :many
SELECT * FROM clients ORDER BY created_at;

-- name: ListClientsWithOwner :many
SELECT clients.*, users.username AS owner_username
FROM clients
LEFT JOIN users ON users.id = clients.created_by
ORDER BY clients.created_at;

-- name: ListClientsForOwner :many
SELECT clients.*, users.username AS owner_username
FROM clients
LEFT JOIN users ON users.id = clients.created_by
WHERE clients.created_by = $1
ORDER BY clients.created_at;

-- name: RenameClient :one
UPDATE clients SET name = $2 WHERE id = $1 RETURNING *;

-- name: DeleteClient :exec
DELETE FROM clients WHERE id = $1;

-- name: TouchClientLastSeen :exec
UPDATE clients SET last_seen_at = now() WHERE id = $1;
