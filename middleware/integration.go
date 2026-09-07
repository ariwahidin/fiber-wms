package middleware

import (
	"crypto/subtle"
	"fiber-app/config"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func IntegrationAuth(ctx *fiber.Ctx) error {
	authHeader := ctx.Get("Authorization")

	if authHeader == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Missing Authorization header",
		})
	}

	parts := strings.Fields(authHeader)

	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid Authorization header format",
		})
	}

	token := parts[1]

	// Pastikan integration token sudah dikonfigurasi
	if config.N8NIntegrationToken == "" {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Integration authentication is not configured",
		})
	}

	// Constant-time comparison
	if subtle.ConstantTimeCompare(
		[]byte(token),
		[]byte(config.N8NIntegrationToken),
	) != 1 {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid integration token",
		})
	}

	// Tandai request sebagai integration request
	ctx.Locals("integration", true)

	return ctx.Next()
}
