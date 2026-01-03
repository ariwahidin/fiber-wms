package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInventoryRoutes(app *fiber.App) {
	inventoryController := &controllers.InventoryController{}
	api := app.Group(config.MAIN_ROUTES+"/inventory", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(inventoryController))

	api.Get("/", inventoryController.GetInventory)
	api.Get("/all", inventoryController.GetAllInventoryAvailable)
	api.Get("/available/grouped", inventoryController.GetAllInventoryAvailableGrouped)
	api.Get("/movements", inventoryController.GetInventoryMovements)
	api.Get("/policy", inventoryController.GetInventoryPolicy)
	api.Get("/excel", inventoryController.ExportExcel)
	api.Post("/rf/pallet", inventoryController.GetInventoryByPalletAndLocation)
	api.Post("/rf/move", inventoryController.MoveItem)
	api.Post("/change", inventoryController.ChangeStatusInventory)
	api.Post("/transfer", inventoryController.TransferInventory)

	api.Post("/policies", inventoryController.CreateInvetoryPolicy)
	api.Get("/policies", inventoryController.GetAllInventoryPolicy)
	api.Put("/policies/:id", inventoryController.UpdateInventoryPolicy)
	api.Delete("/policies/:id", inventoryController.HardDelete)
}
