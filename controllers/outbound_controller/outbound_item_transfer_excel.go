package outbound_controller

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/services"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const (
	itemTransferSheetName       = "Item Transfer Detail"
	itemTransferSourceWarehouse = "Yusen WH"
	itemTransferProcess         = "Transfer Out"
)

type ItemTransferUploadResponse struct {
	Success          bool                          `json:"success"`
	Message          string                        `json:"message"`
	TotalRows        int                           `json:"total_rows"`
	ProcessedRows    int                           `json:"processed_rows"`
	SkippedRows      int                           `json:"skipped_rows"`
	SuccessCount     int                           `json:"success_count"`
	FailedCount      int                           `json:"failed_count"`
	OutboundNumbers  []string                      `json:"outbound_numbers,omitempty"`
	SkippedTransfers []ItemTransferSkipped         `json:"skipped_transfers,omitempty"`
	ValidationErrors []ItemTransferValidationError `json:"validation_errors,omitempty"`
	Errors           []ItemTransferRowError        `json:"errors,omitempty"`
}

type ItemTransferSkipped struct {
	TransferNo string `json:"transfer_no"`
	Reason     string `json:"reason"`
}

type ItemTransferValidationError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ItemTransferRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type ItemTransferRow struct {
	Row           int
	TransferNo    string
	TransferDate  string
	OriginalWh    string
	DestinationWh string
	ItemCode      string
	PartCodeItem  string
	ItemName      string
	ModelName     string
	Quantity      float64
	Unit          string
	SerialNumbers []string
	Process       string
}

type ItemTransferHeaderMap map[string]int

var itemTransferRequiredHeaders = []string{
	"transfer no",
	"date",
	"original warehouse",
	"transfer to warehouse",
	"item code",
	"part code item",
	"item name",
	"model name item",
	"quantity",
	"unit",
	"serial number/production",
	"process item transfer",
}

// CreateOutboundFromItemTransferExcel creates one outbound per Transfer No.
// Only Transfer Out rows originating from Yusen WH are imported.
func (c *OutboundController) CreateOutboundFromItemTransferExcel(ctx *fiber.Ctx) error {
	ownerCode := strings.TrimSpace(ctx.FormValue("owner_code"))
	whsCode := strings.TrimSpace(ctx.FormValue("whs_code"))
	if ownerCode == "" || whsCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "Owner code and warehouse code are required",
		})
	}

	upload, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "No file uploaded",
			Errors: []ItemTransferRowError{{Message: "File Error", Detail: err.Error()}},
		})
	}
	name := strings.ToLower(upload.Filename)
	if !strings.HasSuffix(name, ".xlsx") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "Invalid file format. Only .xlsx files are supported",
		})
	}
	if upload.Size == 0 || upload.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "File must be greater than 0 and not exceed 10MB",
		})
	}

	src, err := upload.Open()
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "Failed to open uploaded file",
			Errors: []ItemTransferRowError{{Message: "File Error", Detail: err.Error()}},
		})
	}
	defer src.Close()

	book, err := excelize.OpenReader(src)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "Unable to read Excel workbook",
			Errors: []ItemTransferRowError{{Message: "Excel Read Error", Detail: err.Error()}},
		})
	}
	defer book.Close()

	sheet, err := findItemTransferSheet(book, itemTransferSheetName)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false,
			Message: fmt.Sprintf("Sheet '%s' not found. Available sheets: %s", itemTransferSheetName, strings.Join(book.GetSheetList(), ", ")),
		})
	}
	rows, err := book.GetRows(sheet)
	if err != nil || len(rows) < 2 {
		msg := "Excel file contains no data rows"
		if err != nil {
			msg = "Failed to read worksheet rows: " + err.Error()
		}
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{Success: false, Message: msg})
	}

	headerMap, err := buildItemTransferHeaderMap(rows[0])
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{Success: false, Message: err.Error()})
	}
	parsed, validationErrors, skippedRows := parseItemTransferRows(rows, headerMap)
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			TotalRows: len(rows) - 1, ProcessedRows: len(parsed), SkippedRows: skippedRows,
			ValidationErrors: validationErrors,
		})
	}
	if len(parsed) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
			Success: false, Message: "No eligible Transfer Out rows found from Yusen WH",
			TotalRows: len(rows) - 1, SkippedRows: skippedRows,
		})
	}

	grouped := make(map[string][]ItemTransferRow)
	for _, row := range parsed {
		grouped[row.TransferNo] = append(grouped[row.TransferNo], row)
	}
	transferNos := make([]string, 0, len(grouped))
	for no := range grouped {
		transferNos = append(transferNos, no)
	}
	sort.Strings(transferNos)

	userValue := ctx.Locals("userID")
	userFloat, ok := userValue.(float64)
	if !ok || userFloat <= 0 {
		return ctx.Status(fiber.StatusUnauthorized).JSON(ItemTransferUploadResponse{Success: false, Message: "Invalid user ID"})
	}
	currentUserID := int(userFloat)

	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{
			Success: false, Message: "Failed to start database transaction",
			Errors: []ItemTransferRowError{{Message: "Transaction Error", Detail: tx.Error.Error()}},
		})
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			log.Printf("Panic recovered in CreateOutboundFromItemTransferExcel: %v", r)
		}
	}()

	var policy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", ownerCode).First(&policy).Error; err != nil {
		_ = tx.Rollback().Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ItemTransferUploadResponse{Success: false, Message: "Owner not found: " + ownerCode})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to validate owner", Errors: []ItemTransferRowError{{Message: "Owner Validation Error", Detail: err.Error()}}})
	}

	var warehouse models.Warehouse
	if err := tx.Where("code = ?", whsCode).First(&warehouse).Error; err != nil {
		_ = tx.Rollback().Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ItemTransferUploadResponse{Success: false, Message: "Warehouse not found: " + whsCode})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to validate warehouse", Errors: []ItemTransferRowError{{Message: "Warehouse Validation Error", Detail: err.Error()}}})
	}

	// Validate duplicate Transfer No against existing outbound shipment_id.
	validTransfers := make([]string, 0, len(transferNos))
	skippedTransfers := make([]ItemTransferSkipped, 0)
	for _, transferNo := range transferNos {
		var count int64
		if err := tx.Model(&models.OutboundHeader{}).Where("shipment_id = ?", transferNo).Count(&count).Error; err != nil {
			_ = tx.Rollback().Error
			return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to check duplicate Transfer No", Errors: []ItemTransferRowError{{Message: "Duplicate Check Error", Detail: err.Error()}}})
		}
		if count > 0 {
			skippedTransfers = append(skippedTransfers, ItemTransferSkipped{TransferNo: transferNo, Reason: "Transfer No already exists as an outbound Shipment ID"})
			continue
		}
		validTransfers = append(validTransfers, transferNo)
	}
	if len(validTransfers) == 0 {
		_ = tx.Rollback().Error
		return ctx.Status(fiber.StatusOK).JSON(ItemTransferUploadResponse{
			Success: false, Message: "No outbound created. All Transfer No values already exist.",
			TotalRows: len(rows) - 1, ProcessedRows: len(parsed), SkippedRows: skippedRows,
			FailedCount: len(skippedTransfers), SkippedTransfers: skippedTransfers,
		})
	}

	// Resolve all destination warehouses against Customer Master.
	destinationNames := map[string]struct{}{}
	for _, no := range validTransfers {
		for _, row := range grouped[no] {
			destinationNames[strings.TrimSpace(row.DestinationWh)] = struct{}{}
		}
	}
	names := make([]string, 0, len(destinationNames))
	for name := range destinationNames {
		names = append(names, name)
	}
	var customers []models.Customer
	if err := tx.Where("customer_name IN ?", names).Find(&customers).Error; err != nil {
		_ = tx.Rollback().Error
		return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to lookup destination customers", Errors: []ItemTransferRowError{{Message: "Customer Lookup Error", Detail: err.Error()}}})
	}
	customerMap := make(map[string]models.Customer, len(customers))
	for _, customer := range customers {
		customerMap[strings.ToLower(strings.TrimSpace(customer.CustomerName))] = customer
	}

	repo := repositories.NewOutboundRepository(tx)
	outboundNumbers := make([]string, 0, len(validTransfers))
	totalDetails := 0

	for _, transferNo := range validTransfers {
		items := grouped[transferNo]
		if len(items) == 0 {
			continue
		}
		first := items[0]

		// One Transfer No must have one destination and one transfer date.
		for _, row := range items {
			if !strings.EqualFold(row.DestinationWh, first.DestinationWh) {
				_ = tx.Rollback().Error
				return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
					Success: false, Message: fmt.Sprintf("Transfer No %s contains more than one Transfer To Warehouse", transferNo),
					Errors: []ItemTransferRowError{{Row: row.Row, Message: "Inconsistent destination", Detail: row.DestinationWh}},
				})
			}
			if row.TransferDate != first.TransferDate {
				_ = tx.Rollback().Error
				return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{
					Success: false, Message: fmt.Sprintf("Transfer No %s contains inconsistent dates", transferNo),
					Errors: []ItemTransferRowError{{Row: row.Row, Message: "Inconsistent transfer date", Detail: row.TransferDate}},
				})
			}
		}

		customer, found := customerMap[strings.ToLower(strings.TrimSpace(first.DestinationWh))]
		if !found {
			_ = tx.Rollback().Error
			return ctx.Status(fiber.StatusNotFound).JSON(ItemTransferUploadResponse{
				Success: false, Message: "Customer not found for Transfer To Warehouse: " + first.DestinationWh,
			})
		}

		outboundNo, err := repo.GenerateOutboundNumber()
		if err != nil {
			_ = tx.Rollback().Error
			return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to generate outbound number", Errors: []ItemTransferRowError{{Row: first.Row, Message: "Generation Error", Detail: err.Error()}}})
		}

		now := time.Now()
		nowDate, nowTime := now.Format("2006-01-02"), now.Format("15:04")
		customerAddress := strings.TrimSpace(strings.Join([]string{customer.CustAddr1, customer.CustAddr2}, " "))
		header := models.OutboundHeader{
			OutboundNo:         outboundNo,
			OutboundDate:       first.TransferDate,
			ShipmentID:         transferNo,
			CustomerCode:       customer.CustomerCode,
			WhsCode:            whsCode,
			OrderType:          "B2B - Normal",
			OwnerCode:          ownerCode,
			Status:             "open",
			RawStatus:          "DRAFT",
			DraftTime:          now,
			Remarks:            fmt.Sprintf("ITEM TRANSFER | Transfer No: %s | From: %s | To: %s", transferNo, first.OriginalWh, first.DestinationWh),
			Source:             "ITEM_TRANSFER",
			Integration:        false,
			CustAddress:        customerAddress,
			CustCity:           customer.CustCity,
			DelivTo:            customer.CustomerCode,
			DelivAddress:       customerAddress,
			DelivCity:          customer.CustCity,
			ActualDelivTo:      customer.CustomerCode,
			ActualDelivAddress: customerAddress,
			CreatedBy:          currentUserID,
			UpdatedBy:          currentUserID,
			PlanPickupDate:     nowDate,
			PlanPickupTime:     "16:00",
			RcvDoDate:          nowDate,
			RcvDoTime:          nowTime,
			StartPickTime:      nowTime,
			EndPickTime:        nowTime,
		}
		if err := tx.Create(&header).Error; err != nil {
			_ = tx.Rollback().Error
			return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to create outbound header for Transfer No " + transferNo, Errors: []ItemTransferRowError{{Row: first.Row, Message: "Header Insert Error", Detail: err.Error()}}})
		}

		expandedItems, err := expandItemTransferOutboundItems(tx, items)
		if err != nil {
			_ = tx.Rollback().Error
			return ctx.Status(fiber.StatusBadRequest).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to expand transfer items", Errors: []ItemTransferRowError{{Row: first.Row, Message: "Bundle Expansion Error", Detail: err.Error()}}})
		}

		seenSerials := make(map[string]bool)
		for _, item := range expandedItems {
			var product models.Product
			if err := tx.Where("item_code = ?", item.ItemCode).First(&product).Error; err != nil {
				_ = tx.Rollback().Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(ItemTransferUploadResponse{Success: false, Message: "Product not found: " + item.ItemCode, Errors: []ItemTransferRowError{{Row: first.Row, Message: "Product Not Found", Detail: item.ItemCode}}})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to lookup product", Errors: []ItemTransferRowError{{Row: first.Row, Message: "Product Lookup Error", Detail: err.Error()}}})
			}

			var conversion models.UomConversion
			if err := tx.Where("item_code = ? AND factor = 1", product.ItemCode).First(&conversion).Error; err != nil {
				if err2 := tx.Where("item_code = ?", product.ItemCode).First(&conversion).Error; err2 != nil {
					_ = tx.Rollback().Error
					return ctx.Status(fiber.StatusNotFound).JSON(ItemTransferUploadResponse{Success: false, Message: "UOM conversion not found for SKU: " + item.ItemCode, Errors: []ItemTransferRowError{{Row: first.Row, Message: "UOM Not Found", Detail: item.ItemCode}}})
				}
			}
			detailUOM := conversion.FromUom
			if strings.TrimSpace(item.UOM) != "" {
				var excelUOM models.UomConversion
				if err := tx.Where("item_code = ? AND from_uom = ?", product.ItemCode, strings.TrimSpace(item.UOM)).First(&excelUOM).Error; err == nil {
					conversion, detailUOM = excelUOM, excelUOM.FromUom
				}
			}
			detail := models.OutboundDetail{
				OutboundNo: outboundNo, OutboundID: header.ID, ItemCode: product.ItemCode,
				ItemID: int(product.ID), Barcode: conversion.Ean, CustomerCode: customer.CustomerCode,
				Uom: detailUOM, Quantity: item.Quantity, WhsCode: whsCode,
				DivisionCode: "REGULAR", QaStatus: "A", OwnerCode: ownerCode,
				SerialNumber: "", BundleProductID: item.BundleProductID,
				BundleProductCode: item.BundleProductCode, BundleQuantity: item.BundleQuantity,
				SNCheck: "N", CreatedBy: currentUserID, UpdatedBy: currentUserID,
			}
			if err := tx.Create(&detail).Error; err != nil {
				_ = tx.Rollback().Error
				return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to create outbound detail for SKU: " + item.ItemCode, Errors: []ItemTransferRowError{{Row: first.Row, Message: "Detail Insert Error", Detail: err.Error()}}})
			}

			for _, serialText := range item.SerialNumbers {
				serialText = strings.TrimSpace(serialText)
				if serialText == "" {
					continue
				}
				key := strings.ToUpper(serialText)
				if seenSerials[key] {
					continue
				}
				seenSerials[key] = true
				serial := models.OutboundSerial{
					OutboundId: int(header.ID), OutboundDetailId: int(detail.ID),
					SerialNumber: serialText, CreatedBy: currentUserID, UpdatedBy: currentUserID,
				}
				if err := tx.Create(&serial).Error; err != nil {
					_ = tx.Rollback().Error
					return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to insert outbound serial", Errors: []ItemTransferRowError{{Row: first.Row, Message: "Serial Insert Error", Detail: err.Error()}}})
				}
			}
			totalDetails++
		}

		if err := helpers.InsertTransactionHistory(tx, outboundNo, "open", "OUTBOUND",
			fmt.Sprintf("Created from Item Transfer Excel - Transfer No: %s", transferNo), currentUserID); err != nil {
			log.Printf("Warning: failed to insert transaction history for %s: %v", outboundNo, err)
		}
		outboundNumbers = append(outboundNumbers, outboundNo)
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ItemTransferUploadResponse{Success: false, Message: "Failed to commit transaction", Errors: []ItemTransferRowError{{Message: "Transaction Commit Error", Detail: err.Error()}}})
	}

	message := fmt.Sprintf("Created %d outbound(s) with %d item(s) from Item Transfer Excel", len(outboundNumbers), totalDetails)
	if len(skippedTransfers) > 0 {
		message += fmt.Sprintf(". %d Transfer No skipped because already exist", len(skippedTransfers))
	}
	if skippedRows > 0 {
		message += fmt.Sprintf(". %d row(s) skipped because they are not Transfer Out from %s", skippedRows, itemTransferSourceWarehouse)
	}
	return ctx.Status(fiber.StatusOK).JSON(ItemTransferUploadResponse{
		Success: true, Message: message, TotalRows: len(rows) - 1, ProcessedRows: len(parsed),
		SkippedRows: skippedRows, SuccessCount: totalDetails, FailedCount: len(skippedTransfers),
		OutboundNumbers: outboundNumbers, SkippedTransfers: skippedTransfers,
	})
}

func expandItemTransferOutboundItems(tx *gorm.DB, rows []ItemTransferRow) ([]OutboundItem, error) {
	bundleService := services.NewProductBundleService(tx)
	var expanded []OutboundItem

	for _, row := range rows {
		var product models.Product
		if err := tx.Where("item_code = ?", strings.TrimSpace(row.ItemCode)).First(&product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("product not found: %s", row.ItemCode)
			}
			return nil, err
		}

		if product.IsBundle != "Y" {
			expanded = append(expanded, OutboundItem{
				ItemCode: product.ItemCode, Quantity: row.Quantity, UOM: row.Unit,
				Remarks: row.PartCodeItem, SerialNumber: "", SerialNumbers: row.SerialNumbers,
			})
			continue
		}

		components, err := bundleService.ExpandBundle(tx, product.ID, row.Quantity)
		if err != nil {
			return nil, fmt.Errorf("failed to expand bundle %s: %w", product.ItemCode, err)
		}
		for _, component := range components {
			expanded = append(expanded, OutboundItem{
				ItemCode: component.ItemCode, Quantity: component.Qty, UOM: component.UOM,
				BundleProductID: int(product.ID), BundleProductCode: product.ItemCode,
				BundleQuantity: row.Quantity, Remarks: row.PartCodeItem,
				SerialNumber: "", SerialNumbers: row.SerialNumbers,
				LotNumber: "", ExpDate: "", CartonNumber: "", CaseNumber: "", Location: "",
			})
		}
	}
	return expanded, nil
}

func buildItemTransferHeaderMap(headerRow []string) (ItemTransferHeaderMap, error) {
	result := make(ItemTransferHeaderMap)
	for i, header := range headerRow {
		key := normalizeItemTransferHeader(header)
		if key == "" {
			continue
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate Excel header found: '%s'", header)
		}
		result[key] = i
	}
	for _, required := range itemTransferRequiredHeaders {
		if _, ok := result[required]; !ok {
			return nil, fmt.Errorf("required Excel header '%s' not found", required)
		}
	}
	return result, nil
}

func parseItemTransferRows(rows [][]string, headers ItemTransferHeaderMap) ([]ItemTransferRow, []ItemTransferValidationError, int) {
	var result []ItemTransferRow
	var validation []ItemTransferValidationError
	skipped := 0
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if itemTransferRowEmpty(row) {
			continue
		}
		rowNo := i + 1
		originalWh := strings.TrimSpace(getItemTransferCell(row, headers, "original warehouse"))
		process := strings.TrimSpace(getItemTransferCell(row, headers, "process item transfer"))
		if !strings.EqualFold(originalWh, itemTransferSourceWarehouse) || !strings.EqualFold(process, itemTransferProcess) {
			skipped++
			continue
		}
		get := func(name string) string { return strings.TrimSpace(getItemTransferCell(row, headers, name)) }
		transferNo := get("transfer no")
		itemCode := cleanItemTransferCode(get("item code"))
		destination := get("transfer to warehouse")
		qtyRaw := strings.ReplaceAll(get("quantity"), ",", "")
		qty, qtyErr := strconv.ParseFloat(qtyRaw, 64)
		date := parseItemTransferDate(get("date"))
		fields := []struct{ name, value string }{
			{"Transfer No", transferNo}, {"Transfer To Warehouse", destination}, {"Item Code", itemCode}, {"Date", date},
		}
		for _, field := range fields {
			if field.value == "" {
				validation = append(validation, ItemTransferValidationError{Row: rowNo, Field: field.name, Message: field.name + " cannot be empty"})
			}
		}
		if qtyErr != nil || qty <= 0 {
			validation = append(validation, ItemTransferValidationError{Row: rowNo, Field: "Quantity", Message: "Quantity must be a number greater than zero"})
		}
		if len(validation) > 0 && validation[len(validation)-1].Row == rowNo {
			continue
		}
		serials := splitItemTransferSerials(get("serial number/production"))
		result = append(result, ItemTransferRow{
			Row: rowNo, TransferNo: transferNo, TransferDate: date,
			OriginalWh: originalWh, DestinationWh: destination, ItemCode: itemCode,
			PartCodeItem: get("part code item"), ItemName: get("item name"),
			ModelName: get("model name item"), Quantity: qty, Unit: get("unit"),
			SerialNumbers: serials, Process: process,
		})
	}
	return result, validation, skipped
}

func getItemTransferCell(row []string, headers ItemTransferHeaderMap, header string) string {
	index, ok := headers[normalizeItemTransferHeader(header)]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func normalizeItemTransferHeader(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func itemTransferRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func cleanItemTransferCode(value string) string {
	value = strings.TrimSpace(value)
	if dot := strings.Index(value, "."); dot >= 0 {
		decimal := value[dot+1:]
		if decimal != "" {
			allZero := true
			for _, ch := range decimal {
				if ch != '0' {
					allZero = false
					break
				}
			}
			if allZero {
				return value[:dot]
			}
		}
	}
	return value
}

func parseItemTransferDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if serial, err := strconv.ParseFloat(raw, 64); err == nil && serial > 0 {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.Add(time.Duration(serial * float64(24*time.Hour))).Format("2006-01-02")
	}
	formats := []string{
		"2006-01-02", "2006-01-02 15:04:05",
		"02/01/2006", "02/01/2006 15:04:05",
		"01/02/2006", "1/2/2006", "02-01-2006",
		"2-Jan-2006", "02-Jan-2006", "2 Jan 2006", "02 Jan 2006",
	}
	for _, layout := range formats {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}

func splitItemTransferSerials(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	normalized := strings.NewReplacer("\r\n", ",", "\n", ",", "\r", ",", ";", ",").Replace(raw)
	parts := strings.Split(normalized, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func findItemTransferSheet(book *excelize.File, expected string) (string, error) {
	target := normalizeItemTransferHeader(expected)
	for _, sheet := range book.GetSheetList() {
		if normalizeItemTransferHeader(sheet) == target {
			return sheet, nil
		}
	}
	return "", fmt.Errorf("sheet '%s' not found", expected)
}
