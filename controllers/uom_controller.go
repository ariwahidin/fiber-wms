package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type UomController struct {
	DB *gorm.DB
}

func NewUomController(DB *gorm.DB) *UomController {
	return &UomController{DB: DB}
}

func (c *UomController) CreateUom(ctx *fiber.Ctx) error {
	var uomPayolad models.UomConversion
	if err := ctx.BodyParser(&uomPayolad); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	if uomPayolad.ItemCode == "" || uomPayolad.Ean == "" || uomPayolad.FromUom == "" || uomPayolad.ToUom == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item, Ean, FromUom, and ToUom are required", "message": "Please provide all required fields"})
	}

	uomExists := false
	errCheck1 := c.DB.Where("item_code = ? AND ean = ? AND from_uom = ? AND to_uom = ?", uomPayolad.ItemCode, uomPayolad.Ean, uomPayolad.FromUom, uomPayolad.ToUom).First(&models.UomConversion{}).Error
	if errCheck1 == nil {
		uomExists = true
	}

	errCheck2 := c.DB.Where("item_code = ? AND ean = ? ", uomPayolad.ItemCode, uomPayolad.Ean).First(&models.UomConversion{}).Error
	if errCheck2 == nil {
		uomExists = true
	}

	if uomExists {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "UOM conversion already exists", "message": "UOM already exists"})
	}

	product := models.Product{}
	if err := c.DB.Where("item_code = ?", uomPayolad.ItemCode).First(&product).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	uom := models.UomConversion{
		ItemID:         product.ID,
		ItemCode:       uomPayolad.ItemCode,
		Ean:            uomPayolad.Ean,
		FromUom:        uomPayolad.FromUom,
		ToUom:          uomPayolad.ToUom,
		ConversionRate: uomPayolad.ConversionRate,
		IsBase:         uomPayolad.IsBase,
		CreatedBy:      int(ctx.Locals("userID").(float64)),
		CreatedAt:      time.Now(),
		UpdatedBy:      int(ctx.Locals("userID").(float64)),
		UpdatedAt:      time.Now(),
	}

	if err := c.DB.Debug().Create(&uom).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "UOM created successfully",
		"data":    uom,
	})
}

func (c *UomController) GetAllUOMConversion(ctx *fiber.Ctx) error {
	var uoms []models.UomConversion
	if err := c.DB.Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// if len(uoms) == 0 {
	// 	return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "No UOMs found"})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "UOMs retrieved successfully",
		"data":    uoms,
	})
}

func (c *UomController) UpdateUOMConversion(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID is required"})
	}

	var uomPayload models.UomConversionInput

	if err := ctx.BodyParser(&uomPayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	var uom models.UomConversion
	if err := c.DB.Debug().First(&uom, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM not found", "message": "No UOM found with the provided ID"})
	}

	if uom.IsLocked {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "UOM is locked", "message": "UOM is locked and cannot be updated"})
	}

	product := models.Product{}
	if err := c.DB.Where("item_code = ?", uom.ItemCode).First(&product).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	uom.ItemID = product.ID
	uom.ItemCode = uomPayload.ItemCode
	uom.Ean = uomPayload.Ean
	uom.FromUom = uomPayload.FromUom
	uom.ToUom = uomPayload.ToUom
	uom.ConversionRate = float64(uomPayload.ConversionRate)
	uom.IsBase = uomPayload.IsBase
	uom.UpdatedAt = time.Now()
	uom.UpdatedBy = int(ctx.Locals("userID").(float64))

	c.DB.Save(&uom)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM updated successfully", "data": uom})
}

func (c *UomController) GetUomByItemCode(ctx *fiber.Ctx) error {
	var payload struct {
		ItemCode string `json:"item_code" validate:"required"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	item_code := payload.ItemCode

	var uoms []models.UomConversion
	// if err := c.DB.Where("item_code = ? AND is_base = ?", item_code, true).Find(&uoms).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if err := c.DB.Where("item_code = ?", item_code).Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(uoms) == 0 {
		uoms = []models.UomConversion{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM retrieved successfully", "data": uoms})
}

func (c *UomController) GetAllUOM(ctx *fiber.Ctx) error {
	var uoms []models.Uom
	if err := c.DB.Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Uoms found", "data": uoms})
}

func (c *UomController) GetUomConversionByItemCodeAndFromUom(ctx *fiber.Ctx) error {
	var payload struct {
		ItemCode string `json:"item_code" validate:"required"`
		FromUom  string `json:"from_uom" validate:"required"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	item_code := payload.ItemCode

	var uoms models.UomConversion

	if err := c.DB.Where("item_code = ? AND from_uom = ?", item_code, payload.FromUom).First(&uoms).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM not found for item: " + item_code + " from UoM: " + payload.FromUom})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM retrieved successfully", "data": uoms})
}

// Upload from excel file

type UomConversionUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *UomController) CreateUomConversionFromExcel(ctx *fiber.Ctx) error {
	// Get file from request
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "File is required",
		})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Only Excel files (.xlsx, .xls) are allowed",
		})
	}

	// Open uploaded file
	fileContent, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to open file",
		})
	}
	defer fileContent.Close()

	// Read Excel file
	f, err := excelize.OpenReader(fileContent)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read Excel file",
		})
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "No sheets found in Excel file",
		})
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read rows",
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Excel file must contain header and at least one data row",
		})
	}

	result := UomConversionUploadResult{
		TotalRows:     len(rows) - 1,
		SuccessCount:  0,
		SkippedCount:  0,
		ErrorCount:    0,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(ctx.Locals("userID").(float64))

	// Cache for validation to reduce DB queries
	productCache := make(map[string]models.Product)
	uomCache := make(map[string]bool)

	// Start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Process each row (skip header)
	for i, row := range rows[1:] {
		rowNum := i + 2 // Excel row number (header is row 1)

		// Skip empty rows
		if len(row) == 0 || (len(row) > 0 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		// Ensure minimum columns
		if len(row) < 6 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns (expected 6)", rowNum))
			continue
		}

		// Sanitize and normalize input
		itemCode := strings.ToUpper(strings.TrimSpace(row[0]))
		ean := strings.ToUpper(strings.TrimSpace(row[1]))
		fromUom := strings.ToUpper(strings.TrimSpace(row[2]))
		toUom := strings.ToUpper(strings.TrimSpace(row[3]))
		conversionRate := parseFloat(row[4])
		isBaseStr := strings.ToUpper(strings.TrimSpace(row[5]))
		isBase := isBaseStr == "YES" || isBaseStr == "TRUE" || isBaseStr == "1"

		// Validate required fields
		if itemCode == "" || ean == "" || fromUom == "" || toUom == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Missing required fields", rowNum))
			continue
		}

		if conversionRate <= 0 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Conversion rate must be greater than 0", rowNum))
			continue
		}

		// Validate Product exists (with cache)
		product, exists := productCache[itemCode]
		if !exists {
			if err := tx.Where("item_code = ?", itemCode).First(&product).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: Product '%s' not found", rowNum, itemCode))
				continue
			}
			productCache[itemCode] = product
		}

		// Validate FromUom exists (with cache)
		if _, exists := uomCache[fromUom]; !exists {
			var fromUomModel models.Uom
			if err := tx.Where("code = ?", fromUom).First(&fromUomModel).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: FromUom '%s' not found", rowNum, fromUom))
				continue
			}
			uomCache[fromUom] = true
		}

		// Validate ToUom exists (with cache)
		if _, exists := uomCache[toUom]; !exists {
			var toUomModel models.Uom
			if err := tx.Where("code = ?", toUom).First(&toUomModel).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: ToUom '%s' not found", rowNum, toUom))
				continue
			}
			uomCache[toUom] = true
		}

		// Check for duplicate - exact match (item_code, ean, from_uom, to_uom)
		var existingUom1 models.UomConversion
		err1 := tx.Where("item_code = ? AND ean = ? AND from_uom = ? AND to_uom = ?",
			itemCode, ean, fromUom, toUom).First(&existingUom1).Error

		if err1 == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems,
				fmt.Sprintf("%s (%s→%s)", itemCode, fromUom, toUom))
			continue
		}

		// Check for duplicate - item_code and ean combination
		var existingUom2 models.UomConversion
		err2 := tx.Where("item_code = ? AND ean = ? AND from_uom = ? AND to_uom = ?",
			itemCode, ean, fromUom, toUom).First(&existingUom2).Error

		if err2 == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems,
				fmt.Sprintf("%s (EAN: %s)", itemCode, ean))
			continue
		}

		// Create UOM Conversion
		uomConversion := models.UomConversion{
			ItemID:         product.ID,
			ItemCode:       itemCode,
			Ean:            ean,
			FromUom:        fromUom,
			ToUom:          toUom,
			ConversionRate: conversionRate,
			IsBase:         isBase,
			CreatedBy:      userID,
			CreatedAt:      time.Now(),
			UpdatedBy:      userID,
			UpdatedAt:      time.Now(),
		}

		if err := tx.Create(&uomConversion).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create UOM conversion - %s", rowNum, err.Error()))
			continue
		}

		result.SuccessCount++
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to commit transaction",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Upload completed: %d success, %d skipped, %d errors",
			result.SuccessCount, result.SkippedCount, result.ErrorCount),
		"data": result,
	})
}
