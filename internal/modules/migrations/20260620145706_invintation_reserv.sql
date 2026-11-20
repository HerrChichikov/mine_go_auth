-- +goose Up
-- +goose StatementBegin
ALTER TABLE invitations 
    ADD COLUMN reserved_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX idx_invitations_reserved_at ON invitations(reserved_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_invitations_reserved_at;
ALTER TABLE invitations DROP COLUMN IF EXISTS reserved_at;
-- +goose StatementEnd