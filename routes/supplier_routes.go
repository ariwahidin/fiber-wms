package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupSupplierRoutes(app *fiber.App) {

	api := app.Group(config.MAIN_ROUTES+"/suppliers", middleware.AuthMiddleware)
	supplierController := &controllers.SupplierController{}
	api.Use(database.InjectDBMiddleware(supplierController))

	api.Post("/upload-excel", supplierController.CreateSupplierFromExcel)
	api.Get("/owner-codes", supplierController.GetOwnerCodes)
	api.Post("/export", supplierController.ExportSuppliers)
	api.Post("/", supplierController.CreateSupplier)
	api.Get("/", supplierController.GetAllSuppliers)
	api.Get("/:id", supplierController.GetSupplierByID)
	api.Put("/:id", supplierController.UpdateSupplier)
	api.Delete("/:id", supplierController.DeleteSupplier)
}
