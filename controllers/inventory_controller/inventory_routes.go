package inventory_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInventoryRoutes(app *fiber.App) {
	inventoryController := &InventoryController{}
	adjustmentController := &AdjustmentController{}
	api := app.Group(config.MAIN_ROUTES+"/inventory", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(inventoryController))
	api.Use(database.InjectDBMiddleware(adjustmentController))

	api.Get("/", inventoryController.GetInventory)
	api.Get("/all", inventoryController.GetAllInventoryAvailable)
	api.Get("/grouped", inventoryController.GetGroupedInventory)
	api.Get("/location", inventoryController.GetItemByLocation)
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

	api.Get("/adjustments/reason-codes", adjustmentController.GetReasonCodes)
	api.Get("/adjustments", adjustmentController.GetAll)
	api.Get("/adjustments/:id", adjustmentController.GetByID)
	api.Post("/adjustments", adjustmentController.Create)
	api.Post("/adjustments/:id/approve", adjustmentController.Approve)
	api.Post("/adjustments/:id/reject", adjustmentController.Reject)
}
