package report_builder

import (
	"errors"
	"fiber-app/models/report_builder"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ReportController struct {
	DB *gorm.DB
}

func NewReportController(db *gorm.DB) *ReportController {
	return &ReportController{DB: db}
}

// GET /api/v1/report-builder/reports
func (c *ReportController) GetAllReports(ctx *fiber.Ctx) error {
	var reports []report_builder.RptReport

	query := c.DB.Preload("Fields", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	})

	if ctx.Query("type") != "" {
		query = query.Where("report_type = ?", ctx.Query("type"))
	}

	if err := query.Where("is_active = ?", true).Order("report_code ASC").Find(&reports).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Reports found", "data": reports,
	})
}

// GET /api/v1/report-builder/reports/:id
func (c *ReportController) GetReportByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var report report_builder.RptReport
	if err := c.DB.Preload("Fields", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&report, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Report not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Report found", "data": report,
	})
}

// POST /api/v1/report-builder/reports
func (c *ReportController) CreateReport(ctx *fiber.Ctx) error {
	type ReqCreateReport struct {
		ReportCode   string `json:"report_code" validate:"required,min=3,max=50"`
		ReportName   string `json:"report_name" validate:"required,min=3,max=100"`
		ReportType   string `json:"report_type" validate:"required,oneof=QUERY DOCUMENT"`
		BaseQuery    string `json:"base_query"`
		DocumentType string `json:"document_type"`
	}

	var input ReqCreateReport
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if input.BaseQuery != "" {
		if err := ValidateBaseQuery(input.BaseQuery); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false, "message": "Invalid base query: " + err.Error(),
			})
		}
	}

	// Cek duplikat report_code
	var existing report_builder.RptReport
	if err := c.DB.Where("report_code = ?", input.ReportCode).First(&existing).Error; err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Report code already exists"})
	}

	report := report_builder.RptReport{
		ReportCode:   input.ReportCode,
		ReportName:   input.ReportName,
		ReportType:   input.ReportType,
		BaseQuery:    input.BaseQuery,
		DocumentType: input.DocumentType,
		IsActive:     true,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
		CreatedAt:    time.Now(),
	}

	if err := c.DB.Create(&report).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Report created successfully", "data": report,
	})
}

// PUT /api/v1/report-builder/reports/:id
func (c *ReportController) UpdateReport(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var report report_builder.RptReport
	if err := c.DB.First(&report, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Report not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	type ReqUpdateReport struct {
		ReportName   string `json:"report_name" validate:"required,min=3,max=100"`
		BaseQuery    string `json:"base_query"`
		DocumentType string `json:"document_type"`
		IsActive     bool   `json:"is_active"`
	}

	var input ReqUpdateReport
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if input.BaseQuery != "" {
		if err := ValidateBaseQuery(input.BaseQuery); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false, "message": "Invalid base query: " + err.Error(),
			})
		}
	}

	if err := c.DB.Model(&report_builder.RptReport{}).Where("id = ?", id).Updates(map[string]interface{}{
		"report_name":   input.ReportName,
		"base_query":    input.BaseQuery,
		"document_type": input.DocumentType,
		"is_active":     input.IsActive,
		"updated_by":    int(ctx.Locals("userID").(float64)),
		"updated_at":    time.Now(),
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Report updated successfully",
	})
}

// ─── Fields ──────────────────────────────────────────────────────────────────

// POST /api/v1/report-builder/reports/:id/fields
func (c *ReportController) AddField(ctx *fiber.Ctx) error {
	reportID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid report ID"})
	}

	type ReqAddField struct {
		FieldKey     string `json:"field_key" validate:"required"`
		FieldLabel   string `json:"field_label" validate:"required"`
		FieldType    string `json:"field_type" validate:"required,oneof=STRING NUMBER DATE BOOLEAN"`
		IsFilterable bool   `json:"is_filterable"`
		IsSortable   bool   `json:"is_sortable"`
		SortOrder    int    `json:"sort_order"`
	}

	var input ReqAddField
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	field := report_builder.RptReportField{
		ReportID:     uint(reportID),
		FieldKey:     input.FieldKey,
		FieldLabel:   input.FieldLabel,
		FieldType:    input.FieldType,
		IsFilterable: input.IsFilterable,
		IsSortable:   input.IsSortable,
		SortOrder:    input.SortOrder,
	}

	if err := c.DB.Create(&field).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Field added successfully", "data": field,
	})
}

// DELETE /api/v1/report-builder/fields/:id
func (c *ReportController) DeleteField(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	if err := c.DB.Delete(&report_builder.RptReportField{}, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Field deleted successfully",
	})
}
