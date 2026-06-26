-- name: CreateApiToken :one
INSERT INTO api_tokens (name, token_hash, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetApiToken :one
SELECT * FROM api_tokens WHERE id = $1;

-- name: GetApiTokenByHashWithOwner :one
SELECT
    api_tokens.id, api_tokens.created_by,
    users.role AS owner_role, users.disabled AS owner_disabled
FROM api_tokens
LEFT JOIN users ON users.id = api_tokens.created_by
WHERE api_tokens.token_hash = $1;

-- name: ListApiTokensWithOwner :many
SELECT api_tokens.*, users.username AS owner_username
FROM api_tokens
LEFT JOIN users ON users.id = api_tokens.created_by
ORDER BY api_tokens.created_at;

-- name: ListApiTokensForOwner :many
SELECT api_tokens.*, users.username AS owner_username
FROM api_tokens
LEFT JOIN users ON users.id = api_tokens.created_by
WHERE api_tokens.created_by = $1
ORDER BY api_tokens.created_at;

-- name: DeleteApiToken :exec
DELETE FROM api_tokens WHERE id = $1;

-- name: TouchApiTokenLastUsed :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;
