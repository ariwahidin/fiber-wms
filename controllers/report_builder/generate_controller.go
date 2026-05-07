package report_builder

import (
	"errors"
	"fiber-app/models/report_builder"
	"fmt"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type GenerateController struct {
	DB *gorm.DB
}

func NewGenerateController(db *gorm.DB) *GenerateController {
	return &GenerateController{DB: db}
}

func (c *GenerateController) Generate(ctx *fiber.Ctx) error {
	var input report_builder.ReqGenerateReport
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Load layout beserta semua relasinya
	var layout report_builder.RptLayout
	if err := c.DB.
		Preload("Report").
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC").Preload("Field")
		}).
		Preload("Filters", func(db *gorm.DB) *gorm.DB {
			return db.Preload("Field")
		}).
		First(&layout, input.LayoutID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Layout not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Validasi required filters
	for _, f := range layout.Filters {
		if f.IsRequired {
			if val, ok := input.Filters[f.Field.FieldKey]; !ok || val == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Filter '%s' is required", f.Field.FieldLabel),
				})
			}
		}
	}

	// Build query
	query, args, err := BuildQuery(layout.Report.BaseQuery, layout.Filters, input.Filters)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Execute query → []map[string]interface{}
	var rawRows []map[string]interface{}
	if err := c.DB.Raw(query, args...).Scan(&rawRows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Query execution failed: " + err.Error(),
		})
	}

	if rawRows == nil {
		rawRows = []map[string]interface{}{}
	}

	filename := fmt.Sprintf("%s_%s", layout.Report.ReportCode, time.Now().Format("20060102_150405"))

	switch input.Format {
	case "excel":
		return c.respondExcel(ctx, layout, rawRows, filename)
	case "csv":
		return c.respondCSV(ctx, layout, rawRows, filename)
	case "pdf":
		// PDF via HTML → browser print (lihat GenerateDocumentHTML untuk pattern serupa)
		return ctx.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"success": false, "message": "PDF direct generation coming soon. Use 'html' format then browser print.",
		})
	default:
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid format. Use: excel, csv, pdf",
		})
	}
}

func (c *GenerateController) respondExcel(ctx *fiber.Ctx, layout report_builder.RptLayout, rows []map[string]interface{}, filename string) error {
	f, err := GenerateExcel(layout, rows)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, filename))

	buf, err := f.WriteToBuffer()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Send(buf.Bytes())
}

func (c *GenerateController) respondCSV(ctx *fiber.Ctx, layout report_builder.RptLayout, rows []map[string]interface{}, filename string) error {
	csv := GenerateCSV(layout, rows)

	ctx.Set("Content-Type", "text/csv; charset=utf-8")
	ctx.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, filename))

	// BOM untuk Excel agar UTF-8 terbaca benar
	return ctx.SendString("\xEF\xBB\xBF" + csv)
}

// ─── Document Printing ───────────────────────────────────────────────────────

// POST /api/v1/report-builder/generate/document
//
// Body:
//
//	{
//	  "layout_id": 2,
//	  "params": {
//	    "outbound_id": "123"
//	  }
//	}
func (c *GenerateController) GenerateDocument(ctx *fiber.Ctx) error {
	type ReqGenerateDoc struct {
		LayoutID uint              `json:"layout_id" validate:"required"`
		Params   map[string]string `json:"params"` // named params dari base_query
	}

	var input ReqGenerateDoc
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Load layout
	var layout report_builder.RptLayout
	if err := c.DB.
		Preload("Report").
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC").Preload("Field")
		}).
		Preload("DocumentConfig").
		First(&layout, input.LayoutID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Layout not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if layout.Report.ReportType != "DOCUMENT" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "This layout is not a document type",
		})
	}

	// Build query dengan named params
	// query, args := BuildQueryForDocument(layout.Report.BaseQuery, input.Params)

	query, args, err := BuildQueryForDocument(layout.Report.BaseQuery, input.Params)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid document query: " + err.Error(),
		})
	}

	var rawRows []map[string]interface{}
	if err := c.DB.Raw(query, args...).Scan(&rawRows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Query execution failed: " + err.Error(),
		})
	}

	if rawRows == nil {
		rawRows = []map[string]interface{}{}
	}

	// Return JSON — frontend yang handle render HTML / print
	// Ini pattern yang paling fleksibel karena layout bisa di-custom per client
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Document data ready",
		"data": fiber.Map{
			"layout":   layout,
			"rows":     rawRows,
			"doc_type": layout.Report.DocumentType,
			"params":   input.Params,
		},
	})
}

// POST /api/v1/report-builder/preview
// Preview data tanpa download — return JSON rows (untuk frontend table preview)
func (c *GenerateController) Preview(ctx *fiber.Ctx) error {
	var input report_builder.ReqGenerateReport
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Paksa format ke preview — tidak perlu validasi format
	input.Format = "preview"

	var layout report_builder.RptLayout
	if err := c.DB.
		Preload("Report").
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC").Preload("Field")
		}).
		Preload("Filters", func(db *gorm.DB) *gorm.DB {
			return db.Preload("Field")
		}).
		First(&layout, input.LayoutID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Layout not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	query, args, err := BuildQuery(layout.Report.BaseQuery, layout.Filters, input.Filters)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Limit preview ke 100 rows
	previewQuery := fmt.Sprintf("SELECT TOP 100 * FROM (%s) AS _preview", query)

	var rawRows []map[string]interface{}
	if err := c.DB.Raw(previewQuery, args...).Scan(&rawRows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Query execution failed: " + err.Error(),
		})
	}

	if rawRows == nil {
		rawRows = []map[string]interface{}{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Preview: %d rows (max 100)", len(rawRows)),
		"data": fiber.Map{
			"columns": layout.Columns,
			"rows":    rawRows,
		},
	})
}
