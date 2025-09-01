package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupDashboardRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/dashboard", middleware.AuthMiddleware)
	dashboardController := &controllers.DashboardController{}
	api.Use(database.InjectDBMiddleware(dashboardController))

	api.Get("/", dashboardController.GetDashboard)
}
