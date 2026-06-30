package rpt_builder_ctrl

import (
	"encoding/json"
	"errors"
	"fiber-app/models/rpt_builder"
	rpt_builder_service "fiber-app/services/rpt_builder"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type GenerateController struct {
	DB     *gorm.DB
	ReadDB *gorm.DB
}

func NewGenerateController(db *gorm.DB, readDB *gorm.DB) *GenerateController {
	return &GenerateController{DB: db, ReadDB: readDB}
}

// POST /api/v1/rpt-builder/preview
func (c *GenerateController) Preview(ctx *fiber.Ctx) error {
	var input rpt_builder.ReqGenerateReport
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

	// Load template + sheets + columns + params
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
		First(&template, input.TemplateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Template not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if !template.IsActive {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template is not active",
		})
	}

	// Validasi required params
	if err := validateRequiredParams(template.Params, input.Params); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Preview hanya sheet pertama, max 100 rows
	if len(template.Sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template has no sheets",
		})
	}

	sheet := template.Sheets[0]
	query, args, err := rpt_builder_service.BuildQuery(sheet.SqlQuery, input.Params)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// previewQuery := fmt.Sprintf("SELECT TOP 100 * FROM (%s) AS _preview", query)
	previewQuery := buildPreviewQuery(query)

	// strippedQuery := stripOuterOrderBy(query)
	// previewQuery := fmt.Sprintf("SELECT TOP 100 * FROM (%s) AS _preview", strippedQuery)

	// // DEBUG - hapus setelah fix
	// fmt.Println("=== STRIPPED QUERY ===")
	// fmt.Println(strippedQuery)
	// fmt.Println("=== PREVIEW QUERY ===")
	// fmt.Println(previewQuery)

	var rows []map[string]interface{}
	if err := c.ReadDB.Raw(previewQuery, args...).Scan(&rows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Query execution failed: " + err.Error(),
		})
	}

	if rows == nil {
		rows = []map[string]interface{}{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Preview: %d rows (max 100)", len(rows)),
		"data": fiber.Map{
			"sheet_name": sheet.SheetName,
			"columns":    sheet.Columns,
			"rows":       rows,
		},
	})
}

// POST /api/v1/rpt-builder/download
func (c *GenerateController) Download(ctx *fiber.Ctx) error {
	var input rpt_builder.ReqGenerateReport
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

	userID := uint(ctx.Locals("userID").(float64))

	// Load template
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
		First(&template, input.TemplateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Template not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if !template.IsActive {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template is not active",
		})
	}

	if len(template.Sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template has no sheets",
		})
	}

	// Validasi required params
	if err := validateRequiredParams(template.Params, input.Params); err != nil {

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Build sheets data
	var sheetsData []rpt_builder_service.SheetData
	totalRows := 0

	for _, sheet := range template.Sheets {
		// Get or init user column prefs untuk sheet ini
		prefs, err := rpt_builder_service.GetOrInitUserColumnPrefs(
			c.DB, userID, template.ID, sheet.ID, sheet.Columns,
		)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false, "message": "Failed to load column preferences: " + err.Error(),
			})
		}

		// Convert prefs ke ColumnConfig
		colConfigs := rpt_builder_service.PrefsToColumnConfigs(prefs, sheet.Columns)

		// Build & execute query
		query, args, err := rpt_builder_service.BuildQuery(sheet.SqlQuery, input.Params)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false, "message": fmt.Sprintf("Sheet '%s': %s", sheet.SheetName, err.Error()),
			})
		}

		fmt.Println("=== QUERY ===")
		fmt.Println(query)
		fmt.Println("=== ARGS ===")
		fmt.Println(args)

		var rows []map[string]interface{}
		if err := c.ReadDB.Raw(query, args...).Scan(&rows).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false, "message": fmt.Sprintf("Sheet '%s' query failed: %s", sheet.SheetName, err.Error()),
			})
		}

		if rows == nil {
			rows = []map[string]interface{}{}
		}

		totalRows += len(rows)
		sheetsData = append(sheetsData, rpt_builder_service.SheetData{
			SheetName: sheet.SheetName,
			Columns:   colConfigs,
			Rows:      rows,
		})
	}

	// Generate Excel
	f, err := rpt_builder_service.GenerateExcel(template.Name, sheetsData)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to generate Excel: " + err.Error(),
		})
	}

	// Log download
	paramsJSON, _ := json.Marshal(input.Params)
	c.DB.Create(&rpt_builder.Rpt2DownloadLog{
		TemplateID: template.ID,
		UserID:     userID,
		ParamsJSON: string(paramsJSON),
		RowCount:   totalRows,
		DownloadAt: time.Now(),
	})

	// Stream response
	filename := fmt.Sprintf("%s_%s.xlsx", template.Code, time.Now().Format("20060102_150405"))
	ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	buf, err := f.WriteToBuffer()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	return ctx.Send(buf.Bytes())
}

// PUT /api/v1/rpt-builder/column-prefs
func (c *GenerateController) UpdateColumnPrefs(ctx *fiber.Ctx) error {
	var input rpt_builder.ReqUpdateColumnPrefs
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

	userID := uint(ctx.Locals("userID").(float64))

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, item := range input.ColumnPrefs {
		if err := tx.Model(&rpt_builder.Rpt2UserColumnPref{}).
			Where("user_id = ? AND sheet_id = ? AND column_key = ?", userID, input.SheetID, item.ColumnKey).
			Updates(map[string]interface{}{
				"is_visible":   item.IsVisible,
				"column_order": item.ColumnOrder,
				"updated_at":   time.Now(),
			}).Error; err != nil {
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
		"success": true, "message": "Column preferences updated successfully",
	})
}

// ── Helper ────────────────────────────────────────────────────────────────────

func validateRequiredParams(params []rpt_builder.Rpt2Param, input map[string]string) error {
	for _, p := range params {

		if p.IsRequired {

			if p.ParamType == "DATERANGE" {
				startVal, startOK := input[p.ParamKey+"_start"]
				endVal, endOK := input[p.ParamKey+"_end"]
				if !startOK || startVal == "" || !endOK || endVal == "" {
					return fmt.Errorf("parameter '%s' is required (both start and end)", p.Label)
				}
			} else {
				val, ok := input[p.ParamKey]
				if !ok || val == "" {
					return fmt.Errorf("parameter '%s' is required", p.Label)
				}
			}

			// val, ok := input[p.ParamKey]
			// if !ok || val == "" {
			// 	return fmt.Errorf("parameter '%s' is required", p.Label)
			// }
		}
	}
	return nil
}

func stripOuterOrderBy(query string) string {
	normalized := strings.ReplaceAll(query, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	re := regexp.MustCompile(`(?is)\s+ORDER\s+BY\s+.+$`)

	depth := 0
	orderByIdx := -1
	runes := []rune(normalized)

	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '(':
			depth++
		case ')':
			depth--
		}

		if depth == 0 {
			remaining := string(runes[i:])
			if re.MatchString(remaining) {
				loc := re.FindStringIndex(remaining)
				if loc != nil && loc[0] == 0 {
					orderByIdx = i
					break
				}
			}
		}
	}

	if orderByIdx >= 0 {
		return strings.TrimRight(string(runes[:orderByIdx]), " \t\n")
	}
	return normalized
}

func buildPreviewQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	stripped := stripOuterOrderBy(trimmed)
	upper := strings.ToUpper(stripped)

	// Kalau CTE, inject TOP 100 ke SELECT utama terakhir
	if strings.HasPrefix(upper, "WITH ") || strings.HasPrefix(upper, "WITH\n") || strings.HasPrefix(upper, "WITH\t") {
		runes := []rune(stripped)
		depth := 0
		lastTopLevelSelect := -1

		for i := 0; i < len(runes); i++ {
			switch runes[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				remaining := strings.ToUpper(string(runes[i:]))
				if strings.HasPrefix(remaining, "SELECT") {
					lastTopLevelSelect = i
				}
			}
		}

		if lastTopLevelSelect >= 0 {
			before := string(runes[:lastTopLevelSelect+6]) // sampai akhir "SELECT"
			after := string(runes[lastTopLevelSelect+6:])
			return before + " TOP 100" + after
		}

		return stripped
	}

	// Non-CTE: wrap biasa
	return fmt.Sprintf("SELECT TOP 100 * FROM (%s) AS _preview", stripped)
}

func (c *GenerateController) ResolveOptions(ctx *fiber.Ctx) error {
	var input struct {
		TemplateID int    `json:"template_id" validate:"required"`
		ParamKey   string `json:"param_key" validate:"required"`
	}
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

	// Ambil param dari DB berdasarkan template_id + param_key
	var param rpt_builder.Rpt2Param
	if err := c.DB.
		Where("template_id = ? AND param_key = ? AND param_type = ?", input.TemplateID, input.ParamKey, "SELECT").
		First(&param).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Param not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if param.OptionsQuery == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Param has no options_query",
		})
	}

	// Eksekusi options_query dari DB (bukan dari request body)
	var rows []map[string]interface{}
	if err := c.ReadDB.Raw(param.OptionsQuery).Scan(&rows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Query execution failed: " + err.Error(),
		})
	}

	if rows == nil {
		rows = []map[string]interface{}{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("%d options found", len(rows)),
		"data":    rows,
	})
}

// POST /api/v1/rpt-builder/download/check
func (c *GenerateController) CheckDownload(ctx *fiber.Ctx) error {
	var input rpt_builder.ReqGenerateReport
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

	// Load template
	var template rpt_builder.Rpt2Template
	if err := c.DB.
		Preload("Sheets", func(db *gorm.DB) *gorm.DB {
			return db.Order("sheet_order ASC")
		}).
		Preload("Params", func(db *gorm.DB) *gorm.DB {
			return db.Order("param_order ASC")
		}).
		First(&template, input.TemplateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Template not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	if !template.IsActive {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template is not active",
		})
	}

	if len(template.Sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Template has no sheets",
		})
	}

	// Validasi required params
	if err := validateRequiredParams(template.Params, input.Params); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": err.Error(),
		})
	}

	// Validasi semua query bisa di-build (cek syntax + params)
	for _, sheet := range template.Sheets {
		if _, _, err := rpt_builder_service.BuildQuery(sheet.SqlQuery, input.Params); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Sheet '%s': %s", sheet.SheetName, err.Error()),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Ready to download",
		"data": fiber.Map{
			"template_name": template.Name,
			"sheet_count":   len(template.Sheets),
		},
	})
}
