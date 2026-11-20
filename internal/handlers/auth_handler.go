package handlers

import (
	"net/http"

	"alpha/internal/interfaces"
	"alpha/internal/objects/dto"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	userUseCase interfaces.UserUseCase
}

func NewAuthHandler(userUseCase interfaces.UserUseCase) *AuthHandler {
	return &AuthHandler{userUseCase: userUseCase}
}

// Register POST /api/auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "некорректный формат запроса",
		})
	}

	user, err := h.userUseCase.Register(c.Context(), req)
	if err != nil {
		status, message := MapErrorToHTTP(err)
		return c.Status(status).JSON(fiber.Map{"error": message})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"user": user,
	})
}

// Login POST /api/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "некорректный формат запроса",
		})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "email и пароль обязательны",
		})
	}

	token, user, err := h.userUseCase.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		status, message := MapErrorToHTTP(err)
		return c.Status(status).JSON(fiber.Map{"error": message})
	}

	// Ставим cookie с токеном сессии
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    token,
		HTTPOnly: true,
		Secure:   true, // В продакшене обязательно
		SameSite: "Strict",
		MaxAge:   7 * 24 * 60 * 60, // 7 дней в секундах
		Path:     "/",
	})

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"user": user,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	token := c.Cookies("session_token")
	if token == "" {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "токен не найден",
		})
	}

	// TODO: Вызвать usecase для удаления сессии из БД
	// h.userUseCase.Logout(c.Context(), token)

	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		MaxAge:   -1,
		Path:     "/",
	})

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "выход выполнен",
	})
}

func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uuid.UUID)
	if !ok {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "неавторизован",
		})
	}
	user, err := h.userUseCase.GetProfile(c.Context(), userID)
	if err != nil {
		status, message := MapErrorToHTTP(err)
		return c.Status(status).JSON(fiber.Map{"error": message})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"user": user,
	})
}
