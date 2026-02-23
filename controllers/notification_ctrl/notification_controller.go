package notification_ctrl

import (
	"encoding/json"
	"fiber-app/models/notification"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	notification_service "fiber-app/services/notification_service"
)

type NotificationController struct {
	DB *gorm.DB
}

func NewNotificationController(db *gorm.DB) *NotificationController {
	return &NotificationController{DB: db}
}

// ─── Email Notification CRUD ──────────────────────────────────────────────────

func (c *NotificationController) GetAll(ctx *fiber.Ctx) error {
	var notifs []notification.EmailNotification
	if err := c.DB.
		Preload("EmailConfig").
		Preload("Recipients").
		Order("created_at desc").
		Find(&notifs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": notifs})
}

func (c *NotificationController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}
	var notif notification.EmailNotification
	if err := c.DB.
		Preload("EmailConfig").
		Preload("Recipients").
		First(&notif, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Notifikasi tidak ditemukan"})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": notif})
}

func (c *NotificationController) Create(ctx *fiber.Ctx) error {
	var input struct {
		Name             string `json:"name" validate:"required"`
		EventKey         string `json:"event_key" validate:"required"`
		Description      string `json:"description"`
		EmailConfigID    uint   `json:"email_config_id" validate:"required"`
		EmailSubject     string `json:"email_subject" validate:"required"`
		EmailHeader      string `json:"email_header"`
		EmailBody        string `json:"email_body"`
		EmailFooter      string `json:"email_footer"`
		HeaderColor      string `json:"header_color"`
		IsActive         bool   `json:"is_active"`
		WithAttachment   bool   `json:"with_attachment"`
		AttachmentName   string `json:"attachment_name"`
		AttachmentFields string `json:"attachment_fields"`
		AttachmentSource string `json:"attachment_source"`
		AttachmentQuery  string `json:"attachment_query"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	notif := notification.EmailNotification{
		Name:             input.Name,
		EventKey:         input.EventKey,
		Description:      input.Description,
		EmailConfigID:    input.EmailConfigID,
		EmailSubject:     input.EmailSubject,
		EmailHeader:      input.EmailHeader,
		EmailBody:        input.EmailBody,
		EmailFooter:      input.EmailFooter,
		HeaderColor:      input.HeaderColor,
		IsActive:         input.IsActive,
		WithAttachment:   input.WithAttachment,
		AttachmentName:   input.AttachmentName,
		AttachmentFields: input.AttachmentFields,
		AttachmentSource: input.AttachmentSource,
		AttachmentQuery:  input.AttachmentQuery,
		CreatedBy:        int(ctx.Locals("userID").(float64)),
	}

	if notif.HeaderColor == "" {
		notif.HeaderColor = "#1E40AF"
	}

	if err := c.DB.Create(&notif).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": notif})
}

func (c *NotificationController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var notif notification.EmailNotification
	if err := c.DB.First(&notif, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Notifikasi tidak ditemukan"})
	}

	var input struct {
		Name             string `json:"name"`
		EventKey         string `json:"event_key"`
		Description      string `json:"description"`
		EmailConfigID    uint   `json:"email_config_id"`
		EmailSubject     string `json:"email_subject"`
		EmailHeader      string `json:"email_header"`
		EmailBody        string `json:"email_body"`
		EmailFooter      string `json:"email_footer"`
		HeaderColor      string `json:"header_color"`
		IsActive         bool   `json:"is_active"`
		WithAttachment   bool   `json:"with_attachment"`
		AttachmentName   string `json:"attachment_name"`
		AttachmentFields string `json:"attachment_fields"`
		AttachmentSource string `json:"attachment_source"`
		AttachmentQuery  string `json:"attachment_query"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Model(&notif).Updates(map[string]interface{}{
		"name":              input.Name,
		"event_key":         input.EventKey,
		"description":       input.Description,
		"email_config_id":   input.EmailConfigID,
		"email_subject":     input.EmailSubject,
		"email_header":      input.EmailHeader,
		"email_body":        input.EmailBody,
		"email_footer":      input.EmailFooter,
		"header_color":      input.HeaderColor,
		"is_active":         input.IsActive,
		"with_attachment":   input.WithAttachment,
		"attachment_name":   input.AttachmentName,
		"attachment_fields": input.AttachmentFields,
		"attachment_source": input.AttachmentSource,
		"attachment_query":  input.AttachmentQuery,
		"updated_by":        int(ctx.Locals("userID").(float64)),
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "message": "Notifikasi berhasil diupdate"})
}

func (c *NotificationController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	tx := c.DB.Begin()
	tx.Where("notification_id = ?", id).Delete(&notification.NotificationRecipient{})
	tx.Where("notification_id = ?", id).Delete(&notification.NotificationHistory{})
	if err := tx.Delete(&notification.EmailNotification{}, id).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	tx.Commit()
	return ctx.JSON(fiber.Map{"success": true, "message": "Notifikasi berhasil dihapus"})
}

// ─── Recipients ───────────────────────────────────────────────────────────────

func (c *NotificationController) GetRecipients(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))
	var recipients []notification.NotificationRecipient
	c.DB.Where("notification_id = ?", id).Find(&recipients)
	return ctx.JSON(fiber.Map{"success": true, "data": recipients})
}

func (c *NotificationController) AddRecipient(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))
	var input struct {
		Email string                     `json:"email" validate:"required,email"`
		Name  string                     `json:"name"`
		Type  notification.RecipientType `json:"type" validate:"required,oneof=TO CC"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Cek duplikat
	var count int64
	c.DB.Model(&notification.NotificationRecipient{}).
		Where("notification_id = ? AND email = ?", id, input.Email).
		Count(&count)
	if count > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email sudah terdaftar"})
	}

	recipient := notification.NotificationRecipient{
		NotificationID: uint(id),
		Email:          input.Email,
		Name:           input.Name,
		Type:           input.Type,
	}
	if err := c.DB.Create(&recipient).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": recipient})
}

func (c *NotificationController) RemoveRecipient(ctx *fiber.Ctx) error {
	recipientID, _ := strconv.Atoi(ctx.Params("recipientId"))
	if err := c.DB.Delete(&notification.NotificationRecipient{}, recipientID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "message": "Penerima berhasil dihapus"})
}

// ─── History ──────────────────────────────────────────────────────────────────

func (c *NotificationController) GetHistory(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 20)
	offset := (page - 1) * limit

	var histories []notification.NotificationHistory
	var total int64

	query := c.DB.Model(&notification.NotificationHistory{}).Where("notification_id = ?", id)
	if s := ctx.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}

	query.Count(&total)
	query.Order("sent_at desc").Limit(limit).Offset(offset).Find(&histories)

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

func (c *NotificationController) GetAllHistory(ctx *fiber.Ctx) error {
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 50)
	offset := (page - 1) * limit

	var histories []notification.NotificationHistory
	var total int64

	query := c.DB.Model(&notification.NotificationHistory{}).Preload("Notification")
	if s := ctx.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}
	if e := ctx.Query("event_key"); e != "" {
		query = query.Where("event_key = ?", e)
	}

	query.Count(&total)
	query.Order("sent_at desc").Limit(limit).Offset(offset).Find(&histories)

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

func (c *NotificationController) Retrigger(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	historyID, err := strconv.Atoi(ctx.Params("historyId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "History ID tidak valid"})
	}

	// Load notifikasi
	var notif notification.EmailNotification
	if err := c.DB.Preload("EmailConfig").Preload("Recipients").First(&notif, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Notifikasi tidak ditemukan"})
	}

	// Load history
	var history notification.NotificationHistory
	if err := c.DB.Where("id = ? AND notification_id = ?", historyID, id).First(&history).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "History tidak ditemukan"})
	}

	// Parse payload
	var eventData map[string]interface{}
	if history.Payload != "" {
		if err := json.Unmarshal([]byte(history.Payload), &eventData); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Gagal parse payload: " + err.Error(),
			})
		}
	} else {
		eventData = map[string]interface{}{}
	}

	// Kirim ulang secara async
	// go func() {
	// 	sendErr := sendOne(c.DB, notif, eventData)
	// 	logHistory(c.DB, notif.ID, history.EventKey, eventData, sendErr)
	// }()

	go notification_service.Retrigger(c.DB, notif, history.EventKey, eventData)

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Retrigger dimulai, cek history untuk hasilnya",
	})
}
