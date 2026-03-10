package report_builder

import (
	"fiber-app/models/report_builder"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// GenerateExcel membuat file Excel dari data hasil query.
// Kolom diambil dari layout.Columns (sudah di-sort by column_order).
func GenerateExcel(
	layout report_builder.RptLayout,
	rows []map[string]interface{},
) (*excelize.File, error) {

	f := excelize.NewFile()
	sheetName := sanitizeSheetName(layout.Report.ReportName)
	f.SetSheetName("Sheet1", sheetName)

	// ── Style ────────────────────────────────────────────────────────────
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F5496"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
			{Type: "bottom", Color: "FFFFFF", Style: 1},
		},
	})

	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	numberStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		NumFmt:    4, // #,##0.00
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	// ── Title row ────────────────────────────────────────────────────────
	titleRow := 1
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", titleRow), layout.Report.ReportName)
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13},
	})
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", titleRow), fmt.Sprintf("A%d", titleRow), titleStyle)

	// Generated at
	genRow := 2
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", genRow),
		fmt.Sprintf("Generated: %s", time.Now().Format("02 Jan 2006 15:04")))

	// ── Header columns ───────────────────────────────────────────────────
	headerRow := 4
	visibleCols := getVisibleColumns(layout.Columns)

	for i, col := range visibleCols {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		label := col.ColumnLabel
		if label == "" {
			label = col.Field.FieldLabel
		}
		f.SetCellValue(sheetName, cell, strings.ToUpper(label))
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
		f.SetColWidth(sheetName, colLetter(i+1), colLetter(i+1), float64(col.ColumnWidth)/7.5)
	}
	f.SetRowHeight(sheetName, headerRow, 20)

	// ── Data rows ────────────────────────────────────────────────────────
	for rowIdx, row := range rows {
		excelRow := headerRow + 1 + rowIdx
		for colIdx, col := range visibleCols {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			val := row[col.Field.FieldKey]

			if col.Field.FieldType == "NUMBER" {
				f.SetCellValue(sheetName, cell, toFloat(val))
				f.SetCellStyle(sheetName, cell, cell, numberStyle)
			} else {
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", nullStr(val)))
				f.SetCellStyle(sheetName, cell, cell, dataStyle)
			}
		}
		f.SetRowHeight(sheetName, excelRow, 15)
	}

	// ── Freeze header ────────────────────────────────────────────────────
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
		ActivePane:  "bottomLeft",
	})

	// ── Summary row ──────────────────────────────────────────────────────
	summaryRow := headerRow + 1 + len(rows)
	summaryStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 9},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", summaryRow),
		fmt.Sprintf("Total: %d rows", len(rows)))
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", summaryRow),
		fmt.Sprintf("A%d", summaryRow), summaryStyle)

	return f, nil
}

// GenerateCSV membuat CSV string dari data hasil query.
func GenerateCSV(
	layout report_builder.RptLayout,
	rows []map[string]interface{},
) string {
	var sb strings.Builder
	visibleCols := getVisibleColumns(layout.Columns)

	// Header
	headers := make([]string, len(visibleCols))
	for i, col := range visibleCols {
		label := col.ColumnLabel
		if label == "" {
			label = col.Field.FieldLabel
		}
		headers[i] = escapeCSV(label)
	}
	sb.WriteString(strings.Join(headers, ",") + "\n")

	// Data
	for _, row := range rows {
		vals := make([]string, len(visibleCols))
		for i, col := range visibleCols {
			vals[i] = escapeCSV(fmt.Sprintf("%v", nullStr(row[col.Field.FieldKey])))
		}
		sb.WriteString(strings.Join(vals, ",") + "\n")
	}

	return sb.String()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func getVisibleColumns(cols []report_builder.RptLayoutColumn) []report_builder.RptLayoutColumn {
	var visible []report_builder.RptLayoutColumn
	for _, c := range cols {
		if c.IsVisible {
			visible = append(visible, c)
		}
	}
	return visible
}

func sanitizeSheetName(name string) string {
	replacer := strings.NewReplacer(":", "", "\\", "", "/", "", "?", "", "*", "", "[", "", "]", "")
	name = replacer.Replace(name)
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

func colLetter(col int) string {
	letter := ""
	for col > 0 {
		col--
		letter = string(rune('A'+col%26)) + letter
		col /= 26
	}
	return letter
}

func nullStr(v interface{}) interface{} {
	if v == nil {
		return ""
	}
	return v
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return `"` + s + `"`
	}
	return s
}
