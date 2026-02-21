package report_mailer

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupReportQueryRoutes(app *fiber.App, db *gorm.DB, queryDB *gorm.DB) {
	ctrl := NewReportQueryController(db, queryDB)

	api := app.Group("/api/v1/report-mailer/reports", middleware.AuthMiddleware)

	api.Get("/:id/queries", ctrl.GetAll)
	api.Post("/:id/queries", ctrl.Create)
	api.Put("/:id/queries/reorder", ctrl.Reorder)
	api.Put("/:id/queries/:queryId", ctrl.Update)
	api.Delete("/:id/queries/:queryId", ctrl.Delete)
	api.Put("/:id/output-mode", ctrl.SetOutputMode)
}
