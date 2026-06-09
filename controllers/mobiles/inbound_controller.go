package mobiles

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobileInboundController struct {
	DB *gorm.DB
}

func NewMobileInboundController(DB *gorm.DB) *MobileInboundController {
	return &MobileInboundController{DB: DB}
}

func (c *MobileInboundController) GetListInbound(ctx *fiber.Ctx) error {
	type listInboundResponse struct {
		ID                 uint      `json:"id"`
		InboundNo          string    `json:"inbound_no"`
		SupplierName       string    `json:"supplier_name"`
		ReceiptID          string    `json:"receipt_id"`
		ReqQty             int       `json:"req_qty"`
		ScanQty            int       `json:"scan_qty"`
		QtyStock           int       `json:"qty_stock"`
		Status             string    `json:"status"`
		RequirePutawayScan bool      `json:"require_putaway_scan"`
		UpdatedAt          time.Time `json:"updated_at"`
	}

	sql := `WITH id AS
	(SELECT inbound_id, SUM(quantity) AS req_qty
	-- , SUM(scan_qty) as scan_qty 
	FROM inbound_details
	GROUP BY inbound_id),

	ib AS (select inbound_id, SUM(quantity) AS qty_stock 
	from inbound_barcodes
	where status = 'in stock'
	group by inbound_id),
	
	ibp AS (select inbound_id, SUM(quantity) AS scan_qty 
	from inbound_barcodes
	group by inbound_id)

	SELECT a.id, a.inbound_no, b.supplier_name, a.receipt_id,
	COALESCE(id.req_qty, 0) as req_qty, COALESCE(ibp.scan_qty, 0) as scan_qty, 
	COALESCE(ib.qty_stock,0) as qty_stock, ip.require_putaway_scan,
	a.status, a.updated_at 
	FROM inbound_headers a
	INNER JOIN suppliers b ON a.supplier_id = b.id
	LEFT JOIN id ON a.id = id.inbound_id
	LEFT JOIN ib ON a.id = ib.inbound_id
	LEFT JOIN ibp ON a.id = ibp.inbound_id
	LEFT JOIN inventory_policies ip ON a.owner_code = ip.owner_code
	WHERE a.status IN ('checking', 'partially received', 'fully received')
	ORDER by a.id DESC`

	var listInbound []listInboundResponse
	if err := c.DB.Raw(sql).Scan(&listInbound).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": listInbound})
}

func (c *MobileInboundController) CheckItem(ctx *fiber.Ctx) error {
	var scanInbound struct {
		InboundNo string `json:"inboundNo"`
		Location  string `json:"location"`
		Barcode   string `json:"barcode"`
		Sku       string `json:"sku"`
	}

	if err := ctx.BodyParser(&scanInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", scanInbound.InboundNo).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found", "message": "Inbound not found"})
	}

	var uomConversion models.UomConversion

	if scanInbound.Sku != "" {
		// Cari by item_code
		if err := c.DB.Where("item_code = ?", scanInbound.Sku).First(&uomConversion).Error; err != nil {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
		}

		// Jika ean masih pakai item_code sebagai placeholder → update ke EAN real
		if uomConversion.Ean == scanInbound.Sku {
			if err := c.DB.Model(&uomConversion).Update("ean", scanInbound.Barcode).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update EAN", "message": err.Error()})
			}

			// Update juga barcode & gmc di products
			if err := c.DB.Model(&models.Product{}).
				Where("item_code = ? AND barcode = ? AND gmc = ?", scanInbound.Sku, scanInbound.Sku, scanInbound.Sku).
				Updates(map[string]interface{}{
					"barcode": scanInbound.Barcode,
					"gmc":     scanInbound.Barcode,
				}).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update product barcode", "message": err.Error()})
			}

			// Update barcode di inbound_details yang masih pakai item_code sebagai placeholder
			if err := c.DB.Model(&models.InboundDetail{}).
				Where("inbound_no = ? AND item_code = ? AND barcode = ?", scanInbound.InboundNo, scanInbound.Sku, scanInbound.Sku).
				Update("barcode", scanInbound.Barcode).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update inbound detail barcode", "message": err.Error()})
			}
		}

	} else {
		// Flow normal: cari by EAN hasil scan
		if err := c.DB.Where("ean = ?", scanInbound.Barcode).First(&uomConversion).Error; err != nil {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
		}
	}

	// if err := c.DB.Where("ean = ?", scanInbound.Barcode).
	// 	First(&uomConversion).Error; err != nil {
	// 	return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	// }

	var inventory models.Inventory
	if err := c.DB.Where("pallet = ?", scanInbound.Location).First(&inventory).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	if inventory.ID > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Pallet " + scanInbound.Location + " already putaway", "message": "Pallet " + scanInbound.Location + " already putaway"})
	}

	var inboundDetail []models.InboundDetail

	if errID := c.DB.Where("inbound_id = ? AND item_code = ? AND uom = ?", inboundHeader.ID, uomConversion.ItemCode, uomConversion.FromUom).
		Find(&inboundDetail).Error; errID != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errID.Error()})
	}

	if len(inboundDetail) < 1 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in inbound details", "message": "Item not found in inbound details"})
	}

	var product models.Product
	if err := c.DB.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
	}

	if product.HasSerial == "Y" {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success":   true,
			"message":   "Item checked successfully",
			"data":      inboundDetail,
			"is_serial": true,
		})
	}

	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", inboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if inventoryPolicy.UseFEFO {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success":   true,
			"message":   "Item checked successfully",
			"is_serial": false,
			"is_fefo":   true,
			"data":      inboundDetail,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":   true,
		"message":   "Item checked successfully",
		"data":      inboundDetail,
		"is_serial": false,
	})
}

func (c *MobileInboundController) ScanInbound(ctx *fiber.Ctx) error {

	var scanInbound struct {
		ID           int      `json:"id"`
		InboundNo    string   `json:"inboundNo"`
		Location     string   `json:"location"`
		Sku          string   `json:"sku"`
		Barcode      string   `json:"barcode"`
		ScanType     string   `json:"scanType"`
		WhsCode      string   `json:"whsCode"`
		QaStatus     string   `json:"qaStatus"`
		Serial       string   `json:"serial"`
		QtyScan      float64  `json:"qtyScan"`
		ProdDate     string   `json:"prodDate"`
		ExpDate      string   `json:"expDate"`
		LotNo        string   `json:"lotNo"`
		QrRaw        string   `json:"qrRaw"`
		Uploaded     bool     `json:"uploaded"`
		InnerSerials []string `json:"innerSerials"`
		CaseNumber   string   `json:"caseNumber"`
		ItemModel    string   `json:"itemModel"`
	}

	if err := ctx.BodyParser(&scanInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// start db transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var inboundHeader models.InboundHeader
	if err := tx.Where("inbound_no = ?", scanInbound.InboundNo).First(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found", "message": "Inbound not found"})
	}

	if inboundHeader.Status == "open" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound is still open, cannot scan items"})
	}

	if inboundHeader.Status == "complete" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already complete"})
	}

	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", inboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var product models.Product
	var uomConversion models.UomConversion

	if scanInbound.Sku != "" {

		if err := tx.Where("item_code = ? AND owner_code = ?", scanInbound.Sku, inboundHeader.OwnerCode).First(&product).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
		}

		if err := tx.Where("item_code = ? AND ean = ?", scanInbound.Sku, product.Barcode).First(&uomConversion).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
		}

	} else {

		if err := tx.Where("owner_code = ? AND barcode = ?", inboundHeader.OwnerCode, scanInbound.Barcode).First(&product).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
		}

		if err := tx.Where("ean = ?", product.Barcode).First(&uomConversion).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
		}
	}

	if inventoryPolicy.UseFEFO && scanInbound.ExpDate == "" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Expiration date is required for FEFO items", "message": "Expiration date is required for FEFO items"})
	}

	if inventoryPolicy.UseLotNo && scanInbound.LotNo == "" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Lot number is required", "message": "Lot number is required"})
	}

	queryInboundDetail := tx.Debug().Model(&models.InboundDetail{}).
		Where("inbound_no = ? AND item_code = ? ", scanInbound.InboundNo, product.ItemCode)

	if inventoryPolicy.ValidateReceiveScan {
		if inventoryPolicy.RequireExpiryDate {
			// kalau pakai lot number
			queryInboundDetail = queryInboundDetail.Where("exp_date = ?", scanInbound.ExpDate)
		}

		if inventoryPolicy.UseLotNo {
			// kalau pakai lot number
			queryInboundDetail = queryInboundDetail.Where("lot_number = ? ", scanInbound.LotNo)
		}

		if inventoryPolicy.UseProductionDate {
			// kalau production date-based
			queryInboundDetail = queryInboundDetail.Where("prod_date = ?", scanInbound.ProdDate)
		}
	}

	var inboundDetail models.InboundDetail
	if err := queryInboundDetail.First(&inboundDetail).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Item not found in inbound details",
			"message": "Item not found in inbound details",
			"detail":  err.Error(),
		})
	}

	var checkPalletInboundBarcode models.InboundBarcode
	if err := tx.Debug().Where("inbound_id = ? AND pallet = ? AND status = ?", inboundHeader.ID, scanInbound.Location, "in stock").First(&checkPalletInboundBarcode).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if checkPalletInboundBarcode.ID > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Pallet " + scanInbound.Location + " already putaway", "message": "Pallet " + scanInbound.Location + " already putaway"})
	}

	inboundBarcodes := []models.InboundBarcode{}
	if err := tx.Where("inbound_detail_id = ?", inboundDetail.ID).Find(&inboundBarcodes).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	qtyScanned := 0.0

	for _, item := range inboundBarcodes {
		qtyScanned += item.Quantity
	}

	if inboundDetail.Quantity < scanInbound.QtyScan+qtyScanned {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity exceeds planned receipt", "message": "Quantity exceeds planned receipt"})
	}

	inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.UpdatedAt = time.Now()

	userID := int(ctx.Locals("userID").(float64))
	scanType := "SERIAL"

	// if product.HasSerial == "N" {
	// 	scanType = "BARCODE"
	// 	scanInbound.Serial = scanInbound.Barcode
	// }

	// ── Case 1: CARTON dengan inner serial range ──────────────────────────
	if len(scanInbound.InnerSerials) > 0 {

		// Validasi total qty tidak melebihi plan
		if inboundDetail.Quantity < scanInbound.QtyScan+qtyScanned {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Quantity exceeds planned receipt",
				"message": "Quantity exceeds planned receipt",
			})
		}

		for _, sn := range scanInbound.InnerSerials {
			// Cek duplikat serial
			var existing models.InboundBarcode

			// if scanInbound.Serial != "" && product.HasSerial == "Y" {
			if scanInbound.Serial != "" {

				if err := tx.Where("item_code = ? AND serial_number = ? AND inbound_id = ? AND case_number = ?", product.ItemCode, sn, inboundHeader.ID, scanInbound.CaseNumber).
					First(&existing).Error; err == nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error":   "Serial number already scanned: " + sn + " in carton " + scanInbound.CaseNumber,
						"message": "Serial number already scanned: " + sn + " in carton " + scanInbound.CaseNumber,
					})
				}

			}

			record := buildInboundBarcode(
				inboundHeader, inboundDetail, product, scanInbound.ItemModel,
				scanInbound.Location, product.Barcode,
				sn, scanType,
				1, // qty per serial = 1
				scanInbound.ProdDate, scanInbound.ExpDate,
				scanInbound.LotNo, scanInbound.QrRaw,
				scanInbound.CaseNumber,
				userID,
			)
			// Simpan case number di ScanData jika ada
			if scanInbound.CaseNumber != "" {
				record.CaseNumber = scanInbound.CaseNumber
			}

			if err := tx.Create(&record).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}

		// ── Case 2: Single scan (serial biasa atau barcode) — behavior lama ───
	} else {
		var checkInboundBarcode models.InboundBarcode
		if product.HasSerial == "Y" || scanInbound.Serial != "" {

			if err := tx.Debug().Where("item_code = ? AND barcode = ? AND serial_number = ? AND inbound_id = ?", product.ItemCode, product.Barcode, scanInbound.Serial, inboundHeader.ID).
				First(&checkInboundBarcode).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

		if checkInboundBarcode.ID > 0 && scanType == "SERIAL" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Serial number already scanned",
				"message": "Serial number already scanned",
			})
		}

		// Check Case Number Already Scanned
		if scanInbound.CaseNumber != "" {

			var existingCaseNumber models.InboundBarcode
			if err := tx.Where("item_code = ? AND case_number = ? AND inbound_id = ?", product.ItemCode, scanInbound.CaseNumber, inboundHeader.ID).
				First(&existingCaseNumber).Error; err == nil {
				tx.Rollback()

				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error":   "Carton number already scanned: " + scanInbound.CaseNumber,
					"message": "Carton number already scanned: " + scanInbound.CaseNumber,
				})
			}
		}

		record := buildInboundBarcode(
			inboundHeader, inboundDetail, product, scanInbound.ItemModel,
			scanInbound.Location, product.Barcode,
			scanInbound.Serial, scanType,
			scanInbound.QtyScan,
			scanInbound.ProdDate, scanInbound.ExpDate,
			scanInbound.LotNo, scanInbound.QrRaw,
			scanInbound.CaseNumber,
			userID,
		)

		if err := tx.Create(&record).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Scan item success (%d records)", func() int {
			if len(scanInbound.InnerSerials) > 0 {
				return len(scanInbound.InnerSerials)
			}
			return 1
		}()),
	})
}

func buildInboundBarcode(
	header models.InboundHeader,
	detail models.InboundDetail,
	product models.Product,
	model string,
	location string,
	barcode string,
	serial string,
	scanType string,
	qty float64,
	prodDate string,
	expDate string,
	lotNo string,
	qrRaw string,
	caseNumber string,
	createdBy int,
) models.InboundBarcode {
	return models.InboundBarcode{
		InboundId:       int(header.ID),
		InboundDetailId: int(detail.ID),
		Location:        location,
		Pallet:          location,
		ItemID:          product.ID,
		ItemCode:        product.ItemCode,
		ItemModel:       model,
		Barcode:         barcode,
		ScanType:        scanType,
		WhsCode:         detail.WhsCode,
		OwnerCode:       detail.OwnerCode,
		DivisionCode:    detail.DivisionCode,
		QaStatus:        detail.QaStatus,
		ScanData: func() string {
			if qrRaw != "" {
				return qrRaw
			}
			return serial
		}(),
		CaseNumber:   caseNumber,
		SerialNumber: serial,
		RecDate:      detail.RecDate,
		ProdDate:     prodDate,
		ExpDate:      expDate,
		LotNumber:    lotNo,
		Quantity:     qty,
		Uom:          detail.Uom,
		Status:       "pending",
		CreatedBy:    createdBy,
	}
}

func (c *MobileInboundController) GetInboundDetail(ctx *fiber.Ctx) error {
	inboundNo := ctx.Params("inbound_no")

	// 1. Get inbound header
	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", inboundNo).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	// 2. Get all inbound details
	var inboundDetails []models.InboundDetail
	if err := c.DB.Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundDetails) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": []interface{}{}})
	}

	// 3. Collect item IDs and detail IDs for batch queries
	itemIDs := make([]uint, 0, len(inboundDetails))
	detailIDs := make([]uint, 0, len(inboundDetails))
	for _, d := range inboundDetails {
		itemIDs = append(itemIDs, d.ItemId)
		detailIDs = append(detailIDs, d.ID)
	}

	// 4. Batch fetch products → map by ID
	var products []models.Product
	if err := c.DB.Where("id IN ?", itemIDs).Find(&products).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	productMap := make(map[uint]models.Product, len(products))
	for _, p := range products {
		productMap[p.ID] = p
	}

	// 5. Batch fetch UOM conversions
	//    Build (item_code, uom) pairs per detail for matching
	type uomKey struct {
		ItemCode string
		FromUom  string
	}
	uomKeys := make([]uomKey, 0, len(inboundDetails))
	for _, d := range inboundDetails {
		if p, ok := productMap[d.ItemId]; ok {
			uomKeys = append(uomKeys, uomKey{p.ItemCode, d.Uom})
		}
	}

	// Collect unique item codes to fetch UOM conversions in one query
	itemCodes := make([]string, 0, len(products))
	for _, p := range products {
		itemCodes = append(itemCodes, p.ItemCode)
	}

	var uomConversions []models.UomConversion
	if err := c.DB.Where("item_code IN ?", itemCodes).Find(&uomConversions).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	uomMap := make(map[uomKey]models.UomConversion, len(uomConversions))
	for _, u := range uomConversions {
		uomMap[uomKey{u.ItemCode, u.FromUom}] = u
	}

	// 6. Batch fetch inbound barcodes → map by detail ID
	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Where("inbound_detail_id IN ?", detailIDs).Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	scanQtyMap := make(map[int]float64, len(inboundDetails))
	for _, b := range inboundBarcodes {
		scanQtyMap[b.InboundDetailId] += b.Quantity
	}

	// 7. Build result
	type InboundDetailResult struct {
		models.InboundDetail
		ItemName string  `json:"item_name"`
		IsSerial bool    `json:"is_serial"`
		ScanQty  float64 `json:"scan_qty"`
	}

	result := make([]InboundDetailResult, 0, len(inboundDetails))
	for _, d := range inboundDetails {
		product := productMap[d.ItemId]
		uomConv := uomMap[uomKey{product.ItemCode, d.Uom}]
		d.Barcode = uomConv.Ean

		result = append(result, InboundDetailResult{
			InboundDetail: d,
			ItemName:      product.ItemName,
			IsSerial:      product.HasSerial == "Y",
			ScanQty:       scanQtyMap[int(d.ID)],
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

func (c *MobileInboundController) GetScanInbound(ctx *fiber.Ctx) error {

	id := ctx.Params("id")

	var inboundBarcode []models.InboundBarcode

	if err := c.DB.
		Preload("Product").
		Order("created_at DESC").
		Where("inbound_detail_id = ?", id).
		Find(&inboundBarcode).
		Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inboundBarcode})
}

func (c *MobileInboundController) DeleteScannedInbound(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var inboundBarcode models.InboundBarcode

	if err := c.DB.Where("id = ?", id).First(&inboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	if inboundBarcode.Status != "pending" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already scanned"})
	}

	// start db transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var inboundDetail models.InboundDetail

	if err := tx.Where("id = ?", inboundBarcode.InboundDetailId).First(&inboundDetail).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// inboundDetail.ScanQty -= inboundBarcode.Quantity
	inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.UpdatedAt = time.Now()

	// if err := tx.Where("id = ?", inboundDetail.ID).
	// 	Select("scan_qty", "updated_by").
	// 	Updates(&models.InboundDetail{
	// 		ScanQty:   inboundDetail.ScanQty,
	// 		UpdatedBy: int(ctx.Locals("userID").(float64)),
	// 	}).Error; err != nil {
	// 	tx.Rollback()
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if err := tx.Unscoped().Delete(&inboundBarcode).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Inbound Barcode : ", inboundBarcode)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

func (c *MobileInboundController) GetInboundBarcodeByLocation(ctx *fiber.Ctx) error {

	// get from post body
	var input struct {
		InboundNo string `json:"inbound_no"`
		Location  string `json:"location"`
		Barcode   string `json:"barcode"`
		Quantity  int    `json:"quantity"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if input.InboundNo == "" || input.Location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound No and Location are required"})
	}

	if input.Barcode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Barcode is required"})
	}

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", input.InboundNo).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Where("inbound_id = ? AND location = ? AND barcode = ? AND status = ?", inboundHeader.ID, input.Location, input.Barcode, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// if len(inboundBarcodes) < 1 {
	// 	return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No barcode found for the given location"})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Get Inbound Barcode By Location", "data": inboundBarcodes})
}

func (c *MobileInboundController) EditInboundBarcode(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var input struct {
		ID       int     `json:"id"`
		Quantity float64 `json:"quantity"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	inboundBarcode := models.InboundBarcode{}
	if err := c.DB.Where("id = ?", id).First(&inboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	inboundDetail := models.InboundDetail{}
	if err := c.DB.Where("id = ?", inboundBarcode.InboundDetailId).First(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundBarcodes := []models.InboundBarcode{}
	if err := c.DB.Where("inbound_detail_id = ?", inboundDetail.ID).Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	qtyScanned := 0.0

	for _, item := range inboundBarcodes {
		qtyScanned += item.Quantity
	}

	qtyScanned -= inboundBarcode.Quantity

	if inboundDetail.Quantity < qtyScanned+input.Quantity {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity is not enough"})
	}

	if err := c.DB.Debug().Where("id = ?", inboundBarcode.ID).
		Select("quantity", "updated_by").
		Updates(&models.InboundBarcode{
			Quantity:  input.Quantity,
			UpdatedBy: int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var result models.InboundBarcode
	if err := c.DB.Debug().Where("id = ?", inboundBarcode.ID).First(&result).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Edit quantity successfully", "data": result})
}

func (c *MobileInboundController) GetSequenceLocation(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")
	inboundHeader := models.InboundHeader{}

	// Ambil header inbound
	if err := c.DB.Where("inbound_no = ?", inbound_no).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Inbound not found",
		})
	}

	// inboundNo := inboundHeader.InboundNo
	prefix := inbound_no // gunakan sebagai prefix untuk pencocokan

	// Ambil semua location yang diawali dengan inboundNo
	var barcodes []models.InboundBarcode
	if err := c.DB.Select("location").
		Where("inbound_id = ? AND location LIKE ?", inboundHeader.ID, prefix+"%").
		Find(&barcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Cari urutan tertinggi dari location yang cocok
	maxSequence := 0
	for _, b := range barcodes {
		loc := b.Location
		if strings.HasPrefix(loc, prefix) && len(loc) > len(prefix) {
			suffix := loc[len(prefix):] // ambil bagian setelah prefix
			if seqNum, err := strconv.Atoi(suffix); err == nil {
				if seqNum > maxSequence {
					maxSequence = seqNum
				}
			}
		}
	}

	// Tambah 1 dari max sequence
	newSequence := maxSequence + 1
	sequenceStr := fmt.Sprintf("%03d", newSequence)
	sequenceLocation := prefix + sequenceStr

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Generated sequence location",
		"data":    sequenceLocation,
	})
}

func (c *MobileInboundController) CheckItemPutaway(ctx *fiber.Ctx) error {
	var scanPutaway struct {
		Filter    string `json:"filter"`
		InboundNo string `json:"inbound_no"`
		Pallet    string `json:"pallet"`
	}

	if err := ctx.BodyParser(&scanPutaway); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", scanPutaway.InboundNo).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found", "message": "Inbound not found"})
	}

	var inboundBarcodes []models.InboundBarcode

	switch scanPutaway.Filter {
	case "working":
		if err := c.DB.Debug().Preload("Product").
			Where("inbound_id = ? AND location = ? AND status = ?", inboundHeader.ID, scanPutaway.Pallet, "pending").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	case "pending":
		if err := c.DB.Debug().Preload("Product").
			Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	case "completed":
		if err := c.DB.Debug().Preload("Product").
			Where("inbound_id = ? AND status = ?", inboundHeader.ID, "in stock").
			Order("created_at DESC").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if len(inboundBarcodes) < 1 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pallet not found", "message": "Pallet not found"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Pallet found", "data": fiber.Map{"inbound": inboundBarcodes}})
}

// func (c *MobileInboundController) CheckItemPutaway(ctx *fiber.Ctx) error {
// 	var scanPutaway struct {
// 		Filter    string `json:"filter"`
// 		InboundNo string `json:"inbound_no"`
// 		Pallet    string `json:"pallet"`
// 	}

// 	if err := ctx.BodyParser(&scanPutaway); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var inboundHeader models.InboundHeader
// 	if err := c.DB.Where("inbound_no = ?", scanPutaway.InboundNo).First(&inboundHeader).Error; err != nil {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found", "message": "Inbound not found"})
// 	}

// 	var inboundBarcodes []models.InboundBarcode

// 	switch scanPutaway.Filter {
// 	case "working":
// 		if err := c.DB.Debug().Where("inbound_id = ? AND location = ? AND status = ?", inboundHeader.ID, scanPutaway.Pallet, "pending").
// 			Find(&inboundBarcodes).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	case "pending":
// 		if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").
// 			Find(&inboundBarcodes).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	case "completed":
// 		if err := c.DB.Debug().
// 			Where("inbound_id = ? AND status = ?", inboundHeader.ID, "in stock").
// 			Order("created_at DESC").
// 			Find(&inboundBarcodes).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	if len(inboundBarcodes) < 1 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pallet not found", "message": "Pallet not found"})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Pallet found", "data": fiber.Map{"inbound": inboundBarcodes}})
// }

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

// 	// Transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	inboundHeader := models.InboundHeader{}
// 	if err := tx.Where("inbound_no = ?", req.InboundNo).First(&inboundHeader).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Inbound not found: " + err.Error(),
// 		})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(tx)

// 	if err := tx.Where("location_code = ?", req.Location).First(&models.Location{}).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Location " + req.Location + " not registered: " + err.Error(),
// 		})
// 	}

// 	for _, itemID := range req.ItemIDs {
// 		_, err := inboundRepo.ProcessPutawayItem(ctx, itemID, req.Location)
// 		if err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to process putaway: " + err.Error(),
// 			})
// 		}
// 	}

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
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway item successfully"})
// }

func (c *MobileInboundController) PutawayAll(ctx *fiber.Ctx) error {

	type PutawayPayload struct {
		InboundNo string `json:"inbound_no"`
		ItemIDs   []int  `json:"item_ids"`
		Location  string `json:"location"`
	}

	fnStart := time.Now()
	log.Printf("[PutawayAll] START")

	var req PutawayPayload

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	if req.InboundNo == "" || len(req.ItemIDs) < 1 || req.Location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "inbound_no, item_ids, and location are required",
		})
	}

	log.Printf("[PutawayAll] Payload parsed | inbound_no=%s location=%s item_count=%d | elapsed=%s",
		req.InboundNo, req.Location, len(req.ItemIDs), time.Since(fnStart))

	// Transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// --- Fetch inbound header ---
	t := time.Now()
	inboundHeader := models.InboundHeader{}
	if err := tx.Where("inbound_no = ?", req.InboundNo).First(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Inbound not found: " + err.Error(),
		})
	}
	log.Printf("[PutawayAll] Fetch inbound header | id=%d | took=%s", inboundHeader.ID, time.Since(t))

	inboundRepo := repositories.NewInboundRepository(tx)

	// --- Validate location ---
	t = time.Now()
	if err := tx.Where("location_code = ?", req.Location).First(&models.Location{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Location " + req.Location + " not registered: " + err.Error(),
		})
	}
	log.Printf("[PutawayAll] Validate location | location=%s | took=%s", req.Location, time.Since(t))

	// --- Loop putaway items ---
	t = time.Now()
	for _, itemID := range req.ItemIDs {
		tItem := time.Now()
		_, err := inboundRepo.ProcessPutawayItem(ctx, itemID, req.Location)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to process putaway: " + err.Error(),
			})
		}
		log.Printf("[PutawayAll] ProcessPutawayItem | item_id=%d | took=%s", itemID, time.Since(tItem))
	}
	log.Printf("[PutawayAll] All items putaway done | item_count=%d | took=%s", len(req.ItemIDs), time.Since(t))

	// --- Update inbound status ---
	t = time.Now()
	if err := inboundRepo.UpdateStatusInbound(ctx, inboundHeader.ID); err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update inbound status: " + err.Error(),
		})
	}
	log.Printf("[PutawayAll] UpdateStatusInbound | took=%s", time.Since(t))

	// --- Commit ---
	t = time.Now()
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction: " + err.Error(),
		})
	}
	log.Printf("[PutawayAll] Commit | took=%s", time.Since(t))

	log.Printf("[PutawayAll] DONE | total_elapsed=%s", time.Since(fnStart))
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway item successfully"})
}
