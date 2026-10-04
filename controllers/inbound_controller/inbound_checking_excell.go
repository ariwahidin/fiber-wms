package inbound_controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"fiber-app/models"
	"fiber-app/repositories"
)

func (c *InboundController) GetCheckingInboundOptions(ctx *fiber.Ctx) error {

	// ============================================================
	// QUERY
	// ============================================================

	search := strings.TrimSpace(
		ctx.Query("search"),
	)

	// ============================================================
	// LIMIT
	// ============================================================

	limit := 20

	if limitQuery := ctx.Query("limit"); limitQuery != "" {

		if parsed, err := strconv.Atoi(limitQuery); err == nil {

			if parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
	}

	// ============================================================
	// QUERY BUILDER
	// ============================================================

	query := c.DB.
		Model(&models.InboundHeader{}).
		Where(
			"LOWER(status) = ?",
			"checking",
		)

	// ============================================================
	// SEARCH
	// ============================================================

	if search != "" {

		searchLike := "%" +
			strings.ToLower(search) +
			"%"

		query = query.Where(
			`(
				LOWER(receipt_id) LIKE ?
				OR LOWER(inbound_no) LIKE ?
				OR LOWER(supplier) LIKE ?
				OR LOWER(owner_code) LIKE ?
			)`,
			searchLike,
			searchLike,
			searchLike,
			searchLike,
		)
	}

	// ============================================================
	// GET DATA
	// ============================================================

	var inbounds []models.InboundHeader

	if err := query.
		Select(`
			id,
			inbound_no,
			receipt_id,
			inbound_date,
			owner_code,
			whs_code,
			supplier,
			status
		`).
		Order("id DESC").
		Limit(limit).
		Find(&inbounds).Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// RESPONSE
	// ============================================================

	type CheckingInboundOption struct {
		ID          uint   `json:"id"`
		InboundNo   string `json:"inbound_no"`
		ReceiptID   string `json:"receipt_id"`
		InboundDate string `json:"inbound_date"`
		OwnerCode   string `json:"owner_code"`
		WhsCode     string `json:"whs_code"`
		Supplier    string `json:"supplier"`
		Status      string `json:"status"`
	}

	result := make(
		[]CheckingInboundOption,
		0,
		len(inbounds),
	)

	for _, inbound := range inbounds {

		result = append(
			result,
			CheckingInboundOption{
				ID:          inbound.ID,
				InboundNo:   inbound.InboundNo,
				ReceiptID:   inbound.ReceiptID,
				InboundDate: inbound.InboundDate,
				OwnerCode:   inbound.OwnerCode,
				WhsCode:     inbound.WhsCode,
				Supplier:    inbound.Supplier,
				Status:      inbound.Status,
			},
		)
	}

	return ctx.Status(
		fiber.StatusOK,
	).JSON(fiber.Map{
		"success": true,
		"items":   result,
	})
}

func (c *InboundController) DownloadCheckingTemplate(ctx *fiber.Ctx) error {

	// ============================================================
	// GET INBOUND NO
	// ============================================================

	inboundNo := ctx.Params("inbound_no")

	if inboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Inbound No is required",
		})
	}

	// ============================================================
	// GET INBOUND HEADER
	// ============================================================

	var inboundHeader models.InboundHeader

	if err := c.DB.
		Where("inbound_no = ?", inboundNo).
		First(&inboundHeader).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Inbound not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// GET INBOUND DETAILS
	// ============================================================

	var inboundDetails []models.InboundDetail

	if err := c.DB.
		Where("inbound_id = ?", inboundHeader.ID).
		Order("id ASC").
		Find(&inboundDetails).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(inboundDetails) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Inbound detail not found",
		})
	}

	// ============================================================
	// GET PRODUCTS
	// ============================================================

	// Ambil semua ItemId terlebih dahulu supaya tidak query
	// Product satu per satu di dalam loop.
	itemIDs := make([]uint, 0, len(inboundDetails))

	itemIDMap := make(map[uint]bool)

	for _, detail := range inboundDetails {

		if detail.ItemId == 0 {
			continue
		}

		if !itemIDMap[detail.ItemId] {
			itemIDs = append(itemIDs, detail.ItemId)
			itemIDMap[detail.ItemId] = true
		}
	}

	// Map Product berdasarkan ID
	productMap := make(map[uint]models.Product)

	if len(itemIDs) > 0 {

		var products []models.Product

		if err := c.DB.
			Where("id IN ?", itemIDs).
			Find(&products).Error; err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		for _, product := range products {
			productMap[product.ID] = product
		}
	}

	// ============================================================
	// CREATE EXCEL
	// ============================================================

	f := excelize.NewFile()

	sheetName := "Checking"

	defaultSheet := f.GetSheetName(0)

	if defaultSheet != sheetName {
		f.SetSheetName(defaultSheet, sheetName)
	}

	// ============================================================
	// EXCEL HEADER
	// ============================================================

	headers := []string{
		"No",
		"Item Code",
		"Item Name",
		"Unit Model",
		"Barcode",
		"Serial Number",
		"Qty Plan",
		"Qty Received",
		"Case No",
		"Carton No",
		"Location",
	}

	for colIndex, header := range headers {

		cell, err := excelize.CoordinatesToCellName(
			colIndex+1,
			1,
		)

		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		if err := f.SetCellValue(
			sheetName,
			cell,
			header,
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	// ============================================================
	// HEADER STYLE
	// ============================================================

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"D9EAF7"},
		},
		Border: []excelize.Border{
			{
				Type:  "left",
				Color: "B7B7B7",
				Style: 1,
			},
			{
				Type:  "right",
				Color: "B7B7B7",
				Style: 1,
			},
			{
				Type:  "top",
				Color: "B7B7B7",
				Style: 1,
			},
			{
				Type:  "bottom",
				Color: "B7B7B7",
				Style: 1,
			},
		},
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if err := f.SetCellStyle(
		sheetName,
		"A1",
		"K1",
		headerStyle,
	); err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// DATA STYLE
	// ============================================================

	dataStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
		Border: []excelize.Border{
			{
				Type:  "left",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "right",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "top",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "bottom",
				Color: "D9D9D9",
				Style: 1,
			},
		},
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// INPUT COLUMN STYLE
	// ============================================================
	//
	// Kolom yang akan diisi user:
	//
	// H = Qty Received
	// I = Case No
	// J = Carton No
	// K = Location
	//

	inputStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"FFF2CC"},
		},
		Border: []excelize.Border{
			{
				Type:  "left",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "right",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "top",
				Color: "D9D9D9",
				Style: 1,
			},
			{
				Type:  "bottom",
				Color: "D9D9D9",
				Style: 1,
			},
		},
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// LOOP DETAIL
	// ============================================================

	row := 2
	no := 1

	for _, detail := range inboundDetails {

		// ========================================================
		// GET PRODUCT
		// ========================================================

		product := models.Product{}

		if detail.ItemId != 0 {

			if p, ok := productMap[detail.ItemId]; ok {
				product = p
			}
		}

		// ========================================================
		// GET SERIALS
		// ========================================================

		var serials []models.InboundSerial

		if err := c.DB.
			Where("inbound_detail_id = ?", detail.ID).
			Order("id ASC").
			Find(&serials).Error; err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		// ========================================================
		// SERIAL ITEM
		// ========================================================
		//
		// Jika ada 10 serial:
		//
		// SN001 -> 1 row
		// SN002 -> 1 row
		// ...
		// SN010 -> 1 row
		//
		// Qty Plan = 1
		//

		if len(serials) > 0 {

			for _, serial := range serials {

				values := []interface{}{
					no,
					detail.ItemCode,
					product.ItemName,
					product.UnitModel,
					detail.Barcode,
					serial.SerialNumber,

					// Qty Plan
					1,

					// User input
					"",
					"",
					"",
					"",
				}

				for colIndex, value := range values {

					cell, err := excelize.CoordinatesToCellName(
						colIndex+1,
						row,
					)

					if err != nil {
						return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
							"success": false,
							"error":   err.Error(),
						})
					}

					if err := f.SetCellValue(
						sheetName,
						cell,
						value,
					); err != nil {

						return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
							"success": false,
							"error":   err.Error(),
						})
					}
				}

				row++
				no++
			}

			continue
		}

		// ========================================================
		// NON SERIAL ITEM
		// ========================================================

		values := []interface{}{
			no,
			detail.ItemCode,
			product.ItemName,
			product.UnitModel,
			detail.Barcode,

			// Serial kosong
			"",

			// Qty Plan
			detail.Quantity,

			// User input
			"",
			"",
			"",
			"",
		}

		for colIndex, value := range values {

			cell, err := excelize.CoordinatesToCellName(
				colIndex+1,
				row,
			)

			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   err.Error(),
				})
			}

			if err := f.SetCellValue(
				sheetName,
				cell,
				value,
			); err != nil {

				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   err.Error(),
				})
			}
		}

		row++
		no++
	}

	// ============================================================
	// APPLY DATA STYLE
	// ============================================================

	lastRow := row - 1

	if lastRow >= 2 {

		if err := f.SetCellStyle(
			sheetName,
			"A2",
			"K"+strconv.Itoa(lastRow),
			dataStyle,
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		// Highlight kolom yang harus diisi user
		if err := f.SetCellStyle(
			sheetName,
			"H2",
			"K"+strconv.Itoa(lastRow),
			inputStyle,
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	// ============================================================
	// COLUMN WIDTH
	// ============================================================

	columnWidths := map[string]float64{
		"A": 8,  // No
		"B": 20, // Item Code
		"C": 35, // Item Name
		"D": 22, // Unit Model
		"E": 20, // Barcode
		"F": 30, // Serial Number
		"G": 12, // Qty Plan
		"H": 15, // Qty Received
		"I": 18, // Case No
		"J": 18, // Carton No
		"K": 20, // Location
	}

	for column, width := range columnWidths {

		if err := f.SetColWidth(
			sheetName,
			column,
			column,
			width,
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	// ============================================================
	// FREEZE HEADER
	// ============================================================

	if err := f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// AUTO FILTER
	// ============================================================

	if lastRow >= 1 {

		if err := f.AutoFilter(
			sheetName,
			"A1:K"+strconv.Itoa(lastRow),
			[]excelize.AutoFilterOptions{},
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	// ============================================================
	// SET ROW HEIGHT
	// ============================================================

	if lastRow >= 2 {

		if err := f.SetRowHeight(
			sheetName,
			1,
			25,
		); err != nil {

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		for i := 2; i <= lastRow; i++ {

			if err := f.SetRowHeight(
				sheetName,
				i,
				20,
			); err != nil {

				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   err.Error(),
				})
			}
		}
	}

	// ============================================================
	// DOWNLOAD FILE
	// ============================================================

	filename := fmt.Sprintf(
		"Receiving_Checking_%s.xlsx",
		inboundHeader.InboundNo,
	)

	ctx.Set(
		"Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	)

	ctx.Set(
		"Content-Disposition",
		fmt.Sprintf(
			`attachment; filename="%s"`,
			filename,
		),
	)

	// ============================================================
	// WRITE FILE TO RESPONSE
	// ============================================================

	if err := f.Write(ctx.Response().BodyWriter()); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return nil
}

func (c *InboundController) UploadCheckingExcel(ctx *fiber.Ctx) error {

	// ============================================================
	// GET INBOUND NO
	// ============================================================

	inboundNo := strings.TrimSpace(
		ctx.Params("inbound_no"),
	)

	if inboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Inbound No is required",
		})
	}

	// ============================================================
	// GET INBOUND HEADER
	// ============================================================

	var inboundHeader models.InboundHeader

	if err := c.DB.
		Where("inbound_no = ?", inboundNo).
		First(&inboundHeader).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Inbound not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// VALIDATE INBOUND HEADER STATUS
	// ============================================================
	//
	// Hanya inbound dengan status CHECKING yang boleh
	// melakukan upload checking Excel.
	//
	// open      -> reject
	// draft     -> reject
	// complete  -> reject
	// checking  -> allow
	//

	headerStatus := strings.ToLower(
		strings.TrimSpace(inboundHeader.Status),
	)

	if headerStatus != "checking" {

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error": fmt.Sprintf(
				"Cannot upload checking Excel. Inbound %s has status '%s'. Only inbound with status 'checking' can be uploaded.",
				inboundHeader.InboundNo,
				inboundHeader.Status,
			),
		})
	}

	// ============================================================
	// CHECK EXISTING INBOUND BARCODE
	// ============================================================
	//
	// Kalau sudah ada "in stock", berarti receiving tersebut
	// sudah putaway.
	//
	// Upload Excel TIDAK BOLEH dilakukan lagi.
	//

	var inStockCount int64

	if err := c.DB.
		Model(&models.InboundBarcode{}).
		Where(
			"inbound_id = ? AND LOWER(status) = ?",
			inboundHeader.ID,
			"in stock",
		).
		Count(&inStockCount).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if inStockCount > 0 {

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"error": fmt.Sprintf(
				"Cannot upload checking Excel. Inbound %s already has %d item(s) with status 'in stock'. The receiving has already been putaway.",
				inboundHeader.InboundNo,
				inStockCount,
			),
		})
	}

	// ============================================================
	// GET FILE
	// ============================================================

	fileHeader, err := ctx.FormFile("file")

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Excel file is required",
		})
	}

	// ============================================================
	// VALIDATE FILE EXTENSION
	// ============================================================
	//
	// Template yang kita generate adalah XLSX.
	//

	filename := strings.ToLower(
		fileHeader.Filename,
	)

	if !strings.HasSuffix(filename, ".xlsx") {

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Only .xlsx Excel files are allowed",
		})
	}

	// ============================================================
	// OPEN FILE
	// ============================================================

	file, err := fileHeader.Open()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to open uploaded file",
		})
	}

	defer file.Close()

	// ============================================================
	// READ FILE
	// ============================================================

	fileBytes, err := io.ReadAll(file)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read uploaded file",
		})
	}

	// ============================================================
	// OPEN EXCEL
	// ============================================================

	excel, err := excelize.OpenReader(
		bytes.NewReader(fileBytes),
	)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid Excel file: " + err.Error(),
		})
	}

	defer excel.Close()

	// ============================================================
	// GET SHEET
	// ============================================================

	sheetName := "Checking"

	rows, err := excel.GetRows(sheetName)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Sheet 'Checking' not found",
		})
	}

	if len(rows) <= 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Excel does not contain checking data",
		})
	}

	// ============================================================
	// VALIDATE HEADER
	// ============================================================

	expectedHeaders := []string{
		"No",
		"Item Code",
		"Item Name",
		"Unit Model",
		"Barcode",
		"Serial Number",
		"Qty Plan",
		"Qty Received",
		"Case No",
		"Carton No",
		"Location",
	}

	header := rows[0]

	if len(header) < len(expectedHeaders) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid Excel template. Missing columns.",
		})
	}

	for i, expected := range expectedHeaders {

		actual := strings.TrimSpace(
			header[i],
		)

		if actual != expected {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Invalid Excel template at column %d. Expected '%s', got '%s'",
					i+1,
					expected,
					actual,
				),
			})
		}
	}

	// ============================================================
	// GET INBOUND DETAILS
	// ============================================================

	var inboundDetails []models.InboundDetail

	if err := c.DB.
		Where("inbound_id = ?", inboundHeader.ID).
		Order("id ASC").
		Find(&inboundDetails).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(inboundDetails) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Inbound detail not found",
		})
	}

	// ============================================================
	// GET ALL SERIALS
	// ============================================================

	var inboundSerials []models.InboundSerial

	if err := c.DB.
		Where("inbound_id = ?", inboundHeader.ID).
		Order("id ASC").
		Find(&inboundSerials).Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// GROUP SERIAL BY INBOUND DETAIL
	// ============================================================

	serialMap := make(map[uint][]models.InboundSerial)

	for _, serial := range inboundSerials {

		detailID := uint(
			serial.InboundDetailId,
		)

		serialMap[detailID] = append(
			serialMap[detailID],
			serial,
		)
	}

	// ============================================================
	// EXPECTED ROW STRUCT
	// ============================================================

	type expectedRow struct {
		No int

		Detail models.InboundDetail

		SerialNumber string

		QtyPlan float64
	}

	expectedRows := make(
		[]expectedRow,
		0,
	)

	rowNo := 1

	for _, detail := range inboundDetails {

		// Karena detail.ID adalah uint,
		// serialMap juga menggunakan uint.
		serials := serialMap[detail.ID]

		// ========================================================
		// SERIAL ITEM
		// ========================================================

		if len(serials) > 0 {

			for _, serial := range serials {

				expectedRows = append(
					expectedRows,
					expectedRow{
						No: rowNo,

						Detail: detail,

						SerialNumber: strings.TrimSpace(
							serial.SerialNumber,
						),

						QtyPlan: 1,
					},
				)

				rowNo++
			}

			continue
		}

		// ========================================================
		// NON SERIAL ITEM
		// ========================================================

		expectedRows = append(
			expectedRows,
			expectedRow{
				No:           rowNo,
				Detail:       detail,
				SerialNumber: "",
				QtyPlan:      detail.Quantity,
			},
		)

		rowNo++
	}

	// ============================================================
	// VALIDATE ROW COUNT
	// ============================================================

	excelDataRows := rows[1:]

	if len(excelDataRows) != len(expectedRows) {

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error": fmt.Sprintf(
				"Excel row count does not match template. Expected %d rows, got %d rows",
				len(expectedRows),
				len(excelDataRows),
			),
		})
	}

	// ============================================================
	// UPLOAD ROW STRUCT
	// ============================================================

	type uploadRow struct {
		RowNumber int

		No int

		Detail models.InboundDetail

		ItemCode string

		Barcode string

		SerialNumber string

		QtyPlan float64

		QtyReceived float64

		CaseNumber string

		CartonNumber string

		Location string
	}

	uploadRows := make([]uploadRow, 0)

	// ============================================================
	// HELPER GET COLUMN
	// ============================================================

	getColumn := func(
		row []string,
		index int,
	) string {

		if len(row) <= index {
			return ""
		}

		return strings.TrimSpace(
			row[index],
		)
	}

	// ============================================================
	// HELPER PARSE FLOAT
	// ============================================================

	parseFloat := func(
		value string,
	) (float64, error) {

		value = strings.TrimSpace(value)

		if value == "" {
			return 0, fmt.Errorf("empty value")
		}

		// Support:
		//
		// 10
		// 10.5
		// 10,5
		//

		value = strings.ReplaceAll(
			value,
			",",
			".",
		)

		return strconv.ParseFloat(
			value,
			64,
		)
	}

	// ============================================================
	// TRACK TOTAL RECEIVED PER DETAIL
	// ============================================================

	receivedByDetail := make(map[uint]float64)

	// ============================================================
	// TRACK SERIAL RECEIVED
	// ============================================================

	serialReceived := make(map[string]float64)

	// ============================================================
	// PARSE EXCEL ROWS
	// ============================================================

	for index, row := range excelDataRows {

		excelRowNumber := index + 2

		expected := expectedRows[index]

		// ========================================================
		// IMPORTANT
		// ========================================================
		//
		// Excelize GetRows() dapat menghilangkan trailing
		// empty cells.
		//
		// Jadi jangan paksa len(row) == 11.
		//
		// Minimal A-G harus ada.
		//
		// H-K boleh kosong secara fisik.
		//

		if len(row) < 7 {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d is missing required template columns",
					excelRowNumber,
				),
			})
		}

		// ========================================================
		// READ VALUES
		// ========================================================

		noText := getColumn(row, 0)

		itemCode := getColumn(row, 1)
		itemName := getColumn(row, 2)
		unitModel := getColumn(row, 3)
		barcode := getColumn(row, 4)
		serialNumber := getColumn(row, 5)

		qtyPlanText := getColumn(row, 6)
		qtyReceivedText := getColumn(row, 7)

		caseNumber := getColumn(row, 8)
		cartonNumber := getColumn(row, 9)
		location := getColumn(row, 10)

		// ========================================================
		// NO
		// ========================================================

		if noText == "" {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: No is required",
					excelRowNumber,
				),
			})
		}

		no, err := strconv.Atoi(noText)

		if err != nil {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: No must be a number",
					excelRowNumber,
				),
			})
		}

		if no != expected.No {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: invalid No. Expected %d, got %d",
					excelRowNumber,
					expected.No,
					no,
				),
			})
		}

		// ========================================================
		// ITEM CODE
		// ========================================================

		if itemCode == "" {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Item Code is required",
					excelRowNumber,
				),
			})
		}

		if itemCode != expected.Detail.ItemCode {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Item Code does not match inbound detail. Expected '%s', got '%s'",
					excelRowNumber,
					expected.Detail.ItemCode,
					itemCode,
				),
			})
		}

		// ========================================================
		// ITEM NAME
		// ========================================================

		var product models.Product

		if expected.Detail.ItemId != 0 {

			if err := c.DB.
				First(
					&product,
					"id = ?",
					expected.Detail.ItemId,
				).Error; err != nil {

				if errors.Is(err, gorm.ErrRecordNotFound) {

					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"success": false,
						"error": fmt.Sprintf(
							"Row %d: Product for Item Code '%s' not found",
							excelRowNumber,
							itemCode,
						),
					})
				}

				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   err.Error(),
				})
			}
		}

		if itemName != strings.TrimSpace(product.ItemName) {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Item Name does not match master product",
					excelRowNumber,
				),
			})
		}

		// ========================================================
		// UNIT MODEL
		// ========================================================

		if unitModel != strings.TrimSpace(product.UnitModel) {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Unit Model does not match master product",
					excelRowNumber,
				),
			})
		}

		// ========================================================
		// BARCODE
		// ========================================================

		if barcode != strings.TrimSpace(
			expected.Detail.Barcode,
		) {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Barcode does not match inbound detail. Expected '%s', got '%s'",
					excelRowNumber,
					expected.Detail.Barcode,
					barcode,
				),
			})
		}

		// ========================================================
		// SERIAL NUMBER
		// ========================================================

		if serialNumber != expected.SerialNumber {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Serial Number does not match inbound serial. Expected '%s', got '%s'",
					excelRowNumber,
					expected.SerialNumber,
					serialNumber,
				),
			})
		}

		// ========================================================
		// QTY PLAN
		// ========================================================

		if qtyPlanText == "" {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Qty Plan is required",
					excelRowNumber,
				),
			})
		}

		qtyPlan, err := parseFloat(qtyPlanText)

		if err != nil {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: invalid Qty Plan '%s'",
					excelRowNumber,
					qtyPlanText,
				),
			})
		}

		if qtyPlan != expected.QtyPlan {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Qty Plan cannot be changed. Expected %.2f, got %.2f",
					excelRowNumber,
					expected.QtyPlan,
					qtyPlan,
				),
			})
		}

		// ========================================================
		// QTY RECEIVED
		// ========================================================

		if qtyReceivedText == "" {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Qty Received is required",
					excelRowNumber,
				),
			})
		}

		qtyReceived, err := parseFloat(
			qtyReceivedText,
		)

		if err != nil {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: invalid Qty Received '%s'",
					excelRowNumber,
					qtyReceivedText,
				),
			})
		}

		// ========================================================
		// QTY NEGATIVE
		// ========================================================

		if qtyReceived < 0 {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Qty Received cannot be negative",
					excelRowNumber,
				),
			})
		}

		// ========================================================
		// QTY RECEIVED > QTY PLAN
		// ========================================================

		if qtyReceived > qtyPlan {

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Qty Received %.2f exceeds Qty Plan %.2f",
					excelRowNumber,
					qtyReceived,
					qtyPlan,
				),
			})
		}

		// ========================================================
		// SERIAL VALIDATION
		// ========================================================

		if expected.SerialNumber != "" {

			// Qty Plan serial harus 1
			if expected.QtyPlan != 1 {

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Invalid serial Qty Plan. Expected 1",
						excelRowNumber,
					),
				})
			}

			// Qty Received hanya boleh 0 atau 1
			if qtyReceived != 0 && qtyReceived != 1 {

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Serial item Qty Received must be 0 or 1",
						excelRowNumber,
					),
				})
			}

			// ====================================================
			// DUPLICATE SERIAL RECEIVED
			// ====================================================

			if qtyReceived > 0 {

				serialKey := strings.ToLower(
					strings.TrimSpace(
						expected.SerialNumber,
					),
				)

				serialReceived[serialKey] += qtyReceived

				if serialReceived[serialKey] > 1 {

					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"success": false,
						"error": fmt.Sprintf(
							"Row %d: Serial Number '%s' is received more than once",
							excelRowNumber,
							expected.SerialNumber,
						),
					})
				}
			}
		}

		// ========================================================
		// TOTAL QTY PER INBOUND DETAIL
		// ========================================================
		//
		// Untuk non-serial:
		//
		// InboundDetail.Quantity = 100
		//
		// Excel:
		// Row 1 = 40
		// Row 2 = 30
		// Row 3 = 30
		//
		// Total = 100 -> OK
		//
		// Kalau:
		// Row 1 = 40
		// Row 2 = 70
		//
		// Total = 110 -> ERROR
		//
		// Untuk serial:
		// masing-masing detail biasanya hanya punya Qty Plan = 1
		// sehingga tetap aman.
		//

		detailID := expected.Detail.ID

		receivedByDetail[detailID] += qtyReceived

		if receivedByDetail[detailID] >
			expected.Detail.Quantity {

			return ctx.Status(
				fiber.StatusBadRequest,
			).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Total Qty Received %.2f for Item '%s' exceeds Inbound Detail Qty %.2f",
					excelRowNumber,
					receivedByDetail[detailID],
					expected.Detail.ItemCode,
					expected.Detail.Quantity,
				),
			})
		}

		// ========================================================
		// USER INPUT
		// ========================================================

		if qtyReceived > 0 {

			// ----------------------------------------------------
			// LOCATION
			// ----------------------------------------------------

			if location == "" {

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Location is required when Qty Received > 0",
						excelRowNumber,
					),
				})
			}

			// ----------------------------------------------------
			// CASE NO
			// ----------------------------------------------------

			if caseNumber == "" {

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Case No is required when Qty Received > 0",
						excelRowNumber,
					),
				})
			}

			// ----------------------------------------------------
			// CARTON NO
			// ----------------------------------------------------

			if cartonNumber == "" {

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Carton No is required when Qty Received > 0",
						excelRowNumber,
					),
				})
			}
		}

		// ========================================================
		// APPEND UPLOAD ROW
		// ========================================================

		uploadRows = append(
			uploadRows,
			uploadRow{
				RowNumber: excelRowNumber,

				No: no,

				Detail: expected.Detail,

				ItemCode: itemCode,

				Barcode: barcode,

				SerialNumber: serialNumber,

				QtyPlan: qtyPlan,

				QtyReceived: qtyReceived,

				CaseNumber: caseNumber,

				CartonNumber: cartonNumber,

				Location: location,
			},
		)
	}

	// ============================================================
	// CHECK LOCATION
	// ============================================================

	locationMap := make(map[string]models.Location)

	for _, row := range uploadRows {

		if row.QtyReceived <= 0 {
			continue
		}

		locationCode := strings.TrimSpace(
			row.Location,
		)

		if _, exists := locationMap[locationCode]; exists {
			continue
		}

		var location models.Location

		if err := c.DB.
			Where(
				"location_code = ?",
				locationCode,
			).
			First(&location).Error; err != nil {

			if errors.Is(
				err,
				gorm.ErrRecordNotFound,
			) {

				return ctx.Status(
					fiber.StatusBadRequest,
				).JSON(fiber.Map{
					"success": false,
					"error": fmt.Sprintf(
						"Row %d: Location '%s' is not registered in system",
						row.RowNumber,
						locationCode,
					),
				})
			}

			return ctx.Status(
				fiber.StatusInternalServerError,
			).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		// Optional: hanya location aktif yang boleh digunakan
		if !location.IsActive {

			return ctx.Status(
				fiber.StatusBadRequest,
			).JSON(fiber.Map{
				"success": false,
				"error": fmt.Sprintf(
					"Row %d: Location '%s' is inactive",
					row.RowNumber,
					locationCode,
				),
			})
		}

		locationMap[locationCode] = location
	}

	// ============================================================
	// GET USER ID
	// ============================================================

	userID := 0

	if value := ctx.Locals("userID"); value != nil {

		switch v := value.(type) {

		case float64:
			userID = int(v)

		case int:
			userID = v

		case int64:
			userID = int(v)
		}
	}

	// ============================================================
	// GENERATE PALLET
	// ============================================================

	inboundRepo := repositories.NewInboundRepository(
		c.DB,
	)

	palletID, err := inboundRepo.GeneratePalletID(
		inboundHeader.InboundNo,
	)

	if err != nil {
		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// TRANSACTION
	// ============================================================

	err = c.DB.Transaction(func(tx *gorm.DB) error {

		// ========================================================
		// RE-CHECK HEADER STATUS INSIDE TRANSACTION
		// ========================================================

		var currentInbound models.InboundHeader

		if err := tx.
			Where("id = ?", inboundHeader.ID).
			First(&currentInbound).Error; err != nil {

			return err
		}

		currentStatus := strings.ToLower(
			strings.TrimSpace(
				currentInbound.Status,
			),
		)

		if currentStatus != "checking" {

			return fmt.Errorf(
				"cannot upload checking Excel. Inbound %s has status '%s'. Only status 'checking' is allowed",
				currentInbound.InboundNo,
				currentInbound.Status,
			)
		}

		// ========================================================
		// RE-CHECK IN STOCK INSIDE TRANSACTION
		// ========================================================

		var txInStockCount int64

		if err := tx.
			Model(&models.InboundBarcode{}).
			Where(
				"inbound_id = ? AND LOWER(status) = ?",
				currentInbound.ID,
				"in stock",
			).
			Count(&txInStockCount).Error; err != nil {

			return err
		}

		if txInStockCount > 0 {

			return fmt.Errorf(
				"cannot upload checking Excel. Inbound %s already has %d item(s) with status 'in stock'. Receiving has already been putaway",
				currentInbound.InboundNo,
				txInStockCount,
			)
		}

		// ========================================================
		// DELETE ONLY PENDING
		// ========================================================
		//
		// IMPORTANT:
		//
		// Jangan delete:
		// status = in stock
		// status = lainnya
		//
		// Hanya pending.
		//

		if err := tx.Unscoped().
			Where(
				"inbound_id = ? AND LOWER(status) = ?",
				currentInbound.ID,
				"pending",
			).
			Delete(
				&models.InboundBarcode{},
			).
			Error; err != nil {

			return err
		}

		// ========================================================
		// INSERT NEW RECEIVING RESULT
		// ========================================================

		for _, row := range uploadRows {

			// ====================================================
			// ZERO RECEIVED
			// ====================================================
			//
			// Kalau Qty Received = 0:
			// tidak dibuat InboundBarcode.
			//

			if row.QtyReceived <= 0 {
				continue
			}

			detail := row.Detail

			location := locationMap[strings.TrimSpace(row.Location)]

			newInboundBarcode := models.InboundBarcode{

				// ------------------------------------------------
				// RELATION
				// ------------------------------------------------

				InboundId: int(
					currentInbound.ID,
				),

				InboundDetailId: uint(
					detail.ID,
				),

				// ------------------------------------------------
				// PRODUCT
				// ------------------------------------------------

				ItemID:   detail.ItemId,
				ItemCode: detail.ItemCode,

				// ------------------------------------------------
				// SCAN
				// ------------------------------------------------

				ScanType: "excel",

				ScanData: detail.Barcode,

				Barcode: detail.Barcode,

				SerialNumber: row.SerialNumber,

				// ------------------------------------------------
				// RECEIVING
				// ------------------------------------------------

				Pallet: location.LocationCode,

				Location: location.LocationCode,

				PutawayLocation: "",

				PutawayQty: 0,

				// ------------------------------------------------
				// DATE / LOT
				// ------------------------------------------------

				RecDate: detail.RecDate,

				ProdDate: detail.ProdDate,

				ExpDate: detail.ExpDate,

				LotNumber: detail.LotNumber,

				// ------------------------------------------------
				// CASE / CARTON
				// ------------------------------------------------

				CartonNumber: row.CartonNumber,

				CaseNumber: row.CaseNumber,

				// ------------------------------------------------
				// QTY
				// ------------------------------------------------

				Quantity: row.QtyReceived,

				Uom: detail.Uom,

				// ------------------------------------------------
				// WAREHOUSE
				// ------------------------------------------------

				WhsCode: detail.WhsCode,

				OwnerCode: detail.OwnerCode,

				DivisionCode: detail.DivisionCode,

				QaStatus: detail.QaStatus,

				// ------------------------------------------------
				// STATUS
				// ------------------------------------------------

				Status: "pending",

				// ------------------------------------------------
				// AUDIT
				// ------------------------------------------------

				CreatedBy: userID,

				UpdatedBy: userID,
			}

			// ----------------------------------------------------
			// INSERT
			// ----------------------------------------------------

			if err := tx.
				Create(&newInboundBarcode).
				Error; err != nil {

				return fmt.Errorf(
					"failed to insert InboundBarcode for Excel row %d: %w",
					row.RowNumber,
					err,
				)
			}
		}

		return nil
	})

	// ============================================================
	// TRANSACTION ERROR
	// ============================================================

	if err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// GET NEW PENDING RESULT
	// ============================================================

	var inboundBarcodes []models.InboundBarcode

	if err := c.DB.
		Where(
			"inbound_id = ? AND LOWER(status) = ?",
			inboundHeader.ID,
			"pending",
		).
		Order("id ASC").
		Find(&inboundBarcodes).Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ============================================================
	// RESPONSE
	// ============================================================

	return ctx.Status(
		fiber.StatusOK,
	).JSON(fiber.Map{

		"success": true,

		"message": "Receiving Excel uploaded successfully",

		"inbound_no": inboundHeader.InboundNo,

		"pallet": palletID,

		"items": inboundBarcodes,
	})
}
