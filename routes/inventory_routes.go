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
	api.Get("/excel", inventoryController.ExportExcel)
	api.Post("/rf/pallet", inventoryController.GetInventoryByPalletAndLocation)
	api.Post("/rf/move", inventoryController.MoveItem)
}
