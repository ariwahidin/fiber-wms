package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupIntegrationRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/integration", middleware.AuthMiddleware)
	integrationController := &controllers.IntegrationController{}
	api.Use(database.InjectDBMiddleware(integrationController))

	api.Post("/inbound/create-inbound", integrationController.CreateInboundFromCsv)
	// api.Post("/", customerController.CreateCustomer)
	// api.Get("/:id", customerController.GetCustomerByID)
	// api.Put("/:id", customerController.UpdateCustomer)
	// api.Delete("/:id", customerController.DeleteCustomer)
}
