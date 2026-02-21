// package services

// import (
// 	"fiber-app/models/report_mailer"
// 	"fmt"
// 	"time"

// 	"github.com/xuri/excelize/v2"
// 	"gorm.io/gorm"
// )

// // GenerateReportExcel menjalankan query dari report, lalu generate file Excel.
// // Mengembalikan []byte siap attach ke email.
// func GenerateReportExcel(db *gorm.DB, report report_mailer.Report) ([]byte, error) {

// 	if err := ValidateSelectOnly(report.Query); err != nil {
// 		return nil, fmt.Errorf("query tidak aman: %w", err)
// 	}

// 	// ── 1. Jalankan query ──────────────────────────────────────────────────────
// 	rows, err := db.Raw(report.Query).Rows()
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal menjalankan query: %w", err)
// 	}
// 	defer rows.Close()

// 	// Ambil nama kolom dari hasil query
// 	columns, err := rows.Columns()
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal membaca kolom: %w", err)
// 	}

// 	// Kumpulkan semua baris data
// 	var data [][]interface{}
// 	for rows.Next() {
// 		values := make([]interface{}, len(columns))
// 		valuePtrs := make([]interface{}, len(columns))
// 		for i := range values {
// 			valuePtrs[i] = &values[i]
// 		}
// 		if err := rows.Scan(valuePtrs...); err != nil {
// 			return nil, fmt.Errorf("gagal scan row: %w", err)
// 		}
// 		// Convert []byte ke string supaya tidak muncul sebagai base64 di Excel
// 		row := make([]interface{}, len(values))
// 		for i, v := range values {
// 			if b, ok := v.([]byte); ok {
// 				row[i] = string(b)
// 			} else {
// 				row[i] = v
// 			}
// 		}
// 		data = append(data, row)
// 	}

// 	// ── 2. Buat file Excel ─────────────────────────────────────────────────────
// 	f := excelize.NewFile()
// 	defer f.Close()

// 	sheet := "Report"
// 	f.NewSheet(sheet)
// 	f.DeleteSheet("Sheet1")

// 	// ── 3. Style ───────────────────────────────────────────────────────────────

// 	// Style Title (bold, font besar)
// 	titleStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{
// 			Bold: true,
// 			Size: 14,
// 		},
// 	})

// 	// Style Subtitle (italic, abu)
// 	subtitleStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{
// 			Italic: true,
// 			Size:   10,
// 			Color:  "666666",
// 		},
// 	})

// 	// Style Generated date (kecil, abu)
// 	dateStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{
// 			Size:  9,
// 			Color: "999999",
// 		},
// 	})

// 	// Style Header kolom (bold, background biru tua, teks putih)
// 	headerStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{
// 			Bold:  true,
// 			Size:  10,
// 			Color: "FFFFFF",
// 		},
// 		Fill: excelize.Fill{
// 			Type:    "pattern",
// 			Color:   []string{"1E40AF"},
// 			Pattern: 1,
// 		},
// 		Alignment: &excelize.Alignment{
// 			Horizontal: "center",
// 			Vertical:   "center",
// 			WrapText:   true,
// 		},
// 		Border: []excelize.Border{
// 			{Type: "left", Color: "FFFFFF", Style: 1},
// 			{Type: "right", Color: "FFFFFF", Style: 1},
// 		},
// 	})

// 	// Style data row genap (background putih)
// 	evenRowStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{Size: 9},
// 		Alignment: &excelize.Alignment{
// 			Vertical: "center",
// 		},
// 		Border: []excelize.Border{
// 			{Type: "bottom", Color: "E5E7EB", Style: 1},
// 		},
// 	})

// 	// Style data row ganjil (background abu sangat muda)
// 	oddRowStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{Size: 9},
// 		Fill: excelize.Fill{
// 			Type:    "pattern",
// 			Color:   []string{"F9FAFB"},
// 			Pattern: 1,
// 		},
// 		Alignment: &excelize.Alignment{
// 			Vertical: "center",
// 		},
// 		Border: []excelize.Border{
// 			{Type: "bottom", Color: "E5E7EB", Style: 1},
// 		},
// 	})

// 	// ── 4. Tulis header info (A1, A2, A3) ─────────────────────────────────────

// 	// A1 - Title
// 	f.SetCellValue(sheet, "A1", report.ExcelTitle)
// 	f.SetCellStyle(sheet, "A1", "A1", titleStyle)
// 	f.SetRowHeight(sheet, 1, 22)

// 	// A2 - Subtitle (hanya kalau ada)
// 	if report.ExcelSubtitle != "" {
// 		f.SetCellValue(sheet, "A2", report.ExcelSubtitle)
// 		f.SetCellStyle(sheet, "A2", "A2", subtitleStyle)
// 	}
// 	f.SetRowHeight(sheet, 2, 16)

// 	// A3 - Generated date
// 	generatedText := fmt.Sprintf("Generated: %s", time.Now().Format("02 January 2006, 15:04"))
// 	f.SetCellValue(sheet, "A3", generatedText)
// 	f.SetCellStyle(sheet, "A3", "A3", dateStyle)
// 	f.SetRowHeight(sheet, 3, 14)

// 	// A4 - baris kosong sebagai pemisah
// 	f.SetRowHeight(sheet, 4, 8)

// 	// ── 5. Tulis header kolom (baris 5) ───────────────────────────────────────

// 	headerRow := 5
// 	for i, col := range columns {
// 		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
// 		f.SetCellValue(sheet, cell, col)
// 		f.SetCellStyle(sheet, cell, cell, headerStyle)
// 	}
// 	f.SetRowHeight(sheet, headerRow, 22)

// 	// ── 6. Tulis data ──────────────────────────────────────────────────────────

// 	for rowIdx, row := range data {
// 		excelRow := headerRow + 1 + rowIdx
// 		style := evenRowStyle
// 		if rowIdx%2 == 0 {
// 			style = oddRowStyle
// 		}
// 		for colIdx, val := range row {
// 			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
// 			f.SetCellValue(sheet, cell, val)
// 			f.SetCellStyle(sheet, cell, cell, style)
// 		}
// 		f.SetRowHeight(sheet, excelRow, 18)
// 	}

// 	// ── 7. Auto width kolom ────────────────────────────────────────────────────

// 	for i, col := range columns {
// 		colName, _ := excelize.ColumnNumberToName(i + 1)

// 		// Hitung lebar berdasarkan nama kolom + sample data
// 		maxLen := len(col)
// 		for _, row := range data {
// 			if i < len(row) {
// 				cellStr := fmt.Sprintf("%v", row[i])
// 				if len(cellStr) > maxLen {
// 					maxLen = len(cellStr)
// 				}
// 			}
// 		}

// 		// Clamp: min 10, max 50
// 		width := float64(maxLen) + 2
// 		if width < 10 {
// 			width = 10
// 		}
// 		if width > 50 {
// 			width = 50
// 		}
// 		f.SetColWidth(sheet, colName, colName, width)
// 	}

// 	// ── 8. Freeze header row ───────────────────────────────────────────────────
// 	f.SetPanes(sheet, &excelize.Panes{
// 		Freeze:      true,
// 		Split:       false,
// 		XSplit:      0,
// 		YSplit:      headerRow, // freeze sampai baris header
// 		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
// 		ActivePane:  "bottomLeft",
// 	})

// 	// ── 9. Set properties file ─────────────────────────────────────────────────
// 	f.SetDocProps(&excelize.DocProperties{
// 		Title:   report.ExcelTitle,
// 		Creator: "WMS Report Mailer",
// 	})

// 	// ── 10. Tulis ke buffer ────────────────────────────────────────────────────
// 	buf, err := f.WriteToBuffer()
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal menulis Excel ke buffer: %w", err)
// 	}

// 	return buf.Bytes(), nil
// }

// // ExcelFileName menghasilkan nama file yang rapi untuk attachment email
// func ExcelFileName(reportName string) string {
// 	t := time.Now()
// 	// Bersihkan karakter yang tidak aman untuk nama file
// 	safe := ""
// 	for _, c := range reportName {
// 		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
// 			(c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' {
// 			safe += string(c)
// 		} else {
// 			safe += "_"
// 		}
// 	}
// 	return fmt.Sprintf("%s_%s.xlsx", safe, t.Format("20060102_1504"))
// }

package services

import (
	"fiber-app/models/report_mailer"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ─── Single Sheet (1 query → 1 sheet) ────────────────────────────────────────

// generateSheet menulis 1 query ke 1 sheet dalam file Excel yang sudah ada.
func generateSheet(f *excelize.File, db *gorm.DB, report report_mailer.Report, rq report_mailer.ReportQuery, sheetName string) error {
	f.NewSheet(sheetName)

	// ── Style ──────────────────────────────────────────────────────────────────
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})
	subtitleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Italic: true, Size: 10, Color: "666666"},
	})
	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 9, Color: "999999"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1E40AF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
		},
	})
	oddRowStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F9FAFB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}},
	})
	evenRowStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}},
	})

	// ── Jalankan query ─────────────────────────────────────────────────────────
	rows, err := db.Raw(rq.Query).Rows()
	if err != nil {
		return fmt.Errorf("gagal jalankan query '%s': %w", rq.Name, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("gagal baca kolom '%s': %w", rq.Name, err)
	}

	var data [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("gagal scan row: %w", err)
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

	// ── Tulis header info ──────────────────────────────────────────────────────
	f.SetCellValue(sheetName, "A1", report.ExcelTitle)
	f.SetCellStyle(sheetName, "A1", "A1", titleStyle)
	f.SetRowHeight(sheetName, 1, 22)

	if report.ExcelSubtitle != "" {
		f.SetCellValue(sheetName, "A2", report.ExcelSubtitle)
		f.SetCellStyle(sheetName, "A2", "A2", subtitleStyle)
	}
	f.SetRowHeight(sheetName, 2, 16)

	generatedText := fmt.Sprintf("Generated: %s", time.Now().Format("02 January 2006, 15:04"))
	f.SetCellValue(sheetName, "A3", generatedText)
	f.SetCellStyle(sheetName, "A3", "A3", dateStyle)
	f.SetRowHeight(sheetName, 3, 14)
	f.SetRowHeight(sheetName, 4, 8)

	// ── Tulis header kolom ─────────────────────────────────────────────────────
	headerRow := 5
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		f.SetCellValue(sheetName, cell, col)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}
	f.SetRowHeight(sheetName, headerRow, 22)

	// ── Tulis data ─────────────────────────────────────────────────────────────
	for rowIdx, row := range data {
		excelRow := headerRow + 1 + rowIdx
		style := evenRowStyle
		if rowIdx%2 == 0 {
			style = oddRowStyle
		}
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			f.SetCellValue(sheetName, cell, val)
			f.SetCellStyle(sheetName, cell, cell, style)
		}
		f.SetRowHeight(sheetName, excelRow, 18)
	}

	// ── Auto width ─────────────────────────────────────────────────────────────
	for i, col := range columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		maxLen := len(col)
		for _, row := range data {
			if i < len(row) {
				s := fmt.Sprintf("%v", row[i])
				if len(s) > maxLen {
					maxLen = len(s)
				}
			}
		}
		width := float64(maxLen) + 2
		if width < 10 {
			width = 10
		}
		if width > 50 {
			width = 50
		}
		f.SetColWidth(sheetName, colName, colName, width)
	}

	// ── Freeze header ──────────────────────────────────────────────────────────
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
		ActivePane:  "bottomLeft",
	})

	return nil
}

// ─── Single File Multi Sheet ──────────────────────────────────────────────────

// GenerateSingleFileExcel generate 1 Excel dengan banyak sheet (1 sheet per query).
// func GenerateSingleFileExcel(db *gorm.DB, report report_mailer.Report) ([]byte, error) {
// 	if len(report.Queries) == 0 {
// 		return nil, fmt.Errorf("report tidak memiliki query")
// 	}

// 	f := excelize.NewFile()
// 	defer f.Close()

// 	// Hapus sheet default
// 	f.DeleteSheet("Sheet1")

// 	for _, rq := range report.Queries {
// 		// Nama sheet dibatasi 31 karakter (limit Excel)
// 		sheetName := rq.Name
// 		if len(sheetName) > 31 {
// 			sheetName = sheetName[:31]
// 		}
// 		if err := generateSheet(f, db, report, rq, sheetName); err != nil {
// 			return nil, err
// 		}

// 	}

// 	// Set active sheet ke yang pertama
// 	firstSheet := report.Queries[0].Name
// 	if len(firstSheet) > 31 {
// 		firstSheet = firstSheet[:31]
// 	}
// 	// f.SetActiveSheet(f.GetSheetIndex(firstSheet))
// 	sheetIndex, err := f.GetSheetIndex(firstSheet)
// 	if err == nil {
// 		f.SetActiveSheet(sheetIndex)
// 	}

// 	f.SetDocProps(&excelize.DocProperties{
// 		Title:   report.ExcelTitle,
// 		Creator: "WMS Report Mailer",
// 	})

// 	buf, err := f.WriteToBuffer()
// 	if err != nil {
// 		return nil, fmt.Errorf("gagal tulis Excel ke buffer: %w", err)
// 	}
// 	return buf.Bytes(), nil
// }

func GenerateSingleFileExcel(db *gorm.DB, report report_mailer.Report) ([]byte, error) {
	if len(report.Queries) == 0 {
		return nil, fmt.Errorf("report tidak memiliki query")
	}

	f := excelize.NewFile()
	defer f.Close()

	for i, rq := range report.Queries {
		// Nama sheet dibatasi 31 karakter (limit Excel)
		sheetName := rq.Name
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}

		if err := generateSheet(f, db, report, rq, sheetName); err != nil {
			return nil, err
		}

		// Hapus Sheet1 setelah sheet pertama kita berhasil dibuat
		// supaya excelize tidak complain "tidak ada sheet aktif"
		if i == 0 {
			f.DeleteSheet("Sheet1")
		}
	}

	// Set active sheet ke yang pertama
	firstSheet := report.Queries[0].Name
	if len(firstSheet) > 31 {
		firstSheet = firstSheet[:31]
	}
	sheetIndex, err := f.GetSheetIndex(firstSheet)
	if err == nil {
		f.SetActiveSheet(sheetIndex)
	}

	f.SetDocProps(&excelize.DocProperties{
		Title:   report.ExcelTitle,
		Creator: "WMS Report Mailer",
	})

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("gagal tulis Excel ke buffer: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── Multi File ───────────────────────────────────────────────────────────────

// ExcelFile representasi 1 file Excel (nama + bytes)
type ExcelFile struct {
	FileName string
	Data     []byte
}

// GenerateMultiFileExcel generate banyak Excel (1 file per query).
func GenerateMultiFileExcel(db *gorm.DB, report report_mailer.Report) ([]ExcelFile, error) {
	if len(report.Queries) == 0 {
		return nil, fmt.Errorf("report tidak memiliki query")
	}

	var files []ExcelFile

	for _, rq := range report.Queries {
		f := excelize.NewFile()
		f.DeleteSheet("Sheet1")

		if err := generateSheet(f, db, report, rq, rq.Name); err != nil {
			f.Close()
			return nil, err
		}

		f.SetDocProps(&excelize.DocProperties{
			Title:   rq.Name,
			Creator: "WMS Report Mailer",
		})

		buf, err := f.WriteToBuffer()
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("gagal tulis Excel '%s': %w", rq.Name, err)
		}

		files = append(files, ExcelFile{
			FileName: ExcelFileName(rq.Name),
			Data:     buf.Bytes(),
		})
	}

	return files, nil
}

// ─── Backward Compat: single query (dipanggil dari preview) ──────────────────

// GenerateReportExcel tetap ada untuk kompatibilitas preview query.
// Untuk pengiriman email, gunakan GenerateSingleFileExcel / GenerateMultiFileExcel.
func GenerateReportExcel(db *gorm.DB, report report_mailer.Report) ([]byte, error) {
	return GenerateSingleFileExcel(db, report)
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func ExcelFileName(name string) string {
	t := time.Now()
	safe := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' {
			safe += string(c)
		} else {
			safe += "_"
		}
	}
	return fmt.Sprintf("%s_%s.xlsx", safe, t.Format("20060102_1504"))
}
