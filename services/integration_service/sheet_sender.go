// package integration_service

// import (
// 	"context"
// 	"database/sql"
// 	"encoding/json"
// 	"fiber-app/models/integration"
// 	rm_services "fiber-app/services/report_mailer"
// 	"fmt"
// 	"strings"
// 	"time"

// 	"golang.org/x/oauth2/google"
// 	"google.golang.org/api/option"
// 	"google.golang.org/api/sheets/v4"
// 	"gorm.io/gorm"
// )

// // ─── Main Sender ──────────────────────────────────────────────────────────────

// func sendViaGoogleSheets(
// 	db *gorm.DB,
// 	queryDB *gorm.DB,
// 	conn integration.IntegrationConnection,
// 	intg integration.Integration,
// 	eventData map[string]interface{},
// ) error {
// 	if conn.SpreadsheetID == "" {
// 		return fmt.Errorf("spreadsheet_id belum dikonfigurasi")
// 	}
// 	if conn.CredentialsJSON == "" {
// 		return fmt.Errorf("credentials_json belum dikonfigurasi")
// 	}

// 	srv, err := buildSheetsService(conn.CredentialsJSON)
// 	if err != nil {
// 		return fmt.Errorf("gagal build Sheets service: %w", err)
// 	}

// 	sheetName := conn.SheetName
// 	if sheetName == "" {
// 		sheetName = "Sheet1"
// 	}

// 	var headers []string
// 	var rows [][]interface{}

// 	switch intg.SourceType {
// 	case integration.SourceQuery:
// 		// Jalankan query langsung — tidak perlu generate file dulu
// 		headers, rows, err = fetchFromQuery(queryDB, intg.Query, eventData)
// 		if err != nil {
// 			return fmt.Errorf("gagal jalankan query: %w", err)
// 		}

// 	default:
// 		// Source = event — pakai event data langsung (1 row)
// 		headers, rows = fetchFromEvent(eventData)
// 	}

// 	if len(rows) == 0 {
// 		return fmt.Errorf("query tidak menghasilkan data")
// 	}

// 	switch conn.AppendMode {
// 	case "overwrite":
// 		return overwriteSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
// 	default:
// 		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
// 	}
// }

// // ─── Fetch dari Query ─────────────────────────────────────────────────────────

// func fetchFromQuery(
// 	db *gorm.DB,
// 	query string,
// 	eventData map[string]interface{},
// ) ([]string, [][]interface{}, error) {
// 	if query == "" {
// 		return nil, nil, fmt.Errorf("query kosong")
// 	}

// 	// Validasi SQL aman
// 	if err := rm_services.ValidateSelectOnly(query); err != nil {
// 		return nil, nil, fmt.Errorf("query tidak aman: %w", err)
// 	}

// 	// Resolve placeholder {{order_no}}, {{outbound_no}}, dll
// 	resolvedQuery := query
// 	for k, v := range eventData {
// 		resolvedQuery = strings.ReplaceAll(resolvedQuery, "{{"+k+"}}", fmt.Sprintf("%v", v))
// 	}

// 	// Jalankan query
// 	sqlRows, err := db.Raw(resolvedQuery).Rows()
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("gagal jalankan query: %w", err)
// 	}
// 	defer sqlRows.Close()

// 	// Ambil nama kolom
// 	columns, err := sqlRows.Columns()
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("gagal baca kolom: %w", err)
// 	}

// 	// Baca semua rows
// 	var rows [][]interface{}
// 	for sqlRows.Next() {
// 		values := make([]interface{}, len(columns))
// 		ptrs := make([]interface{}, len(columns))
// 		for i := range values {
// 			ptrs[i] = &values[i]
// 		}
// 		if err := sqlRows.Scan(ptrs...); err != nil {
// 			return nil, nil, fmt.Errorf("gagal scan row: %w", err)
// 		}

// 		row := make([]interface{}, len(values))
// 		for i, v := range values {
// 			switch val := v.(type) {
// 			case []byte:
// 				row[i] = string(val)
// 			case time.Time:
// 				row[i] = val.Format("2006-01-02 15:04:05")
// 			case sql.NullString:
// 				if val.Valid {
// 					row[i] = val.String
// 				} else {
// 					row[i] = ""
// 				}
// 			case nil:
// 				row[i] = ""
// 			default:
// 				row[i] = fmt.Sprintf("%v", v)
// 			}
// 		}
// 		rows = append(rows, row)
// 	}

// 	return columns, rows, nil
// }

// // ─── Fetch dari Event Data ────────────────────────────────────────────────────

// func fetchFromEvent(eventData map[string]interface{}) ([]string, [][]interface{}) {
// 	var headers []string
// 	for k := range eventData {
// 		if k != "year" && k != "date" && k != "datetime" {
// 			headers = append(headers, k)
// 		}
// 	}

// 	row := make([]interface{}, len(headers))
// 	for i, h := range headers {
// 		v := eventData[h]
// 		if v == nil {
// 			row[i] = ""
// 		} else if t, ok := v.(time.Time); ok {
// 			row[i] = t.Format("2006-01-02 15:04:05")
// 		} else {
// 			row[i] = fmt.Sprintf("%v", v)
// 		}
// 	}

// 	return headers, [][]interface{}{row}
// }

// // ─── Append Mode ──────────────────────────────────────────────────────────────

// func appendToSheet(
// 	srv *sheets.Service,
// 	spreadsheetID string,
// 	sheetName string,
// 	headers []string,
// 	rows [][]interface{},
// 	writeHeader bool,
// ) error {
// 	// Cek apakah sheet sudah ada isinya
// 	readRange := sheetName + "!A1:A1"
// 	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
// 	sheetEmpty := err != nil || len(resp.Values) == 0

// 	var allValues [][]interface{}

// 	// Tulis header hanya kalau sheet masih kosong
// 	if sheetEmpty && writeHeader {
// 		headerRow := make([]interface{}, len(headers))
// 		for i, h := range headers {
// 			headerRow[i] = h // pakai nama kolom dari query as-is (sudah ada alias)
// 		}
// 		allValues = append(allValues, headerRow)
// 	}

// 	allValues = append(allValues, rows...)

// 	appendRange := sheetName + "!A:A"
// 	valueRange := &sheets.ValueRange{Values: allValues}

// 	_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
// 		// ValueInputOption("USER_ENTERED").
// 		ValueInputOption("RAW").
// 		InsertDataOption("INSERT_ROWS").
// 		Do()
// 	if err != nil {
// 		return fmt.Errorf("gagal append ke Google Sheets: %w", err)
// 	}

// 	return nil
// }

// // ─── Overwrite Mode ───────────────────────────────────────────────────────────

// func overwriteSheet(
// 	srv *sheets.Service,
// 	spreadsheetID string,
// 	sheetName string,
// 	headers []string,
// 	rows [][]interface{},
// 	writeHeader bool,
// ) error {
// 	clearRange := sheetName + "!A:ZZ"
// 	_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do()
// 	if err != nil {
// 		return fmt.Errorf("gagal clear sheet: %w", err)
// 	}

// 	var allValues [][]interface{}
// 	if writeHeader {
// 		headerRow := make([]interface{}, len(headers))
// 		for i, h := range headers {
// 			headerRow[i] = h
// 		}
// 		allValues = append(allValues, headerRow)
// 	}
// 	allValues = append(allValues, rows...)

// 	writeRange := sheetName + "!A1"
// 	valueRange := &sheets.ValueRange{Values: allValues}

// 	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).
// 		ValueInputOption("USER_ENTERED").
// 		Do()
// 	if err != nil {
// 		return fmt.Errorf("gagal overwrite Google Sheets: %w", err)
// 	}

// 	return nil
// }

// // ─── Build Sheets Service ─────────────────────────────────────────────────────

// func buildSheetsService(credentialsJSON string) (*sheets.Service, error) {
// 	ctx := context.Background()

// 	var creds map[string]interface{}
// 	if err := json.Unmarshal([]byte(credentialsJSON), &creds); err != nil {
// 		return nil, fmt.Errorf("credentials_json tidak valid: %w", err)
// 	}

// 	conf, err := google.CredentialsFromJSON(
// 		ctx,
// 		[]byte(credentialsJSON),
// 		sheets.SpreadsheetsScope,
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal parse credentials: %w", err)
// 	}

// 	srv, err := sheets.NewService(ctx, option.WithCredentials(conf))
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal buat Sheets service: %w", err)
// 	}

// 	return srv, nil
// }

package integration_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fiber-app/models/integration"
	rm_services "fiber-app/services/report_mailer"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gorm.io/gorm"
)

// ─── Key Column Names untuk auto-detect (fallback) ───────────────────────────
var knownKeyColumns = []string{
	"spk_no", "spk no",
	"order_no", "order no",
	"order_number", "order number",
	"outbound_no", "outbound no",
	"shipment_id", "shipment id",
	"doc_no", "doc no",
	"so_no", "so no",
}

// ─── Main Sender ──────────────────────────────────────────────────────────────

func sendViaGoogleSheets(
	db *gorm.DB,
	queryDB *gorm.DB,
	conn integration.IntegrationConnection,
	intg integration.Integration,
	eventData map[string]interface{},
) error {
	if conn.SpreadsheetID == "" {
		return fmt.Errorf("spreadsheet_id belum dikonfigurasi")
	}
	if conn.CredentialsJSON == "" {
		return fmt.Errorf("credentials_json belum dikonfigurasi")
	}

	srv, err := buildSheetsService(conn.CredentialsJSON)
	if err != nil {
		return err
	}

	sheetName := conn.SheetName
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	if err := validateSheet(srv, conn.SpreadsheetID, sheetName); err != nil {
		return err
	}

	// Ambil data
	var headers []string
	var rows [][]interface{}

	switch intg.SourceType {
	case integration.SourceQuery:
		headers, rows, err = fetchFromQuery(queryDB, intg.Query, eventData)
		if err != nil {
			return fmt.Errorf("gagal jalankan query: %w", err)
		}
	default:
		headers, rows = fetchFromEvent(eventData)
	}

	if len(rows) == 0 {
		return fmt.Errorf("tidak ada data untuk dikirim ke Google Sheets")
	}

	rows = normalizeRows(rows, len(headers))

	orderNo := extractOrderNo(eventData)
	if orderNo == "" {
		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
	}

	// Detect kolom key — prioritas: config manual → auto-detect → fallback kolom A
	_, keyColLetter, err := detectKeyColumn(srv, conn.SpreadsheetID, sheetName, conn.KeyColumn)
	if err != nil {
		fmt.Printf("[GoogleSheets] Warning: gagal detect kolom key, fallback ke kolom A: %v\n", err)
		// keyColIndex = 0
		keyColLetter = "A"
	}

	fmt.Printf("[GoogleSheets] orderNo: '%s', keyCol: '%s'\n", orderNo, keyColLetter)

	existingRows, err := findRowsByKey(srv, conn.SpreadsheetID, sheetName, keyColLetter, orderNo)
	if err != nil {
		fmt.Printf("[GoogleSheets] Warning: gagal lookup baris, fallback append: %v\n", err)
		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
	}

	fmt.Printf("[GoogleSheets] existingRows: %v\n", existingRows)

	if len(existingRows) == 0 {
		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
	}

	// return upsertRows(srv, conn.SpreadsheetID, sheetName, rows, existingRows, keyColIndex)
	return upsertRows(srv, conn.SpreadsheetID, sheetName, rows, existingRows)
}

// ─── Detect Key Column ────────────────────────────────────────────────────────

// detectKeyColumn cari posisi kolom key di sheet
// Priority:
//  1. keyColumn dari config (manual) — paling akurat
//  2. auto-detect dari knownKeyColumns
//  3. fallback kolom A
func detectKeyColumn(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	keyColumn string, // dari conn.KeyColumn, bisa kosong
) (int, string, error) {
	// Baca baris header
	headerRange := sheetName + "!1:1"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, headerRange).Do()
	if err != nil {
		return 0, "A", fmt.Errorf("gagal baca header: %w", err)
	}

	if len(resp.Values) == 0 || len(resp.Values[0]) == 0 {
		return 0, "A", nil
	}

	headers := resp.Values[0]

	// Priority 1: kalau user sudah set KeyColumn di config → cari exact match (case insensitive)
	if keyColumn != "" {
		target := strings.TrimSpace(strings.ToLower(keyColumn))
		for i, cell := range headers {
			colName := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", cell)))
			if colName == target {
				colLetter, _ := excelize.ColumnNumberToName(i + 1)
				fmt.Printf("[GoogleSheets] KeyColumn '%s' ditemukan di kolom %s\n", keyColumn, colLetter)
				return i, colLetter, nil
			}
		}
		// Tidak ketemu meski sudah di-config → warning, lanjut auto-detect
		fmt.Printf("[GoogleSheets] Warning: KeyColumn '%s' tidak ditemukan di header sheet, fallback auto-detect\n", keyColumn)
	}

	// Priority 2: auto-detect dari knownKeyColumns
	for i, cell := range headers {
		colName := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", cell)))
		for _, knownKey := range knownKeyColumns {
			if colName == knownKey {
				colLetter, _ := excelize.ColumnNumberToName(i + 1)
				fmt.Printf("[GoogleSheets] Auto-detect KeyColumn '%s' di kolom %s\n", cell, colLetter)
				return i, colLetter, nil
			}
		}
	}

	// Priority 3: fallback kolom A
	fmt.Printf("[GoogleSheets] Tidak ada kolom key yang cocok, fallback ke kolom A\n")
	return 0, "A", nil
}

// ─── Find Rows By Key ─────────────────────────────────────────────────────────

func findRowsByKey(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	keyColLetter string,
	keyValue string,
) ([]int, error) {
	readRange := fmt.Sprintf("%s!%s:%s", sheetName, keyColLetter, keyColLetter)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("gagal baca kolom %s: %w", keyColLetter, err)
	}

	var matchedRows []int
	for i, row := range resp.Values {
		if len(row) > 0 {
			cellVal := strings.TrimSpace(fmt.Sprintf("%v", row[0]))
			if cellVal == keyValue {
				matchedRows = append(matchedRows, i+1) // 1-based
			}
		}
	}

	return matchedRows, nil
}

// ─── Upsert Rows ──────────────────────────────────────────────────────────────

// func upsertRows(
// 	srv *sheets.Service,
// 	spreadsheetID string,
// 	sheetName string,
// 	newRows [][]interface{},
// 	existingRowIndices []int,
// 	startColIndex int,
// ) error {
// 	newCount := len(newRows)
// 	existCount := len(existingRowIndices)
// 	startColLetter, _ := excelize.ColumnNumberToName(startColIndex + 1)

// 	// Update baris yang sudah ada
// 	updateCount := min(newCount, existCount)
// 	for i := 0; i < updateCount; i++ {
// 		updateRange := fmt.Sprintf("%s!%s%d", sheetName, startColLetter, existingRowIndices[i])
// 		valueRange := &sheets.ValueRange{Values: [][]interface{}{newRows[i]}}
// 		if _, err := srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
// 			ValueInputOption("RAW").Do(); err != nil {
// 			return fmt.Errorf("gagal update baris %d: %w", existingRowIndices[i], err)
// 		}
// 	}

// 	// Baris baru lebih banyak → append sisanya
// 	if newCount > existCount {
// 		extraRows := newRows[existCount:]
// 		appendRange := sheetName + "!A:A"
// 		valueRange := &sheets.ValueRange{Values: extraRows}
// 		if _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
// 			ValueInputOption("RAW").InsertDataOption("INSERT_ROWS").Do(); err != nil {
// 			return fmt.Errorf("gagal append baris tambahan: %w", err)
// 		}
// 	}

// 	// Baris lama lebih banyak → hapus sisa
// 	if existCount > newCount {
// 		sheetID, err := getSheetID(srv, spreadsheetID, sheetName)
// 		if err != nil {
// 			// Fallback: clear isi baris saja
// 			for i := newCount; i < existCount; i++ {
// 				clearRange := fmt.Sprintf("%s!A%d:ZZ%d", sheetName, existingRowIndices[i], existingRowIndices[i])
// 				srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do()
// 			}
// 			return nil
// 		}

// 		// Hapus dari bawah ke atas supaya index tidak bergeser
// 		var deleteRequests []*sheets.Request
// 		for i := existCount - 1; i >= newCount; i-- {
// 			rowIndex := int64(existingRowIndices[i] - 1)
// 			deleteRequests = append(deleteRequests, &sheets.Request{
// 				DeleteDimension: &sheets.DeleteDimensionRequest{
// 					Range: &sheets.DimensionRange{
// 						SheetId:    sheetID,
// 						Dimension:  "ROWS",
// 						StartIndex: rowIndex,
// 						EndIndex:   rowIndex + 1,
// 					},
// 				},
// 			})
// 		}

// 		if len(deleteRequests) > 0 {
// 			batchReq := &sheets.BatchUpdateSpreadsheetRequest{Requests: deleteRequests}
// 			if _, err := srv.Spreadsheets.BatchUpdate(spreadsheetID, batchReq).Do(); err != nil {
// 				fmt.Printf("[GoogleSheets] Warning: gagal hapus baris lama: %v\n", err)
// 			}
// 		}
// 	}

// 	return nil
// }

func upsertRows(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	newRows [][]interface{},
	existingRowIndices []int,
	// startColIndex dihapus karena kita ingin mulai dari A
) error {
	newCount := len(newRows)
	existCount := len(existingRowIndices)

	// 1. Update baris yang sudah ada
	updateCount := min(newCount, existCount)
	for i := 0; i < updateCount; i++ {
		// Kita paksa mulai dari "A" agar data dari query (index 0)
		// masuk ke kolom pertama di Sheets, bukan mulai dari tengah.
		updateRange := fmt.Sprintf("%s!A%d", sheetName, existingRowIndices[i])

		valueRange := &sheets.ValueRange{Values: [][]interface{}{newRows[i]}}
		if _, err := srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
			ValueInputOption("RAW").Do(); err != nil {
			return fmt.Errorf("gagal update baris %d: %w", existingRowIndices[i], err)
		}
	}

	// 2. Jika data baru lebih banyak daripada yang ditemukan di sheet -> Append sisanya
	if newCount > existCount {
		extraRows := newRows[existCount:]
		appendRange := sheetName + "!A:A"
		valueRange := &sheets.ValueRange{Values: extraRows}
		if _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
			ValueInputOption("RAW").InsertDataOption("INSERT_ROWS").Do(); err != nil {
			return fmt.Errorf("gagal append baris tambahan: %w", err)
		}
	}

	// 3. Jika baris lama di sheet lebih banyak daripada data baru -> Hapus sisa baris lama
	// (Logika delete tetap sama seperti sebelumnya)
	if existCount > newCount {
		sheetID, err := getSheetID(srv, spreadsheetID, sheetName)
		if err != nil {
			for i := newCount; i < existCount; i++ {
				clearRange := fmt.Sprintf("%s!A%d:ZZ%d", sheetName, existingRowIndices[i], existingRowIndices[i])
				srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do()
			}
			return nil
		}

		var deleteRequests []*sheets.Request
		for i := existCount - 1; i >= newCount; i-- {
			rowIndex := int64(existingRowIndices[i] - 1)
			deleteRequests = append(deleteRequests, &sheets.Request{
				DeleteDimension: &sheets.DeleteDimensionRequest{
					Range: &sheets.DimensionRange{
						SheetId:    sheetID,
						Dimension:  "ROWS",
						StartIndex: rowIndex,
						EndIndex:   rowIndex + 1,
					},
				},
			})
		}

		if len(deleteRequests) > 0 {
			batchReq := &sheets.BatchUpdateSpreadsheetRequest{Requests: deleteRequests}
			if _, err := srv.Spreadsheets.BatchUpdate(spreadsheetID, batchReq).Do(); err != nil {
				fmt.Printf("[GoogleSheets] Warning: gagal hapus baris lama: %v\n", err)
			}
		}
	}

	return nil
}

// ─── Append to Sheet ─────────────────────────────────────────────────────────

func appendToSheet(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	headers []string,
	rows [][]interface{},
	writeHeader bool,
) error {
	readRange := sheetName + "!A1:A1"
	resp, _ := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	sheetEmpty := resp == nil || len(resp.Values) == 0

	var allValues [][]interface{}
	if sheetEmpty && writeHeader {
		headerRow := make([]interface{}, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		allValues = append(allValues, headerRow)
	}
	allValues = append(allValues, rows...)

	appendRange := sheetName + "!A:A"
	valueRange := &sheets.ValueRange{Values: allValues}

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("RAW").InsertDataOption("INSERT_ROWS").Do()
	if err != nil {
		return fmt.Errorf("gagal append ke Google Sheets: %w", err)
	}
	return nil
}

// ─── Overwrite Sheet ──────────────────────────────────────────────────────────

func overwriteSheet(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	headers []string,
	rows [][]interface{},
	writeHeader bool,
) error {
	clearRange := sheetName + "!A:ZZ"
	if _, err := srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do(); err != nil {
		return fmt.Errorf("gagal clear sheet: %w", err)
	}

	var allValues [][]interface{}
	if writeHeader {
		headerRow := make([]interface{}, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		allValues = append(allValues, headerRow)
	}
	allValues = append(allValues, rows...)

	writeRange := sheetName + "!A1"
	valueRange := &sheets.ValueRange{Values: allValues}

	if _, err := srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).
		ValueInputOption("RAW").Do(); err != nil {
		return fmt.Errorf("gagal overwrite Google Sheets: %w", err)
	}
	return nil
}

// ─── Fetch dari Query ─────────────────────────────────────────────────────────

func fetchFromQuery(
	db *gorm.DB,
	query string,
	eventData map[string]interface{},
) ([]string, [][]interface{}, error) {
	if query == "" {
		return nil, nil, fmt.Errorf("query kosong")
	}

	if err := rm_services.ValidateSelectOnly(query); err != nil {
		return nil, nil, fmt.Errorf("query tidak aman: %w", err)
	}

	resolvedQuery := query
	for k, v := range eventData {
		resolvedQuery = strings.ReplaceAll(resolvedQuery, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}

	sqlRows, err := db.Raw(resolvedQuery).Rows()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal jalankan query: %w", err)
	}
	defer sqlRows.Close()

	columns, err := sqlRows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal baca kolom: %w", err)
	}

	var rows [][]interface{}
	for sqlRows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := sqlRows.Scan(ptrs...); err != nil {
			return nil, nil, fmt.Errorf("gagal scan row: %w", err)
		}

		row := make([]interface{}, len(values))
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				row[i] = string(val)
			case time.Time:
				row[i] = val.Format("2006-01-02 15:04:05")
			case sql.NullTime:
				if val.Valid {
					row[i] = val.Time.Format("2006-01-02 15:04:05")
				} else {
					row[i] = ""
				}
			case sql.NullString:
				if val.Valid {
					row[i] = val.String
				} else {
					row[i] = ""
				}
			case nil:
				row[i] = ""
			default:
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		rows = append(rows, row)
	}

	return columns, rows, nil
}

// ─── Fetch dari Event Data ────────────────────────────────────────────────────

func fetchFromEvent(eventData map[string]interface{}) ([]string, [][]interface{}) {
	var headers []string
	for k := range eventData {
		if k != "year" && k != "date" && k != "datetime" {
			headers = append(headers, k)
		}
	}

	row := make([]interface{}, len(headers))
	for i, h := range headers {
		v := eventData[h]
		if v == nil {
			row[i] = ""
		} else if t, ok := v.(time.Time); ok {
			row[i] = t.Format("2006-01-02 15:04:05")
		} else {
			row[i] = fmt.Sprintf("%v", v)
		}
	}

	return headers, [][]interface{}{row}
}

// ─── Validate Sheet ───────────────────────────────────────────────────────────

func validateSheet(srv *sheets.Service, spreadsheetID string, sheetName string) error {
	spreadsheet, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		if isGoogleAPIError(err, 403) {
			return fmt.Errorf("akses ditolak — pastikan sheet sudah di-share ke service account")
		}
		if isGoogleAPIError(err, 404) {
			return fmt.Errorf("spreadsheet tidak ditemukan — cek Spreadsheet ID")
		}
		return fmt.Errorf("gagal akses spreadsheet: %w", err)
	}

	for _, s := range spreadsheet.Sheets {
		if s.Properties.Title == sheetName {
			return nil
		}
	}

	available := make([]string, 0, len(spreadsheet.Sheets))
	for _, s := range spreadsheet.Sheets {
		available = append(available, s.Properties.Title)
	}
	return fmt.Errorf("sheet '%s' tidak ditemukan. Sheet yang tersedia: %s",
		sheetName, strings.Join(available, ", "))
}

// ─── Get Sheet ID ─────────────────────────────────────────────────────────────

func getSheetID(srv *sheets.Service, spreadsheetID string, sheetName string) (int64, error) {
	spreadsheet, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return 0, err
	}
	for _, s := range spreadsheet.Sheets {
		if s.Properties.Title == sheetName {
			return s.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("sheet '%s' tidak ditemukan", sheetName)
}

// ─── Extract Order No ─────────────────────────────────────────────────────────

func extractOrderNo(eventData map[string]interface{}) string {
	for _, key := range []string{"order_no", "order_number", "outbound_no", "shipment_id", "doc_no", "so_no", "spk_no"} {
		if v, ok := eventData[key]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// ─── Normalize Rows ───────────────────────────────────────────────────────────

func normalizeRows(rows [][]interface{}, colCount int) [][]interface{} {
	for i, row := range rows {
		if len(row) < colCount {
			for j := len(row); j < colCount; j++ {
				rows[i] = append(rows[i], "")
			}
		} else if len(row) > colCount {
			rows[i] = row[:colCount]
		}
	}
	return rows
}

// ─── Build Sheets Service ─────────────────────────────────────────────────────

func buildSheetsService(credentialsJSON string) (*sheets.Service, error) {
	ctx := context.Background()

	var creds map[string]interface{}
	if err := json.Unmarshal([]byte(credentialsJSON), &creds); err != nil {
		return nil, fmt.Errorf("credentials_json tidak valid — pastikan paste seluruh isi file JSON: %w", err)
	}

	conf, err := google.CredentialsFromJSON(ctx, []byte(credentialsJSON), sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("gagal parse credentials service account: %w", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(conf))
	if err != nil {
		return nil, fmt.Errorf("gagal buat Sheets service: %w", err)
	}

	return srv, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isGoogleAPIError(err error, code int) bool {
	if apiErr, ok := err.(*googleapi.Error); ok {
		return apiErr.Code == code
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
