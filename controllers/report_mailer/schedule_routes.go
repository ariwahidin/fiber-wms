package report_mailer

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupScheduleRoutes(app *fiber.App, db *gorm.DB, queryDB *gorm.DB) {
	ctrl := NewScheduleController(db, queryDB)

	api := app.Group("/api/v1/report-mailer", middleware.AuthMiddleware)

	// Schedule per report
	api.Get("/reports/:id/schedule", ctrl.GetSchedule)
	api.Post("/reports/:id/schedule", ctrl.SaveSchedule)
	api.Delete("/reports/:id/schedule", ctrl.DeleteSchedule)

	// Trigger manual kirim sekarang
	api.Post("/reports/:id/send-now", ctrl.SendNow)

	// Reload semua scheduler
	api.Post("/scheduler/reload", ctrl.ReloadAll)
}

// func SetupScheduleRoutes(app *fiber.App, db *gorm.DB) {
// 	ctrl := NewScheduleController(db)

// 	api := app.Group("/api/v1/report-mailer", middleware.AuthMiddleware)

// 	// Schedule per report
// 	api.Get("/reports/:id/schedule", ctrl.GetSchedule)
// 	api.Post("/reports/:id/schedule", ctrl.SaveSchedule)
// 	api.Delete("/reports/:id/schedule", ctrl.DeleteSchedule)

// 	// Trigger manual kirim sekarang
// 	api.Post("/reports/:id/send-now", ctrl.SendNow)

// 	// Reload semua scheduler
// 	api.Post("/scheduler/reload", ctrl.ReloadAll)
// }
