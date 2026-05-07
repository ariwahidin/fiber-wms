package report_mailer

import (
	"fiber-app/models/report_mailer"
	rm_services "fiber-app/services/report_mailer"
	"strconv"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ScheduleController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB // ← tambah
}

func NewScheduleController(db *gorm.DB, queryDB *gorm.DB) *ScheduleController {
	return &ScheduleController{DB: db, QueryDB: queryDB}
}

// ─── Input Struct ─────────────────────────────────────────────────────────────

var scheduleInput struct {
	Frequency  report_mailer.ScheduleFrequency `json:"frequency" validate:"required,oneof=daily weekly monthly"`
	DayOfWeek  *int                            `json:"day_of_week"`  // 0-6, wajib kalau weekly
	DayOfMonth *int                            `json:"day_of_month"` // 1-31, wajib kalau monthly
	Hour       int                             `json:"hour" validate:"min=0,max=23"`
	Minute     int                             `json:"minute" validate:"min=0,max=59"`
	IsActive   bool                            `json:"is_active"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GET /api/v1/report-mailer/reports/:id/schedule
func (c *ScheduleController) GetSchedule(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var schedule report_mailer.ReportSchedule
	if err := c.DB.Where("report_id = ?", reportID).First(&schedule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.JSON(fiber.Map{"success": true, "data": nil})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": schedule})
}

// POST /api/v1/report-mailer/reports/:id/schedule
// Upsert: buat baru kalau belum ada, update kalau sudah ada
func (c *ScheduleController) SaveSchedule(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	// Cek report ada
	var report report_mailer.Report
	if err := c.DB.First(&report, reportID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Report tidak ditemukan"})
	}

	if err := ctx.BodyParser(&scheduleInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(scheduleInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi field tambahan sesuai frequency
	if scheduleInput.Frequency == report_mailer.FrequencyWeekly && scheduleInput.DayOfWeek == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "day_of_week wajib diisi untuk schedule weekly"})
	}
	if scheduleInput.Frequency == report_mailer.FrequencyMonthly && scheduleInput.DayOfMonth == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "day_of_month wajib diisi untuk schedule monthly"})
	}

	// Hitung next_run_at
	nextRun := calcNextRun(scheduleInput.Frequency, scheduleInput.DayOfWeek, scheduleInput.DayOfMonth, scheduleInput.Hour, scheduleInput.Minute)

	// Cek apakah schedule sudah ada (upsert)
	var existing report_mailer.ReportSchedule
	isNew := false
	if err := c.DB.Where("report_id = ?", reportID).First(&existing).Error; err != nil {
		isNew = true
	}

	if isNew {
		existing = report_mailer.ReportSchedule{
			ReportID: uint(reportID),
		}
	}

	existing.Frequency = scheduleInput.Frequency
	existing.DayOfWeek = scheduleInput.DayOfWeek
	existing.DayOfMonth = scheduleInput.DayOfMonth
	existing.Hour = scheduleInput.Hour
	existing.Minute = scheduleInput.Minute
	existing.IsActive = scheduleInput.IsActive
	existing.NextRunAt = &nextRun

	if isNew {
		if err := c.DB.Create(&existing).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if err := c.DB.Save(&existing).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Reload scheduler untuk report ini
	if rm_services.Manager != nil {
		rm_services.Manager.ReloadReport(uint(reportID))
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Schedule berhasil disimpan",
		"data":    existing,
	})
}

// DELETE /api/v1/report-mailer/reports/:id/schedule
func (c *ScheduleController) DeleteSchedule(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	if err := c.DB.Where("report_id = ?", reportID).Delete(&report_mailer.ReportSchedule{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Unregister dari scheduler
	if rm_services.Manager != nil {
		rm_services.Manager.ReloadReport(uint(reportID))
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Schedule berhasil dihapus"})
}

// POST /api/v1/report-mailer/reports/:id/send-now
func (c *ScheduleController) SendNow(ctx *fiber.Ctx) error {
	reportID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	// if err := rm_services.TriggerNow(c.DB, uint(reportID)); err != nil {
	if err := rm_services.TriggerNow(c.DB, c.QueryDB, uint(reportID)); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Report berhasil dikirim",
	})
}

// POST /api/v1/report-mailer/scheduler/reload
// Reload semua schedule (admin utility)
func (c *ScheduleController) ReloadAll(ctx *fiber.Ctx) error {
	if rm_services.Manager != nil {
		rm_services.Manager.ReloadAll()
	}
	return ctx.JSON(fiber.Map{"success": true, "message": "Scheduler berhasil di-reload"})
}

// ─── Helper: hitung next_run_at ───────────────────────────────────────────────

func calcNextRun(
	freq report_mailer.ScheduleFrequency,
	dayOfWeek *int,
	dayOfMonth *int,
	hour int,
	minute int,
) time.Time {
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())

	switch freq {
	case report_mailer.FrequencyDaily:
		if target.Before(now) {
			target = target.Add(24 * time.Hour)
		}

	case report_mailer.FrequencyWeekly:
		if dayOfWeek == nil {
			return target
		}
		daysUntil := (*dayOfWeek - int(now.Weekday()) + 7) % 7
		target = target.AddDate(0, 0, daysUntil)
		if target.Before(now) {
			target = target.AddDate(0, 0, 7)
		}

	case report_mailer.FrequencyMonthly:
		if dayOfMonth == nil {
			return target
		}
		target = time.Date(now.Year(), now.Month(), *dayOfMonth, hour, minute, 0, 0, now.Location())
		if target.Before(now) {
			target = time.Date(now.Year(), now.Month()+1, *dayOfMonth, hour, minute, 0, 0, now.Location())
		}
	}

	return target
}
