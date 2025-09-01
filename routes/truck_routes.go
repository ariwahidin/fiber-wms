package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupTruckRoutes(app *fiber.App) {

	truckController := &controllers.TruckController{}

	api := app.Group(config.MAIN_ROUTES+"/trucks", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(truckController))

	api.Post("/", truckController.Create)
	api.Get("/", truckController.GetAll)
	// api.Get("/:id", supplierController.GetSupplierByID)
	// api.Put("/:id", supplierController.UpdateSupplier)
	// api.Delete("/:id", supplierController.DeleteSupplier)
}
