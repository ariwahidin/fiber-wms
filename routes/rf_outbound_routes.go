package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupRfOutboundRoutes(app *fiber.App, db *gorm.DB) {

	rfOutboundController := controllers.NewRfOutboundController(db)
	api := app.Group("/api/v1/rf/outbound", middleware.AuthMiddleware)

	api.Get("/scan/form", rfOutboundController.ScanForm)
	api.Get("/scan/list", rfOutboundController.GetAllListOutboundPicking)
	api.Get("/scan/list/:id", rfOutboundController.GetOutboundByOutboundID)
	// api.Post("/scan/post/", rfOutboundController.PostScanForm)
}
