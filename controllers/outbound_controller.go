package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type OutboundController struct {
	DB *gorm.DB
}

func NewOutboundController(DB *gorm.DB) *OutboundController {
	return &OutboundController{DB: DB}
}

type Outbound struct {
	ID           int            `json:"ID"`
	OutboundNo   string         `json:"outbound_no"`
	OutboundDate string         `json:"outbound_date"`
	Customer     string         `json:"customer"`
	DeliveryNo   string         `json:"delivery_no"`
	Mode         string         `json:"mode"`
	Status       string         `json:"status"`
	Items        []OutboundItem `json:"items"`
}

type OutboundItem struct {
	ID           int    `json:"ID"`
	OutboundID   int    `json:"outbound_id"`
	ItemCode     string `json:"item_code"`
	Quantity     int    `json:"quantity"`
	WhsCode      string `json:"whs_code"`
	UOM          string `json:"uom"`
	ReceivedDate string `json:"received_date"`
	Remarks      string `json:"remarks"`
	Mode         string `json:"mode"`
}

func (c *OutboundController) CreateOutbound(ctx *fiber.Ctx) error {
	var payload Outbound

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

	repositories := repositories.NewOutboundRepository(tx)

	inbound_no, err := repositories.GenerateOutboundNumber()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate inbound no",
			"error":   err.Error(),
		})
	}
	payload.OutboundNo = inbound_no
	payload.Status = "open"
	userID := int(ctx.Locals("userID").(float64))

	var OutboundHeader models.OutboundHeader

	var customer models.Customer

	if err := tx.Debug().First(&customer, "customer_code = ?", payload.Customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Customer not found",
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get customer",
			"error":   err.Error(),
		})
	}
	// Insert ke inbounds

	OutboundHeader.OutboundNo = payload.OutboundNo
	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.DeliveryNo = payload.DeliveryNo
	OutboundHeader.Customer = payload.Customer
	OutboundHeader.CustomerCode = customer.CustomerCode
	OutboundHeader.CustomerName = customer.CustomerName
	OutboundHeader.CustomerID = int(customer.ID)
	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.DeliveryNo = payload.DeliveryNo
	OutboundHeader.Status = payload.Status
	OutboundHeader.CreatedBy = userID
	OutboundHeader.UpdatedBy = userID
	OutboundHeader.Status = "open"

	res := tx.Create(&OutboundHeader)

	if res.Error != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert outbound header",
			"error":   res.Error.Error(),
		})
	}

	var outboundID int
	if res.RowsAffected == 1 {
		outboundID = int(OutboundHeader.ID)
	}

	// Insert ke outbound details
	for _, item := range payload.Items {

		var product models.Product

		if err := tx.Debug().First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var OutboundDetail models.OutboundDetail
		OutboundDetail.OutboundNo = payload.OutboundNo
		OutboundDetail.OutboundID = outboundID
		OutboundDetail.ItemCode = item.ItemCode
		OutboundDetail.ItemID = int(product.ID)
		OutboundDetail.Barcode = product.Barcode
		OutboundDetail.Uom = product.Uom
		OutboundDetail.Quantity = item.Quantity
		OutboundDetail.WhsCode = item.WhsCode
		// OutboundDetail. = item.ReceivedDate
		OutboundDetail.Remarks = item.Remarks
		OutboundDetail.CreatedBy = userID
		OutboundDetail.UpdatedBy = userID

		res := tx.Create(&OutboundDetail)

		if res.Error != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert outbound detail",
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
		"message": "Outbound created successfully",
		"data": fiber.Map{
			"outbound_id": outboundID,
		},
	})
}

func (c *OutboundController) GetOutboundList(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	rawOutboundList, err := outboundRepo.GetAllOutboundList()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Outbound found",
		"data":    rawOutboundList,
	})
}

func (c *OutboundController) GetOutboundByID(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var OutboundHeader models.OutboundHeader
	var resultOutbound Outbound

	if err := c.DB.Debug().First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resultOutbound = Outbound{
		ID:           int(OutboundHeader.ID),
		OutboundNo:   OutboundHeader.OutboundNo,
		OutboundDate: OutboundHeader.OutboundDate,
		Customer:     OutboundHeader.Customer,
		DeliveryNo:   OutboundHeader.DeliveryNo,
		Status:       OutboundHeader.Status,
	}

	var OutboundDetails []models.OutboundDetail
	if err := c.DB.Debug().Where("outbound_id = ?", OutboundHeader.ID).Find(&OutboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(OutboundDetails) == 0 {
		resultOutbound.Items = []OutboundItem{} // No items found, return empty slice
	} else {

		for _, OutboundDetail := range OutboundDetails {
			resultOutbound.Items = append(resultOutbound.Items, OutboundItem{
				ID:         int(OutboundDetail.ID),
				OutboundID: int(OutboundDetail.OutboundID),
				ItemCode:   OutboundDetail.ItemCode,
				Quantity:   OutboundDetail.Quantity,
				UOM:        OutboundDetail.Uom,
				WhsCode:    OutboundDetail.WhsCode,
				Remarks:    OutboundDetail.Remarks,
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": resultOutbound})

}

func (c *OutboundController) UpdateOutboundByID(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var payload Outbound

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))
	var OutboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var customer models.Customer
	if err := c.DB.Debug().First(&customer, "customer_code = ?", payload.Customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.Customer = payload.Customer
	OutboundHeader.CustomerID = int(customer.ID)
	OutboundHeader.CustomerCode = customer.CustomerCode
	OutboundHeader.CustomerName = customer.CustomerName
	OutboundHeader.DeliveryNo = payload.DeliveryNo
	OutboundHeader.UpdatedBy = userID

	if err := c.DB.Model(&models.OutboundHeader{}).Where("id = ?", OutboundHeader.ID).Updates(OutboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Outbound successfully", "data": OutboundHeader})
}

func (c *OutboundController) SaveItem(ctx *fiber.Ctx) error {

	var payload OutboundItem
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var outbound models.OutboundHeader
	if err := c.DB.Debug().First(&outbound, "id = ?", payload.OutboundID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
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

	var outboundDetail models.OutboundDetail
	isNew := true
	if payload.ID > 0 {
		err := c.DB.Debug().First(&outboundDetail, "id = ?", payload.ID).Error
		if err == nil {
			isNew = false
			// lanjut update
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if isNew {
		// insert
		newItem := models.OutboundDetail{
			OutboundID: payload.OutboundID,
			OutboundNo: outbound.OutboundNo,
			ItemCode:   payload.ItemCode,
			ItemID:     int(product.ID),
			Barcode:    product.Barcode,
			Quantity:   payload.Quantity,
			Uom:        payload.UOM,
			WhsCode:    payload.WhsCode,
			Remarks:    payload.Remarks,
			CreatedBy:  int(ctx.Locals("userID").(float64)),
		}
		if err := c.DB.Debug().Create(&newItem).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		payload.ID = int(newItem.ID)
	} else {
		// update
		outboundDetail.OutboundID = payload.OutboundID
		outboundDetail.ItemCode = payload.ItemCode
		outboundDetail.ItemID = int(product.ID)
		outboundDetail.Barcode = product.Barcode
		outboundDetail.Quantity = payload.Quantity
		outboundDetail.Uom = payload.UOM
		outboundDetail.WhsCode = payload.WhsCode
		outboundDetail.Remarks = payload.Remarks
		outboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Debug().Save(&outboundDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	resultItem := OutboundItem{
		ID:           payload.ID,
		OutboundID:   payload.OutboundID,
		ItemCode:     payload.ItemCode,
		Quantity:     payload.Quantity,
		UOM:          payload.UOM,
		WhsCode:      payload.WhsCode,
		ReceivedDate: payload.ReceivedDate,
		Remarks:      payload.Remarks,
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item saved successfully", "data": resultItem})
}

func (c *OutboundController) GetItem(ctx *fiber.Ctx) error {

	outbound_detail_id := ctx.Params("id")
	var outboundDetail models.OutboundDetail
	if err := c.DB.Debug().First(&outboundDetail, "id = ?", outbound_detail_id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resultItem := OutboundItem{
		ID:         int(outboundDetail.ID),
		OutboundID: outboundDetail.OutboundID,
		ItemCode:   outboundDetail.ItemCode,
		Quantity:   outboundDetail.Quantity,
		UOM:        outboundDetail.Uom,
		WhsCode:    outboundDetail.WhsCode,
		// ReceivedDate: outboundDetail.RecDate,
		Remarks: outboundDetail.Remarks,
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item found successfully", "data": resultItem})
}

func (c *OutboundController) DeleteItem(ctx *fiber.Ctx) error {

	outbound_detail_id := ctx.Params("id")
	var outboundDetail models.OutboundDetail
	if err := c.DB.Debug().First(&outboundDetail, "id = ?", outbound_detail_id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// hard delete
	if err := c.DB.Debug().Unscoped().Delete(&outboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

func (c *OutboundController) PickingOutbound(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var outboundDetails []models.OutboundDetail
	if err := tx.Debug().Where("outbound_id = ?", id).Find(&outboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, outboundDetail := range outboundDetails {

		qtyReq := outboundDetail.Quantity

		var inventories []models.Inventory

		fmt.Println("Picking Query")
		if err := tx.Debug().
			Where("item_id = ? AND whs_code = ? AND qty_available > 0", outboundDetail.ItemID, outboundDetail.WhsCode).
			Order("rec_date, pallet, location ASC").
			Find(&inventories).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if len(inventories) == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Item " + outboundDetail.ItemCode + " not found",
			})
		}

		for _, inventory := range inventories {

			if qtyReq < 1 {
				break
			}

			qtyPick := 0

			if inventory.QtyAvailable >= qtyReq {
				qtyPick = qtyReq
			} else {
				qtyPick = inventory.QtyAvailable
			}

			var product models.Product
			if err := tx.Debug().Where("id = ?", outboundDetail.ItemID).First(&product).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Product not found",
				})
			}

			// Insert picking sheet
			pickingSheet := models.PickingSheet{
				InventoryID:      int(inventory.ID),
				OutboundId:       outboundDetail.OutboundID,
				OutboundDetailId: int(outboundDetail.ID),
				ItemID:           outboundDetail.ItemID,
				Barcode:          product.Barcode,
				ItemCode:         product.ItemCode,
				SerialNumber:     inventory.SerialNumber,
				Pallet:           inventory.Pallet,
				Location:         inventory.Location,
				QtyOnhand:        qtyPick,
				QtyAvailable:     qtyPick,
				WhsCode:          inventory.WhsCode,
				QaStatus:         inventory.QaStatus,
				Status:           "pending",
				IsSuggestion:     "Y",
				CreatedBy:        int(ctx.Locals("userID").(float64)),
			}

			if err := tx.Create(&pickingSheet).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to create picking sheet",
				})
			}

			// Update Inventory
			if err := tx.Debug().
				Model(&models.Inventory{}).
				Where("id = ?", inventory.ID).
				Updates(map[string]interface{}{
					"qty_available": gorm.Expr("qty_available - ?", qtyPick),
					"qty_allocated": gorm.Expr("qty_allocated + ?", qtyPick),
					"updated_by":    int(ctx.Locals("userID").(float64)),
					"updated_at":    time.Now(),
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to update inventory",
				})
			}

			qtyReq -= qtyPick

		}

		if qtyReq > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Insufficient stock for item",
			})
		}
	}

	// update outbound status
	var outboundHeader models.OutboundHeader
	if err := tx.Where("id = ?", id).First(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get outbound header: " + err.Error()})
	}

	outboundHeader.Status = "picking"
	outboundHeader.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := tx.Save(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update outbound header: " + err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Picking Outbound Success"})
}

func (c *OutboundController) GetPickingSheet(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var pickingSheets []repositories.PaperPickingSheet
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	pickingSheets, err = outboundRepo.GetPickingSheet(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Picking Sheet Found", "data": pickingSheets})
}

func (c *OutboundController) PickingComplete(ctx *fiber.Ctx) error {

	fmt.Println("Picking Complete Proccess")

	type input struct {
		OutboundID int `json:"outbound_id" validate:"required"`
	}

	var inputBody input
	if err := ctx.BodyParser(&inputBody); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var outboundDetails []models.OutboundDetail
	if err := tx.Debug().Where("outbound_id = ?", inputBody.OutboundID).Find(&outboundDetails).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, outboundDetail := range outboundDetails {
		if outboundDetail.Quantity != outboundDetail.ScanQty {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Picking not complete"})
		}
	}

	var pickingSheets []models.PickingSheet
	if err := tx.Debug().Where("outbound_id = ?", inputBody.OutboundID).Find(&pickingSheets).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, pickingSheet := range pickingSheets {
		if pickingSheet.QtyAvailable > 0 {
			// update inventory
			if err := tx.Debug().
				Model(&models.Inventory{}).
				Where("id = ?", pickingSheet.InventoryID).
				Updates(map[string]interface{}{
					"qty_available": gorm.Expr("qty_available + ?", pickingSheet.QtyAvailable),
					"qty_allocated": gorm.Expr("qty_allocated - ?", pickingSheet.QtyAvailable),
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	// UPDATE OUTBOUND STATUS
	if err := tx.Debug().
		Model(&models.OutboundHeader{}).
		Where("id = ?", inputBody.OutboundID).
		Updates(map[string]interface{}{
			"status":     "completed",
			"updated_by": int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundHeader models.OutboundHeader
	if err := tx.Where("id = ?", inputBody.OutboundID).First(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get outbound header: " + err.Error()})
	}

	// Create List Order Part
	for _, partOrder := range outboundDetails {

		var customer models.Customer
		if err := tx.Debug().Where("customer_code = ?", outboundHeader.CustomerCode).First(&customer).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get customer: " + err.Error()})
		}

		var product models.Product
		if err := tx.Where("id = ?", partOrder.ItemID).First(&product).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get product: " + err.Error()})
		}

		listOrderPart := models.ListOrderPart{
			OutboundID:       outboundHeader.ID,
			OutboundDetailID: partOrder.ID,
			DeliveryNumber:   outboundHeader.DeliveryNo,
			ItemID:           uint(partOrder.ItemID),
			ItemCode:         partOrder.ItemCode,
			ItemName:         product.ItemName,
			Qty:              partOrder.Quantity,
			CustomerID:       customer.ID,
			CustomerCode:     outboundHeader.CustomerCode,
			CustomerName:     customer.CustomerName,
			Volume:           float64(partOrder.Quantity) * float64(product.Kubikasi),
			CreatedBy:        int(ctx.Locals("userID").(float64)),
		}

		if err := tx.Create(&listOrderPart).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create list order part: " + err.Error()})
		}
	}

	var outboundBarcodes []models.OutboundBarcode
	if err := tx.Debug().Where("outbound_id = ?", inputBody.OutboundID).Find(&outboundBarcodes).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(outboundBarcodes) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound scanned not found"})
	}

	for _, outboundBarcode := range outboundBarcodes {
		// update inventory
		if err := tx.Debug().
			Model(&models.Inventory{}).
			Where("id = ?", outboundBarcode.InventoryID).
			Updates(map[string]interface{}{
				"qty_onhand":    gorm.Expr("qty_onhand - ?", outboundBarcode.Quantity),
				"qty_allocated": gorm.Expr("qty_allocated - ?", outboundBarcode.Quantity),
				"qty_shipped":   gorm.Expr("qty_shipped + ?", outboundBarcode.Quantity),
				"updated_by":    int(ctx.Locals("userID").(float64)),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Picking Outbound Success"})
}
