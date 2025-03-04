package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInboundRoutes(app *fiber.App, inboundController *controllers.InboundController) {
	api := app.Group("/api/v1/inbound", middleware.AuthMiddleware)

	api.Post("/", inboundController.CreateInbound)
	api.Get("/", inboundController.GetAllListInbound)
	api.Get("/:id", inboundController.GetInboundByID)
	api.Put("/:id", inboundController.UpdateInboundByID)

	api.Put("/detail/:id", inboundController.UpdateDetailByID)
	api.Post("/detail/", inboundController.AddNewItemInbound) // Add new item to inbound
	api.Get("/detail/draft", inboundController.GetInboundDetailDraftByUserID)
	api.Delete("/detail/:id", inboundController.DeleteInboundDetail)
	api.Post("/detail/:id", inboundController.AddInboundDetailByID)
}
