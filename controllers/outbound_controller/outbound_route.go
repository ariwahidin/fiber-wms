package outbound_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupOutboundRoutes(app *fiber.App) {
	api := app.Group(
		config.MAIN_ROUTES+"/outbound",
		middleware.AuthMiddleware,
	)
	outboundController := &OutboundController{}
	shopeeController := &ShopeeSyncController{}

	api.Use(database.InjectDBMiddleware(outboundController))
	api.Use(database.InjectDBMiddleware(shopeeController))

	api.Post("/upload-excel", outboundController.CreateOutboundFromExcelFile)
	api.Post("/upload-ecommerce-excel", outboundController.CreateOutboundFromEcommerceExcel)
	api.Post("/", outboundController.CreateOutbound)
	api.Get("/", outboundController.GetOutboundList)
	api.Get("/scan-details/:outbound_no", outboundController.GetOutboundBarcodeByOutboundNo)
	api.Get("/vas", outboundController.GetOutboundVasSummary)
	api.Get("/:outbound_no/vas-items", outboundController.GetOutboundVasByID)
	api.Get("/serial/:outbound_no", outboundController.GetSerialNumberList)
	api.Post("/open", outboundController.HandleOpen)
	api.Post("/open/process", outboundController.ProccesHandleOpen)
	// api.Post("/open/temp", outboundController.HandleOpenBackToOriginLocation)
	api.Get("/handling", outboundController.GetOutboundListOutboundHandling)
	api.Get("/handling/bill/:outbound_no", outboundController.ViewBillHandlingByOutbound)
	api.Get("/handling/:outbound_no", outboundController.GetOutboundHandlingByID)
	api.Put("/handling/:outbound_no", outboundController.UpdateOutboundDetailHandling)
	api.Get("/:outbound_no", outboundController.GetOutboundByID)
	api.Put("/:outbound_no", outboundController.UpdateOutboundByID)
	// api.Post("/item/:id", outboundController.SaveItem)
	api.Get("/item/:id", outboundController.GetItem)
	api.Delete("/item/:id", outboundController.DeleteItem)
	api.Post("/picking/:id", outboundController.PickingOutbound)
	api.Get("/picking/sheet/:id", outboundController.GetPickingSheet)
	api.Post("/picking/complete/:id", outboundController.PickingComplete)
	api.Get("/koli-details/:outbound_no", outboundController.GetKoliDetails)

	api.Post("/packing/generate/", outboundController.CreatePacking)
	api.Get("/packing/all/", outboundController.GetAllPacking)
	api.Get("/:id/packing/:packing_no", outboundController.GetPackingItems)

	// Parse PDF endpoint
	api.Post("/parse-pdf", outboundController.ParseOutboundFromPDFFile)
	api.Post("/create-from-pdf", outboundController.CreateOutboundFromPdf)

	// api.Post("/order", outboundController.CreateOrder)

	// api.Put("/:id", outboundController.SaveOutbound)
	// api.Get("/draft", outboundController.GetOutboundDraft)
	// api.Get("/create", outboundController.CreateOutbound)
	// api.Post("/item", outboundController.CreateItemOutbound)
	// api.Get("/:id", outboundController.GetOutboundByID)
	// api.Delete("/item/:id", outboundController.DeleteItemOutbound)

	// api.Post("/picking/complete/:id", outboundController.PickingComplete)

	// shopeeSyncCtrl := outbound_controller.NewShopeeSyncController(db, queryDB)
	api.Post("/shopee/sync", shopeeController.SyncShopeeOrders)
	// api.Post("/shopee/refresh-token", shopeeController.HandleRefreshToken)

	api.Get("/shopee/tracking/:order_sn", shopeeController.GetTrackingNumber)
	api.Get("/shopee/label/:order_sn", shopeeController.GetShippingLabel)

	api.Get("/shopee/shipping-param/:order_sn", shopeeController.GetShippingParameter)
	api.Post("/shopee/init-shipment", shopeeController.InitShipment)

	api.Get("/shopee/config", shopeeController.GetConfig)
	api.Post("/shopee/config", shopeeController.SaveConfig)
	api.Get("/shopee/auth-url", shopeeController.GenerateAuthURL)
	api.Post("/shopee/refresh-token", shopeeController.HandleRefreshTokenDB)
	api.Post("/shopee/manual-update-token", shopeeController.ManualUpdateToken)
	api.Get("/shopee/config-raw", shopeeController.GetConfigRaw)

	api_auth := app.Group(
		config.MAIN_ROUTES,
	)
	shopeeSyncController := &ShopeeSyncController{}
	api_auth.Use(database.InjectDBMiddlewareFromEnv(shopeeSyncController))
	api_auth.Get("/shopee/callback", shopeeSyncController.OAuthCallback)
}
