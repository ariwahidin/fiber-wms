package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupOutboundRoutes(app *fiber.App, db *gorm.DB) {
	outboundController := controllers.NewOutboundController(db)
	// inboundMidleware := middleware.NewAuthMiddleware(db)
	api := app.Group(
		"/api/v1/outbound",
		middleware.AuthMiddleware,
		// inboundMidleware.CheckPermission("create_inbound"),
	)

	api.Get("/", outboundController.GetOutboundList)
	api.Put("/:id", outboundController.SaveOutbound)
	api.Get("/draft", outboundController.GetOutboundDraft)
	api.Get("/create", outboundController.CreateOutbound)
	api.Post("/item", outboundController.CreateItemOutbound)
	api.Get("/:id", outboundController.GetOutboundByID)
	api.Delete("/item/:id", outboundController.DeleteItemOutbound)

	// api.Get("/", inboundController.GetAllListInbound)
	// api.Get("/:id", inboundController.GetInboundByID)
	// api.Put("/:id", inboundController.UpdateInboundByID)
	// api.Put("/detail/:id", inboundController.UpdateDetailByID)
	// api.Post("/detail/", inboundController.AddNewItemInbound)
	// api.Get("/detail/draft", inboundController.GetInboundDetailDraftByUserID)
	// api.Delete("/detail/:id", inboundController.DeleteInboundDetail)
	// api.Post("/complete/:id", inboundController.ProcessingInboundComplete)
}
