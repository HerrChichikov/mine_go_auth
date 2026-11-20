package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"alpha/constants"
	"alpha/internal/interfaces"
	"alpha/internal/modules"
	"alpha/internal/objects/dto"
	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"

	"github.com/google/uuid"
)

type userUseCase struct {
	userRepo       interfaces.UserInterface
	invitationRepo interfaces.InvitationInterface
	sessionRepo    interfaces.SessionInterface
	tokenService   interfaces.TokenService
}

func NewUserUseCase(
	userRepo interfaces.UserInterface,
	invitationRepo interfaces.InvitationInterface,
	sessionRepo interfaces.SessionInterface,
	tokenService interfaces.TokenService,
) interfaces.UserUseCase {
	return &userUseCase{
		userRepo:       userRepo,
		invitationRepo: invitationRepo,
		sessionRepo:    sessionRepo,
		tokenService:   tokenService,
	}
}

func (uc *userUseCase) Register(ctx context.Context, req dto.RegisterRequest) (*models.User, error) {
	if req.Email == "" || req.Nickname == "" || req.Password == "" {
		return nil, uerr.ErrInvalidInput
	}

	existsEmail, err := uc.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if existsEmail {
		return nil, uerr.ErrEmailExists
	}

	existsNickname, err := uc.userRepo.ExistsByNickname(ctx, req.Nickname)
	if err != nil {
		return nil, fmt.Errorf("check nickname: %w", err)
	}
	if existsNickname {
		return nil, uerr.ErrNicknameExists
	}

	invitation, err := uc.invitationRepo.GetByCode(ctx, req.InvitationCode)
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	if invitation.IsUsed() {
		return nil, uerr.ErrInvitationUsed
	}
	if invitation.IsExpired() {
		return nil, uerr.ErrInvitationExpired
	}

	passwordHash, err := modules.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Nickname:     req.Nickname,
		PasswordHash: passwordHash,
		Role:         models.UserRoleUser,
		IsActive:     true,
		ReferredBy:   &invitation.ID,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	if err := uc.invitationRepo.MarkAsUsed(ctx, invitation.ID, user.ID); err != nil {
		return nil, fmt.Errorf("update invitation: %w", err)
	}

	if err := uc.userRepo.IncrementReferralsCount(ctx, invitation.CreatedBy); err != nil {
		return nil, fmt.Errorf("increment referrals: %w", err)
	}

	return user, nil
}

func (uc *userUseCase) Login(ctx context.Context, email, password string) (string, *models.User, error) {
	user, err := uc.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, uerr.ErrUserNotFound) {
			return "", nil, uerr.ErrInvalidCredentials
		}
		return "", nil, fmt.Errorf("get user: %w", err)
	}

	if !modules.VerifyPassword(password, user.PasswordHash) {
		return "", nil, uerr.ErrInvalidCredentials
	}

	if !user.IsActive {
		return "", nil, uerr.ErrUserInactive
	}

	sessionToken, err := uc.createSession(ctx, user.ID)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	if err := uc.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		return "", nil, fmt.Errorf("update last login: %w", err)
	}

	return sessionToken, user, nil
}

func (uc *userUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, uerr.ErrUserNotFound) {
			return nil, uerr.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

func (uc *userUseCase) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
	sessionToken := modules.GenerateSessionToken()
	tokenHash, err := modules.HashPassword(sessionToken)
	if err != nil {
		return "", fmt.Errorf("hash token: %w", err)
	}

	// Создаем сессию
	session := &models.Session{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(constants.SESSION_EXPIRES_DELAY),
	}

	if err := uc.sessionRepo.Create(ctx, session); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return sessionToken, nil
}
