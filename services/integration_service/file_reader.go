package integration_service

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ParsedRow representasi 1 baris data dari file
type ParsedRow map[string]interface{}

// ReadFile membaca file dan kembalikan slice of ParsedRow
func ReadFile(data []byte, format string, filename string) ([]ParsedRow, error) {
	switch strings.ToLower(format) {
	case "csv":
		return readCSV(data)
	case "txt":
		return readTXT(data)
	case "excel", "xlsx", "xls":
		return readExcel(data)
	case "json":
		return readJSON(data)
	case "xml":
		return readXML(data)
	default:
		// Auto-detect dari ekstensi filename
		ext := strings.ToLower(filename)
		switch {
		case strings.HasSuffix(ext, ".csv"):
			return readCSV(data)
		case strings.HasSuffix(ext, ".txt"):
			return readTXT(data)
		case strings.HasSuffix(ext, ".xlsx") || strings.HasSuffix(ext, ".xls"):
			return readExcel(data)
		case strings.HasSuffix(ext, ".json"):
			return readJSON(data)
		case strings.HasSuffix(ext, ".xml"):
			return readXML(data)
		default:
			return nil, fmt.Errorf("format file tidak dikenali: %s", filename)
		}
	}
}

// ─── CSV ──────────────────────────────────────────────────────────────────────

func readCSV(data []byte) ([]ParsedRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("gagal baca CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV harus memiliki minimal 1 baris header dan 1 baris data")
	}

	headers := records[0]
	var rows []ParsedRow
	for i := 1; i < len(records); i++ {
		row := make(ParsedRow)
		for j, h := range headers {
			if j < len(records[i]) {
				row[strings.TrimSpace(h)] = strings.TrimSpace(records[i][j])
			} else {
				row[strings.TrimSpace(h)] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ─── TXT (pipe / tab / delimiter otomatis) ────────────────────────────────────

func readTXT(data []byte) ([]ParsedRow, error) {
	content := string(data)
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	// Hapus baris kosong
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) < 2 {
		return nil, fmt.Errorf("TXT harus memiliki minimal 1 baris header dan 1 baris data")
	}

	// Auto-detect delimiter: | atau \t atau ,
	delimiter := detectDelimiter(nonEmpty[0])
	headers := strings.Split(nonEmpty[0], delimiter)
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	var rows []ParsedRow
	for i := 1; i < len(nonEmpty); i++ {
		parts := strings.Split(nonEmpty[i], delimiter)
		row := make(ParsedRow)
		for j, h := range headers {
			if j < len(parts) {
				row[h] = strings.TrimSpace(parts[j])
			} else {
				row[h] = ""
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func detectDelimiter(line string) string {
	counts := map[string]int{
		"|":  strings.Count(line, "|"),
		"\t": strings.Count(line, "\t"),
		";":  strings.Count(line, ";"),
		",":  strings.Count(line, ","),
	}
	max := 0
	delim := ","
	for d, c := range counts {
		if c > max {
			max = c
			delim = d
		}
	}
	return delim
}

// ─── Excel ────────────────────────────────────────────────────────────────────

func readExcel(data []byte) ([]ParsedRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gagal buka Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel tidak memiliki sheet")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("gagal baca rows Excel: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel harus memiliki minimal 1 baris header dan 1 baris data")
	}

	headers := rows[0]
	var result []ParsedRow
	for i := 1; i < len(rows); i++ {
		row := make(ParsedRow)
		empty := true
		for j, h := range headers {
			val := ""
			if j < len(rows[i]) {
				val = strings.TrimSpace(rows[i][j])
			}
			if val != "" {
				empty = false
			}
			row[strings.TrimSpace(h)] = val
		}
		if !empty {
			result = append(result, row)
		}
	}
	return result, nil
}

// ─── JSON ─────────────────────────────────────────────────────────────────────

func readJSON(data []byte) ([]ParsedRow, error) {
	// Coba parse sebagai array
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		rows := make([]ParsedRow, len(arr))
		for i, m := range arr {
			rows[i] = ParsedRow(m)
		}
		return rows, nil
	}

	// Coba parse sebagai single object
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		return []ParsedRow{ParsedRow(obj)}, nil
	}

	return nil, fmt.Errorf("gagal parse JSON: bukan array atau object yang valid")
}

// ─── XML (flat) ───────────────────────────────────────────────────────────────

func readXML(data []byte) ([]ParsedRow, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	var rows []ParsedRow
	var currentRow ParsedRow
	var currentKey string
	var depth int

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gagal parse XML: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				// Level 2 = record/row element
				currentRow = make(ParsedRow)
			} else if depth == 3 {
				// Level 3 = field element
				currentKey = t.Name.Local
			}
		case xml.EndElement:
			if depth == 2 && currentRow != nil {
				rows = append(rows, currentRow)
				currentRow = nil
			}
			depth--
		case xml.CharData:
			if depth == 3 && currentKey != "" && currentRow != nil {
				val := strings.TrimSpace(string(t))
				if val != "" {
					currentRow[currentKey] = val
				}
			}
		}
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("tidak ada data ditemukan di XML")
	}
	return rows, nil
}

// ─── Auto-detect headers dari file ───────────────────────────────────────────

// DetectHeaders membaca file dan kembalikan daftar kolom yang tersedia
func DetectHeaders(data []byte, format string, filename string) ([]string, error) {
	rows, err := ReadFile(data, format, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("file kosong")
	}
	// Ambil keys dari baris pertama
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}
	return headers, nil
}
