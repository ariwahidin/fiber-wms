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
	ID            int      `json:"ID"`
	InboundID     int      `json:"inbound_id"`
	ItemCode      string   `json:"item_code"`
	Quantity      float64  `json:"quantity"`
	QaStatus      string   `json:"qa_status"`
	Location      string   `json:"location"`
	WhsCode       string   `json:"whs_code"`
	UOM           string   `json:"uom"`
	RecDate       string   `json:"rec_date"`
	ProdDate      string   `json:"prod_date"`
	ExpDate       string   `json:"exp_date"`
	LotNumber     string   `json:"lot_number"`
	SerialNumber  string   `json:"serial_number"`
	SerialNumbers []string `json:"serial_numbers"`
	CartonNumber  string   `json:"carton_number"`
	CaseNumber    string   `json:"case_number"`
	Remarks       string   `json:"remarks"`
	IsSerial      string   `json:"is_serial"`
	Mode          string   `json:"mode"`
	RefId         int      `json:"ref_id"`
	RefNo         string   `json:"ref_no"`
	DivisionCode  string   `json:"division_code"`
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

		if InventoryPolicy.UseCartonNumber {
			if item.CartonNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Carton number cannot be empty",
					"error":   "Carton number cannot be empty",
				})
			}
		}

		if InventoryPolicy.UseCaseNumber {
			if item.CaseNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Case number cannot be empty",
					"error":   "Case number cannot be empty",
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

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s, division %s, serial_number %s, carton_number %s, case_number %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber),
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
		InboundDetail.CartonNumber = item.CartonNumber
		InboundDetail.CaseNumber = item.CaseNumber
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

		// Insert serial numbers kalau product pakai serial
		if len(item.SerialNumbers) > 0 {

			// validasi jumlah serial harus sama dengan qty
			if len(item.SerialNumbers) != int(item.Quantity) {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Jumlah serial number tidak sesuai dengan quantity untuk item " + item.ItemCode,
					"error":   "Jumlah serial number tidak sesuai dengan quantity",
				})
			}

			seen := make(map[string]bool)
			for _, sn := range item.SerialNumbers {
				sn = strings.TrimSpace(sn)
				if sn == "" {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"success": false,
						"message": "Serial number tidak boleh kosong untuk item " + item.ItemCode,
						"error":   "Serial number kosong",
					})
				}
				if seen[sn] {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"success": false,
						"message": "Serial number duplikat: " + sn,
						"error":   "Serial number duplikat",
					})
				}
				seen[sn] = true

				// cek serial sudah ada di sistem (belum pernah dipakai / masih aktif)
				var existing models.InboundSerial
				errCheck := tx.Debug().Where("serial_number = ?", sn).First(&existing).Error
				if errCheck == nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"success": false,
						"message": "Serial number sudah terdaftar: " + sn,
						"error":   "Serial number sudah terdaftar",
					})
				} else if !errors.Is(errCheck, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errCheck.Error()})
				}

				inboundSerial := models.InboundSerial{
					InboundId:       int(inboundID),
					InboundDetailId: int(InboundDetail.ID),
					SerialNumber:    sn,
					CreatedBy:       userID,
					UpdatedBy:       userID,
				}

				if err := tx.Create(&inboundSerial).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"message": "Failed to insert inbound serial",
						"error":   err.Error(),
					})
				}
			}
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

	// Check duplicate item code (tidak diubah)
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

		// if InventoryPolicy.RequireLotNumber {
		// 	if item.LotNumber == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Lot number cannot be empty",
		// 			"error":   "Lot number cannot be empty",
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

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s, division %s, serial_number %s, carton_number %s, case_number %s",
					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber),
			})
		}

		itemCodes[key] = true
	}

	payloadItem := payload.Items
	userID := int(ctx.Locals("userID").(float64))

	// Start transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}

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

	var supplier models.Supplier
	if err := tx.First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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

		// Update/Create References (tidak diubah)
		for _, item := range payload.References {

			var InboundReference models.InboundReference
			if err := tx.First(&InboundReference, "id = ?", item.ID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
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
				InboundReference.RefNo = item.RefNo
				if err := tx.Model(&models.InboundReference{}).Where("id = ?", InboundReference.ID).Updates(InboundReference).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

		// ===== BATCH PRE-FETCH sebelum loop item =====

		itemCodeSet := make(map[string]bool, len(payloadItem))
		detailIDs := make([]uint, 0, len(payloadItem))

		fmt.Println("detail IDs length:", len(payloadItem))

		for _, item := range payloadItem {
			itemCodeSet[item.ItemCode] = true
			fmt.Println("item ID:", item.ID)
			if item.ID > 0 {
				detailIDs = append(detailIDs, uint(item.ID))
			}
		}

		itemCodeList := make([]string, 0, len(itemCodeSet))
		for code := range itemCodeSet {
			itemCodeList = append(itemCodeList, code)
		}

		// Batch fetch Product
		var products []models.Product
		if err := tx.Where("item_code IN ?", itemCodeList).Find(&products).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		productMap := make(map[string]models.Product, len(products))
		for _, p := range products {
			productMap[p.ItemCode] = p
		}

		// Batch fetch UomConversion
		var uomConversions []models.UomConversion
		if err := tx.Where("item_code IN ?", itemCodeList).Find(&uomConversions).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		uomMap := make(map[string]models.UomConversion, len(uomConversions))
		for _, u := range uomConversions {
			uomMap[u.ItemCode+"|"+u.FromUom] = u
		}

		// Batch fetch existing InboundDetail
		detailMap := make(map[uint]models.InboundDetail)
		if len(detailIDs) > 0 {
			var existingDetails []models.InboundDetail
			if err := tx.Where("id IN ?", detailIDs).Find(&existingDetails).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			for _, d := range existingDetails {
				detailMap[d.ID] = d
			}
		}

		fmt.Println("detail Map : ", len(detailMap))

		// debug isi detailMap
		for k, v := range detailMap {
			fmt.Println("detail Map : ", k, v)
		}

		// Batch fetch InboundBarcode aggregate (gantikan GetInboundBarcodeByOutboundDetailID per-item)
		barcodeMap := make(map[uint]repositories.ResulInboundBarcodeByOutboundDetailID)

		// debug isi barcodeMap

		if len(detailIDs) > 0 {
			bm, err := inboundRepo.GetInboundBarcodesByDetailIDs(detailIDs)
			if err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			barcodeMap = bm
		}

		for k, v := range barcodeMap {
			fmt.Println("barcode Map : ", k, v)
		}

		// Batch fetch InboundBarcode "in stock" (buat serial/carton/case check)
		inStockMap := make(map[int]models.InboundBarcode)
		if len(detailIDs) > 0 && (InventoryPolicy.UseSerialNumber || InventoryPolicy.UseCartonNumber || InventoryPolicy.UseCaseNumber) {
			var inStockBarcodes []models.InboundBarcode
			if err := tx.Where("inbound_detail_id IN ? AND status = 'in stock'", detailIDs).Find(&inStockBarcodes).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			for _, b := range inStockBarcodes {
				inStockMap[b.InboundDetailId] = b
			}
		}

		// Batch fetch existing InboundSerial
		serialMap := make(map[uint][]models.InboundSerial)
		if len(detailIDs) > 0 {
			var existingSerials []models.InboundSerial
			if err := tx.Where("inbound_detail_id IN ?", detailIDs).Find(&existingSerials).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			for _, s := range existingSerials {
				serialMap[uint(s.InboundDetailId)] = append(serialMap[uint(s.InboundDetailId)], s)
			}
		}

		// ===== Loop item pakai map lookup, no query =====

		syncInboundSerials := func(detailID uint, itemCode string, serials []string, alreadyInStock bool) error {
			if len(serials) == 0 {
				return nil
			}

			trimmed := make([]string, 0, len(serials))
			seen := make(map[string]bool)
			for _, sn := range serials {
				sn = strings.TrimSpace(sn)
				if sn == "" {
					return fmt.Errorf("serial number tidak boleh kosong untuk item %s", itemCode)
				}
				if seen[sn] {
					return fmt.Errorf("serial number duplikat: %s", sn)
				}
				seen[sn] = true
				trimmed = append(trimmed, sn)
			}

			if alreadyInStock {
				existingSet := make(map[string]bool)
				for _, e := range serialMap[detailID] {
					existingSet[e.SerialNumber] = true
				}
				for _, sn := range trimmed {
					if !existingSet[sn] {
						if !existingSet[sn] {
							return fmt.Errorf("item %s sudah discan, tidak bisa mengubah serial number", itemCode)
						}
					}
				}
				return nil
			}

			for _, sn := range trimmed {
				var existing models.InboundSerial
				errCheck := tx.Debug().Where("serial_number = ? AND inbound_detail_id != ?", sn, detailID).First(&existing).Error
				if errCheck == nil {
					return fmt.Errorf("serial number sudah terdaftar: %s", sn)
				} else if !errors.Is(errCheck, gorm.ErrRecordNotFound) {
					return errCheck
				}
			}

			if err := tx.Where("inbound_detail_id = ?", detailID).Delete(&models.InboundSerial{}).Error; err != nil {
				return err
			}

			for _, sn := range trimmed {
				newSerial := models.InboundSerial{
					InboundId:       int(InboundHeader.ID),
					InboundDetailId: int(detailID),
					SerialNumber:    sn,
					CreatedBy:       userID,
					UpdatedBy:       userID,
				}
				if err := tx.Create(&newSerial).Error; err != nil {
					return err
				}
			}

			return nil
		}

		// var totalItem = 0
		for _, item := range payloadItem {

			product, ok := productMap[item.ItemCode]
			if !ok {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}

			uomConversion, ok := uomMap[item.ItemCode+"|"+item.UOM]
			if !ok {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found"})
			}

			fmt.Println("Processing item ID:", item.ID, "ItemCode:", item.ItemCode)

			inboundDetail, found := detailMap[uint(item.ID)]

			fmt.Println("Found inbound detail:", found)
			if found {
				fmt.Println("Found inbound detail:", inboundDetail)
			}

			if !InventoryPolicy.UseLotNo {
				item.LotNumber = InboundHeader.InboundNo
			}

			// inputQty := item.Quantity
			// if item.SerialNumber != "" {
			// 	inputQty = 1
			// }

			inputQty := item.Quantity
			if !found {
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
					CartonNumber:  item.CartonNumber,
					CaseNumber:    item.CaseNumber,
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

				if err := tx.Create(&newDetail).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

				if len(item.SerialNumbers) > 0 {
					if len(item.SerialNumbers) != int(item.Quantity) {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Jumlah serial number tidak sesuai quantity untuk item " + item.ItemCode})
					}
					if err := syncInboundSerials(newDetail.ID, item.ItemCode, item.SerialNumbers, false); err != nil {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
					}
				}
				continue
			}

			// Update existing detail
			// inboundBarcode, hasBarcode := barcodeMap[uint(inboundDetail.ID)]
			// if !hasBarcode {
			// 	// setara dengan cabang ErrRecordNotFound di versi lama -> skip update
			// 	continue
			// }

			inboundBarcode, hasBarcode := barcodeMap[uint(inboundDetail.ID)]

			if hasBarcode {
				if inboundBarcode.ID > 0 {
					if inboundBarcode.ItemID != product.ID {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + inboundDetail.ItemCode + " already scanned, cannot update to " + item.ItemCode})
					}
				}

				if inboundBarcode.ItemID == product.ID {
					if inboundBarcode.TotalScan > int(item.Quantity) {
						tx.Rollback()
						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity update for item " + item.ItemCode + " is less than the total scanned quantity"})
					}
				}
			}

			// kode update inboundDetail tetap lanjut di bawah, di luar if-else ini

			fmt.Println("Inbound Barcode for detail ID", inboundDetail.ID, ":", inboundBarcode)

			if inboundBarcode.ID > 0 {
				if inboundBarcode.ItemID != product.ID {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + inboundDetail.ItemCode + " already scanned, cannot update to " + item.ItemCode})
				}
				// if inboundBarcode.ExpDate != item.ExpDate {
				// 	tx.Rollback()
				// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Expiry Date"})
				// }
				// if inboundBarcode.LotNumber != item.LotNumber {
				// 	tx.Rollback()
				// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Lot Number"})
				// }
				// if inboundBarcode.ProdDate != item.ProdDate {
				// 	tx.Rollback()
				// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Production Date"})
				// }
			}

			if inboundBarcode.ItemID == product.ID {
				if inboundBarcode.TotalScan > int(item.Quantity) {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity update for item " + item.ItemCode + " is less than the total scanned quantity"})
				}
			}

			if inStock, ok := inStockMap[int(inboundDetail.ID)]; ok {
				if InventoryPolicy.UseSerialNumber && inStock.SerialNumber != item.SerialNumber {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Serial Number"})
				}
				if InventoryPolicy.UseCartonNumber && inStock.CartonNumber != item.CartonNumber {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Carton Number"})
				}
				if InventoryPolicy.UseCaseNumber && inStock.CaseNumber != item.CaseNumber {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Case Number"})
				}
			}

			fmt.Println("Updating item ID:", inboundDetail.ID, "ItemCode:", item.ItemCode)

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
			inboundDetail.CartonNumber = item.CartonNumber
			inboundDetail.CaseNumber = item.CaseNumber
			inboundDetail.Uom = item.UOM
			inboundDetail.IsSerial = product.HasSerial
			inboundDetail.RefNo = item.RefNo
			inboundDetail.RefId = item.RefId
			inboundDetail.OwnerCode = InboundHeader.OwnerCode
			inboundDetail.QaStatus = item.QaStatus
			inboundDetail.DivisionCode = item.DivisionCode
			inboundDetail.UpdatedBy = userID

			// totalItem += int(inputQty)

			// if err := tx.Save(&inboundDetail).Error; err != nil {
			// 	tx.Rollback()
			// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			// }

			if err := tx.Save(&inboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if len(item.SerialNumbers) > 0 {
				if len(item.SerialNumbers) != int(item.Quantity) {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Jumlah serial number tidak sesuai quantity untuk item " + item.ItemCode})
				}

				_, alreadyInStock := inStockMap[int(inboundDetail.ID)]

				if err := syncInboundSerials(inboundDetail.ID, item.ItemCode, item.SerialNumbers, alreadyInStock); err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}

		message = "Update Inbound " + InboundHeader.InboundNo + " successfully"
	} else {
		message = "Update Inbound Header " + InboundHeader.InboundNo + " successfully"
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": message})
}

// func (c *InboundController) UpdateInboundByID(ctx *fiber.Ctx) error {
// 	inbound_no := ctx.Params("inbound_no")

// 	fmt.Printf("UpdateInboundByID called with inbound_no=%s\n", inbound_no)

// 	var payload Inbound

// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var InventoryPolicy models.InventoryPolicy
// 	if err := c.DB.Where("owner_code = ?", payload.OwnerCode).First(&InventoryPolicy).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to get inventory policy",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Check duplicate item code
// 	itemCodes := make(map[string]bool)
// 	for _, item := range payload.Items {

// 		if item.Quantity == 0 {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"message": "Quantity cannot be zero",
// 				"error":   "Quantity cannot be zero",
// 			})
// 		}

// 		if item.UOM == "" {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"message": "UOM cannot be empty",
// 				"error":   "UOM cannot be empty",
// 			})
// 		}

// 		if InventoryPolicy.RequireLotNumber {
// 			if item.LotNumber == "" {
// 				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 					"success": false,
// 					"message": "Lot number cannot be empty",
// 					"error":   "Lot number cannot be empty",
// 				})
// 			}
// 		}

// 		// if InventoryPolicy.RequireExpiryDate {
// 		// 	if item.ExpDate == "" {
// 		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 		// 			"success": false,
// 		// 			"message": "Expiration date cannot be empty",
// 		// 			"error":   "Expiration date cannot be empty",
// 		// 		})
// 		// 	}
// 		// }

// 		// if InventoryPolicy.UseProductionDate {
// 		// 	if item.ProdDate == "" {
// 		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 		// 			"success": false,
// 		// 			"message": "Production date cannot be empty",
// 		// 			"error":   "Production date cannot be empty",
// 		// 		})
// 		// 	}
// 		// }

// 		if InventoryPolicy.UseReceiveLocation {
// 			if item.Location == "" {
// 				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 					"success": false,
// 					"message": "Receive location cannot be empty",
// 					"error":   "Receive location cannot be empty",
// 				})
// 			}
// 		}

// 		if item.ItemCode == "" {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"message": "Item code cannot be empty",
// 				"error":   "Item code cannot be empty",
// 			})
// 		}

// 		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber)

// 		if itemCodes[key] {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"message": "Duplicate item found: " + item.ItemCode,
// 				"error": fmt.Sprintf("Duplicate item found with ItemCode %s, rec_date %s, exp_date %s, lot_number %s, prod_date %s, location %s, uom %s, status %s, division %s, serial_number %s, carton_number %s, case_number %s",
// 					item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.QaStatus, item.DivisionCode, item.SerialNumber, item.CartonNumber, item.CaseNumber),
// 			})
// 		}

// 		itemCodes[key] = true
// 	}

// 	payloadItem := payload.Items
// 	userID := int(ctx.Locals("userID").(float64))

// 	// Start transaction
// 	tx := c.DB.Begin()
// 	log.Printf("[UpdateInbound] START inbound_no=%s userID=%d", inbound_no, userID)
// 	if tx.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
// 	}

// 	// Defer rollback in case of panic
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	inboundRepo := repositories.NewInboundRepository(tx)

// 	var InboundHeader models.InboundHeader
// 	if err := tx.First(&InboundHeader, "inbound_no = ?", inbound_no).Error; err != nil {
// 		tx.Rollback()
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}
// 	log.Printf("[UpdateInbound] InboundHeader found id=%d status=%s", InboundHeader.ID, InboundHeader.Status)

// 	var supplier models.Supplier
// 	if err := tx.First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
// 		tx.Rollback()
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Check if inbound is complete
// 	if InboundHeader.Status == "complete" {
// 		tx.Rollback()
// 		message := "Inbound " + inbound_no + " is already complete"
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message, "message": message})
// 	}

// 	// Update InboundHeader
// 	InboundHeader.InboundDate = payload.InboundDate
// 	InboundHeader.Supplier = payload.Supplier
// 	InboundHeader.SupplierId = int(supplier.ID)
// 	InboundHeader.ReceiptID = payload.ReceiptID
// 	InboundHeader.Type = payload.Type
// 	InboundHeader.Remarks = payload.Remarks
// 	InboundHeader.UpdatedBy = userID
// 	InboundHeader.Transporter = payload.Transporter
// 	InboundHeader.NoTruck = payload.NoTruck
// 	InboundHeader.Driver = payload.Driver
// 	InboundHeader.Container = payload.Container
// 	InboundHeader.WhsCode = payload.WhsCode
// 	InboundHeader.OwnerCode = payload.OwnerCode
// 	InboundHeader.Origin = payload.Origin
// 	InboundHeader.PoDate = payload.PoDate
// 	InboundHeader.ArrivalTime = payload.ArrivalTime
// 	InboundHeader.StartUnloading = payload.StartUnloading
// 	InboundHeader.EndUnloading = payload.EndUnloading
// 	InboundHeader.TruckSize = payload.TruckSize
// 	InboundHeader.BLNo = payload.BLNo
// 	InboundHeader.Koli = payload.Koli

// 	if err := tx.Model(&models.InboundHeader{}).Where("id = ?", InboundHeader.ID).Updates(InboundHeader).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var message string

// 	if InboundHeader.Status == "open" {
// 		// Update/Create References
// 		for _, item := range payload.References {

// 			var InboundReference models.InboundReference
// 			if err := tx.First(&InboundReference, "id = ?", item.ID).Error; err != nil {
// 				if errors.Is(err, gorm.ErrRecordNotFound) {
// 					// Create new reference
// 					InboundReference.InboundId = uint(InboundHeader.ID)
// 					InboundReference.RefNo = item.RefNo
// 					if err := tx.Create(&InboundReference).Error; err != nil {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 					}
// 				} else {
// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}
// 			} else {
// 				// Update existing reference
// 				InboundReference.RefNo = item.RefNo
// 				if err := tx.Model(&models.InboundReference{}).Where("id = ?", InboundReference.ID).Updates(InboundReference).Error; err != nil {
// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}
// 			}
// 		}

// 		// Update/Create Items
// 		for _, item := range payloadItem {

// 			log.Printf("[UpdateInbound] Processing item=%s id=%d", item.ItemCode, item.ID)
// 			var product models.Product
// 			if err := tx.First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
// 				tx.Rollback()
// 				if errors.Is(err, gorm.ErrRecordNotFound) {
// 					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
// 				}
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}

// 			var uomConversion models.UomConversion
// 			if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, item.UOM).Error; err != nil {
// 				tx.Rollback()
// 				if errors.Is(err, gorm.ErrRecordNotFound) {
// 					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found"})
// 				}
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}

// 			var inboundDetail models.InboundDetail
// 			err := tx.First(&inboundDetail, "id = ?", item.ID).Error

// 			if !InventoryPolicy.UseLotNo {
// 				item.LotNumber = InboundHeader.InboundNo
// 			}

// 			if !InventoryPolicy.RequireExpiryDate {
// 				// item.ExpDate = item.RecDate
// 			}

// 			if !InventoryPolicy.UseProductionDate {
// 				// item.ProdDate = item.RecDate
// 			}

// 			inputQty := item.Quantity
// 			if item.SerialNumber != "" {
// 				inputQty = 1
// 			}

// 			if errors.Is(err, gorm.ErrRecordNotFound) {
// 				// Create new detail
// 				newDetail := models.InboundDetail{
// 					InboundId:     int(InboundHeader.ID),
// 					InboundNo:     InboundHeader.InboundNo,
// 					ItemId:        product.ID,
// 					ProductNumber: product.ProductNumber,
// 					ItemCode:      item.ItemCode,
// 					Barcode:       uomConversion.Ean,
// 					Quantity:      inputQty,
// 					Location:      item.Location,
// 					WhsCode:       InboundHeader.WhsCode,
// 					RecDate:       item.RecDate,
// 					ProdDate:      item.ProdDate,
// 					ExpDate:       item.ExpDate,
// 					LotNumber:     item.LotNumber,
// 					Uom:           item.UOM,
// 					IsSerial:      product.HasSerial,
// 					SerialNumber:  item.SerialNumber,
// 					CartonNumber:  item.CartonNumber,
// 					CaseNumber:    item.CaseNumber,
// 					RefNo:         item.RefNo,
// 					RefId:         item.RefId,
// 					OwnerCode:     InboundHeader.OwnerCode,
// 					QaStatus:      item.QaStatus,
// 					DivisionCode:  item.DivisionCode,
// 					CreatedBy:     userID,
// 				}
// 				if err := tx.Create(&newDetail).Error; err != nil {
// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}
// 			} else if err == nil {

// 				inboundBarcode, err := inboundRepo.GetInboundBarcodeByOutboundDetailID(uint(inboundDetail.ID))
// 				log.Printf("[UpdateInbound] GetInboundBarcode detailID=%d err=%v barcodeID=%d", inboundDetail.ID, err, inboundBarcode.ID)
// 				if err != nil {

// 					if errors.Is(err, gorm.ErrRecordNotFound) {
// 						log.Printf("[UpdateInbound] Barcode not found for detailID=%d, CONTINUE (skipping update)", inboundDetail.ID)
// 						continue
// 					}

// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}

// 				if inboundBarcode.ID > 0 {
// 					if inboundBarcode.ItemID != product.ID {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + inboundDetail.ItemCode + " already scanned, cannot update to " + item.ItemCode})
// 					}

// 					if inboundBarcode.ExpDate != item.ExpDate {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Expiry Date"})
// 					}

// 					if inboundBarcode.LotNumber != item.LotNumber {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Lot Number"})
// 					}

// 					if inboundBarcode.ProdDate != item.ProdDate {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Production Date"})
// 					}

// 				}

// 				if inboundBarcode.ItemID == product.ID {
// 					if inboundBarcode.TotalScan > int(item.Quantity) {
// 						tx.Rollback()
// 						return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity update for item " + item.ItemCode + " is less than the total scanned quantity"})
// 					}
// 				}

// 				if InventoryPolicy.UseSerialNumber {
// 					var inboundBarcode models.InboundBarcode
// 					if err := tx.First(&inboundBarcode, "inbound_detail_id = ? and status = 'in stock'", inboundDetail.ID).Error; err == nil {
// 						if inboundBarcode.SerialNumber != item.SerialNumber {
// 							tx.Rollback()
// 							return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Serial Number"})
// 						}
// 					}
// 				}

// 				if InventoryPolicy.UseCartonNumber {
// 					var inboundBarcode models.InboundBarcode
// 					if err := tx.First(&inboundBarcode, "inbound_detail_id = ? and status = 'in stock'", inboundDetail.ID).Error; err == nil {
// 						if inboundBarcode.CartonNumber != item.CartonNumber {
// 							tx.Rollback()
// 							return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Carton Number"})
// 						}
// 					}
// 				}

// 				if InventoryPolicy.UseCaseNumber {
// 					var inboundBarcode models.InboundBarcode
// 					if err := tx.First(&inboundBarcode, "inbound_detail_id = ? and status = 'in stock'", inboundDetail.ID).Error; err == nil {
// 						if inboundBarcode.CaseNumber != item.CaseNumber {
// 							tx.Rollback()
// 							return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + item.ItemCode + " already scanned, cannot update Case Number"})
// 						}
// 					}
// 				}

// 				// Update existing detail
// 				inboundDetail.ItemId = product.ID
// 				inboundDetail.ItemCode = item.ItemCode
// 				inboundDetail.Barcode = uomConversion.Ean
// 				inboundDetail.Quantity = inputQty
// 				inboundDetail.Location = item.Location
// 				inboundDetail.WhsCode = InboundHeader.WhsCode
// 				inboundDetail.RecDate = item.RecDate
// 				inboundDetail.ProdDate = item.ProdDate
// 				inboundDetail.ExpDate = item.ExpDate
// 				inboundDetail.LotNumber = item.LotNumber
// 				inboundDetail.SerialNumber = item.SerialNumber
// 				inboundDetail.CartonNumber = item.CartonNumber
// 				inboundDetail.CaseNumber = item.CaseNumber
// 				inboundDetail.Uom = item.UOM
// 				inboundDetail.IsSerial = product.HasSerial
// 				inboundDetail.RefNo = item.RefNo
// 				inboundDetail.RefId = item.RefId
// 				inboundDetail.OwnerCode = InboundHeader.OwnerCode
// 				inboundDetail.QaStatus = item.QaStatus
// 				inboundDetail.DivisionCode = item.DivisionCode
// 				inboundDetail.UpdatedBy = userID

// 				if err := tx.Save(&inboundDetail).Error; err != nil {
// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}
// 			} else {
// 				// Other error
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}
// 		}

// 		message = "Update Inbound " + InboundHeader.InboundNo + " successfully"
// 	} else {
// 		message = "Update Inbound Header " + InboundHeader.InboundNo + " successfully"
// 	}

// 	log.Printf("[UpdateInbound] About to COMMIT")
// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	log.Printf("[UpdateInbound] COMMIT SUCCESS")

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": message})
// }

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

	var totalDetails int64
	c.DB.Model(&models.InboundDetail{}).
		Where("inbound_no = ?", inbound_no).
		Count(&totalDetails)

	if err := c.DB.Debug().
		Preload("InboundReferences").
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

	// Ambil serial number per detail
	detailIDs := make([]uint, 0, len(inbounDetails))
	for _, d := range inbounDetails {
		detailIDs = append(detailIDs, d.ID)
	}

	serialsByDetail := make(map[uint][]string)
	if len(detailIDs) > 0 {
		var serials []models.InboundSerial
		if err := c.DB.Where("inbound_detail_id IN ?", detailIDs).Find(&serials).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		for _, s := range serials {
			serialsByDetail[uint(s.InboundDetailId)] = append(serialsByDetail[uint(s.InboundDetailId)], s.SerialNumber)
		}
	}

	// Attach ke tiap detail via map, response gabungan
	type detailWithSerial struct {
		repositories.ResultDetail
		SerialNumbers []string `json:"serial_numbers"`
	}

	responseDetails := make([]detailWithSerial, 0, len(inbounDetails))
	for _, d := range inbounDetails {
		sn := serialsByDetail[d.ID]
		if sn == nil {
			sn = []string{}
		}
		responseDetails = append(responseDetails, detailWithSerial{
			ResultDetail:  d,
			SerialNumbers: sn,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"data":          inbound,
		"details":       responseDetails,
		"details_limit": limit,
		"details_total": totalDetails,
	})
}

// func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
// 	inbound_no := ctx.Params("inbound_no")
// 	limit := ctx.QueryInt("limit", 5000)

// 	var inbound models.InboundHeader
// 	fmt.Println("GetInboundByID inbound_no:", inbound_no)

// 	var totalDetails int64
// 	c.DB.Model(&models.InboundDetail{}).
// 		Where("inbound_no = ?", inbound_no).
// 		Count(&totalDetails)

// 	if err := c.DB.Debug().
// 		Preload("InboundReferences").
// 		// Received dihapus dari sini
// 		Preload("Details", func(db *gorm.DB) *gorm.DB {
// 			return db.Limit(limit).Order("id ASC")
// 		}).
// 		First(&inbound, "inbound_no = ?", inbound_no).Error; err != nil {

// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)
// 	inbounDetails, err := inboundRepo.GetInboundDetailByInboundID(inbound.ID)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success":       true,
// 		"data":          inbound,
// 		"details":       inbounDetails,
// 		"details_limit": limit,
// 		"details_total": totalDetails,
// 	})
// }

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
	// hapus serial number yang terkait dulu (kalau ada), baru detail-nya
	if err := c.DB.Debug().Unscoped().Where("inbound_detail_id = ?", inboundDetail.ID).Delete(&models.InboundSerial{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// hard delete
	if err := c.DB.Debug().Unscoped().Delete(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

// func (c *InboundController) DeleteItem(ctx *fiber.Ctx) error {

// 	inbound_detail_id := ctx.Params("id")
// 	var inboundDetail models.InboundDetail
// 	if err := c.DB.Debug().First(&inboundDetail, "id = ?", inbound_detail_id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var InboundHeader models.InboundHeader
// 	if err := c.DB.Debug().First(&InboundHeader, "inbound_no = ?", inboundDetail.InboundNo).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if InboundHeader.Status != "open" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + inboundDetail.InboundNo + " is not open", "message": "Inbound not open"})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)

// 	inboundBarcode, err := inboundRepo.GetInboundBarcodeByOutboundDetailID(uint(inboundDetail.ID))
// 	if err != nil {

// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			// data barcode tidak ada → boleh lanjut delete
// 			goto DELETE
// 		}

// 		return ctx.Status(fiber.StatusInternalServerError).
// 			JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if inboundBarcode.ItemID == inboundDetail.ItemId {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This item is already scanned", "message": "Item already scanned"})
// 	}

// DELETE:
// 	// hard delete
// 	if err := c.DB.Debug().Unscoped().Delete(&inboundDetail).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
// }

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

	inboundRepo := repositories.NewInboundRepository(c.DB)

	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
	if err != nil {
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

			inputSerialNumber := detail.SerialNumber
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
					CartonNumber:    detail.CartonNumber,
					CaseNumber:      detail.CaseNumber,
					SerialNumber:    inputSerialNumber,
					Pallet:          palletID,
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

// func (c *InboundController) PutawayByInboundNo(ctx *fiber.Ctx) error {
// 	var payload struct {
// 		InboundNo string `json:"inbound_no"`
// 	}

// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
// 	}

// 	inboundHeader := models.InboundHeader{}
// 	if err := c.DB.First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var invPolicy models.InventoryPolicy
// 	if err := c.DB.First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var warehouse models.Warehouse
// 	if err := c.DB.First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if invPolicy.RequirePutawayScan {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
// 	}

// 	var inboundDetail []models.InboundDetail
// 	if err := c.DB.Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if !invPolicy.RequireReceiveScan {

// 		var allRcvLocationIsFilled = true
// 		for _, detail := range inboundDetail {
// 			if detail.Location == "" {
// 				allRcvLocationIsFilled = false
// 				break
// 			}
// 		}
// 		if !allRcvLocationIsFilled {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
// 		}

// 		// ===== BATCH PRE-FETCH =====
// 		detailIDs := make([]uint, 0, len(inboundDetail))
// 		locationCodeSet := make(map[string]bool)
// 		for _, d := range inboundDetail {
// 			detailIDs = append(detailIDs, d.ID)
// 			locationCodeSet[d.Location] = true
// 		}
// 		locationCodes := make([]string, 0, len(locationCodeSet))
// 		for lc := range locationCodeSet {
// 			locationCodes = append(locationCodes, lc)
// 		}

// 		// Batch: existing InboundBarcode per detail (buat hitung totalQtyScanned)
// 		// var existingBarcodes []models.InboundBarcode
// 		// if err := c.DB.Where("inbound_detail_id IN ?", detailIDs).Find(&existingBarcodes).Error; err != nil {
// 		// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		// }

// 		var existingBarcodes []models.InboundBarcode
// 		if err := helpers.FindInChunks(c.DB, "inbound_detail_id IN ?", detailIDs, &existingBarcodes); err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}

// 		scannedQtyMap := make(map[int]float64) // key: InboundDetailId
// 		for _, b := range existingBarcodes {
// 			scannedQtyMap[b.InboundDetailId] += b.Quantity
// 		}

// 		// Batch: Location
// 		// var locations []models.Location
// 		// if err := c.DB.Where("location_code IN ?", locationCodes).Find(&locations).Error; err != nil {
// 		// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		// }

// 		var locations []models.Location
// 		if err := helpers.FindInChunks(c.DB, "location_code IN ?", locationCodes, &locations); err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}

// 		locationMap := make(map[string]models.Location, len(locations))
// 		for _, l := range locations {
// 			locationMap[l.LocationCode] = l
// 		}

// 		// ===== Loop: susun barcode baru pakai map, no query =====
// 		newBarcodes := make([]models.InboundBarcode, 0, len(inboundDetail))
// 		userID := int(ctx.Locals("userID").(float64))

// 		for _, detail := range inboundDetail {

// 			location, ok := locationMap[detail.Location]
// 			if !ok {
// 				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location " + detail.Location + " not registered in system"})
// 			}

// 			totalQtyScanned := scannedQtyMap[int(detail.ID)]
// 			newQtyScanned := detail.Quantity - totalQtyScanned

// 			inputSerialNumber := detail.SerialNumber
// 			if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
// 				inputSerialNumber = detail.SerialNumber
// 			}

// 			if newQtyScanned > 0 {
// 				newBarcodes = append(newBarcodes, models.InboundBarcode{
// 					InboundId:       int(inboundHeader.ID),
// 					InboundDetailId: int(detail.ID),
// 					ItemCode:        detail.ItemCode,
// 					ItemID:          detail.ItemId,
// 					ScanData:        detail.Barcode,
// 					Barcode:         detail.Barcode,
// 					CartonNumber:    detail.CartonNumber,
// 					CaseNumber:      detail.CaseNumber,
// 					SerialNumber:    inputSerialNumber,
// 					Pallet:          payload.InboundNo,
// 					Location:        location.LocationCode,
// 					Quantity:        newQtyScanned,
// 					WhsCode:         detail.WhsCode,
// 					OwnerCode:       detail.OwnerCode,
// 					DivisionCode:    detail.DivisionCode,
// 					QaStatus:        detail.QaStatus,
// 					Status:          "pending",
// 					Uom:             detail.Uom,
// 					RecDate:         detail.RecDate,
// 					ProdDate:        detail.ProdDate,
// 					ExpDate:         detail.ExpDate,
// 					LotNumber:       detail.LotNumber,
// 					CreatedBy:       userID,
// 				})
// 			}
// 		}

// 		// Batch insert sekali (bukan Create() satu-satu)
// 		// if len(newBarcodes) > 0 {
// 		// 	if err := c.DB.Create(&newBarcodes).Error; err != nil {
// 		// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		// 	}
// 		// }

// 		// if len(newBarcodes) > 0 {
// 		// 	if err := c.DB.CreateInBatches(&newBarcodes, 100).Error; err != nil {
// 		// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		// 	}
// 		// }

// 		if len(newBarcodes) > 0 {
// 			if err := helpers.CreateInChunks(c.DB, newBarcodes); err != nil {
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}
// 		}
// 	}

// 	var inboundBarcodes []models.InboundBarcode
// 	if err := c.DB.Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(inboundBarcodes) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)

// 	barcodeIDs := make([]int, 0, len(inboundBarcodes))
// 	for _, b := range inboundBarcodes {
// 		barcodeIDs = append(barcodeIDs, int(b.ID))
// 	}

// 	if err := inboundRepo.ProcessPutawayItemsBatch(ctx, barcodeIDs); err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   err.Error(),
// 			"message": err.Error(),
// 		})
// 	}

// 	if err := inboundRepo.UpdateStatusInbound(ctx, inboundHeader.ID); err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error":   err.Error(),
// 			"message": err.Error(),
// 		})
// 	}

// 	// inboundRepo := repositories.NewInboundRepository(c.DB)

// 	// // Header/Policy/Warehouse sudah di tangan — nggak perlu re-fetch per barcode lagi.
// 	// // ProcessPutawayItem tetap dipanggil per barcode (karena tiap barcode = 1 pergerakan inventory,
// 	// // belum aku batch bagian dalamnya — lihat catatan di bawah).
// 	// for _, barcode := range inboundBarcodes {
// 	// 	if _, err := inboundRepo.ProcessPutawayItem(ctx, int(barcode.ID), ""); err != nil {
// 	// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 	// 			"error":   err.Error(),
// 	// 			"message": err.Error(),
// 	// 		})
// 	// 	}
// 	// }

// 	// // UpdateStatusInbound dipanggil SEKALI di akhir, bukan per barcode
// 	// if err := inboundRepo.UpdateStatusInbound(ctx, inboundHeader.ID); err != nil {
// 	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 	// 		"error":   err.Error(),
// 	// 		"message": err.Error(),
// 	// 	})
// 	// }

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Putaway inbound " + inboundHeader.InboundNo + " successfully"})
// }

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
