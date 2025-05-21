package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupMenuRoutes(app *fiber.App, db *gorm.DB) {
	menuController := controllers.NewMenuController(db)

	api := app.Group(
		"/api/v1/menus",
		middleware.AuthMiddleware,
	)

	api.Get("/permissions/:id", menuController.GetMenuPermission)
	api.Post("/permissions/:id", menuController.UpdatePermissionMenus)
	api.Get("/user", menuController.GetMenuUser)
	api.Get("/", menuController.GetAllMenus)
	api.Get("/:id", menuController.GetMenuByID)
	api.Post("/", menuController.CreateMenu)
	api.Put("/:id", menuController.UpdateMenu)
	api.Delete("/:id", menuController.DeleteMenu)
}
