package report_builder

import (
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupReportBuilderRoutes(app *fiber.App, db *gorm.DB) {
	reportCtrl := NewReportController(db)
	layoutCtrl := NewLayoutController(db)
	generateCtrl := NewGenerateController(db)

	api := app.Group("/api/v1/report-builder", middleware.AuthMiddleware)

	// ── Reports (definisi report + fields) ───────────────────────────────
	api.Get("/reports", reportCtrl.GetAllReports) // ?type=QUERY|DOCUMENT
	api.Get("/reports/:id", reportCtrl.GetReportByID)
	api.Post("/reports", reportCtrl.CreateReport)
	api.Put("/reports/:id", reportCtrl.UpdateReport)

	// Fields
	api.Post("/reports/:id/fields", reportCtrl.AddField)
	api.Delete("/fields/:id", reportCtrl.DeleteField)

	// ── Layouts ───────────────────────────────────────────────────────────
	api.Get("/layouts", layoutCtrl.GetLayouts) // ?report_id=1&owner_code=OWN01
	api.Get("/layouts/:id", layoutCtrl.GetLayoutByID)
	api.Post("/layouts", layoutCtrl.SaveLayout)
	api.Put("/layouts/:id", layoutCtrl.UpdateLayout)
	api.Delete("/layouts/:id", layoutCtrl.DeleteLayout)

	// Document config (khusus tipe DOCUMENT)
	api.Put("/layouts/:id/document-config", layoutCtrl.SaveDocumentConfig)

	// ── Generate ──────────────────────────────────────────────────────────
	api.Post("/generate", generateCtrl.Generate)                  // excel | csv | pdf
	api.Post("/generate/document", generateCtrl.GenerateDocument) // picking list, spk, dll
	api.Post("/preview", generateCtrl.Preview)                    // preview 100 rows (JSON)
}
