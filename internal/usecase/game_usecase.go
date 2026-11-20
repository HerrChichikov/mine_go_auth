package usecase

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"alpha/constants"
	"alpha/internal/modules"
	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	"alpha/internal/repository"
	"alpha/yggdrasil"

	"github.com/google/uuid"
)

var (
	realmPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	nickPattern  = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
)

type GameUseCase struct {
	users    *UserUseCase
	game     *repository.GameRepository
	attempts *repository.AttemptRepository
	signer   *yggdrasil.Signer
}

func NewGameUseCase(users *UserUseCase, game *repository.GameRepository, attempts *repository.AttemptRepository, signer *yggdrasil.Signer) *GameUseCase {
	return &GameUseCase{users, game, attempts, signer}
}

func (uc *GameUseCase) Profiles(ctx context.Context, userID uuid.UUID) ([]repository.GameProfile, error) {
	return uc.game.Profiles(ctx, userID)
}

func (uc *GameUseCase) CreateProfile(ctx context.Context, owner uuid.UUID, realm, nickname string) (repository.GameProfile, error) {
	if owner == uuid.Nil || !realmPattern.MatchString(realm) || !nickPattern.MatchString(nickname) {
		return repository.GameProfile{}, uerr.ErrInvalidInput
	}
	return uc.game.CreateProfile(ctx, owner, realm, nickname)
}

func (uc *GameUseCase) SetSkin(ctx context.Context, owner, id uuid.UUID, url string, slim bool) (repository.GameProfile, error) {
	if url != "" && !uc.signer.AllowsSkin(url) {
		return repository.GameProfile{}, uerr.ErrInvalidInput
	}
	return uc.game.SetSkin(ctx, owner, id, url, url != "" && slim)
}

type LaunchSession struct {
	AccessToken  string                 `json:"accessToken"`
	Profile      repository.GameProfile `json:"profile"`
	InjectorPath string                 `json:"injectorPath"`
}

func (uc *GameUseCase) Launch(ctx context.Context, session models.AccountSession, profileID uuid.UUID) (LaunchSession, error) {
	token := modules.GenerateToken(modules.GamePrefix)
	hash, _ := modules.TokenDigest(modules.GamePrefix, token)
	profile, err := uc.game.Issue(ctx, session.UserID, &session.ID, &profileID, "", hash, modules.ClientToken(), time.Now().UTC().Add(constants.GameTTL))
	if err != nil {
		return LaunchSession{}, err
	}
	return LaunchSession{token, profile, "/yggdrasil/" + profile.Realm + "/"}, nil
}

func (uc *GameUseCase) authenticate(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.Credentials) (*models.User, error) {
	allowed, err := uc.attempts.AllowIP(ctx, "login", scope.ClientIP.String())
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, yggdrasil.ErrInvalidCredentials
	}
	var user *models.User
	if strings.Contains(req.Username, "@") {
		user, err = uc.users.Authenticate(ctx, req.Username, req.Password)
	} else {
		owner, ep := uc.game.ProfileOwner(ctx, scope.Realm, req.Username)
		if ep == nil {
			user, ep = uc.users.GetProfile(ctx, owner)
		}
		if ep != nil && !errors.Is(ep, yggdrasil.ErrNotFound) && !errors.Is(ep, uerr.ErrUserNotFound) {
			return nil, ep
		}
		user, err = uc.users.verifyPassword(ctx, req.Username, user, req.Password)
	}
	if errors.Is(err, uerr.ErrInvalidCredentials) {
		return nil, yggdrasil.ErrInvalidCredentials
	}
	return user, err
}

func (uc *GameUseCase) Authenticate(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.AuthenticateRequest) (yggdrasil.Authentication, error) {
	user, err := uc.authenticate(ctx, scope, req.Credentials)
	if err != nil {
		return yggdrasil.Authentication{}, err
	}
	var clientToken string
	if req.ClientToken == nil {
		clientToken = modules.ClientToken()
	} else {
		clientToken = *req.ClientToken
	}
	token := modules.GenerateToken(modules.GamePrefix)
	hash, _ := modules.TokenDigest(modules.GamePrefix, token)
	profile, err := uc.game.Issue(ctx, user.ID, nil, nil, scope.Realm, hash, clientToken, time.Now().UTC().Add(constants.GameTTL))
	if errors.Is(err, yggdrasil.ErrNotFound) || errors.Is(err, uerr.ErrRateLimited) {
		err = yggdrasil.ErrInvalidCredentials
	}
	if err != nil {
		return yggdrasil.Authentication{}, err
	}
	wire := profile.Wire().Profile
	return yggdrasil.Authentication{
		Session:           gameSession(token, clientToken, wire, user.ID, req.RequestUser),
		AvailableProfiles: []yggdrasil.Profile{wire},
	}, nil
}

func gameSession(token, clientToken string, profile yggdrasil.Profile, owner uuid.UUID, includeUser bool) yggdrasil.Session {
	result := yggdrasil.Session{AccessToken: token, ClientToken: clientToken, SelectedProfile: &profile}
	if includeUser {
		result.User = &yggdrasil.User{ID: strings.ReplaceAll(owner.String(), "-", ""), Properties: []yggdrasil.Property{}}
	}
	return result
}

func (uc *GameUseCase) Refresh(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.RefreshRequest) (yggdrasil.Session, error) {
	hash, ok := modules.TokenDigest(modules.GamePrefix, req.AccessToken)
	if !ok {
		return yggdrasil.Session{}, yggdrasil.ErrInvalidToken
	}
	token := modules.GenerateToken(modules.GamePrefix)
	nextHash, _ := modules.TokenDigest(modules.GamePrefix, token)
	profile, owner, binding, err := uc.game.Rotate(ctx, scope.Realm, hash, nextHash, req.ClientToken, req.SelectedProfile != nil, time.Now().UTC().Add(constants.GameTTL))
	if err != nil {
		return yggdrasil.Session{}, err
	}
	return gameSession(token, binding, profile.Wire().Profile, owner, req.RequestUser), nil
}

func (uc *GameUseCase) Validate(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.TokenRequest) error {
	hash, ok := modules.TokenDigest(modules.GamePrefix, req.AccessToken)
	if !ok {
		return yggdrasil.ErrInvalidToken
	}
	return uc.game.Validate(ctx, scope.Realm, hash, req.ClientToken)
}

func (uc *GameUseCase) Invalidate(ctx context.Context, scope yggdrasil.RequestScope, token string) error {
	hash, ok := modules.TokenDigest(modules.GamePrefix, token)
	if !ok {
		return nil
	}
	return uc.game.Invalidate(ctx, scope.Realm, hash)
}

func (uc *GameUseCase) Signout(ctx context.Context, scope yggdrasil.RequestScope, credentials yggdrasil.Credentials) error {
	user, err := uc.authenticate(ctx, scope, credentials)
	if err != nil {
		return err
	}
	return uc.game.Signout(ctx, scope.Realm, user.ID)
}

func (uc *GameUseCase) Join(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.JoinRequest) error {
	hash, ok := modules.TokenDigest(modules.GamePrefix, req.AccessToken)
	id, err := uuid.Parse(req.SelectedProfile)
	if !ok || err != nil {
		return yggdrasil.ErrInvalidToken
	}
	return uc.game.Join(ctx, scope.Realm, id, hash, req.ServerID, scope.ClientIP, time.Now().UTC().Add(constants.JoinTTL))
}

func (uc *GameUseCase) HasJoined(ctx context.Context, scope yggdrasil.RequestScope, req yggdrasil.HasJoinedRequest) (yggdrasil.ProfileDetails, error) {
	profile, err := uc.game.HasJoined(ctx, scope.Realm, req.Username, req.ServerID, req.ClientIP)
	return profile.Wire(), err
}

func (uc *GameUseCase) Profile(ctx context.Context, scope yggdrasil.RequestScope, raw string) (yggdrasil.ProfileDetails, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return yggdrasil.ProfileDetails{}, yggdrasil.ErrNotFound
	}
	profile, err := uc.game.Profile(ctx, scope.Realm, id)
	return profile.Wire(), err
}

func (uc *GameUseCase) ProfilesByName(ctx context.Context, scope yggdrasil.RequestScope, names []string) ([]yggdrasil.Profile, error) {
	for i := range names {
		names[i] = strings.ToLower(names[i])
	}
	return uc.game.ProfilesByName(ctx, scope.Realm, names)
}
