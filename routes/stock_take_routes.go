package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupStockTakeRoutes(app *fiber.App, db *gorm.DB) {
	stockTakeController := controllers.NewStockTakeController(db)
	// inboundMidleware := middleware.NewAuthMiddleware(db)
	api := app.Group(
		"/api/v1/stock-take",
		middleware.AuthMiddleware,
		// inboundMidleware.CheckPermission("create_inbound"),
	)

	api.Get("/progress/:code", stockTakeController.GetProgressStockTakeByCode)
	api.Post("/scan", stockTakeController.ScanStockTake)
	api.Get("/barcode/:code", stockTakeController.GetStockTakeBarcodeByCode)
	api.Get("/", stockTakeController.GetAllStockTake)
	api.Get("/:code", stockTakeController.GetStockTakeDetail)
	api.Post("/generate", stockTakeController.GenerateDataStockTake)
}
