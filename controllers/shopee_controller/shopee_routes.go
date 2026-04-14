package shopee_controller

import (
	"fiber-app/database"

	"github.com/gofiber/fiber/v2"
)

func SetupShopeeRoutes(app *fiber.App) {
	shopeeController := &ShopeeController{}
	app.Use(database.InjectDBMiddlewareFromEnv(shopeeController))

	// Webhook tidak pakai AuthMiddleware — Shopee yang hit endpoint ini
	webhook := app.Group("/webhook")
	webhook.Post("/shopee", shopeeController.WebhookHandler)
}
