package rpt_builder_ctrl

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupRptBuilderRoutes(app *fiber.App, db *gorm.DB, readDB *gorm.DB) {
	templateCtrl := NewTemplateController(db)
	generateCtrl := NewGenerateController(db, readDB)

	api := app.Group("/api/v1/rpt-builder", middleware.AuthMiddleware)

	// ── Templates ─────────────────────────────────────────────────────────
	api.Get("/templates", templateCtrl.GetAll) // ?category=INBOUND&active_only=true
	api.Get("/templates/:id", templateCtrl.GetByID)
	api.Post("/templates", templateCtrl.Create)
	api.Put("/templates/:id", templateCtrl.Update)
	api.Delete("/templates/:id", templateCtrl.Delete)

	// ── Sheets ────────────────────────────────────────────────────────────
	api.Post("/templates/:id/sheets", templateCtrl.AddSheet)
	api.Put("/sheets/:id", templateCtrl.UpdateSheet)
	api.Delete("/sheets/:id", templateCtrl.DeleteSheet)

	// ── Params ────────────────────────────────────────────────────────────
	api.Post("/templates/:id/params", templateCtrl.AddParam)
	api.Delete("/params/:id", templateCtrl.DeleteParam)

	// ── Generate ──────────────────────────────────────────────────────────
	api.Post("/rpt-preview", generateCtrl.Preview)
	api.Post("/download/check", generateCtrl.CheckDownload)
	api.Post("/download", generateCtrl.Download)

	// ── Column Preferences ────────────────────────────────────────────────
	api.Put("/column-prefs", generateCtrl.UpdateColumnPrefs)

	api.Post("/resolve-options", generateCtrl.ResolveOptions)
}
