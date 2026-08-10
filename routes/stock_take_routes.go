package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupStockTakeRoutes(app *fiber.App) {
	stockTakeController := &controllers.StockTakeController{}
	api := app.Group(
		config.MAIN_ROUTES+"/stock-take",
		middleware.AuthMiddleware,
	)

	api.Use(database.InjectDBMiddleware(stockTakeController))

	api.Get("/locations", stockTakeController.LoadLocations)
	api.Post("/stock-card", stockTakeController.GetCardStockTake)
	api.Get("/progress/:code", stockTakeController.GetProgressStockTakeByCode)
	api.Post("/scan", stockTakeController.ScanStockTake)
	api.Get("/barcode/:code", stockTakeController.GetStockTakeBarcodeByCode)
	api.Delete("/barcode/delete/:id", stockTakeController.DeleteStockTakeBarcode)
	api.Get("/", stockTakeController.GetAllStockTake)
	api.Get("/stats", stockTakeController.GetStockTakeStats)
	api.Get("/summary", stockTakeController.GetAllStockTakeSummary)
	api.Get("/:code", stockTakeController.GetStockTakeDetail)
	api.Get("/:code/print", stockTakeController.GetStockTakePrintDetail)
	api.Post("/generate", stockTakeController.GenerateDataStockTake)
	api.Delete("/:code", stockTakeController.DeleteStockTake)
	api.Post("/:code/close", stockTakeController.CloseStockTake)
	api.Post("/:code/cancel", stockTakeController.CancelStockTake)
	api.Get("/progress-sku/:code", stockTakeController.GetProgressBySKU)
	api.Get("/progress-category/:code", stockTakeController.GetProgressByCategory)
	api.Get("/progress-division/:code", stockTakeController.GetProgressByDivision)
	api.Get("/progress-location/:code", stockTakeController.GetProgressByLocation)
	api.Get("/progress-pic/:code", stockTakeController.GetProgressByPic)
	api.Get("/export-division/:code", stockTakeController.ExportProgressByDivision)
	api.Get("/export-category/:code", stockTakeController.ExportProgressByCategory)
}
