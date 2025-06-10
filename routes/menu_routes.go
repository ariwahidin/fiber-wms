package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMenuRoutes(app *fiber.App) {
	menuController := &controllers.MenuController{}
	api := app.Group(
		config.MAIN_ROUTES+"/menus",
		middleware.AuthMiddleware,
	)
	api.Use(middleware.InjectDBMiddleware(menuController))

	api.Get("/permissions/:id", menuController.GetMenuPermission)
	api.Post("/permissions/:id", menuController.UpdatePermissionMenus)
	api.Get("/user", menuController.GetMenuUser)
	api.Get("/", menuController.GetAllMenus)
	api.Get("/:id", menuController.GetMenuByID)
	api.Post("/", menuController.CreateMenu)
	api.Put("/:id", menuController.UpdateMenu)
	api.Delete("/:id", menuController.DeleteMenu)
}
