package routes

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/database"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupAuthRoutes(app *fiber.App) {

	authController := &controllers.AuthController{}
	api := app.Group(config.MAIN_ROUTES + "/auth")
	api.Post("/login", middleware.LoginMiddleware, controllers.Login)
	api.Post("/login/confirm", middleware.LoginMiddleware, controllers.LoginConfirm)
	api.Post("/login/sessions", middleware.LoginMiddleware, controllers.GetSessionActive)
	api.Post("/refresh", middleware.LoginMiddleware, controllers.RefreshToken)
	// api.Get("/isLoggedIn", middleware.AuthMiddleware, authController.IsLoggedIn)

	apiLogout := app.Group(config.MAIN_ROUTES+"/auth", middleware.AuthMiddleware)
	apiLogout.Use(database.InjectDBMiddleware(authController))
	apiLogout.Get("/logout", authController.Logout)
}
