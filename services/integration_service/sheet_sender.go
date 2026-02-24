// package integration_service

// import (
// 	"context"
// 	"encoding/json"
// 	"fiber-app/models/integration"
// 	"fmt"
// 	"strings"
// 	"time"

// 	"golang.org/x/oauth2/google"
// 	"google.golang.org/api/option"
// 	"google.golang.org/api/sheets/v4"
// )

// // ─── Main Sender ──────────────────────────────────────────────────────────────

// func sendViaGoogleSheets(
// 	conn integration.IntegrationConnection,
// 	file *GeneratedFile,
// 	eventData map[string]interface{},
// 	intg integration.Integration,
// ) error {
// 	if conn.SpreadsheetID == "" {
// 		return fmt.Errorf("spreadsheet_id belum dikonfigurasi")
// 	}
// 	if conn.CredentialsJSON == "" {
// 		return fmt.Errorf("credentials_json belum dikonfigurasi")
// 	}

// 	// Build Sheets service dari service account JSON
// 	srv, err := buildSheetsService(conn.CredentialsJSON)
// 	if err != nil {
// 		return fmt.Errorf("gagal build Sheets service: %w", err)
// 	}

// 	sheetName := conn.SheetName
// 	if sheetName == "" {
// 		sheetName = "Sheet1"
// 	}

// 	// Tentukan data yang dikirim
// 	// Kalau ada file (source=query atau CSV dll) → pakai data dari file
// 	// Kalau tidak ada file (source=event, channel=google_sheets) → pakai event data langsung
// 	var rows [][]interface{}
// 	var headers []string

// 	if file != nil && len(file.Data) > 0 {
// 		// Parse file CSV/JSON yang sudah digenerate
// 		parsedRows, err := ReadFile(file.Data, string(intg.FileFormat), file.Name)
// 		if err != nil {
// 			return fmt.Errorf("gagal parse file untuk Google Sheets: %w", err)
// 		}
// 		if len(parsedRows) > 0 {
// 			// Ambil headers dari key pertama
// 			for k := range parsedRows[0] {
// 				headers = append(headers, k)
// 			}
// 			for _, pr := range parsedRows {
// 				row := make([]interface{}, len(headers))
// 				for i, h := range headers {
// 					row[i] = fmt.Sprintf("%v", pr[h])
// 				}
// 				rows = append(rows, row)
// 			}
// 		}
// 	} else {
// 		// Pakai event data langsung — 1 row
// 		for k := range eventData {
// 			if k != "year" && k != "date" && k != "datetime" {
// 				headers = append(headers, k)
// 			}
// 		}
// 		row := make([]interface{}, len(headers))
// 		for i, h := range headers {
// 			v := eventData[h]
// 			if v == nil {
// 				row[i] = ""
// 			} else if t, ok := v.(time.Time); ok {
// 				row[i] = t.Format("2006-01-02 15:04:05")
// 			} else {
// 				row[i] = fmt.Sprintf("%v", v)
// 			}
// 		}
// 		rows = append(rows, row)
// 	}

// 	if len(rows) == 0 {
// 		return fmt.Errorf("tidak ada data untuk dikirim ke Google Sheets")
// 	}

// 	switch conn.AppendMode {
// 	case "overwrite":
// 		return overwriteSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
// 	default:
// 		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
// 	}
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

// 	// Tulis header hanya kalau sheet masih kosong dan writeHeader = true
// 	if sheetEmpty && writeHeader {
// 		headerRow := make([]interface{}, len(headers))
// 		for i, h := range headers {
// 			headerRow[i] = strings.ToUpper(strings.ReplaceAll(h, "_", " "))
// 		}
// 		allValues = append(allValues, headerRow)
// 	}

// 	allValues = append(allValues, rows...)

// 	appendRange := sheetName + "!A:A"
// 	valueRange := &sheets.ValueRange{Values: allValues}

// 	_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
// 		ValueInputOption("USER_ENTERED").
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
// 	// Clear dulu semua isi sheet
// 	clearRange := sheetName + "!A:ZZ"
// 	_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do()
// 	if err != nil {
// 		return fmt.Errorf("gagal clear sheet: %w", err)
// 	}

// 	var allValues [][]interface{}

// 	if writeHeader {
// 		headerRow := make([]interface{}, len(headers))
// 		for i, h := range headers {
// 			headerRow[i] = strings.ToUpper(strings.ReplaceAll(h, "_", " "))
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

// 	// Parse credentials JSON
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

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gorm.io/gorm"
)

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
		return fmt.Errorf("gagal build Sheets service: %w", err)
	}

	sheetName := conn.SheetName
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	var headers []string
	var rows [][]interface{}

	switch intg.SourceType {
	case integration.SourceQuery:
		// Jalankan query langsung — tidak perlu generate file dulu
		headers, rows, err = fetchFromQuery(queryDB, intg.Query, eventData)
		if err != nil {
			return fmt.Errorf("gagal jalankan query: %w", err)
		}

	default:
		// Source = event — pakai event data langsung (1 row)
		headers, rows = fetchFromEvent(eventData)
	}

	if len(rows) == 0 {
		return fmt.Errorf("query tidak menghasilkan data")
	}

	switch conn.AppendMode {
	case "overwrite":
		return overwriteSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
	default:
		return appendToSheet(srv, conn.SpreadsheetID, sheetName, headers, rows, conn.HeaderRow)
	}
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

	// Validasi SQL aman
	if err := rm_services.ValidateSelectOnly(query); err != nil {
		return nil, nil, fmt.Errorf("query tidak aman: %w", err)
	}

	// Resolve placeholder {{order_no}}, {{outbound_no}}, dll
	resolvedQuery := query
	for k, v := range eventData {
		resolvedQuery = strings.ReplaceAll(resolvedQuery, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}

	// Jalankan query
	sqlRows, err := db.Raw(resolvedQuery).Rows()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal jalankan query: %w", err)
	}
	defer sqlRows.Close()

	// Ambil nama kolom
	columns, err := sqlRows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("gagal baca kolom: %w", err)
	}

	// Baca semua rows
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

// ─── Append Mode ──────────────────────────────────────────────────────────────

func appendToSheet(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	headers []string,
	rows [][]interface{},
	writeHeader bool,
) error {
	// Cek apakah sheet sudah ada isinya
	readRange := sheetName + "!A1:A1"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	sheetEmpty := err != nil || len(resp.Values) == 0

	var allValues [][]interface{}

	// Tulis header hanya kalau sheet masih kosong
	if sheetEmpty && writeHeader {
		headerRow := make([]interface{}, len(headers))
		for i, h := range headers {
			headerRow[i] = h // pakai nama kolom dari query as-is (sudah ada alias)
		}
		allValues = append(allValues, headerRow)
	}

	allValues = append(allValues, rows...)

	appendRange := sheetName + "!A:A"
	valueRange := &sheets.ValueRange{Values: allValues}

	_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		// ValueInputOption("USER_ENTERED").
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return fmt.Errorf("gagal append ke Google Sheets: %w", err)
	}

	return nil
}

// ─── Overwrite Mode ───────────────────────────────────────────────────────────

func overwriteSheet(
	srv *sheets.Service,
	spreadsheetID string,
	sheetName string,
	headers []string,
	rows [][]interface{},
	writeHeader bool,
) error {
	clearRange := sheetName + "!A:ZZ"
	_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
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

	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Do()
	if err != nil {
		return fmt.Errorf("gagal overwrite Google Sheets: %w", err)
	}

	return nil
}

// ─── Build Sheets Service ─────────────────────────────────────────────────────

func buildSheetsService(credentialsJSON string) (*sheets.Service, error) {
	ctx := context.Background()

	var creds map[string]interface{}
	if err := json.Unmarshal([]byte(credentialsJSON), &creds); err != nil {
		return nil, fmt.Errorf("credentials_json tidak valid: %w", err)
	}

	conf, err := google.CredentialsFromJSON(
		ctx,
		[]byte(credentialsJSON),
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal parse credentials: %w", err)
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(conf))
	if err != nil {
		return nil, fmt.Errorf("gagal buat Sheets service: %w", err)
	}

	return srv, nil
}
