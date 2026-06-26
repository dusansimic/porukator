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
  AND (sqlc.narg('batch_id')::uuid IS NULL OR batch_id = sqlc.narg('batch_id'))
ORDER BY received_at DESC
LIMIT sqlc.arg('lim');

-- name: ListMessagesForOwner :many
SELECT messages.* FROM messages
JOIN clients ON clients.id = messages.client_id
WHERE clients.created_by = sqlc.arg('owner')
  AND (sqlc.narg('status')::message_status IS NULL OR messages.status = sqlc.narg('status'))
  AND (sqlc.narg('client_id')::uuid IS NULL OR messages.client_id = sqlc.narg('client_id'))
  AND (sqlc.narg('batch_id')::uuid IS NULL OR messages.batch_id = sqlc.narg('batch_id'))
ORDER BY messages.received_at DESC
LIMIT sqlc.arg('lim');

-- name: GetMessagesByIDs :many
SELECT * FROM messages WHERE id = ANY(sqlc.arg('ids')::uuid[]);

-- name: GetMessagesByIDsForOwner :many
SELECT messages.* FROM messages
JOIN clients ON clients.id = messages.client_id
WHERE messages.id = ANY(sqlc.arg('ids')::uuid[])
  AND clients.created_by = sqlc.arg('owner');
