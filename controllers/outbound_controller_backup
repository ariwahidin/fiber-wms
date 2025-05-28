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

type OutboundController struct {
	DB *gorm.DB
}

type ReqHeaderOutbound struct {
	OutboundID   int    `json:"outbound_id"`
	OutboundNo   string `json:"outbound_no"`
	OutboundDate string `json:"outbound_date" validate:"required"`
	CustomerCode string `json:"customer_code" validate:"required,min=3"`
	DeliveryNo   string `json:"delivery_no" validate:"required"`
}

type ReqItemOutbound struct {
	OutboundDetailID int    `json:"outbound_detail_id"`
	OutboundID       int    `json:"outbound_id" validate:"required"`
	ItemCode         string `json:"item_code" validate:"required,min=3"`
	ItemName         string `json:"item_name"`
	Barcode          string `json:"barcode"`
	Quantity         int    `json:"quantity" validate:"required"`
	HandlingID       int    `json:"handling_id" validate:"required"`
	WhsCode          string `json:"whs_code" validate:"required"`
	OwnerCode        string `json:"owner_code"`
	Uom              string `json:"uom" validate:"required"`
	Remarks          string `json:"remarks"`
}

type FormSubmit struct {
	FormHeader ReqHeaderOutbound `json:"form_header"`
	FormItems  ReqItemOutbound   `json:"form_items"`
}

func NewOutboundController(DB *gorm.DB) *OutboundController {
	return &OutboundController{DB: DB}
}

func (c *OutboundController) CreateOutbound(ctx *fiber.Ctx) error {

	var customers []models.Customer
	if err := c.DB.Find(&customers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var form_header ReqHeaderOutbound

	form_header.OutboundDate = time.Now().Format("2006-01-02")

	var form_items ReqItemOutbound

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customers found", "data": fiber.Map{
		"form_header": form_header,
		"form_items":  form_items,
	}})
}

func (c *OutboundController) SaveOutbound(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var formHeader ReqHeaderOutbound
	if err := ctx.BodyParser(&formHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// validate formHeader
	validate := validator.New()
	if err := validate.Struct(formHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var header models.OutboundHeader
	if err := c.DB.Where("id = ?", id).First(&header).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	header.OutboundNo = formHeader.OutboundNo
	header.OutboundDate = formHeader.OutboundDate
	header.CustomerCode = formHeader.CustomerCode
	if header.Status == "draft" {
		header.Status = "open"
	}
	header.DeliveryNo = formHeader.DeliveryNo
	header.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Save(&header).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound saved successfully"})
}

func (c *OutboundController) CreateItemOutbound(ctx *fiber.Ctx) error {

	var FormSubmit FormSubmit

	if err := ctx.BodyParser(&FormSubmit); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// validate formHeader
	validate := validator.New()
	if err := validate.Struct(FormSubmit.FormHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	handlingRepo := repositories.NewHandlingRepository(c.DB)

	ReqHeaderOutbound := FormSubmit.FormHeader
	var header models.OutboundHeader

	if ReqHeaderOutbound.OutboundID != 0 {
		if err := c.DB.Where("id = ?", ReqHeaderOutbound.OutboundID).First(&header).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	header.OutboundDate = ReqHeaderOutbound.OutboundDate
	header.CustomerCode = ReqHeaderOutbound.CustomerCode
	header.DeliveryNo = ReqHeaderOutbound.DeliveryNo
	header.Status = "open"
	if header.ID == 0 {
		header.OutboundNo, _ = outboundRepo.GenerateOutboundNumber()
		header.CreatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Create(&header).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		header.UpdatedBy = int(ctx.Locals("userID").(float64))
		if err := c.DB.Save(&header).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	ReqItemOutbound := FormSubmit.FormItems

	if ReqItemOutbound.ItemCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item code is required"})
	}

	if ReqItemOutbound.Quantity <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity must be greater than 0"})
	}

	var product models.Product
	err := c.DB.Where("item_code = ?", ReqItemOutbound.ItemCode).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var handling models.Handling
	err = c.DB.Where("id = ?", ReqItemOutbound.HandlingID).First(&handling).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var handlingUsed []repositories.HandlingDetailUsed

	result, err := handlingRepo.GetHandlingUsed(ReqItemOutbound.HandlingID)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, handling := range result {
		handlingUsed = append(handlingUsed, repositories.HandlingDetailUsed{
			HandlingID:        handling.HandlingID,
			HandlingUsed:      handling.HandlingUsed,
			OriginHandlingID:  handling.OriginHandlingID,
			OriginHandling:    handling.OriginHandling,
			HandlingCombineID: handling.HandlingCombineID,
			RateID:            handling.RateID,
			RateIDR:           handling.RateIDR,
		})
	}

	var outboundDetail models.OutboundDetail

	outboundDetail.ID = uint(ReqItemOutbound.OutboundDetailID)
	outboundDetail.OutboundID = int(header.ID)
	outboundDetail.OutboundNo = header.OutboundNo
	outboundDetail.ItemID = int(product.ID)
	outboundDetail.ItemCode = product.ItemCode
	outboundDetail.Barcode = product.Barcode
	outboundDetail.Quantity = ReqItemOutbound.Quantity
	outboundDetail.HandlingId = int(handling.ID)
	outboundDetail.HandlingUsed = handling.Name
	outboundDetail.WhsCode = ReqItemOutbound.WhsCode
	outboundDetail.OwnerCode = ReqItemOutbound.OwnerCode
	outboundDetail.Uom = ReqItemOutbound.Uom
	outboundDetail.Remarks = ReqItemOutbound.Remarks
	outboundDetail.Status = "open"
	outboundDetail.CreatedBy = int(ctx.Locals("userID").(float64))

	outboundDetailID, err := outboundRepo.CreateItemOutbound(&header, &outboundDetail, handlingUsed)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error s": err.Error()})
	}

	fmt.Println("Item Outbound : ", ReqItemOutbound)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item added to inbound successfully", "data": fiber.Map{"header": header, "detail": outboundDetailID}})
}

func (c *OutboundController) GetOutboundByID(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&outboundHeader, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundDetails []models.OutboundDetail
	if err := c.DB.Debug().Where("outbound_id = ?", id).Find(&outboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var formHeader ReqHeaderOutbound
	formHeader.OutboundID = int(outboundHeader.ID)
	formHeader.OutboundNo = outboundHeader.OutboundNo
	formHeader.CustomerCode = outboundHeader.CustomerCode
	formHeader.DeliveryNo = outboundHeader.DeliveryNo
	parsedTime, err := time.Parse(time.RFC3339, outboundHeader.OutboundDate)
	if err != nil {
		fmt.Println("Error parsing time:", err)
	} else {
		formHeader.OutboundDate = parsedTime.Format("2006-01-02")
	}

	var formItems ReqItemOutbound
	formItems.OutboundID = int(outboundHeader.ID)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound found", "data": fiber.Map{"form_header": formHeader, "form_items": formItems, "header": outboundHeader, "details": outboundDetails}})
}

func (c *OutboundController) DeleteItemOutbound(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	sqlDelete := `DELETE FROM outbound_details WHERE id = ?`
	if err := c.DB.Exec(sqlDelete, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

func (c *OutboundController) GetOutboundDraft(ctx *fiber.Ctx) error {

	var outboundHeaders []models.OutboundHeader
	var outboundDetails []models.OutboundDetail

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound found", "data": fiber.Map{"headers": outboundHeaders, "details": outboundDetails}})
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
				"error": "Item not found",
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
