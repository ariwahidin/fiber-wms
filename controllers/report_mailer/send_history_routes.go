package report_mailer

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupSendHistoryRoutes(app *fiber.App, db *gorm.DB) {
	ctrl := NewSendHistoryController(db)

	api := app.Group("/api/v1/report-mailer", middleware.AuthMiddleware)

	// History per report
	api.Get("/reports/:id/history", ctrl.GetByReport)
	api.Delete("/reports/:id/history", ctrl.ClearByReport)

	// History semua report (global)
	api.Get("/history", ctrl.GetAll)
}
