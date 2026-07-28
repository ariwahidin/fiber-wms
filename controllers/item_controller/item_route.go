package item_controller

import (
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/products", middleware.AuthMiddleware)
	productController := &ProductController{}
	api.Use(database.InjectDBMiddleware(productController))

	api.Get("/owner-codes", productController.GetOwnerCodes)
	api.Post("/export", productController.ExportProduct)
	api.Post("/", productController.CreateProduct)
	api.Get("/lookup", productController.LookupProduct)
	api.Get("/:id", productController.GetProductByID)
	api.Put("/:id", productController.UpdateProduct)
	api.Get("/", productController.GetAllProducts)
	api.Delete("/:id", productController.DeleteProduct)
	api.Post("/upload-excel", productController.CreateProductFromExcelFile)

	// UOM Routes
	uom := app.Group("/api/v1/uoms", middleware.AuthMiddleware)
	uomController := &UomController{}
	uom.Use(database.InjectDBMiddleware(uomController))

	uom.Get("/", uomController.GetAllUOM)
	uom.Post("/item/", uomController.GetUomByItemCode)

	uom.Post("/uom-item", uomController.GetUomConversionByItemCodeAndFromUom)
	uom.Post("/conversion", uomController.CreateUom)
	uom.Post("/conversion/upload-excel", uomController.CreateUomConversionFromExcel)
	uom.Get("/conversion", uomController.GetAllUOMConversion)
	uom.Put("/conversion/:id", uomController.UpdateUOMConversion)

	categoryController := &CategoryController{}
	apiCategory := app.Group(config.MAIN_ROUTES+"/categories", middleware.AuthMiddleware)
	apiCategory.Use(database.InjectDBMiddleware(categoryController))

	apiCategory.Get("/", categoryController.GetAllCategory)
	apiCategory.Get("/:id", categoryController.GetCategoryByID)
	apiCategory.Post("/", categoryController.CreateCategory)
	apiCategory.Put("/:id", categoryController.UpdateCategory)
	apiCategory.Delete("/:id", categoryController.DeleteCategory)

	// apiProduct := app.Group(config.MAIN_ROUTES+"/categories", middleware.AuthMiddleware)
	// apiProduct.Use(database.InjectDBMiddleware(productController))

	// apiProduct.Get("/", productController.GetAllCategory)
}
