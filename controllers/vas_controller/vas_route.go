package vas_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupVasRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/vas", middleware.AuthMiddleware)
	vasController := &VasController{}
	api.Use(database.InjectDBMiddleware(vasController))
	api.Post("/main-vas", vasController.CreateMainVas)
	api.Get("/main-vas", vasController.GetAllMainVas)
	api.Put("/main-vas/:id", vasController.UpdateMainVas)
	api.Delete("/main-vas/:id", vasController.DeleteMainVas)

	api.Post("/page", vasController.CreateVas)
	api.Get("/page", vasController.GetAllVas)
	api.Put("/page/:id", vasController.UpdateVas)
	api.Delete("/page/:id", vasController.DeleteVas)
}
