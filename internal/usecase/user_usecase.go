package usecase

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"alpha/constants"
	"alpha/internal/interfaces"
	"alpha/internal/modules"
	"alpha/internal/objects/dto"
	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	"alpha/internal/repository"

	"github.com/google/uuid"
)

var accountName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{2,49}$`)

type UserUseCase struct {
	users     interfaces.UserInterface
	sessions  interfaces.SessionInterface
	attempts  *repository.AttemptRepository
	dummyHash string
}

func NewUserUseCase(users interfaces.UserInterface, sessions interfaces.SessionInterface, attempts *repository.AttemptRepository) (*UserUseCase, error) {
	dummy, err := modules.HashPassword(modules.GenerateToken(""))
	if err != nil {
		return nil, err
	}
	return &UserUseCase{users, sessions, attempts, dummy}, nil
}

func (uc *UserUseCase) Register(ctx context.Context, req dto.RegisterRequest) (*models.User, error) {
	if !accountName.MatchString(req.Nickname) || len(req.Password) < 8 || len(req.Password) > 72 ||
		len(req.InvitationCode) < 1 || len(req.InvitationCode) > 20 || len(req.Email) > 254 {
		return nil, uerr.ErrInvalidInput
	}
	if req.Email != "" {
		addr, err := mail.ParseAddress(req.Email)
		if err != nil || addr.Address != req.Email || addr.Name != "" {
			return nil, uerr.ErrInvalidInput
		}
	}
	passwordHash, err := modules.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		ID: uuid.New(), Email: req.Email, Nickname: req.Nickname, PasswordHash: passwordHash,
		Role: models.UserRoleUser, IsActive: true, MaxReferrals: 5,
	}
	if err := uc.users.CreateWithInvitation(ctx, user, req.InvitationCode); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *UserUseCase) Authenticate(ctx context.Context, login, password string) (*models.User, error) {
	if len(login) < 1 || len(login) > 254 || len(password) < 1 || len(password) > 72 {
		return nil, uerr.ErrInvalidCredentials
	}
	var user *models.User
	var err error
	if strings.Contains(login, "@") {
		user, err = uc.users.GetByEmail(ctx, login)
	} else {
		user, err = uc.users.GetByNickname(ctx, login)
	}
	if err != nil && !errors.Is(err, uerr.ErrUserNotFound) {
		return nil, err
	}
	return uc.verifyPassword(ctx, login, user, password)
}

func (uc *UserUseCase) verifyPassword(ctx context.Context, login string, user *models.User, password string) (*models.User, error) {
	if len(password) < 1 || len(password) > 72 {
		return nil, uerr.ErrInvalidCredentials
	}
	identity := "alias:" + login
	hash := uc.dummyHash
	if user != nil {
		identity = "account:" + user.ID.String()
		hash = user.PasswordHash
	}
	allowed, err := uc.attempts.AllowLogin(ctx, identity)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, uerr.ErrInvalidCredentials
	}
	if !modules.VerifyPassword(password, hash) || user == nil || !user.IsActive {
		return nil, uerr.ErrInvalidCredentials
	}
	return user, nil
}

type UserTokens struct {
	Access       string    `json:"accessToken"`
	Refresh      string    `json:"refreshToken"`
	SessionID    uuid.UUID `json:"sessionId"`
	AccessUntil  time.Time `json:"accessExpiresAt"`
	RefreshUntil time.Time `json:"refreshExpiresAt"`
}

func makeTokens() (UserTokens, string, string) {
	pair := UserTokens{Access: modules.GenerateToken(modules.AccessPrefix), Refresh: modules.GenerateToken(modules.RefreshPrefix)}
	accessHash, _ := modules.TokenDigest(modules.AccessPrefix, pair.Access)
	refreshHash, _ := modules.TokenDigest(modules.RefreshPrefix, pair.Refresh)
	return pair, accessHash, refreshHash
}

func (uc *UserUseCase) Login(ctx context.Context, login, password string) (UserTokens, *models.User, error) {
	user, err := uc.Authenticate(ctx, login, password)
	if err != nil {
		return UserTokens{}, nil, err
	}
	pair, accessHash, refreshHash := makeTokens()
	now := time.Now().UTC()
	pair.SessionID = uuid.New()
	pair.AccessUntil = now.Add(constants.AccessTTL)
	pair.RefreshUntil = now.Add(constants.RefreshTTL)
	if err := uc.users.UpdateLastLogin(ctx, user.ID); err != nil {
		return UserTokens{}, nil, err
	}
	err = uc.sessions.Create(ctx, models.AccountSession{
		ID: pair.SessionID, UserID: user.ID, AccessHash: accessHash, RefreshHash: refreshHash,
		AccessUntil: pair.AccessUntil, RefreshUntil: pair.RefreshUntil,
	})
	return pair, user, err
}

func (uc *UserUseCase) Refresh(ctx context.Context, refresh string) (UserTokens, error) {
	hash, ok := modules.TokenDigest(modules.RefreshPrefix, refresh)
	if !ok {
		return UserTokens{}, uerr.ErrInvalidToken
	}
	pair, nextAccess, nextRefresh := makeTokens()
	session, err := uc.sessions.Rotate(ctx, hash, nextRefresh, nextAccess, time.Now().UTC().Add(constants.AccessTTL))
	if err != nil {
		return UserTokens{}, err
	}
	pair.SessionID, pair.AccessUntil, pair.RefreshUntil = session.ID, session.AccessUntil, session.RefreshUntil
	return pair, nil
}

func (uc *UserUseCase) Resolve(ctx context.Context, access string) (models.AccountSession, error) {
	hash, ok := modules.TokenDigest(modules.AccessPrefix, access)
	if !ok {
		return models.AccountSession{}, uerr.ErrInvalidToken
	}
	return uc.sessions.ResolveAccess(ctx, hash)
}

func (uc *UserUseCase) Logout(ctx context.Context, access, refresh string) error {
	accessHash, _ := modules.TokenDigest(modules.AccessPrefix, access)
	refreshHash, _ := modules.TokenDigest(modules.RefreshPrefix, refresh)
	return uc.sessions.Revoke(ctx, accessHash, refreshHash)
}

func (uc *UserUseCase) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return uc.sessions.RevokeUser(ctx, userID)
}

func (uc *UserUseCase) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return uc.users.GetByID(ctx, userID)
}
