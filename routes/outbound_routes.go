package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupOutboundRoutes(app *fiber.App) {
	outboundController := &controllers.OutboundController{}
	// inboundMidleware := middleware.NewAuthMiddleware(db)
	api := app.Group(
		config.MAIN_ROUTES+"/outbound",
		middleware.AuthMiddleware,
		// inboundMidleware.CheckPermission("create_inbound"),
	)

	api.Use(database.InjectDBMiddleware(outboundController))

	api.Post("/upload-excel", outboundController.CreateOutboundFromExcelFile)
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

	// api.Post("/order", outboundController.CreateOrder)

	// api.Put("/:id", outboundController.SaveOutbound)
	// api.Get("/draft", outboundController.GetOutboundDraft)
	// api.Get("/create", outboundController.CreateOutbound)
	// api.Post("/item", outboundController.CreateItemOutbound)
	// api.Get("/:id", outboundController.GetOutboundByID)
	// api.Delete("/item/:id", outboundController.DeleteItemOutbound)

	// api.Post("/picking/complete/:id", outboundController.PickingComplete)
}
