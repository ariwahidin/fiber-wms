package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupCategoryRoutes(app *fiber.App) {

	api := app.Group(config.MAIN_ROUTES+"/categories", middleware.AuthMiddleware)
	productController := &controllers.ProductController{}
	api.Use(database.InjectDBMiddleware(productController))

	api.Get("/", productController.GetAllCategory)
}
