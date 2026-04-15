package shopee_config_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupShopeeConfigRoutes(app *fiber.App) {
	api := app.Group(
		config.MAIN_ROUTES+"/config",
		middleware.AuthMiddleware,
	)
	shopeeController := &ShopeeConfigController{}
	app.Use(database.InjectDBMiddlewareFromEnv(shopeeController))

	api.Get("/shopee/scheduler/status", shopeeController.GetSchedulerStatus)
	api.Post("/shopee/scheduler/start", shopeeController.StartScheduler)
	api.Post("/shopee/scheduler/stop", shopeeController.StopScheduler)
}
