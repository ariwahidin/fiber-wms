package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

// func SetupShippingRoutes(app *fiber.App) {
// 	shippingController := &controllers.ShippingController{}
// 	api := app.Group(
// 		config.MAIN_ROUTES+"/order",
// 		middleware.AuthMiddleware,
// 	)

// 	api.Use(database.InjectDBMiddleware(shippingController))
// 	api.Use(database.InjectQueryDBMiddleware(shippingController))

// 	api.Post("/", shippingController.CreateOrder)
// 	api.Get("/", shippingController.GetListOrder)
// 	api.Get("/filter", shippingController.GetListOrderFilter)
// 	api.Patch("/status", shippingController.UpdateOrderStatus)
// 	api.Get("/list", shippingController.GetOutboundList)
// 	api.Get("/:order_no", shippingController.GetOrderByNo)
// 	api.Get("/detail/:order_no", shippingController.GetOrderAndDetailByNo)
// 	api.Get("/surat-jalan/:order_no", shippingController.GetSuratJalanByOrderNo)
// 	api.Put("/:order_no", shippingController.UpdateOrderByID)
// 	api.Delete("/item/:id", shippingController.DeleteItemOrderByID)

// }

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
	api.Get("/surat-jalan/:order_no", shippingController.GetSuratJalanByOrderNo)
	api.Get("/:order_no", shippingController.GetOrderByNo)
	api.Put("/:order_no", shippingController.UpdateOrderByID)
	api.Delete("/item/:id", shippingController.DeleteItemOrderByID)
}
