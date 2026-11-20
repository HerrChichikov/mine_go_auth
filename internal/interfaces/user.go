package interfaces

import (
	"context"

	"alpha/internal/objects/dto"
	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type UserInterface interface {
	Create(ctx context.Context, user *models.User) error

	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByNickname(ctx context.Context, nickname string) (*models.User, error)

	ExistsByEmail(ctx context.Context, email string) (bool, error)
	ExistsByNickname(ctx context.Context, nickname string) (bool, error)

	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error
	VerifyEmail(ctx context.Context, id uuid.UUID) error

	IncrementReferralsCount(ctx context.Context, id uuid.UUID) error

	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type UserUseCase interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*models.User, error)
	Login(ctx context.Context, email, password string) (sessionToken string, user *models.User, err error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error)
}
