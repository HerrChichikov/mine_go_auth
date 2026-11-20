package interfaces

import (
	"context"
	"time"

	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type SessionInterface interface {
	Create(ctx context.Context, session models.AccountSession) error
	ResolveAccess(ctx context.Context, hash string) (models.AccountSession, error)
	Rotate(ctx context.Context, refreshHash, nextRefreshHash, nextAccessHash string, accessUntil time.Time) (models.AccountSession, error)
	Revoke(ctx context.Context, accessHash, refreshHash string) error
	RevokeUser(ctx context.Context, userID uuid.UUID) error
}
