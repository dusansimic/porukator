DROP INDEX IF EXISTS api_tokens_created_by_idx;
ALTER TABLE api_tokens DROP COLUMN IF EXISTS created_by;
