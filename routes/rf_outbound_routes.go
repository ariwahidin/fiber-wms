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
	api.Get("/scan/list", rfOutboundController.GetAllListOutboundOpen)
	api.Get("/scan/list/:id", rfOutboundController.GetOutboundByOutboundID)

	// api.Get("/list", rfOutboundController.GetAllListInbound)
	// api.Get("/:id", rfInboundController.GetInboundByInboundID)
	// api.Post("/:id", rfInboundController.PostInboundByInboundID)
	// api.Get("/detail/scanned/:id", rfInboundController.GetInboundDetailScanned)
	// api.Delete("/detail/scanned/:id", rfInboundController.DeleteBarcode)
	// api.Get("/detail/barcode/:id/:detail_id", rfInboundController.GetInboundBarcodeDetail)
	// api.Post("/confirm/putaway", rfInboundController.ConfirmPutaway)
	// api.Post("/barcode/delete", rfInboundController.DeleteBarcode)
}
