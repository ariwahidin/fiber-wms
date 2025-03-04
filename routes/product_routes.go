package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(app *fiber.App, productController *controllers.ProductController) {
	api := app.Group("/api/v1/products", middleware.AuthMiddleware)
	api.Post("/", productController.CreateProduct)
	api.Get("/:id", productController.GetUserByID)
	api.Put("/:id", productController.UpdateProduct)
	api.Get("/", productController.GetAllProducts)
	api.Delete("/:id", productController.DeleteProduct)
}
