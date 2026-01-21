package mobiles

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
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
	}

	if err := ctx.BodyParser(&scanInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", scanInbound.InboundNo).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found", "message": "Inbound not found"})
	}

	var uomConversion models.UomConversion
	if err := c.DB.Where("ean = ?", scanInbound.Barcode).
		First(&uomConversion).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	}

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
		ID        int     `json:"id"`
		InboundNo string  `json:"inboundNo"`
		Location  string  `json:"location"`
		Barcode   string  `json:"barcode"`
		ScanType  string  `json:"scanType"`
		WhsCode   string  `json:"whsCode"`
		QaStatus  string  `json:"qaStatus"`
		Serial    string  `json:"serial"`
		QtyScan   float64 `json:"qtyScan"`
		ProdDate  string  `json:"prodDate"`
		ExpDate   string  `json:"expDate"`
		LotNo     string  `json:"lotNo"`
		Uploaded  bool    `json:"uploaded"`
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

	if inboundHeader.Status == "complete" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already complete"})
	}

	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", inboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var uomConversion models.UomConversion
	if err := tx.Where("ean = ?", scanInbound.Barcode).First(&uomConversion).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	}

	var product models.Product
	if err := tx.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
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
		Where("inbound_no = ? AND item_code = ?", scanInbound.InboundNo, product.ItemCode)

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

	var checkInboundBarcode models.InboundBarcode
	// if err := tx.Debug().Where("inbound_id = ? AND item_code = ? AND serial_number = ?", inboundHeader.ID, product.ItemCode, scanInbound.Serial).First(&checkInboundBarcode).Error; err != nil {
	// 	if !errors.Is(err, gorm.ErrRecordNotFound) {
	// 		tx.Rollback()
	// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// 	}
	// }
	if err := tx.Debug().Where("item_code = ? AND serial_number = ?", product.ItemCode, scanInbound.Serial).First(&checkInboundBarcode).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var scanType = "SERIAL"

	if product.HasSerial == "N" {
		scanType = "BARCODE"
		scanInbound.Serial = scanInbound.Barcode
	}

	if checkInboundBarcode.ID > 0 && scanType == "SERIAL" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number already scanned", "message": "Serial number already scanned"})
	}

	var inboundBarcode = models.InboundBarcode{
		InboundId:       int(inboundHeader.ID),
		InboundDetailId: int(inboundDetail.ID),
		Location:        scanInbound.Location,
		Pallet:          scanInbound.Location,
		ItemID:          product.ID,
		ItemCode:        product.ItemCode,
		Barcode:         scanInbound.Barcode,
		ScanType:        scanType,
		WhsCode:         inboundDetail.WhsCode,
		OwnerCode:       inboundDetail.OwnerCode,
		DivisionCode:    inboundDetail.DivisionCode,
		// QaStatus:        scanInbound.QaStatus,
		QaStatus:     inboundDetail.QaStatus,
		ScanData:     scanInbound.Serial,
		SerialNumber: scanInbound.Serial,
		RecDate:      inboundDetail.RecDate,
		ProdDate:     scanInbound.ProdDate,
		ExpDate:      scanInbound.ExpDate,
		LotNumber:    scanInbound.LotNo,
		Quantity:     scanInbound.QtyScan,
		Uom:          inboundDetail.Uom,
		Status:       "pending",
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := tx.Create(&inboundBarcode).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Scan item success"})
}

func (c *MobileInboundController) GetInboundDetail(ctx *fiber.Ctx) error {

	inbound_no := ctx.Params("inbound_no")

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", inbound_no).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	var inboundDetail []models.InboundDetail
	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type InboundDetailResult struct {
		models.InboundDetail
		IsSerial bool    `json:"is_serial"`
		ScanQty  float64 `json:"scan_qty"`
	}

	var result []InboundDetailResult
	for _, v := range inboundDetail {

		var product models.Product
		isSerial := false

		if err := c.DB.Where("id = ?", v.ItemId).First(&product).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var uomConvesion models.UomConversion
		if err := c.DB.Where("item_code = ? AND from_uom = ?", product.ItemCode, v.Uom).First(&uomConvesion).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if product.HasSerial == "Y" {
			isSerial = true
		}

		var inboundBarcode []models.InboundBarcode
		if err := c.DB.Where("inbound_detail_id = ?", v.ID).Find(&inboundBarcode).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var scanQty float64

		for _, item := range inboundBarcode {
			if int(v.ID) == int(item.InboundDetailId) {
				scanQty += item.Quantity
			}
		}

		v.Barcode = uomConvesion.Ean
		result = append(result, InboundDetailResult{
			InboundDetail: v,
			ScanQty:       scanQty,
			IsSerial:      isSerial,
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
		if err := c.DB.Debug().Where("inbound_id = ? AND location = ? AND status = ?", inboundHeader.ID, scanPutaway.Pallet, "pending").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	case "pending":
		if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	case "completed":
		if err := c.DB.Debug().
			Where("inbound_id = ? AND status = ?", inboundHeader.ID, "in stock").
			Order("created_at DESC").
			Find(&inboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// if err := c.DB.Debug().Where("inbound_id = ? AND location = ? AND status = ?", inboundHeader.ID, scanPutaway.Pallet, "pending").
	// 	Find(&inboundBarcodes).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if len(inboundBarcodes) < 1 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pallet not found", "message": "Pallet not found"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Pallet found", "data": fiber.Map{"inbound": inboundBarcodes}})
}

func (c *MobileInboundController) PutawayAll(ctx *fiber.Ctx) error {

	type PutawayPayload struct {
		InboundNo string `json:"inbound_no"`
		ItemIDs   []int  `json:"item_ids"`
		Location  string `json:"location"`
	}

	var req PutawayPayload

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	if len(req.ItemIDs) < 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Item IDs are required",
		})
	}

	// Transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	inboundHeader := models.InboundHeader{}
	if err := tx.Where("inbound_no = ?", req.InboundNo).First(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Inbound not found: " + err.Error(),
		})
	}

	inboundRepo := repositories.NewInboundRepository(tx)

	for _, itemID := range req.ItemIDs {

		var loc models.Location
		if err := tx.Where("location_code = ?", req.Location).First(&loc).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Location " + req.Location + " not registered: " + err.Error(),
			})
		}

		_, err := inboundRepo.ProcessPutawayItem(ctx, itemID, req.Location)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to process putaway: " + err.Error(),
			})
		}
	}

	if err := inboundRepo.UpdateStatusInbound(ctx, inboundHeader.ID); err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update inbound status: " + err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction: " + err.Error(),
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway item successfully"})
}
