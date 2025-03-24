package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInventoryRoutes(app *fiber.App, inventoryController *controllers.InventoryController) {
	api := app.Group("/api/v1/inventory", middleware.AuthMiddleware)

	// api.Post("/", customerController.CreateCustomer)
	api.Get("/", inventoryController.GetInventory)
	api.Get("/excel", inventoryController.ExportExcel)
	// api.Get("/:id", customerController.GetCustomerByID)
	// api.Put("/:id", customerController.UpdateCustomer)
	// api.Delete("/:id", customerController.DeleteCustomer)
}
