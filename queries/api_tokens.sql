-- name: CreateApiToken :one
INSERT INTO api_tokens (name, token_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetApiTokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = $1;

-- name: ListApiTokens :many
SELECT * FROM api_tokens ORDER BY created_at;

-- name: DeleteApiToken :exec
DELETE FROM api_tokens WHERE id = $1;

-- name: TouchApiTokenLastUsed :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;
