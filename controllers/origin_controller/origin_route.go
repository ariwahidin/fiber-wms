package origin_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupOriginRoutes(app *fiber.App) {
	originController := &OriginController{}
	api := app.Group(config.MAIN_ROUTES+"/origins", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(originController))
	api.Post("/", originController.Create)
	api.Get("/", originController.GetAll)
	api.Put("/:id", originController.Update)
}
