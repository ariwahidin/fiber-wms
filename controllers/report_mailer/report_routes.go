package report_mailer

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// func SetupReportRoutes(app *fiber.App, db *gorm.DB) {
// 	ctrl := NewReportController(db)

// 	api := app.Group("/api/v1/report-mailer/reports", middleware.AuthMiddleware)

// 	// Report CRUD
// 	api.Get("/", ctrl.GetAll)
// 	api.Post("/preview-query", ctrl.PreviewQuery)
// 	api.Get("/:id", ctrl.GetByID)
// 	api.Post("/", ctrl.Create)
// 	api.Put("/:id", ctrl.Update)
// 	api.Delete("/:id", ctrl.Delete)

// 	// Recipient management
// 	api.Get("/:id/recipients", ctrl.GetRecipients)
// 	api.Post("/:id/recipients", ctrl.AddRecipient)
// 	api.Delete("/:id/recipients/:recipientId", ctrl.RemoveRecipient)
// }

func SetupReportRoutes(app *fiber.App, db *gorm.DB, queryDB *gorm.DB) {
	ctrl := NewReportController(db, queryDB)

	api := app.Group("/api/v1/report-mailer/reports", middleware.AuthMiddleware)

	// Report CRUD
	api.Get("/", ctrl.GetAll)
	api.Post("/preview-query", ctrl.PreviewQuery)
	api.Get("/:id", ctrl.GetByID)
	api.Post("/", ctrl.Create)
	api.Put("/:id", ctrl.Update)
	api.Delete("/:id", ctrl.Delete)
	api.Put("/:id/email-template", ctrl.SaveEmailTemplate)

	// Recipient management
	api.Get("/:id/recipients", ctrl.GetRecipients)
	api.Post("/:id/recipients", ctrl.AddRecipient)
	api.Delete("/:id/recipients/:recipientId", ctrl.RemoveRecipient)
}
