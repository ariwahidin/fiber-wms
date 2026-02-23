package integration_ctrl

import (
	"encoding/json"
	"fiber-app/models/integration"
	integration_service "fiber-app/services/integration_service"
	"io"
	"strconv"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type IntegrationController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
}

func NewIntegrationController(db *gorm.DB, queryDB *gorm.DB) *IntegrationController {
	return &IntegrationController{DB: db, QueryDB: queryDB}
}

// ─── CRUD Integration ─────────────────────────────────────────────────────────

func (c *IntegrationController) GetAll(ctx *fiber.Ctx) error {
	var integrations []integration.Integration
	if err := c.DB.
		Preload("Connection").
		Preload("Recipients").
		Order("created_at desc").
		Find(&integrations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": integrations})
}

func (c *IntegrationController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}
	var intg integration.Integration
	if err := c.DB.
		Preload("Connection").
		Preload("Recipients").
		First(&intg, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integrasi tidak ditemukan"})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": intg})
}

func (c *IntegrationController) Create(ctx *fiber.Ctx) error {
	var input struct {
		Name               string                  `json:"name" validate:"required"`
		EventKey           string                  `json:"event_key" validate:"required"`
		Description        string                  `json:"description"`
		ChannelType        integration.ChannelType `json:"channel_type" validate:"required,oneof=sftp ftp api file"`
		FileFormat         integration.FileFormat  `json:"file_format"`
		SourceType         integration.SourceType  `json:"source_type" validate:"required,oneof=query event"`
		Query              string                  `json:"query"`
		FilenamePattern    string                  `json:"filename_pattern"`
		Timing             integration.Timing      `json:"timing" validate:"required,oneof=realtime scheduled"`
		ScheduleFreq       string                  `json:"schedule_freq"`
		ScheduleDayOfWeek  *int                    `json:"schedule_day_of_week"`
		ScheduleDayOfMonth *int                    `json:"schedule_day_of_month"`
		ScheduleHour       int                     `json:"schedule_hour"`
		ScheduleMinute     int                     `json:"schedule_minute"`
		// EmailConfigID      *uint                   `json:"email_config_id"`
		EmailConfigID      *uint  `json:"email_config_id,omitempty"`
		NotifyOnSuccess    bool   `json:"notify_on_success"`
		NotifyOnFailure    bool   `json:"notify_on_failure"`
		NotifyEmailSubject string `json:"notify_email_subject"`
		NotifyEmailBody    string `json:"notify_email_body"`
		IsActive           bool   `json:"is_active"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	intg := integration.Integration{
		Name:               input.Name,
		EventKey:           input.EventKey,
		Description:        input.Description,
		ChannelType:        input.ChannelType,
		FileFormat:         input.FileFormat,
		SourceType:         input.SourceType,
		Query:              input.Query,
		FilenamePattern:    input.FilenamePattern,
		Timing:             input.Timing,
		ScheduleFreq:       input.ScheduleFreq,
		ScheduleDayOfWeek:  input.ScheduleDayOfWeek,
		ScheduleDayOfMonth: input.ScheduleDayOfMonth,
		ScheduleHour:       input.ScheduleHour,
		ScheduleMinute:     input.ScheduleMinute,
		EmailConfigID:      input.EmailConfigID,
		NotifyOnSuccess:    input.NotifyOnSuccess,
		NotifyOnFailure:    input.NotifyOnFailure,
		NotifyEmailSubject: input.NotifyEmailSubject,
		NotifyEmailBody:    input.NotifyEmailBody,
		IsActive:           input.IsActive,
		CreatedBy:          int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&intg).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": intg})
}

func (c *IntegrationController) Update(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var intg integration.Integration
	if err := c.DB.First(&intg, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integrasi tidak ditemukan"})
	}

	if err := ctx.BodyParser(&intg); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	intg.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Save(&intg).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "message": "Integrasi berhasil diupdate"})
}

func (c *IntegrationController) Delete(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	tx := c.DB.Begin()
	tx.Where("integration_id = ?", id).Delete(&integration.IntegrationConnection{})
	tx.Where("integration_id = ?", id).Delete(&integration.IntegrationRecipient{})
	tx.Where("integration_id = ?", id).Delete(&integration.IntegrationHistory{})
	if err := tx.Delete(&integration.Integration{}, id).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	tx.Commit()
	return ctx.JSON(fiber.Map{"success": true, "message": "Integrasi berhasil dihapus"})
}

// ─── Connection ───────────────────────────────────────────────────────────────

func (c *IntegrationController) SaveConnection(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))

	var conn integration.IntegrationConnection
	isNew := false
	if err := c.DB.Where("integration_id = ?", id).First(&conn).Error; err != nil {
		isNew = true
		conn.IntegrationID = uint(id)
	}

	if err := ctx.BodyParser(&conn); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	conn.IntegrationID = uint(id)

	if isNew {
		c.DB.Create(&conn)
	} else {
		c.DB.Save(&conn)
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Connection berhasil disimpan", "data": conn})
}

// ─── Recipients ───────────────────────────────────────────────────────────────

func (c *IntegrationController) AddRecipient(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))
	var input struct {
		Email string                               `json:"email" validate:"required,email"`
		Name  string                               `json:"name"`
		Type  integration.IntegrationRecipientType `json:"type" validate:"required,oneof=TO CC"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var count int64
	c.DB.Model(&integration.IntegrationRecipient{}).
		Where("integration_id = ? AND email = ?", id, input.Email).Count(&count)
	if count > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email sudah terdaftar"})
	}

	r := integration.IntegrationRecipient{
		IntegrationID: uint(id),
		Email:         input.Email,
		Name:          input.Name,
		Type:          input.Type,
	}
	c.DB.Create(&r)
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": r})
}

func (c *IntegrationController) RemoveRecipient(ctx *fiber.Ctx) error {
	rid, _ := strconv.Atoi(ctx.Params("recipientId"))
	c.DB.Delete(&integration.IntegrationRecipient{}, rid)
	return ctx.JSON(fiber.Map{"success": true, "message": "Penerima berhasil dihapus"})
}

// ─── History ──────────────────────────────────────────────────────────────────

func (c *IntegrationController) GetHistory(ctx *fiber.Ctx) error {
	id, _ := strconv.Atoi(ctx.Params("id"))
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 20)
	offset := (page - 1) * limit

	var histories []integration.IntegrationHistory
	var total int64

	query := c.DB.Model(&integration.IntegrationHistory{}).Where("integration_id = ?", id)
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

// ─── Test Run ─────────────────────────────────────────────────────────────────

func (c *IntegrationController) TestRun(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	var intg integration.Integration
	if err := c.DB.Preload("Connection").Preload("Recipients").First(&intg, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integrasi tidak ditemukan"})
	}

	// Test dengan data dummy
	testData := map[string]interface{}{
		"test":     "true",
		"year":     "2026",
		"date":     "20260101",
		"datetime": "2026-01-01 08:00:00",
	}

	go func() {
		err := integration_service.RunIntegrationPublic(c.DB, c.QueryDB, intg, testData, "manual_test")
		integration_service.LogHistoryPublic(c.DB, intg, intg.EventKey, err, "manual_test", testData)
	}()

	return ctx.JSON(fiber.Map{"success": true, "message": "Test run dimulai, cek history untuk hasilnya"})
}

// POST /api/v1/integrations/:id/history/:historyId/retrigger
func (c *IntegrationController) Retrigger(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	historyID, err := strconv.Atoi(ctx.Params("historyId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "History ID tidak valid"})
	}

	// Load integrasi
	var intg integration.Integration
	if err := c.DB.Preload("Connection").Preload("Recipients").First(&intg, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integrasi tidak ditemukan"})
	}

	// Load history
	var history integration.IntegrationHistory
	if err := c.DB.Where("id = ? AND integration_id = ?", historyID, id).First(&history).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "History tidak ditemukan"})
	}

	// Parse payload dari history
	var eventData map[string]interface{}
	if history.PayloadSummary != "" {
		if err := json.Unmarshal([]byte(history.PayloadSummary), &eventData); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Gagal parse payload dari history: " + err.Error(),
			})
		}
	} else {
		// Payload kosong — pakai data minimal
		eventData = map[string]interface{}{}
	}

	// Jalankan ulang secara async
	go func() {
		err := integration_service.RunIntegrationPublic(c.DB, c.QueryDB, intg, eventData, "retrigger")
		integration_service.LogHistoryPublic(c.DB, intg, history.EventKey, err, "retrigger", eventData)
		integration_service.SendNotificationPublic(c.DB, intg, eventData, err)
	}()

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Retrigger dimulai, cek history untuk hasilnya",
	})
}

// ─── Push Endpoint (eksternal POST ke WMS) ────────────────────────────────────

// POST /api/v1/integrations/inbound/:eventKey
// Endpoint ini dipanggil oleh sistem eksternal (SAP, Shopee, dll)
// Tidak perlu auth middleware — atau pakai API key tersendiri
func (c *IntegrationController) InboundPush(ctx *fiber.Ctx) error {
	eventKey := ctx.Params("eventKey")
	if eventKey == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "eventKey wajib diisi"})
	}

	// Parse body JSON
	var data map[string]interface{}
	if err := ctx.BodyParser(&data); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Body harus berupa JSON: " + err.Error(),
		})
	}

	// Default userID untuk integrasi eksternal
	userID := 0
	if uid, ok := ctx.Locals("userID").(float64); ok {
		userID = int(uid)
	}

	// Proses async — langsung balas 202 Accepted
	go func() {
		integration_service.DispatchInboundPush(c.DB, eventKey, data, userID)
	}()

	return ctx.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "Data diterima dan sedang diproses",
	})
}

// ─── Manual Pull Trigger ──────────────────────────────────────────────────────

// POST /api/v1/integrations/:id/pull
// Dipanggil dari UI untuk trigger pull secara manual
func (c *IntegrationController) ManualPull(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Jalankan async
	go integration_service.DispatchInboundPull(c.DB, uint(id), userID)

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Pull dimulai, cek history untuk hasilnya",
	})
}

// ─── Retrigger Inbound dari History ──────────────────────────────────────────

// POST /api/v1/integrations/:id/history/:historyId/retrigger
// Sudah ada untuk outbound, sekarang handle inbound juga
func (c *IntegrationController) RetriggerUnified(ctx *fiber.Ctx) error {
	id, err := strconv.Atoi(ctx.Params("id"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	historyID, err := strconv.Atoi(ctx.Params("historyId"))
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "History ID tidak valid"})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Load integrasi
	var intg integration.Integration
	if err := c.DB.Preload("Connection").Preload("Recipients").First(&intg, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integrasi tidak ditemukan"})
	}

	// Load history
	var history integration.IntegrationHistory
	if err := c.DB.Where("id = ? AND integration_id = ?", historyID, id).First(&history).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "History tidak ditemukan"})
	}

	if history.PayloadSummary == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Payload tidak tersedia untuk retrigger",
		})
	}

	// Tentukan arah integrasi
	go func() {
		if intg.Direction == integration.DirectionInbound {
			// Inbound: proses ulang file/data → create outbound
			integration_service.RetriggerInbound(c.DB, intg, history.PayloadSummary, userID)
		} else {
			// Outbound: kirim ulang ke eksternal
			var eventData map[string]interface{}
			json.Unmarshal([]byte(history.PayloadSummary), &eventData)
			err := integration_service.RunIntegrationPublic(c.DB, c.QueryDB, intg, eventData, "retrigger")
			integration_service.LogHistoryPublic(c.DB, intg, history.EventKey, err, "retrigger", eventData)
			integration_service.SendNotificationPublic(c.DB, intg, eventData, err)
		}
	}()

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Retrigger dimulai, cek history untuk hasilnya",
	})
}

// POST /api/v1/integrations/:id/detect-headers
// Upload sample file → WMS baca header kolom → return ke frontend untuk column mapping UI
func (c *IntegrationController) DetectHeaders(ctx *fiber.Ctx) error {
	// Terima file upload
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Upload file sample terlebih dahulu",
		})
	}

	f, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal buka file: " + err.Error(),
		})
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal baca file: " + err.Error(),
		})
	}

	// Auto-detect headers
	headers, err := integration_service.DetectHeaders(data, "", file.Filename)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Gagal detect kolom: " + err.Error(),
		})
	}

	// WMS fields yang tersedia untuk di-map
	wmsFields := []map[string]string{
		{"key": "outbound_no", "label": "Outbound No"},
		{"key": "shipment_id", "label": "Shipment ID / Order Number"},
		{"key": "outbound_date", "label": "Outbound Date"},
		{"key": "owner_code", "label": "Owner Code"},
		{"key": "customer_code", "label": "Customer Code"},
		{"key": "whs_code", "label": "Warehouse Code"},
		{"key": "deliv_to", "label": "Deliver To"},
		{"key": "deliv_address", "label": "Deliver Address"},
		{"key": "deliv_city", "label": "Deliver City"},
		{"key": "driver", "label": "Driver"},
		{"key": "truck_no", "label": "Truck No"},
		{"key": "transporter_code", "label": "Transporter Code"},
		{"key": "awb_no", "label": "AWB No"},
		{"key": "remarks", "label": "Remarks"},
		{"key": "user_def1", "label": "User Def 1"},
		{"key": "user_def2", "label": "User Def 2"},
		{"key": "user_def3", "label": "User Def 3"},
		{"key": "item_code", "label": "Item Code / SKU"},
		{"key": "quantity", "label": "Quantity"},
		{"key": "uom", "label": "UOM"},
		{"key": "lot_number", "label": "Lot Number"},
		{"key": "exp_date", "label": "Expired Date"},
		{"key": "prod_date", "label": "Production Date"},
		{"key": "division_code", "label": "Division Code"},
		{"key": "qa_status", "label": "QA Status"},
	}

	return ctx.JSON(fiber.Map{
		"success":    true,
		"headers":    headers,   // kolom dari file
		"wms_fields": wmsFields, // field WMS yang tersedia
	})
}
