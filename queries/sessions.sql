-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByHash :one
SELECT
    sessions.id, sessions.user_id, sessions.token_hash, sessions.created_at,
    sessions.last_used_at, sessions.expires_at,
    users.username AS username, users.role AS role, users.disabled AS disabled
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = $1;

-- name: TouchSession :exec
UPDATE sessions SET last_used_at = now() WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionByHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionsByUser :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: ListSessions :many
SELECT
    sessions.id, sessions.user_id, sessions.created_at, sessions.last_used_at,
    users.username AS username
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.expires_at > now()
ORDER BY sessions.last_used_at DESC;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();
