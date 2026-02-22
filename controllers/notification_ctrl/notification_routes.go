package notification_ctrl

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupNotificationRoutes(app *fiber.App, db *gorm.DB) {
	ctrl := NewNotificationController(db)

	api := app.Group("/api/v1/notifications", middleware.AuthMiddleware)

	// CRUD notifikasi
	api.Get("/", ctrl.GetAll)
	api.Get("/:id", ctrl.GetByID)
	api.Post("/", ctrl.Create)
	api.Put("/:id", ctrl.Update)
	api.Delete("/:id", ctrl.Delete)

	// Recipients
	api.Get("/:id/recipients", ctrl.GetRecipients)
	api.Post("/:id/recipients", ctrl.AddRecipient)
	api.Delete("/:id/recipients/:recipientId", ctrl.RemoveRecipient)

	// History per notifikasi
	api.Get("/:id/history", ctrl.GetHistory)

	// History semua (global)
	api.Get("/history/all", ctrl.GetAllHistory)
}
