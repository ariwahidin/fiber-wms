package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupAuthRoutes(app *fiber.App) {

	authController := &controllers.AuthController{}
	api := app.Group(config.MAIN_ROUTES + "/auth")
	api.Post("/login", middleware.LoginMiddleware, controllers.Login)
	api.Get("/isLoggedIn", middleware.AuthMiddleware, authController.IsLoggedIn)

	apiLogout := app.Group(config.MAIN_ROUTES+"/auth", middleware.AuthMiddleware)
	apiLogout.Use(middleware.InjectDBMiddleware(authController))
	apiLogout.Get("/logout", authController.Logout)
}
