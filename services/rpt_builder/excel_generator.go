package rpt_builder_service

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type SheetData struct {
	SheetName string
	Columns   []ColumnConfig
	Rows      []map[string]interface{}
}

type ColumnConfig struct {
	ColumnKey   string
	DisplayName string
	ExcelWidth  int
	ExcelFormat string // TEXT | NUMBER | DATE
	IsVisible   bool
	ColumnOrder int
}

// GenerateExcel membuat file Excel multi-sheet.
// Setiap SheetData menjadi satu sheet di workbook yang sama.
func GenerateExcel(templateName string, sheets []SheetData) (*excelize.File, error) {
	f := excelize.NewFile()

	// Hapus Sheet1 default setelah semua sheet dibuat
	defaultSheetRemoved := false

	for i, sheet := range sheets {
		sheetName := sanitizeSheetName(sheet.SheetName)
		if sheetName == "" {
			sheetName = fmt.Sprintf("Sheet%d", i+1)
		}

		if i == 0 {
			// Rename Sheet1 default ke nama sheet pertama
			f.SetSheetName("Sheet1", sheetName)
			defaultSheetRemoved = true
		} else {
			f.NewSheet(sheetName)
		}

		if err := writeSheet(f, sheetName, templateName, sheet); err != nil {
			return nil, fmt.Errorf("error writing sheet '%s': %w", sheetName, err)
		}
	}

	if !defaultSheetRemoved {
		f.DeleteSheet("Sheet1")
	}

	return f, nil
}

func writeSheet(f *excelize.File, sheetName string, templateName string, sheet SheetData) error {
	// ── Styles ───────────────────────────────────────────────────────────
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
		NumFmt:    4,
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Alignment: &excelize.Alignment{Vertical: "center"},
		NumFmt:    14, // dd/mm/yyyy
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13},
	})

	summaryStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 9},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})

	// ── Title & meta ─────────────────────────────────────────────────────
	f.SetCellValue(sheetName, "A1", templateName)
	f.SetCellStyle(sheetName, "A1", "A1", titleStyle)
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("Sheet: %s", sheet.SheetName))
	f.SetCellValue(sheetName, "A3", fmt.Sprintf("Generated: %s", time.Now().Format("02 Jan 2006 15:04")))

	// ── Filter visible columns & sort by order ───────────────────────────
	visibleCols := getVisibleColumns(sheet.Columns)

	// ── Header row (row 5) ───────────────────────────────────────────────
	headerRow := 5
	for i, col := range visibleCols {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		f.SetCellValue(sheetName, cell, strings.ToUpper(col.DisplayName))
		f.SetCellStyle(sheetName, cell, cell, headerStyle)

		colLetter := toColLetter(i + 1)
		width := float64(col.ExcelWidth) / 7.5
		if width < 8 {
			width = 8
		}
		f.SetColWidth(sheetName, colLetter, colLetter, width)
	}
	f.SetRowHeight(sheetName, headerRow, 20)

	// ── Data rows ────────────────────────────────────────────────────────
	for rowIdx, row := range sheet.Rows {
		excelRow := headerRow + 1 + rowIdx
		for colIdx, col := range visibleCols {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			val := row[col.ColumnKey]

			switch strings.ToUpper(col.ExcelFormat) {
			case "NUMBER":
				f.SetCellValue(sheetName, cell, toFloat(val))
				f.SetCellStyle(sheetName, cell, cell, numberStyle)
			case "DATE":
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", nullStr(val)))
				f.SetCellStyle(sheetName, cell, cell, dateStyle)
			default:
				f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", nullStr(val)))
				f.SetCellStyle(sheetName, cell, cell, dataStyle)
			}
		}
		f.SetRowHeight(sheetName, excelRow, 15)
	}

	// ── Freeze header ────────────────────────────────────────────────────
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		XSplit:      0,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
		ActivePane:  "bottomLeft",
	})

	// ── Summary row ──────────────────────────────────────────────────────
	summaryRow := headerRow + 1 + len(sheet.Rows)
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", summaryRow),
		fmt.Sprintf("Total: %d rows", len(sheet.Rows)))
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", summaryRow),
		fmt.Sprintf("A%d", summaryRow), summaryStyle)

	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func getVisibleColumns(cols []ColumnConfig) []ColumnConfig {
	var visible []ColumnConfig
	for _, c := range cols {
		if c.IsVisible {
			visible = append(visible, c)
		}
	}
	// sort by ColumnOrder
	for i := 0; i < len(visible)-1; i++ {
		for j := i + 1; j < len(visible); j++ {
			if visible[j].ColumnOrder < visible[i].ColumnOrder {
				visible[i], visible[j] = visible[j], visible[i]
			}
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

func toColLetter(col int) string {
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
