package routes

import (
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupLocationRoutes(app *fiber.App) {
	// Group endpoint with prefix and auth middleware
	api := app.Group("/api/v1/locations", middleware.AuthMiddleware)

	// Create controller instance
	locationController := &controllers.LocationController{}

	// Inject DB ke controller
	api.Use(database.InjectDBMiddleware(locationController))

	// Register endpoints
	api.Post("/", locationController.CreateLocation)
	api.Get("/", locationController.GetAllLocations)
	api.Get("/:id", locationController.GetLocationByID)
	api.Put("/:id", locationController.UpdateLocation)
	api.Delete("/:id", locationController.DeleteLocation)
}
