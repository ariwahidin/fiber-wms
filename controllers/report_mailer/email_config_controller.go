package report_mailer

import (
	"crypto/tls"
	"fiber-app/models/report_mailer"
	"fmt"
	"net/smtp"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type EmailConfigController struct {
	DB *gorm.DB
}

func NewEmailConfigController(db *gorm.DB) *EmailConfigController {
	return &EmailConfigController{DB: db}
}

// ─── Input Structs ────────────────────────────────────────────────────────────

var emailConfigInput struct {
	Name      string `json:"name" validate:"required,min=3"`
	Host      string `json:"host" validate:"required"`
	Port      int    `json:"port" validate:"required"`
	Username  string `json:"username" validate:"required"`
	Password  string `json:"password" validate:"required"`
	FromName  string `json:"from_name" validate:"required"`
	FromEmail string `json:"from_email" validate:"required,email"`
	UseTLS    bool   `json:"use_tls"`
	IsActive  bool   `json:"is_active"`
}

var testEmailInput struct {
	To string `json:"to" validate:"required,email"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GET /api/v1/report-mailer/email-configs
func (c *EmailConfigController) GetAll(ctx *fiber.Ctx) error {
	var configs []report_mailer.EmailConfig
	if err := c.DB.Order("created_at desc").Find(&configs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Sembunyikan password dari response
	for i := range configs {
		configs[i].Password = "********"
	}

	return ctx.JSON(fiber.Map{"success": true, "data": configs})
}

// GET /api/v1/report-mailer/email-configs/:id
func (c *EmailConfigController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var config report_mailer.EmailConfig
	if err := c.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	config.Password = "********"
	return ctx.JSON(fiber.Map{"success": true, "data": config})
}

// POST /api/v1/report-mailer/email-configs
func (c *EmailConfigController) Create(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&emailConfigInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(emailConfigInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))

	config := report_mailer.EmailConfig{
		Name:      emailConfigInput.Name,
		Host:      emailConfigInput.Host,
		Port:      emailConfigInput.Port,
		Username:  emailConfigInput.Username,
		Password:  emailConfigInput.Password,
		FromName:  emailConfigInput.FromName,
		FromEmail: emailConfigInput.FromEmail,
		UseTLS:    emailConfigInput.UseTLS,
		IsActive:  emailConfigInput.IsActive,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	if err := c.DB.Create(&config).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	config.Password = "********"
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Email config berhasil dibuat",
		"data":    config,
	})
}

// PUT /api/v1/report-mailer/email-configs/:id
func (c *EmailConfigController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var config report_mailer.EmailConfig
	if err := c.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := ctx.BodyParser(&emailConfigInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(emailConfigInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Kalau password dikirim "********" artinya tidak diubah, pakai yang lama
	password := emailConfigInput.Password
	if password == "********" {
		password = config.Password
	}

	config.Name = emailConfigInput.Name
	config.Host = emailConfigInput.Host
	config.Port = emailConfigInput.Port
	config.Username = emailConfigInput.Username
	config.Password = password
	config.FromName = emailConfigInput.FromName
	config.FromEmail = emailConfigInput.FromEmail
	config.UseTLS = emailConfigInput.UseTLS
	config.IsActive = emailConfigInput.IsActive
	config.UpdatedBy = userID

	if err := c.DB.Save(&config).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	config.Password = "********"
	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Email config berhasil diupdate",
		"data":    config,
	})
}

// DELETE /api/v1/report-mailer/email-configs/:id
func (c *EmailConfigController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var config report_mailer.EmailConfig
	if err := c.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Cek apakah config ini sedang dipakai oleh report
	var reportCount int64
	c.DB.Model(&report_mailer.Report{}).Where("email_config_id = ?", id).Count(&reportCount)
	if reportCount > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Config ini dipakai oleh %d report, hapus atau pindahkan report terlebih dahulu", reportCount),
		})
	}

	if err := c.DB.Delete(&config).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Email config berhasil dihapus",
	})
}

// POST /api/v1/report-mailer/email-configs/:id/test
func (c *EmailConfigController) TestSend(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var config report_mailer.EmailConfig
	if err := c.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Email config tidak ditemukan"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := ctx.BodyParser(&testEmailInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(testEmailInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := sendTestEmail(config, testEmailInput.To); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Gagal kirim email: %s", err.Error()),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Test email berhasil dikirim ke %s", testEmailInput.To),
	})
}

// ─── Helper: kirim test email ─────────────────────────────────────────────────

func sendTestEmail(config report_mailer.EmailConfig, to string) error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	subject := "Test Email - WMS Report Mailer"
	body := fmt.Sprintf(
		"Halo,\r\n\r\nIni adalah test email dari WMS Report Mailer.\r\nKonfigurasi SMTP \"%s\" berhasil terhubung.\r\n\r\nTerima kasih.",
		config.Name,
	)

	msg := []byte(fmt.Sprintf(
		"From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		config.FromName, config.FromEmail, to, subject, body,
	))

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         config.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, config.Host)
		if err != nil {
			return err
		}
		defer client.Close()

		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(config.FromEmail); err != nil {
			return err
		}
		if err = client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(msg)
		if err != nil {
			return err
		}
		return w.Close()
	}

	return smtp.SendMail(addr, auth, config.FromEmail, []string{to}, msg)
}
