package routes

import (
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInboundRoutes(app *fiber.App) {

	inboundController := &controllers.InboundController{}
	api := app.Group("/api/v1/inbound", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(inboundController))

	api.Post("/handle-putaway", inboundController.PutawayByInboundNo)
	api.Post("/putaway-bulk", inboundController.PutawayBulk)

	api.Post("/upload-excel", inboundController.CreateInboundFromExcelFile)
	api.Post("/", inboundController.CreateInbound)
	api.Get("/", inboundController.GetAllListInbound)
	api.Get("/inventory/:inbound_no", inboundController.GetInventoryByInbound)
	api.Put("/:inbound_no", inboundController.UpdateInboundByID)
	api.Get("/:inbound_no", inboundController.GetInboundByID)
	api.Get("/item/:id", inboundController.GetItem)
	api.Delete("/item/:id", inboundController.DeleteItem)
	api.Get("/putaway/sheet/:id", inboundController.GetPutawaySheet)
	api.Post("/complete/:inbound_no", inboundController.HandleComplete)
	api.Post("/open", inboundController.HandleOpen)
	api.Post("/checking", inboundController.HandleChecking)
}
