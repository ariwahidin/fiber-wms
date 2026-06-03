package rpt_builder_ctrl

import (
	"errors"
	"fiber-app/models/rpt_builder"
	rpt_builder_service "fiber-app/services/rpt_builder"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TemplateController struct {
	DB *gorm.DB
}

func NewTemplateController(db *gorm.DB) *TemplateController {
	return &TemplateController{DB: db}
}

// GET /api/v1/rpt-builder/templates
func (c *TemplateController) GetAll(ctx *fiber.Ctx) error {
	var templates []rpt_builder.Rpt2Template

	query := c.DB.Preload("Sheets", func(db *gorm.DB) *gorm.DB {
		return db.Order("sheet_order ASC").Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("column_order ASC")
		})
	}).Preload("Params", func(db *gorm.DB) *gorm.DB {
		return db.Order("param_order ASC")
	})

	if cat := ctx.Query("category"); cat != "" {
		query = query.Where("category = ?", cat)
	}
	if ctx.Query("active_only") == "true" {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("code ASC").Find(&templates).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Templates found", "data": templates,
	})
}

// GET /api/v1/rpt-builder/templates/:id
func (c *TemplateController) GetByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	var template rpt_builder.Rpt2Template
	if err := c.DB.
		Preload("Sheets", func(db *gorm.DB) *gorm.DB {
			return db.Order("sheet_order ASC").Preload("Columns", func(db *gorm.DB) *gorm.DB {
				return db.Order("column_order ASC")
			})
		}).
		Preload("Params", func(db *gorm.DB) *gorm.DB {
			return db.Order("param_order ASC")
		}).
		First(&template, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Template not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Template found", "data": template,
	})
}

// POST /api/v1/rpt-builder/templates
func (c *TemplateController) Create(ctx *fiber.Ctx) error {
	var input rpt_builder.ReqCreateTemplate
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Cek duplikat code
	var existing rpt_builder.Rpt2Template
	if err := c.DB.Where("code = ?", input.Code).First(&existing).Error; err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template code already exists",
		})
	}

	userID := int(ctx.Locals("userID").(float64))
	template := rpt_builder.Rpt2Template{
		Code:        input.Code,
		Name:        input.Name,
		Description: input.Description,
		Category:    input.Category,
		IsActive:    true,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
	}

	if err := c.DB.Create(&template).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Template created successfully", "data": template,
	})
}

// PUT /api/v1/rpt-builder/templates/:id
func (c *TemplateController) Update(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	var input rpt_builder.ReqUpdateTemplate
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	userID := int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&rpt_builder.Rpt2Template{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":        input.Name,
		"description": input.Description,
		"category":    input.Category,
		"is_active":   input.IsActive,
		"updated_by":  userID,
		"updated_at":  time.Now(),
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Template updated successfully",
	})
}

// DELETE /api/v1/rpt-builder/templates/:id (soft delete via is_active = false)
func (c *TemplateController) Delete(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	userID := int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&rpt_builder.Rpt2Template{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_active":  false,
		"updated_by": userID,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Template deactivated successfully",
	})
}

// ── Sheet CRUD ────────────────────────────────────────────────────────────────

// POST /api/v1/rpt-builder/templates/:id/sheets
func (c *TemplateController) AddSheet(ctx *fiber.Ctx) error {
	templateID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid template ID",
		})
	}

	var input rpt_builder.ReqSaveSheet
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Validasi query
	if err := rpt_builder_service.ValidateQuery(input.SqlQuery); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid SQL: " + err.Error(),
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	sheet := rpt_builder.Rpt2Sheet{
		TemplateID: uint(templateID),
		SheetName:  input.SheetName,
		SqlQuery:   input.SqlQuery,
		SheetOrder: input.SheetOrder,
		CreatedBy:  userID,
		CreatedAt:  time.Now(),
	}

	if err := tx.Create(&sheet).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Insert columns
	for _, col := range input.Columns {
		width := col.ExcelWidth
		if width == 0 {
			width = 120
		}
		column := rpt_builder.Rpt2Column{
			TemplateID:       uint(templateID),
			SheetID:          sheet.ID,
			ColumnKey:        col.ColumnKey,
			DisplayName:      col.DisplayName,
			ColumnOrder:      col.ColumnOrder,
			ExcelWidth:       width,
			ExcelFormat:      col.ExcelFormat,
			IsDefaultVisible: col.IsDefaultVisible,
			IsToggleable:     col.IsToggleable,
		}
		if err := tx.Create(&column).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false, "message": err.Error(),
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Sheet added successfully", "data": sheet,
	})
}

// PUT /api/v1/rpt-builder/sheets/:id
func (c *TemplateController) UpdateSheet(ctx *fiber.Ctx) error {
	sheetID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid sheet ID",
		})
	}

	var input rpt_builder.ReqSaveSheet
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if err := rpt_builder_service.ValidateQuery(input.SqlQuery); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid SQL: " + err.Error(),
		})
	}

	var sheet rpt_builder.Rpt2Sheet
	if err := c.DB.First(&sheet, sheetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Sheet not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&rpt_builder.Rpt2Sheet{}).Where("id = ?", sheetID).Updates(map[string]interface{}{
		"sheet_name":  input.SheetName,
		"sql_query":   input.SqlQuery,
		"sheet_order": input.SheetOrder,
		"updated_by":  userID,
		"updated_at":  time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Hapus columns lama lalu insert ulang
	if err := tx.Where("sheet_id = ?", sheetID).Delete(&rpt_builder.Rpt2Column{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	for _, col := range input.Columns {
		width := col.ExcelWidth
		if width == 0 {
			width = 120
		}
		column := rpt_builder.Rpt2Column{
			TemplateID:       sheet.TemplateID,
			SheetID:          uint(sheetID),
			ColumnKey:        col.ColumnKey,
			DisplayName:      col.DisplayName,
			ColumnOrder:      col.ColumnOrder,
			ExcelWidth:       width,
			ExcelFormat:      col.ExcelFormat,
			IsDefaultVisible: col.IsDefaultVisible,
			IsToggleable:     col.IsToggleable,
		}
		if err := tx.Create(&column).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false, "message": err.Error(),
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Sheet updated successfully",
	})
}

// DELETE /api/v1/rpt-builder/sheets/:id
func (c *TemplateController) DeleteSheet(ctx *fiber.Ctx) error {
	sheetID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid sheet ID",
		})
	}

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Hapus columns & user prefs dulu
	tx.Where("sheet_id = ?", sheetID).Delete(&rpt_builder.Rpt2Column{})
	tx.Where("sheet_id = ?", sheetID).Delete(&rpt_builder.Rpt2UserColumnPref{})

	if err := tx.Delete(&rpt_builder.Rpt2Sheet{}, sheetID).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Sheet deleted successfully",
	})
}

// ── Param CRUD ────────────────────────────────────────────────────────────────

// POST /api/v1/rpt-builder/templates/:id/params
func (c *TemplateController) AddParam(ctx *fiber.Ctx) error {
	templateID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid template ID",
		})
	}

	var input rpt_builder.ReqSaveParam
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	param := rpt_builder.Rpt2Param{
		TemplateID:       uint(templateID),
		ParamKey:         input.ParamKey,
		Label:            input.Label,
		ParamType:        input.ParamType,
		DefaultValue:     input.DefaultValue,
		OptionsQuery:     input.OptionsQuery,
		ApplicableSheets: input.ApplicableSheets,
		IsRequired:       input.IsRequired,
		ParamOrder:       input.ParamOrder,
	}

	if err := c.DB.Create(&param).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true, "message": "Param added successfully", "data": param,
	})
}

// DELETE /api/v1/rpt-builder/params/:id
func (c *TemplateController) DeleteParam(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	if err := c.DB.Delete(&rpt_builder.Rpt2Param{}, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true, "message": "Param deleted successfully",
	})
}
