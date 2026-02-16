package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMasterCartonRoutes(app *fiber.App) {
	masterCartonController := &controllers.MasterCartonController{}
	api := app.Group(config.MAIN_ROUTES+"/master-cartons", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(masterCartonController))
	api.Post("/", masterCartonController.Create)
	api.Get("/", masterCartonController.GetAll)
	api.Get("/:id", masterCartonController.GetByID)
	api.Put("/:id", masterCartonController.Update)
	api.Delete("/:id", masterCartonController.Delete)
}
