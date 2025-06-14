package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
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

	api.Use(middleware.InjectDBMiddleware(outboundController))

	api.Post("/", outboundController.CreateOutbound)
	api.Get("/", outboundController.GetOutboundList)
	api.Get("/:outbound_no", outboundController.GetOutboundByID)
	api.Put("/:outbound_no", outboundController.UpdateOutboundByID)
	api.Post("/item/:id", outboundController.SaveItem)
	api.Get("/item/:id", outboundController.GetItem)
	api.Delete("/item/:id", outboundController.DeleteItem)
	api.Post("/picking/:id", outboundController.PickingOutbound)
	api.Get("/picking/sheet/:id", outboundController.GetPickingSheet)
	api.Post("/picking/complete/:id", outboundController.PickingComplete)
	api.Get("/koli-details/:outbound_no", outboundController.GetKoliDetails)

	// api.Put("/:id", outboundController.SaveOutbound)
	// api.Get("/draft", outboundController.GetOutboundDraft)
	// api.Get("/create", outboundController.CreateOutbound)
	// api.Post("/item", outboundController.CreateItemOutbound)
	// api.Get("/:id", outboundController.GetOutboundByID)
	// api.Delete("/item/:id", outboundController.DeleteItemOutbound)

	// api.Post("/picking/complete/:id", outboundController.PickingComplete)
}
