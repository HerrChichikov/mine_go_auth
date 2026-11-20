package interfaces

import (
	"context"

	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type SessionInterface interface {
	Create(ctx context.Context, session *models.Session) error

	GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Session, error)

	UpdateLastActive(ctx context.Context, id uuid.UUID) error

	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}
