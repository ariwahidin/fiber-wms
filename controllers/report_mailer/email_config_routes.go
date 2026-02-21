package report_mailer

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupEmailConfigRoutes(app *fiber.App, db *gorm.DB) {
	ctrl := NewEmailConfigController(db)

	api := app.Group("/api/v1/report-mailer/email-configs", middleware.AuthMiddleware)

	api.Get("/", ctrl.GetAll)
	api.Get("/:id", ctrl.GetByID)
	api.Post("/", ctrl.Create)
	api.Put("/:id", ctrl.Update)
	api.Delete("/:id", ctrl.Delete)
	api.Post("/:id/test", ctrl.TestSend)
}
