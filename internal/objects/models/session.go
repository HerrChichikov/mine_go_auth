package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID           uuid.UUID `db:"id" json:"id"`
	UserID       uuid.UUID `db:"user_id" json:"user_id"`
	TokenHash    string    `db:"token_hash" json:"-"` // Хэш токена, не отдаем в API
	IPAddress    *string   `db:"ip_address" json:"ip_address"`
	UserAgent    *string   `db:"user_agent" json:"user_agent"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	ExpiresAt    time.Time `db:"expires_at" json:"expires_at"`
	LastActiveAt time.Time `db:"last_active_at" json:"last_active_at"`
}

// Методы для удобства
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsActive() bool {
	return !s.IsExpired()
}

func (s *Session) UpdateLastActive() {
	s.LastActiveAt = time.Now()
}
