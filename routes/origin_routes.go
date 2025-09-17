package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupOriginRoutes(app *fiber.App) {

	originController := &controllers.OriginController{}
	api := app.Group(config.MAIN_ROUTES+"/origins", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(originController))
	api.Post("/", originController.Create)
	api.Get("/", originController.GetAll)
	// api.Get("/:id", supplierController.GetSupplierByID)
	api.Put("/:id", originController.Update)
	// api.Delete("/:id", supplierController.DeleteSupplier)
}
