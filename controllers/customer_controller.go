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

type CustomerController struct {
	DB *gorm.DB
}

var customerInput struct {
	ID           uint   `json:"id"`
	CustomerCode string `json:"customer_code" validate:"required,min=3"`
	CustomerName string `json:"customer_name" validate:"required,min=3"`
	CustAddr1    string `json:"cust_addr1"`
	CustAddr2    string `json:"cust_addr2"`
	CustCity     string `json:"cust_city"`
	CustArea     string `json:"cust_area"`
	CustPhone    string `json:"cust_phone"`
	CustCountry  string `json:"cust_country"`
	CustEmail    string `json:"cust_email"`
	OwnerCode    string `json:"owner_code"`
}

func NewCustomerController(db *gorm.DB) *CustomerController {
	return &CustomerController{DB: db}
}

func (c *CustomerController) GetAllCustomers(ctx *fiber.Ctx) error {
	var customers []models.Customer
	if err := c.DB.Find(&customers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customers found", "data": customers})
}

func (c *CustomerController) GetCustomerByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Customer
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer found", "data": result})
}

func (c *CustomerController) CreateCustomer(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&customerInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var owner models.Owner
	if err := c.DB.First(&owner, "code = ?", customerInput.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
	}

	customer := models.Customer{
		OwnerCode:    customerInput.OwnerCode,
		CustomerCode: customerInput.CustomerCode,
		CustomerName: customerInput.CustomerName,
		CustAddr1:    customerInput.CustAddr1,
		CustAddr2:    customerInput.CustAddr2,
		CustCity:     customerInput.CustCity,
		CustArea:     customerInput.CustArea,
		CustPhone:    customerInput.CustPhone,
		CustCountry:  customerInput.CustCountry,
		CustEmail:    customerInput.CustEmail,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer created successfully", "data": customer})
}

func (c *CustomerController) UpdateCustomer(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := ctx.BodyParser(&customerInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var owner models.Owner
	if err := c.DB.First(&owner, "code = ?", customerInput.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
	}

	if err := c.DB.Debug().
		Model(&models.Customer{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			// "customer_code": customerInput.CustomerCode,
			"owner_code":    customerInput.OwnerCode,
			"customer_name": customerInput.CustomerName,
			"cust_addr1":    customerInput.CustAddr1,
			"cust_city":     customerInput.CustCity,
			"cust_area":     customerInput.CustArea,
			"cust_phone":    customerInput.CustPhone,
			"cust_country":  customerInput.CustCountry,
			"cust_email":    customerInput.CustEmail,
			"updated_by":    int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// customer := models.Customer{
	// 	CustomerCode: customerInput.CustomerCode,
	// 	CustomerName: customerInput.CustomerName,
	// 	CustAddr1:    customerInput.CustAddr1,
	// 	CustAddr2:    customerInput.CustAddr2,
	// 	CustCity:     customerInput.CustCity,
	// 	CustArea:     customerInput.CustArea,
	// 	CustPhone:    customerInput.CustPhone,
	// 	CustEmail:    customerInput.CustEmail,
	// 	CustCountry:  customerInput.CustCountry,
	// 	UpdatedBy:    int(ctx.Locals("userID").(float64)),
	// }

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	// if err := c.DB.Select("customer_code", "customer_name", "updated_by").Where("id = ?", id).Updates(&customer).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer updated successfully", "data": customerInput})
}

func (c *CustomerController) DeleteCustomer(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var customer models.Customer
	if err := c.DB.First(&customer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Add CustomerID to DeletedBy field
	customer.DeletedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	if err := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hapus customer
	if err := c.DB.Delete(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer deleted successfully", "data": customer})
}

// ============================================================================
// Begin upload customer from excel file
// ============================================================================

type CustomerUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *CustomerController) CreateCustomerFromExcel(ctx *fiber.Ctx) error {
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

	result := CustomerUploadResult{
		TotalRows:     len(rows) - 1,
		SuccessCount:  0,
		SkippedCount:  0,
		ErrorCount:    0,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(ctx.Locals("userID").(float64))

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
		if len(row) < 9 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns (expected 9)", rowNum))
			continue
		}

		// Sanitize and normalize input
		customerCode := strings.ToUpper(strings.TrimSpace(row[0]))
		customerName := strings.TrimSpace(row[1])
		custAddr1 := strings.TrimSpace(row[2])
		custAddr2 := strings.TrimSpace(row[3])
		custCity := strings.TrimSpace(row[4])
		custArea := strings.TrimSpace(row[5])
		custPhone := strings.TrimSpace(row[6])
		custCountry := strings.TrimSpace(row[7])
		custEmail := strings.TrimSpace(row[8])
		custOwnerCode := strings.ToUpper(strings.TrimSpace(row[9]))

		// Validate required fields
		if customerCode == "" || customerName == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: CUSTOMER_CODE and CUSTOMER_NAME are required", rowNum))
			continue
		}

		// Check if customer code already exists
		var existingCustomer models.Customer
		if err := tx.Where("customer_code = ?", customerCode).First(&existingCustomer).Error; err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems, customerCode)
			continue
		}

		// Validate email format if provided
		if custEmail != "" && !isValidEmail(custEmail) {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Invalid email format '%s'", rowNum, custEmail))
			continue
		}

		// Validate owner code
		var owner models.Owner
		if err := tx.Where("code = ?", custOwnerCode).First(&owner).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Invalid owner code '%s'", rowNum, custOwnerCode))
			continue
		}

		// Create customer
		customer := models.Customer{
			OwnerCode:    owner.Code,
			CustomerCode: customerCode,
			CustomerName: customerName,
			CustAddr1:    custAddr1,
			CustAddr2:    custAddr2,
			CustCity:     custCity,
			CustArea:     custArea,
			CustPhone:    custPhone,
			CustCountry:  custCountry,
			CustEmail:    custEmail,
			CreatedBy:    userID,
		}

		if err := tx.Create(&customer).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create customer - %s", rowNum, err.Error()))
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

// Helper function to validate email format
func isValidEmail(email string) bool {
	if email == "" {
		return true // Empty email is valid (optional field)
	}
	// Simple email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

//==============================================================================
// End Upload Customer From Excel
//==============================================================================

//==============================================================================
// Begin Export Customer To Excel
//==============================================================================

func (c *CustomerController) ExportCustomers(ctx *fiber.Ctx) error {
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
	var customers []models.Customer
	query := c.DB.Model(&models.Customer{})

	// Filter by owner codes if provided
	if len(req.OwnerCodes) > 0 {
		query = query.Where("owner_code IN ?", req.OwnerCodes)
	}

	if err := query.Order("created_at DESC").Find(&customers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch customers",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Customers retrieved successfully",
		"data":    customers,
	})
}

// Add this to get unique owner codes for the dropdown
func (c *CustomerController) GetOwnerCodes(ctx *fiber.Ctx) error {
	var ownerCodes []string

	if err := c.DB.Model(&models.Customer{}).
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

//==============================================================================
// End Export Customer To Excel
//==============================================================================
