package routes

import (
	"alpha/internal/handlers"
	"alpha/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, authHandler *handlers.AuthHandler) {
	// Публичные роуты
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Добро пожаловать в API!")
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// API роуты
	api := app.Group("/api")

	// Auth роуты (публичные)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)

	// Защищенные роуты (требуют аутентификации)
	authProtected := auth.Group("", middleware.AuthRequired())
	authProtected.Get("/profile", authHandler.GetProfile)
}
