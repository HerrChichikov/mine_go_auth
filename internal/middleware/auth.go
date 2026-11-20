package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// TODO: Здесь нужна зависимость от sessionRepo
// Пока заглушка — потом добавим

func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Получаем токен из cookie
		token := c.Cookies("session_token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "требуется авторизация",
			})
		}

		// TODO: Проверить токен в БД
		// session, err := sessionRepo.GetByTokenHash(ctx, hash(token))
		// if err != nil {
		//     return c.Status(fiber.StatusUnauthorized).JSON(...)
		// }

		// TODO: Положить userID в контекст
		// c.Locals("userID", session.UserID)

		// Пока заглушка — пропускаем
		return c.Next()
	}
}
