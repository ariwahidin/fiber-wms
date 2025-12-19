package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupUserRoutes(app *fiber.App) {
	userController := &controllers.UserController{}

	api := app.Group(config.MAIN_ROUTES+"/users", middleware.AuthMiddleware)
	api.Use(database.InjectDBMiddleware(userController))

	api.Post("/", userController.CreateUser)
	api.Put("/:id", userController.UpdateUser)
	api.Get("/:id", userController.GetUserByID)
	api.Get("/", userController.GetAllUsers)
	api.Delete("/:id", userController.DeleteUser)

	// profile := app.Group("/api/v1/user", middleware.AuthMiddleware)
	// profile.Get("/profile", userController.GetProfile)

	role := app.Group(config.MAIN_ROUTES+"/roles", middleware.AuthMiddleware)
	role.Use(database.InjectDBMiddleware(userController))

	role.Get("/", userController.GetRoles)
	role.Get("/:id", userController.GetRoleByID)
	role.Put("/:id", userController.UpdateRole)
	role.Post("/", userController.CreateRole)
	role.Put("/permissions/:id", userController.UpdatePermissionsForRole)

	permission := app.Group(config.MAIN_ROUTES+"/permissions", middleware.AuthMiddleware)
	permission.Use(database.InjectDBMiddleware(userController))

	permission.Get("/", userController.GetPermissions)
	permission.Get("/:id", userController.GetPermissionByID)
	permission.Post("/", userController.CreatePermission)
	permission.Put("/:id", userController.UpdatePermission)

}
