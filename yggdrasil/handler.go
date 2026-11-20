package yggdrasil

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
)

const maxBodyBytes = 16 << 10

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid game token")
	ErrProfileBound       = errors.New("game token already has a profile")
	ErrNotFound           = errors.New("profile or realm not found")
	realmID               = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

type RequestScope struct {
	Realm    string
	ClientIP netip.Addr
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c Credentials) valid() bool {
	return c.Username != "" && len(c.Username) <= 254 && c.Password != "" && len(c.Password) <= 1024
}

type AuthenticateRequest struct {
	Credentials
	ClientToken *string `json:"clientToken"`
	RequestUser bool    `json:"requestUser"`
}

type TokenRequest struct {
	AccessToken string  `json:"accessToken"`
	ClientToken *string `json:"clientToken"`
}

func (t TokenRequest) valid() bool {
	return t.AccessToken != "" && len(t.AccessToken) <= 512 && validClientToken(t.ClientToken)
}

func validClientToken(token *string) bool {
	return token == nil || len(*token) <= 512
}

type RefreshRequest struct {
	TokenRequest
	SelectedProfile *Profile `json:"selectedProfile"`
	RequestUser     bool     `json:"requestUser"`
}

type JoinRequest struct {
	AccessToken     string `json:"accessToken"`
	SelectedProfile string `json:"selectedProfile"`
	ServerID        string `json:"serverId"`
}

type HasJoinedRequest struct {
	Username string
	ServerID string
	ClientIP netip.Addr
}

type User struct {
	ID         string     `json:"id"`
	Properties []Property `json:"properties"`
}

type Session struct {
	AccessToken     string   `json:"accessToken"`
	ClientToken     string   `json:"clientToken"`
	SelectedProfile *Profile `json:"selectedProfile,omitempty"`
	User            *User    `json:"user,omitempty"`
}

type Authentication struct {
	Session
	AvailableProfiles []Profile `json:"availableProfiles"`
}

// UseCase owns authentication, rate limiting, token state and profile access.
// Every operation scopes lookups to the supplied realm. ServerID on join requests
// is a handshake value and must be checked independently of the configured realm.
type UseCase interface {
	Authenticate(context.Context, RequestScope, AuthenticateRequest) (Authentication, error)
	Refresh(context.Context, RequestScope, RefreshRequest) (Session, error)
	Validate(context.Context, RequestScope, TokenRequest) error
	Invalidate(context.Context, RequestScope, string) error
	Signout(context.Context, RequestScope, Credentials) error
	Join(context.Context, RequestScope, JoinRequest) error
	HasJoined(context.Context, RequestScope, HasJoinedRequest) (ProfileDetails, error)
	Profile(context.Context, RequestScope, string) (ProfileDetails, error)
	ProfilesByName(context.Context, RequestScope, []string) ([]Profile, error)
}

// Mount registers the API under /yggdrasil/:realm. The host application may
// configure Fiber's trusted proxy list to recover the client IP behind ingress.
func Mount(router fiber.Router, uc UseCase, signer *Signer, serverName string) {
	group := router.Group("/yggdrasil/:realm", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		if !realmID.MatchString(c.Params("realm")) {
			return protocolError(c, ErrNotFound)
		}
		return c.Next()
	})
	group.Get("/", func(c *fiber.Ctx) error { return c.JSON(signer.Metadata(serverName)) })
	group.Post("/authserver/authenticate", func(c *fiber.Ctx) error {
		var input AuthenticateRequest
		if !readJSON(c, &input) || !input.Credentials.valid() || !validClientToken(input.ClientToken) {
			return badRequest(c)
		}
		result, err := uc.Authenticate(c.UserContext(), scope(c), input)
		if err != nil {
			return protocolError(c, err)
		}
		if result.AvailableProfiles == nil {
			result.AvailableProfiles = []Profile{}
		}
		if !input.RequestUser {
			result.User = nil
		}
		return c.JSON(result)
	})
	group.Post("/authserver/refresh", func(c *fiber.Ctx) error {
		var input RefreshRequest
		if !readJSON(c, &input) || !input.TokenRequest.valid() ||
			(input.SelectedProfile != nil && !input.SelectedProfile.valid()) {
			return badRequest(c)
		}
		result, err := uc.Refresh(c.UserContext(), scope(c), input)
		if err != nil {
			return protocolError(c, err)
		}
		if !input.RequestUser {
			result.User = nil
		}
		return c.JSON(result)
	})
	group.Post("/authserver/validate", func(c *fiber.Ctx) error {
		var input TokenRequest
		if !readJSON(c, &input) || !input.valid() {
			return badRequest(c)
		}
		return noContent(c, uc.Validate(c.UserContext(), scope(c), input))
	})
	group.Post("/authserver/invalidate", func(c *fiber.Ctx) error {
		var input TokenRequest
		if !readJSON(c, &input) || !input.valid() {
			return badRequest(c)
		}
		err := uc.Invalidate(c.UserContext(), scope(c), input.AccessToken)
		if errors.Is(err, ErrInvalidToken) {
			err = nil
		}
		return noContent(c, err)
	})
	group.Post("/authserver/signout", func(c *fiber.Ctx) error {
		var input Credentials
		if !readJSON(c, &input) || !input.valid() {
			return badRequest(c)
		}
		return noContent(c, uc.Signout(c.UserContext(), scope(c), input))
	})
	group.Post("/sessionserver/session/minecraft/join", func(c *fiber.Ctx) error {
		var input JoinRequest
		if !readJSON(c, &input) || !(TokenRequest{AccessToken: input.AccessToken}).valid() ||
			!validProfileID(input.SelectedProfile) || !validServerID(input.ServerID) {
			return badRequest(c)
		}
		return noContent(c, uc.Join(c.UserContext(), scope(c), input))
	})
	group.Get("/sessionserver/session/minecraft/hasJoined", func(c *fiber.Ctx) error {
		input := HasJoinedRequest{Username: strings.Clone(c.Query("username")), ServerID: strings.Clone(c.Query("serverId"))}
		if !nickname.MatchString(input.Username) || !validServerID(input.ServerID) {
			return c.Status(fiber.StatusNoContent).Send(nil)
		}
		if c.Context().QueryArgs().Has("ip") {
			ip, err := netip.ParseAddr(c.Query("ip"))
			if err != nil || ip.Zone() != "" {
				return c.Status(fiber.StatusNoContent).Send(nil)
			}
			input.ClientIP = ip.Unmap()
		}
		profile, err := uc.HasJoined(c.UserContext(), scope(c), input)
		if err == nil && !strings.EqualFold(profile.Name, input.Username) {
			err = ErrNotFound
		}
		return writeProfile(c, signer, profile, err, true)
	})
	group.Get("/sessionserver/session/minecraft/profile/:uuid", func(c *fiber.Ctx) error {
		id := strings.Clone(c.Params("uuid"))
		if !validProfileID(id) {
			return c.Status(fiber.StatusNoContent).Send(nil)
		}
		unsigned := c.Query("unsigned", "true")
		if unsigned != "true" && unsigned != "false" {
			return badRequest(c)
		}
		profile, err := uc.Profile(c.UserContext(), scope(c), id)
		if err == nil && profile.ID != id {
			err = ErrNotFound
		}
		return writeProfile(c, signer, profile, err, unsigned == "false")
	})
	group.Post("/api/profiles/minecraft", func(c *fiber.Ctx) error {
		var names []string
		if !readJSON(c, &names) || names == nil || len(names) > 100 {
			return badRequest(c)
		}
		for _, name := range names {
			if !nickname.MatchString(name) {
				return badRequest(c)
			}
		}
		profiles, err := uc.ProfilesByName(c.UserContext(), scope(c), names)
		if err != nil {
			return protocolError(c, err)
		}
		if profiles == nil {
			profiles = []Profile{}
		}
		return c.JSON(profiles)
	})
}

func scope(c *fiber.Ctx) RequestScope {
	ip, _ := netip.AddrFromSlice(c.Context().RemoteIP())
	if c.App().Config().EnableTrustedProxyCheck {
		ip, _ = netip.ParseAddr(c.IP())
	}
	return RequestScope{Realm: strings.Clone(c.Params("realm")), ClientIP: ip.Unmap()}
}

func readJSON(c *fiber.Ctx, dst any) bool {
	if c.Get(fiber.HeaderContentEncoding) != "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	body := c.BodyRaw()
	return err == nil && mediaType == fiber.MIMEApplicationJSON && len(body) <= maxBodyBytes &&
		utf8.Valid(body) && json.Unmarshal(body, dst) == nil
}

func validServerID(id string) bool {
	return id != "" && len(id) <= 255 && !strings.ContainsAny(id, "\x00\r\n")
}

func writeProfile(c *fiber.Ctx, signer *Signer, profile ProfileDetails, err error, signed bool) error {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidToken) {
		return c.Status(fiber.StatusNoContent).Send(nil)
	}
	if err != nil {
		return protocolError(c, err)
	}
	result, err := signer.Details(profile, signed, time.Now())
	if err != nil {
		return protocolError(c, err)
	}
	return c.JSON(result)
}

func noContent(c *fiber.Ctx, err error) error {
	if err != nil {
		return protocolError(c, err)
	}
	return c.Status(fiber.StatusNoContent).Send(nil)
}

func badRequest(c *fiber.Ctx) error {
	return respondError(c, fiber.StatusBadRequest, "IllegalArgumentException", "Invalid request.")
}

func protocolError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return respondError(c, fiber.StatusForbidden, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	case errors.Is(err, ErrInvalidToken):
		return respondError(c, fiber.StatusForbidden, "ForbiddenOperationException", "Invalid token.")
	case errors.Is(err, ErrProfileBound):
		return respondError(c, fiber.StatusBadRequest, "IllegalArgumentException", "Access token already has a profile assigned.")
	case errors.Is(err, ErrNotFound):
		return respondError(c, fiber.StatusNotFound, "Not Found", "Resource not found.")
	default:
		return respondError(c, fiber.StatusInternalServerError, "Internal Server Error", "Request could not be completed.")
	}
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(struct {
		Error   string `json:"error"`
		Message string `json:"errorMessage"`
	}{code, message})
}
