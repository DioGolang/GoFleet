-- name: CreateOutboxEvent :exec
INSERT INTO outbox (
id, aggregate_type, aggregate_id, event_type, payload, topic, status
) VALUES (
$1, $2, $3, $4, $5, $6, 'PENDING'
);

-- name: FetchPendingOutboxEvents :many
SELECT id, event_type, payload, topic
FROM outbox
WHERE status = 'PENDING'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxAsProcessing :exec
UPDATE outbox
SET status = 'PROCESSING', updated_at = NOW()
WHERE id = $1;

-- name: MarkOutboxAsPublished :exec
UPDATE outbox
SET status = 'PUBLISHED', published_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: MarkOutboxAsFailed :exec
UPDATE outbox
SET status = 'FAILED',
error_msg = $2,
retry_count = retry_count + 1,
updated_at = NOW()
WHERE id = $1;

-- name: DeleteOldOutboxEvents :exec
DELETE FROM outbox
WHERE status IN ('PUBLISHED', 'FAILED')
  -- O cast ::text força o SQLC a gerar o argumento como string no Go
  AND created_at < NOW() - (sqlc.arg(interval)::text)::interval;