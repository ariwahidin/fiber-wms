package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

// func SetupProductRoutes(app *fiber.App, productController *controllers.ProductController) {

// 	api := app.Group("/api/v1/products", middleware.AuthMiddleware)
// 	api.Post("/", productController.CreateProduct)
// 	api.Get("/:id", productController.GetProductByID)
// 	api.Put("/:id", productController.UpdateProduct)
// 	api.Get("/", productController.GetAllProducts)
// 	api.Delete("/:id", productController.DeleteProduct)
// 	uom := app.Group("/api/v1/uoms", middleware.AuthMiddleware)
// 	uom.Get("/", productController.GetAllUOM)

// 	// uom.Post("/", productController.CreateUOM)
// 	// uom.Get("/:id", productController.GetUOMByID)
// 	// uom.Put("/:id", productController.UpdateUOM)
// 	// uom.Delete("/:id", productController.DeleteUOM)
// }

func SetupProductRoutes(app *fiber.App) {

	api := app.Group("/api/v1/products", middleware.AuthMiddleware)
	productController := &controllers.ProductController{}
	api.Use(middleware.InjectDBMiddleware(productController))

	api.Post("/", productController.CreateProduct)
	api.Get("/:id", productController.GetProductByID)
	api.Put("/:id", productController.UpdateProduct)
	api.Get("/", productController.GetAllProducts)
	api.Delete("/:id", productController.DeleteProduct)
	uom := app.Group("/api/v1/uoms", middleware.AuthMiddleware)
	uom.Get("/", productController.GetAllUOM)
}
