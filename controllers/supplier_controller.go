package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type SupplierController struct {
	DB *gorm.DB
}

var supplierInput struct {
	OwnerCode    string `json:"owner_code"`
	SupplierCode string `json:"supplier_code" gorm:"unique"`
	SupplierName string `json:"supplier_name" gorm:"unique"`
	SuppAddr1    string `json:"supp_addr1"`
	SuppCity     string `json:"supp_city"`
	SuppCountry  string `json:"supp_country"`
	SuppPhone    string `json:"supp_phone"`
	SuppEmail    string `json:"supp_email"`
}

func NewSupplierController(db *gorm.DB) *SupplierController {
	return &SupplierController{DB: db}
}

func (c *SupplierController) CreateSupplier(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&supplierInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var owner models.Owner
	if err := c.DB.First(&owner, "code = ?", supplierInput.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
	}

	supplier := models.Supplier{
		OwnerCode:    supplierInput.OwnerCode,
		SupplierCode: supplierInput.SupplierCode,
		SupplierName: supplierInput.SupplierName,
		SuppAddr1:    supplierInput.SuppAddr1,
		SuppCity:     supplierInput.SuppCity,
		SuppCountry:  supplierInput.SuppCountry,
		SuppPhone:    supplierInput.SuppPhone,
		SuppEmail:    supplierInput.SuppEmail,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier created successfully", "data": supplier})
}

func (c *SupplierController) GetAllSuppliers(ctx *fiber.Ctx) error {
	var suppliers []models.Supplier
	if err := c.DB.Find(&suppliers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Suppliers found", "data": suppliers})
}

func (c *SupplierController) GetSupplierByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Supplier
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier found", "data": result})
}

func (c *SupplierController) UpdateSupplier(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := ctx.BodyParser(&supplierInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var owner models.Owner
	if err := c.DB.First(&owner, "code = ?", supplierInput.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
	}

	supplier := models.Supplier{
		SupplierCode: supplierInput.SupplierCode,
		SupplierName: supplierInput.SupplierName,
		SuppAddr1:    supplierInput.SuppAddr1,
		SuppCity:     supplierInput.SuppCity,
		SuppCountry:  supplierInput.SuppCountry,
		SuppPhone:    supplierInput.SuppPhone,
		SuppEmail:    supplierInput.SuppEmail,
		OwnerCode:    supplierInput.OwnerCode,
		UpdatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Model(&supplier).Where("id = ?", id).Updates(supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier updated successfully", "data": supplier})
}

func (c *SupplierController) DeleteSupplier(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var supplier models.Supplier
	if err := c.DB.First(&supplier, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Add SupplierID to DeletedBy field
	supplier.DeletedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	if err := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Delete(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier deleted successfully", "data": supplier})
}

// =========================================================================
// Begin upload supplier from excel file
// =========================================================================

type SupplierUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *SupplierController) CreateSupplierFromExcel(ctx *fiber.Ctx) error {
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

	result := SupplierUploadResult{
		TotalRows:     len(rows) - 1,
		SuccessCount:  0,
		SkippedCount:  0,
		ErrorCount:    0,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(ctx.Locals("userID").(float64))

	// Cache for owner validation
	ownerCache := make(map[string]bool)

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
		if len(row) < 7 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns (expected 7)", rowNum))
			continue
		}

		// Sanitize and normalize input
		supplierCode := strings.ToUpper(strings.TrimSpace(row[0]))
		supplierName := strings.TrimSpace(row[1])
		suppAddr1 := strings.TrimSpace(row[2])
		suppCity := strings.TrimSpace(row[3])
		suppCountry := strings.TrimSpace(row[4])
		suppPhone := strings.TrimSpace(row[5])
		suppEmail := strings.TrimSpace(row[6])
		ownerCode := strings.ToUpper(strings.TrimSpace(row[7]))

		// Validate required fields
		if supplierCode == "" || supplierName == "" || ownerCode == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: SUPPLIER_CODE, SUPPLIER_NAME, and OWNER_CODE are required", rowNum))
			continue
		}

		// Validate Owner exists (with cache)
		if _, exists := ownerCache[ownerCode]; !exists {
			var owner models.Owner
			if err := tx.Where("code = ?", ownerCode).First(&owner).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: Owner '%s' not found", rowNum, ownerCode))
				continue
			}
			ownerCache[ownerCode] = true
		}

		// Check if supplier code already exists
		var existingSupplier models.Supplier
		if err := tx.Where("supplier_code = ?", supplierCode).First(&existingSupplier).Error; err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems, supplierCode)
			continue
		}

		// Validate email format if provided
		if suppEmail != "" && !isValidEmail(suppEmail) {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Invalid email format '%s'", rowNum, suppEmail))
			continue
		}

		// Create supplier
		supplier := models.Supplier{
			SupplierCode: supplierCode,
			SupplierName: supplierName,
			SuppAddr1:    suppAddr1,
			SuppCity:     suppCity,
			SuppCountry:  suppCountry,
			SuppPhone:    suppPhone,
			SuppEmail:    suppEmail,
			OwnerCode:    ownerCode,
			CreatedBy:    userID,
		}

		if err := tx.Create(&supplier).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create supplier - %s", rowNum, err.Error()))
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

// ==========================================================================
// End Upload Supplier From Excel
// ==========================================================================

// ==========================================================================
// Begin Export Supplier To Excel
// ==========================================================================

func (c *SupplierController) ExportSuppliers(ctx *fiber.Ctx) error {
	// Parse request body
	type ExportRequest struct {
		OwnerCodes []string `json:"owner_codes"`
	}

	var req ExportRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Build query
	var suppliers []models.Supplier
	query := c.DB.Model(&models.Supplier{})

	// Filter by owner codes if provided
	if len(req.OwnerCodes) > 0 {
		query = query.Where("owner_code IN ?", req.OwnerCodes)
	}

	if err := query.Order("created_at DESC").Find(&suppliers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch suppliers",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Suppliers retrieved successfully",
		"data":    suppliers,
	})
}

// Add this to get unique owner codes for the dropdown
func (c *SupplierController) GetOwnerCodes(ctx *fiber.Ctx) error {
	var ownerCodes []string

	if err := c.DB.Model(&models.Supplier{}).
		Distinct("owner_code").
		Where("owner_code IS NOT NULL AND owner_code != ''").
		Order("owner_code ASC").
		Pluck("owner_code", &ownerCodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch owner codes",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Owner codes retrieved successfully",
		"data":    ownerCodes,
	})
}

// supplierGroup.Get("/owner-codes", supplierController.GetOwnerCodes)
// supplierGroup.Post("/export", supplierController.ExportSuppliers)

//===========================================================================
// End Export Supplier To Excel
// ==========================================================================
