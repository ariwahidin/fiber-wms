package controllers

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
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
	ID            int                       `json:"ID"`
	InboundNo     string                    `json:"inbound_no"`
	InboundDate   string                    `json:"inbound_date"`
	Supplier      string                    `json:"supplier"`
	PONumber      string                    `json:"po_number"`
	Mode          string                    `json:"mode"`
	Type          string                    `json:"type"`
	Invoice       string                    `json:"invoice"`
	Remarks       string                    `json:"remarks"`
	Status        string                    `json:"status"`
	Transporter   string                    `json:"transporter"`
	NoTruck       string                    `json:"no_truck"`
	Driver        string                    `json:"driver"`
	Container     string                    `json:"container"`
	References    []models.InboundReference `json:"references"`
	Items         []InboundItem             `json:"items"`
	ReceivedItems []ItemInboundBarcode      `json:"received_items"`
}

type InboundItem struct {
	ID        int    `json:"ID"`
	InboundID int    `json:"inbound_id"`
	ItemCode  string `json:"item_code"`
	Quantity  int    `json:"quantity"`
	WhsCode   string `json:"whs_code"`
	UOM       string `json:"uom"`
	RecDate   string `json:"rec_date"`
	Remarks   string `json:"remarks"`
	IsSerial  string `json:"is_serial"`
	Mode      string `json:"mode"`
	RefId     int    `json:"ref_id"`
	RefNo     string `json:"ref_no"`
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

	// Parse JSON payload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
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
	InboundHeader.Invoice = payload.Invoice
	InboundHeader.Supplier = payload.Supplier
	InboundHeader.SupplierId = int(supplier.ID)
	InboundHeader.PoDate = payload.InboundDate
	InboundHeader.PoNumber = payload.PONumber
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

	res := tx.Create(&InboundHeader)

	if res.Error != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert inbound",
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
		InboundDetail.InboundId = inboundID
		InboundDetail.ItemCode = item.ItemCode
		InboundDetail.ItemId = int(product.ID)
		InboundDetail.Barcode = product.Barcode
		InboundDetail.Uom = product.Uom
		InboundDetail.Quantity = item.Quantity
		InboundDetail.WhsCode = item.WhsCode
		InboundDetail.RecDate = item.RecDate
		InboundDetail.Remarks = item.Remarks
		InboundDetail.IsSerial = product.HasSerial
		InboundDetail.RefId = int(InboundReference.ID)
		InboundDetail.RefNo = item.RefNo
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

	// fmt.Println("Payload Data : ", payload)
	// return nil

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
	InboundHeader.PoDate = payload.InboundDate
	InboundHeader.PoNumber = payload.PONumber
	InboundHeader.Invoice = payload.Invoice
	InboundHeader.Type = payload.Type
	InboundHeader.Remarks = payload.Remarks
	InboundHeader.UpdatedBy = userID
	InboundHeader.Transporter = payload.Transporter
	InboundHeader.NoTruck = payload.NoTruck
	InboundHeader.Driver = payload.Driver
	InboundHeader.Container = payload.Container

	if err := c.DB.Model(&models.InboundHeader{}).Where("id = ?", InboundHeader.ID).Updates(InboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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

	var InboundDetails []models.InboundDetail

	if err := c.DB.Debug().Where("inbound_id = ?", InboundHeader.ID).Find(&InboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, item := range InboundDetails {

		var InboundReference models.InboundReference

		// if err := c.DB.Debug().First(&InboundReference, "id = ?", item.RefId).Error; err != nil {
		// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		// }

		err := c.DB.Debug().First(&InboundReference, "id = ?", item.RefId).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if item.RefId == 0 {
			// item.InboundId = InboundHeader.ID
			// item.RefId = InboundReference.ID
			// if err := c.DB.Create(&item).Error; err != nil {
			// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			// }
		} else {
			if err := c.DB.Model(&models.InboundDetail{}).Where("ref_id = ?", InboundReference.ID).
				Updates(map[string]interface{}{"ref_no": InboundReference.RefNo}).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Inbound"})
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")

	var InboundHeader models.InboundHeader

	if err := c.DB.Debug().
		Preload("InboundReferences").
		Preload("Details").
		Preload("Received").
		First(&InboundHeader, "inbound_no = ?", inbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": InboundHeader})
}

func (c *InboundController) SaveItem(ctx *fiber.Ctx) error {

	var payload InboundItem
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inbound models.InboundHeader
	if err := c.DB.Debug().First(&inbound, "id = ?", payload.InboundID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if inbound.Status != "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound not open", "message": "Inbound not open"})
	}

	var product models.Product
	if err := c.DB.Debug().First(&product, "item_code = ?", payload.ItemCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundDetail models.InboundDetail
	isNew := true
	if payload.ID > 0 {
		err := c.DB.Debug().First(&inboundDetail, "id = ?", payload.ID).Error
		if err == nil {
			isNew = false
			// lanjut update
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var InboundReference models.InboundReference

	// if err := c.DB.Debug().First(&InboundReference, "id = ?", payload.RefId).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error s": err.Error()})
	// }

	err := c.DB.Debug().First(&InboundReference, "id = ?", payload.RefId).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if InboundReference.ID > 0 {
		payload.RefId = int(InboundReference.ID)
		payload.RefNo = InboundReference.RefNo
	} else {
		// insert inbound reference
		newInboundReference := models.InboundReference{
			InboundId: uint(payload.InboundID),
			RefNo:     payload.RefNo,
		}

		if err := c.DB.Debug().Create(&newInboundReference).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		payload.RefId = int(newInboundReference.ID)
	}

	if isNew {
		// insert
		newItem := models.InboundDetail{
			InboundId: uint(payload.InboundID),
			InboundNo: inbound.InboundNo,
			ItemCode:  payload.ItemCode,
			ItemId:    int(product.ID),
			Barcode:   product.Barcode,
			Quantity:  payload.Quantity,
			Uom:       payload.UOM,
			WhsCode:   payload.WhsCode,
			RecDate:   payload.RecDate,
			Remarks:   payload.Remarks,
			IsSerial:  product.HasSerial,
			RefId:     payload.RefId,
			RefNo:     payload.RefNo,
			CreatedBy: int(ctx.Locals("userID").(float64)),
		}
		if err := c.DB.Debug().Create(&newItem).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		payload.ID = int(newItem.ID)
	} else {

		fmt.Println("update inbound detail", payload)

		// update
		inboundDetail.InboundId = uint(payload.InboundID)
		inboundDetail.ItemCode = payload.ItemCode
		inboundDetail.ItemId = int(product.ID)
		inboundDetail.Barcode = product.Barcode
		inboundDetail.Quantity = payload.Quantity
		inboundDetail.Uom = payload.UOM
		inboundDetail.WhsCode = payload.WhsCode
		inboundDetail.RecDate = payload.RecDate
		inboundDetail.Remarks = payload.Remarks
		inboundDetail.IsSerial = product.HasSerial
		inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Debug().Save(&inboundDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item saved successfully"})

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
		ID:        int(inboundDetail.ID),
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
		InboundDate  string `json:"inbound_date"`
		PoNumber     string `json:"po_number"`
		InboundNo    string `json:"inbound_no"`
		ItemCode     string `json:"item_code"`
		Barcode      string `json:"barcode"`
		SupplierName string `json:"supplier_name"`
		Quantity     int    `json:"quantity"`
	}

	sql := `SELECT b.inbound_date, b.po_number, b.inbound_no, 
	a.item_code, a.barcode,
	s.supplier_name, a.quantity
	FROM inbound_details a
	INNER JOIN inbound_headers b ON a.inbound_id = b.id
	LEFT JOIN suppliers s ON b.supplier_id = s.id
	WHERE inbound_id = ?`

	var putawaySheet []PutawaySheet
	if err := c.DB.Raw(sql, id).Scan(&putawaySheet).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway Sheet Found", "data": putawaySheet})
}

func (c *InboundController) PutawayPerItem(ctx *fiber.Ctx) error {
	idStr := ctx.Params("id")

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

	id := 0
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

	if qtyRequest != qtyReceived {
		//  update status to partially received
		if err := c.DB.Debug().Model(&models.InboundHeader{}).
			Where("id = ?", inboundHeaderID).
			Updates(map[string]interface{}{"status": "partially received"}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		errHistory := helpers.InsertTransactionHistory(
			c.DB,
			inboundHeader.InboundNo,             // RefNo
			"partially received",                // Status
			"INBOUND",                           // Type
			"",                                  // Detail
			int(ctx.Locals("userID").(float64)), // CreatedBy / UpdatedBy
		)
		if errHistory != nil {
			log.Println("Gagal insert history:", errHistory)
		}

	} else {
		// update status to fully received
		if err := c.DB.Debug().Model(&models.InboundHeader{}).
			Where("id = ?", inboundHeaderID).
			Updates(map[string]interface{}{"status": "fully received"}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		errHistory := helpers.InsertTransactionHistory(
			c.DB,
			inboundHeader.InboundNo,             // RefNo
			"fully received",                    // Status
			"INBOUND",                           // Type
			"",                                  // Detail
			int(ctx.Locals("userID").(float64)), // CreatedBy / UpdatedBy
		)
		if errHistory != nil {
			log.Println("Gagal insert history:", errHistory)
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway per item"})
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

	// update inbound status inbound header with interface
	sqlUpdate := `UPDATE inbound_headers SET status = 'checking', updated_at = ?, updated_by = ? WHERE inbound_no = ?`
	if err := r.DB.Exec(sqlUpdate, time.Now(), int(ctx.Locals("userID").(float64)), payload.InboundNo).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	errHistory := helpers.InsertTransactionHistory(
		r.DB,
		payload.InboundNo,                   // RefNo
		"checking",                          // Status
		"INBOUND",                           // Type
		"",                                  // Detail
		int(ctx.Locals("userID").(float64)), // CreatedBy / UpdatedBy
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change status inbound " + payload.InboundNo + " to checking successfully"})
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
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Error", "message": "Cannot change status to open, inbound " + payload.InboundNo + ", item " + result.ItemCode + " has been scanned"})
		}
	}

	// update inbound status inbound header with interface
	sqlUpdate := `UPDATE inbound_headers SET status = 'open', updated_at = ?, updated_by = ? WHERE inbound_no = ?`
	if err := r.DB.Exec(sqlUpdate, time.Now(), int(ctx.Locals("userID").(float64)), payload.InboundNo).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
func (c *InboundController) ProcessingInboundComplete(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var inboundHeader models.InboundHeader

	if err := c.DB.Debug().First(&inboundHeader, "id = ? AND status <> 'complete'", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

	sqlUpdate := `UPDATE inbound_headers SET status = 'complete' , updated_by = ?, updated_at = ? WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate, userID, time.Now(), id).Error; err != nil {
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
