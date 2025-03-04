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

var itemDetailInbound struct {
	ItemCode string `json:"item_code" validate:"required"`
	Quantity int    `json:"quantity" validate:"required"`
	Uom      string `json:"uom" validate:"required"`
	WhsCode  string `json:"whs_code" validate:"required"`
	RecDate  string `json:"rec_date" validate:"required"`
	Remarks  string `json:"remarks" validate:"required"`
}

type headerInbound struct {
	ID              uint   `json:"id"`
	InboundNo       string `json:"inbound_no"`
	SupplierCode    string `json:"supplier_code"`
	Invoice         string `json:"invoice"`
	TransporterCode string `json:"transporter_code"`
	DriverName      string `json:"driver_name"`
	TruckSize       string `json:"truck_size"`
	TruckNo         string `json:"truck_no"`
	InboundDate     string `json:"inbound_date"`
	ContainerNo     string `json:"container_no"`
	BlNo            string `json:"bl_no"`
	PoNo            string `json:"po_no"`
	PoDate          string `json:"po_date"`
	SjNo            string `json:"sj_no"`
	Origin          string `json:"origin"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	Remarks         string `json:"remarks_header"`
	TotalLine       int    `json:"total_line"`
	TotalQty        int    `json:"total_qty"`
}

type Handling struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

var InputPayload struct {
	InboundID int      `json:"inbound_id"`
	ItemCode  string   `json:"item_code"`
	Quantity  int      `json:"quantity"`
	UOM       string   `json:"uom"`
	RecDate   string   `json:"rec_date"`
	WhsCode   string   `json:"whs_code"`
	Handling  Handling `json:"handling"`
	Remarks   string   `json:"remarks"`
}

func NewInboundController(DB *gorm.DB) *InboundController {
	return &InboundController{DB: DB}
}

func (c *InboundController) AddNewItemInbound(ctx *fiber.Ctx) error {

	fmt.Println("Add New Item Inbound : ", string(ctx.Body()))

	if err := ctx.BodyParser(&InputPayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Payload Data : ", InputPayload)
	// return nil
	var data models.InboundDetail

	data.ItemCode = InputPayload.ItemCode
	data.Quantity = InputPayload.Quantity
	data.Uom = InputPayload.UOM
	data.WhsCode = InputPayload.WhsCode
	data.RecDate = InputPayload.RecDate
	data.Remarks = InputPayload.Remarks
	data.HandlingId = InputPayload.Handling.Value
	data.HandlingUsed = InputPayload.Handling.Label
	data.CreatedBy = int(ctx.Locals("userID").(float64))

	var headerInbound models.InboundHeader

	if InputPayload.InboundID > 0 {
		data.InboundId = InputPayload.InboundID

		if err := c.DB.Debug().Find(&headerInbound, InputPayload.InboundID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		data.ReferenceCode = headerInbound.Code
		data.Status = "open"
	}

	fmt.Println("Data : ", data)

	// return nil

	handlingRepo := repositories.NewHandlingRepository(c.DB)

	var handlingUsed []repositories.HandlingDetailUsed

	result, err := handlingRepo.GetHandlingUsed(InputPayload.Handling.Value)

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
	inboundDetailID, err := inboundRepo.CreateInboundDetail(&data, handlingUsed)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error s": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item added to inbound successfully", "data": fiber.Map{"detail_id": inboundDetailID}})
}

func (c *InboundController) GetInboundDetailDraftByUserID(ctx *fiber.Ctx) error {

	userID := int(ctx.Locals("userID").(float64))

	var inboundDetails []models.DetailResponse

	sqlSelect := `SELECT a.id, a.item_code, b.item_name, b.gmc, a.quantity, a.status, a.whs_code, a.rec_date, a.uom, a.remarks,
				c.name as handling_used, a.handling_id
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

	sqlDelete := `DELETE FROM inbound_details WHERE id = ? AND created_by = ?`
	if err := c.DB.Exec(sqlDelete, id, userID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail deleted successfully"})
}

func (c *InboundController) CreateInbound(ctx *fiber.Ctx) error {

	fmt.Println("Payload Data : ", string(ctx.Body()))
	// return nil

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

	fmt.Println("Inbound No:", inboundNo)

	inboundHeader.Code = inboundNo

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

	type listInbound struct {
		ID              uint   `json:"id"`
		InboundNo       string `json:"inbound_no"`
		SupplierCode    string `json:"supplier_code"`
		SupplierName    string `json:"supplier_name"`
		Status          string `json:"status"`
		Invoice         string `json:"invoice"`
		TransporterCode string `json:"transporter_code"`
		DriverName      string `json:"driver_name"`
		TruckSize       string `json:"truck_size"`
		TruckNo         string `json:"truck_no"`
		InboundDate     string `json:"inbound_date"`
		ContainerNo     string `json:"container_no"`
		BlNo            string `json:"bl_no"`
		PoNo            string `json:"po_no"`
		PoDate          string `json:"po_date"`
		SjNo            string `json:"sj_no"`
		Origin          string `json:"origin"`
		TimeArrival     string `json:"time_arrival"`
		StartUnloading  string `json:"start_unloading"`
		FinishUnloading string `json:"finish_unloading"`
		RemarksHeader   string `json:"remarks_header"`
		TotalLine       int    `json:"total_line"`
		TotalQty        int    `json:"total_qty"`
		TransporterName string `json:"transporter_name"`
	}

	var result []listInbound

	sql := `WITH detail AS (
				SELECT reference_code, COUNT(item_code) as total_line,SUM(quantity) total_qty 
				FROM inbound_details GROUP BY reference_code
			)
			SELECT a.id, a.code as inbound_no, a.supplier_code, 
			a.invoice_no as invoice, a.transporter as transporter_code,
			a.driver_name, a.truck_size, a.truck_no, a.inbound_date,
			a.container_no, a.bl_no, a.po_no, a.po_date, a.sj_no,
			a.origin, a.time_arrival, a.start_unloading, a.finish_unloading,
			a.status, a.inbound_date, a.remarks as remarks_header,
			b.total_line, b.total_qty,
			c.supplier_name, a.status, d.transporter_name
			FROM 
			inbound_headers a
			INNER JOIN detail b ON a.code = b.reference_code
			LEFT JOIN suppliers c ON a.supplier_code = c.supplier_code
			LEFT JOIN transporters d ON a.transporter = d.transporter_code
			ORDER BY a.created_at DESC`

	if err := c.DB.Raw(sql).Scan(&result).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

func (c *InboundController) GetInboundByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	sql := `WITH detail AS (
				SELECT reference_code, COUNT(item_code) as total_line,SUM(quantity) total_qty 
				FROM inbound_details GROUP BY reference_code
			)
			SELECT a.id, a.code as inbound_no, a.supplier_code, 
			a.invoice_no as invoice, a.transporter as transporter_code,
			a.driver_name, a.truck_size, a.truck_no, a.inbound_date,
			a.container_no, a.bl_no, a.po_no, a.po_date, a.sj_no,
			a.origin, a.time_arrival, a.start_unloading, a.finish_unloading,
			a.status, a.inbound_date, a.remarks as remarks_header,
			b.total_line, b.total_qty,
			c.supplier_name, a.status
			FROM 
			inbound_headers a
			INNER JOIN detail b ON a.code = b.reference_code
			LEFT JOIN suppliers c ON a.supplier_code = c.supplier_code
			WHERE a.id = ?`

	// type listInbound struct {
	// 	ID              uint   `json:"id"`
	// 	InboundNo       string `json:"inbound_no"`
	// 	SupplierCode    string `json:"supplier_code"`
	// 	Invoice         string `json:"invoice"`
	// 	TransporterCode string `json:"transporter_code"`
	// 	DriverName      string `json:"driver_name"`
	// 	TruckSize       string `json:"truck_size"`
	// 	TruckNo         string `json:"truck_no"`
	// 	InboundDate     string `json:"inbound_date"`
	// 	ContainerNo     string `json:"container_no"`
	// 	BlNo            string `json:"bl_no"`
	// 	PoNo            string `json:"po_no"`
	// 	PoDate          string `json:"po_date"`
	// 	SjNo            string `json:"sj_no"`
	// 	Origin          string `json:"origin"`
	// 	TimeArrival     string `json:"time_arrival"`
	// 	StartUnloading  string `json:"start_unloading"`
	// 	FinishUnloading string `json:"finish_unloading"`
	// 	RemarksHeader   string `json:"remarks_header"`
	// 	TotalLine       int    `json:"total_line"`
	// 	TotalQty        int    `json:"total_qty"`
	// }

	var result headerInbound

	if err := c.DB.Raw(sql, id).Scan(&result).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	sqlDetail := `SELECT a.id, a.item_code, a.quantity , b.item_name, b.cbm, b.gmc, a.whs_code, a.rec_date, a.uom, a.remarks
	FROM inbound_details a
	INNER JOIN products b ON a.item_code = b.item_code
	WHERE a.reference_code = ?`

	var detail []models.DetailResponse
	if err := c.DB.Raw(sqlDetail, result.InboundNo).Scan(&detail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	result.InboundDate = func() string { t, _ := time.Parse(time.RFC3339, result.InboundDate); return t.Format("2006-01-02") }()
	result.PoDate = func() string { t, _ := time.Parse(time.RFC3339, result.PoDate); return t.Format("2006-01-02") }()

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"header": result, "details": detail}})
}

func (c *InboundController) UpdateDetailByID(ctx *fiber.Ctx) error {

	fmt.Println("payload Edit :", string(ctx.Body()))
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := ctx.BodyParser(&InputPayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := ctx.BodyParser(&itemDetailInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(itemDetailInbound); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))
	updateTime := time.Now()

	handlingRepo := repositories.NewHandlingRepository(c.DB)

	var handlingUsed []repositories.HandlingDetailUsed

	result, err := handlingRepo.GetHandlingUsed(InputPayload.Handling.Value)

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

	sqlUpdate := `UPDATE inbound_details SET item_code = ?, quantity = ?, uom = ?, whs_code = ?, rec_date = ?, updated_by = ?, updated_at = ?, handling_id = ?, handling_used = ?, remarks = ? WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate, itemDetailInbound.ItemCode, itemDetailInbound.Quantity, itemDetailInbound.Uom, itemDetailInbound.WhsCode, itemDetailInbound.RecDate, userID, updateTime, InputPayload.Handling.Value, InputPayload.Handling.Label, itemDetailInbound.Remarks, id).Error; err != nil {
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
			CreatedBy:         int(userID),
		}

		if err := c.DB.Create(&inboundDetailHandling).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail updated successfully", "data": itemDetailInbound})
}

func (c *InboundController) AddInboundDetailByID(ctx *fiber.Ctx) error {

	fmt.Println(string(ctx.Body()))
	return nil

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Check if the inbound header exists
	var inboundHeader models.InboundHeader
	if err := c.DB.First(&inboundHeader, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var detailInput struct {
		ReferenceCode string `json:"reference_code" validate:"required"`
		ItemCode      string `json:"item_code" validate:"required"`
		Quantity      int    `json:"quantity" validate:"required"`
		Uom           string `json:"uom" validate:"required"`
		WhsCode       string `json:"whs_code" validate:"required"`
		RecDate       string `json:"rec_date" validate:"required"`
		Remarks       string `json:"remarks" validate:"required"`
	}

	if err := ctx.BodyParser(&detailInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(detailInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))
	createTime := time.Now()

	sqlInsert := `INSERT INTO inbound_details (reference_code, item_code, quantity, uom, whs_code, rec_date, remarks, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := c.DB.Exec(sqlInsert, detailInput.ReferenceCode, detailInput.ItemCode, detailInput.Quantity, detailInput.Uom, detailInput.WhsCode, detailInput.RecDate, detailInput.Remarks, userID, createTime).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail added successfully", "data": detailInput})
}

func (c *InboundController) UpdateInboundByID(ctx *fiber.Ctx) error {

	fmt.Println("Payload Edit Data Header : ", string(ctx.Body()))
	// return nil

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

	var inputHeader headerInbound
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

	// Update inbound header

	if err := c.DB.Debug().Model(&inboundHeader).Updates(inputHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("ID : ", id)
	fmt.Println("JSON parse : ", inputHeader)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound detail added successfully", "data": inputHeader})
}
