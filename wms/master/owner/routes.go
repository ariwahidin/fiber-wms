package owner

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupOwnerRoutes(app *fiber.App) {
	// To fix import cycle, temporarily remove middleware usage here.
	api := app.Group(config.MAIN_ROUTES+"/owners", middleware.AuthMiddleware)
	ownerController := &OwnerHandler{}
	api.Use(database.InjectDBMiddleware(ownerController))

	// api.Get("/", ownerController.GetAllOwners)
	// api.Post("/", ownerController.CreateOwner)
	// api.Get("/:id", ownerController.GetOwnerByID)
	// api.Put("/:id", ownerController.UpdateOwner)
	// api.Delete("/:id", ownerController.DeleteOwner)
}
