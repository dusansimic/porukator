-- API-token ownership. Pre-existing tokens get NULL (treated as global/admin).
-- Deleting the owner removes their tokens (so a manager's key can't silently
-- become a global legacy key).
ALTER TABLE api_tokens
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX api_tokens_created_by_idx ON api_tokens (created_by);
