package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type RfInboundController struct {
	DB *gorm.DB
}

func NewRfInboundController(DB *gorm.DB) *RfInboundController {
	return &RfInboundController{DB: DB}
}

func (c *RfInboundController) GetAllListInbound(ctx *fiber.Ctx) error {
	inboundRepo := repositories.NewInboundRepository(c.DB)
	result, err := inboundRepo.GetAllInbound()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var whCode []models.WarehouseCode
	if err := c.DB.Find(&whCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Warehouse not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var qaStatus []models.QaStatus

	if err := c.DB.Find(&qaStatus).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Qa Status not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"inbound": result, "wh": whCode, "qa": qaStatus}})
}

func (c *RfInboundController) GetInboundByInboundID(ctx *fiber.Ctx) error {
	inbound_id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	inboundHeader, err := inboundRepo.GetInboundHeaderByInboundID(inbound_id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeader.InboundDate = func() string {
		t, _ := time.Parse(time.RFC3339, inboundHeader.InboundDate)
		return t.Format("2006-01-02")
	}()
	inboundHeader.PoDate = func() string { t, _ := time.Parse(time.RFC3339, inboundHeader.PoDate); return t.Format("2006-01-02") }()

	result, err := inboundRepo.GetDetailItemByInboundID(inbound_id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inbound_barcode, err := inboundRepo.GetAllInboundScannedByInboundID(inbound_id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"header": inboundHeader, "details": result, "barcode": inbound_barcode}})
}

type Input struct {
	ScanType        string `json:"scan_type" validate:"required"`
	InboundID       int    `json:"inbound_id" validate:"required"`
	InboundDetailID int    `json:"inbound_detail_id" validate:"required"`
	Quantity        int    `json:"quantity"`
	Pallet          string `json:"pallet"`
	Location        string `json:"location" validate:"required"`
	WhsCode         string `json:"whs_code" validate:"required"`
	QaStatus        string `json:"qa_status" validate:"required"`
	SerialNumber    string `json:"serial_number"`
	SerialNumber2   string `json:"serial_number2"`
}

var validate = validator.New()

func validateCustom(payload Input) error {
	switch payload.ScanType {
	case "SERIAL":
		// Serial harus ada, Quantity harus 1
		if payload.SerialNumber == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Serial Number is required")
		}
		if payload.Quantity != 1 {
			return fiber.NewError(fiber.StatusBadRequest, "Quantity must be 1 for SERIAL type")
		}

	case "BARCODE":
		// Barcode tidak butuh Serial Number, tapi Quantity wajib
		if payload.Quantity <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Quantity is required for BARCODE type")
		}

	case "SET":
		// Harus ada 2 Serial Number
		if payload.SerialNumber == "" || payload.SerialNumber2 == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Both Serial Numbers are required for SET type")
		}
	}

	if payload.Quantity <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "Quantity must be greater than 0")
	}

	if payload.Pallet == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Pallet is required")
	}

	if payload.QaStatus == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Qa Status is required")
	}

	if payload.WhsCode == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Warehouse code is required")
	}

	return nil
}

func (c *RfInboundController) PostInboundByInboundID(ctx *fiber.Ctx) error {

	fmt.Println("Payload Data Mentah : ", string(ctx.Body()))

	var payload Input
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	// Validasi custom berdasarkan Scan Type
	if err := validateCustom(payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi menggunakan validator
	if err := validate.Struct(payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)
	inboundDetail, err := inboundRepo.GetDetailInbound(payload.InboundID, payload.InboundDetailID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Payload Data : ", payload)
	fmt.Println("Inbound Detail : ", inboundDetail)

	qtyPredict := payload.Quantity + inboundDetail.QtyScan

	if qtyPredict > inboundDetail.Quantity {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity scanned is greater than the planned quantity"})
	}

	// Check Item Id IN DB

	var product models.Product
	if err := c.DB.Debug().Where("item_code = ?", inboundDetail.ItemCode).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found or has no serial number"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if payload.ScanType == "SERIAL" && product.HasSerial != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This item does not have a serial number"})
	}

	if payload.ScanType == "SET" && product.HasSerial != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This item does not have a serial number"})
	}

	if payload.ScanType == "SET" && payload.SerialNumber2 == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Both Serial Numbers are required for SET type"})
	}

	// return nil

	var inboundBarcode models.InboundBarcode

	if payload.ScanType == "SERIAL" {
		inboundBarcode.Quantity = 1
		inboundBarcode.ScanData = payload.SerialNumber
		inboundBarcode.SerialNumber = payload.SerialNumber

	} else if payload.ScanType == "BARCODE" {
		inboundBarcode.Quantity = payload.Quantity
		inboundBarcode.ScanData = product.Barcode
		inboundBarcode.SerialNumber = product.Barcode
	} else if payload.ScanType == "SET" {
		inboundBarcode.Quantity = 1
		inboundBarcode.ScanData = payload.SerialNumber + "-" + payload.SerialNumber2
		inboundBarcode.SerialNumber = payload.SerialNumber + "-" + payload.SerialNumber2
	} else {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scan type"})
	}

	inboundBarcode.ScanType = payload.ScanType

	inboundBarcode.ItemID = int(product.ID)
	inboundBarcode.InboundId = int(inboundDetail.InboundId)
	inboundBarcode.InboundDetailId = int(inboundDetail.ID)
	inboundBarcode.ItemCode = inboundDetail.ItemCode
	inboundBarcode.Barcode = inboundDetail.Barcode
	inboundBarcode.Pallet = payload.Pallet
	inboundBarcode.Location = payload.Location
	inboundBarcode.WhsCode = payload.WhsCode
	inboundBarcode.QaStatus = payload.QaStatus
	inboundBarcode.CreatedBy = int(ctx.Locals("userID").(float64))

	// Check Serial No is Not Same in This Inbound

	if payload.ScanType == "SERIAL" || payload.ScanType == "SET" {
		if err := c.DB.Debug().Where("scan_data = ? AND inbound_id = ?", inboundBarcode.ScanData, inboundDetail.InboundId).First(&models.InboundBarcode{}).Error; err == nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number already scanned"})
		}

		if product.Barcode == inboundBarcode.ScanData {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number cannot be the same as the barcode or GMC"})
		}
	}

	if err := c.DB.Create(&inboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundDetail.QtyScan = qtyPredict

	// Return Response
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"scanned": inboundDetail}})

}

func (c *RfInboundController) GetInboundDetailScanned(ctx *fiber.Ctx) error {

	inbound_detail_id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	inboundDetail := models.InboundDetail{}

	if err := c.DB.Where("id = ?", inbound_detail_id).First(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	inboundScanned, err := inboundRepo.GetDetailInbound(inboundDetail.InboundId, int(inboundDetail.ID))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"scanned": inboundScanned}})
}

func (c *RfInboundController) DeleteBarcode(ctx *fiber.Ctx) error {

	fmt.Println("Payload Data Mentah : ", string(ctx.Body()))

	var request struct {
		IDs []uint `json:"selected_ids"`
	}

	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(400).SendString("Invalid request")
	}

	// check status inbound barcode is pending
	var inboundBarcodes []models.InboundBarcode

	if err := c.DB.Where("id IN (?)", request.IDs).Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, inboundBarcode := range inboundBarcodes {
		if inboundBarcode.Status != "pending" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already confirmed"})
		}
	}

	sqlDelete := `DELETE FROM inbound_barcodes WHERE id IN (?)`
	if err := c.DB.Exec(sqlDelete, request.IDs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail deleted successfully"})
}

func (c *RfInboundController) GetInboundBarcodeDetail(ctx *fiber.Ctx) error {

	inbound_id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	inbound_detail_id, err := ctx.ParamsInt("detail_id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Detail ID"})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	inboundDetail, err := inboundRepo.GetInboundBarcodeDetail(inbound_id, inbound_detail_id)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"detail": inboundDetail}})
}

func (c *RfInboundController) ConfirmPutaway(ctx *fiber.Ctx) error {

	var request struct {
		IDs []uint `json:"selected_ids"`
	}

	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(400).SendString("Invalid request")
	}

	// check status inbound barcode is pending
	var inboundBarcodes []models.InboundBarcode

	if err := c.DB.Where("id IN (?) AND status = 'pending'", request.IDs).Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, inboundBarcode := range inboundBarcodes {

		if inboundBarcode.Status != "pending" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already confirmed"})
		}

		var inboundDetail models.InboundDetail
		if err := c.DB.Where("id = ?", inboundBarcode.InboundDetailId).First(&inboundDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var inventory models.Inventory
		inventory.InboundDetailId = int(inboundDetail.ID)
		inventory.InboundBarcodeId = int(inboundBarcode.ID)
		inventory.RecDate = inboundDetail.RecDate
		inventory.ItemId = int(inboundBarcode.ItemID)
		inventory.ItemCode = inboundBarcode.ItemCode
		inventory.WhsCode = inboundBarcode.WhsCode
		// inventory.Owner = inboundDetail.Owner
		inventory.Pallet = inboundBarcode.Pallet
		inventory.Location = inboundBarcode.Location
		inventory.QaStatus = inboundBarcode.QaStatus
		inventory.SerialNumber = inboundBarcode.ScanData
		inventory.QtyOrigin = inboundBarcode.Quantity
		inventory.QtyOnhand = inboundBarcode.Quantity
		inventory.QtyAvailable = inboundBarcode.Quantity
		inventory.QtyAllocated = 0
		inventory.Trans = "inbound"
		inventory.CreatedBy = int(ctx.Locals("userID").(float64))

		// save to inventory table
		if err := c.DB.Create(&inventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// update inbound barcode status to in stock
		inboundBarcode.Status = "in stock"
		inboundBarcode.UpdatedAt = time.Now()
		inboundBarcode.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Where("id = ?", inboundBarcode.ID).Updates(&inboundBarcode).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail confirmed successfully"})
}

type ScanPallet struct {
	InboundID int    `json:"inbound_id"`
	Pallet    string `json:"pallet"`
	TotalItem int    `json:"total_item"`
	TotalQty  int    `json:"total_qty"`
}

func (c *RfInboundController) ScanPallet(ctx *fiber.Ctx) error {

	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var scanPallets []ScanPallet

	sql := `select a.inbound_id, a.pallet, 
	COUNT(DISTINCT a.item_id) as total_item,
	SUM(a.quantity) AS total_qty
	from inbound_barcodes a
	where a.inbound_id = ? and a.status = 'pending'
	group by a.inbound_id, a.pallet`
	if err := c.DB.Raw(sql, id).Scan(&scanPallets).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"scan_pallet": scanPallets}})

}

// Stuct untuk menerima request
type PutawayRequestPallet struct {
	InboundID    uint     `json:"inbound_id"`
	Pallets      []string `json:"pallets"`
	RackLocation string   `json:"rack_location"`
}

func (c *RfInboundController) PutawayPallet(ctx *fiber.Ctx) error {
	var request PutawayRequestPallet
	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Validasi input
	if request.InboundID == 0 || len(request.Pallets) == 0 || request.RackLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	fmt.Println("Request Payload : ", request)

	// Get Inbound Barcode by Inbound ID and Pallet

	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Where("inbound_id = ? AND pallet IN (?) AND status = 'pending'", request.InboundID, request.Pallets).Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No data found"})
	}

	for _, inboundBarcode := range inboundBarcodes {

		var inboundDetail models.InboundDetail
		var inventory models.Inventory

		if inboundBarcode.Status != "pending" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already confirmed"})
		}

		if err := c.DB.Where("id = ?", inboundBarcode.InboundDetailId).First(&inboundDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		inventory.InboundDetailId = int(inboundDetail.ID)
		inventory.InboundBarcodeId = int(inboundBarcode.ID)
		inventory.RecDate = inboundDetail.RecDate
		inventory.ItemId = int(inboundBarcode.ItemID)
		inventory.Barcode = inboundBarcode.Barcode
		inventory.ItemCode = inboundBarcode.ItemCode
		inventory.WhsCode = inboundBarcode.WhsCode
		// inventory.Owner = inboundDetail.Owner
		inventory.Pallet = inboundBarcode.Pallet
		inventory.Location = request.RackLocation
		inventory.QaStatus = inboundBarcode.QaStatus
		inventory.SerialNumber = inboundBarcode.ScanData
		inventory.QtyOrigin = inboundBarcode.Quantity
		inventory.QtyOnhand = inboundBarcode.Quantity
		inventory.QtyAvailable = inboundBarcode.Quantity
		inventory.QtyAllocated = 0
		inventory.Trans = "putaway by pallet"
		inventory.CreatedBy = int(ctx.Locals("userID").(float64))

		// save to inventory table
		if err := c.DB.Create(&inventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// update inbound barcode status to in stock
		inboundBarcode.Status = "in stock"
		inboundBarcode.UpdatedAt = time.Now()
		inboundBarcode.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Where("id = ?", inboundBarcode.ID).Updates(&inboundBarcode).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Pallet putaway successfully", "data": fiber.Map{"inbound_barcodes": inboundBarcodes}})
}
