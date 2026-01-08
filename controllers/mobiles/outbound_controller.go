package mobiles

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
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
		ID           uint      `json:"id"`
		OutboundNo   string    `json:"outbound_no"`
		CustomerName string    `json:"customer_name"`
		Status       string    `json:"status"`
		ShipmentID   string    `json:"shipment_id"`
		QtyReq       int       `json:"qty_req"`
		QtyScan      int       `json:"qty_scan"`
		QtyPack      int       `json:"qty_pack"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	sql := `WITH od AS
	(SELECT outbound_id, SUM(quantity) qty_req, SUM(scan_qty) as scan_qty 
	FROM outbound_details
	GROUP BY outbound_id),
	kd AS (
	SELECT outbound_id, SUM(quantity) AS qty_pack
	FROM outbound_barcodes
	GROUP BY outbound_id
	)

	SELECT a.id, a.outbound_no, b.customer_name,
	a.shipment_id, od.qty_req, od.scan_qty, kd.qty_pack,
	a.status, a.updated_at, ipo.require_picking_scan
	FROM outbound_headers a
	INNER JOIN customers b ON a.customer_code = b.customer_code
	LEFT JOIN od ON a.id = od.outbound_id	
	LEFT JOIN kd ON a.id = kd.outbound_id
	LEFT JOIN inventory_policies ipo ON a.owner_code = ipo.owner_code
	WHERE a.status = 'picking' and ipo.require_picking_scan <> 0
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

	// var listOutboundDetails []models.OutboundDetail
	// if err := c.DB.Debug().Where("outbound_id = ?", outboundHeader.ID).Find(&listOutboundDetails).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

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

	// query := `
	// 	WITH ob AS (
	// 		SELECT a.outbound_id, a.item_code, a.barcode, a.item_id, a.outbound_detail_id,
	// 		COALESCE(SUM(a.quantity),0) as qty_scan
	// 		FROM outbound_barcodes a
	// 		LEFT JOIN outbound_details b ON a.outbound_detail_id = b.id
	// 		WHERE a.outbound_id = ?
	// 		GROUP BY a.item_code, a.barcode, a.item_id, a.outbound_id, a.outbound_detail_id
	// 	),
	// 	op AS (
	// 		SELECT a.outbound_detail_id, a.item_code, sum(a.quantity) as qty, a.uom
	// 		FROM outbound_pickings a
	// 		WHERE a.outbound_id = ?
	// 		GROUP BY a.outbound_detail_id, a.item_code, a.uom
	// 	)
	// 	SELECT a.id as outbound_detail_id, a.whs_code, a.item_code, a.barcode,
	// 		op.qty as quantity,
	// 		b.item_name,
	// 		b.has_serial,
	// 		COALESCE(ob.qty_scan, 0) as qty_scan,
	// 		op.uom,
	// 		a.owner_code
	// 	FROM outbound_details a
	// 	INNER JOIN products b ON a.item_id = b.id
	// 	LEFT JOIN op ON a.id = op.outbound_detail_id
	// 	LEFT JOIN ob ON ob.outbound_id = a.outbound_id AND a.item_code = ob.item_code AND a.id = ob.outbound_detail_id
	// 	WHERE a.outbound_id = ?
	// 	`

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

	var uomConversion models.UomConversion
	if err := c.DB.Where("ean = ?", scanOutbound.Barcode).First(&uomConversion).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found in UOM conversion", "message": "Item not found in UOM conversion"})
	}

	var product models.Product
	if err := c.DB.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found", "message": "Product not found"})
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
	}

	if err := ctx.BodyParser(&scanOutbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Debug().Where("owner_code = ?", outboundHeader.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inventory policy not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundRepo = repositories.NewOutboundRepository(c.DB)

	if inventoryPolicy.RequireScanPickLocation {
		if scanOutbound.Location == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location is required"})
		}
	}

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
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Ctn no is required"})
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

	if inventoryPolicy.RequireScanPickLocation {
		queryOutboundPicking = queryOutboundPicking.Where("location = ?", scanOutbound.Location)
	}

	var outboundPicking models.OutboundPicking

	if err := queryOutboundPicking.First(&outboundPicking).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Picking not found", "message": "Picking not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var serialNumber string

	if product.HasSerial == "N" {
		serialNumber = product.Barcode
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
		QtyDataScan:      scanOutbound.Qty,
		LocationScan:     scanOutbound.Location,
		UomScan:          uomConversion.FromUom,
		IsSerial:         product.HasSerial == "Y",
		CreatedBy:        int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&outboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
