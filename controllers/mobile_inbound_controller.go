package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
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
		ID           uint      `json:"id"`
		InboundNo    string    `json:"inbound_no"`
		SupplierName string    `json:"supplier_name"`
		Status       string    `json:"status"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	sql := `SELECT a.id, a.inbound_no, b.supplier_name, a.status, a.updated_at FROM inbound_headers a
			INNER JOIN suppliers b ON a.supplier_id = b.id`
	var listInbound []listInboundResponse
	if err := c.DB.Raw(sql).Scan(&listInbound).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": listInbound})
}

func (c *MobileInboundController) ScanInbound(ctx *fiber.Ctx) error {

	var scanInbound struct {
		ID        int    `json:"id"`
		InboundNo string `json:"inboundNo"`
		Location  string `json:"location"`
		Barcode   string `json:"barcode"`
		ScanType  string `json:"scanType"`
		WhsCode   string `json:"whsCode"`
		QaStatus  string `json:"qaStatus"`
		Serial    string `json:"serial"`
		QtyScan   int    `json:"qtyScan"`
		Uploaded  bool   `json:"uploaded"`
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
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	var product models.Product
	if err := tx.Where("barcode = ?", scanInbound.Barcode).First(&product).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	var inboundDetail models.InboundDetail
	if err := tx.Debug().Where("inbound_no = ? AND item_code = ? AND scan_qty < quantity", scanInbound.InboundNo, product.ItemCode).First(&inboundDetail).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound detail not found"})
	}

	if inboundDetail.ScanQty+scanInbound.QtyScan > inboundDetail.Quantity {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Qty scan more than total qty"})
	}

	inboundDetail.ScanQty += scanInbound.QtyScan
	inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.UpdatedAt = time.Now()

	if err := tx.Where("id = ?", inboundDetail.ID).
		Select("scan_qty", "updated_by").
		Updates(&models.InboundDetail{
			ScanQty:   inboundDetail.ScanQty,
			UpdatedBy: int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var checkInboundBarcode models.InboundBarcode
	if err := tx.Where("inbound_id = ? AND item_code = ? AND serial_number = ?", inboundHeader.ID, product.ItemCode, scanInbound.Serial).First(&checkInboundBarcode).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if checkInboundBarcode.ID > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number already scanned"})
	}

	var inboundBarcode = models.InboundBarcode{
		InboundId:       int(inboundHeader.ID),
		InboundDetailId: int(inboundDetail.ID),
		Location:        scanInbound.Location,
		Pallet:          scanInbound.Location,
		ItemID:          int(product.ID),
		ItemCode:        product.ItemCode,
		Barcode:         scanInbound.Barcode,
		ScanType:        scanInbound.ScanType,
		WhsCode:         scanInbound.WhsCode,
		QaStatus:        scanInbound.QaStatus,
		ScanData:        scanInbound.Serial,
		SerialNumber:    scanInbound.Serial,
		Quantity:        scanInbound.QtyScan,
		Status:          "pending",
		CreatedBy:       int(ctx.Locals("userID").(float64)),
	}

	if err := tx.Create(&inboundBarcode).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

func (c *MobileInboundController) GetScanInbound(ctx *fiber.Ctx) error {

	inbound_no := ctx.Params("inbound_no")

	var inboundHeader models.InboundHeader
	if err := c.DB.Where("inbound_no = ?", inbound_no).First(&inboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	var inboundBarcode []models.InboundBarcode

	if err := c.DB.Order("created_at DESC").Where("inbound_id = ?", inboundHeader.ID).Find(&inboundBarcode).Error; err != nil {
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

	inboundDetail.ScanQty -= inboundBarcode.Quantity
	inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.UpdatedAt = time.Now()

	if err := tx.Where("id = ?", inboundDetail.ID).
		Select("scan_qty", "updated_by").
		Updates(&models.InboundDetail{
			ScanQty:   inboundDetail.ScanQty,
			UpdatedBy: int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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

func (c *MobileInboundController) ConfirmPutaway(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")

	var scanInbound struct {
		ID        int    `json:"id"`
		InboundNo string `json:"inboundNo"`
		Location  string `json:"location"`
		Barcode   string `json:"barcode"`
		ScanType  string `json:"scanType"`
		WhsCode   string `json:"whsCode"`
		QaStatus  string `json:"qaStatus"`
		Serial    string `json:"serial"`
		QtyScan   int    `json:"qtyScan"`
		Uploaded  bool   `json:"uploaded"`
	}

	if err := ctx.BodyParser(&scanInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Scan Inbound : ", scanInbound)

	// return nil

	fmt.Println("Inbound No : ", inbound_no)

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
	if err := tx.Where("inbound_no = ?", inbound_no).First(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	}

	var inboundBarcodes []models.InboundBarcode

	if err := tx.Order("created_at DESC").Where("inbound_id = ? AND status = ? AND location = ?", inboundHeader.ID, "pending", scanInbound.Location).Find(&inboundBarcodes).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) < 1 {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "No Item Scanned"})
	}

	for _, inboundBarcode := range inboundBarcodes {

		if inboundBarcode.Status != "pending" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already confirmed"})
		}

		var inboundDetail models.InboundDetail
		if err := tx.Where("id = ?", inboundBarcode.InboundDetailId).First(&inboundDetail).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var inventory models.Inventory
		inventory.InboundDetailId = int(inboundDetail.ID)
		inventory.InboundBarcodeId = int(inboundBarcode.ID)
		inventory.RecDate = inboundDetail.RecDate
		inventory.ItemId = int(inboundBarcode.ItemID)
		inventory.ItemCode = inboundBarcode.ItemCode
		inventory.WhsCode = inboundBarcode.WhsCode
		inventory.Pallet = inboundBarcode.Pallet
		inventory.Location = inboundBarcode.Location
		inventory.QaStatus = inboundBarcode.QaStatus
		inventory.SerialNumber = inboundBarcode.ScanData
		inventory.Quantity = inboundBarcode.Quantity
		inventory.QtyOnhand = inboundBarcode.Quantity
		inventory.QtyAvailable = inboundBarcode.Quantity
		inventory.QtyAllocated = 0
		inventory.Trans = "inbound"
		inventory.CreatedBy = int(ctx.Locals("userID").(float64))

		// save to inventory table
		if err := tx.Create(&inventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// update inbound barcode status to in stock
		inboundBarcode.Status = "in stock"
		inboundBarcode.UpdatedAt = time.Now()
		inboundBarcode.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := tx.Where("id = ?", inboundBarcode.ID).Updates(&inboundBarcode).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Confirm putaway successfully"})
}
