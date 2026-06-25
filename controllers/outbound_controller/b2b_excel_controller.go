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
// B2B EXCEL UPLOAD CONTROLLER
// POST /outbound/upload-b2b-excel
// Form fields:
//   - file       : Excel file (Template_B2B.xlsx format)
//   - owner_code : owner code (dipilih user dari dropdown master owner)
//   - whs_code   : warehouse code (dipilih user dari dropdown)
// ======================================================================

// ─── Column indices (0-based) untuk Template B2B ────────────────────────────
// Col 0  : No             (skip)
// Col 1  : Date send Memo to Yusen  → outbound_date
// Col 2  : Date Pick up from Yusen  (skip / info)
// Col 3  : Memo PO                  → shipment_id (order grouping key)
// Col 4  : SKU Number               → item_code
// Col 5  : Customer Name            → lookup master, DelivTo
// Col 6  : Goods                    → item name / remarks detail
// Col 7  : Qty
// Col 8  : Unit                     → uom hint (pakai UOM dari master)
// Col 9  : Delivery Address         → deliv_address
// Col 10 : Reguler/Express          → order_type hint
// Col 11 : Express Urgent/Sameday   (skip / info)
// Col 12 : PIC No.                  → masuk Remarks header
// Col 13 : Remarks                  → masuk Remarks header
// ─────────────────────────────────────────────────────────────────────────────

const (
	b2bColDate      = 1
	b2bColMemoPO    = 3
	b2bColSKU       = 4
	b2bColCustomer  = 5
	b2bColGoods     = 6
	b2bColQty       = 7
	b2bColUnit      = 8
	b2bColDelivAddr = 9
	b2bColOrderType = 10
	b2bColPIC       = 12
	b2bColRemarks   = 13
	b2bHeaderRows   = 1 // 1 header row
	b2bSheetName    = "Form Ke Yusen"
)

// ─── Structs ─────────────────────────────────────────────────────────────────

type B2BUploadResponse struct {
	Success          bool                 `json:"success"`
	Message          string               `json:"message"`
	TotalRows        int                  `json:"total_rows"`
	SuccessCount     int                  `json:"success_count"`
	FailedCount      int                  `json:"failed_count"`
	OutboundNumbers  []string             `json:"outbound_numbers,omitempty"`
	SkippedOrders    []SkippedOrder       `json:"skipped_orders,omitempty"`
	UnknownCustomers []B2BUnknownCustomer `json:"unknown_customers,omitempty"`
	Errors           []ExcelRowError      `json:"errors,omitempty"`
	ValidationErrors []ValidationError    `json:"validation_errors,omitempty"`
}

// B2BUnknownCustomer — customer name tidak ditemukan di master, outbound tetap dibuat
// tapi user perlu lengkapi manual lewat edit
type B2BUnknownCustomer struct {
	OutboundNo   string `json:"outbound_no"`
	MemoPO       string `json:"memo_po"`
	CustomerName string `json:"customer_name"`
}

// B2BOrderRow — satu baris data dari Excel yang sudah di-parse
type B2BOrderRow struct {
	Row          int
	Date         string
	MemoPO       string // shipment_id
	SKU          string
	CustomerName string
	Goods        string
	Qty          float64
	Unit         string
	DelivAddress string
	OrderType    string // "Reguler" atau "Express Urgent / Sameday"
	PIC          string
	Remarks      string
}

// ─── Handler ─────────────────────────────────────────────────────────────────

func (c *OutboundController) CreateOutboundFromB2BExcel(ctx *fiber.Ctx) error {

	// 1. Ambil & validasi form params
	ownerCode := strings.TrimSpace(ctx.FormValue("owner_code"))
	if ownerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "Owner code is required",
		})
	}

	whsCode := strings.TrimSpace(ctx.FormValue("whs_code"))
	if whsCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "Warehouse code is required",
		})
	}

	// 2. Ambil file
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors:  []ExcelRowError{{Row: 0, Message: "File Error", Detail: err.Error()}},
		})
	}

	// 3. Validasi ekstensi
	lowerName := strings.ToLower(file.Filename)
	if !strings.HasSuffix(lowerName, ".xlsx") && !strings.HasSuffix(lowerName, ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// 4. Validasi ukuran (max 10MB)
	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "File size exceeds maximum limit of 10MB",
		})
	}

	// 5. Buka Excel
	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors:  []ExcelRowError{{Row: 0, Message: "File Error", Detail: err.Error()}},
		})
	}
	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors:  []ExcelRowError{{Row: 0, Message: "Excel Read Error", Detail: err.Error()}},
		})
	}
	defer excelFile.Close()

	// 6. Find sheet (case-insensitive)
	sheetName, err := findSheet(excelFile, b2bSheetName)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: fmt.Sprintf("Sheet '%s' not found. Available sheets: %s",
				b2bSheetName, strings.Join(excelFile.GetSheetList(), ", ")),
		})
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors:  []ExcelRowError{{Row: 0, Message: "Sheet Read Error", Detail: err.Error()}},
		})
	}

	if len(rows) <= b2bHeaderRows {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "Excel file contains no data rows",
		})
	}

	// 7. Parse rows
	orderRows, validationErrors := c.parseB2BRows(rows)
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			ValidationErrors: validationErrors,
			TotalRows:        len(rows) - b2bHeaderRows,
		})
	}

	if len(orderRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(B2BUploadResponse{
			Success: false,
			Message: "No valid data rows found in Excel file",
		})
	}

	// 8. Group by Memo PO
	orderMap := c.groupB2BRowsByMemoPO(orderRows)

	// Sorted order numbers untuk consistent output
	memoPOs := make([]string, 0, len(orderMap))
	for k := range orderMap {
		memoPOs = append(memoPOs, k)
	}
	sort.Strings(memoPOs)

	// 9. Get userID
	userID := int(ctx.Locals("userID").(float64))

	// 10. Start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateOutboundFromB2BExcel: %v", r)
		}
	}()

	// 11. Validasi master data: Owner & Warehouse
	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", ownerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(B2BUploadResponse{
				Success: false,
				Message: "Owner not found: " + ownerCode,
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to validate owner: " + err.Error(),
		})
	}

	var warehouse models.Warehouse
	if err := tx.First(&warehouse, "code = ?", whsCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(B2BUploadResponse{
				Success: false,
				Message: "Warehouse not found: " + whsCode,
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to validate warehouse: " + err.Error(),
		})
	}

	// 12. Validasi SKU per order — skip order yang ada SKU invalid
	invalidOrdersBySKU := c.validateB2BSKUsPerOrder(tx, orderMap)

	// 13. Duplicate check — Memo PO yang sudah ada sebagai shipment_id
	duplicateOrders := c.checkDuplicateShipmentIDs(tx, memoPOs)

	// Merge semua skipped
	skippedOrders := make(map[string]string)
	for k, v := range invalidOrdersBySKU {
		skippedOrders[k] = v
	}
	for k, v := range duplicateOrders {
		skippedOrders[k] = v
	}

	// Filter valid orders
	validMemoPOs := make([]string, 0)
	for _, memoPO := range memoPOs {
		if _, skipped := skippedOrders[memoPO]; !skipped {
			validMemoPOs = append(validMemoPOs, memoPO)
		}
	}

	if len(validMemoPOs) == 0 {
		tx.Rollback()
		skippedList := buildSkippedList(skippedOrders)
		return ctx.Status(fiber.StatusOK).JSON(B2BUploadResponse{
			Success:       false,
			Message:       "No valid orders to process. All orders were skipped.",
			TotalRows:     len(orderRows),
			SuccessCount:  0,
			FailedCount:   len(skippedOrders),
			SkippedOrders: skippedList,
		})
	}

	// 14. Batch lookup semua customer names yang ada di valid orders
	// Build unique customer name set
	customerNameSet := make(map[string]bool)
	for _, memoPO := range validMemoPOs {
		items := orderMap[memoPO]
		if len(items) > 0 {
			customerNameSet[items[0].CustomerName] = true
		}
	}

	// Lookup customer master berdasarkan customer_name
	customerNameToCode := make(map[string]string) // name → customer_code
	if len(customerNameSet) > 0 {
		names := make([]string, 0, len(customerNameSet))
		for n := range customerNameSet {
			names = append(names, n)
		}
		var customers []models.Customer
		tx.Where("customer_name IN ?", names).Find(&customers)
		for _, cust := range customers {
			customerNameToCode[cust.CustomerName] = cust.CustomerCode
		}
	}

	// 15. Buat outbound per valid Memo PO
	repo := repositories.NewOutboundRepository(tx)
	var outboundNumbers []string
	var unknownCustomers []B2BUnknownCustomer
	totalSuccessItems := 0

	for _, memoPO := range validMemoPOs {
		items := orderMap[memoPO]
		firstItem := items[0]

		outboundNo, err := repo.GenerateOutboundNumber()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
				Success: false,
				Message: "Failed to generate outbound number",
				Errors:  []ExcelRowError{{Row: 0, Message: "Generation Error", Detail: err.Error()}},
			})
		}

		// Resolve customer
		custCode, custFound := customerNameToCode[firstItem.CustomerName]

		// Remarks header: gabung PIC + Remarks dari baris pertama
		remarksHeader := fmt.Sprintf("B2B | Memo PO: %s", memoPO)
		if firstItem.PIC != "" {
			remarksHeader += fmt.Sprintf(" | PIC: %s", firstItem.PIC)
		}
		if firstItem.Remarks != "" {
			remarksHeader += fmt.Sprintf(" | %s", firstItem.Remarks)
		}

		// Order type mapping
		orderType := "B2B - Normal"
		if strings.Contains(strings.ToLower(firstItem.OrderType), "express") ||
			strings.Contains(strings.ToLower(firstItem.OrderType), "urgent") ||
			strings.Contains(strings.ToLower(firstItem.OrderType), "sameday") {
			orderType = "B2B - Express"
		}

		now := time.Now()
		nowDate := now.Format("2006-01-02")
		nowTime := now.Format("15:04")
		defaultPickupTime := "16:00"

		// Jika customer tidak ditemukan, customer_code = null (empty string, handled di DB)
		var customerCode *string
		if custFound {
			cc := custCode
			customerCode = &cc
		}

		outboundHeader := models.OutboundHeader{
			OutboundNo:     outboundNo,
			OutboundDate:   firstItem.Date,
			ShipmentID:     memoPO,
			OrderType:      orderType,
			WhsCode:        whsCode,
			OwnerCode:      ownerCode,
			Remarks:        remarksHeader,
			Status:         "open",
			RawStatus:      "DRAFT",
			DraftTime:      now,
			Source:         "B2B",
			DelivTo:        firstItem.CustomerName, // nama customer dari Excel sebagai DelivTo
			DelivAddress:   firstItem.DelivAddress,
			DelivCity:      "",
			PlanPickupDate: nowDate,
			PlanPickupTime: defaultPickupTime,
			RcvDoDate:      nowDate,
			RcvDoTime:      nowTime,
			StartPickTime:  nowTime,
			EndPickTime:    nowTime,
			CreatedBy:      userID,
			UpdatedBy:      userID,
		}

		// Set customer_code hanya jika ditemukan di master
		if customerCode != nil {
			outboundHeader.CustomerCode = *customerCode
			outboundHeader.CustAddress = firstItem.DelivAddress
			outboundHeader.CustCity = ""
		}

		if err := tx.Create(&outboundHeader).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
				Success: false,
				Message: "Failed to create outbound header for Memo PO: " + memoPO,
				Errors:  []ExcelRowError{{Row: firstItem.Row, Message: "Database Insert Error", Detail: err.Error()}},
			})
		}

		// Track unknown customer
		if !custFound && firstItem.CustomerName != "" {
			unknownCustomers = append(unknownCustomers, B2BUnknownCustomer{
				OutboundNo:   outboundNo,
				MemoPO:       memoPO,
				CustomerName: firstItem.CustomerName,
			})
		}

		// Buat detail per item
		for _, item := range items {
			var product models.Product
			if err := tx.First(&product, "item_code = ?", item.SKU).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
					Success: false,
					Message: "Product not found during detail creation: " + item.SKU,
					Errors:  []ExcelRowError{{Row: item.Row, Message: "Product Not Found", Detail: "SKU: " + item.SKU}},
				})
			}

			var uomConversion models.UomConversion
			if err := tx.Where("item_code = ? AND factor = 1", product.ItemCode).First(&uomConversion).Error; err != nil {
				if err2 := tx.Where("item_code = ?", product.ItemCode).First(&uomConversion).Error; err2 != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(B2BUploadResponse{
						Success: false,
						Message: "UOM conversion not found for SKU: " + item.SKU,
						Errors:  []ExcelRowError{{Row: item.Row, Message: "UOM Not Found", Detail: "SKU: " + item.SKU}},
					})
				}
			}

			detailCustomerCode := ""
			if customerCode != nil {
				detailCustomerCode = *customerCode
			}

			outboundDetail := models.OutboundDetail{
				OutboundNo:   outboundNo,
				OutboundID:   outboundHeader.ID,
				ItemCode:     product.ItemCode,
				ItemID:       int(product.ID),
				Barcode:      uomConversion.Ean,
				CustomerCode: detailCustomerCode,
				Uom:          uomConversion.FromUom,
				Quantity:     item.Qty,
				WhsCode:      whsCode,
				DivisionCode: "SALES",
				QaStatus:     "A",
				OwnerCode:    ownerCode,
				Remarks:      item.Goods,
				SNCheck:      "N",
				CreatedBy:    userID,
				UpdatedBy:    userID,
			}

			if err := tx.Create(&outboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
					Success: false,
					Message: "Failed to create outbound detail for SKU: " + item.SKU,
					Errors:  []ExcelRowError{{Row: item.Row, Message: "Database Insert Error", Detail: err.Error()}},
				})
			}

			totalSuccessItems++
		}

		if err := helpers.InsertTransactionHistory(tx, outboundNo, "open", "OUTBOUND",
			fmt.Sprintf("Created from B2B upload - Memo PO: %s", memoPO), userID); err != nil {
			log.Printf("Warning: Failed to insert transaction history for %s: %v", outboundNo, err)
		}

		outboundNumbers = append(outboundNumbers, outboundNo)
	}

	// 16. Commit
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(B2BUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors:  []ExcelRowError{{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()}},
		})
	}

	skippedList := buildSkippedList(skippedOrders)
	msg := fmt.Sprintf("Created %d outbound(s) with %d item(s) from B2B Excel", len(outboundNumbers), totalSuccessItems)
	if len(skippedList) > 0 {
		msg += fmt.Sprintf(". %d Memo PO skipped.", len(skippedList))
	}
	if len(unknownCustomers) > 0 {
		msg += fmt.Sprintf(" %d outbound(s) have unmatched customer — please complete via Edit.", len(unknownCustomers))
	}

	return ctx.Status(fiber.StatusOK).JSON(B2BUploadResponse{
		Success:          true,
		Message:          msg,
		TotalRows:        len(orderRows),
		SuccessCount:     totalSuccessItems,
		FailedCount:      len(skippedList),
		OutboundNumbers:  outboundNumbers,
		SkippedOrders:    skippedList,
		UnknownCustomers: unknownCustomers,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseB2BRows — baca rows Excel dan parse ke []B2BOrderRow
// Skip baris kosong (Memo PO kosong) dan baris error validasi dikumpulkan
func (c *OutboundController) parseB2BRows(rows [][]string) ([]B2BOrderRow, []ValidationError) {
	var result []B2BOrderRow
	var errs []ValidationError

	for i := b2bHeaderRows; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		if len(row) == 0 {
			continue
		}

		// Memo PO — wajib ada (kunci grouping)
		memoPO := strings.TrimSpace(getCell(row, b2bColMemoPO))
		if memoPO == "" {
			continue // baris kosong / trailing rows, skip
		}

		// SKU — wajib ada
		skuRaw := strings.TrimSpace(getCell(row, b2bColSKU))
		if skuRaw == "" {
			errs = append(errs, ValidationError{
				Field:   "SKU Number",
				Message: "SKU cannot be empty",
				Row:     rowNum,
			})
			continue
		}
		// SKU bisa berupa integer dari Excel (e.g. 30026187) → pastikan string tanpa desimal
		sku := cleanSKU(skuRaw)

		// Qty — wajib ada dan > 0
		qtyStr := strings.TrimSpace(getCell(row, b2bColQty))
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

		// Date — ambil dari col 1, bisa Excel serial atau string datetime
		dateRaw := strings.TrimSpace(getCell(row, b2bColDate))
		parsedDate := parseB2BDate(dateRaw)
		if parsedDate == "" {
			errs = append(errs, ValidationError{
				Field:   "Date",
				Message: fmt.Sprintf("Invalid date format: %s", dateRaw),
				Row:     rowNum,
			})
			continue
		}

		result = append(result, B2BOrderRow{
			Row:          rowNum,
			Date:         parsedDate,
			MemoPO:       memoPO,
			SKU:          sku,
			CustomerName: strings.TrimSpace(getCell(row, b2bColCustomer)),
			Goods:        strings.TrimSpace(getCell(row, b2bColGoods)),
			Qty:          qty,
			Unit:         strings.TrimSpace(getCell(row, b2bColUnit)),
			DelivAddress: strings.TrimSpace(getCell(row, b2bColDelivAddr)),
			OrderType:    strings.TrimSpace(getCell(row, b2bColOrderType)),
			PIC:          strings.TrimSpace(getCell(row, b2bColPIC)),
			Remarks:      strings.TrimSpace(getCell(row, b2bColRemarks)),
		})
	}

	return result, errs
}

// groupB2BRowsByMemoPO — group rows by Memo PO number
func (c *OutboundController) groupB2BRowsByMemoPO(rows []B2BOrderRow) map[string][]B2BOrderRow {
	result := make(map[string][]B2BOrderRow)
	for _, row := range rows {
		result[row.MemoPO] = append(result[row.MemoPO], row)
	}
	return result
}

// validateB2BSKUsPerOrder — batch check SKU validity, return map[memoPO]reason untuk yang invalid
func (c *OutboundController) validateB2BSKUsPerOrder(
	tx *gorm.DB,
	orderMap map[string][]B2BOrderRow,
) map[string]string {
	// Collect semua unique SKU
	skuValid := make(map[string]bool)
	skuSeen := make(map[string]bool)
	allSKUs := make([]string, 0)
	for _, items := range orderMap {
		for _, item := range items {
			if !skuSeen[item.SKU] {
				skuSeen[item.SKU] = true
				allSKUs = append(allSKUs, item.SKU)
			}
		}
	}

	// Batch query
	var foundProducts []models.Product
	tx.Where("item_code IN ?", allSKUs).Find(&foundProducts)
	for _, p := range foundProducts {
		skuValid[p.ItemCode] = true
	}

	// Check per order
	invalidOrders := make(map[string]string)
	for memoPO, items := range orderMap {
		for _, item := range items {
			if !skuValid[item.SKU] {
				invalidOrders[memoPO] = fmt.Sprintf("SKU not found: %s", item.SKU)
				break
			}
		}
	}

	return invalidOrders
}

// cleanSKU — handle SKU yang mungkin muncul sebagai "30026187.0" dari Excel float
func cleanSKU(raw string) string {
	// Jika ada titik desimal (e.g. "30026187.0"), buang bagian desimal
	if idx := strings.Index(raw, "."); idx != -1 {
		// Cek apakah bagian desimalnya semua "0"
		decimal := raw[idx+1:]
		allZero := true
		for _, ch := range decimal {
			if ch != '0' {
				allZero = false
				break
			}
		}
		if allZero {
			return raw[:idx]
		}
	}
	return raw
}

// parseB2BDate — handle Excel serial number dan berbagai format string date
func parseB2BDate(raw string) string {
	if raw == "" {
		return ""
	}

	// 1. Coba parse sebagai Excel serial number (integer)
	if days, err := strconv.ParseFloat(raw, 64); err == nil && days > 40000 {
		// Excel epoch: 1899-12-30 (excelize convention)
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		date := excelEpoch.Add(time.Duration(days) * 24 * time.Hour)
		return date.Format("2006-01-02")
	}

	// 2. Coba berbagai format string date
	dateFormats := []string{
		// Format yang dipakai excelize saat membaca cell date sebagai string
		// e.g. "24-Jun-26" → excelize format DD-Mon-YY
		"2-Jan-06",
		"02-Jan-06",
		"2-January-06",
		"02-January-06",
		"2-Jan-2006",
		"02-Jan-2006",
		"2-January-2006",
		"02-January-2006",
		// Format standar lainnya
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
	}

	for _, f := range dateFormats {
		if t, err := time.Parse(f, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}

	return ""
}
