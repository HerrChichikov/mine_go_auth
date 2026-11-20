package interfaces

import (
	"context"
	"time"

	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type InvitationInterface interface {
	Create(ctx context.Context, inv *models.Invitation) error

	GetByID(ctx context.Context, id uuid.UUID) (*models.Invitation, error)
	GetByCode(ctx context.Context, code string) (*models.Invitation, error)
	GetByCreator(ctx context.Context, userID uuid.UUID) ([]*models.Invitation, error)

	Use(ctx context.Context, id uuid.UUID, userID uuid.UUID) error

	ClearExpiredReservations(ctx context.Context) (int64, error)

	Delete(ctx context.Context, id uuid.UUID) error

	Update(ctx context.Context, invite *models.Invitation) error
	MarkAsUsed(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type InviteUseCase interface {
	Create(ctx context.Context, userID uuid.UUID) (invite *models.Invitation, err error)
	Reserve(ctx context.Context, code string) (token string, err error)
}

type TokenService interface {
	GenerateRegistrationToken(invitationID string, ttl time.Duration) (token string, err error)
	ParseRegistrationToken(token string) (invitationID string, err error)
}
