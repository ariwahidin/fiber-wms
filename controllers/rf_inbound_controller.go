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

	return nil
}

func (c *RfInboundController) PostInboundByInboundID(ctx *fiber.Ctx) error {

	fmt.Println("Payload Data Mentah : ", string(ctx.Body()))

	var payload Input
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	// Validasi menggunakan validator
	if err := validate.Struct(payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi custom berdasarkan Scan Type
	if err := validateCustom(payload); err != nil {
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

	// id, err := ctx.ParamsInt("id")
	// if err != nil {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	// }

	// var inboundBarcode models.InboundBarcode
	// if err := c.DB.Where("id = ?", id).First(&inboundBarcode).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// if inboundBarcode.Status == "in_stock" {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already closed"})
	// }

	// sqlDelete := `DELETE FROM inbound_barcodes WHERE id = ?`
	// if err := c.DB.Exec(sqlDelete, id).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail deleted successfully"})
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

	sqlIB := `select inbound_detail_id, item_id, item_code, SUM(quantity) as quantity, whs_code 
	from inbound_barcodes 
	where id in (?)
	group by inbound_detail_id, item_id, item_code, whs_code `

	if err := c.DB.Raw(sqlIB, request.IDs).Scan(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inventories []models.Inventory

	for _, inboundBarcode := range inboundBarcodes {
		inventories = append(inventories, models.Inventory{
			InboundDetailId: inboundBarcode.InboundDetailId,
			ItemId:          inboundBarcode.ItemID,
			ItemCode:        inboundBarcode.ItemCode,
			WhsCode:         inboundBarcode.WhsCode,
			Quantity:        inboundBarcode.Quantity,
			CreatedBy:       int(ctx.Locals("userID").(float64)),
		})
	}

	sqlIBD := `select inbound_detail_id, item_id, item_code, SUM(quantity) as quantity, whs_code, scan_data, location, qa_status
	from inbound_barcodes 
	where id in (?)
	group by inbound_detail_id, item_id, item_code, whs_code, scan_data, location, qa_status `

	if err := c.DB.Raw(sqlIBD, request.IDs).Scan(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inventoryDetails []models.InventoryDetail

	for _, inboundBarcode := range inboundBarcodes {
		inventoryDetails = append(inventoryDetails, models.InventoryDetail{
			InboundDetailId: inboundBarcode.InboundDetailId,
			SerialNumber:    inboundBarcode.ScanData,
			Quantity:        inboundBarcode.Quantity,
			Location:        inboundBarcode.Location,
			QaStatus:        inboundBarcode.QaStatus,
			CreatedBy:       int(ctx.Locals("userID").(float64)),
		})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)
	res, err := inboundRepo.CreateInventories(inventories, inventoryDetails)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("res: ", res)
	updated_at := time.Now()

	if res {
		sqlUpdate := `UPDATE inbound_barcodes SET status = 'in_stock', updated_at = ?, updated_by = ? WHERE id IN (?)`
		if err := c.DB.Exec(sqlUpdate, updated_at, ctx.Locals("userID").(float64), request.IDs).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail confirmed successfully"})
}
