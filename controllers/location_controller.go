package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type LocationController struct {
	DB *gorm.DB
}

func NewLocationController(DB *gorm.DB) *LocationController {
	return &LocationController{DB: DB}
}

// CREATE
func (lc *LocationController) CreateLocation(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := ctx.BodyParser(&location); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	location.LocationCode = location.Row + location.Bay + location.Level + location.Bin

	warehouse := models.Warehouse{}
	if err := lc.DB.First(&warehouse, "code = ?", location.WhsCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	location.WhsCode = warehouse.Code

	bayInt, err := strconv.Atoi(location.Bay)
	if err != nil {
		location.Area = "Unknown"
	} else {
		if bayInt%2 != 0 {
			location.Area = "ganjil"
		} else {
			location.Area = "genap"
		}
	}

	location.CreatedBy = userID
	location.UpdatedBy = userID

	if err := lc.DB.Create(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Location created successfully",
		"data":    location,
	})
}

// READ ALL
func (lc *LocationController) GetAllLocations(ctx *fiber.Ctx) error {
	var locations []models.Location
	if err := lc.DB.Find(&locations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    locations,
	})
}

// READ BY ID
func (lc *LocationController) GetLocationByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var location models.Location

	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    location,
	})
}

// UPDATE
func (lc *LocationController) UpdateLocation(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	var input models.Location
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	bayInt, err := strconv.Atoi(location.Bay)
	if err != nil {
		location.Area = "Unknown"
	} else {
		if bayInt%2 != 0 {
			location.Area = "ganjil"
		} else {
			location.Area = "genap"
		}
	}

	location.WhsCode = input.WhsCode
	location.LocationCode = input.Row + input.Bay + input.Level + input.Bin
	location.Row = input.Row
	location.Bay = input.Bay
	location.Level = input.Level
	location.Bin = input.Bin
	// location.Area = input.Area
	location.IsActive = input.IsActive
	location.UpdatedBy = userID

	if err := lc.DB.Save(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Location updated successfully",
		"data":    location,
	})
}

// DELETE
func (lc *LocationController) DeleteLocation(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	location.DeletedBy = userID
	if err := lc.DB.Save(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := lc.DB.Delete(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Location deleted successfully",
	})
}

//====================================================================
// BEGIN CREATE LOCATION FROM EXCEL
//====================================================================

type ExcelLocationUploadResponse struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	TotalRows        int               `json:"total_rows"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	CreatedLocations []string          `json:"created_locations,omitempty"`
	Errors           []ExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

type ExcelLocationDetail struct {
	LocationCode string
	WhsCode      string
	Row          int
}

func (lc *LocationController) CreateLocationFromExcel(ctx *fiber.Ctx) error {
	// Parse uploaded file
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Error", Detail: err.Error()},
			},
		})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "File size exceeds maximum limit of 10MB",
		})
	}

	// Open uploaded file
	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Processing Error", Detail: err.Error()},
			},
		})
	}
	defer fileHeader.Close()

	// Read Excel file
	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Excel Read Error", Detail: err.Error()},
			},
		})
	}
	defer excelFile.Close()

	// Get first sheet
	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Excel file contains no sheets",
		})
	}

	sheetName := sheets[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Sheet Read Error", Detail: err.Error()},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Excel file must contain at least header row and one data row",
		})
	}

	// Get user ID
	userID := int(ctx.Locals("userID").(float64))

	// Parse detail rows
	details, validationErrors := lc.parseLocationDetailsFromExcel(rows)
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d errors", len(validationErrors)),
			ValidationErrors: validationErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	if len(details) < 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success:   false,
			Message:   "No valid locations found in Excel file",
			TotalRows: len(rows) - 1,
		})
	}

	// Check for duplicate location codes in Excel
	duplicateErrors := lc.checkDuplicateLocations(details)
	if len(duplicateErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success:          false,
			Message:          "Duplicate location codes found in Excel file",
			ValidationErrors: duplicateErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// Start transaction
	tx := lc.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateLocationFromExcel: %v", r)
		}
	}()

	// Validate all warehouses exist
	warehouseValidationErrors := lc.validateWarehouses(tx, details)
	if len(warehouseValidationErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success:          false,
			Message:          "Warehouse validation failed",
			ValidationErrors: warehouseValidationErrors,
			TotalRows:        len(details),
		})
	}

	// Check existing locations in database
	existingErrors := lc.checkExistingLocations(tx, details)
	if len(existingErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelLocationUploadResponse{
			Success:          false,
			Message:          "Some locations already exist in database",
			ValidationErrors: existingErrors,
			TotalRows:        len(details),
		})
	}

	var createdLocations []string
	successCount := 0

	// Create locations
	for _, detail := range details {
		// Parse location code (format: YMK49B102)
		// Example: YMK49B102
		// Row: YMK (positions 0-2)
		// Bay: 49 (positions 3-4)
		// Level: B1 (positions 5-6)
		// Bin: 02 (positions 7-8)

		row := detail.LocationCode[0:2]   // "YMK"
		bay := detail.LocationCode[2:4]   // "49"
		level := detail.LocationCode[4:6] // "B1"
		bin := detail.LocationCode[6:8]   // "02"

		// Determine area based on bay number (odd/even)
		area := "Unknown"
		bayInt, err := strconv.Atoi(bay)
		if err == nil {
			if bayInt%2 != 0 {
				area = "ganjil"
			} else {
				area = "genap"
			}
		}

		location := models.Location{
			LocationCode: detail.LocationCode,
			WhsCode:      detail.WhsCode,
			Row:          row,
			Bay:          bay,
			Level:        level,
			Bin:          bin,
			Area:         area,
			IsActive:     true,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}

		if err := tx.Create(&location).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelLocationUploadResponse{
				Success: false,
				Message: "Failed to create location",
				Errors: []ExcelRowError{
					{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		createdLocations = append(createdLocations, detail.LocationCode)
		successCount++
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelLocationUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()},
			},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(ExcelLocationUploadResponse{
		Success:          true,
		Message:          fmt.Sprintf("Successfully created %d locations", successCount),
		TotalRows:        len(details),
		SuccessCount:     successCount,
		FailedCount:      0,
		CreatedLocations: createdLocations,
	})
}

// Helper functions
func (lc *LocationController) parseLocationDetailsFromExcel(rows [][]string) ([]ExcelLocationDetail, []ValidationError) {
	var details []ExcelLocationDetail
	var errors []ValidationError

	// Start from row 2 (index 1), assuming row 1 is header
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		// Skip completely empty rows
		if len(row) == 0 || (strings.TrimSpace(getCellx(row, 0)) == "" && strings.TrimSpace(getCellx(row, 1)) == "") {
			continue
		}

		detail := ExcelLocationDetail{Row: rowNum}

		// Parse columns
		detail.LocationCode = strings.TrimSpace(strings.ToUpper(getCellx(row, 0)))
		detail.WhsCode = strings.TrimSpace(strings.ToUpper(getCellx(row, 1)))

		// Validate required fields
		if detail.LocationCode == "" {
			errors = append(errors, ValidationError{
				Field:   "LocationCode",
				Message: "Location code cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		if detail.WhsCode == "" {
			errors = append(errors, ValidationError{
				Field:   "WhsCode",
				Message: "Warehouse code cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		// Validate location code length (must be exactly 8 characters)
		if len(detail.LocationCode) != 8 {
			errors = append(errors, ValidationError{
				Field:   "LocationCode",
				Message: fmt.Sprintf("Location code must be exactly 8 characters. Current: %d characters (%s)", len(detail.LocationCode), detail.LocationCode),
				Row:     rowNum,
			})
			continue
		}

		// Validate location code format (alphanumeric only)
		for _, char := range detail.LocationCode {
			if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
				errors = append(errors, ValidationError{
					Field:   "LocationCode",
					Message: "Location code must contain only uppercase letters and numbers",
					Row:     rowNum,
				})
				break
			}
		}

		// Validate bay (positions 3-4) must be numeric
		bay := detail.LocationCode[3:5]
		if _, err := strconv.Atoi(bay); err != nil {
			errors = append(errors, ValidationError{
				Field:   "LocationCode",
				Message: fmt.Sprintf("Bay (positions 4-5) must be numeric. Current bay: %s", bay),
				Row:     rowNum,
			})
			continue
		}

		// Validate bin (positions 7-8) must be numeric
		bin := detail.LocationCode[7:]
		if _, err := strconv.Atoi(bin); err != nil {
			errors = append(errors, ValidationError{
				Field:   "LocationCode",
				Message: fmt.Sprintf("Bin (positions 8-9) must be numeric. Current bin: %s", bin),
				Row:     rowNum,
			})
			continue
		}

		details = append(details, detail)
	}

	return details, errors
}

func (lc *LocationController) checkDuplicateLocations(details []ExcelLocationDetail) []ValidationError {
	var errors []ValidationError
	locationMap := make(map[string]int)

	for _, detail := range details {
		if existingRow, exists := locationMap[detail.LocationCode]; exists {
			errors = append(errors, ValidationError{
				Field:   "Duplicate",
				Message: fmt.Sprintf("Duplicate location code found (same as row %d): %s", existingRow, detail.LocationCode),
				Row:     detail.Row,
			})
		} else {
			locationMap[detail.LocationCode] = detail.Row
		}
	}

	return errors
}

func (lc *LocationController) validateWarehouses(tx *gorm.DB, details []ExcelLocationDetail) []ValidationError {
	var errorss []ValidationError
	warehouseMap := make(map[string]bool)

	// Get unique warehouse codes
	uniqueWhsCodes := make(map[string][]int)
	for _, detail := range details {
		uniqueWhsCodes[detail.WhsCode] = append(uniqueWhsCodes[detail.WhsCode], detail.Row)
	}

	// Check each warehouse exists
	for whsCode, rows := range uniqueWhsCodes {
		if _, checked := warehouseMap[whsCode]; !checked {
			var warehouse models.Warehouse
			if err := tx.Where("code = ?", whsCode).First(&warehouse).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Add error for all rows with this warehouse code
					for _, rowNum := range rows {
						errorss = append(errorss, ValidationError{
							Field:   "WhsCode",
							Message: fmt.Sprintf("Warehouse not found: %s", whsCode),
							Row:     rowNum,
						})
					}
				} else {
					// Database error
					for _, rowNum := range rows {
						errorss = append(errorss, ValidationError{
							Field:   "WhsCode",
							Message: fmt.Sprintf("Failed to validate warehouse: %s", err.Error()),
							Row:     rowNum,
						})
					}
				}
				warehouseMap[whsCode] = false
			} else {
				warehouseMap[whsCode] = true
			}
		}
	}

	return errorss
}

func (lc *LocationController) checkExistingLocations(tx *gorm.DB, details []ExcelLocationDetail) []ValidationError {
	var errorss []ValidationError

	for _, detail := range details {
		var existingLocation models.Location
		if err := tx.Where("location_code = ?", detail.LocationCode).First(&existingLocation).Error; err == nil {
			errorss = append(errorss, ValidationError{
				Field:   "LocationCode",
				Message: fmt.Sprintf("Location already exists in database: %s", detail.LocationCode),
				Row:     detail.Row,
			})
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			errorss = append(errorss, ValidationError{
				Field:   "LocationCode",
				Message: fmt.Sprintf("Failed to check location: %s", err.Error()),
				Row:     detail.Row,
			})
		}
	}

	return errorss
}

func getCellx(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

//====================================================================
// END CREATE LOCATION FROM EXCEL
//====================================================================
