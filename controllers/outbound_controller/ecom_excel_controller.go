package outbound_controller

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ======================================================================
// BEGIN PROCESS UPLOAD OUTBOUND FROM ECOMMERCE EXCEL
// ======================================================================

// --- Structs ---

type EcommerceUploadResponse struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	TotalRows        int               `json:"total_rows"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	OutboundNumbers  []string          `json:"outbound_numbers,omitempty"`
	SkippedOrders    []SkippedOrder    `json:"skipped_orders,omitempty"`
	Errors           []ExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

// EcommercePlatformConfig holds hardcoded defaults per platform
type EcommercePlatformConfig struct {
	WhsCode         string
	OwnerCode       string
	TransporterCode string
	CustomerCode    string // e.g. "SHOPEE", "TOKOPEDIA", etc.
	DelivTo         string
}

// EcommerceOrderRow represents one parsed row from Excel
type EcommerceOrderRow struct {
	No          int
	Date        string
	Platform    string
	OrderNumber string
	AWBNumber   string
	ProductName string
	Qty         float64
	SKU         string
	Row         int
}

// --- Platform Config Map ---
// Sesuaikan value ini dengan data di database Anda

var ecommercePlatformConfigs = map[string]EcommercePlatformConfig{
	"SHOPEE": {
		WhsCode:         "WH-B",
		OwnerCode:       "YUWELL",
		TransporterCode: "JNE",
		CustomerCode:    "SHOPEE",
		DelivTo:         "SHOPEE",
	},
	"TOKOPEDIA": {
		WhsCode:         "WH-B",
		OwnerCode:       "YUWELL",
		TransporterCode: "JNE",
		CustomerCode:    "TOKOPEDIA",
		DelivTo:         "TOKOPEDIA",
	},
	"LAZADA": {
		WhsCode:         "WH01",
		OwnerCode:       "OWN001",
		TransporterCode: "LEX",
		CustomerCode:    "LAZADA",
		DelivTo:         "LAZADA",
	},
	"TIKTOK": {
		WhsCode:         "WH01",
		OwnerCode:       "OWN001",
		TransporterCode: "JNE",
		CustomerCode:    "TIKTOK",
		DelivTo:         "TIKTOK",
	},
}

// ======================================================================
// ECOMMERCE EXCEL UPLOAD - REFACTORED
// Supports native templates from: SHOPEE, TOKOPEDIA
// POST /outbound/upload-ecommerce-excel
// Form fields:
//   - file     : Excel file (native platform export)
//   - platform : SHOPEE | TOKOPEDIA
//   - customer : customer_code
// ======================================================================

// -----------------------------------------------------------------------
// Platform Parser Config
// Defines how to extract data from each platform's native Excel format
// -----------------------------------------------------------------------

type PlatformExcelConfig struct {
	SheetName   string // target sheet name (case-insensitive match)
	HeaderRows  int    // number of rows to skip (1 = header only, 2 = header + description)
	ColOrderNo  int
	ColAWB      int
	ColDate     int
	ColSKU      int
	ColProduct  int
	ColQty      int
	DateFormats []string
}

var platformExcelConfigs = map[string]PlatformExcelConfig{
	"SHOPEE": {
		SheetName:  "orders",
		HeaderRows: 1,
		ColOrderNo: 0,
		ColAWB:     4,
		ColDate:    9,
		ColProduct: 13,
		ColSKU:     14,
		ColQty:     18,
		DateFormats: []string{
			"2006-01-02 15:04",
			"2006-01-02 15:04:05",
			"2006-01-02",
		},
	},
	"TOKOPEDIA": {
		SheetName:  "OrderSKUList",
		HeaderRows: 2, // row 1 = headers, row 2 = column descriptions
		ColOrderNo: 0,
		ColSKU:     6,
		ColProduct: 7,
		ColQty:     9,
		ColDate:    29,
		ColAWB:     38,
		DateFormats: []string{
			"02/01/2006 15:04:05",
			"2/1/2006 15:04:05",
			"02/01/2006 15:04",
			"2/1/2006 15:04",
			"2006-01-02 15:04:05",
			"2006-01-02",
		},
	},
}

// -----------------------------------------------------------------------
// CreateOutboundFromEcommerceExcel
// POST /outbound/upload-ecommerce-excel
// -----------------------------------------------------------------------
func (c *OutboundController) CreateOutboundFromEcommerceExcel(ctx *fiber.Ctx) error {

	// 1. Validate platform
	platform := strings.ToUpper(strings.TrimSpace(ctx.FormValue("platform")))
	if platform == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Platform is required (SHOPEE, TOKOPEDIA)",
		})
	}

	excelCfg, ok := platformExcelConfigs[platform]
	if !ok {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s. Supported: SHOPEE, TOKOPEDIA", platform),
		})
	}

	platformCfg, ok := ecommercePlatformConfigs[platform]
	if !ok {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: fmt.Sprintf("Platform WMS config not found: %s", platform),
		})
	}

	// 2. Validate customer
	customerCode := strings.TrimSpace(ctx.FormValue("customer"))
	if customerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Customer code is required",
		})
	}

	// 3. Parse uploaded file
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors:  []ExcelRowError{{Row: 0, Message: "File Error", Detail: err.Error()}},
		})
	}

	// 4. Validate extension
	lowerName := strings.ToLower(file.Filename)
	if !strings.HasSuffix(lowerName, ".xlsx") && !strings.HasSuffix(lowerName, ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// 5. Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "File size exceeds maximum limit of 10MB",
		})
	}

	// 6. Open & read Excel
	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors:  []ExcelRowError{{Row: 0, Message: "File Processing Error", Detail: err.Error()}},
		})
	}
	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors:  []ExcelRowError{{Row: 0, Message: "Excel Read Error", Detail: err.Error()}},
		})
	}
	defer excelFile.Close()

	// 7. Find target sheet (case-insensitive)
	sheetName, err := findSheet(excelFile, excelCfg.SheetName)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: fmt.Sprintf("Sheet '%s' not found in Excel file. Available sheets: %s",
				excelCfg.SheetName, strings.Join(excelFile.GetSheetList(), ", ")),
		})
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors:  []ExcelRowError{{Row: 0, Message: "Sheet Read Error", Detail: err.Error()}},
		})
	}

	if len(rows) <= excelCfg.HeaderRows {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Excel file contains no data rows",
		})
	}

	// 8. Parse rows using platform config
	orderRows, validationErrors := c.parseNativePlatformRows(rows, excelCfg, platform)
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			ValidationErrors: validationErrors,
			TotalRows:        len(rows) - excelCfg.HeaderRows,
		})
	}

	if len(orderRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "No valid data rows found in Excel file",
		})
	}

	// 9. Group by order number
	orderMap := c.groupEcommerceRowsByOrderNumber(orderRows)

	orderNumbers := make([]string, 0, len(orderMap))
	for orderNo := range orderMap {
		orderNumbers = append(orderNumbers, orderNo)
	}
	sort.Strings(orderNumbers)

	// 10. Get userID
	userID := int(ctx.Locals("userID").(float64))

	// 11. Start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateOutboundFromEcommerceExcel: %v", r)
		}
	}()

	// 12. Validate WMS master data
	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", platformCfg.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(EcommerceUploadResponse{
				Success: false,
				Message: "Inventory Policy not found for owner: " + platformCfg.OwnerCode,
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy: " + err.Error(),
		})
	}

	var customer models.Customer
	if err := tx.First(&customer, "customer_code = ?", customerCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(EcommerceUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Customer not found: %s", customerCode),
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to validate customer: " + err.Error(),
		})
	}

	var warehouse models.Warehouse
	if err := tx.First(&warehouse, "code = ?", platformCfg.WhsCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(EcommerceUploadResponse{
				Success: false,
				Message: "Warehouse not found: " + platformCfg.WhsCode,
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to validate warehouse: " + err.Error(),
		})
	}

	// 13. Validate SKUs per order — skip order yang ada SKU invalid
	invalidOrdersBySKU := c.validateEcommerceSKUsPerOrder(tx, orderMap)

	// 14. Check duplicate ShipmentID (order number already exists)
	duplicateOrders := c.checkDuplicateShipmentIDs(tx, orderNumbers)

	// Merge skipped orders
	skippedOrders := make(map[string]string) // orderNumber → reason
	for orderNo, reason := range invalidOrdersBySKU {
		skippedOrders[orderNo] = reason
	}
	for orderNo, reason := range duplicateOrders {
		skippedOrders[orderNo] = reason
	}

	// Filter to only valid orders
	validOrderNumbers := make([]string, 0)
	for _, orderNo := range orderNumbers {
		if _, skipped := skippedOrders[orderNo]; !skipped {
			validOrderNumbers = append(validOrderNumbers, orderNo)
		}
	}

	if len(validOrderNumbers) == 0 {
		tx.Rollback()
		// Build skipped list for response
		skippedList := buildSkippedList(skippedOrders)
		return ctx.Status(fiber.StatusOK).JSON(EcommerceUploadResponse{
			Success:       false,
			Message:       "No valid orders to process. All orders were skipped.",
			TotalRows:     len(orderRows),
			SuccessCount:  0,
			FailedCount:   len(skippedOrders),
			SkippedOrders: skippedList,
		})
	}

	// 15. Create outbounds for valid orders only
	repo := repositories.NewOutboundRepository(tx)
	var outboundNumbers []string
	totalSuccessItems := 0

	for _, orderNumber := range validOrderNumbers {
		orderItems := orderMap[orderNumber]

		outboundNo, err := repo.GenerateOutboundNumber()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
				Success: false,
				Message: "Failed to generate outbound number",
				Errors:  []ExcelRowError{{Row: 0, Message: "Generation Error", Detail: err.Error()}},
			})
		}

		awb := orderItems[0].AWBNumber
		remarksHeader := fmt.Sprintf("%s | AWB: %s | Order: %s", platform, awb, orderNumber)

		now := time.Now()
		nowDate := now.Format("2006-01-02")
		nowTime := now.Format("15:04")

		outboundHeader := models.OutboundHeader{
			OutboundNo:     outboundNo,
			OutboundDate:   orderItems[0].Date,
			CustomerCode:   customer.CustomerCode,
			ShipmentID:     orderNumber,
			OrderType:      "B2C - Marketplace",
			WhsCode:        platformCfg.WhsCode,
			OwnerCode:      platformCfg.OwnerCode,
			AwbNo:          awb,
			Remarks:        remarksHeader,
			Status:         "open",
			RawStatus:      "DRAFT",
			DraftTime:      now,
			Source:         platformCfg.CustomerCode,
			CustAddress:    customer.CustAddr1,
			CustCity:       customer.CustCity,
			DelivTo:        customer.CustomerCode,
			DelivAddress:   customer.CustAddr1,
			DelivCity:      customer.CustCity,
			PlanPickupDate: nowDate,
			PlanPickupTime: nowTime,
			RcvDoDate:      nowDate,
			RcvDoTime:      nowTime,
			StartPickTime:  nowTime,
			EndPickTime:    nowTime,
			CreatedBy:      userID,
			UpdatedBy:      userID,
		}

		if err := tx.Create(&outboundHeader).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
				Success: false,
				Message: "Failed to create outbound header for order: " + orderNumber,
				Errors:  []ExcelRowError{{Row: orderItems[0].Row, Message: "Database Insert Error", Detail: err.Error()}},
			})
		}

		for _, item := range orderItems {
			var product models.Product
			if err := tx.First(&product, "item_code = ?", item.SKU).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(EcommerceUploadResponse{
					Success: false,
					Message: "Product not found during detail creation",
					Errors:  []ExcelRowError{{Row: item.Row, Message: "Product Not Found", Detail: "SKU: " + item.SKU}},
				})
			}

			var uomConversion models.UomConversion
			if err := tx.Where("item_code = ? AND factor = 1", product.ItemCode).First(&uomConversion).Error; err != nil {
				if err2 := tx.Where("item_code = ?", product.ItemCode).First(&uomConversion).Error; err2 != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(EcommerceUploadResponse{
						Success: false,
						Message: "UOM conversion not found for SKU: " + item.SKU,
						Errors:  []ExcelRowError{{Row: item.Row, Message: "UOM Not Found", Detail: "SKU: " + item.SKU}},
					})
				}
			}

			outboundDetail := models.OutboundDetail{
				OutboundNo:   outboundNo,
				OutboundID:   outboundHeader.ID,
				ItemCode:     product.ItemCode,
				ItemID:       int(product.ID),
				Barcode:      uomConversion.Ean,
				CustomerCode: customer.CustomerCode,
				Uom:          uomConversion.FromUom,
				Quantity:     item.Qty,
				WhsCode:      platformCfg.WhsCode,
				DivisionCode: "E-COMMERCE",
				QaStatus:     "A",
				OwnerCode:    platformCfg.OwnerCode,
				Remarks:      item.ProductName,
				SNCheck:      "N",
				CreatedBy:    userID,
				UpdatedBy:    userID,
			}

			if err := tx.Create(&outboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
					Success: false,
					Message: "Failed to create outbound detail for SKU: " + item.SKU,
					Errors:  []ExcelRowError{{Row: item.Row, Message: "Database Insert Error", Detail: err.Error()}},
				})
			}

			totalSuccessItems++
		}

		if err := helpers.InsertTransactionHistory(tx, outboundNo, "open", "OUTBOUND",
			fmt.Sprintf("Created from %s upload - Order: %s", platform, orderNumber), userID); err != nil {
			log.Printf("Warning: Failed to insert transaction history for %s: %v", outboundNo, err)
		}

		outboundNumbers = append(outboundNumbers, outboundNo)
	}

	// 16. Commit
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(EcommerceUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors:  []ExcelRowError{{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()}},
		})
	}

	skippedList := buildSkippedList(skippedOrders)
	msg := fmt.Sprintf("Created %d outbound(s) with %d item(s) from %s", len(outboundNumbers), totalSuccessItems, platform)
	if len(skippedList) > 0 {
		msg += fmt.Sprintf(". %d order(s) skipped.", len(skippedList))
	}

	return ctx.Status(fiber.StatusOK).JSON(EcommerceUploadResponse{
		Success:         true,
		Message:         msg,
		TotalRows:       len(orderRows),
		SuccessCount:    totalSuccessItems,
		FailedCount:     len(skippedList),
		OutboundNumbers: outboundNumbers,
		SkippedOrders:   skippedList,
	})
}

// -----------------------------------------------------------------------
// Helper: validateEcommerceSKUsPerOrder
// Returns map[orderNumber]reason for orders that have at least 1 invalid SKU
// -----------------------------------------------------------------------
func (c *OutboundController) validateEcommerceSKUsPerOrder(
	tx *gorm.DB,
	orderMap map[string][]EcommerceOrderRow,
) map[string]string {

	// Collect all unique SKUs first (single batch-style check)
	skuValid := make(map[string]bool)
	allSKUs := make([]string, 0)
	skuSeen := make(map[string]bool)
	for _, items := range orderMap {
		for _, item := range items {
			if !skuSeen[item.SKU] {
				skuSeen[item.SKU] = true
				allSKUs = append(allSKUs, item.SKU)
			}
		}
	}

	// Batch query — find which SKUs exist
	var foundProducts []models.Product
	tx.Where("item_code IN ?", allSKUs).Find(&foundProducts)
	for _, p := range foundProducts {
		skuValid[p.ItemCode] = true
	}

	// Check per order
	invalidOrders := make(map[string]string)
	for orderNo, items := range orderMap {
		for _, item := range items {
			if !skuValid[item.SKU] {
				invalidOrders[orderNo] = fmt.Sprintf("SKU not found: %s", item.SKU)
				break // one invalid SKU is enough to skip the whole order
			}
		}
	}

	return invalidOrders
}

// -----------------------------------------------------------------------
// Helper: checkDuplicateShipmentIDs
// Returns map[orderNumber]reason for orders already in DB
// -----------------------------------------------------------------------
func (c *OutboundController) checkDuplicateShipmentIDs(
	tx *gorm.DB,
	orderNumbers []string,
) map[string]string {

	var existing []models.OutboundHeader
	tx.Where("shipment_id IN ?", orderNumbers).Find(&existing)

	duplicates := make(map[string]string)
	for _, h := range existing {
		duplicates[h.ShipmentID] = fmt.Sprintf("Order already exists (Outbound: %s)", h.OutboundNo)
	}
	return duplicates
}

// -----------------------------------------------------------------------
// Helper: buildSkippedList
// -----------------------------------------------------------------------
type SkippedOrder struct {
	OrderNumber string `json:"order_number"`
	Reason      string `json:"reason"`
}

func buildSkippedList(skippedOrders map[string]string) []SkippedOrder {
	list := make([]SkippedOrder, 0, len(skippedOrders))
	for orderNo, reason := range skippedOrders {
		list = append(list, SkippedOrder{
			OrderNumber: orderNo,
			Reason:      reason,
		})
	}
	// Sort for consistent output
	sort.Slice(list, func(i, j int) bool {
		return list[i].OrderNumber < list[j].OrderNumber
	})
	return list
}

// -----------------------------------------------------------------------
// Helper: findSheet - case-insensitive sheet name lookup
// -----------------------------------------------------------------------
func findSheet(f *excelize.File, name string) (string, error) {
	for _, s := range f.GetSheetList() {
		if strings.EqualFold(s, name) {
			return s, nil
		}
	}
	return "", fmt.Errorf("sheet '%s' not found", name)
}

// -----------------------------------------------------------------------
// Helper: parseNativePlatformRows
// Parses rows from native platform Excel using PlatformExcelConfig
// -----------------------------------------------------------------------
func (c *OutboundController) parseNativePlatformRows(
	rows [][]string,
	cfg PlatformExcelConfig,
	platform string,
) ([]EcommerceOrderRow, []ValidationError) {

	var result []EcommerceOrderRow
	var errs []ValidationError

	for i := cfg.HeaderRows; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Order number
		orderNumber := strings.TrimSpace(getCell(row, cfg.ColOrderNo))
		if orderNumber == "" {
			continue
		}

		// SKU
		sku := strings.TrimSpace(getCell(row, cfg.ColSKU))
		if sku == "" {
			errs = append(errs, ValidationError{
				Field:   "SKU",
				Message: "SKU cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		// Qty
		qtyStr := strings.TrimSpace(getCell(row, cfg.ColQty))
		if qtyStr == "" {
			errs = append(errs, ValidationError{
				Field:   "Qty",
				Message: "Quantity cannot be empty",
				Row:     rowNum,
			})
			continue
		}
		qty, err := strconv.ParseFloat(qtyStr, 64)
		if err != nil || qty <= 0 {
			errs = append(errs, ValidationError{
				Field:   "Qty",
				Message: fmt.Sprintf("Invalid quantity: %s", qtyStr),
				Row:     rowNum,
			})
			continue
		}

		// Date
		dateRaw := strings.TrimSpace(getCell(row, cfg.ColDate))
		parsedDate := parseDateWithFormats(dateRaw, cfg.DateFormats)
		if parsedDate == "" {
			errs = append(errs, ValidationError{
				Field:   "Date",
				Message: fmt.Sprintf("Invalid date format: %s", dateRaw),
				Row:     rowNum,
			})
			continue
		}

		result = append(result, EcommerceOrderRow{
			Date:        parsedDate,
			Platform:    platform,
			OrderNumber: orderNumber,
			AWBNumber:   strings.TrimSpace(getCell(row, cfg.ColAWB)),
			ProductName: strings.TrimSpace(getCell(row, cfg.ColProduct)),
			Qty:         qty,
			SKU:         sku,
			Row:         rowNum,
		})
	}

	return result, errs
}

// -----------------------------------------------------------------------
// Helper: parseDateWithFormats
// Try multiple date formats, return "YYYY-MM-DD" or ""
// -----------------------------------------------------------------------
func parseDateWithFormats(raw string, formats []string) string {
	if raw == "" {
		return ""
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// -----------------------------------------------------------------------
// Helper: getCell - safe cell getter with trim
// -----------------------------------------------------------------------
func getCell(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

// -----------------------------------------------------------------------
// Helper: groupEcommerceRowsByOrderNumber (unchanged)
// -----------------------------------------------------------------------
func (c *OutboundController) groupEcommerceRowsByOrderNumber(rows []EcommerceOrderRow) map[string][]EcommerceOrderRow {
	result := make(map[string][]EcommerceOrderRow)
	for _, row := range rows {
		result[row.OrderNumber] = append(result[row.OrderNumber], row)
	}
	return result
}

// -----------------------------------------------------------------------
// Helper: validateEcommerceSKUs (unchanged)
// -----------------------------------------------------------------------
func (c *OutboundController) validateEcommerceSKUs(tx *gorm.DB, rows []EcommerceOrderRow) []ValidationError {
	var errs []ValidationError

	skuSet := make(map[string]bool)
	skuRowMap := make(map[string]int)
	for _, r := range rows {
		if !skuSet[r.SKU] {
			skuSet[r.SKU] = true
			skuRowMap[r.SKU] = r.Row
		}
	}

	for sku, rowNum := range skuRowMap {
		var product models.Product
		if err := tx.First(&product, "item_code = ?", sku).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				errs = append(errs, ValidationError{
					Field:   "SKU",
					Message: fmt.Sprintf("Product not found: %s", sku),
					Row:     rowNum,
				})
			} else {
				errs = append(errs, ValidationError{
					Field:   "SKU",
					Message: "Failed to validate SKU: " + err.Error(),
					Row:     rowNum,
				})
			}
		}
	}

	return errs
}

func getCellAsDateStrict(row []string, index int) (string, error) {
	cellValue := strings.TrimSpace(getCell(row, index))
	if cellValue == "" {
		return "", fmt.Errorf("date value is empty")
	}

	// 1. Excel serial date
	if days, err := strconv.ParseFloat(cellValue, 64); err == nil {
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		date := excelEpoch.Add(time.Duration(days * 24 * float64(time.Hour)))
		return date.Format("2006-01-02"), nil
	}

	// 2. String date formats
	dateFormats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"2-Jan-06",
		"2-January-2006",
	}

	for _, format := range dateFormats {
		if t, err := time.Parse(format, cellValue); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("invalid date format: %s", cellValue)
}
