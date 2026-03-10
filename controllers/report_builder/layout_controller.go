package report_builder

import (
	"errors"
	"fiber-app/models/report_builder"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type LayoutController struct {
	DB *gorm.DB
}

func NewLayoutController(db *gorm.DB) *LayoutController {
	return &LayoutController{DB: db}
}

// GET /api/v1/report-builder/layouts?report_id=1&owner_code=OWNER01
func (c *LayoutController) GetLayouts(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	query := c.DB.Preload("Report").
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC").Preload("Field")
		}).
		Preload("Filters", func(db *gorm.DB) *gorm.DB {
			return db.Preload("Field")
		})

	if ctx.Query("report_id") != "" {
		query = query.Where("report_id = ?", ctx.Query("report_id"))
	}
	if ctx.Query("owner_code") != "" {
		query = query.Where("owner_code = ? OR owner_code = ''", ctx.Query("owner_code"))
	}

	// Tampilkan layout milik user sendiri + layout public
	query = query.Where("created_by = ? OR is_public = ?", userID, true)

	var layouts []report_builder.RptLayout
	if err := query.Order("is_default DESC, layout_name ASC").Find(&layouts).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Layouts found", "data": layouts,
	})
}

// GET /api/v1/report-builder/layouts/:id
func (c *LayoutController) GetLayoutByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var layout report_builder.RptLayout
	if err := c.DB.
		Preload("Report").
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC").Preload("Field")
		}).
		Preload("Filters", func(db *gorm.DB) *gorm.DB {
			return db.Preload("Field")
		}).
		Preload("DocumentConfig").
		First(&layout, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Layout not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Layout found", "data": layout,
	})
}

// POST /api/v1/report-builder/layouts
func (c *LayoutController) SaveLayout(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var input report_builder.ReqSaveLayout
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Validasi report_id exist
	var report report_builder.RptReport
	if err := c.DB.First(&report, input.ReportID).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Report not found"})
	}

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Kalau is_default, reset default layout lain milik user untuk report ini
	if input.IsDefault {
		if err := tx.Model(&report_builder.RptLayout{}).
			Where("report_id = ? AND created_by = ? AND is_default = ?", input.ReportID, userID, true).
			Update("is_default", false).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	layout := report_builder.RptLayout{
		ReportID:   input.ReportID,
		LayoutName: input.LayoutName,
		OwnerCode:  input.OwnerCode,
		IsDefault:  input.IsDefault,
		IsPublic:   input.IsPublic,
		CreatedBy:  userID,
		CreatedAt:  time.Now(),
	}

	if err := tx.Create(&layout).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Insert columns
	for _, col := range input.Columns {
		width := col.ColumnWidth
		if width == 0 {
			width = 120
		}
		column := report_builder.RptLayoutColumn{
			LayoutID:    layout.ID,
			FieldID:     col.FieldID,
			ColumnLabel: col.ColumnLabel,
			ColumnOrder: col.ColumnOrder,
			ColumnWidth: width,
			IsVisible:   col.IsVisible,
		}
		if err := tx.Create(&column).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	// Insert filters
	for _, f := range input.Filters {
		filter := report_builder.RptLayoutFilter{
			LayoutID:    layout.ID,
			FieldID:     f.FieldID,
			Operator:    f.Operator,
			FilterValue: f.FilterValue,
			IsRequired:  f.IsRequired,
		}
		if err := tx.Create(&filter).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Layout saved successfully", "data": layout,
	})
}

// PUT /api/v1/report-builder/layouts/:id
func (c *LayoutController) UpdateLayout(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	userID := int(ctx.Locals("userID").(float64))

	var layout report_builder.RptLayout
	if err := c.DB.First(&layout, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Layout not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Hanya owner yang bisa edit
	if layout.CreatedBy != userID {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "You don't have permission to edit this layout"})
	}

	var input report_builder.ReqSaveLayout
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Reset default kalau perlu
	if input.IsDefault {
		if err := tx.Model(&report_builder.RptLayout{}).
			Where("report_id = ? AND created_by = ? AND is_default = ? AND id != ?", layout.ReportID, userID, true, id).
			Update("is_default", false).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	// Update layout header
	if err := tx.Model(&report_builder.RptLayout{}).Where("id = ?", id).Updates(map[string]interface{}{
		"layout_name": input.LayoutName,
		"owner_code":  input.OwnerCode,
		"is_default":  input.IsDefault,
		"is_public":   input.IsPublic,
		"updated_by":  userID,
		"updated_at":  time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Hapus columns & filters lama lalu insert ulang
	if err := tx.Where("layout_id = ?", id).Delete(&report_builder.RptLayoutColumn{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}
	if err := tx.Where("layout_id = ?", id).Delete(&report_builder.RptLayoutFilter{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	for _, col := range input.Columns {
		width := col.ColumnWidth
		if width == 0 {
			width = 120
		}
		column := report_builder.RptLayoutColumn{
			LayoutID:    uint(id),
			FieldID:     col.FieldID,
			ColumnLabel: col.ColumnLabel,
			ColumnOrder: col.ColumnOrder,
			ColumnWidth: width,
			IsVisible:   col.IsVisible,
		}
		if err := tx.Create(&column).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	for _, f := range input.Filters {
		filter := report_builder.RptLayoutFilter{
			LayoutID:    uint(id),
			FieldID:     f.FieldID,
			Operator:    f.Operator,
			FilterValue: f.FilterValue,
			IsRequired:  f.IsRequired,
		}
		if err := tx.Create(&filter).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Layout updated successfully",
	})
}

// DELETE /api/v1/report-builder/layouts/:id
func (c *LayoutController) DeleteLayout(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	userID := int(ctx.Locals("userID").(float64))

	var layout report_builder.RptLayout
	if err := c.DB.First(&layout, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Layout not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if layout.CreatedBy != userID {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "You don't have permission to delete this layout"})
	}

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Hapus child records dulu
	tx.Where("layout_id = ?", id).Delete(&report_builder.RptLayoutColumn{})
	tx.Where("layout_id = ?", id).Delete(&report_builder.RptLayoutFilter{})
	tx.Where("layout_id = ?", id).Delete(&report_builder.RptDocumentConfig{})

	if err := tx.Delete(&report_builder.RptLayout{}, id).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Layout deleted successfully",
	})
}

// PUT /api/v1/report-builder/layouts/:id/document-config
func (c *LayoutController) SaveDocumentConfig(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	type ReqDocConfig struct {
		PaperSize   string `json:"paper_size"`
		Orientation string `json:"orientation"`
		HeaderJSON  string `json:"header_json"`
		FooterJSON  string `json:"footer_json"`
		ShowPageNum bool   `json:"show_page_num"`
		RowsPerPage int    `json:"rows_per_page"`
	}

	var input ReqDocConfig
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if input.PaperSize == "" {
		input.PaperSize = "A4"
	}
	if input.Orientation == "" {
		input.Orientation = "P"
	}
	if input.RowsPerPage == 0 {
		input.RowsPerPage = 30
	}

	// Upsert document config
	var config report_builder.RptDocumentConfig
	result := c.DB.Where("layout_id = ?", id).First(&config)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		config = report_builder.RptDocumentConfig{
			LayoutID:    uint(id),
			PaperSize:   input.PaperSize,
			Orientation: input.Orientation,
			HeaderJSON:  input.HeaderJSON,
			FooterJSON:  input.FooterJSON,
			ShowPageNum: input.ShowPageNum,
			RowsPerPage: input.RowsPerPage,
		}
		if err := c.DB.Create(&config).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	} else {
		if err := c.DB.Model(&config).Updates(map[string]interface{}{
			"paper_size":    input.PaperSize,
			"orientation":   input.Orientation,
			"header_json":   input.HeaderJSON,
			"footer_json":   input.FooterJSON,
			"show_page_num": input.ShowPageNum,
			"rows_per_page": input.RowsPerPage,
		}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Document config saved", "data": config,
	})
}
