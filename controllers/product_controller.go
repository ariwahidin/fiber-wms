package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ProductController struct {
	DB *gorm.DB
}

func NewProductController(DB *gorm.DB) *ProductController {
	return &ProductController{DB: DB}
}

var productInput struct {
	ID         uint    `json:"id"`
	ItemCode   string  `json:"item_code" validate:"required,min=3"`
	ItemName   string  `json:"item_name" validate:"required,min=3"`
	CBM        float64 `json:"cbm" validate:"required"`
	GMC        string  `json:"gmc" validate:"required,min=6"`
	Width      float64 `json:"width"`
	Length     float64 `json:"length"`
	Height     float64 `json:"height"`
	Weight     float64 `json:"weight"`
	Color      string  `json:"color" gorm:"default:null"`
	Group      string  `json:"group" gorm:"default:null"`
	Category   string  `json:"category" gorm:"default:null"`
	Serial     string  `json:"serial" validate:"required,min=1"`
	Waranty    string  `json:"waranty" validate:"required,min=1"`
	Adaptor    string  `json:"adaptor" validate:"required,min=1"`
	ManualBook string  `json:"manual_book" validate:"required,min=1"`
	Uom        string  `json:"uom" validate:"required,min=3"`
	OwnerCode  string  `json:"owner_code" validate:"required,min=3"`
}

func (c *ProductController) CreateProduct(ctx *fiber.Ctx) error {

	// Parse Body
	if err := ctx.BodyParser(&productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	Uom := models.Uom{}
	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
	if Uom.ID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	// Membuat user dengan memasukkan data ke struct models.Product
	product := models.Product{
		ItemCode:   productInput.ItemCode,
		ItemName:   productInput.ItemName,
		CBM:        productInput.CBM,
		Barcode:    productInput.GMC,
		GMC:        productInput.GMC,
		Width:      productInput.Width,
		Length:     productInput.Length,
		Height:     productInput.Height,
		Weight:     productInput.Weight,
		Color:      productInput.Color,
		Group:      productInput.Group,
		Category:   productInput.Category,
		HasSerial:  productInput.Serial,
		HasWaranty: productInput.Waranty,
		HasAdaptor: productInput.Adaptor,
		ManualBook: productInput.ManualBook,
		Uom:        productInput.Uom,
		OwnerCode:  productInput.OwnerCode,
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&product).Error; err != nil {
		c.DB.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	uomConversion := models.UomConversion{
		ItemID:         product.ID,
		ItemCode:       product.ItemCode,
		Ean:            productInput.GMC,
		FromUom:        product.Uom,
		ToUom:          product.Uom,
		IsBase:         true,
		ConversionRate: 1,
		CreatedBy:      int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&uomConversion).Error; err != nil {
		// Jika terjadi error saat membuat UomConversion, rollback perubahan pada Product
		c.DB.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Product created successfully", "data": product})

}

func (c *ProductController) GetProductByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Periksa apakah user dengan ID tersebut ada
	var result models.Product
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product found", "data": result})
}

func (c *ProductController) UpdateProduct(ctx *fiber.Ctx) error {

	fmt.Println("Payload Edit Data : ", string(ctx.Body()))
	// return nil

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Check if the product exists
	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse Body
	if err := ctx.BodyParser(&productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	Uom := models.Uom{}
	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
	if Uom.ID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	if err := c.DB.Debug().
		Model(&models.Product{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"item_code":   productInput.ItemCode,
			"item_name":   productInput.ItemName,
			"cbm":         productInput.CBM,
			"gmc":         productInput.GMC,
			"barcode":     productInput.GMC,
			"group":       productInput.Group,
			"category":    productInput.Category,
			"width":       productInput.Width,
			"length":      productInput.Length,
			"height":      productInput.Height,
			"weight":      productInput.Weight,
			"color":       productInput.Color,
			"has_serial":  productInput.Serial,
			"has_waranty": productInput.Waranty,
			"has_adaptor": productInput.Adaptor,
			"manual_book": productInput.ManualBook,
			"uom":         productInput.Uom,
			"owner_code":  productInput.OwnerCode,
			"updated_at":  time.Now(),
			"updated_by":  int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	uomConversion := models.UomConversion{}
	c.DB.Where("item_code = ? AND from_uom = ?", product.ItemCode, productInput.Uom).First(&uomConversion)
	if uomConversion.ID != 0 {
		// Update existing UomConversion
		if err := c.DB.Debug().
			Model(&models.UomConversion{}).
			Where("item_code = ? AND from_uom = ?", product.ItemCode, productInput.Uom).
			Updates(map[string]interface{}{
				"ean":             productInput.GMC,
				"from_uom":        productInput.Uom,
				"to_uom":          productInput.Uom,
				"updated_at":      time.Now(),
				"updated_by":      int(ctx.Locals("userID").(float64)),
				"is_base":         false,
				"conversion_rate": 1,
			}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	} else {
		// Create new UomConversion
		newUomConversion := models.UomConversion{
			ItemCode:       product.ItemCode,
			Ean:            product.Barcode,
			FromUom:        productInput.Uom,
			ToUom:          productInput.Uom,
			ConversionRate: 1,
			IsBase:         false,
			CreatedAt:      time.Now(),
			CreatedBy:      int(ctx.Locals("userID").(float64)),
			UpdatedAt:      time.Now(),
			UpdatedBy:      int(ctx.Locals("userID").(float64)),
		}
		if err := c.DB.Create(&newUomConversion).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product updated successfully", "data": product})

}

func (c *ProductController) GetAllProducts(ctx *fiber.Ctx) error {

	if ctx.Query("owner") != "" {
		var products []models.Product
		if err := c.DB.Where("owner_code = ?", ctx.Query("owner")).Order("item_code ASC").Find(&products).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
	}

	var products []models.Product
	if err := c.DB.Order("item_code ASC").Find(&products).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
}

func (c *ProductController) GetAllCategory(ctx *fiber.Ctx) error {

	var categories []models.Category
	if err := c.DB.Find(&categories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Categories found", "data": categories})
}

func (c *ProductController) DeleteProduct(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Periksa apakah user dengan ID tersebut ada
	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&product)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Hapus user
	result = c.DB.Delete(&product)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product deleted successfully", "data": product})
}

// Upload Via Excel File

type ExcelUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *ProductController) CreateProductFromExcelFile(ctx *fiber.Ctx) error {
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

	result := ExcelUploadResult{
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
		if len(row) < 16 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns", rowNum))
			continue
		}

		// Sanitize and normalize input
		itemCode := strings.ToUpper(strings.TrimSpace(row[0]))
		itemName := strings.TrimSpace(row[1])
		width := parseFloat(row[2])
		length := parseFloat(row[3])
		height := parseFloat(row[4])
		cbm := calculateCBM(width, length, height) // Auto calculate CBM
		gmc := strings.ToUpper(strings.TrimSpace(row[5]))
		weight := parseFloat(row[6])
		color := strings.TrimSpace(row[7])
		group := strings.TrimSpace(row[8])
		category := strings.TrimSpace(row[9])
		serial := strings.ToUpper(strings.TrimSpace(row[10]))
		warranty := strings.ToUpper(strings.TrimSpace(row[11]))
		adaptor := strings.ToUpper(strings.TrimSpace(row[12]))
		manualBook := strings.ToUpper(strings.TrimSpace(row[13]))
		uom := strings.ToUpper(strings.TrimSpace(row[14]))
		ownerCode := strings.ToUpper(strings.TrimSpace(row[15]))

		// Validate required fields
		if itemCode == "" || itemName == "" || gmc == "" ||
			serial == "" || warranty == "" || adaptor == "" ||
			manualBook == "" || uom == "" || ownerCode == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Missing required fields", rowNum))
			continue
		}

		// Check if item code already exists
		var existingProduct models.Product
		if err := tx.Where("item_code = ?", itemCode).First(&existingProduct).Error; err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems, itemCode)
			continue
		}

		// Validate owner code exists
		var ownerModel models.Owner
		if err := tx.Where("code = ?", ownerCode).First(&ownerModel).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Owner code '%s' not found", rowNum, ownerCode))
			continue
		}

		// Validate UOM exists
		var uomModel models.Uom
		if err := tx.Where("code = ?", uom).First(&uomModel).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: UOM '%s' not found", rowNum, uom))
			continue
		}

		// Create product
		product := models.Product{
			ItemCode:   itemCode,
			ItemName:   itemName,
			CBM:        cbm,
			Barcode:    gmc,
			GMC:        gmc,
			Width:      width,
			Length:     length,
			Height:     height,
			Weight:     weight,
			Color:      color,
			Group:      group,
			Category:   category,
			HasSerial:  serial,
			HasWaranty: warranty,
			HasAdaptor: adaptor,
			ManualBook: manualBook,
			Uom:        uom,
			OwnerCode:  ownerCode,
			CreatedBy:  userID,
		}

		if err := tx.Create(&product).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create product - %s", rowNum, err.Error()))
			continue
		}

		// Create UOM conversion
		uomConversion := models.UomConversion{
			ItemID:         product.ID,
			ItemCode:       product.ItemCode,
			Ean:            gmc,
			FromUom:        uom,
			ToUom:          uom,
			IsBase:         true,
			ConversionRate: 1,
			CreatedBy:      userID,
		}

		if err := tx.Create(&uomConversion).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create UOM conversion - %s", rowNum, err.Error()))
			tx.Rollback()
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

// Helper function to parse float from string
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

// Helper function to calculate CBM (Cubic Meter)
func calculateCBM(width, length, height float64) float64 {
	if width <= 0 || length <= 0 || height <= 0 {
		return 0
	}
	// Convert cm to meter and calculate volume
	cbm := (width / 100) * (length / 100) * (height / 100)
	// Round to 6 decimal places
	return math.Round(cbm*1000000) / 1000000
}
