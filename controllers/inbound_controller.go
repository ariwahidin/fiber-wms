package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
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

func NewInboundController(DB *gorm.DB) *InboundController {
	return &InboundController{DB: DB}
}

func (c *InboundController) AddNewItemInbound(ctx *fiber.Ctx) error {

	var inboundDetail models.InboundDetail

	if err := ctx.BodyParser(&inboundDetail); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check Item Code IN DB
	var product models.Product

	if err := c.DB.Debug().Where("item_code = ?", inboundDetail.ItemCode).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundDetail.ItemID = int(product.ID) //product.ID
	inboundDetail.ItemCode = product.ItemCode

	// return nil

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

	var inboundHeader models.InboundHeader

	if inboundDetail.InboundId > 0 {

		if err := c.DB.Debug().Find(&inboundHeader, inboundDetail.InboundId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		inboundDetail.ReferenceCode = inboundHeader.Code
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

	inboundRepo := repositories.NewInboundRepository(c.DB)
	inboundDetailID, err := inboundRepo.CreateInboundDetail(&inboundDetail, handlingUsed)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error s": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item added to inbound successfully", "data": fiber.Map{"detail_id": inboundDetailID}})
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

func (c *InboundController) CreateInbound(ctx *fiber.Ctx) error {

	var inboundHeader models.InboundHeader

	// Parse Body
	if err := ctx.BodyParser(&inboundHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(inboundHeader); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeader.CreatedBy = int(ctx.Locals("userID").(float64))

	fmt.Println("Payload Yang Sudah di Parse : ", inboundHeader)

	userID := int(ctx.Locals("userID").(float64))

	sqlDraft := `SELECT * FROM inbound_details WHERE created_by = ? AND status = 'draft'`
	var inboundDetails []models.InboundDetail
	if err := c.DB.Raw(sqlDraft, userID).Scan(&inboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundDetails) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No draft found"})
	}

	var lastInbound models.InboundHeader
	if err := c.DB.Last(&lastInbound).Error; err != nil && err != gorm.ErrRecordNotFound {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// jika ada record inbound
	var inboundNo string
	if lastInbound.Code != "" {
		// ambil nomor inbound terakhir
		inboundNo = lastInbound.Code

		// Jika Bulan Sudah Berbeda
		if time.Now().Format("01") != inboundNo[8:10] {
			inboundNo = fmt.Sprintf("IN-%s-%s-%04d", time.Now().Format("2006"), time.Now().Format("01"), 1)
		} else {
			// ambil nomor urut dari nomor inbound terakhir
			lastInboundNo := inboundNo[len(inboundNo)-4:]

			// tambahkan 1 ke nomor urut
			lastInboundNoInt, _ := strconv.Atoi(lastInboundNo)
			lastInboundNoInt++
			inboundNo = fmt.Sprintf("IN-%s-%s-%04d", time.Now().Format("2006"), time.Now().Format("01"), lastInboundNoInt)
		}

	} else {
		// jika tidak ada record inbound
		inboundNo = fmt.Sprintf("IN-%s-%s-%04d", time.Now().Format("2006"), time.Now().Format("01"), 1)
	}

	inboundHeader.Code = inboundNo
	inboundHeader.Status = "open"

	// Mulai transaksi
	tx := c.DB.Begin()

	if err := tx.Create(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update Inbound Detail
	sqlUpdate := `UPDATE inbound_details SET inbound_id = ?, reference_code = ?, status = 'open' WHERE created_by = ? AND status = 'draft'`
	if err := tx.Exec(sqlUpdate, inboundHeader.ID, inboundNo, userID).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Commit transaction
	tx.Commit()

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound created successfully", "nomor_inbound": inboundNo, "data": inboundHeader})
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

	inboundHeader, err := repositories.NewInboundRepository(c.DB).GetInboundHeaderByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeader.InboundDate = func() string {
		t, _ := time.Parse(time.RFC3339, inboundHeader.InboundDate)
		return t.Format("2006-01-02")
	}()
	inboundHeader.PoDate = func() string { t, _ := time.Parse(time.RFC3339, inboundHeader.PoDate); return t.Format("2006-01-02") }()

	detailItem, err := repositories.NewInboundRepository(c.DB).GetDetailItemByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"header": inboundHeader, "details": detailItem}})
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

	fmt.Println("ID : ", id)

	inboundRepo := repositories.NewInboundRepository(c.DB)

	inboundScanned, err := inboundRepo.GetAllInboundScannedByInboundID(id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundScanned) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound not found"})
	}

	for _, inbound := range inboundScanned {
		if inbound.RemainingQty > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound not complete"})
		}
	}

	// check status inbound header
	var inbound_header models.InboundHeader

	if err := c.DB.First(&inbound_header, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if inbound_header.Status == "closed" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound already closed"})
	}

	var inbound_barcodes []models.InboundBarcode

	if err := c.DB.Where("inbound_id = ? AND status = ?", id, "pending").Find(&inbound_barcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("inbound_barcodes : ", inbound_barcodes)

	return nil

	// // Mulai transaksi
	// tx := c.DB.Begin()

	// // Tangani jika transaksi gagal
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		tx.Rollback()
	// 		log.Println("Transaksi dibatalkan karena error:", r)
	// 	}
	// }()

	// // Simpan data Inventory

	// var inboundDetails []models.InboundDetail
	// if err := tx.Raw(`SELECT a.id, a.item_id, a.item_code, a.whs_code, a.quantity
	// 		FROM inbound_details a
	// 		WHERE inbound_id = ?`, id).Scan(&inboundDetails).Error; err != nil {
	// 	tx.Rollback()
	// }

	// var inventory models.Inventory
	// for _, inboundDetail := range inboundDetails {

	// 	inventory = models.Inventory{
	// 		InboundDetailId: int(inboundDetail.ID),
	// 		ItemId:          int(inboundDetail.ItemID),
	// 		ItemCode:        inboundDetail.ItemCode,
	// 		WhsCode:         inboundDetail.WhsCode,
	// 		Quantity:        inboundDetail.Quantity,
	// 		CreatedBy:       int(ctx.Locals("userID").(float64)),
	// 	}

	// 	if err := tx.Create(&inventory).Error; err != nil {
	// 		tx.Rollback()
	// 		log.Println("Gagal insert Inventory:", err)
	// 	}

	// 	var inboundBarcodes []models.InboundBarcode
	// 	if err := tx.Raw(`select a.inbound_detail_id, a.scan_data as serial_number,
	// 		a.location, SUM(a.quantity) as quantity, a.qa_status
	// 		from inbound_barcodes a
	// 		WHERE inbound_id = ? AND a.inbound_detail_id = ?
	// 		GROUP BY a.inbound_detail_id, a.scan_data, a.location, a.qa_status`, id, inboundDetail.ID).Scan(&inboundBarcodes).Error; err != nil {
	// 		tx.Rollback()
	// 		log.Println("Gagal mengambil data Inbound Barcodes:", err)
	// 	}

	// 	var inventoryDetail models.InventoryDetail

	// 	for _, inboundBarcode := range inboundBarcodes {

	// 		inventoryDetail = models.InventoryDetail{
	// 			InventoryId:     int(inventory.ID),
	// 			Location:        inboundBarcode.Location,
	// 			InboundDetailId: int(inboundDetail.ID),
	// 			SerialNumber:    inboundBarcode.SerialNumber,
	// 			Quantity:        int(inboundBarcode.Quantity),
	// 			QaStatus:        inboundBarcode.QaStatus,
	// 			CreatedBy:       int(ctx.Locals("userID").(float64)),
	// 		}

	// 		if err := tx.Create(&inventoryDetail).Error; err != nil {
	// 			tx.Rollback()
	// 			log.Println("Gagal insert Inventory Detail:", err)
	// 		}

	// 	}

	// }

	// // Update status inbound
	// if err := tx.Model(&models.InboundHeader{}).Where("id = ?", id).Update("status", "closed").Error; err != nil {
	// 	tx.Rollback()
	// 	log.Println("Gagal updating status inbound : ", err)
	// }

	// // Update Status inbound barcodes
	// if err := tx.Model(&models.InboundBarcode{}).Where("inbound_id", id).Update("status", "in_stock").Error; err != nil {
	// 	tx.Rollback()
	// 	log.Println("Gagal updating inbound barcode : ", err)
	// }

	// // Commit transaksi jika semua sukses
	// if err := tx.Commit().Error; err != nil {
	// 	log.Println("Gagal commit transaksi:", err)
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound processed successfully"})
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
