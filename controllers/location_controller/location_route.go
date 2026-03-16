package location_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupLocationRoutes(app *fiber.App) {
	// Group endpoint with prefix and auth middleware
	api := app.Group(config.MAIN_ROUTES+"/locations", middleware.AuthMiddleware)
	locationController := &LocationController{}
	api.Use(database.InjectDBMiddleware(locationController))

	// Register endpoints
	api.Post("/upload-excel", locationController.CreateLocationFromExcel)
	api.Post("/", locationController.CreateLocation)
	api.Get("/", locationController.GetAllLocations)
	api.Get("/:id", locationController.GetLocationByID)
	api.Put("/:id", locationController.UpdateLocation)
	api.Delete("/:id", locationController.DeleteLocation)
}
