package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupCustomerRoutes(app *fiber.App, customerController *controllers.CustomerController) {
	api := app.Group(config.MAIN_ROUTES+"/customers", middleware.AuthMiddleware)

	api.Get("/", customerController.GetAllCustomers)
	api.Post("/", customerController.CreateCustomer)
	api.Get("/:id", customerController.GetCustomerByID)
	api.Put("/:id", customerController.UpdateCustomer)
	api.Delete("/:id", customerController.DeleteCustomer)
}
