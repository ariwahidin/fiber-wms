package integration_service

import (
	"encoding/json"
	"fiber-app/models"
	integrationModel "fiber-app/models/integration"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ─── Column Mapping ───────────────────────────────────────────────────────────

// ApplyColumnMapping mengkonversi row dari file ke map dengan key WMS field
// columnMapping format JSON: {"wms_field": "file_column"}
// contoh: {"outbound_no": "DO_NUMBER", "customer_code": "CUST_CODE", "item_code": "SKU"}
func ApplyColumnMapping(row ParsedRow, columnMapping string) (ParsedRow, error) {
	if columnMapping == "" {
		// Tidak ada mapping — gunakan row apa adanya
		return row, nil
	}

	var mapping map[string]string
	if err := json.Unmarshal([]byte(columnMapping), &mapping); err != nil {
		return nil, fmt.Errorf("gagal parse column_mapping: %w", err)
	}

	result := make(ParsedRow)
	for wmsField, fileCol := range mapping {
		if val, ok := row[fileCol]; ok {
			result[wmsField] = val
		} else {
			result[wmsField] = ""
		}
	}

	// Sisipkan field yang tidak ada di mapping (passthrough)
	for k, v := range row {
		if _, mapped := result[k]; !mapped {
			result[k] = v
		}
	}

	return result, nil
}

// ─── Process Rows ─────────────────────────────────────────────────────────────

type ProcessResult struct {
	TotalRows    int
	SuccessCount int
	FailedCount  int
	Errors       []ProcessError
	OutboundNos  []string
}

type ProcessError struct {
	Row     int
	Message string
	Detail  string
}

// ProcessInboundRows memproses semua rows dari file dan create outbound di WMS
func ProcessInboundRows(
	db *gorm.DB,
	intg integrationModel.Integration,
	rows []ParsedRow,
	userID int,
) ProcessResult {
	result := ProcessResult{TotalRows: len(rows)}

	switch intg.Action {
	case "create_outbound":
		return processCreateOutbound(db, intg, rows, userID)
	default:
		result.FailedCount = len(rows)
		result.Errors = append(result.Errors, ProcessError{
			Row:     0,
			Message: "Action tidak dikenal: " + intg.Action,
		})
		return result
	}
}

// ─── Create Outbound ──────────────────────────────────────────────────────────

// processCreateOutbound membuat outbound header + details dari rows file
// Logika: group rows berdasarkan outbound_no / shipment_id
// Jika tidak ada kolom grouping, tiap row = 1 outbound
func processCreateOutbound(
	db *gorm.DB,
	intg integrationModel.Integration,
	rows []ParsedRow,
	userID int,
) ProcessResult {
	result := ProcessResult{TotalRows: len(rows)}

	// Group rows berdasarkan outbound_no atau shipment_id
	type outboundGroup struct {
		header ParsedRow
		items  []ParsedRow
	}

	groupMap := make(map[string]*outboundGroup)
	groupOrder := []string{}

	for i, row := range rows {
		// Apply column mapping
		mapped, err := ApplyColumnMapping(row, intg.ColumnMapping)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, ProcessError{
				Row:     i + 1,
				Message: "Gagal apply column mapping",
				Detail:  err.Error(),
			})
			continue
		}

		// Tentukan group key
		groupKey := getStr(mapped, "outbound_no")
		if groupKey == "" {
			groupKey = getStr(mapped, "shipment_id")
		}
		if groupKey == "" {
			groupKey = getStr(mapped, "order_number")
		}
		if groupKey == "" {
			// Tiap row = outbound sendiri
			groupKey = fmt.Sprintf("__row_%d__", i)
		}

		if _, exists := groupMap[groupKey]; !exists {
			groupMap[groupKey] = &outboundGroup{header: mapped}
			groupOrder = append(groupOrder, groupKey)
		}
		groupMap[groupKey].items = append(groupMap[groupKey].items, mapped)
	}

	// Proses per group
	repo := repositories.NewOutboundRepository(db)

	for _, key := range groupOrder {
		group := groupMap[key]
		outboundNo, err := createSingleOutbound(db, repo, intg, group.header, group.items, userID)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, ProcessError{
				Row:     0,
				Message: fmt.Sprintf("Gagal buat outbound untuk group '%s'", key),
				Detail:  err.Error(),
			})
			continue
		}
		result.SuccessCount++
		result.OutboundNos = append(result.OutboundNos, outboundNo)
	}

	return result
}

func createSingleOutbound(
	db *gorm.DB,
	repo *repositories.OutboundRepository,
	intg integrationModel.Integration,
	header ParsedRow,
	items []ParsedRow,
	userID int,
) (string, error) {
	tx := db.Begin()
	if tx.Error != nil {
		return "", fmt.Errorf("gagal start transaksi: %w", tx.Error)
	}

	// Generate outbound number
	outboundNo, err := repo.GenerateOutboundNumber()
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("gagal generate outbound number: %w", err)
	}

	now := time.Now()

	// Build outbound header dari mapping
	outboundHeader := models.OutboundHeader{
		OutboundNo:      outboundNo,
		OutboundDate:    getStrDefault(header, "outbound_date", now.Format("2006-01-02")),
		OwnerCode:       getStrDefault(header, "owner_code", ""),
		CustomerCode:    getStr(header, "customer_code"),
		WhsCode:         getStr(header, "whs_code"),
		ShipmentID:      getStr(header, "shipment_id"),
		Remarks:         getStr(header, "remarks"),
		DelivTo:         getStr(header, "deliv_to"),
		DelivAddress:    getStr(header, "deliv_address"),
		DelivCity:       getStr(header, "deliv_city"),
		Driver:          getStr(header, "driver"),
		TruckNo:         getStr(header, "truck_no"),
		TransporterCode: getStr(header, "transporter_code"),
		AwbNo:           getStr(header, "awb_no"),
		PickerName:      getStr(header, "picker_name"),
		User_Def1:       getStr(header, "user_def1"),
		User_Def2:       getStr(header, "user_def2"),
		User_Def3:       getStr(header, "user_def3"),
		User_Def4:       getStr(header, "user_def4"),
		User_Def5:       getStr(header, "user_def5"),
		Status:          "open",
		RawStatus:       "DRAFT",
		DraftTime:       now,
		Integration:     true,
		Source:          "INTEGRATION:" + intg.Name,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}

	// Validasi field wajib
	if outboundHeader.OwnerCode == "" {
		tx.Rollback()
		return "", fmt.Errorf("owner_code wajib diisi")
	}

	if err := tx.Create(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return "", fmt.Errorf("gagal create outbound header: %w", err)
	}

	// Create outbound details
	for i, item := range items {
		// Cari product
		itemCode := getStr(item, "item_code")
		if itemCode == "" {
			itemCode = getStr(item, "sku")
		}
		if itemCode == "" {
			tx.Rollback()
			return "", fmt.Errorf("baris %d: item_code / sku kosong", i+1)
		}

		var product models.Product
		if err := tx.First(&product, "item_code = ?", itemCode).Error; err != nil {
			tx.Rollback()
			return "", fmt.Errorf("baris %d: produk '%s' tidak ditemukan", i+1, itemCode)
		}

		// Cari UOM
		var uomConversion models.UomConversion
		if err := tx.Where("item_code = ? AND factor = 1", itemCode).First(&uomConversion).Error; err != nil {
			if err2 := tx.Where("item_code = ?", itemCode).First(&uomConversion).Error; err2 != nil {
				tx.Rollback()
				return "", fmt.Errorf("baris %d: UOM tidak ditemukan untuk SKU '%s'", i+1, itemCode)
			}
		}

		// Parse quantity
		qtyStr := getStr(item, "quantity")
		if qtyStr == "" {
			qtyStr = getStr(item, "qty")
		}
		qty, err := strconv.ParseFloat(qtyStr, 64)
		if err != nil || qty <= 0 {
			tx.Rollback()
			return "", fmt.Errorf("baris %d: quantity tidak valid '%s'", i+1, qtyStr)
		}

		detail := models.OutboundDetail{
			OutboundNo:   outboundNo,
			OutboundID:   outboundHeader.ID,
			ItemCode:     product.ItemCode,
			ItemID:       int(product.ID),
			Barcode:      getStrDefault(item, "barcode", uomConversion.Ean),
			CustomerCode: outboundHeader.CustomerCode,
			OwnerCode:    outboundHeader.OwnerCode,
			WhsCode:      outboundHeader.WhsCode,
			DivisionCode: getStrDefault(item, "division_code", "REGULAR"),
			Uom:          getStrDefault(item, "uom", uomConversion.FromUom),
			Quantity:     qty,
			QaStatus:     getStrDefault(item, "qa_status", "A"),
			LotNumber:    getStr(item, "lot_number"),
			ExpDate:      getStr(item, "exp_date"),
			ProdDate:     getStr(item, "prod_date"),
			Remarks:      getStr(item, "remarks"),
			SNCheck:      "N",
			Status:       "draft",
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}

		if err := tx.Create(&detail).Error; err != nil {
			tx.Rollback()
			return "", fmt.Errorf("baris %d: gagal create outbound detail: %w", i+1, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return "", fmt.Errorf("gagal commit transaksi: %w", err)
	}

	return outboundNo, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getStr(row ParsedRow, key string) string {
	if v, ok := row[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}

func getStrDefault(row ParsedRow, key, defaultVal string) string {
	v := getStr(row, key)
	if v == "" {
		return defaultVal
	}
	return v
}
