package routes

import (
	"fiber-app/config"
	"fiber-app/controllers/mobiles"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMobileInboundRoutes(app *fiber.App) {
	mobileInboundController := &mobiles.MobileInboundController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(middleware.InjectDBMiddleware(mobileInboundController))

	api.Get("/inbound/list/open", mobileInboundController.GetListInbound)
	api.Post("/inbound/scan", mobileInboundController.ScanInbound)
	api.Get("/inbound/scan/:id", mobileInboundController.GetScanInbound)
	api.Delete("/inbound/scan/:id", mobileInboundController.DeleteScannedInbound)
	api.Put("/inbound/scan/putaway/:inbound_no", mobileInboundController.ConfirmPutaway)
	api.Get("/inbound/detail/:inbound_no", mobileInboundController.GetInboundDetail)
	api.Post("/inbound/search/location", mobileInboundController.GetInboundBarcodeByLocation)
	api.Post("/inbound/putaway/location/:inbound_no", mobileInboundController.ConfirmPutawayByLocation)
	api.Put("/inbound/barcode/:id", mobileInboundController.EditInboundBarcode)
	api.Get("/inbound/barcode/getlocation/:inbound_no", mobileInboundController.GetSequenceLocation)
}

func SetupMobileInventoryRoutes(app *fiber.App) {
	mobileInventoryController := &mobiles.MobileInventoryController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(middleware.InjectDBMiddleware(mobileInventoryController))

	api.Get("/inventory/location/:location", mobileInventoryController.GetItemsByLocation)
	api.Post("/inventory/dummy", mobileInventoryController.CreateDummyInventory)
	api.Post("/inventory/location/barcode", mobileInventoryController.GetItemsByLocationAndBarcode)
	api.Post("/inventory/transfer/location/barcode", mobileInventoryController.ConfirmTransferByLocationAndBarcode)
	api.Post("/inventory/transfer/location/serial", mobileInventoryController.ConfirmTransferBySerial)
}

func SetupMobileOutboundRoutes(app *fiber.App) {
	mobileOutboundController := &mobiles.MobileOutboundController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(middleware.InjectDBMiddleware(mobileOutboundController))

	api.Get("/outbound/list/open", mobileOutboundController.GetListOutbound)
	api.Get("/outbound/detail/:outbound_no", mobileOutboundController.GetListOutboundDetail)
	// api.Post("/outbound/picking/scan", mobileOutboundController.ScanPicking)
	// api.Get("/outbound/picking/scan/:id", mobileOutboundController.GetListOutboundBarcode)
	// api.Delete("/outbound/picking/scan/:id", mobileOutboundController.DeleteOutboundBarcode)
}

func SetupMobileShippingGuestRoutes(app *fiber.App, shippingGuestController *mobiles.ShippingGuestController) {
	// api := app.Group("/api/v1/mobile/", middleware.AuthMiddleware)
	api := app.Group("/guest/api/v1")
	api.Get("/shipping/open/:spk", shippingGuestController.GetListShippingOpenBySPK)
	api.Put("/shipping/update/:order_no", shippingGuestController.UpdateShipping)
}

func SetupMobilePackingRoutes(app *fiber.App) {
	packingController := &mobiles.MobilePackingController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(middleware.InjectDBMiddleware(packingController))

	api.Post("/packing/generate", packingController.GenerateKoli)
	api.Get("/packing/koli/:outbound_no", packingController.GetKoliByOutbound)
	api.Post("/packing/add", packingController.AddToKoli)
	api.Delete("/packing/koli/detail/:id", packingController.RemoveItemFromKoli)
	api.Delete("/packing/koli/:id", packingController.RemoveKoliByID)
}
