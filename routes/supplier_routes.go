package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupSupplierRoutes(app *fiber.App, supplierController *controllers.SupplierController) {
	api := app.Group("/api/v1/suppliers", middleware.AuthMiddleware)
	api.Post("/", supplierController.CreateSupplier)
	api.Get("/", supplierController.GetAllSuppliers)
	api.Get("/:id", supplierController.GetSupplierByID)
	api.Put("/:id", supplierController.UpdateSupplier)
	api.Delete("/:id", supplierController.DeleteSupplier)
}
