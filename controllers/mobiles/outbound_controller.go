package mobiles

import (
	"errors"
	"fiber-app/models"
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
		DeliveryNo   string    `json:"delivery_no"`
		QtyReq       int       `json:"qty_req"`
		QtyScan      int       `json:"qty_scan"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	sql := `WITH od AS
	(SELECT outbound_id, SUM(quantity) qty_req, SUM(scan_qty) as scan_qty 
	FROM outbound_details
	GROUP BY outbound_id)

	SELECT a.id, a.outbound_no, b.customer_name,
	a.delivery_no, od.qty_req, od.scan_qty,
	a.status, a.updated_at
	FROM outbound_headers a
	INNER JOIN customers b ON a.customer_code = b.customer_code
	LEFT JOIN od ON a.id = od.outbound_id
	WHERE a.status = 'picking'
	ORDER BY a.id DESC`
	var listOutbound []listOutboundResponse
	if err := c.DB.Raw(sql).Scan(&listOutbound).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

	var listOutboundDetails []models.OutboundDetail
	if err := c.DB.Debug().Where("outbound_id = ?", outboundHeader.ID).Find(&listOutboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listOutboundDetails})
}

func (c *MobileOutboundController) ScanPicking(ctx *fiber.Ctx) error {

	type input struct {
		ScanType   string `json:"scan_type"`
		OutboundNo string `json:"outbound_no"`
		Barcode    string `json:"barcode"`
		SerialNo   string `json:"serial_no"`
		Qty        int    `json:"qty"`
		SeqBox     int    `json:"seq_box"`
	}

	var inputScan input
	if err := ctx.BodyParser(&inputScan); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if inputScan.OutboundNo == "" || inputScan.Barcode == "" || inputScan.SerialNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "outbound_no, barcode and serial_no are required"})
	}

	// start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var outboundHeader models.OutboundHeader
	if err := tx.Debug().Where("outbound_no = ?", inputScan.OutboundNo).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound_no not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundDetail models.OutboundDetail
	if err := tx.Debug().Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, inputScan.Barcode).First(&outboundDetail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "barcode not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if outboundDetail.ScanQty+inputScan.Qty > outboundDetail.Quantity {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "qty is over"})
	}

	var product models.Product
	if err := tx.Debug().Where("item_code = ?", outboundDetail.ItemCode).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "item_code not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if product.HasSerial == "Y" && inputScan.ScanType != "SERIAL" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Scan type must be SERIAL"})
	}

	if inputScan.ScanType == "SERIAL" {

		var pickingSheet models.PickingSheet
		if err := tx.Debug().Where("outbound_detail_id = ? AND serial_number = ? AND qty_available > 0 AND qty_available >= ?", outboundDetail.ID, inputScan.SerialNo, inputScan.Qty).First(&pickingSheet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Tidak ketemu di PickingSheet, cari di Inventory
				var inventory models.Inventory
				if err := tx.Debug().
					Where("barcode = ? AND serial_number = ? AND whs_code = ? AND qty_available > 0 AND qty_available >= ?",
						inputScan.Barcode, inputScan.SerialNo, outboundDetail.WhsCode, inputScan.Qty).
					Order("inbound_barcode_id ASC").
					First(&inventory).
					Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						tx.Rollback()
						return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "serial_number not found in picking_sheet and inventory"})
					}
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

				if err := tx.Debug().
					Model(&models.Inventory{}).
					Where("id = ?", inventory.ID).
					Updates(map[string]interface{}{
						"qty_available": inventory.QtyAvailable - inputScan.Qty,
						"qty_allocated": inventory.QtyAllocated + inputScan.Qty,
						"updated_by":    int(ctx.Locals("userID").(float64)),
						"updated_at":    time.Now(),
					}).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

				// Kalau ketemu di inventory, buat outboundBarcode
				var outboundBarcode models.OutboundBarcode
				outboundBarcode.InventoryID = int(inventory.ID)
				outboundBarcode.OutboundId = int(outboundHeader.ID)
				outboundBarcode.OutboundDetailId = int(outboundDetail.ID)
				outboundBarcode.ItemID = int(product.ID)
				outboundBarcode.ItemCode = outboundDetail.ItemCode
				outboundBarcode.ScanData = inputScan.SerialNo
				outboundBarcode.Barcode = product.Barcode
				outboundBarcode.Quantity = inputScan.Qty
				outboundBarcode.WhsCode = inventory.WhsCode
				outboundBarcode.QaStatus = inventory.QaStatus
				outboundBarcode.SeqBox = inputScan.SeqBox
				outboundBarcode.Status = "picking"
				outboundBarcode.ScanType = inputScan.ScanType
				outboundBarcode.Location = inventory.Location
				outboundBarcode.Pallet = inventory.Pallet
				outboundBarcode.SerialNumber = inventory.SerialNumber
				outboundBarcode.CreatedBy = int(ctx.Locals("userID").(float64))

				if err := tx.Debug().Create(&outboundBarcode).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

			} else {
				// Error lain saat cek pickingSheet
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		} else {
			// Ketemu di PickingSheet
			var outboundBarcode models.OutboundBarcode
			outboundBarcode.InventoryID = pickingSheet.InventoryID
			outboundBarcode.OutboundId = int(outboundHeader.ID)
			outboundBarcode.OutboundDetailId = int(outboundDetail.ID)
			outboundBarcode.PickingSheetId = int(pickingSheet.ID)
			outboundBarcode.ItemID = int(product.ID)
			outboundBarcode.ItemCode = outboundDetail.ItemCode
			outboundBarcode.ScanData = inputScan.SerialNo
			outboundBarcode.Barcode = product.Barcode
			outboundBarcode.Quantity = inputScan.Qty
			outboundBarcode.WhsCode = pickingSheet.WhsCode
			outboundBarcode.QaStatus = pickingSheet.QaStatus
			outboundBarcode.SeqBox = inputScan.SeqBox
			outboundBarcode.Status = "picking"
			outboundBarcode.ScanType = inputScan.ScanType
			outboundBarcode.Location = pickingSheet.Location
			outboundBarcode.Pallet = pickingSheet.Pallet
			outboundBarcode.SerialNumber = pickingSheet.SerialNumber
			outboundBarcode.CreatedBy = int(ctx.Locals("userID").(float64))

			if err := tx.Debug().Create(&outboundBarcode).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if err := tx.Debug().
				Model(&models.PickingSheet{}).
				Where("id = ?", pickingSheet.ID).
				Updates(map[string]interface{}{
					"qty_available": pickingSheet.QtyAvailable - inputScan.Qty,
					"qty_allocated": pickingSheet.QtyAllocated + inputScan.Qty,
					"updated_by":    int(ctx.Locals("userID").(float64)),
					"updated_at":    time.Now(),
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}

		if err := tx.Debug().
			Model(&models.OutboundDetail{}).
			Where("id = ?", outboundDetail.ID).
			Updates(map[string]interface{}{
				"scan_qty":   outboundDetail.ScanQty + inputScan.Qty,
				"updated_by": int(ctx.Locals("userID").(float64)),
				"updated_at": time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	// commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Picking scan successful"})
}

func (c *MobileOutboundController) GetListOutboundBarcode(ctx *fiber.Ctx) error {

	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	var outboundBarcodes []models.OutboundBarcode
	if err := c.DB.Debug().Where("outbound_detail_id = ?", id).Find(&outboundBarcodes).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "id not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": outboundBarcodes})
}
func (c *MobileOutboundController) DeleteOutboundBarcode(ctx *fiber.Ctx) error {

	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}

	// start transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var outboundBarcode models.OutboundBarcode
	if err := tx.Debug().Where("id = ?", id).First(&outboundBarcode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "id not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundDetail models.OutboundDetail
	if err := tx.Debug().Where("id = ?", outboundBarcode.OutboundDetailId).First(&outboundDetail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "id not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inventory models.Inventory
	if err := tx.Debug().Where("id = ?", outboundBarcode.InventoryID).First(&inventory).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "id not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if outboundBarcode.PickingSheetId != 0 {

		// update picking sheet
		if err := tx.Debug().
			Model(&models.PickingSheet{}).
			Where("id = ?", outboundBarcode.PickingSheetId).
			Updates(map[string]interface{}{
				"qty_available": inventory.QtyAvailable + outboundBarcode.Quantity,
				"qty_allocated": inventory.QtyAllocated - outboundBarcode.Quantity,
				"updated_by":    int(ctx.Locals("userID").(float64)),
				"updated_at":    time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	} else {
		// Update inventory
		if err := tx.Debug().
			Model(&models.Inventory{}).
			Where("id = ?", outboundBarcode.InventoryID).
			Updates(map[string]interface{}{
				"qty_available": inventory.QtyAvailable + outboundBarcode.Quantity,
				"qty_allocated": inventory.QtyAllocated - outboundBarcode.Quantity,
				"updated_by":    int(ctx.Locals("userID").(float64)),
				"updated_at":    time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Update outbound detail
	if err := tx.Debug().
		Model(&models.OutboundDetail{}).
		Where("id = ?", outboundBarcode.OutboundDetailId).
		Updates(map[string]interface{}{
			"scan_qty":   outboundDetail.ScanQty - outboundBarcode.Quantity,
			"updated_by": int(ctx.Locals("userID").(float64)),
			"updated_at": time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Delete permanently
	sql := `DELETE FROM outbound_barcodes WHERE id = ?`
	if err := tx.Exec(sql, id).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Outbound barcode deleted successfully for ID:", id)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound barcode deleted successfully"})
}
