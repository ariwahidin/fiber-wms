package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupShippingRoutes(app *fiber.App) {
	shippingController := &controllers.ShippingController{}

	api := app.Group(
		config.MAIN_ROUTES+"/order",
		middleware.AuthMiddleware,
	)

	api.Use(database.InjectDBMiddleware(shippingController))
	api.Use(database.InjectQueryDBMiddleware(shippingController))

	// Delivery Hub Master
	api.Get("/delivery-hubs", shippingController.GetDeliveryHubs)
	api.Post("/delivery-hubs", shippingController.CreateDeliveryHub)
	api.Put("/delivery-hubs/:id", shippingController.UpdateDeliveryHub)
	api.Delete("/delivery-hubs/:id", shippingController.DeleteDeliveryHub)

	// Existing SPK routes
	api.Post("/", shippingController.CreateOrder)
	api.Get("/", shippingController.GetListOrder)
	api.Get("/filter", shippingController.GetListOrderFilter)
	api.Patch("/status", shippingController.UpdateOrderStatus)
	api.Get("/list", shippingController.GetOutboundList)
	api.Get("/detail/:order_no", shippingController.GetOrderAndDetailByNo)

	api.Get("/sj/:sj_no", shippingController.GetSuratJalanDetail)
	api.Put("/sj/:sj_no", shippingController.UpdateSuratJalan)
	api.Delete("/sj/:sj_no", shippingController.DeleteSuratJalan)
	api.Patch("/sj/:sj_no/release", shippingController.ReleaseSuratJalan)
	api.Patch("/sj/:sj_no/cancel", shippingController.CancelSuratJalan)
	api.Patch("/sj/:sj_no/delivered", shippingController.MarkSuratJalanDelivered)

	api.Get("/surat-jalan/:order_no", shippingController.GetSuratJalanByOrderNo)

	api.Get("/:order_no/surat-jalan", shippingController.GetSuratJalanListByOrderNo)
	api.Get("/:order_no/surat-jalan/available-packages", shippingController.GetAvailablePackages)
	api.Post("/:order_no/surat-jalan", shippingController.CreateSuratJalan)

	api.Get("/:order_no", shippingController.GetOrderByNo)
	api.Put("/:order_no", shippingController.UpdateOrderByID)
	api.Delete("/item/:id", shippingController.DeleteItemOrderByID)
}
