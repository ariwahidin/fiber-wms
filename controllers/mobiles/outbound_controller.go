package mobiles

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobileOutboundController struct {
	DB *gorm.DB
}

func NewMobileOutboundController(DB *gorm.DB) *MobileOutboundController {
	return &MobileOutboundController{DB: DB}
}

func (c *MobileOutboundController) GetListOutbound(ctx *fiber.Ctx) error {
	type listOutboundResponse struct {
		ID                 uint      `json:"id"`
		OutboundNo         string    `json:"outbound_no"`
		CustomerName       string    `json:"customer_name"`
		Status             string    `json:"status"`
		ShipmentID         string    `json:"shipment_id"`
		QtyReq             int       `json:"qty_req"`
		QtyScan            int       `json:"qty_scan"`
		QtyPack            int       `json:"qty_pack"`
		PickingWithScanner bool      `json:"picking_with_scanner"`
		RequirePickingScan bool      `json:"require_picking_scan"`
		UpdatedAt          time.Time `json:"updated_at"`
	}

	sql := `WITH 
	od AS
		(SELECT outbound_id, SUM(quantity) qty_req, SUM(scan_qty) as scan_qty 
		FROM outbound_details
		GROUP BY outbound_id),
	kd AS (
		SELECT outbound_id, SUM(quantity) AS qty_pack
		FROM outbound_barcodes
		GROUP BY outbound_id
		),
	os AS(
		SELECT outbound_id, SUM(quantity) AS qty_scan
		FROM outbound_picking_scans
		WHERE deleted_at IS NULL
		GROUP BY outbound_id
	)
		SELECT a.id, a.outbound_no, b.customer_name,
		a.shipment_id, od.qty_req, os.qty_scan, kd.qty_pack,
		a.status, a.updated_at, ipo.require_picking_scan, ipo.picking_with_scanner
		FROM outbound_headers a
		INNER JOIN customers b ON a.customer_code = b.customer_code
		LEFT JOIN od ON a.id = od.outbound_id	
		LEFT JOIN kd ON a.id = kd.outbound_id
		LEFT JOIN os ON a.id = os.outbound_id
		LEFT JOIN inventory_policies ipo ON a.owner_code = ipo.owner_code
		WHERE a.status IN ('picking', 'packing') and ipo.require_picking_scan <> 0
		ORDER BY a.id DESC;`
	var listOutbound []listOutboundResponse
	if err := c.DB.Raw(sql).Scan(&listOutbound).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(listOutbound) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": []interface{}{}})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": listOutbound})
}

func (c *MobileOutboundController) GetListOutboundDetail(ctx *fiber.Ctx) error {

	outbound_no := ctx.Params("outbound_no")

	if outbound_no == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "outbound_no is required"})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().Where("outbound_no = ?", outbound_no).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound_no not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type OutboundResult struct {
		OutboundDetailID int     `json:"outbound_detail_id"`
		WhsCode          string  `json:"whs_code"`
		ItemCode         string  `json:"item_code"`
		Barcode          string  `json:"barcode"`
		Quantity         int     `json:"quantity"`
		ItemName         string  `json:"item_name"`
		HasSerial        string  `json:"has_serial"`
		QtyScan          float64 `json:"scan_qty"`
		UOM              string  `json:"uom"`
		OwnerCode        string  `json:"owner_code"`
	}

	var results []OutboundResult

	query := `WITH ob AS (
			SELECT a.outbound_id, a.item_code, a.barcode, a.item_id, a.outbound_detail_id,
			COALESCE(SUM(a.quantity),0) as qty_scan
			FROM outbound_barcodes a
			LEFT JOIN outbound_details b ON a.outbound_detail_id = b.id
			WHERE a.outbound_id = ?
			GROUP BY a.item_code, a.barcode, a.item_id, a.outbound_id, a.outbound_detail_id
		),
		op AS (
			SELECT a.outbound_detail_id, a.item_code, sum(a.quantity) as qty, a.uom 
			FROM outbound_pickings a
			WHERE a.outbound_id = ?
			GROUP BY a.outbound_detail_id, a.item_code, a.uom 
		),
		opb AS (
			SELECT a.id as outbound_detail_id, a.whs_code, a.item_code, a.barcode,
				a.quantity, 
				a.uom,
				b.item_name, 
				COALESCE(ob.qty_scan, 0) as qty_scan, 
				b.has_serial, 
				a.owner_code
			FROM outbound_details a
			INNER JOIN products b ON a.item_id = b.id
			LEFT JOIN op ON a.id = op.outbound_detail_id
			LEFT JOIN ob ON ob.outbound_id = a.outbound_id AND a.item_code = ob.item_code AND a.id = ob.outbound_detail_id
			WHERE a.outbound_id = ?
		)
		select 
		opb.outbound_detail_id,
		opb.whs_code,
		opb.item_code,
		opb.barcode,
		opb.quantity,
		opb.uom,
		opb.item_name,
		ROUND(opb.qty_scan / oc.conversion_rate, 3) AS qty_scan,
		opb.owner_code
		from opb
		left join uom_conversions oc ON oc.item_code = opb.item_code and oc.ean = opb.barcode and opb.uom = oc.from_uom`

	err := c.DB.Raw(query, outboundHeader.ID, outboundHeader.ID, outboundHeader.ID).Scan(&results).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": results})
}

func (c *MobileOutboundController) CheckItem(ctx *fiber.Ctx) error {

	outbound_no := ctx.Params("outbound_no")

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().Where("outbound_no = ?", outbound_no).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound_no not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var scanOutbound struct {
		PackingNo  string `json:"packing_no"`
		OutboundNo string `json:"outbound_no"`
		Barcode    string `json:"barcode"`
		Sku        string `json:"sku"`
		Qty        int    `json:"qty"`
	}

	if err := ctx.BodyParser(&scanOutbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var packing models.OutboundPacking

	if scanOutbound.PackingNo != "" {
		if err := c.DB.Debug().Where("packing_no = ?", scanOutbound.PackingNo).First(&packing).Error; err != nil {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Packing No not found", "message": "Packing No not found"})
		}
	}

	var product models.Product

	if scanOutbound.Sku != "" {
		if err := c.DB.Where("item_code = ?", scanOutbound.Sku).First(&product).Error; err != nil {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
		}

	} else {
		if err := c.DB.Where("barcode = ?", scanOutbound.Barcode).First(&product).Error; err != nil {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
		}
	}

	var uomConversion models.UomConversion
	if err := c.DB.Where("ean = ?", product.Barcode).First(&uomConversion).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	}

	if product.HasSerial == "Y" {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "Item checked successfully",
			"data": fiber.Map{
				"product": product,
				"uom":     uomConversion,
			},
			"is_serial": true,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Item checked successfully",
		"data": fiber.Map{
			"product": product,
			"uom":     uomConversion,
		},
		"is_serial": false,
	})
}

func (c *MobileOutboundController) ScanPicking(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().Where("outbound_no = ?", outbound_no).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound_no not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var scanOutbound struct {
		PackingNo  string  `json:"packing_no"`
		PackCtnNo  string  `json:"pack_ctn_no"`
		Location   string  `json:"location"`
		OutboundNo string  `json:"outbound_no"`
		Barcode    string  `json:"barcode"`
		SerialNo   string  `json:"serial_no"`
		Qty        float64 `json:"qty"`
		Uom        string  `json:"uom"`
		CartonID   uint    `json:"carton_id"`
		CartonCode string  `json:"carton_code"`
		QrRaw      string  `json:"qr_raw"`
		LotNo      string  `json:"lot_no"`    // ← tambah
		ProdDate   string  `json:"prod_date"` // ← tambah
	}

	if err := ctx.BodyParser(&scanOutbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println(scanOutbound)

	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Debug().Where("owner_code = ?", outboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inventory policy not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundRepo = repositories.NewOutboundRepository(c.DB)
	var packings []models.OutboundPacking
	var packing models.OutboundPacking

	if inventoryPolicy.RequirePackingScan {
		if scanOutbound.PackingNo != "" {

			if err := c.DB.Debug().Where("packing_no = ?", scanOutbound.PackingNo).Find(&packings).Error; err != nil {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Packing No not found", "message": "Packing No not found"})
			}

			if len(packings) == 0 {
				packing.PackingNo = scanOutbound.PackingNo
				packing.CreatedAt = time.Now()
				packing.CreatedBy = int(ctx.Locals("userID").(float64))
				if err := c.DB.Create(&packing).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"message": "Failed to create packing",
						"error":   err.Error(),
					})
				}
			} else {
				packing = packings[0]
			}
		} else {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Packing number is required"})
		}

		if scanOutbound.PackCtnNo == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CTN is required"})
		}

	}

	var uomConversion models.UomConversion
	if err := c.DB.Where("ean = ?", scanOutbound.Barcode).First(&uomConversion).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	}

	uomRepo := repositories.NewUomRepository(c.DB)
	uom, errUOM := uomRepo.ConversionQty(uomConversion.ItemCode, scanOutbound.Qty, uomConversion.FromUom)

	if errUOM != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errUOM.Error(), "message": errUOM.Error()})
	}

	var outboundDetail models.OutboundDetail
	if err := c.DB.Where("outbound_id = ? AND item_code = ?", outboundHeader.ID, uomConversion.ItemCode).First(&outboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in outbound detail", "message": "Item not found in outbound detail"})
	}

	var product models.Product
	if err := c.DB.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
	}

	if product.HasSerial == "Y" {
		var outboundBarcodes []models.OutboundBarcode

		if err := c.DB.Where("outbound_id = ? AND barcode = ? AND serial_number = ?", outboundHeader.ID, scanOutbound.Barcode, scanOutbound.SerialNo).Find(&outboundBarcodes).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if len(outboundBarcodes) > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Item already scanned", "data": outboundBarcodes, "is_serial": true})
		}

		fmt.Println("Inventory Policy Validation SN:", inventoryPolicy.ValidationSN)
		if inventoryPolicy.ValidationSN {
			_, err := outboundRepo.ValidateSerialNumber(product.ItemCode, scanOutbound.SerialNo, int(outboundHeader.ID))
			if err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	queryOutboundPicking := c.DB.Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode)

	var outboundPicking models.OutboundPicking

	if err := queryOutboundPicking.First(&outboundPicking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Picking not found", "message": "Picking not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// helper parse field dari QR raw
	// parseQRField := func(raw, field string) string {
	// 	key := field + "="
	// 	parts := strings.SplitN(raw, key, 2)
	// 	if len(parts) < 2 {
	// 		return ""
	// 	}
	// 	// trim sampai field berikutnya "(" atau end of string
	// 	value := strings.SplitN(parts[1], "(", 2)[0]
	// 	return strings.TrimSpace(value)
	// }

	var serialNumber string

	// if product.HasSerial == "N" {
	// 	serialNumber = product.Barcode
	// } else {
	// 	serialNumber = scanOutbound.SerialNo
	// }

	if product.HasSerial == "N" {
		serialNumber = func() string {
			if scanOutbound.QrRaw != "" {
				return scanOutbound.SerialNo
			}
			return scanOutbound.Barcode
		}()
	} else {
		serialNumber = scanOutbound.SerialNo
	}

	type PickingSum struct {
		QtyPickingList int
	}

	var result PickingSum

	err := c.DB.Table("outbound_pickings").
		Select("COALESCE(SUM(quantity), 0) as qty_picking_list").
		Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode).
		Scan(&result).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type Result struct {
		QtyBarcode int
	}

	var res Result

	errBarcode := c.DB.Table("outbound_barcodes").
		Select("COALESCE(SUM(quantity), 0) AS qty_barcode").
		Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode).
		Scan(&res).Error

	if errBarcode != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errBarcode.Error()})
	}

	if res.QtyBarcode+int(uom.QtyConverted) > result.QtyPickingList {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity exceeds the limit"})
	}

	var carton models.MasterCarton
	errCarton := c.DB.Where("id = ?", scanOutbound.CartonID).First(&carton).Error
	if errCarton != nil {
		if errors.Is(errCarton, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Carton not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errCarton.Error()})
	}

	outboundBarcode := models.OutboundBarcode{
		OutboundId:       outboundHeader.ID,
		OutboundNo:       outboundHeader.OutboundNo,
		PackingId:        packing.ID,
		PackingNo:        packing.PackingNo,
		PackCtnNo:        scanOutbound.PackCtnNo,
		OutboundDetailId: outboundPicking.OutboundDetailId,
		ItemID:           int(product.ID),
		ItemCode:         product.ItemCode,
		Barcode:          product.Barcode,
		Uom:              product.Uom,
		SerialNumber:     serialNumber,
		Quantity:         uom.QtyConverted,
		Status:           "pending",
		BarcodeDataScan:  scanOutbound.Barcode,
		DataScan: func() string {
			if scanOutbound.QrRaw != "" {
				return scanOutbound.QrRaw
			}
			return scanOutbound.Barcode
		}(),
		ProdDate:      scanOutbound.ProdDate,
		LotNumber:     scanOutbound.LotNo,
		QtyDataScan:   scanOutbound.Qty,
		LocationScan:  scanOutbound.Location,
		UomScan:       uomConversion.FromUom,
		IsSerial:      product.HasSerial == "Y",
		CartonID:      scanOutbound.CartonID,
		CartonCode:    scanOutbound.CartonCode,
		CtnLength:     carton.Length,
		CtnWidth:      carton.Width,
		CtnHeight:     carton.Height,
		CtnVolume:     carton.Volume,
		CtnMaxWeight:  carton.MaxWeight,
		CtnTareWeight: carton.TareWeight,
		CreatedBy:     int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&outboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	outboundHeader.Status = "packing"
	outboundHeader.RawStatus = "PACKING"
	outboundHeader.ConfirmTime = time.Now()
	outboundHeader.ConfirmBy = int(ctx.Locals("userID").(float64))
	outboundHeader.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Save(&outboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update outbound header: " + err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item scanned successfully"})
}

func (c *MobileOutboundController) GetListOutboundBarcode(ctx *fiber.Ctx) error {

	id := ctx.Params("id")

	var outboundBarcodes []models.OutboundBarcode

	if err := c.DB.Where("outbound_detail_id = ?", id).Find(&outboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": outboundBarcodes})
}

func (c *MobileOutboundController) GetPickingList(ctx *fiber.Ctx) error {

	outbound_no := ctx.Params("outbound_no")

	var pickingList []models.OutboundPicking

	if err := c.DB.Where("outbound_no = ?", outbound_no).Find(&pickingList).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": pickingList})
}

func (c *MobileOutboundController) OverridePicking(ctx *fiber.Ctx) error {

	picking_list_id := ctx.Params("id")

	var newPicking struct {
		PickingListID int    `json:"picking_list_id"`
		NewBarcode    string `json:"new_barcode"`
		NewLocation   string `json:"new_location"`
		NewQty        int    `json:"new_qty"`
		Reason        string `json:"reason"`
	}

	if err := ctx.BodyParser(&newPicking); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var oldPickingList models.OutboundPicking

	if err := tx.Where("id = ?", picking_list_id).First(&oldPickingList).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var findInventory models.Inventory

	if err := tx.Where("location = ? AND barcode = ? AND whs_code = ? AND qa_status = ? AND qty_available > 0", newPicking.NewLocation, newPicking.NewBarcode, oldPickingList.WhsCode, oldPickingList.QaStatus).First(&findInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "message": "Inventory not found"})
	}

	if findInventory.QtyAvailable < float64(newPicking.NewQty) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not enough"})
	}

	var oldInventory models.Inventory

	if err := tx.Where("id = ?", oldPickingList.InventoryID).First(&oldInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// kembalikan stok lama
	if err := tx.Debug().
		Model(&models.Inventory{}).
		Where("id = ?", oldPickingList.InventoryID).
		Updates(map[string]interface{}{
			"qty_available": gorm.Expr("qty_available + ?", newPicking.NewQty),
			"qty_allocated": gorm.Expr("qty_allocated - ?", newPicking.NewQty),
			"updated_by":    int(ctx.Locals("userID").(float64)),
			"updated_at":    time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update inventory",
		})
	}

	// ambil stok baru
	if err := tx.Debug().
		Model(&models.Inventory{}).
		Where("id = ?", findInventory.ID).
		Updates(map[string]interface{}{
			"qty_available": gorm.Expr("qty_available - ?", newPicking.NewQty),
			"qty_allocated": gorm.Expr("qty_allocated + ?", newPicking.NewQty),
			"updated_by":    int(ctx.Locals("userID").(float64)),
			"updated_at":    time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update inventory",
		})
	}

	// update old picking list
	if err := tx.Debug().
		Model(&models.OutboundPicking{}).
		Where("id = ?", oldPickingList.ID).
		Updates(map[string]interface{}{
			"quantity":   oldPickingList.Quantity - float64(newPicking.NewQty),
			"reason":     newPicking.Reason + " [old]",
			"updated_by": int(ctx.Locals("userID").(float64)),
			"updated_at": time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update inventory",
		})
	}

	// create new picking list
	pickingSheet := models.OutboundPicking{
		InventoryID:      int(findInventory.ID),
		OutboundId:       oldPickingList.OutboundId,
		OutboundNo:       oldPickingList.OutboundNo,
		OutboundDetailId: int(oldPickingList.OutboundDetailId),
		OwnerCode:        findInventory.OwnerCode,
		ItemID:           oldPickingList.ItemID,
		Barcode:          findInventory.Barcode,
		ItemCode:         findInventory.ItemCode,
		Pallet:           findInventory.Pallet,
		Location:         findInventory.Location,
		Quantity:         float64(newPicking.NewQty),
		WhsCode:          findInventory.WhsCode,
		QaStatus:         findInventory.QaStatus,
		Reason:           newPicking.Reason + " [new]",
		CreatedBy:        int(ctx.Locals("userID").(float64)),
	}

	if err := tx.Create(&pickingSheet).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create picking sheet",
		})
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": pickingSheet, "message": "Picking list updated successfully"})
}

func (c *MobileOutboundController) DeleteOutboundBarcode(ctx *fiber.Ctx) error {
	idBarcode := ctx.Params("id")

	var outboundBarcodes models.OutboundBarcode

	if err := c.DB.Where("id = ?", idBarcode).First(&outboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
	}

	if outboundBarcodes.Status != "pending" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item cannot be deleted"})
	}

	// Hard Delete
	if err := c.DB.Where("id = ?", idBarcode).Unscoped().Delete(&models.OutboundBarcode{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

func (c *MobileOutboundController) GetCartonNoByOutboundNo(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	// Validasi parameter
	if outboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Outbound number is required",
		})
	}

	var cartons []struct {
		PackCtnNo       string  `json:"pack_ctn_no"`
		Quantity        float64 `json:"qty"`
		Count           int64   `json:"count"`
		CartonID        uint    `json:"carton_id"`
		CtnActualWeight float64 `json:"ctn_actual_weight"`
		CtnStatus       string  `json:"ctn_status"`
	}

	// Query untuk mendapatkan PackCtnNo yang di-group by
	err := c.DB.Model(&models.OutboundBarcode{}).
		Select("pack_ctn_no, SUM(quantity) as quantity, COUNT(*) as count, carton_id, ctn_actual_weight, ctn_status").
		Where("outbound_no = ? AND pack_ctn_no != ? AND pack_ctn_no != ?", outboundNo, "", "0").
		Group("pack_ctn_no, carton_id, ctn_actual_weight, ctn_status").
		Order("pack_ctn_no ASC").
		Find(&cartons).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch carton data",
			"error":   err.Error(),
		})
	}

	// Return response
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Carton data retrieved successfully",
		"data": fiber.Map{
			"outbound_no": outboundNo,
			"cartons":     cartons,
			"total":       len(cartons),
		},
	})
}

// GetCarton - Get cartons for specific outbound
func (c *MobileOutboundController) GetCarton(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	if outboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Outbound number is required",
		})
	}

	var cartons []struct {
		PackCtnNo string `json:"pack_ctn_no"`
		Qty       int    `json:"qty"`
		Count     int    `json:"count"`
	}

	// Query untuk mendapatkan list karton berdasarkan outbound_no
	// Sesuaikan dengan struktur tabel Anda
	err := c.DB.Table("outbound_details").
		Select("pack_ctn_no, SUM(qty) as qty, COUNT(DISTINCT item_id) as count").
		Where("outbound_no = ? AND pack_ctn_no IS NOT NULL AND pack_ctn_no != ''", outboundNo).
		Group("pack_ctn_no").
		Order("pack_ctn_no ASC").
		Scan(&cartons).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch carton data",
			"error":   err.Error(),
		})
	}

	total := 0
	for _, carton := range cartons {
		total += carton.Qty
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Carton data retrieved successfully",
		"data": fiber.Map{
			"outbound_no": outboundNo,
			"cartons":     cartons,
			"total":       total,
		},
	})
}

// GetMasterCartons - Get list of active master cartons
func (c *MobileOutboundController) GetMasterCartons(ctx *fiber.Ctx) error {
	var masterCartons []models.MasterCarton

	// Query master cartons yang aktif, diurutkan berdasarkan default dan nama
	err := c.DB.Where("is_active = ?", true).
		Order("is_default DESC, carton_name ASC").
		Find(&masterCartons).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch master cartons",
			"error":   err.Error(),
		})
	}

	// Convert to response format
	responses := make([]models.MasterCartonResponse, len(masterCartons))
	for i, mc := range masterCartons {
		responses[i] = mc.ToResponse()
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Master cartons retrieved successfully",
		"data":    responses,
	})
}

// GetMasterCartonByID - Get specific master carton by ID
func (c *MobileOutboundController) GetMasterCartonByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var masterCarton models.MasterCarton

	err := c.DB.Where("id = ? AND is_active = ?", id, true).
		First(&masterCarton).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Master carton not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch master carton",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Master carton retrieved successfully",
		"data":    masterCarton.ToResponse(),
	})
}

// UpdateCartonTypeRequest - Request body untuk update carton type
type UpdateCartonTypeRequest struct {
	OutboundNo  string `json:"outbound_no" validate:"required"`
	PackCtnNo   string `json:"pack_ctn_no" validate:"required"`
	NewCartonID uint   `json:"new_carton_id" validate:"required"`
}

// EditCartonTypeByOrderNoAndPackNo - Update carton type untuk semua item dalam carton tertentu
func (c *MobileOutboundController) EditCartonTypeByOrderNoAndPackNo(ctx *fiber.Ctx) error {
	var req UpdateCartonTypeRequest

	// Parse request body
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validasi required fields
	if req.OutboundNo == "" || req.PackCtnNo == "" || req.NewCartonID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "outbound_no, pack_ctn_no, and new_carton_id are required",
		})
	}

	var carton models.MasterCarton
	err := c.DB.Where("id = ?", req.NewCartonID).
		First(&carton).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Master carton not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch master carton",
			"error":   err.Error(),
		})
	}

	var outboundBarcode models.OutboundBarcode
	err = c.DB.Where("outbound_no = ? AND pack_ctn_no = ?", req.OutboundNo, req.PackCtnNo).
		First(&outboundBarcode).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch outbound barcodes",
			"error":   err.Error(),
		})
	}

	err = c.DB.Model(&models.OutboundBarcode{}).
		Where("outbound_id = ? AND pack_ctn_no = ?", outboundBarcode.OutboundId, outboundBarcode.PackCtnNo).
		Updates(map[string]interface{}{
			"carton_id":       req.NewCartonID,
			"carton_code":     carton.CartonCode,
			"ctn_length":      carton.Length,
			"ctn_width":       carton.Width,
			"ctn_height":      carton.Height,
			"ctn_volume":      carton.Volume,
			"ctn_max_weight":  carton.MaxWeight,
			"ctn_tare_weight": carton.TareWeight,
			"updated_at":      time.Now(),
			"updated_by":      int(ctx.Locals("userID").(float64)),
		}).Error

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Carton type updated successfully",
	})
}

type SealCartonTypeRequest struct {
	OutboundNo string  `json:"outbound_no" validate:"required"`
	PackCtnNo  string  `json:"ctn_no" validate:"required"`
	PackingNo  string  `json:"packing_no" validate:"required"`
	Weight     float64 `json:"weight" validate:"required"`
}

func (c *MobileOutboundController) SealCarton(ctx *fiber.Ctx) error {
	var req SealCartonTypeRequest

	// Parse request body
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validasi required fields
	if req.OutboundNo == "" || req.PackCtnNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "outbound_no, pack_ctn_no, and packing_no are required",
		})
	}

	var outboundBarcode models.OutboundBarcode
	err := c.DB.Where("outbound_no = ? AND pack_ctn_no = ?", req.OutboundNo, req.PackCtnNo).
		First(&outboundBarcode).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch outbound barcodes",
			"error":   err.Error(),
		})
	}

	err = c.DB.Model(&models.OutboundBarcode{}).
		Where("outbound_id = ? AND pack_ctn_no = ? AND packing_no = ?", outboundBarcode.OutboundId, outboundBarcode.PackCtnNo, req.PackingNo).
		Updates(map[string]interface{}{
			"ctn_status":        "sealed",
			"ctn_actual_weight": req.Weight,
			"updated_at":        time.Now(),
			"updated_by":        int(ctx.Locals("userID").(float64)),
		}).Error

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Carton sealed successfully",
	})
}

func (c *MobileOutboundController) GetItemInCartonByOutbound(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	if outboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Outbound number is required",
		})
	}

	var cartons []struct {
		PackCtnNo string `json:"pack_ctn_no" gorm:"pack_ctn_no"`
		ItemCode  string `json:"item_code" gorm:"item_code"`
		Barcode   string `json:"barcode" gorm:"barcode"`
		TotalQty  int    `json:"total_qty" gorm:"total_qty"`
	}

	// Query untuk mendapatkan list karton berdasarkan outbound_no
	err := c.DB.Table("outbound_barcodes").
		Select(`
        pack_ctn_no,
        SUM(quantity) AS total_qty,
		item_code,
		barcode,
        COUNT(DISTINCT item_id) AS total_item
    `).
		Where(`
        outbound_no = ?
        AND pack_ctn_no IS NOT NULL
        AND pack_ctn_no <> ''
    `, outboundNo).
		Group("pack_ctn_no, item_code, barcode").
		Order("pack_ctn_no ASC").
		Scan(&cartons).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch carton data",
			"error":   err.Error(),
		})
	}

	total := 0
	for _, carton := range cartons {
		total += carton.TotalQty
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Carton item data retrieved successfully",
		"data": fiber.Map{
			"outbound_no": outboundNo,
			"cartons":     cartons,
			"total":       total,
		},
	})
}

func (c *MobileOutboundController) NewCarton(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	var maxCtnNo string
	err := c.DB.Model(&models.OutboundBarcode{}).
		Where("outbound_no = ?", outboundNo).
		// Select("COALESCE(MAX(CAST(pack_ctn_no AS UNSIGNED)), 0)").
		Select("COALESCE(MAX(CAST(pack_ctn_no AS BIGINT)), 0)").
		Scan(&maxCtnNo).Error

	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get carton data",
		})
	}

	maxNo, _ := strconv.Atoi(maxCtnNo)
	nextNo := maxNo + 1

	return ctx.JSON(fiber.Map{
		"success":     true,
		"last_ctn_no": maxNo,
		"next_ctn_no": nextNo,
	})
}

// PICKING STARTS HERE
// package mobiles

// Tambahkan handler-handler ini ke MobileOutboundController yang sudah ada.
// File: /controller/mobile/outbound_controller.go

// Import tambahan yang diperlukan (merge dengan import yang sudah ada):
//   "fiber-app/repositories"
//   "fiber-app/models"

// ─── Request/Response Types ───────────────────────────────────────────────────

type PickingScanRequest struct {
	OutboundPickingID int     `json:"outbound_picking_id"` // wajib: dari picking sheet
	Barcode           string  `json:"barcode"`             // EAN yang di-scan
	BarcodeRaw        string  `json:"barcode_raw"`         // raw QR (opsional)
	ScanType          string  `json:"scan_type"`           // "EAN" | "QR_UNIT" | "QR_CARTON"
	LabelType         string  `json:"label_type"`          // "UNIT" | "CARTON" | ""
	Quantity          float64 `json:"quantity"`
	Location          string  `json:"location"`
	SerialNumber      string  `json:"serial_number"`
	CaseNumber        string  `json:"case_number"`
	LotNumber         string  `json:"lot_number"`
	ProdDate          string  `json:"prod_date"`
}

// ─── GET /api/wms/picking/:outbound_no ───────────────────────────────────────
// Ambil picking sheet beserta progress per item.

func (c *MobileOutboundController) GetPickingSheet(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")
	if outboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "outbound_no is required",
		})
	}

	repo := repositories.NewOutboundPickingRepository(c.DB)
	rows, err := repo.GetPickingSheet(outboundNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	// Hitung summary
	var totalRequired, totalPicked float64
	for _, r := range rows {
		totalRequired += r.QtyRequired
		totalPicked += r.QtyPicked
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    rows,
		"summary": fiber.Map{
			"total_required": totalRequired,
			"total_picked":   totalPicked,
			"is_complete":    totalPicked >= totalRequired && totalRequired > 0,
		},
	})
}

// ─── POST /api/wms/picking/:outbound_no/scan ─────────────────────────────────
// Submit hasil scan satu item/carton.

func (c *MobileOutboundController) SubmitPickingScan(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	var req PickingScanRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validasi field wajib
	if req.OutboundPickingID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "outbound_picking_id wajib diisi",
		})
	}
	if req.Barcode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "barcode wajib diisi",
		})
	}
	if req.Quantity <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "quantity harus lebih dari 0",
		})
	}

	repo := repositories.NewOutboundPickingRepository(c.DB)

	// ── Ambil OutboundPicking yang di-refer ──────────────────────────────────
	var picking models.OutboundPicking
	if err := c.DB.First(&picking, req.OutboundPickingID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Picking item tidak ditemukan",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	// Pastikan outbound_no match
	if picking.OutboundNo != outboundNo {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Picking item tidak sesuai dengan outbound ini",
		})
	}

	// ── Validasi barcode cocok dengan picking sheet ───────────────────────────
	// Toleransi: EAN scan boleh cocok ke barcode atau ean_display di picking
	if picking.Barcode != req.Barcode && picking.EanDisplay != req.Barcode {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Barcode %s tidak sesuai dengan item yang harus dipick (%s)", req.Barcode, picking.Barcode),
		})
	}

	// ── Cek over-pick ────────────────────────────────────────────────────────
	qtyPicked, err := repo.GetQtyPicked(req.OutboundPickingID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	if qtyPicked+req.Quantity > picking.Quantity {
		remaining := picking.Quantity - qtyPicked
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Over-pick! Qty tersisa: %.0f, scan qty: %.0f", remaining, req.Quantity),
		})
	}

	// ── Cek duplikat serial (jika ada) ───────────────────────────────────────
	if req.SerialNumber != "" {
		var dupCount int64
		c.DB.Model(&models.OutboundPickingScan{}).
			Where("outbound_no = ? AND serial_number = ? AND deleted_at IS NULL", outboundNo, req.SerialNumber).
			Count(&dupCount)
		if dupCount > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Serial number %s sudah pernah di-scan", req.SerialNumber),
			})
		}
	}

	// ── Tentukan scan_type jika kosong ───────────────────────────────────────
	scanType := req.ScanType
	if scanType == "" {
		if req.BarcodeRaw != "" {
			if req.LabelType == "CARTON" {
				scanType = "QR_CARTON"
			} else {
				scanType = "QR_UNIT"
			}
		} else {
			scanType = "EAN"
		}
	}

	// ── Ambil user session (opsional, graceful jika tidak ada) ───────────────
	userID := 0
	if uid, ok := ctx.Locals("user_id").(int); ok {
		userID = uid
	}

	// ── Validasi lokasi jika inventory policy mengharuskan ─────────────────
	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ? AND whs_code = ?", picking.OwnerCode, picking.WhsCode).First(&inventoryPolicy).Error; err == nil {
		if inventoryPolicy.RequireScanPickLocation {
			if req.Location == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location is required"})
			}
		}
	} else {
		// Jika tidak ada inventory policy, default ke require location
		inventoryPolicy.RequireScanPickLocation = true
	}

	queryOutboundPicking := c.DB.Where("outbound_id = ? AND barcode = ?", picking.OutboundId, picking.Barcode).Where("deleted_at IS NULL")

	if inventoryPolicy.RequireScanPickLocation {
		queryOutboundPicking = queryOutboundPicking.Where("location = ?", req.Location)
	}

	var outboundPicking models.OutboundPicking

	if err := queryOutboundPicking.First(&outboundPicking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Picking not found", "message": "Picking not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// ── Buat scan record ─────────────────────────────────────────────────────
	scan := &models.OutboundPickingScan{
		OutboundID:        uint(picking.OutboundId),
		OutboundNo:        outboundNo,
		OutboundDetailID:  picking.OutboundDetailId,
		OutboundPickingID: req.OutboundPickingID,
		OwnerCode:         picking.OwnerCode,
		WhsCode:           picking.WhsCode,
		ItemID:            picking.ItemID,
		ItemCode:          picking.ItemCode,
		Barcode:           req.Barcode,
		BarcodeRaw:        req.BarcodeRaw,
		ScanType:          scanType,
		LabelType:         req.LabelType,
		Quantity:          req.Quantity,
		Uom:               picking.Uom,
		Location:          req.Location,
		SerialNumber:      req.SerialNumber,
		CaseNumber:        req.CaseNumber,
		LotNumber:         req.LotNumber,
		ProdDate:          req.ProdDate,
		Status:            "pending",
		CreatedBy:         userID,
	}

	if err := repo.CreateScan(scan); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal menyimpan scan: " + err.Error(),
		})
	}

	// Hitung ulang progress setelah insert
	newQtyPicked := qtyPicked + req.Quantity
	isItemComplete := newQtyPicked >= picking.Quantity

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Successfully scanned item",
		"data": fiber.Map{
			"scan_id":          scan.ID,
			"qty_picked":       newQtyPicked,
			"qty_required":     picking.Quantity,
			"is_item_complete": isItemComplete,
		},
	})
}

// ─── DELETE /api/wms/picking/scan/:scan_id ───────────────────────────────────

func (c *MobileOutboundController) DeletePickingScan(ctx *fiber.Ctx) error {
	scanIDStr := ctx.Params("scan_id")
	scanID, err := strconv.Atoi(scanIDStr)
	if err != nil || scanID <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "scan_id tidak valid",
		})
	}

	repo := repositories.NewOutboundPickingRepository(c.DB)
	if err := repo.DeleteScan(uint(scanID)); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Scan dihapus",
	})
}

// ─── POST /api/wms/picking/:outbound_no/confirm ──────────────────────────────
// Confirm picking setelah semua item complete → status outbound jadi 'packing'.

func (c *MobileOutboundController) ConfirmPicking(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	repo := repositories.NewOutboundPickingRepository(c.DB)

	// Pastikan semua item sudah complete sebelum confirm
	isComplete, err := repo.IsPickingComplete(outboundNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}
	if !isComplete {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Belum semua item selesai dipick. Selesaikan picking terlebih dahulu.",
		})
	}

	userID := 0
	if uid, ok := ctx.Locals("user_id").(int); ok {
		userID = uid
	}

	if err := repo.ConfirmPicking(outboundNo, userID); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal konfirmasi picking: " + err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Picking %s berhasil dikonfirmasi", outboundNo),
	})
}

func (c *MobileOutboundController) GetPickingScans(ctx *fiber.Ctx) error {
	idStr := ctx.Params("outbound_picking_id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "outbound_picking_id tidak valid",
		})
	}

	repo := repositories.NewOutboundPickingRepository(c.DB)
	scans, err := repo.GetScans(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    scans,
	})
}

func (c *MobileOutboundController) ScanPickingBatch(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	// ── 1. Load outbound header (sekali, shared untuk semua item) ─────────────

	var outboundHeader models.OutboundHeader
	if err := c.DB.Where("outbound_no = ?", outboundNo).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "outbound_no not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ── 2. Load inventory policy (sekali, shared) ─────────────────────────────

	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", outboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Inventory policy not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// ── 3. Parse request body ─────────────────────────────────────────────────

	// Struct payload per scan — identik dengan field di ScanPicking
	type ScanPayload struct {
		PackingNo  string  `json:"packing_no"`
		PackCtnNo  string  `json:"pack_ctn_no"`
		Location   string  `json:"location"`
		OutboundNo string  `json:"outbound_no"`
		Barcode    string  `json:"barcode"`
		SerialNo   string  `json:"serial_no"`
		Qty        float64 `json:"qty"`
		Uom        string  `json:"uom"`
		CartonID   uint    `json:"carton_id"`
		CartonCode string  `json:"carton_code"`
		QrRaw      string  `json:"qr_raw"`
		LotNo      string  `json:"lot_no"`
		ProdDate   string  `json:"prod_date"`
	}

	type ScanItem struct {
		LocalID string      `json:"localId"` // UUID dari IndexedDB frontend
		Payload ScanPayload `json:"payload"`
	}

	type BatchRequest struct {
		Scans []ScanItem `json:"scans"`
	}

	var req BatchRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(req.Scans) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "No scans provided",
		})
	}

	// ── 4. Struct hasil per item ──────────────────────────────────────────────

	type ScanResult struct {
		LocalID string `json:"localId"` // untuk korelasi hasil ke item di frontend
		Status  string `json:"status"`  // "ok" | "duplicate" | "invalid" | "error"
		Message string `json:"message"` // pesan detail untuk frontend
	}

	// ── 5. Cache packing per PackingNo ───────────────────────────────────────
	// Hindari query ulang ke DB untuk PackingNo yang sama di batch ini
	packingCache := make(map[string]models.OutboundPacking)

	// ── 6. Cache carton per CartonID ─────────────────────────────────────────
	cartonCache := make(map[uint]models.MasterCarton)

	// ── 7. Ambil userID sekali ────────────────────────────────────────────────
	userID := int(ctx.Locals("userID").(float64))

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	uomRepo := repositories.NewUomRepository(c.DB)

	results := make([]ScanResult, 0, len(req.Scans))

	// ── 8. Proses tiap scan secara independen ─────────────────────────────────

	for _, scanItem := range req.Scans {

		fmt.Printf("Processing scan: %+v\n", scanItem) // Debug log untuk melihat payload tiap item

		scan := scanItem.Payload
		localID := scanItem.LocalID

		// Helper: return result untuk item ini dan lanjut ke item berikutnya
		// (tidak stop seluruh batch)
		addResult := func(status, message string) {
			results = append(results, ScanResult{
				LocalID: localID,
				Status:  status,
				Message: message,
			})
		}

		// ── 8a. Validasi packing (jika policy aktif) ──────────────────────────

		var packing models.OutboundPacking

		if inventoryPolicy.RequirePackingScan {
			if scan.PackingNo == "" {
				addResult("invalid", "Packing number is required")
				continue
			}
			if scan.PackCtnNo == "" {
				addResult("invalid", "CTN number is required")
				continue
			}

			// Cek cache dulu
			if cached, ok := packingCache[scan.PackingNo]; ok {
				packing = cached
			} else {
				var packings []models.OutboundPacking
				if err := c.DB.Where("packing_no = ?", scan.PackingNo).Find(&packings).Error; err != nil {
					addResult("error", "Failed to query packing: "+err.Error())
					continue
				}

				if len(packings) == 0 {
					// Buat packing baru
					newPacking := models.OutboundPacking{
						PackingNo: scan.PackingNo,
						// CreatedAt: time.Now(), // GORM otomatis set
						CreatedBy: userID,
					}
					if err := c.DB.Create(&newPacking).Error; err != nil {
						addResult("error", "Failed to create packing: "+err.Error())
						continue
					}
					packing = newPacking
				} else {
					packing = packings[0]
				}
				packingCache[scan.PackingNo] = packing
			}
		}

		// ── 8b. UOM lookup & konversi ─────────────────────────────────────────

		var uomConversion models.UomConversion
		if err := c.DB.Where("ean = ?", scan.Barcode).First(&uomConversion).Error; err != nil {
			addResult("invalid", "Item not found in UOM conversion: "+scan.Barcode)
			continue
		}

		uom, errUOM := uomRepo.ConversionQty(uomConversion.ItemCode, scan.Qty, uomConversion.FromUom)
		if errUOM != nil {
			addResult("error", "UOM conversion failed: "+errUOM.Error())
			continue
		}

		// ── 8c. Outbound detail lookup ────────────────────────────────────────

		var outboundDetail models.OutboundDetail
		if err := c.DB.Where("outbound_id = ? AND item_code = ?", outboundHeader.ID, uomConversion.ItemCode).First(&outboundDetail).Error; err != nil {
			addResult("invalid", "Item not found in outbound detail: "+uomConversion.ItemCode)
			continue
		}

		// ── 8d. Product lookup ────────────────────────────────────────────────

		var product models.Product
		if err := c.DB.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
			addResult("invalid", "Product not found: "+uomConversion.ItemCode)
			continue
		}

		// ── 8e. Serial number validation (jika produk HasSerial = "Y") ────────

		if product.HasSerial == "Y" {
			// Cek duplikat serial di outbound ini
			var existing []models.OutboundBarcode
			if err := c.DB.Where(
				"outbound_id = ? AND barcode = ? AND serial_number = ?",
				outboundHeader.ID, scan.Barcode, scan.SerialNo,
			).Find(&existing).Error; err != nil {
				addResult("error", "Failed to check duplicate serial: "+err.Error())
				continue
			}

			if len(existing) > 0 {
				// Status "duplicate" — bukan error fatal, frontend tampilkan warning
				addResult("duplicate", "Serial number sudah pernah discan: "+scan.SerialNo)
				continue
			}

			// Validasi serial ke inventory (jika policy aktif)
			if inventoryPolicy.ValidationSN {
				_, err := outboundRepo.ValidateSerialNumber(product.ItemCode, scan.SerialNo, int(outboundHeader.ID))
				if err != nil {
					addResult("invalid", "Serial number tidak valid: "+err.Error())
					continue
				}
			}
		}

		// ── 8f. Outbound picking lookup ───────────────────────────────────────

		var outboundPicking models.OutboundPicking
		if err := c.DB.Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode).First(&outboundPicking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				addResult("invalid", "Picking not found untuk barcode: "+product.Barcode)
				continue
			}
			addResult("error", "DB error: "+err.Error())
			continue
		}

		// ── 8g. Cek qty limit (tidak boleh melebihi picking plan) ─────────────

		type PickingSum struct{ QtyPickingList int }
		var pickingSum PickingSum
		if err := c.DB.Table("outbound_pickings").
			Select("COALESCE(SUM(quantity), 0) as qty_picking_list").
			Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode).
			Scan(&pickingSum).Error; err != nil {
			addResult("error", "Failed to sum picking qty: "+err.Error())
			continue
		}

		type BarcodeSum struct{ QtyBarcode int }
		var barcodeSum BarcodeSum
		if err := c.DB.Table("outbound_barcodes").
			Select("COALESCE(SUM(quantity), 0) AS qty_barcode").
			Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, product.Barcode).
			Scan(&barcodeSum).Error; err != nil {
			addResult("error", "Failed to sum barcode qty: "+err.Error())
			continue
		}

		if barcodeSum.QtyBarcode+int(uom.QtyConverted) > pickingSum.QtyPickingList {
			addResult("invalid", fmt.Sprintf(
				"Qty melebihi limit. Sudah scan: %d, Akan scan: %.0f, Limit: %d",
				barcodeSum.QtyBarcode, uom.QtyConverted, pickingSum.QtyPickingList,
			))
			continue
		}

		// ── 8h. Carton lookup (dengan cache) ──────────────────────────────────

		var carton models.MasterCarton
		if cached, ok := cartonCache[scan.CartonID]; ok {
			carton = cached
		} else {
			if err := c.DB.Where("id = ?", scan.CartonID).First(&carton).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					addResult("invalid", fmt.Sprintf("Carton ID %d tidak ditemukan", scan.CartonID))
					continue
				}
				addResult("error", "Failed to load carton: "+err.Error())
				continue
			}
			cartonCache[scan.CartonID] = carton
		}

		// ── 8i. Tentukan serial number final ──────────────────────────────────

		serialNumber := scan.SerialNo
		if product.HasSerial == "N" {
			serialNumber = product.Barcode
		}

		// ── 8j. Insert OutboundBarcode ────────────────────────────────────────

		outboundBarcode := models.OutboundBarcode{
			OutboundId:       outboundHeader.ID,
			OutboundNo:       outboundHeader.OutboundNo,
			PackingId:        packing.ID,
			PackingNo:        packing.PackingNo,
			PackCtnNo:        scan.PackCtnNo,
			OutboundDetailId: outboundPicking.OutboundDetailId,
			ItemID:           int(product.ID),
			ItemCode:         product.ItemCode,
			Barcode:          product.Barcode,
			Uom:              product.Uom,
			SerialNumber:     serialNumber,
			Quantity:         uom.QtyConverted,
			Status:           "pending",
			BarcodeDataScan:  scan.Barcode,
			DataScan: func() string {
				if scan.QrRaw != "" {
					return scan.QrRaw
				}
				return scan.Barcode
			}(),
			ProdDate:      scan.ProdDate,
			LotNumber:     scan.LotNo,
			QtyDataScan:   scan.Qty,
			LocationScan:  scan.Location,
			UomScan:       uomConversion.FromUom,
			IsSerial:      product.HasSerial == "Y",
			CartonID:      scan.CartonID,
			CartonCode:    scan.CartonCode,
			CtnLength:     carton.Length,
			CtnWidth:      carton.Width,
			CtnHeight:     carton.Height,
			CtnVolume:     carton.Volume,
			CtnMaxWeight:  carton.MaxWeight,
			CtnTareWeight: carton.TareWeight,
			CreatedBy:     userID,
		}

		if err := c.DB.Create(&outboundBarcode).Error; err != nil {
			addResult("error", "Failed to save scan: "+err.Error())
			continue
		}

		// ── 8k. Item berhasil ─────────────────────────────────────────────────
		addResult("ok", "Item scanned successfully")
	}

	// ── 9. Update outbound header status (sekali di akhir, jika ada yang ok) ──
	// Hanya update jika minimal satu scan berhasil

	anyOk := false
	for _, r := range results {
		if r.Status == "ok" {
			anyOk = true
			break
		}
	}

	if anyOk {
		outboundHeader.Status = "packing"
		outboundHeader.RawStatus = "PACKING"
		outboundHeader.ConfirmTime = time.Now()
		outboundHeader.ConfirmBy = userID
		outboundHeader.UpdatedBy = userID

		if err := c.DB.Save(&outboundHeader).Error; err != nil {
			// Jangan gagalkan seluruh response — log saja
			// Insert sudah berhasil, status header bisa di-retry
			fmt.Println("Warning: Failed to update outbound header status:", err.Error())
		}
	}

	// ── 10. Return semua result ke frontend ───────────────────────────────────

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"results": results,
	})
}
