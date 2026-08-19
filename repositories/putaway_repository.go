package repositories

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================
// FILE: putaway_optimized.go
// Berisi 3 function:
//   1. GetScanDataBatch     — di InboundRepository
//   2. ProcessPutawayItemFast — di InboundRepository
//   3. PutawayAll           — di MobileInboundController
// ============================================================

// ─────────────────────────────────────────────────────────────
// 1. GetScanDataBatch
//    Replaces: GetScanData (dipanggil per item di loop lama)
//    Lokasi: inbound_repository.go
// ─────────────────────────────────────────────────────────────

// func (r *InboundRepository) GetScanDataBatch(inboundBarcodeIDs []uint) (map[uint]*ScanDataResult, error) {
// 	if len(inboundBarcodeIDs) == 0 {
// 		return map[uint]*ScanDataResult{}, nil
// 	}

// 	query := `
// 		SELECT
// 			ib.id AS inbound_barcode_id,
// 			ib.inbound_id,
// 			ib.inbound_detail_id,
// 			ib.lot_number,
// 			ib.scan_data AS raw_scan_data,
// 			CASE
// 				WHEN ib.scan_data IS NULL OR LEN(TRIM(ib.scan_data)) = 0 THEN 'INVALID - EMPTY'
// 				WHEN CHARINDEX('(1)SKU=', ib.scan_data) = 0 THEN 'INVALID - UNKNOWN FORMAT'
// 				WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 THEN 'CARTON'
// 				ELSE 'UNIT'
// 			END AS item_type,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(1)SKU=', ib.scan_data) > 0 AND CHARINDEX('(2)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(1)SKU=', ib.scan_data) + 7, CHARINDEX('(2)', ib.scan_data) - CHARINDEX('(1)SKU=', ib.scan_data) - 7)
// 			END AS sku,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(2)EAN=', ib.scan_data) > 0 AND CHARINDEX('(3)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(2)EAN=', ib.scan_data) + 7, CHARINDEX('(3)', ib.scan_data) - CHARINDEX('(2)EAN=', ib.scan_data) - 7)
// 			END AS ean,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(3)PRODUCT=', ib.scan_data) > 0 AND CHARINDEX('(4)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(3)PRODUCT=', ib.scan_data) + 11, CHARINDEX('(4)', ib.scan_data) - CHARINDEX('(3)PRODUCT=', ib.scan_data) - 11)
// 			END AS product,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(4)BRAND=', ib.scan_data) > 0 AND CHARINDEX('(5)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(4)BRAND=', ib.scan_data) + 9, CHARINDEX('(5)', ib.scan_data) - CHARINDEX('(4)BRAND=', ib.scan_data) - 9)
// 			END AS brand,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(5)MODEL=', ib.scan_data) > 0 AND CHARINDEX('(6)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(5)MODEL=', ib.scan_data) + 9, CHARINDEX('(6)', ib.scan_data) - CHARINDEX('(5)MODEL=', ib.scan_data) - 9)
// 			END AS model,
// 			CASE WHEN CHARINDEX('(6)SERIAL=', ib.scan_data) > 0 AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(6)SERIAL=', ib.scan_data) + 10, CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)SERIAL=', ib.scan_data) - 10)
// 			END AS serial,
// 			CASE WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
// 				SUBSTRING(ib.scan_data, CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) + 17, CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) - 17)
// 			END AS carton_serial,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(7)BATCH=', ib.scan_data) > 0 AND CHARINDEX('(8)', ib.scan_data) > 0 THEN
// 				TRIM(SUBSTRING(ib.scan_data, CHARINDEX('(7)BATCH=', ib.scan_data) + 9, CHARINDEX('(8)', ib.scan_data) - CHARINDEX('(7)BATCH=', ib.scan_data) - 9))
// 			END AS batch,
// 			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
// 				TRIM(CASE
// 					WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
// 						SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
// 					ELSE
// 						SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
// 				END)
// 			END AS mfg_date_raw,
// 			TRY_CONVERT(DATE,
// 				CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
// 					TRIM(CASE
// 						WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
// 							SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
// 						ELSE
// 							SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
// 					END)
// 				END
// 			, 112) AS mfg_date,
// 			CASE WHEN CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) > 0 THEN
// 				TRY_CAST(
// 					TRIM(SUBSTRING(ib.scan_data, CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) + 18, LEN(ib.scan_data) - CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) - 17))
// 				AS INT)
// 			END AS qty_per_carton
// 		FROM inbound_barcodes ib
// 		WHERE ib.id IN ?
// 	`

// 	resultMap := make(map[uint]*ScanDataResult)

// 	chunkSize := 2000
// 	for i := 0; i < len(inboundBarcodeIDs); i += chunkSize {
// 		end := i + chunkSize
// 		if end > len(inboundBarcodeIDs) {
// 			end = len(inboundBarcodeIDs)
// 		}
// 		chunk := inboundBarcodeIDs[i:end]

// 		var results []ScanDataResult
// 		if err := r.db.Raw(query, chunk).Scan(&results).Error; err != nil {
// 			return nil, err
// 		}
// 		for j := range results {
// 			resultMap[results[j].InboundBarcodeID] = &results[j]
// 		}
// 	}

// 	return resultMap, nil
// }

// ─────────────────────────────────────────────────────────────
// 2. ProcessPutawayItemFast
//    Replaces: ProcessPutawayItem
//    Logika identik, tapi semua lookup data sudah di-pass dari luar
//    (tidak ada lagi query fetch barcode/detail/product/scandata)
//    Lokasi: inbound_repository.go
// ─────────────────────────────────────────────────────────────

func (r *InboundRepository) ProcessPutawayItemFast(
	ctx *fiber.Ctx,
	barcode models.InboundBarcode,
	detail models.InboundDetail,
	product models.Product,
	uomConv UomConversionResult,
	cartonSerial string,
	location string,
	userID int,
) (bool, error) {
	movementID := uuid.NewString()
	qtyConverted := uomConv.QtyConverted

	if location == "" {
		location = barcode.Location
	}

	// Cek apakah data inventory dengan kombinasi yang sama sudah ada
	var existingInv models.Inventory
	invQuery := r.db.Where(`
			inbound_id = ? AND
			inbound_detail_id = ? AND
			item_code = ? AND
			location = ? AND
			barcode = ? AND
			whs_code = ? AND
			qa_status = ? AND
			rec_date = ? AND
			COALESCE(prod_date, '') = COALESCE(?, '') AND
			COALESCE(exp_date, '') = COALESCE(?, '') AND
			COALESCE(lot_number, '') = COALESCE(?, '') AND
			COALESCE(carton_number, '') = COALESCE(?, '')
		`,
		barcode.InboundId,
		barcode.InboundDetailId,
		barcode.ItemCode,
		location,
		product.Barcode,
		barcode.WhsCode,
		barcode.QaStatus,
		barcode.RecDate,
		barcode.ProdDate,
		barcode.ExpDate,
		barcode.LotNumber,
		cartonSerial,
	).First(&existingInv)

	if errors.Is(invQuery.Error, gorm.ErrRecordNotFound) {
		// Tidak ada data → Insert baru
		newInv := models.Inventory{
			InboundID:       detail.InboundId,
			InboundDetailId: int(detail.ID),
			RecDate:         detail.RecDate,
			ItemId:          barcode.ItemID,
			ItemCode:        barcode.ItemCode,
			Barcode:         product.Barcode,
			WhsCode:         barcode.WhsCode,
			OwnerCode:       barcode.OwnerCode,
			DivisionCode:    barcode.DivisionCode,
			Pallet:          barcode.Pallet,
			Location:        location,
			CartonNumber:    cartonSerial,
			QaStatus:        barcode.QaStatus,
			Uom:             uomConv.ToUom,
			QtyOrigin:       qtyConverted,
			QtyOnhand:       qtyConverted,
			QtyAvailable:    qtyConverted,
			ExpDate:         barcode.ExpDate,
			ProdDate:        barcode.ProdDate,
			LotNumber:       barcode.LotNumber,
			Trans:           "INBOUND PUTAWAY",
			CreatedBy:       userID,
		}

		if err := r.db.Create(&newInv).Error; err != nil {
			return false, err
		}

		helpers.InsertInventoryMovement(r.db, helpers.InventoryMovementPayload{
			InventoryID:        newInv.ID,
			MovementID:         movementID,
			RefType:            "INBOUND PUTAWAY",
			RefID:              uint(barcode.InboundId),
			ItemID:             product.ID,
			ItemCode:           product.ItemCode,
			ToWhsCode:          newInv.WhsCode,
			QtyOnhandChange:    qtyConverted,
			QtyAvailableChange: qtyConverted,
			FromLocation:       barcode.Location,
			NewQaStatus:        barcode.QaStatus,
			ToLocation:         location,
			Reason:             detail.InboundNo + " PUTAWAY",
			CreatedBy:          userID,
		})

	} else if invQuery.Error == nil {
		// Sudah ada → Update qty
		if err := r.db.Model(&existingInv).Updates(map[string]interface{}{
			"qty_origin":    existingInv.QtyOrigin + qtyConverted,
			"qty_onhand":    existingInv.QtyOnhand + qtyConverted,
			"qty_available": existingInv.QtyAvailable + qtyConverted,
			"updated_at":    time.Now().UTC(),
			"updated_by":    userID,
		}).Error; err != nil {
			return false, err
		}

		helpers.InsertInventoryMovement(r.db, helpers.InventoryMovementPayload{
			InventoryID:        existingInv.ID,
			MovementID:         movementID,
			RefType:            "INBOUND PUTAWAY",
			RefID:              uint(barcode.InboundId),
			ItemID:             product.ID,
			ItemCode:           product.ItemCode,
			ToWhsCode:          existingInv.WhsCode,
			QtyOnhandChange:    qtyConverted,
			QtyAvailableChange: qtyConverted,
			NewQaStatus:        barcode.QaStatus,
			FromLocation:       barcode.Location,
			ToLocation:         location,
			Reason:             detail.InboundNo + " PUTAWAY",
			CreatedBy:          userID,
		})
	} else {
		return false, invQuery.Error
	}

	// Update status barcode ke "in stock"
	if err := r.db.Model(&barcode).Updates(map[string]interface{}{
		"status":           "in stock",
		"putaway_location": location,
		"putaway_qty":      barcode.Quantity,
		"putaway_at":       time.Now().UTC(),
		"putaway_by":       userID,
		"updated_at":       time.Now().UTC(),
		"updated_by":       userID,
	}).Error; err != nil {
		return false, err
	}

	return true, nil
}

// ─────────────────────────────────────────────────────────────
// 3. PutawayAll (optimized)
//    Replaces: PutawayAll lama
//    Lokasi: mobile_inbound_controller.go
// ─────────────────────────────────────────────────────────────

// func (c *MobileInboundController) PutawayAll(ctx *fiber.Ctx) error {

// 	type PutawayPayload struct {
// 		InboundNo string `json:"inbound_no"`
// 		ItemIDs   []int  `json:"item_ids"`
// 		Location  string `json:"location"`
// 	}

// 	var req PutawayPayload
// 	if err := ctx.BodyParser(&req); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body: " + err.Error(),
// 		})
// 	}

// 	if req.InboundNo == "" || len(req.ItemIDs) < 1 || req.Location == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "inbound_no, item_ids, and location are required",
// 		})
// 	}

// 	userID, ok := ctx.Locals("userID").(float64)
// 	if !ok {
// 		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "invalid user ID",
// 		})
// 	}

// 	// Transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	// Validasi inbound header
// 	var inboundHeader models.InboundHeader
// 	if err := tx.Where("inbound_no = ?", req.InboundNo).First(&inboundHeader).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Inbound not found: " + err.Error(),
// 		})
// 	}

// 	// Validasi location
// 	if err := tx.Where("location_code = ?", req.Location).First(&models.Location{}).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Location " + req.Location + " not registered: " + err.Error(),
// 		})
// 	}

// 	// ════════════════════════════════════════════════════════
// 	// BATCH PRE-FETCH — semua lookup query dijalankan sekali
// 	// ════════════════════════════════════════════════════════

// 	// 1. Fetch semua barcodes sekaligus, pastikan semua "pending"
// 	var barcodes []models.InboundBarcode
// 	if err := tx.Where("id IN ? AND status = ?", req.ItemIDs, "pending").
// 		Find(&barcodes).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch barcodes: " + err.Error(),
// 		})
// 	}
// 	if len(barcodes) != len(req.ItemIDs) {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": fmt.Sprintf(
// 				"Expected %d pending items, found %d. Some items may not exist or are not in pending status.",
// 				len(req.ItemIDs), len(barcodes),
// 			),
// 		})
// 	}

// 	// Build lookup structures dari barcodes
// 	barcodeMap := make(map[int]models.InboundBarcode, len(barcodes))
// 	detailIDSet := make(map[int]struct{})
// 	itemCodeSet := make(map[string]struct{})
// 	barcodeUIDs := make([]uint, 0, len(barcodes))

// 	for _, b := range barcodes {
// 		barcodeMap[int(b.ID)] = b
// 		detailIDSet[b.InboundDetailId] = struct{}{}
// 		itemCodeSet[b.ItemCode] = struct{}{}
// 		barcodeUIDs = append(barcodeUIDs, uint(b.ID))
// 	}

// 	// Flatten ke slice untuk query IN
// 	detailIDs := make([]int, 0, len(detailIDSet))
// 	for id := range detailIDSet {
// 		detailIDs = append(detailIDs, id)
// 	}
// 	itemCodes := make([]string, 0, len(itemCodeSet))
// 	for code := range itemCodeSet {
// 		itemCodes = append(itemCodes, code)
// 	}

// 	// 2. Fetch semua inbound details
// 	var details []models.InboundDetail
// 	if err := tx.Where("id IN ?", detailIDs).Find(&details).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch inbound details: " + err.Error(),
// 		})
// 	}
// 	detailMap := make(map[int]models.InboundDetail, len(details))
// 	for _, d := range details {
// 		detailMap[int(d.ID)] = d
// 	}

// 	// 3. Fetch semua products
// 	var products []models.Product
// 	if err := tx.Where("item_code IN ?", itemCodes).Find(&products).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch products: " + err.Error(),
// 		})
// 	}
// 	productMap := make(map[string]models.Product, len(products))
// 	for _, p := range products {
// 		productMap[p.ItemCode] = p
// 	}

// 	// 4. Build UOM conversion map — per unique item_code+fromUom kombinasi
// 	//    Key: "itemCode|fromUom" agar beda UOM di detail yang sama tidak tabrakan
// 	uomRepo := repositories.NewUomRepository(tx)
// 	type uomKey struct {
// 		ItemCode string
// 		FromUom  string
// 	}
// 	uomMap := make(map[uomKey]UomConversionResult)
// 	for _, b := range barcodes {
// 		detail, ok := detailMap[b.InboundDetailId]
// 		if !ok {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": fmt.Sprintf("detail not found for barcode id %d", b.ID),
// 			})
// 		}
// 		key := uomKey{ItemCode: b.ItemCode, FromUom: detail.Uom}
// 		if _, exists := uomMap[key]; !exists {
// 			conv, err := uomRepo.ConversionQty(b.ItemCode, b.Quantity, detail.Uom)
// 			if err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": fmt.Sprintf("UOM conversion failed for %s: %s", b.ItemCode, err.Error()),
// 				})
// 			}
// 			uomMap[key] = conv
// 		}
// 	}

// 	// 5. Fetch semua scan data sekaligus (1 query, bukan N query)
// 	inboundRepo := repositories.NewInboundRepository(tx)
// 	scanDataMap, err := inboundRepo.GetScanDataBatch(barcodeUIDs)
// 	if err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch scan data: " + err.Error(),
// 		})
// 	}

// 	// ════════════════════════════════════════════════════════
// 	// LOOP — semua data sudah di-memory, tidak ada lagi lookup query
// 	// ════════════════════════════════════════════════════════

// 	for _, itemID := range req.ItemIDs {
// 		barcode := barcodeMap[itemID]
// 		detail := detailMap[barcode.InboundDetailId]
// 		product := productMap[barcode.ItemCode]
// 		uomConv := uomMap[uomKey{ItemCode: barcode.ItemCode, FromUom: detail.Uom}]

// 		cartonSerial := ""
// 		if sd, ok := scanDataMap[uint(barcode.ID)]; ok && sd.CartonSerial != nil {
// 			cartonSerial = *sd.CartonSerial
// 		}

// 		_, err := inboundRepo.ProcessPutawayItemFast(
// 			ctx,
// 			barcode,
// 			detail,
// 			product,
// 			uomConv,
// 			cartonSerial,
// 			req.Location,
// 			int(userID),
// 		)
// 		if err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": fmt.Sprintf("Failed to process item %d: %s", itemID, err.Error()),
// 			})
// 		}
// 	}

// 	// Update status inbound header
// 	if err := inboundRepo.UpdateStatusInbound(ctx, inboundHeader.ID); err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update inbound status: " + err.Error(),
// 		})
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction: " + err.Error(),
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Putaway item successfully",
// 	})
// }
