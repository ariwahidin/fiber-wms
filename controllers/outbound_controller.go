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

	fmt.Println("Payload Data Item Outbound : ", string(ctx.Body()))
	// return nil

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
	outboundDetail.Quantity = ReqItemOutbound.Quantity
	outboundDetail.HandlingId = int(handling.ID)
	outboundDetail.HandlingUsed = handling.Name
	outboundDetail.WhsCode = ReqItemOutbound.WhsCode
	outboundDetail.OwnerCode = ReqItemOutbound.OwnerCode
	outboundDetail.Uom = ReqItemOutbound.Uom
	outboundDetail.Remarks = ReqItemOutbound.Remarks
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

	var outboundDetails []models.OutboundDetail
	if err := c.DB.Debug().Where("outbound_id = ?", id).Find(&outboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	inventoryRepo := repositories.NewInventoryRepository(tx)

	for _, outboundDetail := range outboundDetails {

		stocks, err := inventoryRepo.GetStockByRequest(id)

		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		qtyReq := outboundDetail.Quantity
		itemID := outboundDetail.ItemID
		stockFound := false
		sisaRequest := 0

		for _, stock := range stocks {

			if qtyReq <= 0 {
				break
			}

			if itemID == stock.ItemID {

				if stock.Available <= 0 {
					continue
				}

				qtyPick := qtyReq
				if qtyReq > stock.Available {
					qtyPick = stock.Available
				}

				pickingSheet := models.PickingSheet{
					InventoryID:      stock.InventoryID,
					OutboundId:       id,
					OutboundDetailId: int(outboundDetail.ID),
					ItemID:           stock.ItemID,
					ItemCode:         stock.ItemCode,
					Location:         stock.Location,
					WhsCode:          stock.WhsCode,
					QaStatus:         stock.QaStatus,
					Quantity:         qtyPick,
					CreatedBy:        int(ctx.Locals("userID").(float64)),
				}

				if err := tx.Create(&pickingSheet).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create picking sheet: " + err.Error()})
				}
				stockFound = true
				qtyReq -= qtyPick
				sisaRequest = qtyReq
			}
		}

		if sisaRequest > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Stock not enough for item " + outboundDetail.ItemCode})
		}

		if !stockFound {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No available stock for item " + outboundDetail.ItemCode})
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
