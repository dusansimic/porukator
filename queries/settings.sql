-- name: GetSettings :one
SELECT delay_ms, jitter_ms FROM settings WHERE id = 1;

-- name: UpdateSettings :one
UPDATE settings SET delay_ms = $1, jitter_ms = $2 WHERE id = 1
RETURNING delay_ms, jitter_ms;
