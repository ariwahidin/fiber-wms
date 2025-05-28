package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
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
	ID          int           `json:"ID"`
	InboundNo   string        `json:"inbound_no"`
	InboundDate string        `json:"inbound_date"`
	Supplier    string        `json:"supplier"`
	PONumber    string        `json:"po_number"`
	Mode        string        `json:"mode"`
	Status      string        `json:"status"`
	Items       []InboundItem `json:"items"`
}

type InboundItem struct {
	ID           int    `json:"ID"`
	InboundID    int    `json:"inbound_id"`
	ItemCode     string `json:"item_code"`
	Quantity     int    `json:"quantity"`
	WhsCode      string `json:"whs_code"`
	UOM          string `json:"uom"`
	ReceivedDate string `json:"received_date"`
	Remarks      string `json:"remarks"`
	Mode         string `json:"mode"`
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
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get supplier",
			"error":   err.Error(),
		})
	}
	// Insert ke inbounds

	InboundHeader.InboundNo = payload.InboundNo
	InboundHeader.InboundDate = payload.InboundDate
	InboundHeader.InvoiceNo = payload.PONumber
	InboundHeader.Supplier = payload.Supplier
	InboundHeader.SupplierId = int(supplier.ID)
	InboundHeader.PoDate = payload.InboundDate
	InboundHeader.PoNumber = payload.PONumber
	InboundHeader.Status = payload.Status
	InboundHeader.CreatedBy = userID
	InboundHeader.UpdatedBy = userID
	InboundHeader.Status = "open"

	res := tx.Create(&InboundHeader)

	if res.Error != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert inbound",
			"error":   res.Error.Error(),
		})
	}

	var inboundID int
	if res.RowsAffected == 1 {
		inboundID = int(InboundHeader.ID)
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
		InboundDetail.InboundNo = payload.InboundNo
		InboundDetail.InboundId = inboundID
		InboundDetail.ItemCode = item.ItemCode
		InboundDetail.ItemId = int(product.ID)
		InboundDetail.Barcode = product.Barcode
		InboundDetail.Uom = product.Uom
		InboundDetail.Quantity = item.Quantity
		InboundDetail.WhsCode = item.WhsCode
		InboundDetail.RecDate = item.ReceivedDate
		InboundDetail.Remarks = item.Remarks
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
	// InboundHeader.Status = payload.Status
	InboundHeader.UpdatedBy = userID

	if err := c.DB.Model(&models.InboundHeader{}).Where("id = ?", InboundHeader.ID).Updates(InboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Inbound"})
}

func (c *InboundController) GetAllListInbound(ctx *fiber.Ctx) error {

	inboundRepo := repositories.NewInboundRepository(c.DB)
	result, err := inboundRepo.GetAllInbound()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
	inbound_no := ctx.Params("inbound_no")

	var InboundHeader models.InboundHeader
	var resultInbound Inbound

	if err := c.DB.Debug().First(&InboundHeader, "inbound_no = ?", inbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resultInbound = Inbound{
		ID:          int(InboundHeader.ID),
		InboundNo:   InboundHeader.InboundNo,
		InboundDate: InboundHeader.InboundDate,
		Supplier:    InboundHeader.Supplier,
		PONumber:    InboundHeader.PoNumber,
		Status:      InboundHeader.Status,
	}

	var InboundDetails []models.InboundDetail
	if err := c.DB.Debug().Where("inbound_no = ?", inbound_no).Find(&InboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(InboundDetails) == 0 {
		resultInbound.Items = []InboundItem{} // No items found, return empty slice
	} else {

		for _, InboundDetail := range InboundDetails {
			resultInbound.Items = append(resultInbound.Items, InboundItem{
				ID:           int(InboundDetail.ID),
				InboundID:    int(InboundDetail.InboundId),
				ItemCode:     InboundDetail.ItemCode,
				Quantity:     InboundDetail.Quantity,
				UOM:          InboundDetail.Uom,
				WhsCode:      InboundDetail.WhsCode,
				ReceivedDate: InboundDetail.RecDate,
				Remarks:      InboundDetail.Remarks,
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": resultInbound})

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

	if isNew {
		// insert
		newItem := models.InboundDetail{
			InboundId: payload.InboundID,
			InboundNo: inbound.InboundNo,
			ItemCode:  payload.ItemCode,
			ItemId:    int(product.ID),
			Barcode:   product.Barcode,
			Quantity:  payload.Quantity,
			Uom:       payload.UOM,
			WhsCode:   payload.WhsCode,
			RecDate:   payload.ReceivedDate,
			Remarks:   payload.Remarks,
			CreatedBy: int(ctx.Locals("userID").(float64)),
		}
		if err := c.DB.Debug().Create(&newItem).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		payload.ID = int(newItem.ID)
	} else {
		// update
		inboundDetail.InboundId = payload.InboundID
		inboundDetail.ItemCode = payload.ItemCode
		inboundDetail.ItemId = int(product.ID)
		inboundDetail.Barcode = product.Barcode
		inboundDetail.Quantity = payload.Quantity
		inboundDetail.Uom = payload.UOM
		inboundDetail.WhsCode = payload.WhsCode
		inboundDetail.RecDate = payload.ReceivedDate
		inboundDetail.Remarks = payload.Remarks
		inboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Debug().Save(&inboundDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	resultItem := InboundItem{
		ID:           payload.ID,
		InboundID:    payload.InboundID,
		ItemCode:     payload.ItemCode,
		Quantity:     payload.Quantity,
		UOM:          payload.UOM,
		WhsCode:      payload.WhsCode,
		ReceivedDate: payload.ReceivedDate,
		Remarks:      payload.Remarks,
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item saved successfully", "data": resultItem})
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
		ID:           int(inboundDetail.ID),
		InboundID:    inboundDetail.InboundId,
		ItemCode:     inboundDetail.ItemCode,
		Quantity:     inboundDetail.Quantity,
		UOM:          inboundDetail.Uom,
		WhsCode:      inboundDetail.WhsCode,
		ReceivedDate: inboundDetail.RecDate,
		Remarks:      inboundDetail.Remarks,
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

func (c *InboundController) ProcessingInboundComplete(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var inboundHeader models.InboundHeader

	// cari inbound header
	sql := `select a.* from inbound_headers a
	inner join inbound_details b on a.id = b.inbound_id
	where b.id = ? and a.status = 'open'
	`
	if err := c.DB.Raw(sql, id).Scan(&inboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// if err := c.DB.Debug().First(&inboundHeader, id).Error; err != nil {
	// 	if errors.Is(err, gorm.ErrRecordNotFound) {
	// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
	// 	}
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// type InboundBarcodeWithDetail struct {
	// 	InboundBarcode models.InboundBarcode `gorm:"embedded"`
	// 	RecDate        string                `json:"rec_date" gorm:"column:rec_date"`
	// }

	// var inboundBarcodes []InboundBarcodeWithDetail

	// sql := `select a.*, b.rec_date from inbound_barcodes a
	// inner join inbound_details b on a.inbound_detail_id = b.id
	// where b.inbound_id = ?
	// and a.status = 'pending'
	// `

	// if err := c.DB.Raw(sql, id).Scan(&inboundBarcodes).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// if len(inboundBarcodes) == 0 {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound not found"})
	// }

	type CheckResult struct {
		InboundDetailId int `json:"inbound_detail_id"`
		ItemId          int `json:"item_id"`
		Quantity        int `json:"quantity"`
		QtyScan         int `json:"qty_scan"`
	}

	sqlCheck := `WITH ib AS
	(
		SELECT inbound_id, inbound_detail_id, item_id, SUM(quantity) AS qty_scan, status 
		FROM inbound_barcodes WHERE inbound_id = ? AND status = 'in stock'
		GROUP BY inbound_id, inbound_detail_id, item_id, status
	)


	SELECT a.id, a.inbound_id, a.item_id, a.quantity, COALESCE(ib.qty_scan, 0) AS qty_scan
	FROM inbound_details a
	LEFT JOIN ib ON a.id = ib.inbound_detail_id
	WHERE a.inbound_id = ?`

	var checkResult []CheckResult
	if err := c.DB.Raw(sqlCheck, id, id).Scan(&checkResult).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, result := range checkResult {
		if result.Quantity != result.QtyScan {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Putaway not complete"})
		}
	}

	// update inbound status inbound header with interface

	userID := int(ctx.Locals("userID").(float64))

	sqlUpdate := `UPDATE inbound_headers SET status = 'complete' , updated_by = ?, updated_at = ? WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate, userID, time.Now(), id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway complete"})
}
