-- +goose Down
-- +goose StatementBegin
-- Сначала удаляем поле referred_by из users
DROP INDEX IF EXISTS idx_users_referred_by;
ALTER TABLE users DROP COLUMN IF EXISTS referred_by;

-- Потом удаляем таблицу invitations
DROP TABLE IF EXISTS invitations;
-- +goose StatementEnd