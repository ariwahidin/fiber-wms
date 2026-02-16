package routes

import (
	"fiber-app/config"
	"fiber-app/controllers/mobiles"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMobileInboundRoutes(app *fiber.App) {
	mobileInboundController := &mobiles.MobileInboundController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(mobileInboundController))

	api.Get("/inbound/list/open", mobileInboundController.GetListInbound)
	api.Post("/inbound/check", mobileInboundController.CheckItem)
	api.Post("/inbound/scan", mobileInboundController.ScanInbound)
	api.Get("/inbound/scan/:id", mobileInboundController.GetScanInbound)
	api.Delete("/inbound/scan/:id", mobileInboundController.DeleteScannedInbound)
	// api.Put("/inbound/scan/putaway/:inbound_no", mobileInboundController.ConfirmPutaway)
	api.Get("/inbound/detail/:inbound_no", mobileInboundController.GetInboundDetail)
	api.Post("/inbound/search/location", mobileInboundController.GetInboundBarcodeByLocation)
	// api.Post("/inbound/putaway/location/:inbound_no", mobileInboundController.ConfirmPutawayByLocation)
	api.Put("/inbound/barcode/:id", mobileInboundController.EditInboundBarcode)
	api.Get("/inbound/barcode/getlocation/:inbound_no", mobileInboundController.GetSequenceLocation)
	api.Post("/inbound/putaway/scan-pallet", mobileInboundController.CheckItemPutaway)
	api.Post("/inbound/putaway/all", mobileInboundController.PutawayAll)
}

func SetupMobileInventoryRoutes(app *fiber.App) {
	mobileInventoryController := &mobiles.MobileInventoryController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(mobileInventoryController))

	api.Get("/inventory/by-item/:barcode", mobileInventoryController.GetItemsByBarcode)
	api.Get("/inventory/location/:location", mobileInventoryController.GetItemsByLocation)
	// api.Post("/inventory/dummy", mobileInventoryController.CreateDummyInventory)
	api.Post("/inventory/location/barcode", mobileInventoryController.GetItemsByLocationAndBarcode)
	api.Post("/inventory/transfer/location/barcode", mobileInventoryController.ConfirmTransferByLocationAndBarcode)
	api.Post("/inventory/transfer-by-inventory-id", mobileInventoryController.ConfirmTransferByInventoryID)
	api.Post("/inventory/add-location", mobileInventoryController.CreateLocation)
	// api.Post("/inventory/add-item", mobileInventoryController.CreateRegisterProduct)
	// Route untuk registrasi produk baru
	api.Post("/inventory/add-item", mobileInventoryController.CreateRegisterProduct)

	// Route untuk mendapatkan semua produk
	api.Get("/inventory/products", mobileInventoryController.GetAllProducts)

	// Route untuk mendapatkan produk berdasarkan ID
	api.Get("/inventory/products/:id", mobileInventoryController.GetProductByID)

	// Route untuk update produk
	api.Put("/inventory/products/:id", mobileInventoryController.UpdateProduct)

	// Route untuk hapus produk
	api.Delete("/inventory/products/:id", mobileInventoryController.DeleteProduct)
}

func SetupMobileOutboundRoutes(app *fiber.App) {
	mobileOutboundController := &mobiles.MobileOutboundController{}
	api := app.Group(config.MAIN_ROUTES+"/mobile", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(mobileOutboundController))

	api.Get("/outbound/list/open", mobileOutboundController.GetListOutbound)
	api.Get("/outbound/detail/:outbound_no", mobileOutboundController.GetListOutboundDetail)
	api.Post("/outbound/item-check/:outbound_no", mobileOutboundController.CheckItem)
	api.Post("/outbound/picking/scan/:outbound_no", mobileOutboundController.ScanPicking)
	api.Get("/outbound/picking/scan/:id", mobileOutboundController.GetListOutboundBarcode)
	api.Delete("/outbound/picking/scan/:id", mobileOutboundController.DeleteOutboundBarcode)
	api.Get("/outbound/picking/list/:outbound_no", mobileOutboundController.GetPickingList)
	api.Post("/outbound/picking/override/:id", mobileOutboundController.OverridePicking)
	api.Get("/outbound/:outbound_no/cartons", mobileOutboundController.GetCartonNoByOutboundNo)

	// Get all active master cartons
	api.Get("/outbound/master-cartons", mobileOutboundController.GetMasterCartons)

	// Get specific master carton
	api.Get("/outbound/master-cartons/:id", mobileOutboundController.GetMasterCartonByID)

	// Update Carton Type By Outbound No and Carton No
	api.Put("/outbound/picking/update-carton/:outbound_no", mobileOutboundController.EditCartonTypeByOrderNoAndPackNo)
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
	api.Use(database.InjectDBMiddleware(packingController))

	api.Post("/packing/generate", packingController.GenerateKoli)
	api.Get("/packing/koli/:outbound_no", packingController.GetKoliByOutbound)
	api.Post("/packing/add", packingController.AddToKoli)
	api.Delete("/packing/koli/detail/:id", packingController.RemoveItemFromKoli)
	api.Delete("/packing/koli/:id", packingController.RemoveKoliByID)
}
