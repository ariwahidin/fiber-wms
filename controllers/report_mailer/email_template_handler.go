package report_mailer

import (
	"fiber-app/models/report_mailer"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Input struct untuk email template
var emailTemplateInput struct {
	EmailSubject string `json:"email_subject"`
	EmailHeader  string `json:"email_header"`
	EmailBody    string `json:"email_body"`
	EmailFooter  string `json:"email_footer"`
	HeaderColor  string `json:"header_color"`
}

// PUT /api/v1/report-mailer/reports/:id/email-template
func (c *ReportController) SaveEmailTemplate(ctx *fiber.Ctx) error {
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

	if err := ctx.BodyParser(&emailTemplateInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Model(&report).Updates(map[string]interface{}{
		"email_subject": emailTemplateInput.EmailSubject,
		"email_header":  emailTemplateInput.EmailHeader,
		"email_body":    emailTemplateInput.EmailBody,
		"email_footer":  emailTemplateInput.EmailFooter,
		"header_color":  emailTemplateInput.HeaderColor,
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Email template berhasil disimpan",
	})
}
