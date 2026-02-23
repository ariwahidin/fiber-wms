package notification_service

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fiber-app/models/notification"
	rm_services "fiber-app/services/report_mailer"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ─── Generate Attachment ──────────────────────────────────────────────────────

// generateAttachment adalah entry point — pilih source event atau query
func generateAttachment(
	db *gorm.DB,
	notif notification.EmailNotification,
	data map[string]interface{},
) ([]byte, string, error) {
	// Resolve filename
	fileName := notif.AttachmentName
	if fileName == "" {
		fileName = strings.ReplaceAll(notif.Name, " ", "_") + "_{{date}}.xlsx"
	}
	fileName = resolvePlaceholders(fileName, data)
	if !strings.HasSuffix(strings.ToLower(fileName), ".xlsx") {
		fileName += ".xlsx"
	}

	switch notif.AttachmentSource {
	case "query":
		return generateFromQuery(db, notif, data, fileName)
	default:
		// "event" — pakai data dari event payload
		return generateFromEvent(notif, data, fileName)
	}
}

// ─── Source: Event ────────────────────────────────────────────────────────────

func generateFromEvent(
	notif notification.EmailNotification,
	data map[string]interface{},
	fileName string,
) ([]byte, string, error) {
	// Parse field yang dipilih
	var fields []string
	if notif.AttachmentFields != "" {
		if err := json.Unmarshal([]byte(notif.AttachmentFields), &fields); err != nil {
			return nil, "", fmt.Errorf("gagal parse attachment_fields: %w", err)
		}
	}
	if len(fields) == 0 {
		for k := range data {
			if k != "year" && k != "date" && k != "datetime" {
				fields = append(fields, k)
			}
		}
	}

	// Build columns + 1 row dari event data
	columns := fields
	rows := make([][]interface{}, 1)
	rows[0] = make([]interface{}, len(fields))
	for i, f := range fields {
		if v, ok := data[f]; ok && v != nil {
			switch v := v.(type) {
			case time.Time:
				rows[0][i] = v.Format("02 Jan 2006 15:04")
			default:
				rows[0][i] = fmt.Sprintf("%v", v)
			}
		} else {
			rows[0][i] = ""
		}
	}

	excelBytes, err := buildExcel(notif.Name, columns, rows)
	if err != nil {
		return nil, "", err
	}
	return excelBytes, fileName, nil
}

// ─── Source: Query ────────────────────────────────────────────────────────────

func generateFromQuery(
	db *gorm.DB,
	notif notification.EmailNotification,
	data map[string]interface{},
	fileName string,
) ([]byte, string, error) {
	if notif.AttachmentQuery == "" {
		return nil, "", fmt.Errorf("attachment_query kosong")
	}

	// ← Validasi SQL aman (reuse dari report_mailer)
	if err := rm_services.ValidateSelectOnly(notif.AttachmentQuery); err != nil {
		return nil, "", fmt.Errorf("query tidak aman: %w", err)
	}

	// Resolve placeholder di query — misal {{outbound_no}} → nilai dari event data
	resolvedQuery := resolvePlaceholders(notif.AttachmentQuery, data)

	// Jalankan query
	sqlRows, err := db.Raw(resolvedQuery).Rows()
	if err != nil {
		return nil, "", fmt.Errorf("gagal jalankan query: %w", err)
	}
	defer sqlRows.Close()

	// Baca columns dari hasil query
	columns, err := sqlRows.Columns()
	if err != nil {
		return nil, "", fmt.Errorf("gagal baca kolom: %w", err)
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
			return nil, "", fmt.Errorf("gagal scan row: %w", err)
		}

		row := make([]interface{}, len(values))
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				row[i] = string(val)
			case time.Time:
				row[i] = val.Format("02 Jan 2006 15:04")
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

	if len(rows) == 0 {
		return nil, "", fmt.Errorf("query tidak menghasilkan data")
	}

	excelBytes, err := buildExcel(notif.Name, columns, rows)
	if err != nil {
		return nil, "", err
	}
	return excelBytes, fileName, nil
}

// ─── Build Excel ──────────────────────────────────────────────────────────────

func buildExcel(title string, columns []string, rows [][]interface{}) ([]byte, error) {
	f := excelize.NewFile()
	sheet := "Data"
	f.SetSheetName("Sheet1", sheet)

	// Style header
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1E3A5F"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})

	// Style data biasa
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "E5E7EB", Style: 1},
		},
	})

	// Style data alternating (baris genap)
	dataStyleAlt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F8FAFC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "E5E7EB", Style: 1},
		},
	})

	// Tulis header
	for col, colName := range columns {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		label := strings.ToUpper(strings.ReplaceAll(colName, "_", " "))
		f.SetCellValue(sheet, cell, label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)

		// Set lebar kolom
		colLetter, _ := excelize.ColumnNumberToName(col + 1)
		f.SetColWidth(sheet, colLetter, colLetter, 22)
	}
	f.SetRowHeight(sheet, 1, 24)

	// Tulis data rows
	for rowIdx, row := range rows {
		excelRow := rowIdx + 2 // row 1 = header
		style := dataStyle
		if rowIdx%2 == 1 {
			style = dataStyleAlt
		}

		for col, val := range row {
			cell, _ := excelize.CoordinatesToCellName(col+1, excelRow)
			f.SetCellValue(sheet, cell, val)
			f.SetCellStyle(sheet, cell, cell, style)
		}
		f.SetRowHeight(sheet, excelRow, 18)
	}

	// Freeze header
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Auto filter
	if len(rows) > 0 && len(columns) > 0 {
		lastCol, _ := excelize.ColumnNumberToName(len(columns))
		lastRow := len(rows) + 1
		f.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastCol, lastRow), nil)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("gagal export Excel: %w", err)
	}
	return buf.Bytes(), nil
}
