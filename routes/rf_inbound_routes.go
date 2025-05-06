package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRfInboundRoutes(app *fiber.App, rfInboundController *controllers.RfInboundController) {
	api := app.Group("/api/v1/rf/inbound", middleware.AuthMiddleware)
	// api.Get("/list", rfInboundController.GetAllListInbound)
	api.Get("/:id", rfInboundController.GetInboundByInboundID)
	api.Post("/:id", rfInboundController.PostInboundByInboundID)
	api.Get("/detail/scanned/:id", rfInboundController.GetInboundDetailScanned)
	// api.Delete("/detail/scanned/:id", rfInboundController.DeleteBarcode)
	api.Get("/detail/barcode/:id/:detail_id", rfInboundController.GetInboundBarcodeDetail)
	api.Post("/confirm/putaway", rfInboundController.ConfirmPutaway)
	api.Post("/barcode/delete", rfInboundController.DeleteBarcode)
	api.Get("/scan/pallet/:id", rfInboundController.ScanPallet)
	api.Post("/scan/pallet/putaway", rfInboundController.PutawayPallet)
}
