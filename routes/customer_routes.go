package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupCustomerRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/customers", middleware.AuthMiddleware)
	customerController := &controllers.CustomerController{}
	api.Use(database.InjectDBMiddleware(customerController))

	api.Get("/", customerController.GetAllCustomers)
	api.Post("/", customerController.CreateCustomer)
	api.Post("/upload-excel", customerController.CreateCustomerFromExcel)
	api.Get("/owner-codes", customerController.GetOwnerCodes)
	api.Get("/:id", customerController.GetCustomerByID)
	api.Put("/:id", customerController.UpdateCustomer)
	api.Delete("/:id", customerController.DeleteCustomer)
	api.Post("/export", customerController.ExportCustomers)
}
