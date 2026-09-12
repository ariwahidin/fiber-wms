package outbound_controller

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ============================================================================
// FURUNO EXCEL UPLOAD CONTROLLER
//
// Khusus template Excel Delivery Order dari Furuno.
//
// IMPORTANT:
// - Tidak mengubah flow CreateOutboundFromB2BExcel
// - Tidak menggunakan hardcode column index
// - Mapping berdasarkan nama HEADER Excel
// - Posisi kolom boleh berpindah
// - Hanya row dengan "Name Warehouse" = "Yusen WH" yang diproses
// - 1 "Number" / DO = 1 Outbound Header
// - Setiap row valid = 1 Outbound Detail
//
// Expected headers:
//
//   Code#
//   Item Name
//   Quantity
//   Unit
//   Number
//   Date
//   Customer
//   Serial/Production Number
//   Name Warehouse
//
// ============================================================================

const (
	furunoSheetName = "Delivery Order Detail"

	// Hanya data dengan warehouse ini yang akan diproses.
	furunoAllowedWarehouse = "Yusen WH"
)

// ============================================================================
// RESPONSE
// ============================================================================

type FurunoUploadResponse struct {
	Success          bool                    `json:"success"`
	Message          string                  `json:"message"`
	TotalRows        int                     `json:"total_rows"`
	ProcessedRows    int                     `json:"processed_rows"`
	SkippedRows      int                     `json:"skipped_rows"`
	SuccessCount     int                     `json:"success_count"`
	FailedCount      int                     `json:"failed_count"`
	OutboundNumbers  []string                `json:"outbound_numbers,omitempty"`
	SkippedOrders    []FurunoSkippedOrder    `json:"skipped_orders,omitempty"`
	ValidationErrors []FurunoValidationError `json:"validation_errors,omitempty"`
	Errors           []FurunoExcelRowError   `json:"errors,omitempty"`
}

type FurunoSkippedOrder struct {
	ShipmentID string `json:"shipment_id"`
	Reason     string `json:"reason"`
}

type FurunoValidationError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type FurunoExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ============================================================================
// INTERNAL DATA STRUCT
// ============================================================================

type FurunoOrderRow struct {
	Row int

	ItemCode      string
	ItemName      string
	Quantity      float64
	Unit          string
	ShipmentID    string
	OutboundDate  string
	CustomerName  string
	SerialNumber  string
	WarehouseName string
}

// ============================================================================
// HEADER CONFIGURATION
// ============================================================================

// Header Excel dinormalisasi menjadi lowercase + trim.
//
// Contoh:
//
// "Code#"       -> "code#"
// "Item Name"   -> "item name"
// "Quantity"    -> "quantity"
//
// Jadi posisi kolom tidak penting.
type FurunoHeaderMap map[string]int

// Required headers.
var furunoRequiredHeaders = []string{
	"code#",
	"item name",
	"quantity",
	"unit",
	"number",
	"date",
	"customer",
	"serial/production number",
	"name warehouse",
}

// ============================================================================
// MAIN HANDLER
// ============================================================================

// CreateOutboundFromFurunoExcel
//
// POST:
// /outbound/upload-furuno-excel
//
// Form Data:
//
// file       : Excel file
// owner_code : owner code
// whs_code   : warehouse code
//
// Hanya row:
//
// Name Warehouse = Yusen WH
//
// yang diproses.
func (c *OutboundController) CreateOutboundFromFurunoExcel(ctx *fiber.Ctx) error {

	// =========================================================================
	// 1. FORM PARAMETER
	// =========================================================================

	ownerCode := strings.TrimSpace(ctx.FormValue("owner_code"))

	if ownerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Owner code is required",
		})
	}

	whsCode := strings.TrimSpace(ctx.FormValue("whs_code"))

	if whsCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Warehouse code is required",
		})
	}

	// =========================================================================
	// 2. GET FILE
	// =========================================================================

	file, err := ctx.FormFile("file")

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "File Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	// =========================================================================
	// 3. VALIDATE EXTENSION
	// =========================================================================

	lowerName := strings.ToLower(file.Filename)

	if !strings.HasSuffix(lowerName, ".xlsx") &&
		!strings.HasSuffix(lowerName, ".xls") {

		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// =========================================================================
	// 4. VALIDATE SIZE
	// =========================================================================

	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "File size exceeds maximum limit of 10MB",
		})
	}

	// =========================================================================
	// 5. OPEN EXCEL
	// =========================================================================

	fileHeader, err := file.Open()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "File Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "Excel Read Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	defer excelFile.Close()

	// =========================================================================
	// 6. FIND SHEET
	// =========================================================================

	sheetName, err := findFurunoSheet(excelFile, furunoSheetName)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: fmt.Sprintf(
				"Sheet '%s' not found. Available sheets: %s",
				furunoSheetName,
				strings.Join(excelFile.GetSheetList(), ", "),
			),
		})
	}

	// =========================================================================
	// 7. READ ROWS
	// =========================================================================

	rows, err := excelFile.GetRows(sheetName)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "Sheet Read Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Excel file contains no data rows",
		})
	}

	// =========================================================================
	// 8. BUILD HEADER MAP
	// =========================================================================

	headerMap, err := buildFurunoHeaderMap(rows[0])

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	// =========================================================================
	// 9. PARSE EXCEL
	// =========================================================================

	orderRows, validationErrors, skippedRows :=
		parseFurunoRows(rows, headerMap)

	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			TotalRows:        len(rows) - 1,
			ProcessedRows:    len(orderRows),
			SkippedRows:      skippedRows,
			ValidationErrors: validationErrors,
		})
	}

	if len(orderRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(FurunoUploadResponse{
			Success:       false,
			Message:       fmt.Sprintf("No valid rows found. Only '%s' rows are processed", furunoAllowedWarehouse),
			TotalRows:     len(rows) - 1,
			SkippedRows:   skippedRows,
			ProcessedRows: 0,
		})
	}

	// =========================================================================
	// 10. GROUP BY NUMBER / SHIPMENT ID
	// =========================================================================

	orderMap := groupFurunoRowsByShipmentID(orderRows)

	shipmentIDs := make([]string, 0, len(orderMap))

	for shipmentID := range orderMap {
		shipmentIDs = append(shipmentIDs, shipmentID)
	}

	sort.Strings(shipmentIDs)

	// =========================================================================
	// 11. USER ID
	// =========================================================================

	userIDValue := ctx.Locals("userID")

	if userIDValue == nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(FurunoUploadResponse{
			Success: false,
			Message: "User ID not found",
		})
	}

	userID, ok := userIDValue.(float64)

	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	currentUserID := int(userID)

	// =========================================================================
	// 12. START TRANSACTION
	// =========================================================================

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to start database transaction",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "Transaction Error",
					Detail:  tx.Error.Error(),
				},
			},
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()

			log.Printf(
				"Panic recovered in CreateOutboundFromFurunoExcel: %v",
				r,
			)
		}
	}()

	// =========================================================================
	// 13. VALIDATE OWNER
	// =========================================================================

	var inventoryPolicy models.InventoryPolicy

	if err := tx.
		Where("owner_code = ?", ownerCode).
		First(&inventoryPolicy).Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(FurunoUploadResponse{
				Success: false,
				Message: "Owner not found: " + ownerCode,
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to validate owner",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "Owner Validation Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	// =========================================================================
	// 14. VALIDATE WAREHOUSE
	// =========================================================================

	var warehouse models.Warehouse

	if err := tx.
		Where("code = ?", whsCode).
		First(&warehouse).Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(FurunoUploadResponse{
				Success: false,
				Message: "Warehouse not found: " + whsCode,
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
			Success: false,
			Message: "Failed to validate warehouse",
			Errors: []FurunoExcelRowError{
				{
					Row:     0,
					Message: "Warehouse Validation Error",
					Detail:  err.Error(),
				},
			},
		})
	}

	// =========================================================================
	// 15. CHECK DUPLICATE SHIPMENT ID
	// =========================================================================

	duplicateShipments := make(map[string]string)

	for _, shipmentID := range shipmentIDs {

		var count int64

		err := tx.
			Model(&models.OutboundHeader{}).
			Where("shipment_id = ?", shipmentID).
			Count(&count).Error

		if err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
				Success: false,
				Message: "Failed to check duplicate shipment ID",
				Errors: []FurunoExcelRowError{
					{
						Row:     0,
						Message: "Duplicate Check Error",
						Detail:  err.Error(),
					},
				},
			})
		}

		if count > 0 {
			duplicateShipments[shipmentID] =
				fmt.Sprintf("Shipment ID / DO '%s' already exists", shipmentID)
		}
	}

	// =========================================================================
	// 16. FILTER VALID ORDERS
	// =========================================================================

	validShipmentIDs := make([]string, 0)

	skippedOrders := make([]FurunoSkippedOrder, 0)

	for _, shipmentID := range shipmentIDs {

		if reason, exists := duplicateShipments[shipmentID]; exists {

			skippedOrders = append(
				skippedOrders,
				FurunoSkippedOrder{
					ShipmentID: shipmentID,
					Reason:     reason,
				},
			)

			continue
		}

		validShipmentIDs = append(
			validShipmentIDs,
			shipmentID,
		)
	}

	if len(validShipmentIDs) == 0 {

		tx.Rollback()

		return ctx.Status(fiber.StatusOK).JSON(FurunoUploadResponse{
			Success:         false,
			Message:         "No valid outbound to process. All DOs already exist.",
			TotalRows:       len(rows) - 1,
			ProcessedRows:   len(orderRows),
			SkippedRows:     skippedRows,
			SuccessCount:    0,
			FailedCount:     len(skippedOrders),
			SkippedOrders:   skippedOrders,
			OutboundNumbers: []string{},
		})
	}

	// =========================================================================
	// 17. CUSTOMER LOOKUP
	// =========================================================================

	customerNameSet := make(map[string]bool)

	for _, shipmentID := range validShipmentIDs {

		items := orderMap[shipmentID]

		if len(items) == 0 {
			continue
		}

		customerName := strings.TrimSpace(
			items[0].CustomerName,
		)

		if customerName != "" {
			customerNameSet[customerName] = true
		}
	}

	customerNameToCode := make(map[string]string)

	if len(customerNameSet) > 0 {

		names := make([]string, 0, len(customerNameSet))

		for name := range customerNameSet {
			names = append(names, name)
		}

		var customers []models.Customer

		if err := tx.
			Where("customer_name IN ?", names).
			Find(&customers).Error; err != nil {

			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
				Success: false,
				Message: "Failed to lookup customers",
				Errors: []FurunoExcelRowError{
					{
						Row:     0,
						Message: "Customer Lookup Error",
						Detail:  err.Error(),
					},
				},
			})
		}

		for _, customer := range customers {
			customerNameToCode[strings.TrimSpace(customer.CustomerName)] = customer.CustomerCode
		}
	}

	// =========================================================================
	// 18. CREATE OUTBOUND
	// =========================================================================

	repo := repositories.NewOutboundRepository(tx)

	var outboundNumbers []string

	totalSuccessItems := 0

	for _, shipmentID := range validShipmentIDs {

		items := orderMap[shipmentID]

		if len(items) == 0 {
			continue
		}

		firstItem := items[0]

		// ---------------------------------------------------------------------
		// Generate Outbound Number
		// ---------------------------------------------------------------------

		outboundNo, err := repo.GenerateOutboundNumber()

		if err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(FurunoUploadResponse{
				Success: false,
				Message: "Failed to generate outbound number",
				Errors: []FurunoExcelRowError{
					{
						Row:     firstItem.Row,
						Message: "Generation Error",
						Detail:  err.Error(),
					},
				},
			})
		}

		// ---------------------------------------------------------------------
		// Resolve Customer
		// ---------------------------------------------------------------------

		customerCode := ""

		if code, found := customerNameToCode[strings.TrimSpace(firstItem.CustomerName)]; found {
			customerCode = code
		}

		// ---------------------------------------------------------------------
		// Current Time
		// ---------------------------------------------------------------------

		now := time.Now()

		nowDate := now.Format("2006-01-02")
		nowTime := now.Format("15:04")

		// ---------------------------------------------------------------------
		// Remarks
		// ---------------------------------------------------------------------

		remarks := fmt.Sprintf(
			"FURUNO | DO: %s",
			shipmentID,
		)

		// ---------------------------------------------------------------------
		// Outbound Header
		// ---------------------------------------------------------------------

		outboundHeader := models.OutboundHeader{
			OutboundNo:   outboundNo,
			OutboundDate: firstItem.OutboundDate,
			ShipmentID:   shipmentID,
			CustomerCode: customerCode,
			WhsCode:      whsCode,
			OwnerCode:    ownerCode,
			Status:       "open",
			RawStatus:    "DRAFT",
			DraftTime:    now,
			Remarks:      remarks,
			Source:       "FURUNO",
			Integration:  false,
			DelivTo:      firstItem.CustomerName,
			CreatedBy:    currentUserID,
			UpdatedBy:    currentUserID,

			PlanPickupDate: nowDate,
			PlanPickupTime: "16:00",

			RcvDoDate:     nowDate,
			RcvDoTime:     nowTime,
			StartPickTime: nowTime,
			EndPickTime:   nowTime,
		}

		// ---------------------------------------------------------------------
		// Insert Header
		// ---------------------------------------------------------------------

		if err := tx.
			Create(&outboundHeader).
			Error; err != nil {

			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(
				FurunoUploadResponse{
					Success: false,
					Message: fmt.Sprintf(
						"Failed to create outbound header for DO %s",
						shipmentID,
					),
					Errors: []FurunoExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Header Insert Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		// ---------------------------------------------------------------------
		// Insert Details
		// ---------------------------------------------------------------------

		for _, item := range items {

			// ================================================================
			// Product Lookup
			// ================================================================

			var product models.Product

			if err := tx.
				Where("item_code = ?", item.ItemCode).
				First(&product).Error; err != nil {

				tx.Rollback()

				if errors.Is(err, gorm.ErrRecordNotFound) {

					return ctx.Status(fiber.StatusNotFound).JSON(
						FurunoUploadResponse{
							Success: false,
							Message: fmt.Sprintf(
								"Product not found: %s",
								item.ItemCode,
							),
							Errors: []FurunoExcelRowError{
								{
									Row:     item.Row,
									Message: "Product Not Found",
									Detail: fmt.Sprintf(
										"SKU: %s",
										item.ItemCode,
									),
								},
							},
						},
					)
				}

				return ctx.Status(fiber.StatusInternalServerError).JSON(
					FurunoUploadResponse{
						Success: false,
						Message: "Failed to lookup product",
						Errors: []FurunoExcelRowError{
							{
								Row:     item.Row,
								Message: "Product Lookup Error",
								Detail:  err.Error(),
							},
						},
					},
				)
			}

			// ================================================================
			// UOM Conversion
			//
			// Sama seperti flow B2B:
			//
			// 1. Coba factor = 1
			// 2. Kalau tidak ada, ambil conversion pertama
			// ================================================================

			var uomConversion models.UomConversion

			if err := tx.
				Where(
					"item_code = ? AND factor = 1",
					product.ItemCode,
				).
				First(&uomConversion).Error; err != nil {

				if err2 := tx.
					Where(
						"item_code = ?",
						product.ItemCode,
					).
					First(&uomConversion).Error; err2 != nil {

					tx.Rollback()

					return ctx.Status(fiber.StatusNotFound).JSON(
						FurunoUploadResponse{
							Success: false,
							Message: fmt.Sprintf(
								"UOM conversion not found for SKU: %s",
								item.ItemCode,
							),
							Errors: []FurunoExcelRowError{
								{
									Row:     item.Row,
									Message: "UOM Not Found",
									Detail: fmt.Sprintf(
										"SKU: %s",
										item.ItemCode,
									),
								},
							},
						},
					)
				}
			}

			// ================================================================
			// UOM
			//
			// Kalau Excel Unit kosong, gunakan UOM conversion.
			// Kalau Excel Unit ada, tetap gunakan UOM conversion karena
			// conversion master adalah sumber UOM WMS.
			// ================================================================

			detailUOM := uomConversion.FromUom

			if strings.TrimSpace(item.Unit) != "" {

				// Cari conversion berdasarkan UOM dari Excel.
				var excelUOMConversion models.UomConversion

				errUOM := tx.
					Where(
						"item_code = ? AND from_uom = ?",
						product.ItemCode,
						strings.TrimSpace(item.Unit),
					).
					First(&excelUOMConversion).Error

				if errUOM == nil {
					uomConversion = excelUOMConversion
					detailUOM = excelUOMConversion.FromUom
				}
			}

			// ================================================================
			// Customer Code
			// ================================================================

			detailCustomerCode := customerCode

			// ================================================================
			// Outbound Detail
			// ================================================================

			outboundDetail := models.OutboundDetail{
				OutboundNo:   outboundNo,
				OutboundID:   outboundHeader.ID,
				ItemCode:     product.ItemCode,
				ItemID:       int(product.ID),
				Barcode:      uomConversion.Ean,
				CustomerCode: detailCustomerCode,
				Uom:          detailUOM,
				Quantity:     item.Quantity,
				WhsCode:      whsCode,
				DivisionCode: "SALES",
				QaStatus:     "A",
				OwnerCode:    ownerCode,
				Remarks:      item.ItemName,
				SNCheck:      "N",
				SerialNumber: item.SerialNumber,
				CreatedBy:    currentUserID,
				UpdatedBy:    currentUserID,
			}

			if err := tx.
				Create(&outboundDetail).
				Error; err != nil {

				tx.Rollback()

				return ctx.Status(
					fiber.StatusInternalServerError,
				).JSON(FurunoUploadResponse{
					Success: false,
					Message: fmt.Sprintf(
						"Failed to create outbound detail for SKU: %s",
						item.ItemCode,
					),
					Errors: []FurunoExcelRowError{
						{
							Row:     item.Row,
							Message: "Detail Insert Error",
							Detail:  err.Error(),
						},
					},
				})
			}

			// ================================================================
			// Insert Serial
			// ================================================================

			if strings.TrimSpace(item.SerialNumber) != "" {

				serial := models.OutboundSerial{
					OutboundId:       int(outboundHeader.ID),
					OutboundDetailId: int(outboundDetail.ID),
					SerialNumber:     strings.TrimSpace(item.SerialNumber),
					CreatedBy:        currentUserID,
					UpdatedBy:        currentUserID,
				}

				if err := tx.
					Create(&serial).
					Error; err != nil {

					tx.Rollback()

					return ctx.Status(
						fiber.StatusInternalServerError,
					).JSON(FurunoUploadResponse{
						Success: false,
						Message: "Failed to insert outbound serial",
						Errors: []FurunoExcelRowError{
							{
								Row:     item.Row,
								Message: "Serial Insert Error",
								Detail:  err.Error(),
							},
						},
					})
				}
			}

			totalSuccessItems++
		}

		// ---------------------------------------------------------------------
		// Transaction History
		// ---------------------------------------------------------------------

		if err := helpers.InsertTransactionHistory(
			tx,
			outboundNo,
			"open",
			"OUTBOUND",
			fmt.Sprintf(
				"Created from FURUNO Excel - DO: %s",
				shipmentID,
			),
			currentUserID,
		); err != nil {

			// Sama seperti existing B2B:
			// history failure tidak menggagalkan outbound.
			log.Printf(
				"Warning: Failed to insert transaction history for %s: %v",
				outboundNo,
				err,
			)
		}

		outboundNumbers = append(
			outboundNumbers,
			outboundNo,
		)
	}

	// =========================================================================
	// 19. COMMIT
	// =========================================================================

	if err := tx.Commit().Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoUploadResponse{
				Success: false,
				Message: "Failed to commit transaction",
				Errors: []FurunoExcelRowError{
					{
						Row:     0,
						Message: "Transaction Commit Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	// =========================================================================
	// 20. RESPONSE
	// =========================================================================

	message := fmt.Sprintf(
		"Created %d outbound(s) with %d item(s) from FURUNO Excel",
		len(outboundNumbers),
		totalSuccessItems,
	)

	if len(skippedOrders) > 0 {
		message += fmt.Sprintf(
			". %d DO(s) skipped because already exist",
			len(skippedOrders),
		)
	}

	if skippedRows > 0 {
		message += fmt.Sprintf(
			". %d row(s) skipped because Name Warehouse is not '%s'",
			skippedRows,
			furunoAllowedWarehouse,
		)
	}

	return ctx.Status(fiber.StatusOK).JSON(
		FurunoUploadResponse{
			Success:         true,
			Message:         message,
			TotalRows:       len(rows) - 1,
			ProcessedRows:   len(orderRows),
			SkippedRows:     skippedRows,
			SuccessCount:    totalSuccessItems,
			FailedCount:     len(skippedOrders),
			OutboundNumbers: outboundNumbers,
			SkippedOrders:   skippedOrders,
		},
	)
}

// ============================================================================
// HEADER MAP
// ============================================================================

// buildFurunoHeaderMap
//
// Membuat mapping:
//
// Excel Header -> Column Index
//
// Contoh:
//
// Customer -> 6
// Quantity -> 2
// Number   -> 4
//
// Kalau posisi berubah:
//
// Quantity -> 0
// Customer -> 4
// Number   -> 7
//
// tetap jalan.
func buildFurunoHeaderMap(
	headerRow []string,
) (FurunoHeaderMap, error) {

	headerMap := make(FurunoHeaderMap)

	for index, header := range headerRow {

		normalized := normalizeFurunoHeader(header)

		if normalized == "" {
			continue
		}

		// Duplicate header tidak diperbolehkan.
		if _, exists := headerMap[normalized]; exists {
			return nil, fmt.Errorf(
				"duplicate Excel header found: '%s'",
				header,
			)
		}

		headerMap[normalized] = index
	}

	// Check required headers.
	for _, required := range furunoRequiredHeaders {

		if _, exists := headerMap[required]; !exists {
			return nil, fmt.Errorf(
				"required Excel header '%s' not found",
				required,
			)
		}
	}

	return headerMap, nil
}

// ============================================================================
// PARSE ROWS
// ============================================================================

func parseFurunoRows(
	rows [][]string,
	headerMap FurunoHeaderMap,
) (
	[]FurunoOrderRow,
	[]FurunoValidationError,
	int,
) {

	var result []FurunoOrderRow
	var errs []FurunoValidationError

	skippedRows := 0

	// Row 0 = header.
	for i := 1; i < len(rows); i++ {

		row := rows[i]

		rowNum := i + 1

		// ================================================================
		// Skip empty row
		// ================================================================

		if furunoRowIsEmpty(row) {
			continue
		}

		// ================================================================
		// Get Warehouse
		// ================================================================

		warehouseName := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"name warehouse",
			),
		)

		// ================================================================
		// IMPORTANT:
		// Hanya Yusen WH
		// ================================================================

		if !strings.EqualFold(
			warehouseName,
			furunoAllowedWarehouse,
		) {

			skippedRows++

			continue
		}

		// ================================================================
		// Shipment ID / Number
		// ================================================================

		shipmentID := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"number",
			),
		)

		if shipmentID == "" {

			errs = append(
				errs,
				FurunoValidationError{
					Row:     rowNum,
					Field:   "Number",
					Message: "Number / DO cannot be empty",
				},
			)

			continue
		}

		// ================================================================
		// Item Code
		// ================================================================

		itemCodeRaw := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"code#",
			),
		)

		if itemCodeRaw == "" {

			errs = append(
				errs,
				FurunoValidationError{
					Row:     rowNum,
					Field:   "Code#",
					Message: "Code# cannot be empty",
				},
			)

			continue
		}

		itemCode := cleanFurunoItemCode(itemCodeRaw)

		// ================================================================
		// Item Name
		// ================================================================

		itemName := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"item name",
			),
		)

		// ================================================================
		// Quantity
		// ================================================================

		qtyRaw := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"quantity",
			),
		)

		if qtyRaw == "" {

			errs = append(
				errs,
				FurunoValidationError{
					Row:     rowNum,
					Field:   "Quantity",
					Message: "Quantity cannot be empty",
				},
			)

			continue
		}

		qty, err := strconv.ParseFloat(
			qtyRaw,
			64,
		)

		if err != nil || qty <= 0 {

			errs = append(
				errs,
				FurunoValidationError{
					Row:   rowNum,
					Field: "Quantity",
					Message: fmt.Sprintf(
						"Invalid quantity: %s",
						qtyRaw,
					),
				},
			)

			continue
		}

		// ================================================================
		// Unit
		// ================================================================

		unit := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"unit",
			),
		)

		// ================================================================
		// Date
		// ================================================================

		dateRaw := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"date",
			),
		)

		parsedDate := parseFurunoDate(dateRaw)

		if parsedDate == "" {

			errs = append(
				errs,
				FurunoValidationError{
					Row:   rowNum,
					Field: "Date",
					Message: fmt.Sprintf(
						"Invalid date format: %s",
						dateRaw,
					),
				},
			)

			continue
		}

		// ================================================================
		// Customer
		// ================================================================

		customerName := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"customer",
			),
		)

		// ================================================================
		// Serial
		// ================================================================

		serialNumber := strings.TrimSpace(
			getFurunoCell(
				row,
				headerMap,
				"serial/production number",
			),
		)

		// ================================================================
		// Append
		// ================================================================

		result = append(
			result,
			FurunoOrderRow{
				Row:           rowNum,
				ItemCode:      itemCode,
				ItemName:      itemName,
				Quantity:      qty,
				Unit:          unit,
				ShipmentID:    shipmentID,
				OutboundDate:  parsedDate,
				CustomerName:  customerName,
				SerialNumber:  serialNumber,
				WarehouseName: warehouseName,
			},
		)
	}

	return result, errs, skippedRows
}

// ============================================================================
// GROUP
// ============================================================================

func groupFurunoRowsByShipmentID(
	rows []FurunoOrderRow,
) map[string][]FurunoOrderRow {

	result := make(
		map[string][]FurunoOrderRow,
	)

	for _, row := range rows {

		result[row.ShipmentID] =
			append(
				result[row.ShipmentID],
				row,
			)
	}

	return result
}

// ============================================================================
// GET CELL
// ============================================================================

func getFurunoCell(
	row []string,
	headerMap FurunoHeaderMap,
	header string,
) string {

	index, exists := headerMap[normalizeFurunoHeader(header)]

	if !exists {
		return ""
	}

	if index < 0 || index >= len(row) {
		return ""
	}

	return row[index]
}

// ============================================================================
// NORMALIZE HEADER
// ============================================================================

func normalizeFurunoHeader(
	header string,
) string {

	header = strings.TrimSpace(
		header,
	)

	header = strings.ToLower(
		header,
	)

	// Normalize multiple spaces.
	header = strings.Join(
		strings.Fields(header),
		" ",
	)

	return header
}

// ============================================================================
// EMPTY ROW
// ============================================================================

func furunoRowIsEmpty(
	row []string,
) bool {

	for _, cell := range row {

		if strings.TrimSpace(cell) != "" {
			return false
		}
	}

	return true
}

// ============================================================================
// CLEAN ITEM CODE
// ============================================================================

// Contoh:
//
// 1945800.0 -> 1945800
// 00001945800 -> tetap 00001945800
//
// Catatan:
// excelize biasanya mempertahankan string cell sebagai string jika
// Excel cell memang disimpan sebagai text.
func cleanFurunoItemCode(
	raw string,
) string {

	raw = strings.TrimSpace(raw)

	if idx := strings.Index(
		raw,
		".",
	); idx != -1 {

		decimal := raw[idx+1:]

		allZero := true

		for _, ch := range decimal {

			if ch != '0' {

				allZero = false

				break
			}
		}

		if allZero {

			return raw[:idx]
		}
	}

	return raw
}

// ============================================================================
// DATE PARSER
// ============================================================================

func parseFurunoDate(
	raw string,
) string {

	raw = strings.TrimSpace(raw)

	if raw == "" {
		return ""
	}

	// ================================================================
	// Excel serial number
	// ================================================================

	if days, err := strconv.ParseFloat(
		raw,
		64,
	); err == nil && days > 40000 {

		excelEpoch := time.Date(
			1899,
			12,
			30,
			0,
			0,
			0,
			0,
			time.UTC,
		)

		date := excelEpoch.Add(
			time.Duration(days) * 24 * time.Hour,
		)

		return date.Format(
			"2006-01-02",
		)
	}

	// ================================================================
	// String formats
	// ================================================================

	dateFormats := []string{

		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",

		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",

		"01/02/2006",

		"2/1/2006",
		"1/2/2006",

		// DD-MM-YY
		"02-01-06",

		// DD-MM-YYYY
		"02-01-2006",

		"2-Jan-06",
		"02-Jan-06",
		"2-January-06",
		"02-January-06",

		"2-Jan-2006",
		"02-Jan-2006",
		"2-January-2006",
		"02-January-2006",

		// Format: 09 Sep 2026
		"2 Jan 2006",
		"02 Jan 2006",
	}

	for _, format := range dateFormats {

		if t, err := time.Parse(
			format,
			raw,
		); err == nil {

			return t.Format(
				"2006-01-02",
			)
		}
	}

	return ""
}

// ============================================================================
// FIND SHEET
// ============================================================================

func findFurunoSheet(
	file *excelize.File,
	expected string,
) (string, error) {

	expectedNormalized :=
		normalizeFurunoHeader(expected)

	for _, sheet := range file.GetSheetList() {

		if normalizeFurunoHeader(sheet) ==
			expectedNormalized {

			return sheet, nil
		}
	}

	return "", fmt.Errorf(
		"sheet '%s' not found",
		expected,
	)
}
