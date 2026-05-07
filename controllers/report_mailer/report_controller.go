package report_mailer

import (
	"fiber-app/models/report_mailer"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ReportController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB // read-only DB khusus eksekusi query report
}

func NewReportController(db *gorm.DB, queryDB *gorm.DB) *ReportController {
	return &ReportController{DB: db, QueryDB: queryDB}
}

// ─── Input Structs ────────────────────────────────────────────────────────────

var reportInput struct {
	Name        string `json:"name" validate:"required,min=3"`
	Description string `json:"description"`
	// Query         string `json:"query" validate:"required"`
	EmailConfigID uint   `json:"email_config_id" validate:"required"`
	ExcelTitle    string `json:"excel_title" validate:"required"`
	ExcelSubtitle string `json:"excel_subtitle"`
	IsActive      bool   `json:"is_active"`
}

var recipientInput struct {
	Email string                      `json:"email" validate:"required,email"`
	Name  string                      `json:"name"`
	Type  report_mailer.RecipientType `json:"type" validate:"required,oneof=TO CC"`
}

// ─── Report CRUD ──────────────────────────────────────────────────────────────

// GET /api/v1/report-mailer/reports
func (c *ReportController) GetAll(ctx *fiber.Ctx) error {
	var reports []report_mailer.Report
	if err := c.DB.
		Preload("Queries", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc")
		}).
		Preload("EmailConfig").
		Preload("Recipients").
		Preload("Schedule").
		Order("created_at desc").
		Find(&reports).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": reports})
}

// GET /api/v1/report-mailer/reports/:id
func (c *ReportController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var report report_mailer.Report
	if err := c.DB.
		Preload("Queries", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc")
		}).
		Preload("EmailConfig").
		Preload("Recipients").
		Preload("Schedule").
		First(&report, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": report})
}

// POST /api/v1/report-mailer/reports
func (c *ReportController) Create(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&reportInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(reportInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi email config ada
	var emailConfig report_mailer.EmailConfig
	if err := c.DB.First(&emailConfig, reportInput.EmailConfigID).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
	}

	userID := int(ctx.Locals("userID").(float64))

	report := report_mailer.Report{
		Name:          reportInput.Name,
		Description:   reportInput.Description,
		EmailConfigID: reportInput.EmailConfigID,
		ExcelTitle:    reportInput.ExcelTitle,
		ExcelSubtitle: reportInput.ExcelSubtitle,
		IsActive:      reportInput.IsActive,
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}

	if err := c.DB.Create(&report).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Report berhasil dibuat",
		"data":    report,
	})
}

// PUT /api/v1/report-mailer/reports/:id
func (c *ReportController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var report report_mailer.Report
	if err := c.DB.First(&report, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := ctx.BodyParser(&reportInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(reportInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi email config ada
	var emailConfig report_mailer.EmailConfig
	if err := c.DB.First(&emailConfig, reportInput.EmailConfigID).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
	}

	userID := int(ctx.Locals("userID").(float64))

	report.Name = reportInput.Name
	report.Description = reportInput.Description
	report.EmailConfigID = reportInput.EmailConfigID
	report.ExcelTitle = reportInput.ExcelTitle
	report.ExcelSubtitle = reportInput.ExcelSubtitle
	report.IsActive = reportInput.IsActive
	report.UpdatedBy = userID

	if err := c.DB.Save(&report).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Report berhasil diupdate",
		"data":    report,
	})
}

// DELETE /api/v1/report-mailer/reports/:id
func (c *ReportController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var report report_mailer.Report
	if err := c.DB.First(&report, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hapus semua data terkait dalam transaksi
	tx := c.DB.Begin()

	if err := tx.Where("report_id = ?", id).Delete(&report_mailer.ReportRecipient{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Where("report_id = ?", id).Delete(&report_mailer.ReportSchedule{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Where("report_id = ?", id).Delete(&report_mailer.SendHistory{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Delete(&report).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tx.Commit()

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Report berhasil dihapus",
	})
}

// ─── Recipient Management ─────────────────────────────────────────────────────

// GET /api/v1/report-mailer/reports/:id/recipients
func (c *ReportController) GetRecipients(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var recipients []report_mailer.ReportRecipient
	if err := c.DB.Where("report_id = ?", id).Find(&recipients).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": recipients})
}

// POST /api/v1/report-mailer/reports/:id/recipients
func (c *ReportController) AddRecipient(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	// Cek report ada
	var report report_mailer.Report
	if err := c.DB.First(&report, reportID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
	}

	if err := ctx.BodyParser(&recipientInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(recipientInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Cek duplikat email di report yang sama
	var existing report_mailer.ReportRecipient
	result := c.DB.Where("report_id = ? AND email = ?", reportID, recipientInput.Email).First(&existing)
	if result.Error == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email ini sudah terdaftar di report ini"})
	}

	recipient := report_mailer.ReportRecipient{
		ReportID: uint(reportID),
		Email:    recipientInput.Email,
		Name:     recipientInput.Name,
		Type:     recipientInput.Type,
	}

	if err := c.DB.Create(&recipient).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Recipient berhasil ditambahkan",
		"data":    recipient,
	})
}

// DELETE /api/v1/report-mailer/reports/:id/recipients/:recipientId
func (c *ReportController) RemoveRecipient(ctx *fiber.Ctx) error {
	recipientID, err := strconv.Atoi(ctx.Params("recipientId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Recipient ID tidak valid"})
	}

	var recipient report_mailer.ReportRecipient
	if err := c.DB.First(&recipient, recipientID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Recipient tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Delete(&recipient).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Recipient berhasil dihapus",
	})
}
