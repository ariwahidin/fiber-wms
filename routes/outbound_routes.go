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

	api.Post("/picking/:id", outboundController.PickingOutbound)
	api.Get("/picking/sheet/:id", outboundController.GetPickingSheet)
	api.Post("/picking/complete/:id", outboundController.PickingComplete)
}
