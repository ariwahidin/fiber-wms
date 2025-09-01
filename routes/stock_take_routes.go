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
	api.Get("/", stockTakeController.GetAllStockTake)
	api.Get("/:code", stockTakeController.GetStockTakeDetail)
	api.Post("/generate", stockTakeController.GenerateDataStockTake)
}
