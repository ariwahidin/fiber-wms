// package controllers

// import (
// 	"errors"
// 	"fiber-app/models"
// 	"fmt"
// 	"math"
// 	"strings"
// 	"time"

// 	"github.com/go-playground/validator"
// 	"github.com/gofiber/fiber/v2"
// 	"github.com/xuri/excelize/v2"
// 	"gorm.io/gorm"
// )

// type ProductController struct {
// 	DB *gorm.DB
// }

// func NewProductController(DB *gorm.DB) *ProductController {
// 	return &ProductController{DB: DB}
// }

// var productInput struct {
// 	ID         uint    `json:"id"`
// 	ItemCode   string  `json:"item_code" validate:"required,min=3"`
// 	ItemName   string  `json:"item_name" validate:"required,min=3"`
// 	CBM        float64 `json:"cbm" validate:"required"`
// 	GMC        string  `json:"gmc" validate:"required,min=6"`
// 	Width      float64 `json:"width"`
// 	Length     float64 `json:"length"`
// 	Height     float64 `json:"height"`
// 	Weight     float64 `json:"weight"`
// 	Color      string  `json:"color" gorm:"default:null"`
// 	Group      string  `json:"group" gorm:"default:null"`
// 	Category   string  `json:"category" gorm:"default:null"`
// 	Serial     string  `json:"serial" validate:"required,min=1"`
// 	Waranty    string  `json:"waranty" validate:"required,min=1"`
// 	Adaptor    string  `json:"adaptor" validate:"required,min=1"`
// 	ManualBook string  `json:"manual_book" validate:"required,min=1"`
// 	Uom        string  `json:"uom" validate:"required,min=3"`
// 	OwnerCode  string  `json:"owner_code" validate:"required,min=3"`
// 	UserDef1   string  `json:"user_def1" gorm:"default:null"`
// }

// func (c *ProductController) CreateProduct(ctx *fiber.Ctx) error {

// 	// Parse Body
// 	if err := ctx.BodyParser(&productInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Validasi input menggunakan validator
// 	validate := validator.New()
// 	if err := validate.Struct(productInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	Uom := models.Uom{}
// 	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
// 	if Uom.ID == 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
// 	}

// 	// Membuat user dengan memasukkan data ke struct models.Product
// 	product := models.Product{
// 		ItemCode:   productInput.ItemCode,
// 		ItemName:   productInput.ItemName,
// 		CBM:        productInput.CBM,
// 		Barcode:    productInput.GMC,
// 		GMC:        productInput.GMC,
// 		Width:      productInput.Width,
// 		Length:     productInput.Length,
// 		Height:     productInput.Height,
// 		Weight:     productInput.Weight,
// 		Color:      productInput.Color,
// 		Group:      productInput.Group,
// 		Category:   productInput.Category,
// 		HasSerial:  productInput.Serial,
// 		HasWaranty: productInput.Waranty,
// 		HasAdaptor: productInput.Adaptor,
// 		ManualBook: productInput.ManualBook,
// 		Uom:        productInput.Uom,
// 		OwnerCode:  productInput.OwnerCode,
// 		CreatedBy:  int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&product).Error; err != nil {
// 		c.DB.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	uomConversion := models.UomConversion{
// 		ItemID:         product.ID,
// 		ItemCode:       product.ItemCode,
// 		Ean:            productInput.GMC,
// 		FromUom:        product.Uom,
// 		ToUom:          product.Uom,
// 		IsBase:         true,
// 		ConversionRate: 1,
// 		CreatedBy:      int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&uomConversion).Error; err != nil {
// 		// Jika terjadi error saat membuat UomConversion, rollback perubahan pada Product
// 		c.DB.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Respons sukses
// 	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Product created successfully", "data": product})

// }

// func (c *ProductController) GetProductByID(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	// Periksa apakah user dengan ID tersebut ada
// 	var result models.Product
// 	if err := c.DB.First(&result, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product found", "data": result})
// }

// func (c *ProductController) UpdateProduct(ctx *fiber.Ctx) error {

// 	fmt.Println("Payload Edit Data : ", string(ctx.Body()))
// 	// return nil

// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	// Check if the product exists
// 	var product models.Product
// 	if err := c.DB.First(&product, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Parse Body
// 	if err := ctx.BodyParser(&productInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Validasi input menggunakan validator
// 	validate := validator.New()
// 	if err := validate.Struct(productInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	fmt.Println("Existing Product Item Code : ", product.ItemCode)
// 	fmt.Println("Incoming Product Item Code : ", productInput.ItemCode)

// 	if productInput.ItemCode != product.ItemCode || productInput.Uom != product.Uom || productInput.GMC != product.Barcode {

// 		// Check item id any transaction
// 		var inboundHeader models.InboundDetail
// 		if err := c.DB.Where("item_id = ?", id).First(&inboundHeader).Error; err != nil {
// 			if errors.Is(err, gorm.ErrRecordNotFound) {
// 				// return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
// 			}
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}

// 		if inboundHeader.ID > 0 {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already used in transaction"})
// 		}
// 	}

// 	Uom := models.Uom{}
// 	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
// 	if Uom.ID == 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
// 	}

// 	if err := c.DB.Debug().
// 		Model(&models.Product{}).
// 		Where("id = ?", id).
// 		Updates(map[string]interface{}{
// 			"item_code":   productInput.ItemCode,
// 			"item_name":   productInput.ItemName,
// 			"cbm":         productInput.CBM,
// 			"gmc":         productInput.GMC,
// 			"barcode":     productInput.GMC,
// 			"group":       productInput.Group,
// 			"category":    productInput.Category,
// 			"width":       productInput.Width,
// 			"length":      productInput.Length,
// 			"height":      productInput.Height,
// 			"weight":      productInput.Weight,
// 			"color":       productInput.Color,
// 			"has_serial":  productInput.Serial,
// 			"has_waranty": productInput.Waranty,
// 			"has_adaptor": productInput.Adaptor,
// 			"manual_book": productInput.ManualBook,
// 			"uom":         productInput.Uom,
// 			"owner_code":  productInput.OwnerCode,
// 			"user_def1":   productInput.UserDef1,
// 			"updated_at":  time.Now(),
// 			"updated_by":  int(ctx.Locals("userID").(float64)),
// 		}).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	uomConversion := models.UomConversion{}
// 	c.DB.Where("item_code = ? AND from_uom = ?", product.ItemCode, productInput.Uom).First(&uomConversion)
// 	if uomConversion.ID != 0 {
// 		// Update existing UomConversion
// 		if err := c.DB.Debug().
// 			Model(&models.UomConversion{}).
// 			Where("item_code = ? AND from_uom = ?", product.ItemCode, productInput.Uom).
// 			Updates(map[string]interface{}{
// 				"ean":             productInput.GMC,
// 				"from_uom":        productInput.Uom,
// 				"to_uom":          productInput.Uom,
// 				"updated_at":      time.Now(),
// 				"updated_by":      int(ctx.Locals("userID").(float64)),
// 				"is_base":         false,
// 				"conversion_rate": 1,
// 			}).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}

// 	} else {
// 		// Create new UomConversion
// 		newUomConversion := models.UomConversion{
// 			ItemCode:       product.ItemCode,
// 			Ean:            product.Barcode,
// 			FromUom:        productInput.Uom,
// 			ToUom:          productInput.Uom,
// 			ConversionRate: 1,
// 			IsBase:         false,
// 			CreatedAt:      time.Now(),
// 			CreatedBy:      int(ctx.Locals("userID").(float64)),
// 			UpdatedAt:      time.Now(),
// 			UpdatedBy:      int(ctx.Locals("userID").(float64)),
// 		}
// 		if err := c.DB.Create(&newUomConversion).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	// Respons sukses
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product updated successfully", "data": product})

// }

// func (c *ProductController) GetAllProducts(ctx *fiber.Ctx) error {

// 	if ctx.Query("owner") != "" {
// 		var products []models.Product
// 		if err := c.DB.Where("owner_code = ?", ctx.Query("owner")).Order("item_code ASC").Find(&products).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
// 	}

// 	var products []models.Product
// 	if err := c.DB.Order("item_code ASC").Find(&products).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
// }

// func (c *ProductController) GetAllCategory(ctx *fiber.Ctx) error {

// 	var categories []models.Category
// 	if err := c.DB.Find(&categories).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Categories found", "data": categories})
// }

// func (c *ProductController) DeleteProduct(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	// Periksa apakah user dengan ID tersebut ada
// 	var product models.Product
// 	if err := c.DB.First(&product, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Check item id any transaction
// 	var inboundHeader models.InboundDetail
// 	if err := c.DB.Where("item_id = ?", id).First(&inboundHeader).Error; err != nil {
// 		if !errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	if inboundHeader.ID > 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already used in transaction"})
// 	}

// 	// Hanya menyimpan field yang dipilih dengan menggunakan Select
// 	result := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&product)
// 	if result.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
// 	}

// 	// Hapus user
// 	result = c.DB.Delete(&product)
// 	if result.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
// 	}

// 	// Respons sukses
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product deleted successfully", "data": product})
// }

// // ===============================================================================
// // Begin upload Via Excel File
// // ===============================================================================

// type ExcelUploadResult struct {
// 	TotalRows     int      `json:"total_rows"`
// 	SuccessCount  int      `json:"success_count"`
// 	SkippedCount  int      `json:"skipped_count"`
// 	ErrorCount    int      `json:"error_count"`
// 	SkippedItems  []string `json:"skipped_items"`
// 	ErrorMessages []string `json:"error_messages"`
// }

// func (c *ProductController) CreateProductFromExcelFile(ctx *fiber.Ctx) error {
// 	// Get file from request
// 	file, err := ctx.FormFile("file")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "File is required",
// 		})
// 	}

// 	// Validate file extension
// 	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
// 		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Only Excel files (.xlsx, .xls) are allowed",
// 		})
// 	}

// 	// Open uploaded file
// 	fileContent, err := file.Open()
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to open file",
// 		})
// 	}
// 	defer fileContent.Close()

// 	// Read Excel file
// 	f, err := excelize.OpenReader(fileContent)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to read Excel file",
// 		})
// 	}
// 	defer f.Close()

// 	// Get first sheet
// 	sheets := f.GetSheetList()
// 	if len(sheets) == 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "No sheets found in Excel file",
// 		})
// 	}

// 	rows, err := f.GetRows(sheets[0])
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to read rows",
// 		})
// 	}

// 	if len(rows) < 2 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Excel file must contain header and at least one data row",
// 		})
// 	}

// 	result := ExcelUploadResult{
// 		TotalRows:     len(rows) - 1,
// 		SuccessCount:  0,
// 		SkippedCount:  0,
// 		ErrorCount:    0,
// 		SkippedItems:  []string{},
// 		ErrorMessages: []string{},
// 	}

// 	userID := int(ctx.Locals("userID").(float64))

// 	// Start transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	// Process each row (skip header)
// 	for i, row := range rows[1:] {
// 		rowNum := i + 2 // Excel row number (header is row 1)

// 		// Skip empty rows
// 		if len(row) == 0 || (len(row) > 0 && strings.TrimSpace(row[0]) == "") {
// 			continue
// 		}

// 		// Ensure minimum columns
// 		if len(row) < 16 {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: Insufficient columns", rowNum))
// 			continue
// 		}

// 		// Sanitize and normalize input
// 		itemCode := strings.ToUpper(strings.TrimSpace(row[0]))
// 		itemName := strings.TrimSpace(row[1])
// 		width := parseFloat(row[2])
// 		length := parseFloat(row[3])
// 		height := parseFloat(row[4])
// 		cbm := calculateCBM(width, length, height) // Auto calculate CBM
// 		gmc := strings.ToUpper(strings.TrimSpace(row[5]))
// 		weight := parseFloat(row[6])
// 		color := strings.TrimSpace(row[7])
// 		group := strings.TrimSpace(row[8])
// 		category := strings.TrimSpace(row[9])
// 		serial := strings.ToUpper(strings.TrimSpace(row[10]))
// 		warranty := strings.ToUpper(strings.TrimSpace(row[11]))
// 		adaptor := strings.ToUpper(strings.TrimSpace(row[12]))
// 		manualBook := strings.ToUpper(strings.TrimSpace(row[13]))
// 		uom := strings.ToUpper(strings.TrimSpace(row[14]))
// 		ownerCode := strings.ToUpper(strings.TrimSpace(row[15]))

// 		// Validate required fields
// 		if itemCode == "" || itemName == "" || gmc == "" ||
// 			serial == "" || warranty == "" || adaptor == "" ||
// 			manualBook == "" || uom == "" || ownerCode == "" {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: Missing required fields", rowNum))
// 			continue
// 		}

// 		// Check if item code already exists
// 		var existingProduct models.Product
// 		if err := tx.Where("item_code = ?", itemCode).First(&existingProduct).Error; err == nil {
// 			result.SkippedCount++
// 			result.SkippedItems = append(result.SkippedItems, itemCode)
// 			continue
// 		}

// 		// Validate owner code exists
// 		var ownerModel models.Owner
// 		if err := tx.Where("code = ?", ownerCode).First(&ownerModel).Error; err != nil {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: Owner code '%s' not found", rowNum, ownerCode))
// 			continue
// 		}

// 		// Validate UOM exists
// 		var uomModel models.Uom
// 		if err := tx.Where("code = ?", uom).First(&uomModel).Error; err != nil {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: UOM '%s' not found", rowNum, uom))
// 			continue
// 		}

// 		// Create product
// 		product := models.Product{
// 			ItemCode:   itemCode,
// 			ItemName:   itemName,
// 			CBM:        cbm,
// 			Barcode:    gmc,
// 			GMC:        gmc,
// 			Width:      width,
// 			Length:     length,
// 			Height:     height,
// 			Weight:     weight,
// 			Color:      color,
// 			Group:      group,
// 			Category:   category,
// 			HasSerial:  serial,
// 			HasWaranty: warranty,
// 			HasAdaptor: adaptor,
// 			ManualBook: manualBook,
// 			Uom:        uom,
// 			OwnerCode:  ownerCode,
// 			CreatedBy:  userID,
// 		}

// 		if err := tx.Create(&product).Error; err != nil {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: Failed to create product - %s", rowNum, err.Error()))
// 			continue
// 		}

// 		// Create UOM conversion
// 		uomConversion := models.UomConversion{
// 			ItemID:         product.ID,
// 			ItemCode:       product.ItemCode,
// 			Ean:            gmc,
// 			FromUom:        uom,
// 			ToUom:          uom,
// 			IsBase:         true,
// 			ConversionRate: 1,
// 			CreatedBy:      userID,
// 		}

// 		if err := tx.Create(&uomConversion).Error; err != nil {
// 			result.ErrorCount++
// 			result.ErrorMessages = append(result.ErrorMessages,
// 				fmt.Sprintf("Row %d: Failed to create UOM conversion - %s", rowNum, err.Error()))
// 			tx.Rollback()
// 			continue
// 		}

// 		result.SuccessCount++
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to commit transaction",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": fmt.Sprintf("Upload completed: %d success, %d skipped, %d errors",
// 			result.SuccessCount, result.SkippedCount, result.ErrorCount),
// 		"data": result,
// 	})
// }

// // Helper function to parse float from string
// func parseFloat(s string) float64 {
// 	s = strings.TrimSpace(s)
// 	if s == "" {
// 		return 0
// 	}
// 	var val float64
// 	fmt.Sscanf(s, "%f", &val)
// 	return val
// }

// // Helper function to calculate CBM (Cubic Meter)
// func calculateCBM(width, length, height float64) float64 {
// 	if width <= 0 || length <= 0 || height <= 0 {
// 		return 0
// 	}
// 	// Convert cm to meter and calculate volume
// 	cbm := (width / 100) * (length / 100) * (height / 100)
// 	// Round to 6 decimal places
// 	return math.Round(cbm*1000000) / 1000000
// }

// // =================================================================================
// // End upload product from excel
// // =================================================================================

// // ==========================================================================
// // Begin Export Product To Excel
// // ==========================================================================

// func (c *ProductController) ExportProduct(ctx *fiber.Ctx) error {
// 	// Parse request body
// 	type ExportRequest struct {
// 		OwnerCodes []string `json:"owner_codes"`
// 	}

// 	var req ExportRequest
// 	if err := ctx.BodyParser(&req); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Invalid request body",
// 		})
// 	}

// 	// Build query
// 	var products []models.Product
// 	query := c.DB.Model(&models.Product{})

// 	// Filter by owner codes if provided
// 	if len(req.OwnerCodes) > 0 {
// 		query = query.Where("owner_code IN ?", req.OwnerCodes)
// 	}

// 	if err := query.Order("created_at DESC").Find(&products).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to fetch products",
// 			"error":   err.Error(),
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Products retrieved successfully",
// 		"data":    products,
// 	})
// }

// // Add this to get unique owner codes for the dropdown
// func (c *ProductController) GetOwnerCodes(ctx *fiber.Ctx) error {
// 	var ownerCodes []string

// 	if err := c.DB.Model(&models.Product{}).
// 		Distinct("owner_code").
// 		Where("owner_code IS NOT NULL AND owner_code != ''").
// 		Order("owner_code ASC").
// 		Pluck("owner_code", &ownerCodes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to fetch owner codes",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Owner codes retrieved successfully",
// 		"data":    ownerCodes,
// 	})
// }

// // ==========================================================================
// // End Export Product To Excel
// // ==========================================================================

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

// Use a struct (not package-level var) to avoid shared state between requests
type productInputStruct struct {
	ID         uint    `json:"id"`
	ItemCode   string  `json:"item_code" validate:"required,min=3"`
	ItemName   string  `json:"item_name" validate:"required,min=3"`
	UnitModel  string  `json:"unit_model"`
	CBM        float64 `json:"cbm"`
	GMC        string  `json:"gmc" validate:"required,min=6"`
	Width      float64 `json:"width"`
	Length     float64 `json:"length"`
	Height     float64 `json:"height"`
	Weight     float64 `json:"weight"`
	Color      string  `json:"color"`
	Group      string  `json:"group"`
	Category   string  `json:"category"`
	Serial     string  `json:"serial" validate:"required,min=1"`
	Waranty    string  `json:"waranty" validate:"required,min=1"`
	Adaptor    string  `json:"adaptor" validate:"required,min=1"`
	ManualBook string  `json:"manual_book" validate:"required,min=1"`
	Uom        string  `json:"uom" validate:"required,min=1"`
	OwnerCode  string  `json:"owner_code" validate:"required,min=3"`
	UserDef1   string  `json:"user_def1"`
}

func (c *ProductController) CreateProduct(ctx *fiber.Ctx) error {
	var input productInputStruct

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check UOM exists
	var uomModel models.Uom
	if err := c.DB.Where("code = ?", input.Uom).First(&uomModel).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	// Check duplicate item code
	var existing models.Product
	if err := c.DB.Where("item_code = ?", input.ItemCode).First(&existing).Error; err == nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Item code already exists"})
	}

	userID := int(ctx.Locals("userID").(float64))

	product := models.Product{
		ItemCode:   input.ItemCode,
		ItemName:   input.ItemName,
		UnitModel:  input.UnitModel,
		CBM:        input.CBM,
		Barcode:    input.GMC,
		GMC:        input.GMC,
		Width:      input.Width,
		Length:     input.Length,
		Height:     input.Height,
		Weight:     input.Weight,
		Color:      input.Color,
		Group:      input.Group,
		Category:   input.Category,
		HasSerial:  input.Serial,
		HasWaranty: input.Waranty,
		HasAdaptor: input.Adaptor,
		ManualBook: input.ManualBook,
		Uom:        input.Uom,
		OwnerCode:  input.OwnerCode,
		UserDef1:   input.UserDef1,
		CreatedBy:  userID,
	}

	if err := c.DB.Create(&product).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	uomConversion := models.UomConversion{
		ItemID:         product.ID,
		ItemCode:       product.ItemCode,
		Ean:            input.GMC,
		FromUom:        product.Uom,
		ToUom:          product.Uom,
		IsBase:         true,
		ConversionRate: 1,
		CreatedBy:      userID,
	}

	if err := c.DB.Create(&uomConversion).Error; err != nil {
		c.DB.Delete(&product)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Product created successfully", "data": product})
}

func (c *ProductController) GetProductByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

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
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var input productInputStruct
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check if key fields changed — block if transactions exist
	if input.ItemCode != product.ItemCode || input.Uom != product.Uom || input.GMC != product.Barcode {
		var inboundDetail models.InboundDetail
		err := c.DB.Where("item_id = ?", id).First(&inboundDetail).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if inboundDetail.ID > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already used in transaction"})
		}
	}

	// Check UOM exists
	var uomModel models.Uom
	if err := c.DB.Where("code = ?", input.Uom).First(&uomModel).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	userID := int(ctx.Locals("userID").(float64))

	if err := c.DB.Model(&models.Product{}).Where("id = ?", id).Updates(map[string]interface{}{
		"item_code":   input.ItemCode,
		"item_name":   input.ItemName,
		"unit_model":  input.UnitModel,
		"cbm":         input.CBM,
		"gmc":         input.GMC,
		"barcode":     input.GMC,
		"group":       input.Group,
		"category":    input.Category,
		"width":       input.Width,
		"length":      input.Length,
		"height":      input.Height,
		"weight":      input.Weight,
		"color":       input.Color,
		"has_serial":  input.Serial,
		"has_waranty": input.Waranty,
		"has_adaptor": input.Adaptor,
		"manual_book": input.ManualBook,
		"uom":         input.Uom,
		"owner_code":  input.OwnerCode,
		"user_def1":   input.UserDef1,
		"updated_at":  time.Now(),
		"updated_by":  userID,
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Upsert UomConversion for base UOM
	var uomConversion models.UomConversion
	c.DB.Where("item_code = ? AND is_base = ?", product.ItemCode, true).First(&uomConversion)
	if uomConversion.ID != 0 {
		c.DB.Model(&models.UomConversion{}).Where("id = ?", uomConversion.ID).Updates(map[string]interface{}{
			"ean":             input.GMC,
			"from_uom":        input.Uom,
			"to_uom":          input.Uom,
			"updated_at":      time.Now(),
			"updated_by":      userID,
			"conversion_rate": 1,
		})
	} else {
		newConv := models.UomConversion{
			ItemID:         product.ID,
			ItemCode:       product.ItemCode,
			Ean:            input.GMC,
			FromUom:        input.Uom,
			ToUom:          input.Uom,
			ConversionRate: 1,
			IsBase:         true,
			CreatedBy:      userID,
			UpdatedBy:      userID,
		}
		c.DB.Create(&newConv)
	}

	// Return updated product
	c.DB.First(&product, id)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product updated successfully", "data": product})
}

func (c *ProductController) GetAllProducts(ctx *fiber.Ctx) error {
	var products []models.Product
	query := c.DB.Order("item_code ASC")
	if owner := ctx.Query("owner"); owner != "" {
		query = query.Where("owner_code = ?", owner)
	}
	if err := query.Find(&products).Error; err != nil {
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

	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Block delete if used in transactions
	var inboundDetail models.InboundDetail
	if err := c.DB.Where("item_id = ?", id).First(&inboundDetail).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	if inboundDetail.ID > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already used in transaction"})
	}

	if err := c.DB.Delete(&product).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product deleted successfully"})
}

// ===============================================================================
// Excel Upload
// ===============================================================================

type ExcelUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (c *ProductController) CreateProductFromExcelFile(ctx *fiber.Ctx) error {
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "File is required"})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Only Excel files (.xlsx, .xls) are allowed"})
	}

	fileContent, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to open file"})
	}
	defer fileContent.Close()

	f, err := excelize.OpenReader(fileContent)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Failed to read Excel file"})
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "No sheets found in Excel file"})
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to read rows"})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Excel file must contain header and at least one data row"})
	}

	result := ExcelUploadResult{
		TotalRows:     len(rows) - 1,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(ctx.Locals("userID").(float64))

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Column order: item_code, item_name, unit_model, width, length, height, ean, weight, color,
	//               group, category, serial, warranty, adaptor, manual_book, uom, owner_code
	for i, row := range rows[1:] {
		rowNum := i + 2

		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}

		if len(row) < 17 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: Insufficient columns (need 17)", rowNum))
			continue
		}

		itemCode := strings.ToUpper(strings.TrimSpace(row[0]))
		itemName := strings.TrimSpace(row[1])
		unitModel := strings.TrimSpace(row[2])
		width := parseFloat(row[3])
		length := parseFloat(row[4])
		height := parseFloat(row[5])
		cbm := calculateCBM(width, length, height)
		gmc := strings.ToUpper(strings.TrimSpace(row[6]))
		weight := parseFloat(row[7])
		color := strings.TrimSpace(row[8])
		group := strings.TrimSpace(row[9])
		category := strings.TrimSpace(row[10])
		serial := strings.ToUpper(strings.TrimSpace(row[11]))
		warranty := strings.ToUpper(strings.TrimSpace(row[12]))
		adaptor := strings.ToUpper(strings.TrimSpace(row[13]))
		manualBook := strings.ToUpper(strings.TrimSpace(row[14]))
		uom := strings.ToUpper(strings.TrimSpace(row[15]))
		ownerCode := strings.ToUpper(strings.TrimSpace(row[16]))

		if itemCode == "" || itemName == "" || gmc == "" || serial == "" ||
			warranty == "" || adaptor == "" || manualBook == "" || uom == "" || ownerCode == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: Missing required fields", rowNum))
			continue
		}

		// Skip duplicates
		var existingProduct models.Product
		if err := tx.Where("item_code = ?", itemCode).First(&existingProduct).Error; err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems, itemCode)
			continue
		}

		// Validate owner
		var ownerModel models.Owner
		if err := tx.Where("code = ?", ownerCode).First(&ownerModel).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: Owner code '%s' not found", rowNum, ownerCode))
			continue
		}

		// Validate UOM
		var uomModel models.Uom
		if err := tx.Where("code = ?", uom).First(&uomModel).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: UOM '%s' not found", rowNum, uom))
			continue
		}

		product := models.Product{
			ItemCode:   itemCode,
			ItemName:   itemName,
			UnitModel:  unitModel,
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
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: Failed to create product - %s", rowNum, err.Error()))
			continue
		}

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
			result.ErrorMessages = append(result.ErrorMessages, fmt.Sprintf("Row %d: Failed to create UOM conversion - %s", rowNum, err.Error()))
			continue
		}

		result.SuccessCount++
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to commit transaction"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Upload completed: %d success, %d skipped, %d errors", result.SuccessCount, result.SkippedCount, result.ErrorCount),
		"data":    result,
	})
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

func calculateCBM(width, length, height float64) float64 {
	if width <= 0 || length <= 0 || height <= 0 {
		return 0
	}
	cbm := (width / 100) * (length / 100) * (height / 100)
	return math.Round(cbm*1000000) / 1000000
}

// ===============================================================================
// Export
// ===============================================================================

func (c *ProductController) ExportProduct(ctx *fiber.Ctx) error {
	type ExportRequest struct {
		OwnerCodes []string `json:"owner_codes"`
	}

	var req ExportRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	var products []models.Product
	query := c.DB.Model(&models.Product{})
	if len(req.OwnerCodes) > 0 {
		query = query.Where("owner_code IN ?", req.OwnerCodes)
	}

	if err := query.Order("created_at DESC").Find(&products).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch products", "error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products retrieved successfully", "data": products})
}

func (c *ProductController) GetOwnerCodes(ctx *fiber.Ctx) error {
	var ownerCodes []string
	if err := c.DB.Model(&models.Product{}).
		Distinct("owner_code").
		Where("owner_code IS NOT NULL AND owner_code != ''").
		Order("owner_code ASC").
		Pluck("owner_code", &ownerCodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch owner codes"})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner codes retrieved successfully", "data": ownerCodes})
}
