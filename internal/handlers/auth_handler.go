package handlers

import (
	"encoding/json"
	"errors"
	"mime"
	"time"

	"alpha/internal/middleware"
	"alpha/internal/objects/dto"
	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	"alpha/internal/repository"
	"alpha/internal/usecase"
	"alpha/yggdrasil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const refreshCookie = "__Host-alpha_refresh"

type AuthHandler struct {
	users    *usecase.UserUseCase
	game     *usecase.GameUseCase
	attempts *repository.AttemptRepository
}

func NewAuthHandler(users *usecase.UserUseCase, game *usecase.GameUseCase, attempts *repository.AttemptRepository) *AuthHandler {
	return &AuthHandler{users, game, attempts}
}

func jsonContent(c *fiber.Ctx) bool {
	kind, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	return err == nil && kind == fiber.MIMEApplicationJSON && c.Get(fiber.HeaderContentEncoding) == ""
}

func readJSON(c *fiber.Ctx, value any) bool {
	return jsonContent(c) && len(c.BodyRaw()) <= 16<<10 && json.Unmarshal(c.BodyRaw(), value) == nil
}

func publicUser(u *models.User) dto.UserPublic {
	return dto.UserPublic{ID: u.ID, Email: u.Email, Nickname: u.Nickname, Role: u.Role, EmailVerifiedAt: u.EmailVerifiedAt, CreatedAt: u.CreatedAt}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if !readJSON(c, &req) {
		return apiError(c, 400, "invalid_request")
	}
	allowed, err := h.attempts.AllowIP(c.UserContext(), "register", c.IP())
	if err != nil {
		return internalError(c)
	}
	if !allowed {
		return apiError(c, 429, "try_later")
	}
	user, err := h.users.Register(c.UserContext(), req)
	switch {
	case errors.Is(err, uerr.ErrInvalidInput), errors.Is(err, uerr.ErrInvitationNotFound), errors.Is(err, uerr.ErrUserExists):
		return apiError(c, 400, "registration_rejected")
	case err != nil:
		return internalError(c)
	}
	return c.Status(201).JSON(fiber.Map{"user": publicUser(user)})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Client   string `json:"client"`
	}
	if !readJSON(c, &req) || (req.Client != "" && req.Client != "launcher") {
		return apiError(c, 400, "invalid_request")
	}
	if req.Login == "" {
		req.Login = req.Email
	}
	if len(req.Login) > 254 || len(req.Password) > 72 {
		return apiError(c, 401, "invalid_credentials")
	}
	allowed, err := h.attempts.AllowIP(c.UserContext(), "login", c.IP())
	if err != nil {
		return internalError(c)
	}
	if !allowed {
		return apiError(c, 429, "try_later")
	}
	pair, user, err := h.users.Login(c.UserContext(), req.Login, req.Password)
	if errors.Is(err, uerr.ErrRateLimited) {
		return apiError(c, 429, "try_later")
	}
	if errors.Is(err, uerr.ErrInvalidCredentials) {
		return apiError(c, 401, "invalid_credentials")
	}
	if err != nil {
		return internalError(c)
	}
	if req.Client == "launcher" {
		return c.JSON(struct {
			usecase.UserTokens
			User dto.UserPublic `json:"user"`
		}{pair, publicUser(user)})
	}
	setCookies(c, pair)
	return c.JSON(fiber.Map{"user": publicUser(user)})
}

type sessionRequest struct {
	Refresh string `json:"refreshToken"`
	Client  string `json:"client"`
}

func parseSessionRequest(c *fiber.Ctx, req *sessionRequest) bool {
	if !jsonContent(c) {
		return false
	}
	if len(c.BodyRaw()) == 0 {
		req.Refresh = c.Cookies(refreshCookie)
		return true
	}
	if !readJSON(c, req) || (req.Client != "" && req.Client != "launcher") {
		return false
	}
	if req.Client != "launcher" {
		req.Refresh = c.Cookies(refreshCookie)
	}
	return true
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req sessionRequest
	if !parseSessionRequest(c, &req) {
		return apiError(c, 400, "invalid_request")
	}
	pair, err := h.users.Refresh(c.UserContext(), req.Refresh)
	if errors.Is(err, uerr.ErrRateLimited) {
		return apiError(c, 429, "try_later")
	}
	if errors.Is(err, uerr.ErrInvalidToken) {
		if req.Client != "launcher" {
			clearCookies(c)
		}
		return apiError(c, 401, "invalid_token")
	}
	if err != nil {
		return internalError(c)
	}
	if req.Client == "launcher" {
		return c.JSON(pair)
	}
	setCookies(c, pair)
	return c.Status(204).Send(nil)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req sessionRequest
	if !parseSessionRequest(c, &req) {
		return apiError(c, 400, "invalid_request")
	}
	if err := h.users.Logout(c.UserContext(), middleware.AccessToken(c), req.Refresh); err != nil {
		return internalError(c)
	}
	clearCookies(c)
	return c.Status(204).Send(nil)
}

func (h *AuthHandler) LogoutAll(c *fiber.Ctx) error {
	if c.Get(fiber.HeaderAuthorization) == "" && !jsonContent(c) {
		return apiError(c, 400, "invalid_request")
	}
	if err := h.users.LogoutAll(c.UserContext(), middleware.Session(c).UserID); err != nil {
		return internalError(c)
	}
	clearCookies(c)
	return c.Status(204).Send(nil)
}

func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	user, err := h.users.GetProfile(c.UserContext(), middleware.Session(c).UserID)
	if errors.Is(err, uerr.ErrUserNotFound) {
		return apiError(c, 401, "authentication_required")
	}
	if err != nil {
		return internalError(c)
	}
	return c.JSON(fiber.Map{"user": publicUser(user)})
}

func (h *AuthHandler) GameProfiles(c *fiber.Ctx) error {
	profiles, err := h.game.Profiles(c.UserContext(), middleware.Session(c).UserID)
	if err != nil {
		return internalError(c)
	}
	return c.JSON(fiber.Map{"profiles": profiles})
}

func (h *AuthHandler) Launch(c *fiber.Ctx) error {
	var req struct {
		ProfileID uuid.UUID `json:"profileId"`
	}
	if !readJSON(c, &req) || req.ProfileID == uuid.Nil {
		return apiError(c, 400, "invalid_request")
	}
	session, err := h.game.Launch(c.UserContext(), middleware.Session(c), req.ProfileID)
	if errors.Is(err, uerr.ErrRateLimited) {
		return apiError(c, 429, "try_later")
	}
	if errors.Is(err, yggdrasil.ErrNotFound) {
		return apiError(c, 404, "profile_not_found")
	}
	if err != nil {
		return internalError(c)
	}
	return c.Status(201).JSON(session)
}

func (h *AuthHandler) SetSkin(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	var req struct {
		URL  string `json:"url"`
		Slim bool   `json:"slim"`
	}
	if err != nil || !readJSON(c, &req) {
		return apiError(c, 400, "invalid_request")
	}
	p, err := h.game.SetSkin(c.UserContext(), middleware.Session(c).UserID, id, req.URL, req.Slim)
	if errors.Is(err, yggdrasil.ErrNotFound) {
		return apiError(c, 404, "profile_not_found")
	}
	if errors.Is(err, uerr.ErrInvalidInput) {
		return apiError(c, 400, "invalid_request")
	}
	if err != nil {
		return internalError(c)
	}
	return c.JSON(p)
}

func (h *AuthHandler) CreateGameProfile(c *fiber.Ctx) error {
	var req struct {
		UserID   uuid.UUID `json:"userId"`
		Realm    string    `json:"realm"`
		Nickname string    `json:"nickname"`
	}
	if !readJSON(c, &req) {
		return apiError(c, 400, "invalid_request")
	}
	p, err := h.game.CreateProfile(c.UserContext(), req.UserID, req.Realm, req.Nickname)
	if errors.Is(err, yggdrasil.ErrNotFound) {
		return apiError(c, 404, "user_not_found")
	}
	if errors.Is(err, uerr.ErrInvalidInput) {
		return apiError(c, 400, "invalid_request")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return apiError(c, 409, "nickname_unavailable")
	}
	if err != nil {
		return internalError(c)
	}
	return c.JSON(p)
}

func setCookies(c *fiber.Ctx, pair usecase.UserTokens) {
	setCookie(c, middleware.AccessCookie, pair.Access, "/", int(time.Until(pair.AccessUntil).Seconds()))
	setCookie(c, refreshCookie, pair.Refresh, "/", int(time.Until(pair.RefreshUntil).Seconds()))
}

func clearCookies(c *fiber.Ctx) {
	setCookie(c, middleware.AccessCookie, "", "/", -1)
	setCookie(c, refreshCookie, "", "/", -1)
	setCookie(c, "session_token", "", "/", -1)
}

func setCookie(c *fiber.Ctx, name, value, path string, maxAge int) {
	c.Cookie(&fiber.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, HTTPOnly: true, Secure: true, SameSite: "Strict"})
}

func apiError(c *fiber.Ctx, status int, code string) error {
	return c.Status(status).JSON(fiber.Map{"error": code})
}
func internalError(c *fiber.Ctx) error { return apiError(c, 500, "internal_error") }
