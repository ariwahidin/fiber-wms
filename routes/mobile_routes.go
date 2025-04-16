package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMobileInboundRoutes(app *fiber.App, mobileInboundController *controllers.MobileInboundController) {
	api := app.Group("/api/v1/mobile/", middleware.AuthMiddleware)
	api.Get("/inbound/list/open", mobileInboundController.GetListInbound)
	api.Post("/inbound/scan", mobileInboundController.ScanInbound)
	api.Get("/inbound/scan/:inbound_no", mobileInboundController.GetScanInbound)
	api.Delete("/inbound/scan/:id", mobileInboundController.DeleteScannedInbound)
	api.Put("/inbound/scan/putaway/:inbound_no", mobileInboundController.ConfirmPutaway)

	// api.Get("/:id", rfInboundController.GetInboundByInboundID)
	// api.Post("/:id", rfInboundController.PostInboundByInboundID)
	// api.Get("/detail/scanned/:id", rfInboundController.GetInboundDetailScanned)
	// // api.Delete("/detail/scanned/:id", rfInboundController.DeleteBarcode)
	// api.Get("/detail/barcode/:id/:detail_id", rfInboundController.GetInboundBarcodeDetail)
	// api.Post("/confirm/putaway", rfInboundController.ConfirmPutaway)
	// api.Post("/barcode/delete", rfInboundController.DeleteBarcode)
	// api.Get("/scan/pallet/:id", rfInboundController.ScanPallet)
	// api.Post("/scan/pallet/putaway", rfInboundController.PutawayPallet)
}
