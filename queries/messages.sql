-- name: InsertMessage :one
INSERT INTO messages (batch_id, phone_number, content, client_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListPendingForClient :many
SELECT * FROM messages
WHERE client_id = $1 AND status = 'pending'
ORDER BY received_at;

-- name: MarkDispatched :execrows
UPDATE messages
SET status = 'dispatched', dispatched_at = now()
WHERE id = $1 AND status = 'pending';

-- name: MarkSent :one
UPDATE messages
SET status = 'sent', sent_at = $2, error = ''
WHERE id = $1
RETURNING *;

-- name: MarkFailed :one
UPDATE messages
SET status = 'failed', error = $2
WHERE id = $1
RETURNING *;

-- name: ListMessages :many
SELECT * FROM messages
WHERE (sqlc.narg('status')::message_status IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('client_id')::uuid IS NULL OR client_id = sqlc.narg('client_id'))
ORDER BY received_at DESC
LIMIT sqlc.arg('lim');
