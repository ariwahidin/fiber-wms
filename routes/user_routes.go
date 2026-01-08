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

	api.Get("/owners/available", userController.GetAvailableOwners)

	// Get all users with their owners
	api.Get("/owners/all", userController.GetAllUserOwners)

	// Create user-owner relationships (assign multiple owners to user)
	api.Post("/owners/", userController.CreateUserOwner)

	// Get all owners for a specific user
	api.Get("/owners/:userId", userController.GetUserOwners)

	// Update user owners (replace all)
	api.Put("/owners/:userId", userController.UpdateUserOwners)

	// Delete specific user-owner relationship
	api.Delete("/owners/:userId/:ownerId", userController.DeleteUserOwner)

	// Delete all owners from a user
	api.Delete("/owners/:userId", userController.DeleteAllUserOwners)

	profile := app.Group("/api/v1/user", middleware.AuthMiddleware)
	profile.Use(database.InjectDBMiddleware(userController))
	profile.Get("/profile", userController.GetProfile)
	profile.Put("/profile", userController.UpdateUserProfile)

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
