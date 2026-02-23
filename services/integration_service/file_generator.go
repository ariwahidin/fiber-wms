package integration_service

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fiber-app/models/integration"
	report_mailer "fiber-app/models/report_mailer"
	rm_services "fiber-app/services/report_mailer"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// GeneratedFile hasil generate file
type GeneratedFile struct {
	Name string
	Data []byte
}

// GenerateFile generate file sesuai format dan source type
func GenerateFile(
	db *gorm.DB,
	intg integration.Integration,
	eventData map[string]interface{},
) (*GeneratedFile, error) {
	fileName := resolveFilename(intg.FilenamePattern, intg.Name, intg.FileFormat, eventData)

	switch intg.SourceType {
	case integration.SourceQuery:
		return generateFromQuery(db, intg, fileName)
	case integration.SourceEvent:
		return generateFromEvent(intg, eventData, fileName)
	default:
		return nil, fmt.Errorf("source_type tidak dikenal: %s", intg.SourceType)
	}
}

// ─── Generate dari Query ──────────────────────────────────────────────────────

func generateFromQuery(db *gorm.DB, intg integration.Integration, fileName string) (*GeneratedFile, error) {
	if intg.Query == "" {
		return nil, fmt.Errorf("query kosong")
	}

	// Validasi query aman
	if err := rm_services.ValidateSelectOnly(intg.Query); err != nil {
		return nil, fmt.Errorf("query tidak aman: %w", err)
	}

	// Jalankan query
	rows, err := db.Raw(intg.Query).Rows()
	if err != nil {
		return nil, fmt.Errorf("gagal jalankan query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("gagal baca kolom: %w", err)
	}

	var data [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("gagal scan row: %w", err)
		}
		row := make([]interface{}, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		data = append(data, row)
	}

	return buildFile(intg.FileFormat, fileName, columns, data, intg)
}

// ─── Generate dari Event Data ─────────────────────────────────────────────────

func generateFromEvent(intg integration.Integration, eventData map[string]interface{}, fileName string) (*GeneratedFile, error) {
	// Konversi map ke columns + rows
	columns := make([]string, 0, len(eventData))
	row := make([]interface{}, 0, len(eventData))

	for k, v := range eventData {
		columns = append(columns, k)
		row = append(row, v)
	}

	data := [][]interface{}{row}
	return buildFile(intg.FileFormat, fileName, columns, data, intg)
}

// ─── Build File sesuai Format ─────────────────────────────────────────────────

func buildFile(format integration.FileFormat, fileName string, columns []string, data [][]interface{}, intg integration.Integration) (*GeneratedFile, error) {
	switch format {
	case integration.FormatCSV:
		b, err := buildCSV(columns, data)
		if err != nil {
			return nil, err
		}
		return &GeneratedFile{Name: fileName, Data: b}, nil

	case integration.FormatJSON:
		b, err := buildJSON(columns, data)
		if err != nil {
			return nil, err
		}
		return &GeneratedFile{Name: fileName, Data: b}, nil

	case integration.FormatTXT:
		b := buildTXT(columns, data)
		return &GeneratedFile{Name: fileName, Data: b}, nil

	case integration.FormatExcel:
		b, err := buildExcel(columns, data, intg)
		if err != nil {
			return nil, err
		}
		return &GeneratedFile{Name: fileName, Data: b}, nil

	default:
		return nil, fmt.Errorf("format tidak dikenal: %s", format)
	}
}

// ─── CSV ──────────────────────────────────────────────────────────────────────

func buildCSV(columns []string, data [][]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	if err := w.Write(columns); err != nil {
		return nil, err
	}

	// Rows
	for _, row := range data {
		record := make([]string, len(row))
		for i, v := range row {
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ─── JSON ─────────────────────────────────────────────────────────────────────

func buildJSON(columns []string, data [][]interface{}) ([]byte, error) {
	var records []map[string]interface{}
	for _, row := range data {
		record := make(map[string]interface{})
		for i, col := range columns {
			if i < len(row) {
				record[col] = row[i]
			}
		}
		records = append(records, record)
	}
	return json.MarshalIndent(records, "", "  ")
}

// ─── TXT (pipe-delimited) ─────────────────────────────────────────────────────

func buildTXT(columns []string, data [][]interface{}) []byte {
	var sb strings.Builder
	sb.WriteString(strings.Join(columns, "|") + "\n")
	for _, row := range data {
		parts := make([]string, len(row))
		for i, v := range row {
			if v == nil {
				parts[i] = ""
			} else {
				parts[i] = fmt.Sprintf("%v", v)
			}
		}
		sb.WriteString(strings.Join(parts, "|") + "\n")
	}
	return []byte(sb.String())
}

// ─── Excel ────────────────────────────────────────────────────────────────────

func buildExcel(columns []string, data [][]interface{}, intg integration.Integration) ([]byte, error) {
	// Buat report dummy untuk reuse excel_service
	report := report_mailer.Report{
		Name:          intg.Name,
		ExcelTitle:    intg.Name,
		ExcelSubtitle: intg.Description,
	}

	// Buat query dummy yang sudah di-preload datanya
	// Kita reuse GenerateSingleFileExcel tapi inject data manual
	// dengan cara buat temporary in-memory "table"
	// Karena excel_service butuh db + query, kita gunakan pendekatan langsung dengan excelize
	return buildExcelDirect(columns, data, intg.Name, intg.Description, report)
}

func buildExcelDirect(columns []string, data [][]interface{}, title, subtitle string, _ report_mailer.Report) ([]byte, error) {
	// Import excelize inline
	type styleConf struct {
		bold  bool
		bg    string
		color string
	}

	// Gunakan raw excelize
	import_excelize := func() interface{} { return nil }
	_ = import_excelize

	// Kita pakai buildCSV sebagai fallback Excel karena sudah ada excel_service
	// Untuk reuse penuh, panggil generateSheet dari excel_service langsung
	// Di sini kita return CSV bytes sebagai fallback sederhana
	// agar tidak duplicate kode excel styling
	// TODO: refactor excel_service agar bisa dipanggil dengan raw data (bukan db+query)
	_ = title
	_ = subtitle
	return buildCSV(columns, data) // temporary fallback ke CSV
}

// ─── Filename Resolver ────────────────────────────────────────────────────────

func resolveFilename(pattern, name string, format integration.FileFormat, data map[string]interface{}) string {
	now := time.Now()
	ext := string(format)
	if format == integration.FormatExcel {
		ext = "xlsx"
	}

	if pattern == "" {
		safe := strings.ReplaceAll(name, " ", "_")
		return fmt.Sprintf("%s_%s.%s", safe, now.Format("20060102_1504"), ext)
	}

	result := pattern
	for k, v := range data {
		result = strings.ReplaceAll(result, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	result = strings.ReplaceAll(result, "{{date}}", now.Format("20060102"))
	result = strings.ReplaceAll(result, "{{datetime}}", now.Format("20060102_1504"))
	result = strings.ReplaceAll(result, "{{year}}", now.Format("2006"))
	result = strings.ReplaceAll(result, "{{month}}", now.Format("01"))

	// Sanitize karakter tidak valid untuk nama file
	var safe strings.Builder
	for _, c := range result {
		if c == '/' || c == '\\' || c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|' {
			safe.WriteRune('_')
		} else {
			safe.WriteRune(c)
		}
	}

	return safe.String() + "." + ext
}
