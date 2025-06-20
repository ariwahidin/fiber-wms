package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(app *fiber.App) {

	api := app.Group("/api/v1/products", middleware.AuthMiddleware)
	productController := &controllers.ProductController{}
	api.Use(middleware.InjectDBMiddleware(productController))

	api.Post("/", productController.CreateProduct)
	api.Get("/:id", productController.GetProductByID)
	api.Put("/:id", productController.UpdateProduct)
	api.Get("/", productController.GetAllProducts)
	api.Delete("/:id", productController.DeleteProduct)

	// UOM Routes
	uom := app.Group("/api/v1/uoms", middleware.AuthMiddleware)
	uomController := &controllers.UomController{}
	uom.Use(middleware.InjectDBMiddleware(uomController))

	uom.Get("/", productController.GetAllUOM)
	uom.Post("/conversion", uomController.CreateUom)
	uom.Get("/conversion", uomController.GetAllUOMConversion)
	uom.Put("/conversion/:id", uomController.UpdateUOMConversion)
}
