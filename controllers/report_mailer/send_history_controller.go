package report_mailer

import (
	"fiber-app/models/report_mailer"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type SendHistoryController struct {
	DB *gorm.DB
}

func NewSendHistoryController(db *gorm.DB) *SendHistoryController {
	return &SendHistoryController{DB: db}
}

// GET /api/v1/report-mailer/reports/:id/history
func (c *SendHistoryController) GetByReport(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	// Query params
	limit := 20
	if l := ctx.QueryInt("limit", 20); l > 0 && l <= 100 {
		limit = l
	}
	page := ctx.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	statusFilter := ctx.Query("status") // "success" / "failed" / "" (semua)

	query := c.DB.Model(&report_mailer.SendHistory{}).Where("report_id = ?", reportID)
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var total int64
	query.Count(&total)

	var histories []report_mailer.SendHistory
	if err := query.
		Order("sent_at desc").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    histories,
		"meta": fiber.Map{
			"total": total,
			"page":  page,
			"limit": limit,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GET /api/v1/report-mailer/history
// Semua history dari semua report (untuk dashboard global)
func (c *SendHistoryController) GetAll(ctx *fiber.Ctx) error {
	limit := 50
	if l := ctx.QueryInt("limit", 50); l > 0 && l <= 100 {
		limit = l
	}
	page := ctx.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	statusFilter := ctx.Query("status")

	query := c.DB.Model(&report_mailer.SendHistory{}).Preload("Report")
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var total int64
	query.Count(&total)

	var histories []report_mailer.SendHistory
	if err := query.
		Order("sent_at desc").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    histories,
		"meta": fiber.Map{
			"total": total,
			"page":  page,
			"limit": limit,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// DELETE /api/v1/report-mailer/reports/:id/history
// Hapus semua history untuk satu report
func (c *SendHistoryController) ClearByReport(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	if err := c.DB.Where("report_id = ?", reportID).Delete(&report_mailer.SendHistory{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "History berhasil dihapus"})
}
