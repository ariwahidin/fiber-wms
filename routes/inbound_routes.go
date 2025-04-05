package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupInboundRoutes(app *fiber.App, inboundController *controllers.InboundController, db *gorm.DB) {

	inboundMidleware := middleware.NewAuthMiddleware(db)

	api := app.Group("/api/v1/inbound", middleware.AuthMiddleware, inboundMidleware.CheckPermission("create_inbound"))

	api.Get("/create", inboundController.PreapareInbound)

	api.Post("/", inboundController.CreateInbound)
	api.Get("/", inboundController.GetAllListInbound)
	api.Get("/:id", inboundController.GetInboundByID)
	api.Put("/:id", inboundController.UpdateInboundByID)
	api.Put("/detail/:id", inboundController.UpdateDetailByID)
	api.Post("/detail/", inboundController.AddNewItemInbound)
	api.Get("/detail/draft", inboundController.GetInboundDetailDraftByUserID)
	api.Delete("/detail/:id", inboundController.DeleteInboundDetail)
	api.Post("/complete/:id", inboundController.ProcessingInboundComplete)
}
