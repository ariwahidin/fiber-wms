package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupHandlingRoutes(app *fiber.App) {
	handlingController := &controllers.HandlingController{}
	api := app.Group(config.MAIN_ROUTES+"/handling", middleware.AuthMiddleware)

	api.Use(database.InjectDBMiddleware(handlingController))

	// api.Get("/items", handlingController.GetAllItemHandling)
	// api.Get("/items/:id", handlingController.GetItemHandlingByID)
	// api.Put("/items/:id", handlingController.UpdateItemHandlingByID)
	// api.Delete("/items/:id", handlingController.DeleteItemHandling)

	api.Post("/", handlingController.Create)
	// api.Post("/combine", handlingController.CreateCombineHandling)
	// api.Get("/", handlingController.GetAll)
	// api.Get("/origin", handlingController.GetAllOriginHandling)
	// api.Get("/:id", handlingController.GetByID)
	// api.Put("/:id", handlingController.Update)
	// api.Delete("/:id", handlingController.Delete)

	// api.Post("/items", handlingController.CreateItemHandling)

}
