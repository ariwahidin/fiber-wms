package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupUserRoutes(app *fiber.App, userController *controllers.UserController) {
	api := app.Group("/api/v1/users", middleware.AuthMiddleware)

	api.Post("/", userController.CreateUser)
	api.Put("/:id", userController.UpdateUser)
	api.Get("/:id", userController.GetUserByID)
	api.Get("/", userController.GetAllUsers)
	api.Delete("/:id", userController.DeleteUser)

	profile := app.Group("/api/v1/user", middleware.AuthMiddleware)
	profile.Get("/profile", userController.GetProfile)

	role := app.Group("/api/v1/roles", middleware.AuthMiddleware)
	role.Get("/", userController.GetRoles)
	role.Post("/", userController.CreateRole)
	role.Put("/permissions/:id", userController.UpdatePermissionsForRole)

	permission := app.Group("/api/v1/permissions", middleware.AuthMiddleware)
	permission.Get("/", userController.GetPermissions)
	permission.Get("/:id", userController.GetPermissionByID)
	permission.Post("/", userController.CreatePermission)
	permission.Put("/:id", userController.UpdatePermission)

}
