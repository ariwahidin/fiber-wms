package report_mailer

import (
	"fiber-app/models/report_mailer"
	rm_services "fiber-app/services/report_mailer"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ReportQueryController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
}

func NewReportQueryController(db *gorm.DB, queryDB *gorm.DB) *ReportQueryController {
	return &ReportQueryController{DB: db, QueryDB: queryDB}
}

// ─── Input Structs ────────────────────────────────────────────────────────────

var reportQueryInput struct {
	Name      string `json:"name" validate:"required,min=1"`
	Query     string `json:"query" validate:"required"`
	SortOrder int    `json:"sort_order"`
}

var outputModeInput struct {
	OutputMode report_mailer.OutputMode `json:"output_mode" validate:"required,oneof=single_file multi_file"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GET /api/v1/report-mailer/reports/:id/queries
func (c *ReportQueryController) GetAll(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var queries []report_mailer.ReportQuery
	if err := c.DB.
		Where("report_id = ?", reportID).
		Order("sort_order asc").
		Find(&queries).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": queries})
}

// POST /api/v1/report-mailer/reports/:id/queries
func (c *ReportQueryController) Create(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	// Cek report ada
	var report report_mailer.Report
	if err := c.DB.First(&report, reportID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
	}

	if err := ctx.BodyParser(&reportQueryInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(reportQueryInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi query aman
	if err := rm_services.ValidateSelectOnly(reportQueryInput.Query); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Query tidak aman: " + err.Error(),
			"success": false,
		})
	}

	// Auto sort_order: ambil max + 1
	var maxOrder int
	c.DB.Model(&report_mailer.ReportQuery{}).
		Where("report_id = ?", reportID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxOrder)

	rq := report_mailer.ReportQuery{
		ReportID:  uint(reportID),
		Name:      reportQueryInput.Name,
		Query:     reportQueryInput.Query,
		SortOrder: maxOrder + 1,
	}

	if err := c.DB.Create(&rq).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Query berhasil ditambahkan",
		"data":    rq,
	})
}

// PUT /api/v1/report-mailer/reports/:id/queries/:queryId
func (c *ReportQueryController) Update(ctx *fiber.Ctx) error {
	queryID, err := strconv.Atoi(ctx.Params("queryId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Query ID tidak valid"})
	}

	var rq report_mailer.ReportQuery
	if err := c.DB.First(&rq, queryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Query tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := ctx.BodyParser(&reportQueryInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(reportQueryInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi query aman
	if err := rm_services.ValidateSelectOnly(reportQueryInput.Query); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Query tidak aman: " + err.Error(),
			"success": false,
		})
	}

	rq.Name = reportQueryInput.Name
	rq.Query = reportQueryInput.Query
	rq.SortOrder = reportQueryInput.SortOrder

	if err := c.DB.Save(&rq).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Query berhasil diupdate",
		"data":    rq,
	})
}

// DELETE /api/v1/report-mailer/reports/:id/queries/:queryId
func (c *ReportQueryController) Delete(ctx *fiber.Ctx) error {
	queryID, err := strconv.Atoi(ctx.Params("queryId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Query ID tidak valid"})
	}

	var rq report_mailer.ReportQuery
	if err := c.DB.First(&rq, queryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Query tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Delete(&rq).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Query berhasil dihapus"})
}

// PUT /api/v1/report-mailer/reports/:id/queries/reorder
// Update sort_order semua query sekaligus
func (c *ReportQueryController) Reorder(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var input []struct {
		ID        uint `json:"id"`
		SortOrder int  `json:"sort_order"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	for _, item := range input {
		c.DB.Model(&report_mailer.ReportQuery{}).
			Where("id = ? AND report_id = ?", item.ID, reportID).
			Update("sort_order", item.SortOrder)
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Urutan berhasil disimpan"})
}

// PUT /api/v1/report-mailer/reports/:id/output-mode
func (c *ReportQueryController) SetOutputMode(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	if err := ctx.BodyParser(&outputModeInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(outputModeInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Model(&report_mailer.Report{}).
		Where("id = ?", reportID).
		Update("output_mode", outputModeInput.OutputMode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Output mode berhasil diupdate",
	})
}
