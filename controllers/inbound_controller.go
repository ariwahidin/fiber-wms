package controllers

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
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
	ID        int     `json:"ID"`
	InboundID int     `json:"inbound_id"`
	ItemCode  string  `json:"item_code"`
	Quantity  float64 `json:"quantity"`
	QaStatus  string  `json:"qa_status"`
	Location  string  `json:"location"`
	WhsCode   string  `json:"whs_code"`
	UOM       string  `json:"uom"`
	RecDate   string  `json:"rec_date"`
	ProdDate  string  `json:"prod_date"`
	ExpDate   string  `json:"exp_date"`
	LotNumber string  `json:"lot_number"`
	Remarks   string  `json:"remarks"`
	IsSerial  string  `json:"is_serial"`
	Mode      string  `json:"mode"`
	RefId     int     `json:"ref_id"`
	RefNo     string  `json:"ref_no"`
	Division  string  `json:"division"`
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

	fmt.Println("PAYLOAD", payload)

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

		if InventoryPolicy.UseLotNo {
			if item.LotNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number cannot be empty",
					"error":   "Lot number cannot be empty",
				})
			}
		}

		if InventoryPolicy.UseProductionDate {
			if item.ProdDate == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Production date cannot be empty",
					"error":   "Production date cannot be empty",
				})
			}
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

		if InventoryPolicy.RequireExpiryDate {
			if item.ExpDate == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Expiration date cannot be empty",
					"error":   "Expiration date cannot be empty",
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

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus),
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

	repositories := repositories.NewInboundRepository(tx)

	inbound_no, err := repositories.GenerateInboundNo()
	if err != nil {
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

		InboundDetail.InboundNo = payload.InboundNo
		InboundDetail.InboundId = int(inboundID)
		InboundDetail.ItemCode = item.ItemCode
		InboundDetail.ItemId = product.ID
		InboundDetail.ProductNumber = product.ProductNumber
		InboundDetail.Barcode = uomConversion.Ean
		InboundDetail.Uom = item.UOM
		InboundDetail.Quantity = item.Quantity
		InboundDetail.Location = item.Location
		InboundDetail.QaStatus = item.QaStatus
		InboundDetail.WhsCode = item.WhsCode
		InboundDetail.RecDate = item.RecDate
		InboundDetail.ProdDate = item.ProdDate
		InboundDetail.ExpDate = item.ExpDate
		InboundDetail.LotNumber = item.LotNumber
		// InboundDetail.Remarks = item.Remarks
		InboundDetail.IsSerial = product.HasSerial
		InboundDetail.RefId = int(InboundReference.ID)
		InboundDetail.RefNo = item.RefNo
		InboundDetail.OwnerCode = payload.OwnerCode
		InboundDetail.WhsCode = payload.WhsCode
		// InboundDetail.DivisionCode = item.Division
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

		if InventoryPolicy.UseLotNo {
			if item.LotNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number cannot be empty",
					"error":   "Lot number cannot be empty",
				})
			}
		}

		if InventoryPolicy.RequireExpiryDate {
			if item.ExpDate == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Expiration date cannot be empty",
					"error":   "Expiration date cannot be empty",
				})
			}
		}

		if InventoryPolicy.UseProductionDate {
			if item.ProdDate == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Production date cannot be empty",
					"error":   "Production date cannot be empty",
				})
			}
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

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus),
			})
		}

		itemCodes[key] = true

	}

	payloadItem := payload.Items

	userID := int(ctx.Locals("userID").(float64))
	var InboundHeader models.InboundHeader
	if err := c.DB.Debug().First(&InboundHeader, "inbound_no = ?", inbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var supplier models.Supplier
	if err := c.DB.Debug().First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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

	if err := c.DB.Model(&models.InboundHeader{}).Where("id = ?", InboundHeader.ID).Updates(InboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var message string

	if InboundHeader.Status == "complete" {
		message = "Inbound " + inbound_no + " is already complete"
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message, "message": message})
	}

	if InboundHeader.Status == "open" {

		for _, item := range payload.References {

			var InboundReference models.InboundReference
			if err := c.DB.Debug().First(&InboundReference, "id = ?", item.ID).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			if InboundReference.ID == 0 {
				InboundReference.InboundId = uint(InboundHeader.ID)
				InboundReference.RefNo = item.RefNo
				if err := c.DB.Create(&InboundReference).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else {
				InboundReference.RefNo = item.RefNo
				if err := c.DB.Model(&models.InboundReference{}).Where("id = ?", InboundReference.ID).Updates(InboundReference).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

		for _, item := range payloadItem {
			var inboundDetail models.InboundDetail

			var product models.Product
			if err := c.DB.Debug().First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			var uomConversion models.UomConversion
			if err := c.DB.Debug().First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, item.UOM).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found"})
				}
				// c.DB.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Coba cari berdasarkan ID
			err := c.DB.Debug().First(&inboundDetail, "id = ?", item.ID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// ❌ Tidak ditemukan → insert baru
				newDetail := models.InboundDetail{
					InboundId:     int(InboundHeader.ID),
					InboundNo:     InboundHeader.InboundNo,
					ItemId:        product.ID,
					ProductNumber: product.ProductNumber,
					ItemCode:      item.ItemCode,
					Barcode:       uomConversion.Ean,
					Quantity:      item.Quantity,
					Location:      item.Location,
					WhsCode:       InboundHeader.WhsCode,
					RecDate:       item.RecDate,
					ProdDate:      item.ProdDate,
					ExpDate:       item.ExpDate,
					LotNumber:     item.LotNumber,
					Uom:           item.UOM,
					IsSerial:      product.HasSerial,
					RefNo:         item.RefNo,
					RefId:         item.RefId,
					OwnerCode:     InboundHeader.OwnerCode,
					// DivisionCode:  item.Division,
					QaStatus:  item.QaStatus,
					CreatedBy: int(ctx.Locals("userID").(float64)),
				}
				if err := c.DB.Create(&newDetail).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else if err == nil {
				// ✅ Ditemukan → update
				inboundDetail.ItemId = product.ID
				inboundDetail.ItemCode = item.ItemCode
				inboundDetail.Barcode = uomConversion.Ean
				inboundDetail.Quantity = item.Quantity
				inboundDetail.Location = item.Location
				inboundDetail.WhsCode = InboundHeader.WhsCode
				inboundDetail.RecDate = item.RecDate
				inboundDetail.ProdDate = item.ProdDate
				inboundDetail.ExpDate = item.ExpDate
				inboundDetail.LotNumber = item.LotNumber
				inboundDetail.Uom = item.UOM
				inboundDetail.IsSerial = product.HasSerial
				inboundDetail.RefNo = item.RefNo
				inboundDetail.RefId = item.RefId
				inboundDetail.OwnerCode = InboundHeader.OwnerCode
				// inboundDetail.DivisionCode = item.Division
				inboundDetail.QaStatus = item.QaStatus
				inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))

				if err := c.DB.Save(&inboundDetail).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else {
				// ❌ Error lain
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}

		message = "Update Inbound " + InboundHeader.InboundNo + " successfully"
	} else {
		message = "Update Inbound Header " + InboundHeader.InboundNo + " successfully"
	}

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

func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")
	limit := ctx.QueryInt("limit", 5000)

	// if limit > 5000 {
	// 	limit = 5000
	// }

	var inbound models.InboundHeader
	fmt.Println("GetInboundByID inbound_no:", inbound_no)

	// Hitung total detail jika dibutuhkan
	var totalDetails int64
	c.DB.Model(&models.InboundDetail{}).
		Where("inbound_no = ?", inbound_no).
		Count(&totalDetails)

	// Load data dengan preload terbatas
	if err := c.DB.Debug().
		Preload("InboundReferences").
		Preload("Received").
		Preload("Received.Product").
		Preload("Details", func(db *gorm.DB) *gorm.DB {
			return db.Limit(limit).Order("id ASC")
		}).
		First(&inbound, "inbound_no = ?", inbound_no).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Tambahkan informasi total detail
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"data":          inbound,
		"details_limit": limit,
		"details_total": totalDetails,
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

	var inboundBarcodesCheck01 []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundBarcodesCheck01).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var invPolicy models.InventoryPolicy
	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodesCheck01) == 0 {
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
				newInboundBarcode := models.InboundBarcode{
					InboundId:       int(inboundHeader.ID),
					InboundDetailId: int(detail.ID),
					ItemCode:        detail.ItemCode,
					ItemID:          detail.ItemId,
					ScanData:        detail.Barcode,
					Barcode:         detail.Barcode,
					SerialNumber:    detail.Barcode,
					Pallet:          detail.Location,
					Location:        detail.Location,
					Quantity:        detail.Quantity,
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
		return errors.New("ID cannot be empty")
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

	inboundHeaderID := inboundHeader.ID

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return errors.New("Invalid ID")
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	_, errs := inboundRepo.ProcessPutawayItem(ctx, id, "")
	if errs != nil {
		fmt.Println("ADA NIH", errs.Error())
		return errs
	}

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
	if err := c.DB.Raw(sqlCheck, inboundHeaderID, inboundHeaderID).Scan(&checkResult).Error; err != nil {
		return errors.New(err.Error())
	}

	qtyRequest := 0
	qtyReceived := 0
	for _, result := range checkResult {
		qtyRequest += result.Quantity
		qtyReceived += result.QtyScan
	}

	statusInbound := "fully received"
	if qtyRequest != qtyReceived {
		statusInbound = "partially received"
	}

	userID := int(ctx.Locals("userID").(float64))

	now := time.Now()
	updateData := models.InboundHeader{
		Status:    statusInbound,
		PutawayAt: &now,
		PutawayBy: userID,
	}
	if err := c.DB.Debug().Model(&models.InboundHeader{}).
		Where("id = ?", inboundHeaderID).
		Updates(updateData).Error; err != nil {
		return errors.New(err.Error())
	}

	errHistory := helpers.InsertTransactionHistory(
		c.DB,
		inboundHeader.InboundNo,
		statusInbound,
		"INBOUND",
		"",
		userID,
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
		return errors.New(errHistory.Error())
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

				"error":   fmt.Sprintf("Failed putaway ID %s: %v", id, errPutaway),
				"message": fmt.Sprintf("Failed putaway ID %s: %v", id, errPutaway),
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

	userID := int(ctx.Locals("userID").(float64))
	now := time.Now()

	err := r.DB.Model(&InboundHeader).Updates(map[string]interface{}{
		"status":       "checking",
		"raw_status":   "CONFIRMED",
		"confirm_time": now,
		"confirm_by":   userID,
		"updated_at":   now,
		"updated_by":   userID,
		"checking_at":  now,
		"checking_by":  userID,
	}).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// sqlUpdate := `UPDATE inbound_headers SET status = 'checking', updated_at = ?, updated_by = ?, checking_at = ?, checking_by = ? WHERE inbound_no = ?`
	// if err := r.DB.Exec(sqlUpdate, time.Now(), userID, time.Now(), userID, payload.InboundNo).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	errHistory := helpers.InsertTransactionHistory(
		r.DB,
		payload.InboundNo, // RefNo
		"checking",        // Status
		"INBOUND",         // Type
		"",                // Detail
		userID,            // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
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

	type CheckResultInbound struct {
		InboundNo       string `json:"inbound_no"`
		InboundDetailId int    `json:"inbound_detail_id"`
		ItemId          int    `json:"item_id"`
		ItemCode        string `json:"item_code"`
		Quantity        int    `json:"quantity"`
		QtyScan         int    `json:"qty_scan"`
	}

	sqlCheck := `WITH ib AS
	(
		SELECT inbound_id, inbound_detail_id, item_id, SUM(quantity) AS qty_scan, status 
		FROM inbound_barcodes WHERE inbound_id = ?
		GROUP BY inbound_id, inbound_detail_id, item_id, status
	)

	SELECT a.id, a.inbound_no, a.inbound_id, a.item_id, a.quantity, COALESCE(ib.qty_scan, 0) AS qty_scan, a.item_code
	FROM inbound_details a
	LEFT JOIN ib ON a.id = ib.inbound_detail_id
	WHERE a.inbound_id = ?`

	var checkResult []CheckResultInbound
	if err := r.DB.Raw(sqlCheck, InboundHeader.ID, InboundHeader.ID).Scan(&checkResult).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, result := range checkResult {
		if result.QtyScan > 0 {
			message := "Inbound " + payload.InboundNo + ", item " + result.ItemCode + " has been scanned"
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message, "message": "Cannot change status to open, inbound " + payload.InboundNo + ", item " + result.ItemCode + " has been scanned"})
		}
	}

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
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Putaway not complete", "message": "Inbound " + result.InboundNo + " not all putaway completed"})
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
// BEGIN IMPORT FROM EXCEL FILE
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

type ExcelInboundHeader struct {
	ReceiptID      string
	InboundDate    string
	Supplier       string
	Transporter    string
	NoTruck        string
	Driver         string
	Container      string
	Remarks        string
	Type           string
	WhsCode        string
	OwnerCode      string
	Origin         string
	PoDate         string
	ArrivalTime    string
	StartUnloading string
	EndUnloading   string
	TruckSize      string
	BLNo           string
	Koli           string
}

type ExcelInboundDetail struct {
	RefNo     string
	ItemCode  string
	UOM       string
	Quantity  float64
	Location  string
	QaStatus  string
	WhsCode   string
	RecDate   string
	ProdDate  string
	ExpDate   string
	LotNumber string
	Division  string
}

func (c *InboundController) CreateInboundFromExcelFile(ctx *fiber.Ctx) error {
	// Parse uploaded file
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

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// Open uploaded file
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

	// Read Excel file
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

	// Get first sheet
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

	// Parse header information (assuming header info is in first row)
	headerInfo, err := c.parseHeaderFromExcel(rows)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to parse header information",
			ValidationErrors: []ValidationError{
				{Field: "Header", Message: err.Error(), Row: 1},
			},
		})
	}

	// Get user ID
	userID := int(ctx.Locals("userID").(float64))

	// Validate inventory policy
	var inventoryPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", headerInfo.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy for owner: " + headerInfo.OwnerCode,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Inventory Policy Error", Detail: err.Error()},
			},
		})
	}

	// Validate Warehouse
	var warehouse models.Warehouse
	if err := c.DB.Where("code = ?", headerInfo.WhsCode).First(&warehouse).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get warehouse: " + headerInfo.WhsCode,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Warehouse Error", Detail: err.Error()},
			},
		})
	}

	// Validate Supplier
	var supplier models.Supplier
	if err := c.DB.Where("supplier_code = ?", headerInfo.Supplier).First(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get supplier: " + headerInfo.Supplier,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Supplier Error", Detail: err.Error()},
			},
		})
	}

	// Validate Origin
	var origin models.Origin
	if err := c.DB.Where("country = ?", headerInfo.Origin).First(&origin).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
			Success: false,
			Message: "Failed to get origin: " + headerInfo.Origin,
			Errors: []ExcelRowError{
				{Row: 1, Message: "Origin Error", Detail: err.Error()},
			},
		})
	}

	// Parse detail rows (starting from row 2, assuming row 1 is header)
	details, validationErrors := c.parseDetailsFromExcel(rows, inventoryPolicy)
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d errors", len(validationErrors)),
			ValidationErrors: validationErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// Check for duplicate items
	duplicateErrors := c.checkDuplicateItems(details)
	if len(duplicateErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelUploadResponse{
			Success:          false,
			Message:          "Duplicate items found in Excel file",
			ValidationErrors: duplicateErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// Group by references for creating multiple inbounds if needed
	groupedDetails := c.groupDetailsByReference(details)

	// Start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateInboundFromExcelFile: %v", r)
		}
	}()

	repositories := repositories.NewInboundRepository(tx)

	var createdInbounds []string
	var processErrors []ExcelRowError
	successCount := 0

	// Create inbound(s)
	for refNo, detailGroup := range groupedDetails {
		inboundNo, err := repositories.GenerateInboundNo()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to generate inbound number",
				Errors: []ExcelRowError{
					{Row: 0, Message: "Inbound Generation Error", Detail: err.Error()},
				},
			})
		}

		// Validate supplier
		var supplier models.Supplier
		if err := tx.First(&supplier, "supplier_code = ?", headerInfo.Supplier).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
					Success: false,
					Message: "Supplier not found: " + headerInfo.Supplier,
					Errors: []ExcelRowError{
						{Row: 1, Message: "Supplier Not Found", Detail: "Supplier code: " + headerInfo.Supplier},
					},
				})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to validate supplier",
				Errors: []ExcelRowError{
					{Row: 1, Message: "Database Error", Detail: err.Error()},
				},
			})
		}

		// Create inbound header
		inboundHeader := models.InboundHeader{
			InboundNo:   inboundNo,
			InboundDate: headerInfo.InboundDate,
			ReceiptID:   headerInfo.ReceiptID,
			Supplier:    headerInfo.Supplier,
			SupplierId:  int(supplier.ID),
			Status:      "open",
			RawStatus:   "DRAFT",
			DraftTime:   time.Now(),
			Transporter: headerInfo.Transporter,
			NoTruck:     headerInfo.NoTruck,
			Driver:      headerInfo.Driver,
			Container:   headerInfo.Container,
			Remarks:     headerInfo.Remarks,
			// Type:           headerInfo.Type,
			Type:           "NORMAL",
			WhsCode:        headerInfo.WhsCode,
			OwnerCode:      headerInfo.OwnerCode,
			Origin:         headerInfo.Origin,
			PoDate:         headerInfo.PoDate,
			ArrivalTime:    headerInfo.ArrivalTime,
			StartUnloading: headerInfo.StartUnloading,
			EndUnloading:   headerInfo.EndUnloading,
			TruckSize:      headerInfo.TruckSize,
			BLNo:           headerInfo.BLNo,
			// Koli:           headerInfo.Koli,
			CreatedBy: userID,
			UpdatedBy: userID,
		}

		if err := tx.Create(&inboundHeader).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to create inbound header",
				Errors: []ExcelRowError{
					{Row: 1, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		// Create inbound reference
		inboundReference := models.InboundReference{
			InboundId: uint(inboundHeader.ID),
			RefNo:     refNo,
		}

		if err := tx.Create(&inboundReference).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
				Success: false,
				Message: "Failed to create inbound reference",
				Errors: []ExcelRowError{
					{Row: 1, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		// Create inbound details
		for _, detail := range detailGroup {
			// Validate product
			var product models.Product
			if err := tx.First(&product, "item_code = ? AND owner_code = ?", detail.ItemCode, headerInfo.OwnerCode).Error; err != nil {
				tx.Rollback()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
						Success: false,
						Message: "Product not found for item code: " + detail.ItemCode,
						Errors: []ExcelRowError{
							{Row: detail.Row, Message: "Product Not Found", Detail: "Item code: " + detail.ItemCode},
						},
					})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: "Failed to validate product",
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Error", Detail: err.Error()},
					},
				})
			}

			// Validate UOM conversion
			var uomConversion models.UomConversion
			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
				tx.Rollback()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
						Success: false,
						Message: "UOM conversion not found",
						Errors: []ExcelRowError{
							{Row: detail.Row, Message: "UOM Not Found", Detail: fmt.Sprintf("Item: %s, UOM: %s", detail.ItemCode, detail.UOM)},
						},
					})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: "Failed to validate UOM",
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Error", Detail: err.Error()},
					},
				})
			}

			// Validate QA status
			var qaStatus models.QaStatus
			if err := tx.First(&qaStatus, "qa_status = ?", detail.QaStatus).Error; err != nil {
				tx.Rollback()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(ExcelUploadResponse{
						Success: false,
						Message: "QA status not found",
						Errors: []ExcelRowError{
							{Row: detail.Row, Message: "QA Status Not Found", Detail: "Status: " + detail.QaStatus},
						},
					})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: "Failed to validate QA status",
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Error", Detail: err.Error()},
					},
				})
			}

			// Create inbound detail
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
				RefNo:         detail.RefNo,
				OwnerCode:     headerInfo.OwnerCode,
				WhsCode:       headerInfo.WhsCode,
				DivisionCode:  detail.Division,
				CreatedBy:     userID,
				UpdatedBy:     userID,
			}

			if err := tx.Create(&inboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelUploadResponse{
					Success: false,
					Message: "Failed to create inbound detail",
					Errors: []ExcelRowError{
						{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
					},
				})
			}

			successCount++
		}

		// Insert transaction history
		if err := helpers.InsertTransactionHistory(tx, inboundNo, "open", "INBOUND", "Created from Excel upload", userID); err != nil {
			log.Printf("Warning: Failed to insert transaction history for %s: %v", inboundNo, err)
			// Don't rollback for history error, just log it
		}

		createdInbounds = append(createdInbounds, inboundNo)
	}

	// Commit transaction
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
		TotalRows:      len(details),
		SuccessCount:   successCount,
		FailedCount:    len(processErrors),
		InboundNumbers: createdInbounds,
		Errors:         processErrors,
	})
}

// Helper functions
func (c *InboundController) parseHeaderFromExcel(rows [][]string) (*ExcelInboundHeader, error) {
	if len(rows) < 1 {
		return nil, errors.New("no header row found")
	}

	// Assuming header info is in specific columns or you can parse from first data row
	// Adjust this logic based on your Excel template structure
	header := &ExcelInboundHeader{}

	// Example: parse from first data row (row index 1)
	if len(rows) > 1 && len(rows[1]) > 0 {
		// Map columns - adjust indices based on your template
		header.ReceiptID = getCell(rows[1], 0)
		header.InboundDate = getCell(rows[1], 1)
		header.Supplier = getCell(rows[1], 2)
		header.WhsCode = getCell(rows[1], 3)
		header.OwnerCode = getCell(rows[1], 4)
		header.Origin = getCell(rows[1], 5)
		// ... map other fields
	}

	// Validate required fields
	if header.ReceiptID == "" {
		return nil, errors.New("receipt ID is required")
	}
	if header.OwnerCode == "" {
		return nil, errors.New("owner code is required")
	}

	return header, nil
}

func (c *InboundController) parseDetailsFromExcel(rows [][]string, policy models.InventoryPolicy) ([]struct {
	ExcelInboundDetail
	Row int
}, []ValidationError) {
	var details []struct {
		ExcelInboundDetail
		Row int
	}
	var errors []ValidationError

	// Start from row 2 (index 1), assuming row 1 is header
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		detail := struct {
			ExcelInboundDetail
			Row int
		}{Row: rowNum}

		// Parse columns - adjust indices based on your template
		detail.RefNo = strings.TrimSpace(getCell(row, 0))
		detail.ItemCode = strings.TrimSpace(getCell(row, 6))
		detail.UOM = strings.TrimSpace(getCell(row, 7))

		qtyStr := strings.TrimSpace(getCell(row, 8))
		if qtyStr != "" {
			qty, err := strconv.ParseFloat(qtyStr, 64)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   "Quantity",
					Message: "Invalid quantity format: " + qtyStr,
					Row:     rowNum,
				})
				continue
			}
			detail.Quantity = qty
		}

		RecDate, err := getCellAsDateStrict(row, 11)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "RecDate",
				Message: "Invalid RecDate format: " + RecDate,
				Row:     rowNum,
			})
			continue
		}

		ProdDate, err := getCellAsDateStrict(row, 12)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "ProdDate",
				Message: "Invalid ProdDate format: " + ProdDate,
				Row:     rowNum,
			})
			continue
		}

		ExpDate, err := getCellAsDateStrict(row, 13)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "ExpDate",
				Message: "Invalid ExpDate format: " + ExpDate,
				Row:     rowNum,
			})
			continue
		}

		detail.Location = strings.TrimSpace(getCell(row, 9))
		detail.QaStatus = strings.TrimSpace(getCell(row, 10))
		// detail.RecDate = strings.TrimSpace(getCell(row, 11))
		// detail.ProdDate = strings.TrimSpace(getCell(row, 12))
		// detail.ExpDate = strings.TrimSpace(getCell(row, 13))
		detail.RecDate = RecDate
		detail.ProdDate = ProdDate
		detail.ExpDate = ExpDate
		detail.LotNumber = strings.TrimSpace(getCell(row, 14))
		detail.Division = strings.TrimSpace(getCell(row, 15))

		// Validate required fields
		if detail.ItemCode == "" {
			errors = append(errors, ValidationError{
				Field:   "ItemCode",
				Message: "Item code cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		if detail.UOM == "" {
			errors = append(errors, ValidationError{
				Field:   "UOM",
				Message: "UOM cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		if detail.Quantity == 0 {
			errors = append(errors, ValidationError{
				Field:   "Quantity",
				Message: "Quantity cannot be zero",
				Row:     rowNum,
			})
			continue
		}

		if detail.RefNo == "" {
			errors = append(errors, ValidationError{
				Field:   "RefNo",
				Message: "Reference number cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		// Validate based on inventory policy
		if policy.UseLotNo && detail.LotNumber == "" {
			errors = append(errors, ValidationError{
				Field:   "LotNumber",
				Message: "Lot number is required by inventory policy",
				Row:     rowNum,
			})
			continue
		}

		if policy.UseProductionDate && detail.ProdDate == "" {
			errors = append(errors, ValidationError{
				Field:   "ProdDate",
				Message: "Production date is required by inventory policy",
				Row:     rowNum,
			})
			continue
		}

		if policy.UseReceiveLocation && detail.Location == "" {
			errors = append(errors, ValidationError{
				Field:   "Location",
				Message: "Receive location is required by inventory policy",
				Row:     rowNum,
			})
			continue
		}

		if policy.UseFEFO && detail.ExpDate == "" {
			errors = append(errors, ValidationError{
				Field:   "ExpDate",
				Message: "Expiration date is required by inventory policy",
				Row:     rowNum,
			})
			continue
		}

		details = append(details, detail)
	}

	return details, errors
}

func (c *InboundController) checkDuplicateItems(details []struct {
	ExcelInboundDetail
	Row int
}) []ValidationError {
	var errors []ValidationError
	itemMap := make(map[string]int)

	for _, detail := range details {
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
			detail.ItemCode, detail.RecDate, detail.ExpDate,
			detail.LotNumber, detail.ProdDate, detail.Location,
			detail.UOM, detail.QaStatus)

		if existingRow, exists := itemMap[key]; exists {
			errors = append(errors, ValidationError{
				Field: "Duplicate",
				Message: fmt.Sprintf("Duplicate item found (same as row %d): %s",
					existingRow, detail.ItemCode),
				Row: detail.Row,
			})
		} else {
			itemMap[key] = detail.Row
		}
	}

	return errors
}

func (c *InboundController) groupDetailsByReference(details []struct {
	ExcelInboundDetail
	Row int
}) map[string][]struct {
	ExcelInboundDetail
	Row int
} {
	grouped := make(map[string][]struct {
		ExcelInboundDetail
		Row int
	})

	for _, detail := range details {
		grouped[detail.RefNo] = append(grouped[detail.RefNo], detail)
	}

	return grouped
}

func getCell(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

// func getDateCell(f *excelize.File, sheet string, axis string) string {
//     cellValue, err := f.GetCellValue(sheet, axis)
//     if err != nil || cellValue == "" {
//         return ""
//     }

//     // Coba parse sebagai angka (Excel serial date)
//     if days, err := strconv.ParseFloat(cellValue, 64); err == nil {
//         // Excel menyimpan date sebagai angka sejak 1 Jan 1900
//         // Tapi ada bug di Excel, jadi pakai 30 Dec 1899
//         excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
//         date := excelEpoch.Add(time.Duration(days * 24 * float64(time.Hour)))
//         return date.Format("2006-01-02")
//     }

//     // Jika bukan angka, coba parse sebagai string date
//     dateFormats := []string{
//         "2006-01-02",
//         "02/01/2006",
//         "01/02/2006",
//         "2/1/2006",
//         "1/2/2006",
//         "2006/01/02",
//         "02-01-2006",
//         "01-02-2006",
//         "2-Jan-06",
//         "2-January-2006",
//     }

//     for _, format := range dateFormats {
//         if t, err := time.Parse(format, cellValue); err == nil {
//             return t.Format("2006-01-02")
//         }
//     }

//     // Kalau semua gagal, return string original
//     return cellValue
// }

// Tambahkan fungsi baru untuk parse date
// func getCellAsDate(row []string, index int) string {
// 	cellValue := strings.TrimSpace(getCell(row, index))
// 	if cellValue == "" {
// 		return ""
// 	}

// 	// Coba parse sebagai angka (Excel serial date)
// 	if days, err := strconv.ParseFloat(cellValue, 64); err == nil {
// 		// Excel epoch: 30 Dec 1899
// 		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
// 		date := excelEpoch.Add(time.Duration(days * 24 * float64(time.Hour)))
// 		return date.Format("2006-01-02")
// 	}

// 	// Jika bukan angka, coba parse sebagai string date
// 	dateFormats := []string{
// 		"2006-01-02",
// 		"02/01/2006",
// 		"01/02/2006",
// 		"2/1/2006",
// 		"1/2/2006",
// 		"2006/01/02",
// 		"02-01-2006",
// 		"01-02-2006",
// 		"2-Jan-06",
// 		"2-January-2006",
// 	}

// 	for _, format := range dateFormats {
// 		if t, err := time.Parse(format, cellValue); err == nil {
// 			return t.Format("2006-01-02")
// 		}
// 	}

// 	// Return original kalau gagal
// 	return cellValue
// }

func getCellAsDateStrict(row []string, index int) (string, error) {
	cellValue := strings.TrimSpace(getCell(row, index))
	if cellValue == "" {
		return "", fmt.Errorf("date value is empty")
	}

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

// =======================================
// END IMPORT FROM EXCEL FILE
// =======================================
