package routes

import (
	"fiber-app/controllers"
	"fiber-app/controllers/mobiles"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMobileInboundRoutes(app *fiber.App, mobileInboundController *controllers.MobileInboundController) {
	api := app.Group("/api/v1/mobile/", middleware.AuthMiddleware)
	api.Get("/inbound/list/open", mobileInboundController.GetListInbound)
	api.Post("/inbound/scan", mobileInboundController.ScanInbound)
	api.Get("/inbound/scan/:id", mobileInboundController.GetScanInbound)
	api.Delete("/inbound/scan/:id", mobileInboundController.DeleteScannedInbound)
	api.Put("/inbound/scan/putaway/:inbound_no", mobileInboundController.ConfirmPutaway)
	api.Get("/inbound/detail/:inbound_no", mobileInboundController.GetInboundDetail)
	api.Post("/inbound/search/location", mobileInboundController.GetInboundBarcodeByLocation)
	api.Post("/inbound/putaway/location/:inbound_no", mobileInboundController.ConfirmPutawayByLocation)
}

func SetupMobileInventoryRoutes(app *fiber.App, mobileInventoryController *mobiles.MobileInventoryController) {
	api := app.Group("/api/v1/mobile/", middleware.AuthMiddleware)
	api.Get("/inventory/location/:location", mobileInventoryController.GetItemsByLocation)
	api.Post("/inventory/dummy", mobileInventoryController.CreateDummyInventory)
	api.Post("/inventory/location/barcode", mobileInventoryController.GetItemsByLocationAndBarcode)
	api.Post("/inventory/transfer/location/barcode", mobileInventoryController.ConfirmTransferByLocationAndBarcode)
	api.Post("/inventory/transfer/location/serial", mobileInventoryController.ConfirmTransferBySerial)
}

func SetupMobileOutboundRoutes(app *fiber.App, mobileOutboundController *mobiles.MobileOutboundController) {
	api := app.Group("/api/v1/mobile/", middleware.AuthMiddleware)
	api.Get("/outbound/list/open", mobileOutboundController.GetListOutbound)
	api.Get("/outbound/detail/:outbound_no", mobileOutboundController.GetListOutboundDetail)
	api.Post("/outbound/picking/scan", mobileOutboundController.ScanPicking)
	api.Get("/outbound/picking/scan/:id", mobileOutboundController.GetListOutboundBarcode)
	api.Delete("/outbound/picking/scan/:id", mobileOutboundController.DeleteOutboundBarcode)
}
