package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupIntegrationRoutes(app *fiber.App) {

	api := app.Group(config.MAIN_ROUTES + "/integration")

	integrationController := &controllers.IntegrationController{}

	// n8n integration
	api.Post(
		"/n8n/test",
		middleware.IntegrationAuth,
		database.InjectDBMiddlewareFromEnv(integrationController),
		integrationController.TestN8N,
	)

	// Existing SAP integration
	api.Post(
		"/inbound/create-inbound",
		middleware.AuthMiddleware,
		database.InjectDBMiddleware(integrationController),
		integrationController.CreateInboundFromCsv,
	)

	// route integration lainnya...
}
