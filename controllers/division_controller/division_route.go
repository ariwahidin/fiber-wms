package division_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupDivisionRoutes(app *fiber.App) {
	divisionController := &DivisionController{}
	api := app.Group(config.MAIN_ROUTES+"/divisions", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(divisionController))
	api.Post("/", divisionController.Create)
	api.Get("/", divisionController.GetAll)
	api.Get("/:id", divisionController.GetByID)
	api.Put("/:id", divisionController.Update)
	api.Delete("/:id", divisionController.Delete)
}
