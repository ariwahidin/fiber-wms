package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type InboundController struct {
	DB *gorm.DB
}

type Handling struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

var InputPayload struct {
	InboundID    int    `json:"inbound_id"`
	ItemCode     string `json:"item_code"`
	Quantity     int    `json:"quantity"`
	UOM          string `json:"uom"`
	RecDate      string `json:"rec_date"`
	WhsCode      string `json:"whs_code"`
	HandlingID   int    `json:"handling_id"`
	HandlingUsed string `json:"handling_used"`
	Remarks      string `json:"remarks"`
	Location     string `json:"location"`
}

var itemDetailInbound struct {
	ItemCode     string `json:"item_code" validate:"required"`
	Quantity     int    `json:"quantity" validate:"required"`
	Uom          string `json:"uom" validate:"required"`
	WhsCode      string `json:"whs_code" validate:"required"`
	RecDate      string `json:"rec_date" validate:"required"`
	Remarks      string `json:"remarks" validate:"required"`
	HandlingId   int    `json:"handling_id" validate:"required"`
	HandlingUsed string `json:"handling_used" validate:"required"`
	Location     string `json:"location" validate:"required"`
}

type FormHeaderInbound struct {
	InboundID       int    `json:"inbound_id"`
	InboundNo       string `json:"inbound_no"`
	SupplierID      int    `json:"supplier_id" validate:"required"`
	SupplierName    string `json:"supplier_name"`
	TransporterID   int    `json:"transporter_id"`
	Driver          string `json:"driver"`
	TruckID         int    `json:"truck_id"`
	TruckNo         string `json:"truck_no"`
	InboundDate     string `json:"inbound_date"`
	ContainerNo     string `json:"container_no"`
	BlNo            string `json:"bl_no"`
	PoNo            string `json:"po_no"`
	Invoice         string `json:"invoice"`
	PoDate          string `json:"po_date"`
	SjNo            string `json:"sj_no"`
	OriginID        int    `json:"origin_id"`
	Origin          string `json:"origin"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	RemarksHeader   string `json:"remarks_header"`
}

type Form struct {
	FormHeader FormHeaderInbound      `json:"form_header"`
	FormItem   models.FormItemInbound `json:"form_item"`
}

func NewInboundController(DB *gorm.DB) *InboundController {
	return &InboundController{DB: DB}
}

func (c *InboundController) PreapareInbound(ctx *fiber.Ctx) error {
	var formHeader FormHeaderInbound
	formHeader.InboundNo = "Auto Generate"
	formHeader.InboundDate = time.Now().Format("2006-01-02")
	formHeader.PoDate = time.Now().Format("2006-01-02")

	var formItem models.FormItemInbound
	formItem.Location = "STAGING"
	formItem.RecDate = time.Now().Format("2006-01-02")
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true,
		"data": fiber.Map{
			"form_header": formHeader,
			"form_item":   formItem,
		},
	})
}

func (c *InboundController) CreateOrUpdateItemInbound(ctx *fiber.Ctx) error {

	var Form Form

	if err := ctx.BodyParser(&Form); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validator := validator.New()
	if err := validator.Struct(Form); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	formHeader := Form.FormHeader
	formItem := Form.FormItem

	inboundRepo := repositories.NewInboundRepository(c.DB)

	var inboundHeader models.InboundHeader
	inboundHeader.SupplierId = formHeader.SupplierID
	inboundHeader.TransporterID = formHeader.TransporterID
	inboundHeader.TruckId = formHeader.TruckID
	inboundHeader.OriginId = formHeader.OriginID
	inboundHeader.InboundDate = formHeader.InboundDate
	inboundHeader.InvoiceNo = formHeader.Invoice
	inboundHeader.Driver = formHeader.Driver
	inboundHeader.ContainerNo = formHeader.ContainerNo
	inboundHeader.BlNo = formHeader.BlNo
	inboundHeader.PoDate = formHeader.PoDate
	inboundHeader.PoNo = formHeader.PoNo
	inboundHeader.Status = "open"
	inboundHeader.TruckNo = formHeader.TruckNo
	inboundHeader.TimeArrival = formHeader.TimeArrival
	inboundHeader.StartUnloading = formHeader.StartUnloading
	inboundHeader.FinishUnloading = formHeader.FinishUnloading
	inboundHeader.SjNo = formHeader.SjNo
	inboundHeader.Remarks = formHeader.RemarksHeader

	if formHeader.InboundID == 0 {
		inboundNo, _ := inboundRepo.GenerateInboundNo()
		inboundHeader.InboundNo = inboundNo
		inboundHeader.CreatedBy = int(ctx.Locals("userID").(float64))

		if err := c.DB.Create(&inboundHeader).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	} else {
		inboundHeader.ID = uint(formHeader.InboundID)
		inboundHeader.InboundNo = formHeader.InboundNo
		inboundHeader.UpdatedBy = int(ctx.Locals("userID").(float64))

		if err := c.DB.Model(&models.InboundHeader{}).Where("id = ?", inboundHeader.ID).Updates(inboundHeader).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var inboundDetail models.InboundDetail
	var handling models.Handling
	if err := c.DB.Debug().First(&handling, formItem.HandlingID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var product models.Product
	if err := c.DB.Debug().First(&product, formItem.ItemID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundDetail.ID = uint(formItem.InboundDetailID)
	inboundDetail.InboundNo = inboundHeader.InboundNo
	inboundDetail.HandlingId = int(handling.ID)
	inboundDetail.ItemId = formItem.ItemID
	inboundDetail.ItemCode = formItem.ItemCode
	inboundDetail.Barcode = product.Barcode
	inboundDetail.Quantity = formItem.Quantity
	inboundDetail.Location = formItem.Location
	inboundDetail.HandlingUsed = handling.Name
	inboundDetail.WhsCode = formItem.WhsCode
	inboundDetail.RecDate = formItem.RecDate
	inboundDetail.Uom = formItem.Uom
	inboundDetail.Remarks = formItem.Remarks
	inboundDetail.CreatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.InboundId = int(inboundHeader.ID)

	if inboundDetail.InboundId > 0 {
		if err := c.DB.Debug().Find(&inboundHeader, inboundDetail.InboundId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		inboundDetail.InboundNo = inboundHeader.InboundNo
		inboundDetail.Status = "open"
	}

	handlingRepo := repositories.NewHandlingRepository(c.DB)
	var handlingUsed []repositories.HandlingDetailUsed
	result, err := handlingRepo.GetHandlingUsed(inboundDetail.HandlingId)

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

	inboundDetailID, err := inboundRepo.CreateInboundDetail(&inboundDetail, handlingUsed)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error s": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item added to inbound successfully", "data": fiber.Map{
		"inbound_id":        inboundDetail.InboundId,
		"inbound_no":        inboundHeader.InboundNo,
		"inbound_detail_id": inboundDetailID,
	}})
}

func (c *InboundController) GetInboundDetailDraftByUserID(ctx *fiber.Ctx) error {

	userID := int(ctx.Locals("userID").(float64))

	var inboundDetails []repositories.DetailItem

	sqlSelect := `SELECT a.id, a.item_code, b.item_name, b.gmc, a.quantity, a.status, a.whs_code, a.rec_date, a.uom, a.remarks,
				c.name as handling_used, a.handling_id, a.location
				FROM inbound_details a
				INNER JOIN products b ON a.item_code = b.item_code 
				INNER JOIN handlings c ON a.handling_id = c.id
				WHERE a.created_by = ? AND a.status = 'draft'`

	if err := c.DB.Raw(sqlSelect, userID).Scan(&inboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound details found", "data": fiber.Map{"details": inboundDetails}})
}

func (c *InboundController) DeleteInboundDetail(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	userID := int(ctx.Locals("userID").(float64))

	var inboundDetail models.InboundDetail

	if err := c.DB.Debug().First(&inboundDetail, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound detail not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundHeader models.InboundHeader
	if err := c.DB.Debug().First(&inboundHeader, inboundDetail.InboundId).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound Header not found"})
	}

	if inboundHeader.Status == "closed" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already closed"})
	}

	sqlDelete := `DELETE FROM inbound_details WHERE id = ? AND created_by = ?`
	if err := c.DB.Exec(sqlDelete, id, userID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail deleted successfully"})
}

func (c *InboundController) SaveHeaderInbound(ctx *fiber.Ctx) error {

	fmt.Println("Payload Data Mentah Inbound : ", string(ctx.Body()))

	var Form Form

	if err := ctx.BodyParser(&Form); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validator := validator.New()
	if err := validator.Struct(Form.FormHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	formHeader := Form.FormHeader

	inboundRepo := repositories.NewInboundRepository(c.DB)

	var inboundHeader models.InboundHeader

	inboundHeader.SupplierId = formHeader.SupplierID
	inboundHeader.TransporterID = formHeader.TransporterID
	inboundHeader.TruckId = formHeader.TruckID
	inboundHeader.OriginId = formHeader.OriginID
	inboundHeader.InboundDate = formHeader.InboundDate
	inboundHeader.InvoiceNo = formHeader.Invoice
	inboundHeader.Driver = formHeader.Driver
	inboundHeader.ContainerNo = formHeader.ContainerNo
	inboundHeader.BlNo = formHeader.BlNo
	inboundHeader.PoDate = formHeader.PoDate
	inboundHeader.PoNo = formHeader.PoNo
	inboundHeader.Status = "open"
	inboundHeader.TruckNo = formHeader.TruckNo
	inboundHeader.TimeArrival = formHeader.TimeArrival
	inboundHeader.StartUnloading = formHeader.StartUnloading
	inboundHeader.FinishUnloading = formHeader.FinishUnloading
	inboundHeader.SjNo = formHeader.SjNo
	inboundHeader.Remarks = formHeader.RemarksHeader

	if formHeader.InboundID == 0 {
		inboundNo, _ := inboundRepo.GenerateInboundNo()
		inboundHeader.InboundNo = inboundNo
		inboundHeader.CreatedBy = int(ctx.Locals("userID").(float64))

		if err := c.DB.Create(&inboundHeader).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	} else {
		inboundHeader.ID = uint(formHeader.InboundID)
		inboundHeader.InboundNo = formHeader.InboundNo
		inboundHeader.UpdatedBy = int(ctx.Locals("userID").(float64))

		if err := c.DB.Model(&models.InboundHeader{}).Where("id = ?", inboundHeader.ID).Updates(inboundHeader).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound header saved successfully", "data": inboundHeader})
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
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// var formHeader FormHeaderInbound

	inboundHeader, err := repositories.NewInboundRepository(c.DB).GetInboundHeaderByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	formHeader, err := repositories.NewInboundRepository(c.DB).GetInboundHeaderByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	formHeader.InboundDate = func() string {
		t, _ := time.Parse(time.RFC3339, formHeader.InboundDate)
		return t.Format("2006-01-02")
	}()

	formHeader.PoDate = func() string { t, _ := time.Parse(time.RFC3339, formHeader.PoDate); return t.Format("2006-01-02") }()

	var formItem models.FormItemInbound

	inboundHeader.InboundDate = func() string {
		t, _ := time.Parse(time.RFC3339, inboundHeader.InboundDate)
		return t.Format("2006-01-02")
	}()
	inboundHeader.PoDate = func() string { t, _ := time.Parse(time.RFC3339, inboundHeader.PoDate); return t.Format("2006-01-02") }()

	detailItem, err := repositories.NewInboundRepository(c.DB).GetDetailItemByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{
		"form_header": formHeader,
		"form_item":   formItem,
		"header":      inboundHeader,
		"details":     detailItem},
	})
}

func (c *InboundController) UpdateDetailByID(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var inboundDetail models.InboundDetail

	if err := ctx.BodyParser(&inboundDetail); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Get Handling From DB Using ID
	var handling models.Handling
	if err := c.DB.Debug().First(&handling, inboundDetail.HandlingId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundDetail.HandlingUsed = handling.Name
	inboundDetail.CreatedBy = int(ctx.Locals("userID").(float64))
	inboundDetail.UpdatedAt = time.Now()

	var product models.Product
	if err := c.DB.Debug().Where("item_code = ?", inboundDetail.ItemCode).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(inboundDetail); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	handlingRepo := repositories.NewHandlingRepository(c.DB)

	var handlingUsed []repositories.HandlingDetailUsed

	result, err := handlingRepo.GetHandlingUsed(inboundDetail.HandlingId)

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

	sqlUpdate := `UPDATE inbound_details 
	SET
	item_id = ?, 
	item_code = ?, 
	quantity = ?, 
	uom = ?, 
	whs_code = ?, 
	rec_date = ?, 
	updated_by = ?, 
	updated_at = ?, 
	handling_id = ?, 
	handling_used = ?, 
	remarks = ?, 
	location = ? 
	WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate,
		int(product.ID),
		inboundDetail.ItemCode,
		inboundDetail.Quantity,
		inboundDetail.Uom,
		inboundDetail.WhsCode,
		inboundDetail.RecDate,
		inboundDetail.CreatedBy,
		inboundDetail.UpdatedAt,
		inboundDetail.HandlingId,
		inboundDetail.HandlingUsed,
		inboundDetail.Remarks,
		inboundDetail.Location,
		id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	sqlDelete := `DELETE FROM inbound_detail_handlings WHERE inbound_detail_id = ?`
	if err := c.DB.Exec(sqlDelete, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, handling := range handlingUsed {
		inboundDetailHandling := models.InboundDetailHandling{
			InboundDetailId:   int(id),
			HandlingId:        handling.HandlingID,
			HandlingUsed:      handling.HandlingUsed,
			HandlingCombineId: handling.HandlingCombineID,
			OriginHandlingId:  handling.OriginHandlingID,
			OriginHandling:    handling.OriginHandling,
			RateId:            handling.RateID,
			RateIdr:           handling.RateIDR,
			CreatedBy:         inboundDetail.CreatedBy,
		}

		if err := c.DB.Create(&inboundDetailHandling).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail updated successfully", "data": itemDetailInbound})
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

	// for _, inboundBarcode := range inboundBarcodes {
	// 	inventory := models.Inventory{
	// 		InboundDetailId:  inboundBarcode.InboundBarcode.InboundDetailId,
	// 		InboundBarcodeId: int(inboundBarcode.InboundBarcode.ID),
	// 		ItemId:           inboundBarcode.InboundBarcode.ItemID,
	// 		ItemCode:         inboundBarcode.InboundBarcode.ItemCode,
	// 		Barcode:          inboundBarcode.InboundBarcode.Barcode,
	// 		WhsCode:          inboundBarcode.InboundBarcode.WhsCode,
	// 		QtyOrigin:        inboundBarcode.InboundBarcode.Quantity,
	// 		QtyOnhand:        inboundBarcode.InboundBarcode.Quantity,
	// 		Trans:            "inbound",
	// 		RecDate:          inboundBarcode.RecDate,
	// 		Location:         inboundBarcode.InboundBarcode.Location,
	// 		QaStatus:         inboundBarcode.InboundBarcode.QaStatus,
	// 		SerialNumber:     inboundBarcode.InboundBarcode.SerialNumber,
	// 		CreatedBy:        int(ctx.Locals("userID").(float64)),
	// 	}

	// 	if err := c.DB.Create(&inventory).Error; err != nil {
	// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// 	}

	// 	err := c.DB.Model(&models.InboundBarcode{}).Where("id = ?", inboundBarcode.InboundBarcode.ID).Updates(map[string]interface{}{
	// 		"status":     "in stock",
	// 		"updated_by": ctx.Locals("userID").(float64),
	// 	}).Error

	// 	if err != nil {
	// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// 	}
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Putaway complete"})
}

func (c *InboundController) UpdateInboundByID(ctx *fiber.Ctx) error {

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	// Check if the inbound header exists
	var inboundHeader models.InboundHeader
	if err := c.DB.Debug().First(&inboundHeader, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inputHeader repositories.HeaderInbound
	// Parse Body
	if err := ctx.BodyParser(&inputHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(inputHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Membuat user dengan memasukkan data ke struct models.User
	userID := int(ctx.Locals("userID").(float64))
	inboundHeader.UpdatedBy = userID

	if err := c.DB.Debug().Model(&inboundHeader).Updates(inputHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail added successfully", "data": inputHeader})
}

func (c *InboundController) UploadInboundFromExcel(ctx *fiber.Ctx) error {
	type InboundExcel struct {
		Date     string `json:"Date"`
		Invoice  string `json:"Invoice"`
		Supplier string `json:"SupplierCode"`
		ItemCode string `json:"ItemCode"`
		Qty      int    `json:"Qty"`
	}

	var inboundExcel []InboundExcel
	// Parse Body
	if err := ctx.BodyParser(&inboundExcel); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// start DB transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var supplier models.Supplier
	if err := tx.Debug().First(&supplier, "supplier_code = ?", inboundExcel[0].Supplier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for _, inbound := range inboundExcel {

		// check item code
		var item models.Product
		if err := tx.Debug().First(&item, "item_code = ?", inbound.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item " + inbound.ItemCode + " not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	inboundRepo := repositories.NewInboundRepository(tx)
	inboundNo, _ := inboundRepo.GenerateInboundNo()

	receiveDate := inboundExcel[0].Date
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(receiveDate))
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	receiveDate = parsed.Format("2006-01-02")

	fmt.Println("receiveDate : ", receiveDate)

	inboundHeader := models.InboundHeader{
		InboundNo:   inboundNo,
		InvoiceNo:   inboundExcel[0].Invoice,
		SupplierId:  int(supplier.ID),
		InboundDate: receiveDate,
		PoDate:      receiveDate,
		Status:      "open",
		CreatedBy:   int(ctx.Locals("userID").(float64)),
		UpdatedBy:   int(ctx.Locals("userID").(float64)),
	}

	// insert inbound header
	if err := tx.Debug().Create(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// insert inbound detail

	for _, inbound := range inboundExcel {

		// check item code
		var item models.Product
		if err := tx.Debug().First(&item, "item_code = ?", inbound.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item " + inbound.ItemCode + " not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		receiveDateItem := inbound.Date
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(receiveDateItem))
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		receiveDateItem = parsed.Format("2006-01-02")

		inboundDetail := models.InboundDetail{

			InboundNo: inboundHeader.InboundNo,
			InboundId: int(inboundHeader.ID),
			ItemId:    int(item.ID),
			ItemCode:  item.ItemCode,
			Barcode:   item.Barcode,
			Location:  "RCVDOCK",
			Status:    "open",
			RecDate:   receiveDateItem,
			Quantity:  int(inbound.Qty),
			Uom:       item.Uom,
			CreatedBy: int(ctx.Locals("userID").(float64)),
			UpdatedBy: int(ctx.Locals("userID").(float64)),
		}

		// insert inbound detail
		if err := tx.Debug().Create(&inboundDetail).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	// commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Payload Data Mentah Inbound : ", inboundExcel)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail added successfully"})
}

func (c *InboundController) GetPutawaySheet(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	type PutawaySheet struct {
		InboundDate  string `json:"inbound_date"`
		PoNo         string `json:"po_no"`
		InboundNo    string `json:"inbound_no"`
		ItemCode     string `json:"item_code"`
		Barcode      string `json:"barcode"`
		SupplierName string `json:"supplier_name"`
		Quantity     int    `json:"quantity"`
	}

	sql := `SELECT b.inbound_date, b.po_no, b.inbound_no, 
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
