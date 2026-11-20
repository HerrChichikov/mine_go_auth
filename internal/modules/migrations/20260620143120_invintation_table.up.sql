-- +goose Up
-- +goose StatementBegin
-- Сначала создаем таблицу invitations
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Код приглашения
    code VARCHAR(20) UNIQUE NOT NULL,
    
    -- Кто создал приглашение
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Кто использовал приглашение (NULL если не использован)
    used_by UUID REFERENCES users(id) ON DELETE SET NULL,
    used_at TIMESTAMP WITH TIME ZONE, -- NULL если не использован
    
    -- Срок действия (опционально)
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Временные метки
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для invitations
CREATE INDEX idx_invitations_code ON invitations(code);
CREATE INDEX idx_invitations_created_by ON invitations(created_by);
CREATE INDEX idx_invitations_used_by ON invitations(used_by);
CREATE INDEX idx_invitations_expires_at ON invitations(expires_at);
CREATE INDEX idx_invitations_used_at ON invitations(used_at);

-- Теперь добавляем поле referred_by в users
ALTER TABLE users ADD COLUMN referred_by UUID REFERENCES invitations(id) ON DELETE SET NULL;
CREATE INDEX idx_users_referred_by ON users(referred_by);
-- +goose StatementEnd