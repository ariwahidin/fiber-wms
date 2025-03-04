package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupCustomerRoutes(app *fiber.App, customerController *controllers.CustomerController) {
	api := app.Group("/api/v1/customers", middleware.AuthMiddleware)

	api.Post("/", customerController.CreateCustomer)
	api.Get("/", customerController.GetAllCustomers)
	api.Get("/:id", customerController.GetCustomerByID)
	api.Put("/:id", customerController.UpdateCustomer)
	api.Delete("/:id", customerController.DeleteCustomer)
}
