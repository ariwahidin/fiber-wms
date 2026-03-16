package transporter_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupTransporterRoutes(app *fiber.App) {

	transporterController := &TransporterController{}
	api := app.Group(config.MAIN_ROUTES+"/transporters", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(transporterController))

	api.Get("/export", transporterController.ExportTransporters)
	api.Post("/", transporterController.CreateTransporter)
	api.Get("/", transporterController.GetAllTransporter)
	api.Put("/:id", transporterController.UpdateTransporter)
}
