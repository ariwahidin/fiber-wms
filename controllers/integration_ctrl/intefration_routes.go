package integration_ctrl

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupIntegrationRoutes(app *fiber.App, db *gorm.DB, queryDB *gorm.DB) {
	ctrl := NewIntegrationController(db, queryDB)

	api := app.Group("/api/v1/integrations", middleware.AuthMiddleware)

	// CRUD
	api.Get("/", ctrl.GetAll)
	api.Get("/:id", ctrl.GetByID)
	api.Post("/", ctrl.Create)
	api.Put("/:id", ctrl.Update)
	api.Delete("/:id", ctrl.Delete)

	// Connection
	api.Put("/:id/connection", ctrl.SaveConnection)

	// Recipients
	api.Post("/:id/recipients", ctrl.AddRecipient)
	api.Delete("/:id/recipients/:recipientId", ctrl.RemoveRecipient)

	// History
	api.Get("/:id/history", ctrl.GetHistory)

	// Test Run
	api.Post("/:id/test-run", ctrl.TestRun)

	api.Post("/:id/history/:historyId/retrigger", ctrl.Retrigger)
}
