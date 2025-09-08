package controllers

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/types"
	"fmt"
	"log"
	"strconv"
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
	ID types.SnowflakeID `json:"ID"`
	// ID             int                       `json:"ID"`
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
	// ID int `json:"ID"`
	ID          types.SnowflakeID `json:"ID"`
	InboundID   types.SnowflakeID `json:"inbound_id"`
	ItemCode    string            `json:"item_code"`
	Quantity    int               `json:"quantity"`
	RcvLocation string            `json:"rcv_location"`
	WhsCode     string            `json:"whs_code"`
	UOM         string            `json:"uom"`
	RecDate     string            `json:"rec_date"`
	Remarks     string            `json:"remarks"`
	IsSerial    string            `json:"is_serial"`
	Mode        string            `json:"mode"`
	RefId       int               `json:"ref_id"`
	RefNo       string            `json:"ref_no"`
	Division    string            `json:"division"`
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

	// Check duplicate item code
	itemCodes := make(map[string]bool) // gunakan map untuk cek duplikat
	for _, item := range payload.Items {
		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		if itemCodes[item.ItemCode] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item code found: " + item.ItemCode,
				"error":   "Duplicate item code",
			})
		}

		itemCodes[item.ItemCode] = true // tandai sebagai sudah ditemukan
	}

	// return nil
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
				"message": "Reference no cannot be empty",
				"error":   "Reference no cannot be empty",
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
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var InboundDetail models.InboundDetail

		var InboundReference models.InboundReference

		if err := tx.Debug().First(&InboundReference, "ref_no = ?", item.RefNo).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound Reference not found"})
			}
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		InboundDetail.InboundNo = payload.InboundNo
		InboundDetail.InboundId = types.SnowflakeID(int64(inboundID))
		InboundDetail.ItemCode = item.ItemCode
		InboundDetail.ItemId = types.SnowflakeID(int64(product.ID))
		InboundDetail.Barcode = product.Barcode
		InboundDetail.Uom = item.UOM
		InboundDetail.Quantity = item.Quantity
		InboundDetail.RcvLocation = item.RcvLocation
		InboundDetail.QaStatus = "A"
		InboundDetail.WhsCode = item.WhsCode
		InboundDetail.RecDate = item.RecDate
		InboundDetail.Remarks = item.Remarks
		InboundDetail.IsSerial = product.HasSerial
		InboundDetail.RefId = int(InboundReference.ID)
		InboundDetail.RefNo = item.RefNo
		InboundDetail.OwnerCode = payload.OwnerCode
		InboundDetail.WhsCode = payload.WhsCode
		InboundDetail.DivisionCode = item.Division
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

	// Check duplicate item code
	itemCodes := make(map[string]bool) // gunakan map untuk cek duplikat
	for _, item := range payload.Items {
		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		if itemCodes[item.ItemCode] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item code found: " + item.ItemCode,
				"error":   "Duplicate item code",
			})
		}

		itemCodes[item.ItemCode] = true
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

			// Coba cari berdasarkan ID
			err := c.DB.Debug().First(&inboundDetail, "id = ?", item.ID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// ❌ Tidak ditemukan → insert baru
				newDetail := models.InboundDetail{
					InboundId:    types.SnowflakeID(int64(InboundHeader.ID)),
					InboundNo:    InboundHeader.InboundNo,
					ItemId:       product.ID,
					ItemCode:     item.ItemCode,
					Barcode:      product.Barcode,
					Quantity:     item.Quantity,
					RcvLocation:  item.RcvLocation,
					WhsCode:      InboundHeader.WhsCode,
					RecDate:      item.RecDate,
					Uom:          item.UOM,
					IsSerial:     product.HasSerial,
					RefNo:        item.RefNo,
					RefId:        item.RefId,
					OwnerCode:    InboundHeader.OwnerCode,
					DivisionCode: item.Division,
					QaStatus:     "A",
					CreatedBy:    int(ctx.Locals("userID").(float64)),
				}
				if err := c.DB.Create(&newDetail).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			} else if err == nil {
				// ✅ Ditemukan → update
				inboundDetail.ItemId = product.ID
				inboundDetail.ItemCode = item.ItemCode
				inboundDetail.Barcode = product.Barcode
				inboundDetail.Quantity = item.Quantity
				inboundDetail.RcvLocation = item.RcvLocation
				inboundDetail.WhsCode = InboundHeader.WhsCode
				inboundDetail.RecDate = item.RecDate
				inboundDetail.Uom = item.UOM
				inboundDetail.IsSerial = product.HasSerial
				inboundDetail.RefNo = item.RefNo
				inboundDetail.RefId = item.RefId
				inboundDetail.OwnerCode = InboundHeader.OwnerCode
				inboundDetail.DivisionCode = item.Division
				inboundDetail.QaStatus = "A"
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
	limit := ctx.QueryInt("limit", 1000)

	if limit > 5000 {
		limit = 5000
	}

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
		ID: types.SnowflakeID(inboundDetail.ID),
		// ID:        int(inboundDetail.ID),
		InboundID: types.SnowflakeID(inboundDetail.InboundId),
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
		InboundDate    string  `json:"inbound_date"`
		ReceiptID      string  `json:"receipt_id"`
		PoNumber       string  `json:"po_number"`
		InboundNo      string  `json:"inbound_no"`
		ItemCode       string  `json:"item_code"`
		Barcode        string  `json:"barcode"`
		SupplierName   string  `json:"supplier_name"`
		Quantity       int     `json:"quantity"`
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
	b.no_truck, b.driver, b.truck_size, b.arrival_time, b.start_unloading, b.end_unloading,
	a.item_code, a.barcode, p.cbm, b.bl_no, b.remarks, b.koli, b.container, a.whs_code,
	s.supplier_name, a.quantity
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway Sheet Found", "data": putawaySheet})
}

func (c *InboundController) PutawayPerItemByInboundNo(ctx *fiber.Ctx) error {
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
	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No pending barcodes found"})
	}

	for _, barcode := range inboundBarcodes {
		barcodeIDStr := strconv.Itoa(int(barcode.ID))

		if err := c.putawayPerItem(ctx, barcodeIDStr); err != nil {
			return err
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway inbound " + inboundHeader.InboundNo + " successfully"})
}

func (c *InboundController) putawayPerItem(ctx *fiber.Ctx, idStr string) error {
	if idStr == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	inboundBarcode := models.InboundBarcode{}
	if err := c.DB.Debug().First(&inboundBarcode, "id = ?", idStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "id = ?", inboundBarcode.InboundId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeaderID := inboundHeader.ID

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	_, errs := inboundRepo.PutawayItem(ctx, id, "")
	if errs != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errs.Error(), "message": errs.Error()})
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
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

	// if err := c.DB.Debug().Model(&models.InboundHeader{}).
	// 	Where("id = ?", inboundHeaderID).
	// 	Updates(map[string]interface{}{
	// 		"status":     statusInbound,
	// 		"putaway_at": time.Now(),
	// 		"putaway_by": userID}).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	now := time.Now()
	updateData := models.InboundHeader{
		Status:    statusInbound,
		PutawayAt: &now,
		PutawayBy: userID,
	}
	if err := c.DB.Debug().Model(&models.InboundHeader{}).
		Where("id = ?", inboundHeaderID).
		Updates(updateData).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway per item"})
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

	fmt.Println("Received IDs:", req.ItemIDs)

	for _, id := range req.ItemIDs {
		if err := c.putawayPerItem(ctx, id); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Gagal putaway ID %s: %v", id, err),
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

	userID := int(ctx.Locals("userID").(float64))
	// update inbound status inbound header with interface
	sqlUpdate := `UPDATE inbound_headers SET status = 'checking', updated_at = ?, updated_by = ?, checking_at = ?, checking_by = ? WHERE inbound_no = ?`
	if err := r.DB.Exec(sqlUpdate, time.Now(), userID, time.Now(), userID, payload.InboundNo).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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
		if detail.RcvLocation == "" {
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
			ItemID:          int(product.ID),
			ItemCode:        detail.ItemCode,
			ScanType:        "BARCODE",
			ScanData:        product.Barcode,
			Barcode:         product.Barcode,
			SerialNumber:    product.Barcode,
			Pallet:          detail.RcvLocation,
			Location:        detail.RcvLocation,
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

	userID := int(ctx.Locals("userID").(float64))
	// update inbound status inbound header with interface
	sqlUpdate := `UPDATE inbound_headers SET status = 'open', updated_at = ?, updated_by = ?, cancel_at = ?, cancel_by = ? WHERE inbound_no = ?`
	if err := r.DB.Exec(sqlUpdate, time.Now(), userID, time.Now(), userID, payload.InboundNo).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	errHistory := helpers.InsertTransactionHistory(
		r.DB,
		payload.InboundNo, // RefNo
		"open",            // Status
		"INBOUND",         // Type
		"",                // Detail
		userID,            // CreatedBy / UpdatedBy
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
	userID := int(ctx.Locals("userID").(float64))

	sqlUpdate := `UPDATE inbound_headers SET status = 'complete' , updated_by = ?, updated_at = ?, complete_at = ?, complete_by = ? WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate, userID, time.Now(), time.Now(), userID, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
