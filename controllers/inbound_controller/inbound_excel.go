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

// =======================================
// STRUCTS - REVISED
// =======================================

type ExcelUploadResponse struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	TotalRows        int               `json:"total_rows"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	InboundNumbers   []string          `json:"inbound_numbers,omitempty"`
	Errors           []ExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

type ExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Row     int    `json:"row"`
}

// ExcelInboundRow merepresentasikan SATU baris Excel secara penuh:
// gabungan field header (yang akan sama untuk semua baris dalam 1 ReceiptID)
// dan field detail (yang unik per baris).
//
// Sebelumnya header dan detail dipisah menjadi dua struct berbeda
// (ExcelInboundHeader & ExcelInboundDetail), dengan header hanya dibaca
// sekali dari baris pertama. Sekarang setiap baris membawa kedua informasi
// ini sekaligus, supaya grouping per ReceiptID bisa dilakukan dengan benar.
type ExcelInboundRow struct {
	Row int // nomor baris Excel (untuk pesan error)

	// ---- Header fields (kolom 0-18) ----
	ReceiptID      string
	InboundDate    string
	Type           string // IB Type: RETURN / NORMAL
	Supplier       string
	Transporter    string // Trucker
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

	// ---- Detail fields (kolom 19-28) ----
	ItemCode     string
	UOM          string
	Quantity     float64
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

// InboundGroup adalah hasil grouping baris-baris Excel berdasarkan ReceiptID.
// HeaderRow menyimpan informasi header (sudah divalidasi konsisten),
// Details menyimpan semua baris detail dalam grup tersebut.
type InboundGroup struct {
	ReceiptID string
	HeaderRow ExcelInboundRow   // representasi header grup (diambil dari baris pertama, sudah divalidasi konsisten)
	Details   []ExcelInboundRow // semua baris (dipakai juga sebagai detail)
}

// Kolom-kolom header yang WAJIB konsisten (sama) di semua baris
// dalam satu ReceiptID yang sama.
var headerConsistencyFields = []string{
	"InboundDate", "Type", "Supplier", "Transporter", "Driver",
	"WhsCode", "OwnerCode", "Origin", "PoDate", "NoTruck",
	"Container", "TruckSize", "ArrivalTime", "StartUnloading",
	"EndUnloading", "BLNo", "Remarks",
	// Koli tidak dimasukkan di sini karena perlu perbandingan pointer khusus,
	// ditangani terpisah di validateHeaderConsistency.
}

// validIBTypes adalah daftar nilai valid untuk kolom IB Type.
// Case-sensitive, harus persis sama.
var validIBTypes = map[string]bool{
	"RETURN": true,
	"NORMAL": true,
}

// =======================================
// CELL READING HELPERS
// =======================================

func getCell(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

// Kolom-kolom di template Excel (header dulu, baru detail).
// Disusun sebagai konstanta supaya gampang dirawat kalau urutan kolom berubah lagi.
const (
	colReceiptID      = 0
	colInboundDate    = 1
	colType           = 2 // IB Type
	colSupplier       = 3
	colTransporter    = 4 // Trucker
	colDriver         = 5
	colWhsCode        = 6
	colOwnerCode      = 7
	colOrigin         = 8
	colPoDate         = 9
	colNoTruck        = 10
	colContainer      = 11
	colTruckSize      = 12
	colArrivalTime    = 13
	colStartUnloading = 14
	colEndUnloading   = 15
	colBLNo           = 16
	colKoli           = 17
	colRemarks        = 18

	colItemCode     = 19
	colUOM          = 20
	colQuantity     = 21
	colLocation     = 22
	colQaStatus     = 23
	colRecDate      = 24
	colProdDate     = 25
	colExpDate      = 26
	colLotNumber    = 27
	colDivision     = 28
	colCartonNumber = 29
	colCaseNumber   = 30
	colSerialNumber = 31
)

// getCellAsDateStrict mem-parsing cell sebagai tanggal, mendukung Excel
// serial date maupun beberapa format string umum. Mengembalikan format
// kanonik "2006-01-02". Dipakai untuk InboundDate, PoDate, RecDate, dst.
func getCellAsDateStrict(row []string, index int) (string, error) {
	cellValue := strings.TrimSpace(getCell(row, index))
	if cellValue == "" {
		return "", fmt.Errorf("date value is empty")
	}
	return parseDateString(cellValue)
}

func parseDateString(cellValue string) (string, error) {
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

// validateTimeStrict memvalidasi format jam HH:mm (24 jam), contoh "10:30", "15:54".
// String kosong dianggap valid (karena field jam ini opsional) dan akan
// mengembalikan string kosong tanpa error.
func validateTimeStrict(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	t, err := time.Parse("15:04", value)
	if err != nil {
		return "", fmt.Errorf("invalid time format (expected HH:mm): %s", value)
	}
	// Normalisasi ke format HH:mm dua digit (misal "9:5" -> "09:05")
	return t.Format("15:04"), nil
}

// parseKoli mem-parsing kolom Koli yang opsional menjadi *int.
// String kosong menghasilkan nil (bukan 0), supaya bisa disimpan sebagai
// NULL di database dan tidak ambigu dengan "memang nol koli".
func parseKoli(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid koli value (must be numeric): %s", value)
	}
	return n, nil
}

// =======================================
// PARSE ALL ROWS (header + detail digabung per baris)
// =======================================

// parseAllRows membaca SEMUA baris data (mulai row Excel ke-2) sebagai
// ExcelInboundRow lengkap (header + detail). Ini menggantikan kombinasi
// parseHeaderFromExcel (yang lama, hanya baca row pertama) dan
// parseDetailsFromExcel (yang lama, tidak membaca kolom header).
//
// policy dipakai untuk validasi conditional yang sudah ada sebelumnya
// (UseLotNo, UseProductionDate, UseReceiveLocation, UseFEFO).
func (c *InboundController) parseAllRows(rows [][]string, policy models.InventoryPolicy) ([]ExcelInboundRow, []ValidationError) {
	var parsedRows []ExcelInboundRow
	var errs []ValidationError

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1 // nomor baris Excel (1-indexed, +1 karena row Excel dimulai dari 1 dan ada header)

		// Lewati baris yang benar-benar kosong (semua kolom blank)
		if isRowEmpty(row) {
			continue
		}

		r := ExcelInboundRow{Row: rowNum}
		rowErrs := []ValidationError{}

		addErr := func(field, message string) {
			rowErrs = append(rowErrs, ValidationError{Field: field, Message: message, Row: rowNum})
		}

		// ---------- HEADER FIELDS ----------

		r.ReceiptID = getCell(row, colReceiptID)
		if r.ReceiptID == "" {
			addErr("ReceiptID", "Receipt ID cannot be empty")
		}

		inboundDate, err := getCellAsDateStrict(row, colInboundDate)
		if err != nil {
			addErr("InboundDate", "Invalid or empty Inbound Date: "+err.Error())
		}
		r.InboundDate = inboundDate

		r.Type = getCell(row, colType)
		if r.Type == "" {
			addErr("Type", "IB Type cannot be empty")
		} else if !validIBTypes[r.Type] {
			addErr("Type", fmt.Sprintf("IB Type must be RETURN or NORMAL (case-sensitive), got: %s", r.Type))
		}

		r.Supplier = getCell(row, colSupplier)
		if r.Supplier == "" {
			addErr("Supplier", "Supplier cannot be empty")
		}

		r.Transporter = getCell(row, colTransporter)
		if r.Transporter == "" {
			addErr("Transporter", "Trucker cannot be empty")
		}

		r.Driver = getCell(row, colDriver) // opsional

		r.WhsCode = getCell(row, colWhsCode)
		if r.WhsCode == "" {
			addErr("WhsCode", "Warehouse Code cannot be empty")
		}

		r.OwnerCode = getCell(row, colOwnerCode)
		if r.OwnerCode == "" {
			addErr("OwnerCode", "Owner Code cannot be empty")
		}

		r.Origin = getCell(row, colOrigin)
		if r.Origin == "" {
			addErr("Origin", "Origin cannot be empty")
		}

		poDate, err := getCellAsDateStrict(row, colPoDate)
		if err != nil {
			addErr("PoDate", "Invalid or empty PO Date: "+err.Error())
		}
		r.PoDate = poDate

		r.NoTruck = getCell(row, colNoTruck)     // opsional
		r.Container = getCell(row, colContainer) // opsional
		r.TruckSize = getCell(row, colTruckSize) // opsional

		arrivalTime, err := validateTimeStrict(getCell(row, colArrivalTime))
		if err != nil {
			addErr("ArrivalTime", err.Error())
		}
		r.ArrivalTime = arrivalTime

		startUnloading, err := validateTimeStrict(getCell(row, colStartUnloading))
		if err != nil {
			addErr("StartUnloading", err.Error())
		}
		r.StartUnloading = startUnloading

		endUnloading, err := validateTimeStrict(getCell(row, colEndUnloading))
		if err != nil {
			addErr("EndUnloading", err.Error())
		}
		r.EndUnloading = endUnloading

		r.BLNo = getCell(row, colBLNo) // opsional

		koli, err := parseKoli(getCell(row, colKoli))
		if err != nil {
			addErr("Koli", err.Error())
		}
		r.Koli = koli

		r.Remarks = getCell(row, colRemarks) // opsional

		// ---------- DETAIL FIELDS ----------

		r.ItemCode = getCell(row, colItemCode)
		if r.ItemCode == "" {
			addErr("ItemCode", "Item code cannot be empty")
		}

		r.UOM = getCell(row, colUOM)
		if r.UOM == "" {
			addErr("UOM", "UOM cannot be empty")
		}

		qtyStr := getCell(row, colQuantity)
		if qtyStr == "" {
			addErr("Quantity", "Quantity cannot be empty")
		} else {
			qty, err := strconv.ParseFloat(qtyStr, 64)
			if err != nil {
				addErr("Quantity", "Invalid quantity format: "+qtyStr)
			} else if qty == 0 {
				addErr("Quantity", "Quantity cannot be zero")
			} else {
				r.Quantity = qty
			}
		}

		r.Location = getCell(row, colLocation)
		r.QaStatus = getCell(row, colQaStatus)

		recDate, err := getCellAsDateStrict(row, colRecDate)
		if err != nil {
			addErr("RecDate", "Invalid RecDate format: "+err.Error())
		}
		r.RecDate = recDate

		prodDate, err := getCellAsDateStrict(row, colProdDate)
		if err != nil {
			if policy.UseProductionDate {
				addErr("ProdDate", "Production date is required by inventory policy: "+err.Error())
			}
			// kalau tidak required oleh policy, biarkan kosong tanpa error
		}
		r.ProdDate = prodDate

		expDate, err := getCellAsDateStrict(row, colExpDate)
		if err != nil {
			if policy.UseFEFO {
				addErr("ExpDate", "Expiration date is required by inventory policy: "+err.Error())
			}
		}
		r.ExpDate = expDate

		r.LotNumber = getCell(row, colLotNumber)
		if policy.UseLotNo && r.LotNumber == "" {
			addErr("LotNumber", "Lot number is required by inventory policy")
		}

		if policy.UseReceiveLocation && r.Location == "" {
			addErr("Location", "Receive location is required by inventory policy")
		}

		r.Division = getCell(row, colDivision)

		r.CartonNumber = getCell(row, colCartonNumber)
		// if policy.UseCartonNumber && r.CartonNumber == "" {
		// 	addErr("CartonNumber", "Carton number is required by inventory policy")
		// }

		r.CaseNumber = getCell(row, colCaseNumber)
		// if policy.UseCaseNumber && r.CaseNumber == "" {
		// 	addErr("CaseNumber", "Case number is required by inventory policy")
		// }

		r.SerialNumber = getCell(row, colSerialNumber)
		// if policy.UseSerialNumber && r.SerialNumber == "" {
		// 	addErr("SerialNumber", "Serial number is required by inventory policy")
		// }

		// Validasi item & UOM ke database (sama seperti logic lama)
		if r.ItemCode != "" {
			var product models.Product
			if err := c.DB.First(&product, "item_code = ? AND owner_code = ?", r.ItemCode, r.OwnerCode).Error; err != nil {
				addErr("ItemCode", "Product not found for item code: "+r.ItemCode)
			} else if r.UOM != "" {
				var uomConversion models.UomConversion
				if err := c.DB.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, r.UOM).Error; err != nil {
					addErr("UOM", "UOM Conversion not found for item code: "+r.ItemCode)
				}
			}
		}

		if len(rowErrs) > 0 {
			errs = append(errs, rowErrs...)
			continue // baris ini tidak diikutkan ke parsedRows karena ada error
		}

		parsedRows = append(parsedRows, r)
	}

	return parsedRows, errs
}

func isRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// =======================================
// GROUPING & CONSISTENCY VALIDATION
// =======================================

// groupRowsByReceiptID mengelompokkan baris-baris berdasarkan ReceiptID,
// dengan urutan grup yang STABIL sesuai kemunculan pertama ReceiptID
// tersebut di file Excel (bukan urutan acak seperti map biasa di Go).
func groupRowsByReceiptID(rows []ExcelInboundRow) []InboundGroup {
	groupIndex := make(map[string]int) // ReceiptID -> index di slice groups
	var groups []InboundGroup

	for _, row := range rows {
		idx, exists := groupIndex[row.ReceiptID]
		if !exists {
			groups = append(groups, InboundGroup{
				ReceiptID: row.ReceiptID,
				HeaderRow: row, // baris pertama jadi representasi header
			})
			idx = len(groups) - 1
			groupIndex[row.ReceiptID] = idx
		}
		groups[idx].Details = append(groups[idx].Details, row)
	}

	return groups
}

// validateHeaderConsistency memastikan semua baris dalam satu grup
// (ReceiptID yang sama) punya nilai field header yang SAMA. Kalau ada
// baris yang berbeda, dianggap kesalahan input dan harus direject dengan
// pesan yang jelas (field apa, row mana, beda dengan apa).
func validateGroupHeaderConsistency(group InboundGroup) []ValidationError {
	var errs []ValidationError
	ref := group.HeaderRow

	for _, row := range group.Details {
		if row.Row == ref.Row {
			continue // baris referensi sendiri, skip
		}

		for _, field := range headerConsistencyFields {
			refVal := getFieldValue(ref, field)
			rowVal := getFieldValue(row, field)
			if refVal != rowVal {
				errs = append(errs, ValidationError{
					Field: field,
					Row:   row.Row,
					Message: fmt.Sprintf(
						"Inconsistent %s within Receipt ID %s: row %d has '%s', expected '%s' (from row %d)",
						field, group.ReceiptID, row.Row, rowVal, refVal, ref.Row,
					),
				})
			}
		}

		// Koli ditangani khusus karena *int, bukan string
		if !koliEqual(ref.Koli, row.Koli) {
			errs = append(errs, ValidationError{
				Field: "Koli",
				Row:   row.Row,
				Message: fmt.Sprintf(
					"Inconsistent Koli within Receipt ID %s: row %d has '%s', expected '%s' (from row %d)",
					group.ReceiptID, row.Row, koliString(row.Koli), koliString(ref.Koli), ref.Row,
				),
			})
		}
	}

	return errs
}

// getFieldValue mengambil nilai field header (bertipe string) dari
// ExcelInboundRow berdasarkan nama field, dipakai untuk perbandingan
// konsistensi secara generic tanpa menulis if-else berulang untuk
// setiap field satu per satu.
func getFieldValue(row ExcelInboundRow, field string) string {
	switch field {
	case "InboundDate":
		return row.InboundDate
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

func koliEqual(a, b int) bool {
	if a == 0 && b == 0 {
		return true
	}
	if a == 0 || b == 0 {
		return false
	}
	return a == b
}

func koliString(v int) string {
	if v == 0 {
		return "(empty)"
	}
	return strconv.Itoa(v)
}

// =======================================
// DUPLICATE CHECK (sama seperti logic lama, disesuaikan ke struct baru)
// =======================================

func checkDuplicateItems(rows []ExcelInboundRow) []ValidationError {
	var errs []ValidationError
	itemMap := make(map[string]int)

	for _, row := range rows {
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
			row.ReceiptID, row.ItemCode, row.RecDate, row.ExpDate,
			row.LotNumber, row.ProdDate, row.Location,
			row.UOM, row.QaStatus, row.CartonNumber, row.CaseNumber, row.SerialNumber)

		if existingRow, exists := itemMap[key]; exists {
			errs = append(errs, ValidationError{
				Field: "Duplicate",
				Message: fmt.Sprintf(
					"Duplicate item found (same as row %d) within Receipt ID %s: %s / %s / %s / %s / %s / %s / %s / %s / %s / %s / %s",
					existingRow, row.ReceiptID, row.ItemCode, row.RecDate, row.ExpDate, row.LotNumber, row.ProdDate, row.Location, row.UOM, row.QaStatus, row.CartonNumber, row.CaseNumber, row.SerialNumber,
				),
				Row: row.Row,
			})
		} else {
			itemMap[key] = row.Row
		}
	}

	return errs
}

// =======================================
// MAIN CONTROLLER (REVISED)
// =======================================

func (c *InboundController) CreateInboundFromExcelFile(ctx *fiber.Ctx) error {
	// ---------- Parse uploaded file ----------
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Error", Detail: err.Error()},
			},
		})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Processing Error", Detail: err.Error()},
			},
		})
	}
	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Excel Read Error", Detail: err.Error()},
			},
		})
	}
	defer excelFile.Close()

	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Excel file contains no sheets",
		})
	}

	sheetName := sheets[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Sheet Read Error", Detail: err.Error()},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Excel file must contain at least header row and one data row",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	// ---------- Validasi inventory policy ----------
	// NOTE: policy diambil berdasarkan OwnerCode di baris pertama data,
	// dengan asumsi satu file = satu Owner Code. Kalau Owner Code juga
	// bisa berbeda antar grup dalam satu file, ini perlu diperluas
	// menjadi pencarian per-grup. Untuk saat ini OwnerCode termasuk
	// field yang divalidasi konsisten di seluruh file lewat
	// validateGroupHeaderConsistency, jadi aman selama semua baris
	// dalam SATU ReceiptID owner-nya sama. Jika owner BERBEDA antar
	// ReceiptID yang berbeda, policy lookup di bawah perlu disesuaikan.
	firstOwnerCode := strings.TrimSpace(getCell(rows[1], colOwnerCode))
	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", firstOwnerCode).First(&inventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy for owner: " + firstOwnerCode,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Inventory Policy Error", Detail: err.Error()},
			},
		})
	}

	// ---------- Parse semua baris (header + detail digabung) ----------
	parsedRows, parseErrors := c.parseAllRows(rows, inventoryPolicy)
	if len(parseErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d errors", len(parseErrors)),
			ValidationErrors: parseErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	if len(parsedRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "No valid data rows found in Excel file",
		})
	}

	// ---------- Group by ReceiptID ----------
	groups := groupRowsByReceiptID(parsedRows)

	// ---------- Validasi konsistensi header per grup ----------
	var consistencyErrors []ValidationError
	for _, group := range groups {
		consistencyErrors = append(consistencyErrors, validateGroupHeaderConsistency(group)...)
	}
	if len(consistencyErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Header consistency validation failed with %d errors", len(consistencyErrors)),
			ValidationErrors: consistencyErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// ---------- Validasi duplikat item (dalam masing-masing ReceiptID) ----------

	duplicateErrors := checkDuplicateItems(parsedRows)
	if len(duplicateErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          "Duplicate items found in Excel file",
			ValidationErrors: duplicateErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// ---------- Validasi referensi master data per grup (Warehouse, Supplier, Origin) ----------
	// Dilakukan sebelum transaction dimulai supaya fail-fast.
	for _, group := range groups {
		h := group.HeaderRow

		var warehouse models.Warehouse
		if err := c.DB.Where("code = ?", h.WhsCode).First(&warehouse).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get warehouse: " + h.WhsCode,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Warehouse Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := c.DB.Where("supplier_code = ?", h.Supplier).First(&supplier).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get supplier: " + h.Supplier,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Supplier Error", Detail: err.Error()},
				},
			})
		}

		var origin models.Origin
		if err := c.DB.Where("country = ?", h.Origin).First(&origin).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get origin: " + h.Origin,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Origin Error", Detail: err.Error()},
				},
			})
		}

		var transporter models.Transporter
		if err := c.DB.Where("transporter_code = ?", h.Transporter).First(&transporter).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get transporter: " + h.Transporter,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Transporter Error", Detail: err.Error()},
				},
			})
		}

		// Cek existing ReceiptID di DB sebelum transaction dimulai (fail-fast,
		// menghindari rollback besar di tengah jalan hanya karena receipt
		// yang sama pernah di-upload sebelumnya).
		var existingCount int64
		if err := c.DB.Model(&models.InboundHeader{}).
			Where("receipt_id = ?", h.ReceiptID).
			Count(&existingCount).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to check existing Receipt ID: " + h.ReceiptID,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}
		if existingCount > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Receipt ID already exists: " + h.ReceiptID,
				ValidationErrors: []ValidationError{
					{Field: "ReceiptID", Message: "Receipt ID already exists in database: " + h.ReceiptID, Row: h.Row},
				},
			})
		}
	}

	// ---------- Mulai transaction (ALL-OR-NOTHING untuk seluruh file) ----------
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateInboundFromExcelFile: %v", r)
		}
	}()

	repo := repositories.NewInboundRepository(tx)

	var createdInbounds []string
	successCount := 0

	// Iterasi grup dengan urutan STABIL (sesuai kemunculan pertama di file),
	// bukan urutan acak map.
	for _, group := range groups {
		h := group.HeaderRow

		inboundNo, err := repo.GenerateInboundNo()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate inbound number for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Inbound Generation Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := tx.First(&supplier, "supplier_code = ?", h.Supplier).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to validate supplier for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}

		inboundHeader := models.InboundHeader{
			InboundNo:      inboundNo,
			InboundDate:    h.InboundDate,
			ReceiptID:      h.ReceiptID,
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
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound header for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		// RefNo = ReceiptID (disepakati keduanya konsep yang sama)
		inboundReference := models.InboundReference{
			InboundId: uint(inboundHeader.ID),
			RefNo:     h.ReceiptID,
		}
		if err := tx.Create(&inboundReference).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound reference for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		for _, detail := range group.Details {
			var product models.Product
			if err := tx.First(&product, "item_code = ? AND owner_code = ?", detail.ItemCode, h.OwnerCode).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Product not found for item code: %s (Receipt ID %s)", detail.ItemCode, h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Product Not Found", Detail: "Item code: " + detail.ItemCode},
					},
				})
			}

			var uomConversion models.UomConversion
			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("UOM conversion not found (Receipt ID %s)", h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "UOM Not Found", Detail: fmt.Sprintf("Item: %s, UOM: %s", detail.ItemCode, detail.UOM)},
					},
				})
			}

			if detail.QaStatus != "" {
				var qaStatus models.QaStatus
				if err := tx.First(&qaStatus, "qa_status = ?", detail.QaStatus).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
						Success: false,
						Message: fmt.Sprintf("QA status not found (Receipt ID %s)", h.ReceiptID),
						Errors: []ExcelRowError{
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
				Quantity:      detail.Quantity,
				RcvLocation:   detail.Location,
				Location:      detail.Location,
				QaStatus:      detail.QaStatus,
				RecDate:       detail.RecDate,
				ProdDate:      detail.ProdDate,
				ExpDate:       detail.ExpDate,
				LotNumber:     detail.LotNumber,
				CartonNumber:  detail.CartonNumber,
				CaseNumber:    detail.CaseNumber,
				SerialNumber:  detail.SerialNumber,
				IsSerial:      product.HasSerial,
				SN:            product.HasSerial,
				RefId:         int(inboundReference.ID),
				RefNo:         h.ReceiptID,
				OwnerCode:     h.OwnerCode,
				WhsCode:       h.WhsCode,
				DivisionCode:  detail.Division,
				CreatedBy:     userID,
				UpdatedBy:     userID,
			}

			if err := tx.Create(&inboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to create inbound detail (Receipt ID %s)", h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
					},
				})
			}

			successCount++
		}

		if err := helpers.InsertTransactionHistory(tx, inboundNo, "open", "INBOUND", "Created from Excel upload", userID); err != nil {
			log.Printf("Warning: Failed to insert transaction history for %s: %v", inboundNo, err)
		}

		createdInbounds = append(createdInbounds, inboundNo)
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()},
			},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(ExcelUploadResponse{
		Success:        true,
		Message:        fmt.Sprintf("Successfully created %d inbound(s) with %d items", len(createdInbounds), successCount),
		TotalRows:      len(parsedRows),
		SuccessCount:   successCount,
		FailedCount:    0,
		InboundNumbers: createdInbounds,
	})
}

// =======================================
// END IMPORT FROM EXCEL FILE
// =======================================
