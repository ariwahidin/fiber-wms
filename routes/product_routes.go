package routes

import (
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(app *fiber.App) {

	api := app.Group("/api/v1/products", middleware.AuthMiddleware)
	productController := &controllers.ProductController{}
	api.Use(database.InjectDBMiddleware(productController))

	api.Post("/", productController.CreateProduct)
	api.Get("/:id", productController.GetProductByID)
	api.Put("/:id", productController.UpdateProduct)
	api.Get("/", productController.GetAllProducts)
	api.Delete("/:id", productController.DeleteProduct)
	api.Post("/upload-excel", productController.CreateProductFromExcelFile)

	// UOM Routes
	uom := app.Group("/api/v1/uoms", middleware.AuthMiddleware)
	uomController := &controllers.UomController{}
	uom.Use(database.InjectDBMiddleware(uomController))

	uom.Get("/", uomController.GetAllUOM)
	uom.Post("/item/", uomController.GetUomByItemCode)

	uom.Post("/uom-item", uomController.GetUomConversionByItemCodeAndFromUom)
	uom.Post("/conversion", uomController.CreateUom)
	uom.Post("/conversion/upload-excel", uomController.CreateUomConversionFromExcel)
	uom.Get("/conversion", uomController.GetAllUOMConversion)
	uom.Put("/conversion/:id", uomController.UpdateUOMConversion)
}
