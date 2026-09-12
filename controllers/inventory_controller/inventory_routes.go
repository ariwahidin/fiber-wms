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

	api.Get("/available/summary", inventoryController.GetAvailableSummaryCards)
	api.Get("/available/grouped", inventoryController.GetAvailableGrouped)
	api.Get("/available/detail", inventoryController.GetAvailableInventoryDetail)
	api.Get(
		"/available/detail/:inventory_number/serials",
		inventoryController.GetInventorySerialDetail,
	)
	api.Get("/available/filter-options", inventoryController.GetFilterOptions)
	api.Get("/grouped-by-item", inventoryController.GetInventoryGroupedByItem)
	api.Get("/cartons", inventoryController.GetCartonInventory)
	api.Get("/location", inventoryController.GetItemByLocation)
	api.Get("/available/grouped", inventoryController.GetAllInventoryAvailableGrouped)
	api.Get("/movements", inventoryController.GetInventoryMovements)
	api.Get("/policy", inventoryController.GetInventoryPolicy)
	api.Get("/excel", inventoryController.ExportExcel)
	api.Post("/bulk-update-lot-excel", inventoryController.BulkUpdateLotNumberFromExcel)
	api.Post("/rf/pallet", inventoryController.GetInventoryByPalletAndLocation)
	api.Post("/rf/move", inventoryController.MoveItem)
	api.Post("/change", inventoryController.ChangeStatusInventory)
	api.Post("/transfer", inventoryController.TransferInventory)

	api.Get(
		"/internal-transfer/inventories",
		inventoryController.GetInternalTransferInventories,
	)

	api.Get(
		"/internal-transfer/serials",
		inventoryController.GetInternalTransferSerials,
	)

	api.Post(
		"/internal-transfer",
		inventoryController.TransferInventoryInternal,
	)

	// api.Get("/inventory/serials", inventoryController.GetInventorySerials)
	// api.Post("/inventory/transfer-v2", inventoryController.TransferInventoryV2)

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
