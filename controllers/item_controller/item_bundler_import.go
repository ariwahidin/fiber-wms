package item_controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ProductBundleImportController struct {
	DB *gorm.DB
}

func NewProductBundleImportController(DB *gorm.DB) *ProductBundleImportController {
	return &ProductBundleImportController{DB: DB}
}

type bundleImportRow struct {
	RowNumber         int
	BundleItemCode    string
	ComponentItemCode string
	Qty               float64
}

type bundleImportError struct {
	Row       int    `json:"row"`
	Bundle    string `json:"bundle_item_code,omitempty"`
	Component string `json:"component_item_code,omitempty"`
	Column    string `json:"column,omitempty"`
	Message   string `json:"message"`
}

type bundleImportPreview struct {
	Success       bool                `json:"success"`
	TotalRows     int                 `json:"total_rows"`
	BundleCount   int                 `json:"bundle_count"`
	ValidRows     int                 `json:"valid_rows"`
	ErrorRows     int                 `json:"error_rows"`
	WillSetBundle int                 `json:"will_set_bundle"`
	Errors        []bundleImportError `json:"errors"`
}

type bundleImportGroup struct {
	BundleProduct models.Product
	Rows          []bundleImportRow
}

// GET /products/bundles/import/template
func (c *ProductBundleImportController) DownloadTemplate(ctx *fiber.Ctx) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		sheet = "Bundle Import"
		_ = f.SetSheetName("Sheet1", sheet)
	}

	headers := []string{"Bundle Item Code", "Component Item Code", "Qty"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
	}

	examples := [][]interface{}{
		{"BUNDLE001", "ITEM001", 1},
		{"BUNDLE001", "ITEM002", 2},
		{"BUNDLE002", "ITEM010", 1},
	}

	for r, values := range examples {
		for cidx, value := range values {
			cell, _ := excelize.CoordinatesToCellName(cidx+1, r+2)
			_ = f.SetCellValue(sheet, cell, value)
		}
	}

	if style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	}); err == nil {
		_ = f.SetCellStyle(sheet, "A1", "C1", style)
	}

	_ = f.SetColWidth(sheet, "A", "B", 25)
	_ = f.SetColWidth(sheet, "C", "C", 12)

	buffer, err := f.WriteToBuffer()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate Excel template",
			"error":   err.Error(),
		})
	}

	ctx.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set(fiber.HeaderContentDisposition, `attachment; filename="product_bundle_import_template.xlsx"`)

	return ctx.Send(buffer.Bytes())
}

// POST /products/bundles/import/preview
// multipart/form-data: file
func (c *ProductBundleImportController) PreviewImportExcel(ctx *fiber.Ctx) error {
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Excel file is required. Use form-data field 'file'",
		})
	}

	rows, parseErrors, err := parseBundleExcel(file)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read Excel file",
			"error":   err.Error(),
		})
	}

	preview := c.validateBundleImport(rows, parseErrors)

	status := fiber.StatusOK
	if !preview.Success {
		status = fiber.StatusBadRequest
	}

	return ctx.Status(status).JSON(preview)
}

// POST /products/bundles/import
// multipart/form-data: file
func (c *ProductBundleImportController) ImportExcel(ctx *fiber.Ctx) error {
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Excel file is required. Use form-data field 'file'",
		})
	}

	rows, parseErrors, err := parseBundleExcel(file)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read Excel file",
			"error":   err.Error(),
		})
	}

	preview := c.validateBundleImport(rows, parseErrors)
	if !preview.Success {
		return ctx.Status(fiber.StatusBadRequest).JSON(preview)
	}

	if len(rows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Excel file contains no data rows",
		})
	}

	userID, err := getBundleImportUserID(ctx)
	if err != nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	groups, err := c.loadBundleGroups(rows)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to validate products",
			"error":   err.Error(),
		})
	}

	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	rollback := func(status int, message string, err error) error {
		_ = tx.Rollback()
		response := fiber.Map{"success": false, "message": message}
		if err != nil {
			response["error"] = err.Error()
		}
		return ctx.Status(status).JSON(response)
	}

	bundleCount := 0
	componentCount := 0

	for _, group := range groups {
		var bundleProduct models.Product

		if err := tx.
			Where("id = ?", group.BundleProduct.ID).
			Where("deleted_at IS NULL").
			First(&bundleProduct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return rollback(fiber.StatusBadRequest,
					fmt.Sprintf("Bundle product not found: %s", group.BundleProduct.ItemCode), nil)
			}
			return rollback(fiber.StatusInternalServerError,
				"Failed to load bundle product", err)
		}

		// Replace existing bundle configuration.
		if err := tx.
			Where("bundle_product_id = ?", bundleProduct.ID).
			Delete(&models.ProductBundle{}).Error; err != nil {
			return rollback(fiber.StatusInternalServerError,
				fmt.Sprintf("Failed to remove existing bundle items for %s", bundleProduct.ItemCode), err)
		}

		// N, empty, or NULL becomes Y only inside this transaction.
		if err := tx.Model(&models.Product{}).
			Where("id = ?", bundleProduct.ID).
			Updates(map[string]interface{}{
				"is_bundle":  "Y",
				"updated_by": userID,
			}).Error; err != nil {
			return rollback(fiber.StatusInternalServerError,
				fmt.Sprintf("Failed to mark %s as bundle", bundleProduct.ItemCode), err)
		}

		for _, row := range group.Rows {
			var component models.Product

			if err := tx.
				Where("item_code = ?", row.ComponentItemCode).
				Where("deleted_at IS NULL").
				First(&component).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return rollback(fiber.StatusBadRequest,
						fmt.Sprintf("Component product not found: %s (row %d)",
							row.ComponentItemCode, row.RowNumber), nil)
				}
				return rollback(fiber.StatusInternalServerError,
					"Failed to load component product", err)
			}

			bundleItem := models.ProductBundle{
				BundleProductId: bundleProduct.ID,
				ItemId:          component.ID,
				ItemCode:        component.ItemCode,
				Qty:             row.Qty,
				CreatedBy:       userID,
				UpdatedBy:       userID,
			}

			if err := tx.Create(&bundleItem).Error; err != nil {
				return rollback(fiber.StatusInternalServerError,
					fmt.Sprintf("Failed to create bundle component at row %d", row.RowNumber), err)
			}

			componentCount++
		}

		bundleCount++
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit bundle import",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product bundle Excel imported successfully",
		"data": fiber.Map{
			"total_rows":        len(rows),
			"bundle_count":      bundleCount,
			"component_count":   componentCount,
			"updated_is_bundle": true,
		},
	})
}

func (c *ProductBundleImportController) validateBundleImport(
	rows []bundleImportRow,
	initialErrors []bundleImportError,
) bundleImportPreview {
	preview := bundleImportPreview{
		Success:   true,
		TotalRows: len(rows),
		Errors:    append([]bundleImportError{}, initialErrors...),
	}

	seen := make(map[string]map[string]bool)
	bundleCodes := make(map[string]bool)

	for _, row := range rows {
		bundleCode := normalizeBundleCode(row.BundleItemCode)
		componentCode := normalizeBundleCode(row.ComponentItemCode)

		if bundleCode == "" {
			preview.Errors = append(preview.Errors, bundleImportError{
				Row: row.RowNumber, Column: "Bundle Item Code",
				Message: "Bundle Item Code is required",
			})
			continue
		}
		if componentCode == "" {
			preview.Errors = append(preview.Errors, bundleImportError{
				Row: row.RowNumber, Bundle: bundleCode,
				Column:  "Component Item Code",
				Message: "Component Item Code is required",
			})
			continue
		}
		if row.Qty <= 0 {
			preview.Errors = append(preview.Errors, bundleImportError{
				Row: row.RowNumber, Bundle: bundleCode, Component: componentCode,
				Column: "Qty", Message: "Qty must be greater than 0",
			})
			continue
		}
		if bundleCode == componentCode {
			preview.Errors = append(preview.Errors, bundleImportError{
				Row: row.RowNumber, Bundle: bundleCode, Component: componentCode,
				Message: "Bundle product cannot be one of its own items",
			})
			continue
		}

		if seen[bundleCode] == nil {
			seen[bundleCode] = make(map[string]bool)
		}
		if seen[bundleCode][componentCode] {
			preview.Errors = append(preview.Errors, bundleImportError{
				Row: row.RowNumber, Bundle: bundleCode, Component: componentCode,
				Message: "Duplicate component found for this bundle",
			})
			continue
		}

		seen[bundleCode][componentCode] = true
		bundleCodes[bundleCode] = true
	}

	preview.BundleCount = len(bundleCodes)

	if len(preview.Errors) > 0 {
		preview.Success = false
		preview.ErrorRows = len(preview.Errors)
		preview.ValidRows = preview.TotalRows - preview.ErrorRows
		if preview.ValidRows < 0 {
			preview.ValidRows = 0
		}
		return preview
	}

	grouped, err := c.loadBundleGroups(rows)
	if err != nil {
		preview.Errors = append(preview.Errors, bundleImportError{
			Column: "Database", Message: err.Error(),
		})
		preview.Success = false
		preview.ErrorRows = len(preview.Errors)
		return preview
	}

	for bundleCode, group := range grouped {
		isBundle := strings.ToUpper(strings.TrimSpace(group.BundleProduct.IsBundle))

		switch isBundle {
		case "", "N", "Y":
		default:
			preview.Errors = append(preview.Errors, bundleImportError{
				Row:    group.Rows[0].RowNumber,
				Bundle: bundleCode,
				Column: "IsBundle",
				Message: fmt.Sprintf(
					"Invalid IsBundle value %q. Allowed values are Y, N, empty, or NULL",
					group.BundleProduct.IsBundle,
				),
			})
		}

		for _, row := range group.Rows {
			var component models.Product
			err := c.DB.
				Where("item_code = ?", row.ComponentItemCode).
				Where("deleted_at IS NULL").
				First(&component).Error

			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					preview.Errors = append(preview.Errors, bundleImportError{
						Row: row.RowNumber, Bundle: row.BundleItemCode,
						Component: row.ComponentItemCode,
						Message:   "Component product not found",
					})
					continue
				}
				preview.Errors = append(preview.Errors, bundleImportError{
					Row: row.RowNumber, Bundle: row.BundleItemCode,
					Component: row.ComponentItemCode,
					Message:   "Failed to validate component product: " + err.Error(),
				})
			}
		}
	}

	if circularErrors := c.validateCircularBundles(grouped); len(circularErrors) > 0 {
		preview.Errors = append(preview.Errors, circularErrors...)
	}

	preview.Success = len(preview.Errors) == 0
	preview.ErrorRows = len(preview.Errors)
	preview.ValidRows = preview.TotalRows - preview.ErrorRows
	if preview.ValidRows < 0 {
		preview.ValidRows = 0
	}

	for _, group := range grouped {
		if strings.ToUpper(strings.TrimSpace(group.BundleProduct.IsBundle)) != "Y" {
			preview.WillSetBundle++
		}
	}

	return preview
}

func (c *ProductBundleImportController) loadBundleGroups(
	rows []bundleImportRow,
) (map[string]bundleImportGroup, error) {
	grouped := make(map[string]bundleImportGroup)

	for _, row := range rows {
		bundleCode := normalizeBundleCode(row.BundleItemCode)

		if group, exists := grouped[bundleCode]; exists {
			group.Rows = append(group.Rows, row)
			grouped[bundleCode] = group
			continue
		}

		var bundleProduct models.Product
		if err := c.DB.
			Where("item_code = ?", bundleCode).
			Where("deleted_at IS NULL").
			First(&bundleProduct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf(
					"bundle product not found: %s (row %d)",
					bundleCode, row.RowNumber,
				)
			}
			return nil, err
		}

		grouped[bundleCode] = bundleImportGroup{
			BundleProduct: bundleProduct,
			Rows:          []bundleImportRow{row},
		}
	}

	return grouped, nil
}

// Detect cycles using existing DB relationships, with Excel rows replacing
// the outgoing edges of bundles included in this import.
func (c *ProductBundleImportController) validateCircularBundles(
	groups map[string]bundleImportGroup,
) []bundleImportError {
	var existing []models.ProductBundle
	if err := c.DB.
		Where("deleted_at IS NULL").
		Find(&existing).Error; err != nil {
		return []bundleImportError{{
			Column:  "Database",
			Message: "Failed to validate existing bundle relationships: " + err.Error(),
		}}
	}

	graph := make(map[uint][]uint)

	for _, item := range existing {
		graph[item.BundleProductId] = append(graph[item.BundleProductId], item.ItemId)
	}

	for _, group := range groups {
		graph[group.BundleProduct.ID] = nil

		for _, row := range group.Rows {
			var component models.Product
			if err := c.DB.
				Where("item_code = ?", row.ComponentItemCode).
				Where("deleted_at IS NULL").
				First(&component).Error; err != nil {
				continue
			}
			graph[group.BundleProduct.ID] = append(
				graph[group.BundleProduct.ID], component.ID,
			)
		}
	}

	var result []bundleImportError

	for _, group := range groups {
		startID := group.BundleProduct.ID
		visited := make(map[uint]bool)
		stack := make(map[uint]bool)

		var visit func(uint) bool
		visit = func(node uint) bool {
			if stack[node] {
				return true
			}
			if visited[node] {
				return false
			}

			visited[node] = true
			stack[node] = true

			for _, next := range graph[node] {
				if next == startID || visit(next) {
					return true
				}
			}

			delete(stack, node)
			return false
		}

		if visit(startID) {
			result = append(result, bundleImportError{
				Row:     group.Rows[0].RowNumber,
				Bundle:  group.BundleProduct.ItemCode,
				Column:  "Bundle",
				Message: "Circular bundle relationship detected",
			})
		}
	}

	return result
}

func parseBundleExcel(
	file *multipart.FileHeader,
) ([]bundleImportRow, []bundleImportError, error) {
	src, err := file.Open()
	if err != nil {
		return nil, nil, err
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, nil, fmt.Errorf("workbook does not contain a worksheet")
	}

	rawRows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	if len(rawRows) == 0 {
		return nil, nil, fmt.Errorf("worksheet is empty")
	}

	headerMap, err := parseBundleHeaders(rawRows[0])
	if err != nil {
		return nil, nil, err
	}

	var rows []bundleImportRow
	var errorsList []bundleImportError

	for i := 1; i < len(rawRows); i++ {
		excelRow := rawRows[i]
		if isBlankExcelRow(excelRow) {
			continue
		}

		rowNumber := i + 1
		bundleCode := excelCell(excelRow, headerMap["bundle"])
		componentCode := excelCell(excelRow, headerMap["component"])
		qtyText := excelCell(excelRow, headerMap["qty"])

		qty, err := parseExcelQty(qtyText)
		if err != nil {
			errorsList = append(errorsList, bundleImportError{
				Row:       rowNumber,
				Bundle:    strings.TrimSpace(bundleCode),
				Component: strings.TrimSpace(componentCode),
				Column:    "Qty",
				Message:   "Invalid Qty: " + err.Error(),
			})
			continue
		}

		rows = append(rows, bundleImportRow{
			RowNumber:         rowNumber,
			BundleItemCode:    strings.TrimSpace(bundleCode),
			ComponentItemCode: strings.TrimSpace(componentCode),
			Qty:               qty,
		})
	}

	return rows, errorsList, nil
}

func parseBundleHeaders(header []string) (map[string]int, error) {
	result := make(map[string]int)

	for i, value := range header {
		switch normalizeBundleHeader(value) {
		case "bundleitemcode", "bundlecode", "bundleitem":
			result["bundle"] = i
		case "componentitemcode", "componentcode", "itemcode", "componentitem":
			result["component"] = i
		case "qty", "quantity":
			result["qty"] = i
		}
	}

	for _, key := range []string{"bundle", "component", "qty"} {
		if _, ok := result[key]; !ok {
			return nil, fmt.Errorf(
				"missing required Excel column: %s",
				displayBundleHeaderName(key),
			)
		}
	}

	return result, nil
}

func parseExcelQty(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("Qty is required")
	}

	qty, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func normalizeBundleHeader(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func normalizeBundleCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func displayBundleHeaderName(key string) string {
	switch key {
	case "bundle":
		return "Bundle Item Code"
	case "component":
		return "Component Item Code"
	case "qty":
		return "Qty"
	default:
		return key
	}
}

func excelCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func isBlankExcelRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func getBundleImportUserID(ctx *fiber.Ctx) (int, error) {
	value := ctx.Locals("userID")
	if value == nil {
		return 0, fmt.Errorf("userID is not available")
	}

	switch v := value.(type) {
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint64:
		return int(v), nil
	case string:
		id, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid userID")
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unsupported userID type %T", value)
	}
}
