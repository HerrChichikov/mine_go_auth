-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_invitations_reserved_token_hash;
DROP INDEX IF EXISTS idx_invitations_reserved_at;
ALTER TABLE invitations 
    DROP COLUMN IF EXISTS reserved_token_hash,
    DROP COLUMN IF EXISTS reserved_at;
-- +goose StatementEnd