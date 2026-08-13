package inbound_controller

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// InboundController represents the controller for inbound operations.
type InboundController struct {
	DB *gorm.DB
}

func NewInboundController(DB *gorm.DB) *InboundController {
	return &InboundController{DB: DB}
}

type Inbound struct {
	ID             int                       `json:"ID"`
	InboundNo      string                    `json:"inbound_no"`
	InboundDate    string                    `json:"inbound_date"`
	Supplier       string                    `json:"supplier"`
	PONumber       string                    `json:"po_number"`
	Mode           string                    `json:"mode"`
	Type           string                    `json:"type"`
	Invoice        string                    `json:"invoice"`
	Remarks        string                    `json:"remarks"`
	Status         string                    `json:"status"`
	Transporter    string                    `json:"transporter"`
	NoTruck        string                    `json:"no_truck"`
	Driver         string                    `json:"driver"`
	Container      string                    `json:"container"`
	OwnerCode      string                    `json:"owner_code"`
	WhsCode        string                    `json:"whs_code"`
	ReceiptID      string                    `json:"receipt_id"`
	Origin         string                    `json:"origin"`
	PoDate         string                    `json:"po_date"`
	ArrivalTime    string                    `json:"arrival_time"`
	StartUnloading string                    `json:"start_unloading"`
	EndUnloading   string                    `json:"end_unloading"`
	TruckSize      string                    `json:"truck_size"`
	BLNo           string                    `json:"bl_no"`
	Koli           int                       `json:"koli"`
	References     []models.InboundReference `json:"references"`
	Items          []InboundItem             `json:"items"`
	ReceivedItems  []ItemInboundBarcode      `json:"received_items"`
}

type InboundItem struct {
	ID           int     `json:"ID"`
	InboundID    int     `json:"inbound_id"`
	ItemCode     string  `json:"item_code"`
	Quantity     float64 `json:"quantity"`
	QaStatus     string  `json:"qa_status"`
	Location     string  `json:"location"`
	WhsCode      string  `json:"whs_code"`
	UOM          string  `json:"uom"`
	RecDate      string  `json:"rec_date"`
	ProdDate     string  `json:"prod_date"`
	ExpDate      string  `json:"exp_date"`
	LotNumber    string  `json:"lot_number"`
	SerialNumber string  `json:"serial_number"`
	Remarks      string  `json:"remarks"`
	IsSerial     string  `json:"is_serial"`
	Mode         string  `json:"mode"`
	RefId        int     `json:"ref_id"`
	RefNo        string  `json:"ref_no"`
	DivisionCode string  `json:"division_code"`
}

type ItemInboundBarcode struct {
	ID           int    `json:"ID"`
	ItemCode     string `json:"item_code"`
	Barcode      string `json:"barcode"`
	SerialNumber string `json:"serial_number"`
	Location     string `json:"location"`
	WhsCode      string `json:"whs_code"`
	Status       string `json:"status"`
	QaStatus     string `json:"qa_status"`
	Qty          int    `json:"qty"`
	CreatedAt    string `json:"created_at"`
}

func (c *InboundController) CreateInbound(ctx *fiber.Ctx) error {
	var payload Inbound

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	if payload.ReceiptID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Receipt ID cannot be empty",
			"error":   "Receipt ID cannot be empty",
		})
	}

	// fmt.Println("PAYLOAD", payload)

	var InventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", payload.OwnerCode).First(&InventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	// Check duplicate item code / line
	itemCodes := make(map[string]bool) // gunakan map untuk cek duplikat
	for _, item := range payload.Items {

		if item.Quantity == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Quantity cannot be zero",
				"error":   "Quantity cannot be zero",
			})
		}

		if item.UOM == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "UOM cannot be empty",
				"error":   "UOM cannot be empty",
			})
		}

		if InventoryPolicy.UseReceiveLocation {
			if item.Location == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Receive location cannot be empty",
					"error":   "Receive location cannot be empty",
				})
			}
		}

		// if InventoryPolicy.RequireExpiryDate {
		// 	if item.ExpDate == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Expiration date cannot be empty",
		// 			"error":   "Expiration date cannot be empty",
		// 		})
		// 	}
		// }

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s, division %s, serial_number %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber),
			})
		}

		itemCodes[key] = true

	}

	// Mulai transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check duplicate receipt_id
	if err := tx.Debug().Where("receipt_id = ?", payload.ReceiptID).First(&models.InboundHeader{}).Error; err == nil {

		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Receipt ID already exists",
			"error":   "Receipt ID already exists: " + payload.ReceiptID,
		})
	}

	repositories := repositories.NewInboundRepository(tx)

	inbound_no, err := repositories.GenerateInboundNo()
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate inbound no",
			"error":   err.Error(),
		})
	}
	payload.InboundNo = inbound_no
	payload.Status = "open"
	userID := int(ctx.Locals("userID").(float64))

	var InboundHeader models.InboundHeader
	var supplier models.Supplier

	if err := tx.Debug().First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Supplier not found",
				"error":   "Supplier not found : " + payload.Supplier,
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get supplier",
			"error":   err.Error(),
		})
	}
	// Insert ke inbound_headers
	InboundHeader.InboundNo = payload.InboundNo
	InboundHeader.InboundDate = payload.InboundDate
	InboundHeader.ReceiptID = payload.ReceiptID
	InboundHeader.Supplier = payload.Supplier
	InboundHeader.SupplierId = int(supplier.ID)
	InboundHeader.Status = payload.Status
	InboundHeader.CreatedBy = userID
	InboundHeader.UpdatedBy = userID
	InboundHeader.Status = "open"
	InboundHeader.RawStatus = "DRAFT"
	InboundHeader.DraftTime = time.Now()
	InboundHeader.Transporter = payload.Transporter
	InboundHeader.NoTruck = payload.NoTruck
	InboundHeader.Driver = payload.Driver
	InboundHeader.Container = payload.Container
	InboundHeader.Remarks = payload.Remarks
	InboundHeader.Type = payload.Type
	InboundHeader.WhsCode = payload.WhsCode
	InboundHeader.OwnerCode = payload.OwnerCode
	InboundHeader.Origin = payload.Origin
	InboundHeader.PoDate = payload.PoDate
	InboundHeader.ArrivalTime = payload.ArrivalTime
	InboundHeader.StartUnloading = payload.StartUnloading
	InboundHeader.EndUnloading = payload.EndUnloading
	InboundHeader.TruckSize = payload.TruckSize
	InboundHeader.BLNo = payload.BLNo
	InboundHeader.Koli = payload.Koli

	res := tx.Create(&InboundHeader)

	if res.Error != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert inbound header",
			"error":   res.Error.Error(),
		})
	}

	var inboundID uint
	if res.RowsAffected == 1 {
		inboundID = uint(InboundHeader.ID)
	}

	// Insert ke inbound references
	for _, ref := range payload.References {

		if ref.RefNo == "" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invoice no cannot be empty",
				"error":   "Invoice no cannot be empty",
			})
		}

		var InboundReference models.InboundReference
		InboundReference.InboundId = inboundID
		InboundReference.RefNo = ref.RefNo
		res := tx.Create(&InboundReference)
		if res.Error != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert inbound references",
				"error":   res.Error.Error(),
			})
		}

	}

	// Insert ke inbound details
	for _, item := range payload.Items {

		var product models.Product

		if err := tx.Debug().First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var uomConversion models.UomConversion
		if err := tx.Debug().First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, item.UOM).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found"})
			}
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var InboundDetail models.InboundDetail
		var InboundReference models.InboundReference

		if len(payload.References) == 1 {
			if err := tx.Debug().First(&InboundReference, "ref_no = ?", payload.References[0].RefNo).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound Reference not found!"})
				}
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		} else {
			if err := tx.Debug().First(&InboundReference, "ref_no = ?", item.RefNo).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound Reference not found"})
				}
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}

		if !InventoryPolicy.UseLotNo {
			item.LotNumber = InboundHeader.InboundNo
		}

		if !InventoryPolicy.RequireExpiryDate {
			// item.ExpDate = item.RecDate
		}

		if !InventoryPolicy.UseProductionDate {
			// item.ProdDate = item.RecDate
		}

		inputQty := item.Quantity
		if item.SerialNumber != "" {
			inputQty = 1
		}

		InboundDetail.InboundNo = payload.InboundNo
		InboundDetail.InboundId = int(inboundID)
		InboundDetail.ItemCode = item.ItemCode
		InboundDetail.ItemId = product.ID
		InboundDetail.ProductNumber = product.ProductNumber
		InboundDetail.Barcode = uomConversion.Ean
		InboundDetail.Uom = item.UOM
		InboundDetail.Quantity = inputQty
		InboundDetail.Location = item.Location
		InboundDetail.QaStatus = item.QaStatus
		InboundDetail.WhsCode = item.WhsCode
		InboundDetail.RecDate = item.RecDate
		InboundDetail.ProdDate = item.ProdDate
		InboundDetail.ExpDate = item.ExpDate
		InboundDetail.LotNumber = item.LotNumber
		InboundDetail.SerialNumber = item.SerialNumber
		InboundDetail.RefNo = item.RefNo
		InboundDetail.IsSerial = product.HasSerial
		InboundDetail.RefId = int(InboundReference.ID)
		InboundDetail.RefNo = InboundReference.RefNo
		InboundDetail.OwnerCode = payload.OwnerCode
		InboundDetail.WhsCode = payload.WhsCode
		InboundDetail.DivisionCode = item.DivisionCode
		InboundDetail.CreatedBy = userID
		InboundDetail.UpdatedBy = userID

		res := tx.Create(&InboundDetail)

		if res.Error != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert inbound detail",
				"error":   res.Error.Error(),
			})
		}
	}

	errHistory := helpers.InsertTransactionHistory(
		tx,
		payload.InboundNo, // RefNo
		"open",            // Status
		"INBOUND",         // Type
		"",                // Detail
		userID,            // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		tx.Rollback()
		log.Println("Gagal insert history:", errHistory)
	}

	// Commit
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inbound created successfully",
		"data": fiber.Map{
			"inbound_id": inboundID,
		},
	})
}

func (c *InboundController) UpdateInboundByID(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")

	fmt.Printf("UpdateInboundByID called with inbound_no=%s\n", inbound_no)

	var payload Inbound

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var InventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", payload.OwnerCode).First(&InventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	// Check duplicate item code
	itemCodes := make(map[string]bool)
	for _, item := range payload.Items {

		if item.Quantity == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Quantity cannot be zero",
				"error":   "Quantity cannot be zero",
			})
		}

		if item.UOM == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "UOM cannot be empty",
				"error":   "UOM cannot be empty",
			})
		}

		// if InventoryPolicy.UseLotNo {
		// 	if item.LotNumber == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Lot number cannot be empty",
		// 			"error":   "Lot number cannot be empty",
		// 		})
		// 	}
		// }

		// if InventoryPolicy.RequireExpiryDate {
		// 	if item.ExpDate == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Expiration date cannot be empty",
		// 			"error":   "Expiration date cannot be empty",
		// 		})
		// 	}
		// }

		// if InventoryPolicy.UseProductionDate {
		// 	if item.ProdDate == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Production date cannot be empty",
		// 			"error":   "Production date cannot be empty",
		// 		})
		// 	}
		// }

		if InventoryPolicy.UseReceiveLocation {
			if item.Location == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Receive location cannot be empty",
					"error":   "Receive location cannot be empty",
				})
			}
		}

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s, division %s, serial_number %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber),
			})
		}

		itemCodes[key] = true
	}

	payloadItem := payload.Items
	userID := int(ctx.Locals("userID").(float64))

	// Start transaction
	tx := c.DB.Begin()
	log.Printf("[UpdateInbound] START inbound_no=%s userID=%d", inbound_no, userID)
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}

	// Defer rollback in case of panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	inboundRepo := repositories.NewInboundRepository(tx)

	var InboundHeader models.InboundHeader
	if err := tx.First(&InboundHeader, "inbound_no = ?", inbound_no).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	log.Printf("[UpdateInbound] InboundHeader found id=%d status=%s", InboundHeader.ID, InboundHeader.Status)

	var supplier models.Supplier
	if err := tx.First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Check if inbound is complete
	if InboundHeader.Status == "complete" {
		tx.Rollback()
		message := "Inbound " + inbound_no + " is already complete"
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message, "message": message})
	}

	// Update InboundHeader
	InboundHeader.InboundDate = payload.InboundDate
	InboundHeader.Supplier = payload.Supplier
	InboundHeader.SupplierId = int(supplier.ID)
	InboundHeader.ReceiptID = payload.ReceiptID
	InboundHeader.Type = payload.Type
	InboundHeader.Remarks = payload.Remarks
	InboundHeader.UpdatedBy = userID
	InboundHeader.Transporter = payload.Transporter
	InboundHeader.NoTruck = payload.NoTruck
	InboundHeader.Driver = payload.Driver
	InboundHeader.Container = payload.Container
	InboundHeader.WhsCode = payload.WhsCode
	InboundHeader.OwnerCode = payload.OwnerCode
	InboundHeader.Origin = payload.Origin
	InboundHeader.PoDate = payload.PoDate
	InboundHeader.ArrivalTime = payload.ArrivalTime
	InboundHeader.StartUnloading = payload.StartUnloading
	InboundHeader.EndUnloading = payload.EndUnloading
	InboundHeader.TruckSize = payload.TruckSize
	InboundHeader.BLNo = payload.BLNo
	InboundHeader.Koli = payload.Koli

	if err := tx.Model(&models.InboundHeader{}).Where("id = ?", InboundHeader.ID).Updates(InboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var message string

	if InboundHeader.Status == "open" {
		// Update/Create References
		for _, item := range payload.References {

			var InboundReference models.InboundReference
			if err := tx.First(&InboundReference, "id = ?", item.ID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Create new reference
					InboundReference.InboundId = uint(InboundHeader.ID)
					InboundReference.RefNo = item.RefNo
					if err := tx.Create(&InboundReference).Error; err != nil {
						tx.Rollback()
						return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
					}
				} else {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else {
				// Update existing reference
				InboundReference.RefNo = item.RefNo
				if err := tx.Model(&models.InboundReference{}).Where("id = ?", InboundReference.ID).Updates(InboundReference).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

		// Update/Create Items
		for _, item := range payloadItem {

			log.Printf("[UpdateInbound] Processing item=%s id=%d", item.ItemCode, item.ID)
			var product models.Product
			if err := tx.First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
				tx.Rollback()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			var uomConversion models.UomConversion
			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, item.UOM).Error; err != nil {
				tx.Rollback()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found"})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			var inboundDetail models.InboundDetail
			err := tx.First(&inboundDetail, "id = ?", item.ID).Error

			if !InventoryPolicy.UseLotNo {
				item.LotNumber = InboundHeader.InboundNo
			}

			if !InventoryPolicy.RequireExpiryDate {
				// item.ExpDate = item.RecDate
			}

			if !InventoryPolicy.UseProductionDate {
				// item.ProdDate = item.RecDate
			}

			inputQty := item.Quantity
			if item.SerialNumber != "" {
				inputQty = 1
			}

			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create new detail
				newDetail := models.InboundDetail{
					InboundId:     int(InboundHeader.ID),
					InboundNo:     InboundHeader.InboundNo,
					ItemId:        product.ID,
					ProductNumber: product.ProductNumber,
					ItemCode:      item.ItemCode,
					Barcode:       uomConversion.Ean,
					Quantity:      inputQty,
					Location:      item.Location,
					WhsCode:       InboundHeader.WhsCode,
					RecDate:       item.RecDate,
					ProdDate:      item.ProdDate,
					ExpDate:       item.ExpDate,
					LotNumber:     item.LotNumber,
					Uom:           item.UOM,
					IsSerial:      product.HasSerial,
					SerialNumber:  item.SerialNumber,
					RefNo:         item.RefNo,
					RefId:         item.RefId,
					OwnerCode:     InboundHeader.OwnerCode,
					QaStatus:      item.QaStatus,
					DivisionCode:  item.DivisionCode,
					CreatedBy:     userID,
				}
				if err := tx.Create(&newDetail).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else if err == nil {

				inboundBarcode, err := inboundRepo.GetInboundBarcodeByOutboundDetailID(uint(inboundDetail.ID))
				log.Printf("[UpdateInbound] GetInboundBarcode detailID=%d err=%v barcodeID=%d", inboundDetail.ID, err, inboundBarcode.ID)
				if err != nil {

					if errors.Is(err, gorm.ErrRecordNotFound) {
						log.Printf("[UpdateInbound] Barcode not found for detailID=%d, CONTINUE (skipping update)", inboundDetail.ID)
						continue
					}

					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

				if inboundBarcode.ID > 0 {
					if inboundBarcode.ItemID != product.ID {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + inboundDetail.ItemCode + " already scanned, cannot update to " + item.ItemCode})
					}

					if inboundBarcode.ExpDate != item.ExpDate {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Expiry Date"})
					}

					if inboundBarcode.LotNumber != item.LotNumber {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Lot Number"})
					}

					if inboundBarcode.ProdDate != item.ProdDate {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Production Date"})
					}

				}

				if inboundBarcode.ItemID == product.ID {
					if inboundBarcode.TotalScan > int(item.Quantity) {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity update for item " + item.ItemCode + " is less than the total scanned quantity"})
					}
				}

				if InventoryPolicy.UseSerialNumber {
					var inboundBarcode models.InboundBarcode
					if err := tx.First(&inboundBarcode, "inbound_detail_id = ? and status = 'in stock'", inboundDetail.ID).Error; err == nil {
						if inboundBarcode.SerialNumber != item.SerialNumber {
							tx.Rollback()
							return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Serial Number"})
						}
					}
				}

				// Update existing detail
				inboundDetail.ItemId = product.ID
				inboundDetail.ItemCode = item.ItemCode
				inboundDetail.Barcode = uomConversion.Ean
				inboundDetail.Quantity = inputQty
				inboundDetail.Location = item.Location
				inboundDetail.WhsCode = InboundHeader.WhsCode
				inboundDetail.RecDate = item.RecDate
				inboundDetail.ProdDate = item.ProdDate
				inboundDetail.ExpDate = item.ExpDate
				inboundDetail.LotNumber = item.LotNumber
				inboundDetail.SerialNumber = item.SerialNumber
				inboundDetail.Uom = item.UOM
				inboundDetail.IsSerial = product.HasSerial
				inboundDetail.RefNo = item.RefNo
				inboundDetail.RefId = item.RefId
				inboundDetail.OwnerCode = InboundHeader.OwnerCode
				inboundDetail.QaStatus = item.QaStatus
				inboundDetail.DivisionCode = item.DivisionCode
				inboundDetail.UpdatedBy = userID

				if err := tx.Save(&inboundDetail).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else {
				// Other error
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}

		message = "Update Inbound " + InboundHeader.InboundNo + " successfully"
	} else {
		message = "Update Inbound Header " + InboundHeader.InboundNo + " successfully"
	}

	log.Printf("[UpdateInbound] About to COMMIT")
	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	log.Printf("[UpdateInbound] COMMIT SUCCESS")

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": message})
}

func (c *InboundController) GetAllListInbound(ctx *fiber.Ctx) error {

	inboundRepo := repositories.NewInboundRepository(c.DB)
	result, err := inboundRepo.GetAllInbound()

	if len(result) == 0 {
		result = []repositories.ListInbound{}
	}

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("GetAllListInbound result:", result)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

func (c *InboundController) GetInboundListFilter(ctx *fiber.Ctx) error {
	// Parse statuses dari query string "open,checking,partially received"
	var statuses []string
	if raw := ctx.Query("statuses"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	// Parse types (IB Type) dari query string
	var types []string
	if raw := ctx.Query("types"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				types = append(types, s)
			}
		}
	}

	// Parse owners dari query string
	var owners []string
	if raw := ctx.Query("owners"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				owners = append(owners, s)
			}
		}
	}

	params := repositories.InboundFilterParams{
		StartDate:  ctx.Query("start_date"),
		EndDate:    ctx.Query("end_date"),
		Search:     ctx.Query("search"),
		SearchItem: ctx.Query("search_item"),
		Statuses:   statuses,
		Types:      types,
		Owners:     owners,
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)
	result, err := inboundRepo.GetInboundListWithFilter(params)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	if result == nil {
		result = []repositories.ListInbound{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inbound found",
		"data":    result,
	})
}

func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")
	limit := ctx.QueryInt("limit", 5000)

	var inbound models.InboundHeader
	fmt.Println("GetInboundByID inbound_no:", inbound_no)

	var totalDetails int64
	c.DB.Model(&models.InboundDetail{}).
		Where("inbound_no = ?", inbound_no).
		Count(&totalDetails)

	if err := c.DB.Debug().
		Preload("InboundReferences").
		// Received dihapus dari sini
		Preload("Details", func(db *gorm.DB) *gorm.DB {
			return db.Limit(limit).Order("id ASC")
		}).
		First(&inbound, "inbound_no = ?", inbound_no).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)
	inbounDetails, err := inboundRepo.GetInboundDetailByInboundID(inbound.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"data":          inbound,
		"details":       inbounDetails,
		"details_limit": limit,
		"details_total": totalDetails,
	})
}

func (c *InboundController) GetReceivedByInboundNo(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 100)
	offset := (page - 1) * limit
	search := ctx.Query("search", "")
	pallet := ctx.Query("pallet", "")
	caseNumber := ctx.Query("case_number", "")

	// Base query
	query := c.DB.Model(&models.InboundBarcode{}).
		Where("inbound_id = (SELECT id FROM inbound_headers WHERE inbound_no = ? AND deleted_at IS NULL)", inbound_no).
		Where("deleted_at IS NULL")

	// Filter opsional
	if pallet != "" {
		query = query.Where("pallet = ?", pallet)
	}
	if caseNumber != "" {
		query = query.Where("case_number = ?", caseNumber)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"item_code LIKE ? OR barcode LIKE ? OR serial_number LIKE ? OR pallet LIKE ? OR case_number LIKE ? OR lot_number LIKE ?",
			like, like, like, like, like, like,
		)
	}

	var total int64
	query.Count(&total)

	var received []models.InboundBarcode
	if err := query.
		Preload("Product").
		Order("id ASC").
		Limit(limit).
		Offset(offset).
		Find(&received).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    received,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (c *InboundController) GetPalletSummary(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")

	type PalletSummary struct {
		Pallet       string  `json:"pallet"`
		ItemCount    int64   `json:"item_count"`
		CartonCount  int64   `json:"carton_count"`
		TotalQty     float64 `json:"total_qty"`
		PendingCount int64   `json:"pending_count"`
		InStockCount int64   `json:"in_stock_count"`
	}

	var result []PalletSummary
	err := c.DB.Raw(`
	SELECT 
		pallet,
		COUNT(*) as item_count,
		COUNT(DISTINCT NULLIF(case_number, '')) as carton_count,
		SUM(quantity) as total_qty,
		SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_count,
		SUM(CASE WHEN status = 'in stock' THEN 1 ELSE 0 END) as in_stock_count
	FROM inbound_barcodes
	WHERE inbound_id = (
		SELECT id FROM inbound_headers 
		WHERE inbound_no = ? AND deleted_at IS NULL
	)
	AND deleted_at IS NULL
	GROUP BY pallet
	ORDER BY 
		CAST(SUBSTRING(pallet, CHARINDEX('-', pallet, LEN(pallet) - CHARINDEX('-', REVERSE(pallet)) + 1) + 1, LEN(pallet)) AS INT) DESC
    `, inbound_no).Scan(&result).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

func (c *InboundController) GetCartonSummary(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 50)
	offset := (page - 1) * limit
	search := ctx.Query("search", "")

	// Build WHERE tambahan untuk search
	searchWhere := ""
	searchArgs := []interface{}{inbound_no}
	if search != "" {
		searchWhere = "AND (case_number LIKE ? OR pallet LIKE ?)"
		like := "%" + search + "%"
		searchArgs = append(searchArgs, like, like)
	}

	type CartonSummary struct {
		CaseNumber string  `json:"case_number"`
		Pallet     string  `json:"pallet"`
		ItemCount  int64   `json:"item_count"`
		TotalQty   float64 `json:"total_qty"`
		AllInStock int     `json:"all_in_stock"`
	}

	var total int64
	countArgs := append([]interface{}{}, searchArgs...)
	c.DB.Raw(`
        SELECT COUNT(DISTINCT case_number)
        FROM inbound_barcodes
        WHERE inbound_id = (
            SELECT id FROM inbound_headers
            WHERE inbound_no = ? AND deleted_at IS NULL
        )
        AND deleted_at IS NULL
        AND case_number != ''
        `+searchWhere,
		countArgs...,
	).Scan(&total)

	dataArgs := append(searchArgs, offset, limit)
	var result []CartonSummary
	err := c.DB.Raw(`
        SELECT
            case_number,
            MAX(pallet) as pallet,
            COUNT(*) as item_count,
            SUM(quantity) as total_qty,
            CASE WHEN SUM(CASE WHEN status != 'in stock' THEN 1 ELSE 0 END) = 0
                 THEN 1 ELSE 0 END as all_in_stock
        FROM inbound_barcodes
        WHERE inbound_id = (
            SELECT id FROM inbound_headers
            WHERE inbound_no = ? AND deleted_at IS NULL
        )
        AND deleted_at IS NULL
        AND case_number != ''
        `+searchWhere+`
        GROUP BY case_number
        ORDER BY case_number DESC
        OFFSET ? ROWS FETCH NEXT ? ROWS ONLY
    `, dataArgs...).Scan(&result).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 {
		totalPages = 1
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

func (c *InboundController) GetItem(ctx *fiber.Ctx) error {

	inbound_detail_id := ctx.Params("id")
	var inboundDetail models.InboundDetail
	if err := c.DB.Debug().First(&inboundDetail, "id = ?", inbound_detail_id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resultItem := InboundItem{
		// ID: types.SnowflakeID(inboundDetail.ID),
		ID: int(inboundDetail.ID),
		// InboundID: types.SnowflakeID(inboundDetail.InboundId),
		InboundID: int(inboundDetail.InboundId),
		ItemCode:  inboundDetail.ItemCode,
		Quantity:  inboundDetail.Quantity,
		UOM:       inboundDetail.Uom,
		WhsCode:   inboundDetail.WhsCode,
		RecDate:   inboundDetail.RecDate,
		Remarks:   inboundDetail.Remarks,
		IsSerial:  inboundDetail.IsSerial,
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item found successfully", "data": resultItem})
}

func (c *InboundController) DeleteItem(ctx *fiber.Ctx) error {

	inbound_detail_id := ctx.Params("id")
	var inboundDetail models.InboundDetail
	if err := c.DB.Debug().First(&inboundDetail, "id = ?", inbound_detail_id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var InboundHeader models.InboundHeader
	if err := c.DB.Debug().First(&InboundHeader, "inbound_no = ?", inboundDetail.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if InboundHeader.Status != "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + inboundDetail.InboundNo + " is not open", "message": "Inbound not open"})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	inboundBarcode, err := inboundRepo.GetInboundBarcodeByOutboundDetailID(uint(inboundDetail.ID))
	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// data barcode tidak ada → boleh lanjut delete
			goto DELETE
		}

		return ctx.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{"error": err.Error()})
	}

	if inboundBarcode.ItemID == inboundDetail.ItemId {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This item is already scanned", "message": "Item already scanned"})
	}

DELETE:
	// hard delete
	if err := c.DB.Debug().Unscoped().Delete(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

func (c *InboundController) GetPutawaySheet(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	type PutawaySheet struct {
		OwnerCode      string  `json:"owner_code"`
		InboundDate    string  `json:"inbound_date"`
		ReceiptID      string  `json:"receipt_id"`
		PoNumber       string  `json:"po_number"`
		InboundNo      string  `json:"inbound_no"`
		ItemCode       string  `json:"item_code"`
		ItemName       string  `json:"item_name"`
		Barcode        string  `json:"barcode"`
		SupplierName   string  `json:"supplier_name"`
		LotNumber      string  `json:"lot_number"`
		Quantity       int     `json:"quantity"`
		RecDate        string  `json:"rec_date"`
		ProdDate       string  `json:"prod_date"`
		ExpDate        string  `json:"exp_date"`
		Uom            string  `json:"uom"`
		Transporter    string  `json:"transporter"`
		NoTruck        string  `json:"no_truck"`
		Driver         string  `json:"driver"`
		TruckSize      string  `json:"truck_size"`
		ArrivalTime    string  `json:"arrival_time"`
		StartUnloading string  `json:"start_unloading"`
		EndUnloading   string  `json:"end_unloading"`
		Cbm            float64 `json:"cbm"`
		BLNo           string  `json:"bl_no"`
		Remarks        string  `json:"remarks"`
		Koli           int     `json:"koli"`
		Container      string  `json:"container"`
		WhsCode        string  `json:"whs_code"`
	}

	sql := `SELECT b.inbound_date, b.inbound_no, b.receipt_id, tp.transporter_name as transporter,
	b.no_truck, b.driver, b.truck_size, b.arrival_time, b.start_unloading, b.end_unloading, p.item_name,
	a.item_code, a.barcode, p.cbm, b.bl_no, b.remarks, b.koli, b.container, a.whs_code,
	s.supplier_name, a.quantity, a.uom, b.owner_code, a.exp_date, a.prod_date, a.rec_date, a.lot_number
	FROM inbound_details a
	INNER JOIN inbound_headers b ON a.inbound_id = b.id
	LEFT JOIN suppliers s ON b.supplier_id = s.id
	LEFT JOIN transporters tp ON b.transporter = tp.transporter_code
	LEFT JOIN products p ON a.item_code = p.item_code
	WHERE inbound_id = ?`

	var putawaySheet []PutawaySheet
	if err := c.DB.Raw(sql, id).Scan(&putawaySheet).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println(putawaySheet)

	var inventoryPolicy models.InventoryPolicy
	if len(putawaySheet) > 0 {
		if err := c.DB.Debug().First(&inventoryPolicy, "owner_code = ?", putawaySheet[0].OwnerCode).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway Sheet Found", "data": fiber.Map{
		"putaway_sheet":    putawaySheet,
		"inventory_policy": inventoryPolicy,
	}})
}

func (c *InboundController) PutawayByInboundNo(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// var inboundBarcodesCheck01 []models.InboundBarcode
	// if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundBarcodesCheck01).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	var invPolicy models.InventoryPolicy
	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var warehouse models.Warehouse
	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if invPolicy.RequirePutawayScan {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
	}

	var inboundDetail []models.InboundDetail
	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if !invPolicy.RequireReceiveScan {

		var allRcvLocationIsFilled bool = true
		for _, detail := range inboundDetail {
			if detail.Location == "" {
				allRcvLocationIsFilled = false
				break
			}
		}

		if !allRcvLocationIsFilled {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
		}

		for _, detail := range inboundDetail {

			var inboundBarcodesCheck []models.InboundBarcode
			if err := c.DB.Debug().Where("inbound_detail_id = ?", detail.ID).Find(&inboundBarcodesCheck).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			totalQtyReq := detail.Quantity
			totalQtyScanned := float64(0)
			newQtyScanned := float64(0)

			for _, inboundBarcode := range inboundBarcodesCheck {
				totalQtyScanned += inboundBarcode.Quantity
			}

			newQtyScanned = totalQtyReq - totalQtyScanned

			inputSerialNumber := detail.Barcode
			if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
				inputSerialNumber = detail.SerialNumber
			}

			var location models.Location
			if err := c.DB.Debug().First(&location, "location_code = ?", detail.Location).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location " + detail.Location + " not registered in system"})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if newQtyScanned > 0 {
				newInboundBarcode := models.InboundBarcode{
					InboundId:       int(inboundHeader.ID),
					InboundDetailId: int(detail.ID),
					ItemCode:        detail.ItemCode,
					ItemID:          detail.ItemId,
					ScanData:        detail.Barcode,
					Barcode:         detail.Barcode,
					SerialNumber:    inputSerialNumber,
					Pallet:          payload.InboundNo,
					Location:        location.LocationCode,
					Quantity:        newQtyScanned,
					WhsCode:         detail.WhsCode,
					OwnerCode:       detail.OwnerCode,
					DivisionCode:    detail.DivisionCode,
					QaStatus:        detail.QaStatus,
					Status:          "pending",
					Uom:             detail.Uom,
					RecDate:         detail.RecDate,
					ProdDate:        detail.ProdDate,
					ExpDate:         detail.ExpDate,
					LotNumber:       detail.LotNumber,
					CreatedBy:       int(ctx.Locals("userID").(float64)),
				}

				if err := c.DB.Debug().Create(&newInboundBarcode).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

	}

	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
	}

	for _, barcode := range inboundBarcodes {
		barcodeIDStr := strconv.Itoa(int(barcode.ID))

		if err := c.servicePutawayPerItem(ctx, barcodeIDStr); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   err.Error(),
				"message": err.Error(),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Putaway inbound " + inboundHeader.InboundNo + " successfully"})
}

func (c *InboundController) servicePutawayPerItem(ctx *fiber.Ctx, idStr string) error {
	if idStr == "" {
		return errors.New("Inbound Barcode ID cannot be empty")
	}

	inboundBarcode := models.InboundBarcode{}
	if err := c.DB.Debug().First(&inboundBarcode, "id = ?", idStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Inbound barcode not found")
		}
		return errors.New(err.Error())
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "id = ?", inboundBarcode.InboundId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Inbound not found")
		}
		return errors.New(err.Error())
	}

	invPolicy := models.InventoryPolicy{}
	if err := c.DB.Debug().Where("owner_code = ?", inboundHeader.OwnerCode).First(&invPolicy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Inventory policy not found")
		}
		return errors.New(err.Error())
	}

	if invPolicy.RequirePutawayScan {
		return errors.New("Putaway scan required")
	}

	var warehouse models.Warehouse
	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
		return errors.New("Warehouse not found")
	}

	inboundHeaderID := inboundHeader.ID

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return errors.New("Invalid ID")
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	_, errs := inboundRepo.ProcessPutawayItem(ctx, id, "")
	if errs != nil {
		fmt.Println("Error ProcessPutawayItem", errs.Error())
		return errs
	}

	errs = inboundRepo.UpdateStatusInbound(ctx, inboundHeaderID)
	if errs != nil {
		fmt.Println("Error UpdateStatusInbound", errs.Error())
		return errs
	}

	return nil
}

type PutawayBulkRequest struct {
	ItemIDs []string `json:"item_ids"`
}

func (c *InboundController) PutawayBulk(ctx *fiber.Ctx) error {
	var req PutawayBulkRequest

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	for _, id := range req.ItemIDs {
		if errPutaway := c.servicePutawayPerItem(ctx, id); errPutaway != nil {

			fmt.Println("Putaway Error", errPutaway)
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{

				"error": fmt.Sprintf("Failed putaway ID %s: %v", id, errPutaway),
			})
		}
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("%d items putaway successfully", len(req.ItemIDs)),
	})
}

func (r *InboundController) HandleChecking(ctx *fiber.Ctx) error {

	var payload struct {
		InboundNo string `json:"inbound_no"`
	}

	// Parse JSON payload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	InboundHeader := models.InboundHeader{}
	if err := r.DB.First(&InboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	Warehouse := models.Warehouse{}
	if err := r.DB.First(&Warehouse, "code = ?", InboundHeader.WhsCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Warehouse not found"})
	}

	Transporter := models.Transporter{}
	if err := r.DB.First(&Transporter, "transporter_code = ?", InboundHeader.Transporter).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transporter not found"})
	}

	if InboundHeader.PoDate == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PO date is required"})
	}

	inboundDetail := []models.InboundDetail{}
	if err := r.DB.Where("inbound_id = ?", InboundHeader.ID).Find(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if len(inboundDetail) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " has no details", "message": "Inbound has no details"})
	}

	uomRepo := repositories.NewUomRepository(r.DB)

	for _, detail := range inboundDetail {
		_, errUom := uomRepo.CheckUomConversionExists(detail.ItemCode, detail.Uom)
		if errUom != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   errUom.Error(),
				"message": errUom.Error(),
			})
		}
	}

	if InboundHeader.Status == "checking" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " is already checking", "message": "Inbound already checking"})
	}

	if InboundHeader.Status != "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " is not open", "message": "Inbound not open"})
	}

	// userID := int(ctx.Locals("userID").(float64))
	// now := time.Now()

	// err := r.DB.Model(&InboundHeader).Updates(map[string]interface{}{
	// 	"status":       "checking",
	// 	"raw_status":   "CONFIRMED",
	// 	"confirm_time": now,
	// 	"confirm_by":   userID,
	// 	"updated_at":   now,
	// 	"updated_by":   userID,
	// 	"checking_at":  now,
	// 	"checking_by":  userID,
	// }).Error

	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"error": err.Error(),
	// 	})
	// }

	// errHistory := helpers.InsertTransactionHistory(
	// 	r.DB,
	// 	payload.InboundNo, // RefNo
	// 	"checking",        // Status
	// 	"INBOUND",         // Type
	// 	"",                // Detail
	// 	userID,            // CreatedBy / UpdatedBy
	// )
	// if errHistory != nil {
	// 	log.Println("Gagal insert history:", errHistory)
	// }

	inboundRepo := repositories.NewInboundRepository(r.DB)
	err := inboundRepo.UpdateStatusInbound(ctx, InboundHeader.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change status inbound " + payload.InboundNo + " to checking successfully"})
}

func (r *InboundController) HandleChecked(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}

	// Parse JSON payload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	InboundHeader := models.InboundHeader{}
	if err := r.DB.Debug().First(&InboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if InboundHeader.Status == "checking" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " is already checking", "message": "Inbound already checking"})
	}

	if InboundHeader.Status != "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " is not open", "message": "Inbound not open"})
	}

	InboundDetails := []models.InboundDetail{}
	if err := r.DB.Debug().Where("inbound_id = ?", InboundHeader.ID).
		Find(&InboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(InboundDetails) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " has no details", "message": "Inbound has no details"})
	}

	for _, detail := range InboundDetails {
		if detail.Location == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " has no rcv_location for item " + detail.ItemCode, "message": "Inbound has no rcv_location"})
		}
	}

	for _, detail := range InboundDetails {

		product := models.Product{}
		if err := r.DB.Debug().First(&product, "item_code = ?", detail.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " has item " + detail.ItemCode + " not found", "message": "Inbound item not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		inboundBarcode := models.InboundBarcode{
			InboundId:       int(InboundHeader.ID),
			InboundDetailId: int(detail.ID),
			ItemID:          product.ID,
			ItemCode:        detail.ItemCode,
			ScanType:        "BARCODE",
			ScanData:        product.Barcode,
			Barcode:         product.Barcode,
			SerialNumber:    product.Barcode,
			Pallet:          detail.Location,
			Location:        detail.Location,
			Quantity:        detail.Quantity,
			WhsCode:         InboundHeader.WhsCode,
			OwnerCode:       InboundHeader.OwnerCode,
			DivisionCode:    detail.DivisionCode,
			QaStatus:        detail.QaStatus,
			Status:          "pending",
			CreatedBy:       int(ctx.Locals("userID").(float64)),
			UpdatedBy:       int(ctx.Locals("userID").(float64)),
		}

		// Create InboundBarcode
		if err := r.DB.Debug().Create(&inboundBarcode).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// update inbound status inbound header with interface
	sqlUpdate := `UPDATE inbound_headers SET status = 'checked', updated_at = ?, updated_by = ? WHERE inbound_no = ?`
	if err := r.DB.Exec(sqlUpdate, time.Now(), int(ctx.Locals("userID").(float64)), payload.InboundNo).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	errHistory := helpers.InsertTransactionHistory(
		r.DB,
		payload.InboundNo, // RefNo
		"checked",         // Status
		"INBOUND",         // Type
		"All items checked without scan using scanner", // Detail
		int(ctx.Locals("userID").(float64)),            // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change status inbound " + payload.InboundNo + " to checked successfully"})
}

func (r *InboundController) HandleOpen(ctx *fiber.Ctx) error {

	var payload struct {
		InboundNo string `json:"inbound_no"`
	}

	// Parse JSON payload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	InboundHeader := models.InboundHeader{}
	if err := r.DB.Debug().First(&InboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if InboundHeader.Status == "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + payload.InboundNo + " is already open", "message": "Inbound " + payload.InboundNo + " already open"})
	}

	allowed := map[string]bool{
		"checking":           true,
		"partially received": true,
		"fully received":     true,
	}

	if !allowed[InboundHeader.Status] {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Cannot open Inbound " + payload.InboundNo + " because it is " + InboundHeader.Status,
			"message": "Cannot open Inbound " + payload.InboundNo + " because it is " + InboundHeader.Status,
		})
	}

	// type CheckResultInbound struct {
	// 	InboundNo       string `json:"inbound_no"`
	// 	InboundDetailId int    `json:"inbound_detail_id"`
	// 	ItemId          int    `json:"item_id"`
	// 	ItemCode        string `json:"item_code"`
	// 	Quantity        int    `json:"quantity"`
	// 	QtyScan         int    `json:"qty_scan"`
	// }

	// sqlCheck := `WITH ib AS
	// (
	// 	SELECT inbound_id, inbound_detail_id, item_id, SUM(quantity) AS qty_scan, status
	// 	FROM inbound_barcodes WHERE inbound_id = ?
	// 	GROUP BY inbound_id, inbound_detail_id, item_id, status
	// )

	// SELECT a.id, a.inbound_no, a.inbound_id, a.item_id, a.quantity, COALESCE(ib.qty_scan, 0) AS qty_scan, a.item_code
	// FROM inbound_details a
	// LEFT JOIN ib ON a.id = ib.inbound_detail_id
	// WHERE a.inbound_id = ?`

	// var checkResult []CheckResultInbound
	// if err := r.DB.Raw(sqlCheck, InboundHeader.ID, InboundHeader.ID).Scan(&checkResult).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// for _, result := range checkResult {
	// 	if result.QtyScan > 0 {
	// 		message := "Inbound " + payload.InboundNo + ", item " + result.ItemCode + " has been scanned"
	// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message, "message": "Cannot change status to open, inbound " + payload.InboundNo + ", item " + result.ItemCode + " has been scanned"})
	// 	}
	// }

	userID := uint64(ctx.Locals("userID").(float64))
	now := time.Now()

	err := r.DB.Model(&InboundHeader).Updates(map[string]interface{}{
		"status":               "open",
		"raw_status":           "DRAFT",
		"change_to_draft_time": now,
		"change_to_draft_by":   userID,
		"updated_at":           now,
		"updated_by":           userID,
		"cancel_at":            now,
		"cancel_by":            userID,
	}).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	errHistory := helpers.InsertTransactionHistory(
		r.DB,
		payload.InboundNo,                   // RefNo
		"open",                              // Status
		"INBOUND",                           // Type
		"",                                  // Detail
		int(ctx.Locals("userID").(float64)), // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change status inbound " + payload.InboundNo + " to open successfully"})
}
func (c *InboundController) HandleComplete(ctx *fiber.Ctx) error {

	inboundNo := ctx.Params("inbound_no")
	if inboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Inbound No"})
	}

	var inboundHeader models.InboundHeader

	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ? AND status <> 'complete'", inboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var transporter models.Transporter
	if err := c.DB.Debug().First(&transporter, "transporter_code = ?", inboundHeader.Transporter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Transporter not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	id := inboundHeader.ID

	type CheckResult struct {
		InboundNo       string `json:"inbound_no"`
		InboundDetailId int    `json:"inbound_detail_id"`
		ItemId          int    `json:"item_id"`
		Quantity        int    `json:"quantity"`
		QtyScan         int    `json:"qty_scan"`
	}

	sqlCheck := `WITH ib AS
	(
		SELECT inbound_id, inbound_detail_id, item_id, SUM(quantity) AS qty_scan, status 
		FROM inbound_barcodes WHERE inbound_id = ? AND status = 'in stock'
		GROUP BY inbound_id, inbound_detail_id, item_id, status
	)

	SELECT a.id, a.inbound_no, a.inbound_id, a.item_id, a.quantity, COALESCE(ib.qty_scan, 0) AS qty_scan
	FROM inbound_details a
	LEFT JOIN ib ON a.id = ib.inbound_detail_id
	WHERE a.inbound_id = ?`

	var checkResult []CheckResult
	if err := c.DB.Raw(sqlCheck, id, id).Scan(&checkResult).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, result := range checkResult {
		if result.Quantity != result.QtyScan {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Putaway not complete", "message": "Inbound " + result.InboundNo + " cannot be completed, item " + strconv.Itoa(result.ItemId) + " has not been scanned completely"})
		}
	}

	// update inbound status inbound header with interface
	userID := uint64(ctx.Locals("userID").(float64))
	now := time.Now()

	err := c.DB.Model(&inboundHeader).Updates(map[string]interface{}{
		"status":        "complete",
		"raw_status":    "COMPLETED",
		"complete_time": now,
		"updated_by":    userID,
		"updated_at":    now,
		"complete_at":   now,
		"complete_by":   userID,
	}).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	errHistory := helpers.InsertTransactionHistory(
		c.DB,
		inboundHeader.InboundNo,             // RefNo
		"complete",                          // Status
		"INBOUND",                           // Type
		"",                                  // Detail
		int(ctx.Locals("userID").(float64)), // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound " + inboundHeader.InboundNo + " completed successfully"})
}

func (c *InboundController) GetInventoryByInbound(ctx *fiber.Ctx) error {

	inboundNo := ctx.Params("inbound_no")
	if inboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Inbound No"})
	}

	var inboundHeader models.InboundHeader

	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", inboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundID := int(inboundHeader.ID)
	repositories := repositories.NewInventoryRepository(c.DB)
	inventories, err := repositories.GetInventoryByInbound(inboundID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inventories})
}

func (c *InboundController) GetSummaryInboundActivity(ctx *fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

// =======================================
// STRUCTS - REVISED
// =======================================

type ExcelUploadResponse struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	TotalRows        int               `json:"total_rows"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	InboundNumbers   []string          `json:"inbound_numbers,omitempty"`
	Errors           []ExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

type ExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Row     int    `json:"row"`
}

// ExcelInboundRow merepresentasikan SATU baris Excel secara penuh:
// gabungan field header (yang akan sama untuk semua baris dalam 1 ReceiptID)
// dan field detail (yang unik per baris).
//
// Sebelumnya header dan detail dipisah menjadi dua struct berbeda
// (ExcelInboundHeader & ExcelInboundDetail), dengan header hanya dibaca
// sekali dari baris pertama. Sekarang setiap baris membawa kedua informasi
// ini sekaligus, supaya grouping per ReceiptID bisa dilakukan dengan benar.
type ExcelInboundRow struct {
	Row int // nomor baris Excel (untuk pesan error)

	// ---- Header fields (kolom 0-18) ----
	ReceiptID      string
	InboundDate    string
	Type           string // IB Type: RETURN / NORMAL
	Supplier       string
	Transporter    string // Trucker
	Driver         string
	WhsCode        string
	OwnerCode      string
	Origin         string
	PoDate         string
	NoTruck        string
	Container      string
	TruckSize      string
	ArrivalTime    string
	StartUnloading string
	EndUnloading   string
	BLNo           string
	Koli           int
	Remarks        string

	// ---- Detail fields (kolom 19-28) ----
	ItemCode  string
	UOM       string
	Quantity  float64
	Location  string
	QaStatus  string
	RecDate   string
	ProdDate  string
	ExpDate   string
	LotNumber string
	Division  string
}

// InboundGroup adalah hasil grouping baris-baris Excel berdasarkan ReceiptID.
// HeaderRow menyimpan informasi header (sudah divalidasi konsisten),
// Details menyimpan semua baris detail dalam grup tersebut.
type InboundGroup struct {
	ReceiptID string
	HeaderRow ExcelInboundRow   // representasi header grup (diambil dari baris pertama, sudah divalidasi konsisten)
	Details   []ExcelInboundRow // semua baris (dipakai juga sebagai detail)
}

// Kolom-kolom header yang WAJIB konsisten (sama) di semua baris
// dalam satu ReceiptID yang sama.
var headerConsistencyFields = []string{
	"InboundDate", "Type", "Supplier", "Transporter", "Driver",
	"WhsCode", "OwnerCode", "Origin", "PoDate", "NoTruck",
	"Container", "TruckSize", "ArrivalTime", "StartUnloading",
	"EndUnloading", "BLNo", "Remarks",
	// Koli tidak dimasukkan di sini karena perlu perbandingan pointer khusus,
	// ditangani terpisah di validateHeaderConsistency.
}

// validIBTypes adalah daftar nilai valid untuk kolom IB Type.
// Case-sensitive, harus persis sama.
var validIBTypes = map[string]bool{
	"RETURN": true,
	"NORMAL": true,
}

// =======================================
// CELL READING HELPERS
// =======================================

func getCell(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

// Kolom-kolom di template Excel (header dulu, baru detail).
// Disusun sebagai konstanta supaya gampang dirawat kalau urutan kolom berubah lagi.
const (
	colReceiptID      = 0
	colInboundDate    = 1
	colType           = 2 // IB Type
	colSupplier       = 3
	colTransporter    = 4 // Trucker
	colDriver         = 5
	colWhsCode        = 6
	colOwnerCode      = 7
	colOrigin         = 8
	colPoDate         = 9
	colNoTruck        = 10
	colContainer      = 11
	colTruckSize      = 12
	colArrivalTime    = 13
	colStartUnloading = 14
	colEndUnloading   = 15
	colBLNo           = 16
	colKoli           = 17
	colRemarks        = 18

	colItemCode  = 19
	colUOM       = 20
	colQuantity  = 21
	colLocation  = 22
	colQaStatus  = 23
	colRecDate   = 24
	colProdDate  = 25
	colExpDate   = 26
	colLotNumber = 27
	colDivision  = 28
)

// getCellAsDateStrict mem-parsing cell sebagai tanggal, mendukung Excel
// serial date maupun beberapa format string umum. Mengembalikan format
// kanonik "2006-01-02". Dipakai untuk InboundDate, PoDate, RecDate, dst.
func getCellAsDateStrict(row []string, index int) (string, error) {
	cellValue := strings.TrimSpace(getCell(row, index))
	if cellValue == "" {
		return "", fmt.Errorf("date value is empty")
	}
	return parseDateString(cellValue)
}

func parseDateString(cellValue string) (string, error) {
	// 1. Excel serial date
	if days, err := strconv.ParseFloat(cellValue, 64); err == nil {
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		date := excelEpoch.Add(time.Duration(days * 24 * float64(time.Hour)))
		return date.Format("2006-01-02"), nil
	}

	// 2. String date formats
	dateFormats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"2-Jan-06",
		"2-January-2006",
	}

	for _, format := range dateFormats {
		if t, err := time.Parse(format, cellValue); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("invalid date format: %s", cellValue)
}

// validateTimeStrict memvalidasi format jam HH:mm (24 jam), contoh "10:30", "15:54".
// String kosong dianggap valid (karena field jam ini opsional) dan akan
// mengembalikan string kosong tanpa error.
func validateTimeStrict(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	t, err := time.Parse("15:04", value)
	if err != nil {
		return "", fmt.Errorf("invalid time format (expected HH:mm): %s", value)
	}
	// Normalisasi ke format HH:mm dua digit (misal "9:5" -> "09:05")
	return t.Format("15:04"), nil
}

// parseKoli mem-parsing kolom Koli yang opsional menjadi *int.
// String kosong menghasilkan nil (bukan 0), supaya bisa disimpan sebagai
// NULL di database dan tidak ambigu dengan "memang nol koli".
func parseKoli(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid koli value (must be numeric): %s", value)
	}
	return n, nil
}

// =======================================
// PARSE ALL ROWS (header + detail digabung per baris)
// =======================================

// parseAllRows membaca SEMUA baris data (mulai row Excel ke-2) sebagai
// ExcelInboundRow lengkap (header + detail). Ini menggantikan kombinasi
// parseHeaderFromExcel (yang lama, hanya baca row pertama) dan
// parseDetailsFromExcel (yang lama, tidak membaca kolom header).
//
// policy dipakai untuk validasi conditional yang sudah ada sebelumnya
// (UseLotNo, UseProductionDate, UseReceiveLocation, UseFEFO).
func (c *InboundController) parseAllRows(rows [][]string, policy models.InventoryPolicy) ([]ExcelInboundRow, []ValidationError) {
	var parsedRows []ExcelInboundRow
	var errs []ValidationError

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1 // nomor baris Excel (1-indexed, +1 karena row Excel dimulai dari 1 dan ada header)

		// Lewati baris yang benar-benar kosong (semua kolom blank)
		if isRowEmpty(row) {
			continue
		}

		r := ExcelInboundRow{Row: rowNum}
		rowErrs := []ValidationError{}

		addErr := func(field, message string) {
			rowErrs = append(rowErrs, ValidationError{Field: field, Message: message, Row: rowNum})
		}

		// ---------- HEADER FIELDS ----------

		r.ReceiptID = getCell(row, colReceiptID)
		if r.ReceiptID == "" {
			addErr("ReceiptID", "Receipt ID cannot be empty")
		}

		inboundDate, err := getCellAsDateStrict(row, colInboundDate)
		if err != nil {
			addErr("InboundDate", "Invalid or empty Inbound Date: "+err.Error())
		}
		r.InboundDate = inboundDate

		r.Type = getCell(row, colType)
		if r.Type == "" {
			addErr("Type", "IB Type cannot be empty")
		} else if !validIBTypes[r.Type] {
			addErr("Type", fmt.Sprintf("IB Type must be RETURN or NORMAL (case-sensitive), got: %s", r.Type))
		}

		r.Supplier = getCell(row, colSupplier)
		if r.Supplier == "" {
			addErr("Supplier", "Supplier cannot be empty")
		}

		r.Transporter = getCell(row, colTransporter)
		if r.Transporter == "" {
			addErr("Transporter", "Trucker cannot be empty")
		}

		r.Driver = getCell(row, colDriver) // opsional

		r.WhsCode = getCell(row, colWhsCode)
		if r.WhsCode == "" {
			addErr("WhsCode", "Warehouse Code cannot be empty")
		}

		r.OwnerCode = getCell(row, colOwnerCode)
		if r.OwnerCode == "" {
			addErr("OwnerCode", "Owner Code cannot be empty")
		}

		r.Origin = getCell(row, colOrigin)
		if r.Origin == "" {
			addErr("Origin", "Origin cannot be empty")
		}

		poDate, err := getCellAsDateStrict(row, colPoDate)
		if err != nil {
			addErr("PoDate", "Invalid or empty PO Date: "+err.Error())
		}
		r.PoDate = poDate

		r.NoTruck = getCell(row, colNoTruck)     // opsional
		r.Container = getCell(row, colContainer) // opsional
		r.TruckSize = getCell(row, colTruckSize) // opsional

		arrivalTime, err := validateTimeStrict(getCell(row, colArrivalTime))
		if err != nil {
			addErr("ArrivalTime", err.Error())
		}
		r.ArrivalTime = arrivalTime

		startUnloading, err := validateTimeStrict(getCell(row, colStartUnloading))
		if err != nil {
			addErr("StartUnloading", err.Error())
		}
		r.StartUnloading = startUnloading

		endUnloading, err := validateTimeStrict(getCell(row, colEndUnloading))
		if err != nil {
			addErr("EndUnloading", err.Error())
		}
		r.EndUnloading = endUnloading

		r.BLNo = getCell(row, colBLNo) // opsional

		koli, err := parseKoli(getCell(row, colKoli))
		if err != nil {
			addErr("Koli", err.Error())
		}
		r.Koli = koli

		r.Remarks = getCell(row, colRemarks) // opsional

		// ---------- DETAIL FIELDS ----------

		r.ItemCode = getCell(row, colItemCode)
		if r.ItemCode == "" {
			addErr("ItemCode", "Item code cannot be empty")
		}

		r.UOM = getCell(row, colUOM)
		if r.UOM == "" {
			addErr("UOM", "UOM cannot be empty")
		}

		qtyStr := getCell(row, colQuantity)
		if qtyStr == "" {
			addErr("Quantity", "Quantity cannot be empty")
		} else {
			qty, err := strconv.ParseFloat(qtyStr, 64)
			if err != nil {
				addErr("Quantity", "Invalid quantity format: "+qtyStr)
			} else if qty == 0 {
				addErr("Quantity", "Quantity cannot be zero")
			} else {
				r.Quantity = qty
			}
		}

		r.Location = getCell(row, colLocation)
		r.QaStatus = getCell(row, colQaStatus)

		recDate, err := getCellAsDateStrict(row, colRecDate)
		if err != nil {
			addErr("RecDate", "Invalid RecDate format: "+err.Error())
		}
		r.RecDate = recDate

		prodDate, err := getCellAsDateStrict(row, colProdDate)
		if err != nil {
			if policy.UseProductionDate {
				addErr("ProdDate", "Production date is required by inventory policy: "+err.Error())
			}
			// kalau tidak required oleh policy, biarkan kosong tanpa error
		}
		r.ProdDate = prodDate

		expDate, err := getCellAsDateStrict(row, colExpDate)
		if err != nil {
			if policy.UseFEFO {
				addErr("ExpDate", "Expiration date is required by inventory policy: "+err.Error())
			}
		}
		r.ExpDate = expDate

		r.LotNumber = getCell(row, colLotNumber)
		if policy.UseLotNo && r.LotNumber == "" {
			addErr("LotNumber", "Lot number is required by inventory policy")
		}

		if policy.UseReceiveLocation && r.Location == "" {
			addErr("Location", "Receive location is required by inventory policy")
		}

		r.Division = getCell(row, colDivision)

		// Validasi item & UOM ke database (sama seperti logic lama)
		if r.ItemCode != "" {
			var product models.Product
			if err := c.DB.First(&product, "item_code = ? AND owner_code = ?", r.ItemCode, r.OwnerCode).Error; err != nil {
				addErr("ItemCode", "Product not found for item code: "+r.ItemCode)
			} else if r.UOM != "" {
				var uomConversion models.UomConversion
				if err := c.DB.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, r.UOM).Error; err != nil {
					addErr("UOM", "UOM Conversion not found for item code: "+r.ItemCode)
				}
			}
		}

		if len(rowErrs) > 0 {
			errs = append(errs, rowErrs...)
			continue // baris ini tidak diikutkan ke parsedRows karena ada error
		}

		parsedRows = append(parsedRows, r)
	}

	return parsedRows, errs
}

func isRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// =======================================
// GROUPING & CONSISTENCY VALIDATION
// =======================================

// groupRowsByReceiptID mengelompokkan baris-baris berdasarkan ReceiptID,
// dengan urutan grup yang STABIL sesuai kemunculan pertama ReceiptID
// tersebut di file Excel (bukan urutan acak seperti map biasa di Go).
func groupRowsByReceiptID(rows []ExcelInboundRow) []InboundGroup {
	groupIndex := make(map[string]int) // ReceiptID -> index di slice groups
	var groups []InboundGroup

	for _, row := range rows {
		idx, exists := groupIndex[row.ReceiptID]
		if !exists {
			groups = append(groups, InboundGroup{
				ReceiptID: row.ReceiptID,
				HeaderRow: row, // baris pertama jadi representasi header
			})
			idx = len(groups) - 1
			groupIndex[row.ReceiptID] = idx
		}
		groups[idx].Details = append(groups[idx].Details, row)
	}

	return groups
}

// validateHeaderConsistency memastikan semua baris dalam satu grup
// (ReceiptID yang sama) punya nilai field header yang SAMA. Kalau ada
// baris yang berbeda, dianggap kesalahan input dan harus direject dengan
// pesan yang jelas (field apa, row mana, beda dengan apa).
func validateGroupHeaderConsistency(group InboundGroup) []ValidationError {
	var errs []ValidationError
	ref := group.HeaderRow

	for _, row := range group.Details {
		if row.Row == ref.Row {
			continue // baris referensi sendiri, skip
		}

		for _, field := range headerConsistencyFields {
			refVal := getFieldValue(ref, field)
			rowVal := getFieldValue(row, field)
			if refVal != rowVal {
				errs = append(errs, ValidationError{
					Field: field,
					Row:   row.Row,
					Message: fmt.Sprintf(
						"Inconsistent %s within Receipt ID %s: row %d has '%s', expected '%s' (from row %d)",
						field, group.ReceiptID, row.Row, rowVal, refVal, ref.Row,
					),
				})
			}
		}

		// Koli ditangani khusus karena *int, bukan string
		if !koliEqual(ref.Koli, row.Koli) {
			errs = append(errs, ValidationError{
				Field: "Koli",
				Row:   row.Row,
				Message: fmt.Sprintf(
					"Inconsistent Koli within Receipt ID %s: row %d has '%s', expected '%s' (from row %d)",
					group.ReceiptID, row.Row, koliString(row.Koli), koliString(ref.Koli), ref.Row,
				),
			})
		}
	}

	return errs
}

// getFieldValue mengambil nilai field header (bertipe string) dari
// ExcelInboundRow berdasarkan nama field, dipakai untuk perbandingan
// konsistensi secara generic tanpa menulis if-else berulang untuk
// setiap field satu per satu.
func getFieldValue(row ExcelInboundRow, field string) string {
	switch field {
	case "InboundDate":
		return row.InboundDate
	case "Type":
		return row.Type
	case "Supplier":
		return row.Supplier
	case "Transporter":
		return row.Transporter
	case "Driver":
		return row.Driver
	case "WhsCode":
		return row.WhsCode
	case "OwnerCode":
		return row.OwnerCode
	case "Origin":
		return row.Origin
	case "PoDate":
		return row.PoDate
	case "NoTruck":
		return row.NoTruck
	case "Container":
		return row.Container
	case "TruckSize":
		return row.TruckSize
	case "ArrivalTime":
		return row.ArrivalTime
	case "StartUnloading":
		return row.StartUnloading
	case "EndUnloading":
		return row.EndUnloading
	case "BLNo":
		return row.BLNo
	case "Remarks":
		return row.Remarks
	default:
		return ""
	}
}

func koliEqual(a, b int) bool {
	if a == 0 && b == 0 {
		return true
	}
	if a == 0 || b == 0 {
		return false
	}
	return a == b
}

func koliString(v int) string {
	if v == 0 {
		return "(empty)"
	}
	return strconv.Itoa(v)
}

// =======================================
// DUPLICATE CHECK (sama seperti logic lama, disesuaikan ke struct baru)
// =======================================

func checkDuplicateItems(rows []ExcelInboundRow) []ValidationError {
	var errs []ValidationError
	itemMap := make(map[string]int)

	for _, row := range rows {
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
			row.ReceiptID, row.ItemCode, row.RecDate, row.ExpDate,
			row.LotNumber, row.ProdDate, row.Location,
			row.UOM, row.QaStatus)

		if existingRow, exists := itemMap[key]; exists {
			errs = append(errs, ValidationError{
				Field: "Duplicate",
				Message: fmt.Sprintf(
					"Duplicate item found (same as row %d) within Receipt ID %s: %s",
					existingRow, row.ReceiptID, row.ItemCode),
				Row: row.Row,
			})
		} else {
			itemMap[key] = row.Row
		}
	}

	return errs
}

// =======================================
// MAIN CONTROLLER (REVISED)
// =======================================

func (c *InboundController) CreateInboundFromExcelFile(ctx *fiber.Ctx) error {
	// ---------- Parse uploaded file ----------
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Error", Detail: err.Error()},
			},
		})
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Processing Error", Detail: err.Error()},
			},
		})
	}
	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Excel Read Error", Detail: err.Error()},
			},
		})
	}
	defer excelFile.Close()

	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Excel file contains no sheets",
		})
	}

	sheetName := sheets[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Sheet Read Error", Detail: err.Error()},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Excel file must contain at least header row and one data row",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	// ---------- Validasi inventory policy ----------
	// NOTE: policy diambil berdasarkan OwnerCode di baris pertama data,
	// dengan asumsi satu file = satu Owner Code. Kalau Owner Code juga
	// bisa berbeda antar grup dalam satu file, ini perlu diperluas
	// menjadi pencarian per-grup. Untuk saat ini OwnerCode termasuk
	// field yang divalidasi konsisten di seluruh file lewat
	// validateGroupHeaderConsistency, jadi aman selama semua baris
	// dalam SATU ReceiptID owner-nya sama. Jika owner BERBEDA antar
	// ReceiptID yang berbeda, policy lookup di bawah perlu disesuaikan.
	firstOwnerCode := strings.TrimSpace(getCell(rows[1], colOwnerCode))
	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", firstOwnerCode).First(&inventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy for owner: " + firstOwnerCode,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Inventory Policy Error", Detail: err.Error()},
			},
		})
	}

	// ---------- Parse semua baris (header + detail digabung) ----------
	parsedRows, parseErrors := c.parseAllRows(rows, inventoryPolicy)
	if len(parseErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d errors", len(parseErrors)),
			ValidationErrors: parseErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	if len(parsedRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "No valid data rows found in Excel file",
		})
	}

	// ---------- Group by ReceiptID ----------
	groups := groupRowsByReceiptID(parsedRows)

	// ---------- Validasi konsistensi header per grup ----------
	var consistencyErrors []ValidationError
	for _, group := range groups {
		consistencyErrors = append(consistencyErrors, validateGroupHeaderConsistency(group)...)
	}
	if len(consistencyErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Header consistency validation failed with %d errors", len(consistencyErrors)),
			ValidationErrors: consistencyErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// ---------- Validasi duplikat item (dalam masing-masing ReceiptID) ----------
	duplicateErrors := checkDuplicateItems(parsedRows)
	if len(duplicateErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          "Duplicate items found in Excel file",
			ValidationErrors: duplicateErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// ---------- Validasi referensi master data per grup (Warehouse, Supplier, Origin) ----------
	// Dilakukan sebelum transaction dimulai supaya fail-fast.
	for _, group := range groups {
		h := group.HeaderRow

		var warehouse models.Warehouse
		if err := c.DB.Where("code = ?", h.WhsCode).First(&warehouse).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get warehouse: " + h.WhsCode,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Warehouse Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := c.DB.Where("supplier_code = ?", h.Supplier).First(&supplier).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get supplier: " + h.Supplier,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Supplier Error", Detail: err.Error()},
				},
			})
		}

		var origin models.Origin
		if err := c.DB.Where("country = ?", h.Origin).First(&origin).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get origin: " + h.Origin,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Origin Error", Detail: err.Error()},
				},
			})
		}

		var transporter models.Transporter
		if err := c.DB.Where("transporter_code = ?", h.Transporter).First(&transporter).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to get transporter: " + h.Transporter,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Transporter Error", Detail: err.Error()},
				},
			})
		}

		// Cek existing ReceiptID di DB sebelum transaction dimulai (fail-fast,
		// menghindari rollback besar di tengah jalan hanya karena receipt
		// yang sama pernah di-upload sebelumnya).
		var existingCount int64
		if err := c.DB.Model(&models.InboundHeader{}).
			Where("receipt_id = ?", h.ReceiptID).
			Count(&existingCount).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to check existing Receipt ID: " + h.ReceiptID,
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}
		if existingCount > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Receipt ID already exists: " + h.ReceiptID,
				ValidationErrors: []ValidationError{
					{Field: "ReceiptID", Message: "Receipt ID already exists in database: " + h.ReceiptID, Row: h.Row},
				},
			})
		}
	}

	// ---------- Mulai transaction (ALL-OR-NOTHING untuk seluruh file) ----------
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateInboundFromExcelFile: %v", r)
		}
	}()

	repo := repositories.NewInboundRepository(tx)

	var createdInbounds []string
	successCount := 0

	// Iterasi grup dengan urutan STABIL (sesuai kemunculan pertama di file),
	// bukan urutan acak map.
	for _, group := range groups {
		h := group.HeaderRow

		inboundNo, err := repo.GenerateInboundNo()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate inbound number for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Inbound Generation Error", Detail: err.Error()},
				},
			})
		}

		var supplier models.Supplier
		if err := tx.First(&supplier, "supplier_code = ?", h.Supplier).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to validate supplier for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Error", Detail: err.Error()},
				},
			})
		}

		inboundHeader := models.InboundHeader{
			InboundNo:      inboundNo,
			InboundDate:    h.InboundDate,
			ReceiptID:      h.ReceiptID,
			Supplier:       h.Supplier,
			SupplierId:     int(supplier.ID),
			Status:         "open",
			RawStatus:      "DRAFT",
			DraftTime:      time.Now(),
			Transporter:    h.Transporter,
			NoTruck:        h.NoTruck,
			Driver:         h.Driver,
			Container:      h.Container,
			Remarks:        h.Remarks,
			Type:           h.Type,
			WhsCode:        h.WhsCode,
			OwnerCode:      h.OwnerCode,
			Origin:         h.Origin,
			PoDate:         h.PoDate,
			ArrivalTime:    h.ArrivalTime,
			StartUnloading: h.StartUnloading,
			EndUnloading:   h.EndUnloading,
			TruckSize:      h.TruckSize,
			BLNo:           h.BLNo,
			Koli:           h.Koli,
			CreatedBy:      userID,
			UpdatedBy:      userID,
		}

		if err := tx.Create(&inboundHeader).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound header for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		// RefNo = ReceiptID (disepakati keduanya konsep yang sama)
		inboundReference := models.InboundReference{
			InboundId: uint(inboundHeader.ID),
			RefNo:     h.ReceiptID,
		}
		if err := tx.Create(&inboundReference).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to create inbound reference for Receipt ID %s", h.ReceiptID),
				Errors: []ExcelRowError{
					{Row: h.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		for _, detail := range group.Details {
			var product models.Product
			if err := tx.First(&product, "item_code = ? AND owner_code = ?", detail.ItemCode, h.OwnerCode).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Product not found for item code: %s (Receipt ID %s)", detail.ItemCode, h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Product Not Found", Detail: "Item code: " + detail.ItemCode},
					},
				})
			}

			var uomConversion models.UomConversion
			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("UOM conversion not found (Receipt ID %s)", h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "UOM Not Found", Detail: fmt.Sprintf("Item: %s, UOM: %s", detail.ItemCode, detail.UOM)},
					},
				})
			}

			if detail.QaStatus != "" {
				var qaStatus models.QaStatus
				if err := tx.First(&qaStatus, "qa_status = ?", detail.QaStatus).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
						Success: false,
						Message: fmt.Sprintf("QA status not found (Receipt ID %s)", h.ReceiptID),
						Errors: []ExcelRowError{
							{Row: detail.Row, Message: "QA Status Not Found", Detail: "Status: " + detail.QaStatus},
						},
					})
				}
			}

			inboundDetail := models.InboundDetail{
				InboundNo:     inboundNo,
				InboundId:     int(inboundHeader.ID),
				ItemCode:      detail.ItemCode,
				ItemId:        product.ID,
				ProductNumber: product.ProductNumber,
				Barcode:       uomConversion.Ean,
				Uom:           detail.UOM,
				Quantity:      detail.Quantity,
				RcvLocation:   detail.Location,
				Location:      detail.Location,
				QaStatus:      detail.QaStatus,
				RecDate:       detail.RecDate,
				ProdDate:      detail.ProdDate,
				ExpDate:       detail.ExpDate,
				LotNumber:     detail.LotNumber,
				IsSerial:      product.HasSerial,
				SN:            product.HasSerial,
				RefId:         int(inboundReference.ID),
				RefNo:         h.ReceiptID,
				OwnerCode:     h.OwnerCode,
				WhsCode:       h.WhsCode,
				DivisionCode:  detail.Division,
				CreatedBy:     userID,
				UpdatedBy:     userID,
			}

			if err := tx.Create(&inboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to create inbound detail (Receipt ID %s)", h.ReceiptID),
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
					},
				})
			}

			successCount++
		}

		if err := helpers.InsertTransactionHistory(tx, inboundNo, "open", "INBOUND", "Created from Excel upload", userID); err != nil {
			log.Printf("Warning: Failed to insert transaction history for %s: %v", inboundNo, err)
		}

		createdInbounds = append(createdInbounds, inboundNo)
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()},
			},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(ExcelUploadResponse{
		Success:        true,
		Message:        fmt.Sprintf("Successfully created %d inbound(s) with %d items", len(createdInbounds), successCount),
		TotalRows:      len(parsedRows),
		SuccessCount:   successCount,
		FailedCount:    0,
		InboundNumbers: createdInbounds,
	})
}

// =======================================
// END IMPORT FROM EXCEL FILE
// =======================================
