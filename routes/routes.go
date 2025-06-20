package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupWarehouseRoutes(app *fiber.App) {
	controller := &controllers.WarehouseController{}
	api := app.Group(config.MAIN_ROUTES+"/warehouses", middleware.AuthMiddleware)
	api.Use(middleware.InjectDBMiddleware(controller))
	api.Get("/", controller.GetAllWarehouses)
}
