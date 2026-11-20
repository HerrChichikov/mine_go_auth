package middleware

import (
	"errors"
	"strings"

	uerr "alpha/internal/objects/errors"
	"alpha/internal/objects/models"
	"alpha/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

const AccessCookie = "__Host-alpha_access"

type sessionKey struct{}

func AccessToken(c *fiber.Ctx) string {
	if header := c.Get(fiber.HeaderAuthorization); header != "" {
		scheme, value, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.ContainsAny(value, " \t") {
			return ""
		}
		return value
	}
	return c.Cookies(AccessCookie)
}

func AuthRequired(users *usecase.UserUseCase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		session, err := users.Resolve(c.UserContext(), AccessToken(c))
		if errors.Is(err, uerr.ErrInvalidToken) {
			return c.Status(401).JSON(fiber.Map{"error": "authentication_required"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal_error"})
		}
		c.Locals(sessionKey{}, session)
		return c.Next()
	}
}

func Session(c *fiber.Ctx) models.AccountSession {
	return c.Locals(sessionKey{}).(models.AccountSession)
}
