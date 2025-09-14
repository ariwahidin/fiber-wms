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
	// inboundMidleware := middleware.NewAuthMiddleware(db)
	api := app.Group(
		config.MAIN_ROUTES+"/order",
		middleware.AuthMiddleware,
	)

	api.Use(database.InjectDBMiddleware(shippingController))

	api.Post("/", shippingController.CreateOrder)
	api.Get("/", shippingController.GetListOrder)
	api.Get("/list", shippingController.GetOutboundList)
	api.Get("/:order_no", shippingController.GetOrderByNo)
	api.Get("/detail/:order_no", shippingController.GetOrderAndDetailByNo)
	api.Put("/:order_no", shippingController.UpdateOrderByID)
	api.Delete("/item/:id", shippingController.DeleteItemOrderByID)

	// api.Get("/list-order-part", shippingController.GetListDNOpen)
	// api.Get("/order/:order_no", shippingController.GetOrderByID)
	// api.Post("/order/ungroup", shippingController.UnGroupOrder)
	// api.Put("/order/detail/:id", shippingController.UpdateOrderDetailByID)
	// api.Get("/order/detail/:order_no", shippingController.GetOrderDetailItemsByOrderNo)
}
