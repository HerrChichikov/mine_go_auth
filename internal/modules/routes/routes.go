package routes

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"alpha/constants"
	"alpha/internal/handlers"
	"alpha/internal/middleware"
	"alpha/internal/usecase"
	"alpha/yggdrasil"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, auth *handlers.AuthHandler, users *usecase.UserUseCase, game *usecase.GameUseCase, signer *yggdrasil.Signer, provisionToken string) {
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("Alpha Auth") })
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("OK") })
	app.Use(func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), constants.RequestTimeout)
		defer cancel()
		c.SetUserContext(ctx)
		return c.Next()
	})
	account := app.Group("/api/auth", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Next()
	})
	account.Post("/register", auth.Register)
	account.Post("/login", auth.Login)
	account.Post("/refresh", auth.Refresh)
	account.Post("/logout", auth.Logout)
	protected := account.Group("", middleware.AuthRequired(users))
	protected.Get("/profile", auth.GetProfile)
	protected.Post("/logout-all", auth.LogoutAll)
	protected.Get("/game-profiles", auth.GameProfiles)
	protected.Put("/game-profiles/:id/skin", auth.SetSkin)
	protected.Post("/game-token", auth.Launch)

	if provisionToken != "" {
		expected := sha256.Sum256([]byte("Bearer " + provisionToken))
		app.Post("/internal/game-profiles", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "no-store")
			header := c.Get(fiber.HeaderAuthorization)
			if !strings.HasPrefix(header, "Bearer ") || len(header) > 520 {
				return c.Status(401).JSON(fiber.Map{"error": "authentication_required"})
			}
			actual := sha256.Sum256([]byte(header))
			if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
				return c.Status(401).JSON(fiber.Map{"error": "authentication_required"})
			}
			return auth.CreateGameProfile(c)
		})
	}
	yggdrasil.Mount(app, game, signer, "Alpha")
}
