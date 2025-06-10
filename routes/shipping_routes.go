package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupShippingRoutes(app *fiber.App) {
	shippingController := &controllers.ShippingController{}
	// inboundMidleware := middleware.NewAuthMiddleware(db)
	api := app.Group(
		config.MAIN_ROUTES+"/shipping",
		middleware.AuthMiddleware,
		// inboundMidleware.CheckPermission("create_inbound"),
	)

	api.Use(middleware.InjectDBMiddleware(shippingController))

	api.Put("/order/:id", shippingController.UpdateOrderHeaderByID)
	api.Get("/list-order-part", shippingController.GetListDNOpen)
	api.Post("/combine-order", shippingController.CreateOrder)
	api.Get("/list-order", shippingController.GetListOrder)
	api.Get("/order/:order_no", shippingController.GetOrderByID)
	api.Post("/order/ungroup", shippingController.UnGroupOrder)
	api.Put("/order/detail/:id", shippingController.UpdateOrderDetailByID)
	api.Get("/order/detail/:order_no", shippingController.GetOrderDetailItemsByOrderNo)
}
