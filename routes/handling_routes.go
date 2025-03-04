package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupHandlingRoutes(app *fiber.App, handlingController *controllers.HandlingController) {
	api := app.Group("/api/v1/handling", middleware.AuthMiddleware)

	api.Post("/", handlingController.Create)
	api.Post("/combine", handlingController.CreateCombineHandling)

	api.Get("/", handlingController.GetAll)
	api.Get("/origin", handlingController.GetAllOriginHandling)
	api.Get("/:id", handlingController.GetByID)
	api.Put("/:id", handlingController.Update)
	api.Delete("/:id", handlingController.Delete)

}
