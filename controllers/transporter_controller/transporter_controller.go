package transporter_controller

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type TransporterController struct {
	DB *gorm.DB
}

func NewTransporterController(db *gorm.DB) *TransporterController {
	return &TransporterController{DB: db}
}

func (c *TransporterController) CreateTransporter(ctx *fiber.Ctx) error {

	var transporter models.Transporter

	if err := ctx.BodyParser(&transporter); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	transporter.CreatedBy = int(ctx.Locals("userID").(float64))

	fmt.Println("Payload Data : ", transporter)

	// return nil

	if err := c.DB.Create(&transporter).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter created successfully", "data": transporter})
}

func (c *TransporterController) GetAllTransporter(ctx *fiber.Ctx) error {

	var transporters []models.Transporter
	if err := c.DB.Find(&transporters).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporters found", "data": transporters})
}

func (c *TransporterController) GetTransporterByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Transporter
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Transporter not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter found", "data": result})
}

func (c *TransporterController) UpdateTransporter(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var transporter models.Transporter
	if err := ctx.BodyParser(&transporter); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	transporter.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&transporter).Where("id = ?", id).Updates(transporter).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter updated successfully", "data": transporter})
}

// =======================================================================
// Begin Export Transporter To Excel
// =======================================================================

func (c *TransporterController) ExportTransporters(ctx *fiber.Ctx) error {
	var transporters []models.Transporter

	if err := c.DB.Order("transporter_code ASC").Find(&transporters).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch transporters",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Transporters retrieved successfully",
		"data":    transporters,
	})
}

// ==========================================================================
// End Export Transporter To Excel
// ==========================================================================

type TransporterUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *TransporterController) CreateTransporterFromExcel(ctx *fiber.Ctx) error {
	// 1. Get uploaded file
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "File is required",
		})
	}

	// 2. Validate extension
	if !strings.EqualFold(
		file.Filename[len(file.Filename)-len(".xlsx"):],
		".xlsx",
	) || len(file.Filename) < len(".xlsx") {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Only Excel .xlsx files are allowed",
		})
	}

	// 3. Open file
	fileContent, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to open file",
		})
	}
	defer fileContent.Close()

	// 4. Read Excel
	f, err := excelize.OpenReader(fileContent)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read Excel file",
		})
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "No sheets found in Excel file",
		})
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read Excel rows",
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Excel must contain header and at least one data row",
		})
	}

	// 5. Initialize result
	result := TransporterUploadResult{
		TotalRows:     len(rows) - 1,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(ctx.Locals("userID").(float64))

	// 6. Start transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to start transaction",
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 7. Process Excel rows
	for i, row := range rows[1:] {
		rowNum := i + 2

		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		isEmpty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			continue
		}

		// Required minimum columns
		if len(row) < 8 {
			result.ErrorCount++
			result.ErrorMessages = append(
				result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns (expected at least 8)", rowNum),
			)
			continue
		}

		// Read columns
		transporterCode := strings.ToUpper(strings.TrimSpace(row[0]))
		transporterName := strings.TrimSpace(row[1])
		address := strings.TrimSpace(row[2])
		city := strings.TrimSpace(row[3])
		pic := strings.TrimSpace(row[4])
		phone := strings.TrimSpace(row[5])
		email := strings.TrimSpace(row[6])
		fax := strings.TrimSpace(row[7])

		// Validate required fields
		if transporterCode == "" || transporterName == "" {
			result.ErrorCount++
			result.ErrorMessages = append(
				result.ErrorMessages,
				fmt.Sprintf("Row %d: TRANSPORTER_CODE and TRANSPORTER_NAME are required", rowNum),
			)
			continue
		}

		// Validate email
		if email != "" && !helpers.IsValidEmail(email) {
			result.ErrorCount++
			result.ErrorMessages = append(
				result.ErrorMessages,
				fmt.Sprintf("Row %d: Invalid email format '%s'", rowNum, email),
			)
			continue
		}

		// Parse IS_ACTIVE (column 9, optional)
		isActive := true
		if len(row) > 8 && strings.TrimSpace(row[8]) != "" {
			activeValue := strings.ToUpper(strings.TrimSpace(row[8]))

			switch activeValue {
			case "TRUE", "1", "YES", "Y", "ACTIVE":
				isActive = true
			case "FALSE", "0", "NO", "N", "INACTIVE":
				isActive = false
			default:
				result.ErrorCount++
				result.ErrorMessages = append(
					result.ErrorMessages,
					fmt.Sprintf("Row %d: Invalid IS_ACTIVE value '%s'. Use TRUE or FALSE", rowNum, row[8]),
				)
				continue
			}
		}

		// Check duplicate transporter code
		var existing models.Transporter
		err := tx.Where("transporter_code = ?", transporterCode).
			First(&existing).Error

		if err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems, transporterCode)
			continue
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			result.ErrorCount++
			result.ErrorMessages = append(
				result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to check transporter - %s", rowNum, err.Error()),
			)
			continue
		}

		// Create transporter
		transporter := models.Transporter{
			TransporterCode:    transporterCode,
			TransporterName:    transporterName,
			TransporterAddress: address,
			City:               city,
			Pic:                pic,
			Phone:              phone,
			Email:              email,
			Fax:                fax,
			IsActive:           isActive,
			CreatedBy:          userID,
		}

		if err := tx.Create(&transporter).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(
				result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create transporter - %s", rowNum, err.Error()),
			)
			continue
		}

		result.SuccessCount++
	}

	// 8. Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to commit transaction: " + err.Error(),
		})
	}

	// 9. Return result
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf(
			"Upload completed: %d success, %d skipped, %d errors",
			result.SuccessCount,
			result.SkippedCount,
			result.ErrorCount,
		),
		"data": result,
	})
}
