package item_controller

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
	ID           uint    `json:"id"`
	ItemCode     string  `json:"item_code" validate:"required,min=3"`
	ItemName     string  `json:"item_name" validate:"required,min=3"`
	UnitModel    string  `json:"unit_model"`
	CBM          float64 `json:"cbm"`
	GMC          string  `json:"gmc" validate:"required,min=6"`
	Width        float64 `json:"width"`
	Length       float64 `json:"length"`
	Height       float64 `json:"height"`
	Weight       float64 `json:"weight"`
	Color        string  `json:"color"`
	Group        string  `json:"group"`
	QtyPerCarton int     `json:"qty_per_carton"`
	Category     string  `json:"category"`
	Serial       string  `json:"serial" validate:"required,min=1"`
	Waranty      string  `json:"waranty" validate:"required,min=1"`
	Adaptor      string  `json:"adaptor" validate:"required,min=1"`
	ManualBook   string  `json:"manual_book" validate:"required,min=1"`
	Uom          string  `json:"uom" validate:"required,min=1"`
	OwnerCode    string  `json:"owner_code" validate:"required,min=3"`
	UserDef1     string  `json:"user_def1"`
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
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item code already exists"})
	}

	userID := int(ctx.Locals("userID").(float64))

	product := models.Product{
		ItemCode:     input.ItemCode,
		ItemName:     input.ItemName,
		UnitModel:    input.UnitModel,
		CBM:          input.CBM,
		Barcode:      input.GMC,
		GMC:          input.GMC,
		Width:        input.Width,
		Length:       input.Length,
		Height:       input.Height,
		Weight:       input.Weight,
		Color:        input.Color,
		Group:        input.Group,
		QtyPerCarton: input.QtyPerCarton,
		Category:     input.Category,
		HasSerial:    input.Serial,
		HasWaranty:   input.Waranty,
		HasAdaptor:   input.Adaptor,
		ManualBook:   input.ManualBook,
		Uom:          input.Uom,
		OwnerCode:    input.OwnerCode,
		UserDef1:     input.UserDef1,
		CreatedBy:    userID,
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
	if input.ItemCode != product.ItemCode || input.Uom != product.Uom || input.GMC != product.GMC {
		var inboundDetails []models.InboundDetail
		if err := c.DB.Where("item_id = ?", id).Find(&inboundDetails).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if len(inboundDetails) > 0 {
			// Kumpulkan semua inbound_no yang terlibat
			var inboundNos []string
			for _, d := range inboundDetails {
				inboundNos = append(inboundNos, d.InboundNo)
			}

			// Cek apakah ada header yang sudah complete — kalau ada, block
			var completeCount int64
			if err := c.DB.Model(&models.InboundHeader{}).
				Where("inbound_no IN ? AND status = ?", inboundNos, "complete").
				Count(&completeCount).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Ambil inbound_no yang belum complete
			var incompleteHeaders []models.InboundHeader
			if err := c.DB.Select("inbound_no").
				Where("inbound_no IN ? AND status != ?", inboundNos, "complete").
				Find(&incompleteHeaders).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if len(incompleteHeaders) > 0 {
				var incompleteNos []string
				for _, h := range incompleteHeaders {
					incompleteNos = append(incompleteNos, h.InboundNo)
				}

				// Update barcode hanya di detail yang belum complete
				if err := c.DB.Model(&models.InboundDetail{}).
					Where("item_id = ? AND inbound_no IN ?", id, incompleteNos).
					Update("barcode", input.GMC).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}
	}

	// Check UOM exists
	var uomModel models.Uom
	if err := c.DB.Where("code = ?", input.Uom).First(&uomModel).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	userID := int(ctx.Locals("userID").(float64))

	if err := c.DB.Model(&models.Product{}).Where("id = ?", id).Updates(map[string]interface{}{
		"item_code":      input.ItemCode,
		"item_name":      input.ItemName,
		"unit_model":     input.UnitModel,
		"cbm":            input.CBM,
		"gmc":            input.GMC,
		"barcode":        input.GMC,
		"group":          input.Group,
		"category":       input.Category,
		"width":          input.Width,
		"length":         input.Length,
		"height":         input.Height,
		"weight":         input.Weight,
		"color":          input.Color,
		"qty_per_carton": input.QtyPerCarton,
		"has_serial":     input.Serial,
		"has_waranty":    input.Waranty,
		"has_adaptor":    input.Adaptor,
		"manual_book":    input.ManualBook,
		"uom":            input.Uom,
		"owner_code":     input.OwnerCode,
		"user_def1":      input.UserDef1,
		"updated_at":     time.Now(),
		"updated_by":     userID,
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

	// Update semua inventory dengan ean baru jika ean diubah tapi yang stock available > 0
	if input.GMC != product.GMC {
		c.DB.Model(&models.Inventory{}).Where("item_code = ? AND qty_available > 0", product.ItemCode).Updates(map[string]interface{}{
			"barcode": input.GMC,
		})
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
