package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"alpha/constants"
	"alpha/internal/interfaces"
	"alpha/internal/modules"
	ierr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type inviteUseCase struct {
	invitationRepo interfaces.InvitationInterface
	userRepo       interfaces.UserInterface
	tokenService   interfaces.TokenService
}

func NewInviteUseCase(
	invitationRepo interfaces.InvitationInterface,
	userRepo interfaces.UserInterface,
	tokenService interfaces.TokenService,
) interfaces.InviteUseCase {
	return &inviteUseCase{
		invitationRepo: invitationRepo,
		userRepo:       userRepo,
		tokenService:   tokenService,
	}
}

func (uc *inviteUseCase) Reserve(ctx context.Context, code string) (string, error) {
	if code == "" {
		return "", ierr.ErrInvalidInput
	}

	_, _ = uc.invitationRepo.ClearExpiredReservations(ctx)

	invitation, err := uc.invitationRepo.GetByCode(ctx, code)
	if err != nil {
		return "", err
	}

	if invitation.IsUsed() {
		return "", ierr.ErrInvitationUsed
	}
	if invitation.IsExpired() {
		return "", ierr.ErrInvitationExpired
	}
	if invitation.IsReserved() {
		return "", ierr.ErrInvitationReserved
	}

	token, err := uc.tokenService.GenerateRegistrationToken(
		invitation.ID.String(),
		constants.RESERVATION_TTL,
	)
	if err != nil {
		return "", fmt.Errorf("Ошибка генерации токена приглашения: %w", err)
	}

	invitation.Reserve()
	if err := uc.invitationRepo.Update(ctx, invitation); err != nil {
		return "", fmt.Errorf("Ошибка обновления приглашения: %w", err)
	}

	return token, nil
}

func (uc *inviteUseCase) Create(ctx context.Context, userID uuid.UUID) (*models.Invitation, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ierr.ErrUserNotFound) {
			return nil, ierr.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	activeInvites, err := uc.invitationRepo.GetByCreator(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get active invitations: %w", err)
	}

	activeCount := 0
	for _, inv := range activeInvites {
		if !inv.IsUsed() {
			activeCount++
		}
	}

	if activeCount >= user.MaxReferrals {
		return nil, ierr.ErrInvitationLimit
	}

	code := modules.GenerateCode()

	now := time.Now()
	invitation := &models.Invitation{
		ID:        uuid.New(),
		Code:      code,
		CreatedBy: userID,
		ExpiresAt: nil,
		CreatedAt: now,
	}

	if err := uc.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	return invitation, nil
}
