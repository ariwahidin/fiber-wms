package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupTransporterRoutes(app *fiber.App, transporterController *controllers.TransporterController) {
	api := app.Group("/api/v1/transporters", middleware.AuthMiddleware)
	api.Post("/", transporterController.CreateTransporter)
	api.Get("/", transporterController.GetAllTransporter)
	// api.Get("/:id", supplierController.GetSupplierByID)
	// api.Put("/:id", supplierController.UpdateSupplier)
	// api.Delete("/:id", supplierController.DeleteSupplier)
}
