package inbound_controller

import (
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// ============================================================================
// FURUNO INBOUND EXCEL IMPORT
// ============================================================================
// Dedicated Furuno controller functions/types intentionally use the Furuno
// prefix so this file can live beside the existing/default inbound importer
// without redeclaration conflicts.
//
// Expected Furuno columns:
//   0  DO Number (informational only; not used as ReceiptID)
//   1  DO Date
//   2  Packing List # -> ReceiptID
//   3  Vessel Related Remarks
//   4  Customer Name
//   5  Cust. Ref No.
//   6  Model/Part No
//   7  Item code
//   8  Model Name
//   9  DO Quantity
//   10 Vendor Serial Number
//   11 FSG Serial Number
//   12 FSG Serial Number Quantity
//   13 Remarks For DO
//
// The Excel file does NOT contain several fields required by the WMS inbound
// model. Those fields are supplied through multipart form fields.
// ============================================================================

// ============================================================================
// RESPONSE / VALIDATION TYPES
// ============================================================================

type FurunoExcelUploadResponse struct {
	Success          bool                    `json:"success"`
	Message          string                  `json:"message"`
	TotalRows        int                     `json:"total_rows"`
	ProcessedRows    int                     `json:"processed_rows"`
	SkippedRows      int                     `json:"skipped_rows"`
	SuccessCount     int                     `json:"success_count"`
	FailedCount      int                     `json:"failed_count"`
	InboundNumbers   []string                `json:"inbound_numbers,omitempty"`
	Errors           []FurunoExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []FurunoValidationError `json:"validation_errors,omitempty"`
}

type FurunoExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type FurunoValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Row     int    `json:"row"`
}

// ============================================================================
// FURUNO SOURCE ROW / GROUP TYPES
// ============================================================================

type FurunoInboundRow struct {
	Row int

	// Source / header information from Furuno Excel.
	DODate             string
	DONumber           string
	PackingListNumber  string
	VesselRemarks      string
	CustomerName       string
	CustomerReference  string
	ModelPartNumber    string
	ItemCode           string
	ModelName          string
	DOQuantity         float64
	VendorSerialNumber string
	FSGSerialNumber    string
	FSGSerialQuantity  float64
	RemarksForDO       string

	// Resolved/import settings.
	InboundDate    string
	Type           string
	Supplier       string
	Transporter    string
	Driver         string
	WhsCode        string
	OwnerCode      string
	Origin         string
	PoDate         string
	NoTruck        string
	Container      string
	TruckSize      string
	ArrivalTime    string
	StartUnloading string
	EndUnloading   string
	BLNo           string
	Koli           int
	Remarks        string

	// Detail values derived from Furuno row + import settings.
	UOM          string
	Location     string
	QaStatus     string
	RecDate      string
	ProdDate     string
	ExpDate      string
	LotNumber    string
	Division     string
	CartonNumber string
	CaseNumber   string
	SerialNumber string
}

type FurunoInboundGroup struct {
	// Grouping key / ReceiptID is the Packing List #.
	PackingListNumber string
	DONumber          string // informational source value only
	HeaderRow         FurunoInboundRow
	Details           []FurunoInboundRow
}

type FurunoImportOptions struct {
	OwnerCode    string
	WhsCode      string
	SupplierCode string
	Origin       string
	Type         string
	UOM          string
	Location     string
	QaStatus     string
	Division     string
	Transporter  string
	Driver       string
	NoTruck      string
	Container    string
	TruckSize    string
	ArrivalTime  string
	StartUnload  string
	EndUnload    string
}

type FurunoMergedDetail struct {
	Representative FurunoInboundRow
	Rows           []FurunoInboundRow
	Quantity       float64
	SerialNumbers  []string
}

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	furunoInboundSheetName         = "MonthlyDOIssuedDetailReportWit"
	furunoInboundMaxHeaderScanRows = 10

	furunoColDONumber          = 0
	furunoColDODate            = 1
	furunoColPackingList       = 2
	furunoColVesselRemarks     = 3
	furunoColCustomerName      = 4
	furunoColCustomerReference = 5
	furunoColModelPartNumber   = 6
	furunoColItemCode          = 7
	furunoColModelName         = 8
	furunoColDOQuantity        = 9
	furunoColVendorSerial      = 10
	furunoColFSGSerial         = 11
	furunoColFSGSerialQuantity = 12
	furunoColRemarksForDO      = 13
)

var furunoInboundValidIBTypes = map[string]bool{
	"RETURN": true,
	"NORMAL": true,
}

var furunoInboundRequiredHeaders = []string{
	"DO Date",
	"Packing List #",
	"Vessel Related Remarks",
	"Customer Name",
	"Cust. Ref No.",
	"Model/Part No",
	"Item code",
	"Model Name",
	"DO Quantity",
	"Vendor Serial Number",
	"FSG Serial Number",
	"FSG Serial Number Quantity",
	"Remarks For DO",
}

// ============================================================================
// GENERIC FURUNO HELPERS
// ============================================================================

func furunoInboundNormalizeHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.ToLower(value)
}

func furunoInboundNormalizeValue(value string) string {
	return strings.TrimSpace(value)
}

func furunoInboundGetCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return furunoInboundNormalizeValue(row[index])
}

func furunoInboundParseFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("value is empty")
	}

	value = strings.ReplaceAll(value, ",", "")

	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value: %s", value)
	}

	return n, nil
}

func furunoInboundExcelSerialToDate(serial float64) string {
	excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	date := excelEpoch.Add(time.Duration(serial * 24 * float64(time.Hour)))
	return date.Format("2006-01-02")
}

func furunoInboundParseDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("date value is empty")
	}

	// Excel serial date.
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 0 {
		return furunoInboundExcelSerialToDate(serial), nil
	}

	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"02-01-2006",
		"01-02-2006",
		"02-01-06",
		"01-02-06",
		"2-Jan-06",
		"02-Jan-06",
		"2-January-06",
		"02-January-06",
		"2-Jan-2006",
		"02-Jan-2006",
		"2-January-2006",
		"02-January-2006",
		"2 Jan 2006",
		"02 Jan 2006",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("invalid date format: %s", value)
}

func furunoInboundValidateTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	formats := []string{
		"15:04",
		"15:04:05",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed.Format("15:04"), nil
		}
	}

	return "", fmt.Errorf("invalid time format (expected HH:mm): %s", value)
}

func furunoInboundJoinNonEmpty(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " | ")
}

func furunoInboundIsEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// ============================================================================
// FURUNO HEADER DETECTION
// ============================================================================

func furunoInboundFindHeaderColumn(headerRow []string, headerName string) int {
	target := furunoInboundNormalizeHeader(headerName)
	for i, cell := range headerRow {
		if furunoInboundNormalizeHeader(cell) == target {
			return i
		}
	}
	return -1
}

func furunoInboundValidateHeaders(headerRow []string) []FurunoValidationError {
	var errs []FurunoValidationError

	for _, required := range furunoInboundRequiredHeaders {
		if furunoInboundFindHeaderColumn(headerRow, required) < 0 {
			errs = append(errs, FurunoValidationError{
				Field:   required,
				Message: "Required Furuno Excel header is missing",
				Row:     1,
			})
		}
	}

	return errs
}

func furunoInboundDetectHeaderRow(rows [][]string) (int, error) {
	if len(rows) == 0 {
		return -1, fmt.Errorf("Excel file contains no rows")
	}

	limit := len(rows)
	if limit > furunoInboundMaxHeaderScanRows {
		limit = furunoInboundMaxHeaderScanRows
	}

	for i := 0; i < limit; i++ {
		headerSet := make(map[string]bool)
		for _, cell := range rows[i] {
			h := furunoInboundNormalizeHeader(cell)
			if h != "" {
				headerSet[h] = true
			}
		}

		matched := 0
		for _, required := range furunoInboundRequiredHeaders {
			if headerSet[furunoInboundNormalizeHeader(required)] {
				matched++
			}
		}

		// We require all Furuno columns because this is a dedicated template.
		if matched == len(furunoInboundRequiredHeaders) {
			return i, nil
		}
	}

	return -1, fmt.Errorf("Furuno inbound template headers were not found in the first %d rows", furunoInboundMaxHeaderScanRows)
}

// ============================================================================
// FURUNO IMPORT OPTIONS
// ============================================================================

func furunoInboundBuildOptions(ctx *fiber.Ctx) (FurunoImportOptions, []FurunoValidationError) {
	options := FurunoImportOptions{
		OwnerCode:    strings.TrimSpace(ctx.FormValue("owner_code")),
		WhsCode:      strings.TrimSpace(ctx.FormValue("whs_code")),
		SupplierCode: strings.TrimSpace(ctx.FormValue("supplier_code")),
		Origin:       strings.TrimSpace(ctx.FormValue("origin")),
		Type:         strings.TrimSpace(ctx.FormValue("type")),
		UOM:          strings.TrimSpace(ctx.FormValue("uom")),
		Location:     strings.TrimSpace(ctx.FormValue("location")),
		QaStatus:     strings.TrimSpace(ctx.FormValue("qa_status")),
		Division:     strings.TrimSpace(ctx.FormValue("division")),
		Transporter:  strings.TrimSpace(ctx.FormValue("transporter")),
		Driver:       strings.TrimSpace(ctx.FormValue("driver")),
		NoTruck:      strings.TrimSpace(ctx.FormValue("no_truck")),
		Container:    strings.TrimSpace(ctx.FormValue("container")),
		TruckSize:    strings.TrimSpace(ctx.FormValue("truck_size")),
		ArrivalTime:  strings.TrimSpace(ctx.FormValue("arrival_time")),
		StartUnload:  strings.TrimSpace(ctx.FormValue("start_unloading")),
		EndUnload:    strings.TrimSpace(ctx.FormValue("end_unloading")),
	}

	if options.Type == "" {
		options.Type = "NORMAL"
	}

	if options.UOM == "" {
		options.UOM = "PCS"
	}

	var errs []FurunoValidationError

	if options.OwnerCode == "" {
		errs = append(errs, FurunoValidationError{
			Field:   "OwnerCode",
			Message: `Form field "owner_code" is required for Furuno import`,
			Row:     0,
		})
	}
	if options.WhsCode == "" {
		errs = append(errs, FurunoValidationError{
			Field:   "WhsCode",
			Message: `Form field "whs_code" is required for Furuno import`,
			Row:     0,
		})
	}
	if options.SupplierCode == "" {
		errs = append(errs, FurunoValidationError{
			Field:   "SupplierCode",
			Message: `Form field "supplier_code" is required for Furuno import`,
			Row:     0,
		})
	}
	if options.Origin == "" {
		errs = append(errs, FurunoValidationError{
			Field:   "Origin",
			Message: `Form field "origin" is required for Furuno import`,
			Row:     0,
		})
	}
	if !furunoInboundValidIBTypes[options.Type] {
		errs = append(errs, FurunoValidationError{
			Field:   "Type",
			Message: fmt.Sprintf("IB Type must be RETURN or NORMAL (case-sensitive), got: %s", options.Type),
			Row:     0,
		})
	}

	if value, err := furunoInboundValidateTime(options.ArrivalTime); err != nil {
		errs = append(errs, FurunoValidationError{Field: "ArrivalTime", Message: err.Error(), Row: 0})
	} else {
		options.ArrivalTime = value
	}
	if value, err := furunoInboundValidateTime(options.StartUnload); err != nil {
		errs = append(errs, FurunoValidationError{Field: "StartUnloading", Message: err.Error(), Row: 0})
	} else {
		options.StartUnload = value
	}
	if value, err := furunoInboundValidateTime(options.EndUnload); err != nil {
		errs = append(errs, FurunoValidationError{Field: "EndUnloading", Message: err.Error(), Row: 0})
	} else {
		options.EndUnload = value
	}

	return options, errs
}

// ============================================================================
// FURUNO ROW PARSER
// ============================================================================

func furunoInboundParseRows(
	rows [][]string,
	headerRowIndex int,
	options FurunoImportOptions,
) ([]FurunoInboundRow, []FurunoValidationError) {
	if headerRowIndex < 0 || headerRowIndex >= len(rows) {
		return nil, []FurunoValidationError{{
			Field:   "Header",
			Message: "Invalid Furuno header row",
			Row:     headerRowIndex + 1,
		}}
	}

	headerRow := rows[headerRowIndex]
	var errs []FurunoValidationError

	if headerErrors := furunoInboundValidateHeaders(headerRow); len(headerErrors) > 0 {
		return nil, headerErrors
	}

	col := make(map[string]int)
	for _, required := range furunoInboundRequiredHeaders {
		col[furunoInboundNormalizeHeader(required)] = furunoInboundFindHeaderColumn(headerRow, required)
	}

	// DO Number is optional/informational. It is never used as ReceiptID.
	col[furunoInboundNormalizeHeader("DO Number")] = furunoInboundFindHeaderColumn(headerRow, "DO Number")

	var parsed []FurunoInboundRow

	for i := headerRowIndex + 1; i < len(rows); i++ {
		row := rows[i]
		excelRow := i + 1

		if furunoInboundIsEmptyRow(row) {
			continue
		}

		addErr := func(field, message string) {
			errs = append(errs, FurunoValidationError{
				Field:   field,
				Message: message,
				Row:     excelRow,
			})
		}

		r := FurunoInboundRow{Row: excelRow}

		// Source fields.
		r.DONumber = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("DO Number")])
		r.PackingListNumber = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Packing List #")])
		r.VesselRemarks = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Vessel Related Remarks")])
		r.CustomerName = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Customer Name")])
		r.CustomerReference = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Cust. Ref No.")])
		r.ModelPartNumber = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Model/Part No")])
		r.ItemCode = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Item code")])
		r.ModelName = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Model Name")])
		r.VendorSerialNumber = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Vendor Serial Number")])
		r.FSGSerialNumber = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("FSG Serial Number")])
		r.RemarksForDO = furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("Remarks For DO")])

		// DO Date.
		doDateRaw := furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("DO Date")])
		if doDateRaw == "" {
			addErr("DO Date", "DO Date cannot be empty")
		} else if parsedDate, err := furunoInboundParseDate(doDateRaw); err != nil {
			addErr("DO Date", err.Error())
		} else {
			r.DODate = parsedDate
		}

		// Quantity.
		qtyRaw := furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("DO Quantity")])
		if qtyRaw == "" {
			addErr("DO Quantity", "DO Quantity cannot be empty")
		} else if qty, err := furunoInboundParseFloat(qtyRaw); err != nil {
			addErr("DO Quantity", err.Error())
		} else if qty <= 0 {
			addErr("DO Quantity", "DO Quantity must be greater than zero")
		} else {
			r.DOQuantity = qty
		}

		// FSG serial quantity.
		fsgQtyRaw := furunoInboundGetCell(row, col[furunoInboundNormalizeHeader("FSG Serial Number Quantity")])
		if fsgQtyRaw != "" {
			if fsgQty, err := furunoInboundParseFloat(fsgQtyRaw); err != nil {
				addErr("FSG Serial Number Quantity", err.Error())
			} else if fsgQty < 0 {
				addErr("FSG Serial Number Quantity", "FSG Serial Number Quantity cannot be negative")
			} else {
				r.FSGSerialQuantity = fsgQty
			}
		}

		// Required source fields.
		// DO Number is informational only and is intentionally NOT required.
		// ReceiptID is derived from Packing List # below.
		if r.PackingListNumber == "" {
			addErr("Packing List #", "Packing List # cannot be empty")
		}
		if r.ItemCode == "" {
			addErr("Item code", "Item code cannot be empty")
		}

		// FSG serial takes priority; vendor serial is a fallback.
		if r.FSGSerialNumber != "" {
			r.SerialNumber = r.FSGSerialNumber
		} else {
			r.SerialNumber = r.VendorSerialNumber
		}

		// Derived WMS header values.
		r.InboundDate = r.DODate
		r.PoDate = r.DODate
		r.RecDate = r.DODate
		r.Type = options.Type
		r.Supplier = options.SupplierCode
		r.Transporter = options.Transporter
		r.Driver = options.Driver
		r.WhsCode = options.WhsCode
		r.OwnerCode = options.OwnerCode
		r.Origin = options.Origin
		r.NoTruck = options.NoTruck
		r.Container = options.Container
		r.TruckSize = options.TruckSize
		r.ArrivalTime = options.ArrivalTime
		r.StartUnloading = options.StartUnload
		r.EndUnloading = options.EndUnload
		r.BLNo = r.PackingListNumber
		r.Koli = 0
		r.UOM = options.UOM
		r.Location = options.Location
		r.QaStatus = options.QaStatus
		r.Division = options.Division

		// Furuno source does not have separate WMS lot/production/expiry fields.
		r.ProdDate = ""
		r.ExpDate = ""
		r.LotNumber = ""
		r.CartonNumber = ""
		// Furuno: Vessel Related Remarks menjadi Case Number pada inbound detail.
		r.CaseNumber = r.VesselRemarks

		// Keep useful source information in Remarks.
		// DO Number is intentionally stored here instead of being used as ReceiptID.
		r.Remarks = furunoInboundJoinNonEmpty(
			func() string {
				if r.DONumber == "" {
					return ""
				}
				return "DO Number: " + r.DONumber
			}(),
			r.VesselRemarks,
			r.CustomerName,
			r.CustomerReference,
			r.ModelPartNumber,
			r.ModelName,
			r.RemarksForDO,
		)

		parsed = append(parsed, r)
	}

	return parsed, errs
}

// ============================================================================
// FURUNO GROUPING / CONSISTENCY
// ============================================================================

func furunoInboundGroupRows(rows []FurunoInboundRow) []FurunoInboundGroup {
	index := make(map[string]int)
	groups := make([]FurunoInboundGroup, 0)

	for _, row := range rows {
		// ReceiptID is Packing List #, so all detail rows belonging to the
		// same Packing List # become one inbound group.
		groupKey := row.PackingListNumber
		idx, exists := index[groupKey]
		if !exists {
			groups = append(groups, FurunoInboundGroup{
				PackingListNumber: row.PackingListNumber,
				DONumber:          row.DONumber, // informational only
				HeaderRow:         row,
				Details:           []FurunoInboundRow{},
			})
			idx = len(groups) - 1
			index[groupKey] = idx
		}

		groups[idx].Details = append(groups[idx].Details, row)
	}

	return groups
}

func furunoInboundHeaderValue(row FurunoInboundRow, field string) string {
	switch field {
	case "DODate":
		return row.DODate
	case "Type":
		return row.Type
	case "Supplier":
		return row.Supplier
	case "Transporter":
		return row.Transporter
	case "Driver":
		return row.Driver
	case "WhsCode":
		return row.WhsCode
	case "OwnerCode":
		return row.OwnerCode
	case "Origin":
		return row.Origin
	case "PoDate":
		return row.PoDate
	case "NoTruck":
		return row.NoTruck
	case "Container":
		return row.Container
	case "TruckSize":
		return row.TruckSize
	case "ArrivalTime":
		return row.ArrivalTime
	case "StartUnloading":
		return row.StartUnloading
	case "EndUnloading":
		return row.EndUnloading
	case "BLNo":
		return row.BLNo
	case "Remarks":
		return row.Remarks
	default:
		return ""
	}
}

func furunoInboundValidateGroupConsistency(group FurunoInboundGroup) []FurunoValidationError {
	fields := []string{
		"DODate",
		"Type",
		"Supplier",
		"Transporter",
		"Driver",
		"WhsCode",
		"OwnerCode", "Origin", "PoDate", "NoTruck",
		"Container", "TruckSize", "ArrivalTime", "StartUnloading",
		"EndUnloading", "BLNo",
		// "Remarks",
	}

	var errs []FurunoValidationError
	ref := group.HeaderRow

	for _, row := range group.Details {
		if row.Row == ref.Row {
			continue
		}

		for _, field := range fields {
			refValue := furunoInboundHeaderValue(ref, field)
			rowValue := furunoInboundHeaderValue(row, field)
			if refValue != rowValue {
				errs = append(errs, FurunoValidationError{
					Field: field,
					Message: fmt.Sprintf(
						"Inconsistent %s within Packing List # %s: row %d has '%s', expected '%s' from row %d",
						field,
						group.PackingListNumber,
						row.Row,
						rowValue,
						refValue,
						ref.Row,
					),
					Row: row.Row,
				})
			}
		}
	}

	return errs
}

func furunoInboundCheckDuplicateSerials(rows []FurunoInboundRow) []FurunoValidationError {
	var errs []FurunoValidationError
	seen := make(map[string]int)

	for _, row := range rows {
		if row.SerialNumber == "" {
			continue
		}

		key := row.SerialNumber + "|" + row.PackingListNumber
		if existingRow, exists := seen[key]; exists {
			errs = append(errs, FurunoValidationError{
				Field: "SerialNumber",
				Message: fmt.Sprintf(
					"Duplicate serial number '%s' within Packing List # %s (same as row %d)",
					row.SerialNumber,
					row.PackingListNumber,
					existingRow,
				),
				Row: row.Row,
			})
		} else {
			seen[key] = row.Row
		}
	}

	return errs
}

func furunoInboundDetailGroupKey(row FurunoInboundRow) string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%s",
		row.ItemCode,
		row.UOM,
		row.Location,
		row.QaStatus,
		row.ProdDate,
		row.ExpDate,
		row.LotNumber,
		row.CartonNumber,
		row.CaseNumber,
	)
}

func furunoInboundMergeDetails(rows []FurunoInboundRow) []FurunoMergedDetail {
	index := make(map[string]int)
	merged := make([]FurunoMergedDetail, 0)

	for _, row := range rows {
		key := furunoInboundDetailGroupKey(row)

		idx, exists := index[key]
		if !exists {
			merged = append(merged, FurunoMergedDetail{
				Representative: row,
				Rows:           []FurunoInboundRow{row},
				Quantity:       row.DOQuantity,
			})
			idx = len(merged) - 1
			index[key] = idx
		} else {
			merged[idx].Quantity += row.DOQuantity
			merged[idx].Rows = append(merged[idx].Rows, row)
		}
	}

	for i := range merged {
		for _, row := range merged[i].Rows {
			if row.SerialNumber != "" {
				merged[i].SerialNumbers = append(merged[i].SerialNumbers, row.SerialNumber)
			}
		}
	}

	return merged
}

// ============================================================================
// FURUNO CONTROLLER
// ============================================================================

func (c *InboundController) CreateInboundFromFurunoExcelFile(ctx *fiber.Ctx) error {
	// --------------------------------------------------------------------------
	// Upload / extension
	// --------------------------------------------------------------------------
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "File Error", Detail: err.Error()},
			},
		})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx files are allowed for Furuno inbound import",
		})
	}

	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to open uploaded Furuno file",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "File Processing Error", Detail: err.Error()},
			},
		})
	}
	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to read Furuno Excel file. Please ensure the file is a valid .xlsx workbook",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "Excel Read Error", Detail: err.Error()},
			},
		})
	}
	defer excelFile.Close()

	// --------------------------------------------------------------------------
	// Sheet
	// --------------------------------------------------------------------------
	sheetName := furunoInboundSheetName
	sheetList := excelFile.GetSheetList()

	if len(sheetList) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Furuno Excel file contains no worksheets",
		})
	}

	// Allow the configured sheet first; if the vendor changes the workbook
	// sheet suffix, fallback to the first sheet instead of hard failing.
	foundSheet := false
	for _, name := range sheetList {
		if name == sheetName {
			foundSheet = true
			break
		}
	}
	if !foundSheet {
		sheetName = sheetList[0]
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to read rows from Furuno Excel sheet",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "Sheet Read Error", Detail: err.Error()},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Furuno Excel file must contain a header row and at least one data row",
		})
	}

	// --------------------------------------------------------------------------
	// Import options
	// --------------------------------------------------------------------------
	options, optionErrors := furunoInboundBuildOptions(ctx)
	if len(optionErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Furuno import configuration validation failed with %d error(s)", len(optionErrors)),
			ValidationErrors: optionErrors,
			TotalRows:        0,
		})
	}

	// --------------------------------------------------------------------------
	// Find / validate Furuno header row
	// --------------------------------------------------------------------------
	headerIndex, err := furunoInboundDetectHeaderRow(rows)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Invalid Furuno inbound template",
			Errors: []FurunoExcelRowError{
				{Row: 1, Message: "Template Error", Detail: err.Error()},
			},
			TotalRows: len(rows),
		})
	}

	// --------------------------------------------------------------------------
	// Parse rows
	// --------------------------------------------------------------------------
	parsedRows, parseErrors := furunoInboundParseRows(rows, headerIndex, options)
	dataRowCount := len(rows) - headerIndex - 1
	if dataRowCount < 0 {
		dataRowCount = 0
	}

	if len(parseErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Furuno validation failed with %d error(s)", len(parseErrors)),
			TotalRows:        dataRowCount,
			ValidationErrors: parseErrors,
		})
	}

	if len(parsedRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:   false,
			Message:   "No valid data rows found in Furuno Excel file",
			TotalRows: dataRowCount,
		})
	}

	// --------------------------------------------------------------------------
	// Group by Packing List # (used as ReceiptID)
	// --------------------------------------------------------------------------
	groups := furunoInboundGroupRows(parsedRows)

	// --------------------------------------------------------------------------
	// Header consistency
	// --------------------------------------------------------------------------
	var consistencyErrors []FurunoValidationError
	for _, group := range groups {
		consistencyErrors = append(consistencyErrors, furunoInboundValidateGroupConsistency(group)...)
	}
	if len(consistencyErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Furuno DO consistency validation failed with %d error(s)", len(consistencyErrors)),
			TotalRows:        dataRowCount,
			ValidationErrors: consistencyErrors,
		})
	}

	// --------------------------------------------------------------------------
	// Duplicate serial check within file (scoped by Packing List #)
	// --------------------------------------------------------------------------
	duplicateSerialErrors := furunoInboundCheckDuplicateSerials(parsedRows)
	if len(duplicateSerialErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:          false,
			Message:          "Duplicate Furuno serial numbers found in Excel file",
			TotalRows:        dataRowCount,
			ValidationErrors: duplicateSerialErrors,
		})
	}

	// --------------------------------------------------------------------------
	// User ID
	// --------------------------------------------------------------------------
	userID := 0
	if raw := ctx.Locals("userID"); raw != nil {
		if parsed, ok := raw.(float64); ok {
			userID = int(parsed)
		} else if parsed, ok := raw.(int); ok {
			userID = parsed
		}
	}

	// --------------------------------------------------------------------------
	// Inventory policy
	// --------------------------------------------------------------------------
	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", options.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy for owner: " + options.OwnerCode,
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "Inventory Policy Error", Detail: err.Error()},
			},
		})
	}

	// --------------------------------------------------------------------------
	// Existing serial check in DB
	// --------------------------------------------------------------------------
	var serialConflictErrors []FurunoValidationError
	for _, row := range parsedRows {
		if row.SerialNumber == "" {
			continue
		}

		var existing models.InboundSerial
		err := c.DB.Where("serial_number = ?", row.SerialNumber).First(&existing).Error
		if err == nil {
			serialConflictErrors = append(serialConflictErrors, FurunoValidationError{
				Field:   "SerialNumber",
				Message: fmt.Sprintf("Serial number '%s' already registered in system", row.SerialNumber),
				Row:     row.Row,
			})
		}
	}

	if len(serialConflictErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
			Success:          false,
			Message:          "Furuno serial numbers already exist in system",
			TotalRows:        dataRowCount,
			ValidationErrors: serialConflictErrors,
		})
	}

	// Keep policy variable intentionally used so this dedicated importer follows
	// the same owner-level policy lookup as the default inbound importer.
	_ = inventoryPolicy

	// --------------------------------------------------------------------------
	// Master data validation (fail-fast before transaction)
	// --------------------------------------------------------------------------
	for _, group := range groups {
		h := group.HeaderRow

		var warehouse models.Warehouse
		if err := c.DB.Where("code = ?", h.WhsCode).First(&warehouse).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: "Failed to get warehouse: " + h.WhsCode,
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Warehouse Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := c.DB.Where("supplier_code = ?", h.Supplier).First(&supplier).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: "Failed to get supplier: " + h.Supplier,
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Supplier Error", Detail: err.Error()},
				},
			})
		}

		var origin models.Origin
		if err := c.DB.Where("country = ?", h.Origin).First(&origin).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: "Failed to get origin: " + h.Origin,
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Origin Error", Detail: err.Error()},
				},
			})
		}

		if h.Transporter != "" {
			var transporter models.Transporter
			if err := c.DB.Where("transporter_code = ?", h.Transporter).First(&transporter).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
					Success: false,
					Message: "Failed to get transporter: " + h.Transporter,
					Errors: []FurunoExcelRowError{
						{Row: h.Row, Message: "Transporter Error", Detail: err.Error()},
					},
				})
			}
		}

		var existingCount int64
		if err := c.DB.Model(&models.InboundHeader{}).
			Where("receipt_id = ?", h.PackingListNumber).
			Count(&existingCount).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: "Failed to check existing Packing List #: " + h.PackingListNumber,
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}

		if existingCount > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(FurunoExcelUploadResponse{
				Success:   false,
				Message:   "Packing List # already exists: " + h.PackingListNumber,
				TotalRows: dataRowCount,
				ValidationErrors: []FurunoValidationError{
					{Field: "Packing List #", Message: "Packing List # already exists in database: " + h.PackingListNumber, Row: h.Row},
				},
			})
		}
	}

	// --------------------------------------------------------------------------
	// Transaction: all-or-nothing
	// --------------------------------------------------------------------------
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to start database transaction",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "Transaction Error", Detail: tx.Error.Error()},
			},
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateInboundFromFurunoExcelFile: %v", r)
		}
	}()

	repo := repositories.NewInboundRepository(tx)

	createdInbounds := make([]string, 0, len(groups))
	successCount := 0

	for _, group := range groups {
		h := group.HeaderRow

		inboundNo, err := repo.GenerateInboundNo()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate inbound number for Packing List # %s", h.PackingListNumber),
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Inbound Generation Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := tx.First(&supplier, "supplier_code = ?", h.Supplier).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to validate supplier for Packing List # %s", h.PackingListNumber),
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}

		inboundHeader := models.InboundHeader{
			InboundNo:      inboundNo,
			InboundDate:    h.InboundDate,
			ReceiptID:      h.PackingListNumber,
			Supplier:       h.Supplier,
			SupplierId:     int(supplier.ID),
			Status:         "open",
			RawStatus:      "DRAFT",
			DraftTime:      time.Now(),
			Transporter:    h.Transporter,
			NoTruck:        h.NoTruck,
			Driver:         h.Driver,
			Container:      h.Container,
			Remarks:        h.Remarks,
			Type:           h.Type,
			WhsCode:        h.WhsCode,
			OwnerCode:      h.OwnerCode,
			Origin:         h.Origin,
			PoDate:         h.PoDate,
			ArrivalTime:    h.ArrivalTime,
			StartUnloading: h.StartUnloading,
			EndUnloading:   h.EndUnloading,
			TruckSize:      h.TruckSize,
			BLNo:           h.BLNo,
			Koli:           h.Koli,
			CreatedBy:      userID,
			UpdatedBy:      userID,
		}

		if err := tx.Create(&inboundHeader).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound header for Packing List # %s", h.PackingListNumber),
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		inboundReference := models.InboundReference{
			InboundId: uint(inboundHeader.ID),
			RefNo:     h.PackingListNumber,
		}

		if err := tx.Create(&inboundReference).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound reference for Packing List # %s", h.PackingListNumber),
				Errors: []FurunoExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		mergedDetails := furunoInboundMergeDetails(group.Details)

		for _, md := range mergedDetails {
			detail := md.Representative

			var product models.Product
			if err := tx.First(&product, "item_code = ? AND owner_code = ?", detail.ItemCode, h.OwnerCode).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(FurunoExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Product not found for item code: %s (Packing List # %s)", detail.ItemCode, h.PackingListNumber),
					Errors: []FurunoExcelRowError{
						{Row: detail.Row, Message: "Product Not Found", Detail: "Item code: " + detail.ItemCode},
					},
				})
			}

			var uomConversion models.UomConversion
			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(FurunoExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("UOM conversion not found (Packing List # %s)", h.PackingListNumber),
					Errors: []FurunoExcelRowError{
						{
							Row:     detail.Row,
							Message: "UOM Not Found",
							Detail:  fmt.Sprintf("Item: %s, UOM: %s", detail.ItemCode, detail.UOM),
						},
					},
				})
			}

			if detail.QaStatus != "" {
				var qaStatus models.QaStatus
				if err := tx.First(&qaStatus, "qa_status = ?", detail.QaStatus).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(FurunoExcelUploadResponse{
						Success: false,
						Message: fmt.Sprintf("QA status not found (Packing List # %s)", h.PackingListNumber),
						Errors: []FurunoExcelRowError{
							{Row: detail.Row, Message: "QA Status Not Found", Detail: "Status: " + detail.QaStatus},
						},
					})
				}
			}

			inboundDetail := models.InboundDetail{
				InboundNo:     inboundNo,
				InboundId:     int(inboundHeader.ID),
				ItemCode:      detail.ItemCode,
				ItemId:        product.ID,
				ProductNumber: product.ProductNumber,
				Barcode:       uomConversion.Ean,
				Uom:           detail.UOM,
				Quantity:      md.Quantity,
				RcvLocation:   detail.Location,
				Location:      detail.Location,
				QaStatus:      detail.QaStatus,
				RecDate:       detail.RecDate,
				ProdDate:      detail.ProdDate,
				ExpDate:       detail.ExpDate,
				LotNumber:     detail.LotNumber,
				CartonNumber:  detail.CartonNumber,
				CaseNumber:    detail.CaseNumber,
				SerialNumber:  "",
				IsSerial:      product.HasSerial,
				SN:            product.HasSerial,
				RefId:         int(inboundReference.ID),
				RefNo:         h.PackingListNumber,
				OwnerCode:     h.OwnerCode,
				WhsCode:       h.WhsCode,
				DivisionCode:  detail.Division,
				CreatedBy:     userID,
				UpdatedBy:     userID,
			}

			if err := tx.Create(&inboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to create inbound detail (Packing List # %s)", h.PackingListNumber),
					Errors: []FurunoExcelRowError{
						{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
					},
				})
			}

			// Intentionally store Furuno serials independently of Product.HasSerial.
			// The source file explicitly provides serial information.
			for _, serial := range md.SerialNumbers {
				if serial == "" {
					continue
				}

				inboundSerial := models.InboundSerial{
					InboundId:       int(inboundHeader.ID),
					InboundDetailId: int(inboundDetail.ID),
					SerialNumber:    serial,
					CreatedBy:       userID,
					UpdatedBy:       userID,
				}

				if err := tx.Create(&inboundSerial).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
						Success: false,
						Message: fmt.Sprintf("Failed to create inbound serial (Packing List # %s, SN: %s)", h.PackingListNumber, serial),
						Errors: []FurunoExcelRowError{
							{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
						},
					})
				}
			}

			successCount += len(md.Rows)
		}

		if err := helpers.InsertTransactionHistory(tx, inboundNo, "open", "INBOUND", "Created from Furuno Excel upload", userID); err != nil {
			log.Printf("Warning: Failed to insert Furuno transaction history for %s: %v", inboundNo, err)
		}

		createdInbounds = append(createdInbounds, inboundNo)
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoExcelUploadResponse{
			Success: false,
			Message: "Failed to commit Furuno inbound transaction",
			Errors: []FurunoExcelRowError{
				{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()},
			},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(FurunoExcelUploadResponse{
		Success:        true,
		Message:        fmt.Sprintf("Successfully created %d Furuno inbound(s) with %d items", len(createdInbounds), successCount),
		TotalRows:      dataRowCount,
		ProcessedRows:  len(parsedRows),
		SkippedRows:    dataRowCount - len(parsedRows),
		SuccessCount:   successCount,
		FailedCount:    0,
		InboundNumbers: createdInbounds,
	})
}

// ============================================================================
// FURUNO INTEGRATION ENTRY POINT
// ============================================================================

func (c *InboundController) CreateInboundFromFurunoExcelIntegration(ctx *fiber.Ctx) error {
	ctx.Locals("userID", float64(0))
	return c.CreateInboundFromFurunoExcelFile(ctx)
}
