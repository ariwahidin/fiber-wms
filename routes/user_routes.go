package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupUserRoutes(app *fiber.App, userController *controllers.UserController) {
	api := app.Group("/api/v1/users", middleware.AuthMiddleware)

	api.Post("/", userController.CreateUser)
	api.Get("/:id", userController.GetUserByID)
	api.Get("/", userController.GetAllUsers)
	api.Put("/:id", userController.UpdateUser)
	api.Delete("/:id", userController.DeleteUser)

	profile := app.Group("/api/v1/user", middleware.AuthMiddleware)
	profile.Get("/profile", userController.GetProfile)
}
