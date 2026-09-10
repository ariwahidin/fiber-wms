package outbound_controller

import (
	"encoding/json"
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/types"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	integration_service "fiber-app/services/integration_service"
	notification_service "fiber-app/services/notification_service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Row     int    `json:"row"`
}

type OutboundController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
}

func NewOutboundController(db *gorm.DB, queryDB *gorm.DB) *OutboundController {
	return &OutboundController{DB: db, QueryDB: queryDB}
}

type Outbound struct {
	ID              types.SnowflakeID `json:"ID"`
	OutboundNo      string            `json:"outbound_no"`
	OutboundDate    string            `json:"outbound_date"`
	CustomerCode    string            `json:"customer_code"`
	ShipmentID      string            `json:"shipment_id"`
	Mode            string            `json:"mode"`
	Status          string            `json:"status"`
	WhsCode         string            `json:"whs_code"`
	OwnerCode       string            `json:"owner_code"`
	Remarks         string            `json:"remarks"`
	TransporterCode string            `json:"transporter_code"`
	PickerName      string            `json:"picker_name"`
	CustAddress     string            `json:"cust_address"`
	CustCity        string            `json:"cust_city"`
	OrderType       string            `json:"order_type"`
	PlanPickupDate  string            `json:"plan_pickup_date"`
	PlanPickupTime  string            `json:"plan_pickup_time"`
	RcvDoDate       string            `json:"rcv_do_date"`
	RcvDoTime       string            `json:"rcv_do_time"`
	StartPickTime   string            `json:"start_pick_time"`
	EndPickTime     string            `json:"end_pick_time"`
	DelivTo         string            `json:"deliv_to"`
	DelivAddress    string            `json:"deliv_address"`
	DelivCity       string            `json:"deliv_city"`
	Driver          string            `json:"driver"`
	QtyKoli         int               `json:"qty_koli"`
	QtyKoliSeal     int               `json:"qty_koli_seal"`
	TruckSize       string            `json:"truck_size"`
	TruckNo         string            `json:"truck_no"`
	Items           []OutboundItem    `json:"items"`
}

type OutboundItem struct {
	ID            int               `json:"ID"`
	OutboundID    types.SnowflakeID `json:"outbound_id"`
	ItemCode      string            `json:"item_code"`
	Quantity      float64           `json:"quantity"`
	UOM           string            `json:"uom"`
	SN            string            `json:"sn"`
	Location      string            `json:"location"`
	Remarks       string            `json:"remarks"`
	Mode          string            `json:"mode"`
	VasID         int               `json:"vas_id"`
	ExpDate       string            `json:"exp_date"`
	LotNumber     string            `json:"lot_number"`
	CartonNumber  string            `json:"carton_number"`
	CaseNumber    string            `json:"case_number"`
	SerialNumber  string            `json:"serial_number"`
	SerialNumbers []string          `json:"serial_numbers"`
	DivisionCode  string            `json:"division_code"`
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

	fmt.Println("Create Outbound Payload:", payload)

	// return nil

	// Validate shipment id
	if payload.ShipmentID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Shipment ID / DO Number is required",
			"error":   "Shipment ID / DO Number is required",
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

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.UOM, item.LotNumber, item.ExpDate, item.CartonNumber, item.CaseNumber, item.SerialNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item with code %s,  uom %s, lot number %s, expiration date %s, carton number %s, case number %s, serial number %s",
					item.ItemCode, item.UOM, item.LotNumber, item.ExpDate, item.CartonNumber, item.CaseNumber, item.SerialNumber),
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

	fmt.Println("Start DB Transaction:", payload)

	var invetoryPolicy models.InventoryPolicy
	if err := tx.Debug().First(&invetoryPolicy, "owner_code = ?", payload.OwnerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory Policy not found",
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	if invetoryPolicy.RequireLotNumber {
		for _, item := range payload.Items {
			if item.LotNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number is required",
					"error":   "Lot number is required",
				})
			}
		}
	}

	repositories := repositories.NewOutboundRepository(tx)

	outbound_no, err := repositories.GenerateOutboundNumber()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate inbound no",
			"error":   err.Error(),
		})
	}

	fmt.Println("Outbound No:", outbound_no)

	payload.OutboundNo = outbound_no
	payload.Status = "open"
	userID := int(ctx.Locals("userID").(float64))

	var OutboundHeader models.OutboundHeader

	var customer models.Customer

	if err := tx.Debug().First(&customer, "customer_code = ?", payload.CustomerCode).Error; err != nil {
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
	// Insert ke inbounds Header

	OutboundHeader.OutboundNo = payload.OutboundNo
	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.CustomerCode = customer.CustomerCode
	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.ShipmentID = payload.ShipmentID
	OutboundHeader.WhsCode = payload.WhsCode
	OutboundHeader.OwnerCode = payload.OwnerCode
	OutboundHeader.Remarks = payload.Remarks
	OutboundHeader.CreatedBy = userID
	OutboundHeader.UpdatedBy = userID
	OutboundHeader.Status = "open"
	OutboundHeader.RawStatus = "DRAFT"
	OutboundHeader.DraftTime = time.Now()
	OutboundHeader.TransporterCode = payload.TransporterCode
	OutboundHeader.PickerName = payload.PickerName
	OutboundHeader.CustAddress = payload.CustAddress
	OutboundHeader.CustCity = payload.CustCity
	OutboundHeader.OrderType = payload.OrderType
	OutboundHeader.PlanPickupDate = payload.PlanPickupDate
	OutboundHeader.PlanPickupTime = payload.PlanPickupTime
	OutboundHeader.RcvDoDate = payload.RcvDoDate
	OutboundHeader.RcvDoTime = payload.RcvDoTime
	OutboundHeader.StartPickTime = payload.StartPickTime
	OutboundHeader.EndPickTime = payload.EndPickTime
	OutboundHeader.DelivTo = payload.DelivTo
	OutboundHeader.DelivAddress = payload.DelivAddress
	OutboundHeader.DelivCity = payload.DelivCity
	OutboundHeader.Driver = payload.Driver
	OutboundHeader.QtyKoli = payload.QtyKoli
	OutboundHeader.QtyKoliSeal = payload.QtyKoliSeal
	OutboundHeader.TruckSize = payload.TruckSize
	OutboundHeader.TruckNo = payload.TruckNo

	res := tx.Create(&OutboundHeader)

	if res.Error != nil {
		tx.Rollback()

		errMsg := res.Error.Error()
		userMessage := "Failed to save outbound data"

		if strings.Contains(errMsg, "UNIQUE KEY constraint") {
			switch {
			case strings.Contains(errMsg, "uni_outbound_headers_shipment_id"):
				userMessage = fmt.Sprintf("DO No '%s' is already in use, please use a different ID", OutboundHeader.ShipmentID)
			default:
				userMessage = "The data you entered already exists (duplicate)"
			}
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": userMessage,
		})
	}

	var outboundID uint
	if res.RowsAffected == 1 {
		outboundID = OutboundHeader.ID
	}

	if len(payload.Items) < 1 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No items found",
			"error":   "No items found",
		})
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

		var vas models.Vas

		if invetoryPolicy.UseVAS {
			if err := tx.Debug().First(&vas, "id = ?", item.VasID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Vas not found"})
				}

				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
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

		var OutboundDetail models.OutboundDetail
		OutboundDetail.OutboundNo = payload.OutboundNo
		OutboundDetail.OutboundID = outboundID
		OutboundDetail.ItemCode = item.ItemCode
		OutboundDetail.ItemID = int(product.ID)
		OutboundDetail.Barcode = uomConversion.Ean
		OutboundDetail.CustomerCode = OutboundHeader.CustomerCode
		OutboundDetail.Uom = item.UOM
		OutboundDetail.Quantity = item.Quantity
		OutboundDetail.ExpDate = item.ExpDate
		OutboundDetail.LotNumber = item.LotNumber
		OutboundDetail.CartonNumber = item.CartonNumber
		OutboundDetail.CaseNumber = item.CaseNumber
		OutboundDetail.SerialNumber = item.SerialNumber
		OutboundDetail.WhsCode = OutboundHeader.WhsCode
		if item.DivisionCode == "" {
			OutboundDetail.DivisionCode = "REGULAR"
		} else {
			OutboundDetail.DivisionCode = item.DivisionCode
		}
		OutboundDetail.Location = item.Location
		OutboundDetail.QaStatus = "A"
		OutboundDetail.SN = item.SN
		OutboundDetail.SNCheck = "N"
		OutboundDetail.OwnerCode = OutboundHeader.OwnerCode
		OutboundDetail.LotNumber = item.LotNumber
		OutboundDetail.Remarks = item.Remarks
		OutboundDetail.VasID = item.VasID
		OutboundDetail.VasName = vas.Name
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

		for _, sn := range item.SerialNumbers {
			sn = strings.TrimSpace(sn)

			if sn == "" {
				continue
			}

			serial := models.OutboundSerial{
				OutboundId:       int(outboundID),
				OutboundDetailId: int(OutboundDetail.ID),
				SerialNumber:     sn,
				CreatedBy:        userID,
				UpdatedBy:        userID,
			}

			if err := tx.Create(&serial).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Failed to insert outbound serial",
					"error":   err.Error(),
				})
			}
		}

	}

	fmt.Println("End DB Transaction: ", outboundID)

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

func (c *OutboundController) GetOutboundListFilter(ctx *fiber.Ctx) error {
	// Parse statuses dari query string "open,picking,packing"
	var statuses []string
	if raw := ctx.Query("statuses"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	// Parse order_types dari query string "B2B - Consignment,B2B - Normal"
	var orderTypes []string
	if raw := ctx.Query("order_types"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				orderTypes = append(orderTypes, s)
			}
		}
	}

	var owners []string
	if raw := ctx.Query("owners"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				owners = append(owners, s)
			}
		}
	}

	params := repositories.OutboundFilterParams{
		StartDate:  ctx.Query("start_date"),
		EndDate:    ctx.Query("end_date"),
		Search:     ctx.Query("search"),
		SearchItem: ctx.Query("search_item"),
		Statuses:   statuses,
		OrderTypes: orderTypes,
		Owners:     owners, // ← tambahan
	}

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	list, err := outboundRepo.GetOutboundListWithFilter(params)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Outbound found",
		"data":    list,
	})
}

func (c *OutboundController) GetOutboundListComplete(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	rawOutboundList, err := outboundRepo.GetAllOutboundListComplete()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Outbound found",
		"data":    rawOutboundList,
	})
}
func (c *OutboundController) GetOutboundListOutboundHandling(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	rawOutboundList, err := outboundRepo.GetAllOutboundListOutboundHandling()
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
	if err := c.DB.Debug().
		Preload("OutboundDetails.Product").
		First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	OutboundBarcodes, err := outboundRepo.GetOutboundBarcodeByOutboundID(OutboundHeader.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Ambil OutboundSerial untuk semua detail, group by detail ID
	detailIDs := make([]uint, 0, len(OutboundHeader.OutboundDetails))
	for _, d := range OutboundHeader.OutboundDetails {
		detailIDs = append(detailIDs, d.ID)
	}

	serialsByDetail := make(map[uint][]string)
	if len(detailIDs) > 0 {
		var serials []models.OutboundSerial
		if err := c.DB.Where("outbound_detail_id IN ?", detailIDs).Find(&serials).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		for _, s := range serials {
			serialsByDetail[uint(s.OutboundDetailId)] = append(serialsByDetail[uint(s.OutboundDetailId)], s.SerialNumber)
		}
	}

	// Sisipkan serial_numbers ke tiap detail sebagai field tambahan di response
	type detailWithSerial struct {
		models.OutboundDetail
		SerialNumbers []string `json:"serial_numbers"`
	}

	detailsWithSerial := make([]detailWithSerial, 0, len(OutboundHeader.OutboundDetails))
	for _, d := range OutboundHeader.OutboundDetails {
		sn := serialsByDetail[d.ID]
		if sn == nil {
			sn = []string{}
		}
		detailsWithSerial = append(detailsWithSerial, detailWithSerial{
			OutboundDetail: d,
			SerialNumbers:  sn,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"outbound": OutboundHeader,
			"barcodes": OutboundBarcodes,
			"details":  detailsWithSerial, // tambahan: detail + serial_numbers
		},
		"message": "Outbound found",
	})
}

func (c *OutboundController) UpdateOutboundByID(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var payload Outbound

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate shipment id
	if payload.ShipmentID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Shipment ID / DO Number is required",
			"error":   "Shipment ID / DO Number is required",
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

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", item.ItemCode, item.UOM, item.LotNumber, item.ExpDate, item.CartonNumber, item.CaseNumber, item.SerialNumber)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item with code %s,  uom %s, lot number %s, expiration date %s, carton number %s, case number %s, serial number %s",
					item.ItemCode, item.UOM, item.LotNumber, item.ExpDate, item.CartonNumber, item.CaseNumber, item.SerialNumber),
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

	var invetoryPolicy models.InventoryPolicy
	if err := tx.Debug().First(&invetoryPolicy, "owner_code = ?", payload.OwnerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory Policy not found",
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	if invetoryPolicy.RequireLotNumber {
		for _, item := range payload.Items {
			if item.LotNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number is required",
					"error":   "Lot number is required",
				})
			}
		}
	}

	userID := int(ctx.Locals("userID").(float64))
	var OutboundHeader models.OutboundHeader
	if err := tx.Debug().First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var customer models.Customer
	if err := tx.Debug().First(&customer, "customer_code = ?", payload.CustomerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	OutboundHeader.OutboundDate = payload.OutboundDate
	OutboundHeader.ShipmentID = payload.ShipmentID
	OutboundHeader.CustomerCode = payload.CustomerCode
	OutboundHeader.WhsCode = payload.WhsCode
	OutboundHeader.OwnerCode = payload.OwnerCode
	OutboundHeader.Remarks = payload.Remarks
	OutboundHeader.UpdatedBy = userID
	OutboundHeader.UpdatedAt = time.Now()
	OutboundHeader.TransporterCode = payload.TransporterCode
	OutboundHeader.PickerName = payload.PickerName
	OutboundHeader.CustAddress = payload.CustAddress
	OutboundHeader.CustCity = payload.CustCity
	OutboundHeader.OrderType = payload.OrderType
	OutboundHeader.PlanPickupDate = payload.PlanPickupDate
	OutboundHeader.PlanPickupTime = payload.PlanPickupTime
	OutboundHeader.RcvDoDate = payload.RcvDoDate
	OutboundHeader.RcvDoTime = payload.RcvDoTime
	OutboundHeader.StartPickTime = payload.StartPickTime
	OutboundHeader.EndPickTime = payload.EndPickTime
	OutboundHeader.DelivTo = payload.DelivTo
	OutboundHeader.DelivAddress = payload.DelivAddress
	OutboundHeader.DelivCity = payload.DelivCity
	OutboundHeader.Driver = payload.Driver
	OutboundHeader.QtyKoli = payload.QtyKoli
	OutboundHeader.QtyKoliSeal = payload.QtyKoliSeal
	OutboundHeader.TruckSize = payload.TruckSize
	OutboundHeader.TruckNo = payload.TruckNo

	if err := tx.Model(&models.OutboundHeader{}).Where("id = ?", OutboundHeader.ID).Updates(OutboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(payload.Items) < 1 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No items found",
			"error":   "No items found",
		})
	}

	// update outbound detail
	for _, item := range payload.Items {
		var outboundDetail models.OutboundDetail

		var product models.Product
		if err := tx.Debug().First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var vas models.Vas

		if invetoryPolicy.UseVAS {

			if err := tx.Debug().First(&vas, "id = ?", item.VasID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Vas not found"})
				}

				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
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

		var DivisionCode string
		if item.DivisionCode == "" {
			DivisionCode = "REGULAR"
		} else {
			DivisionCode = item.DivisionCode
		}

		// Coba cari berdasarkan ID
		err := tx.Debug().First(&outboundDetail, "id = ?", item.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {

			if OutboundHeader.Status == "open" {

				newDetail := models.OutboundDetail{
					OutboundID:   OutboundHeader.ID,
					OutboundNo:   OutboundHeader.OutboundNo,
					ItemID:       int(product.ID),
					ItemCode:     item.ItemCode,
					Barcode:      uomConversion.Ean,
					Quantity:     item.Quantity,
					ExpDate:      item.ExpDate,
					LotNumber:    item.LotNumber,
					CartonNumber: item.CartonNumber,
					CaseNumber:   item.CaseNumber,
					SerialNumber: item.SerialNumber,
					Location:     item.Location,
					WhsCode:      OutboundHeader.WhsCode,
					OwnerCode:    OutboundHeader.OwnerCode,
					CustomerCode: customer.CustomerCode,
					Uom:          item.UOM,
					DivisionCode: DivisionCode,
					QaStatus:     "A",
					Remarks:      item.Remarks,
					SN:           item.SN,
					SNCheck:      "N",
					VasID:        item.VasID,
					VasName:      vas.Name,
					CreatedBy:    int(ctx.Locals("userID").(float64)),
				}
				if err := tx.Create(&newDetail).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				}

				if err := createOutboundSerial(
					tx,
					OutboundHeader.ID,
					newDetail.ID,
					item.SerialNumbers,
					userID,
				); err != nil {
					tx.Rollback()

					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"message": "Failed to insert outbound serial",
						"error":   err.Error(),
					})
				}

			}

		} else if err == nil {
			if OutboundHeader.Status == "open" {
				outboundDetail.OutboundID = OutboundHeader.ID
				outboundDetail.ItemID = int(product.ID)
				outboundDetail.ItemCode = item.ItemCode
				outboundDetail.Barcode = uomConversion.Ean
				outboundDetail.Uom = item.UOM
				outboundDetail.WhsCode = OutboundHeader.WhsCode
				outboundDetail.OwnerCode = OutboundHeader.OwnerCode
				outboundDetail.DivisionCode = DivisionCode
				outboundDetail.CustomerCode = customer.CustomerCode
				outboundDetail.QaStatus = "A"
				outboundDetail.ExpDate = item.ExpDate
				outboundDetail.LotNumber = item.LotNumber
				outboundDetail.CartonNumber = item.CartonNumber
				outboundDetail.CaseNumber = item.CaseNumber
				outboundDetail.SerialNumber = item.SerialNumber
				outboundDetail.Quantity = item.Quantity
				outboundDetail.Location = item.Location
				outboundDetail.Remarks = item.Remarks
				outboundDetail.SN = item.SN
				outboundDetail.SNCheck = "N"
				outboundDetail.VasID = item.VasID
				outboundDetail.VasName = vas.Name
				outboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
				outboundDetail.UpdatedAt = time.Now()
			} else {
				outboundDetail.VasID = item.VasID
				outboundDetail.VasName = vas.Name
				outboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
				outboundDetail.UpdatedAt = time.Now()
			}

			if err := tx.Save(&outboundDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Sync serial numbers
			if OutboundHeader.Status == "open" {

				// Hapus serial lama
				if err := tx.
					Where("outbound_detail_id = ?", outboundDetail.ID).
					Delete(&models.OutboundSerial{}).Error; err != nil {

					tx.Rollback()

					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"message": "Failed to delete existing outbound serials",
						"error":   err.Error(),
					})
				}

				// Insert serial terbaru
				if err := createOutboundSerial(
					tx,
					OutboundHeader.ID,
					outboundDetail.ID,
					item.SerialNumbers,
					userID,
				); err != nil {

					tx.Rollback()

					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"success": false,
						"message": "Failed to insert outbound serial",
						"error":   err.Error(),
					})
				}
			}
		} else {
			// ❌ Error lain
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Outbound successfully", "data": OutboundHeader})
}

func (c *OutboundController) SaveOutboundSerial(ctx *fiber.Ctx) error {
	detailIDParam := ctx.Params("id")
	detailID, err := strconv.Atoi(detailIDParam)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid detail ID"})
	}

	var payload struct {
		SerialNumbers []string `json:"serial_numbers"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	var outboundDetail models.OutboundDetail
	if err := c.DB.Debug().First(&outboundDetail, "id = ?", detailID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound detail not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&outboundHeader, "id = ?", outboundDetail.OutboundID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi jumlah & isi
	trimmed := make([]string, 0, len(payload.SerialNumbers))
	seen := make(map[string]bool)
	for _, sn := range payload.SerialNumbers {
		sn = strings.TrimSpace(sn)
		if sn == "" {
			continue // izinkan kosong (belum lengkap semua), skip aja
		}
		if seen[sn] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Duplicate serial number: " + sn})
		}
		seen[sn] = true
		trimmed = append(trimmed, sn)
	}

	if len(trimmed) > int(outboundDetail.Quantity) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number is more than expected quantity"})
	}

	userID := int(ctx.Locals("userID").(float64))

	txErr := c.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("outbound_detail_id = ?", detailID).Delete(&models.OutboundSerial{}).Error; err != nil {
			return err
		}

		for _, sn := range trimmed {
			newSerial := models.OutboundSerial{
				OutboundId:       int(outboundDetail.OutboundID),
				OutboundDetailId: detailID,
				SerialNumber:     sn,
				CreatedBy:        userID,
				UpdatedBy:        userID,
			}
			if err := tx.Create(&newSerial).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if txErr != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": txErr.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Serial number saved successfully",
	})
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
		OutboundID: types.SnowflakeID(outboundDetail.OutboundID),
		ItemCode:   outboundDetail.ItemCode,
		Quantity:   outboundDetail.Quantity,
		UOM:        outboundDetail.Uom,
		Remarks:    outboundDetail.Remarks,
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

	// check status in outbound headers
	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&outboundHeader, "id = ?", outboundDetail.OutboundID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound header not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if outboundHeader.Status != "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "This outbound is not open"})
	}

	// hard delete
	if err := c.DB.Debug().Unscoped().Delete(&outboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully", "data": outboundDetail})
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

	var outboundHeader models.OutboundHeader
	if err := tx.Where("id = ?", id).First(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get outbound header: " + err.Error()})
	}

	var outboundDetails []models.OutboundDetail
	if err := tx.Debug().Where("outbound_id = ?", id).Find(&outboundDetails).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(outboundDetails) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Detail items not found"})
	}

	// validate customer
	var customer models.Customer
	if err := tx.Debug().First(&customer, "customer_code = ?", outboundHeader.CustomerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
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

	// validate shipto
	var customerShipTo models.Customer
	if err := tx.Debug().First(&customerShipTo, "customer_code = ?", outboundHeader.DelivTo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Delivery to not found",
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get delivery to",
			"error":   err.Error(),
		})
	}

	var invetoryPolicy models.InventoryPolicy
	if err := tx.Debug().First(&invetoryPolicy, "owner_code = ?", outboundHeader.OwnerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory Policy not found",
				"error":   err.Error(),
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	if invetoryPolicy.RequireLotNumber {
		for _, item := range outboundDetails {
			if item.LotNumber == "" {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number is required",
					"error":   "Lot number is required",
				})
			}
		}
	}

	uomRepo := repositories.NewUomRepository(tx)
	locationRepo := repositories.NewLocationRepository(tx)

	for _, outboundDetail := range outboundDetails {

		uomConversion, err := uomRepo.ConversionQty(outboundDetail.ItemCode, outboundDetail.Quantity, outboundDetail.Uom)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "UOM Conversion Error: " + err.Error()})
		}

		qtyReq := uomConversion.QtyConverted

		fmt.Println("Picking Query")

		queryInventory := tx.Debug().
			Table("inventories as i").
			Joins("LEFT JOIN locations as l ON i.whs_code = l.whs_code AND i.location = l.location_code").
			Joins(`LEFT JOIN (
				SELECT location, whs_code, item_id,
					MIN(rec_date) as min_rec_date,
					MIN(qty_available) as min_qty,
					MIN(exp_date) as min_exp_date
				FROM inventories
				WHERE qty_available > 0 AND deleted_at IS NULL
				GROUP BY location, whs_code, item_id
			) as loc_rank ON loc_rank.location = i.location 
						AND loc_rank.whs_code = i.whs_code 
						AND loc_rank.item_id = i.item_id`).
			Where(`
					i.item_id = ?
					AND i.whs_code = ?
					AND i.qty_available > 0
					AND i.uom = ?
					AND i.owner_code = ?
					AND i.qa_status = ?
					AND i.division_code = ?
					AND (
						l.id IS NULL
						OR (l.is_active = 1 AND l.is_pickable = 1)
					)
				`,
				outboundDetail.ItemID,
				outboundDetail.WhsCode,
				uomConversion.ToUom,
				outboundHeader.OwnerCode,
				outboundDetail.QaStatus,
				outboundDetail.DivisionCode,
			)

		if invetoryPolicy.RequireLotNumber {
			if outboundDetail.LotNumber == "" {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number is required",
					"error":   "Lot number is required",
				})
			}
			queryInventory = queryInventory.Where("i.lot_number = ?", outboundDetail.LotNumber)
		}

		if invetoryPolicy.AllocationLotByOrder {
			if outboundDetail.LotNumber != "" {
				queryInventory = queryInventory.
					Where("i.lot_number = ? AND i.qty_available > 0", outboundDetail.LotNumber)
			}
		}

		if invetoryPolicy.AllocationLocationByOrder {
			if outboundDetail.Location != "" {
				queryInventory = queryInventory.
					Where("i.location = ? AND i.qty_available > 0", outboundDetail.Location)
			}
		}

		if invetoryPolicy.AllocationCaseByOrder {
			if outboundDetail.CaseNumber != "" && invetoryPolicy.UseCaseNumber {
				queryInventory = queryInventory.
					Where("i.case_number = ? AND i.qty_available > 0", outboundDetail.CaseNumber)
			}
		}

		if invetoryPolicy.AllocationCartonByOrder {
			if outboundDetail.CartonNumber != "" && invetoryPolicy.UseCartonNumber {
				queryInventory = queryInventory.
					Where("i.carton_number = ? AND i.qty_available > 0", outboundDetail.CartonNumber)
			}
		}

		if invetoryPolicy.AllocationSerialByOrder {
			if outboundDetail.SerialNumber != "" && invetoryPolicy.UseSerialNumber {
				queryInventory = queryInventory.
					Where("i.serial_number = ? AND i.qty_available > 0", outboundDetail.SerialNumber)
			}
		}

		if invetoryPolicy.UseFEFO {
			queryInventory = queryInventory.Order("loc_rank.min_exp_date ASC, loc_rank.min_qty ASC, i.location ASC, i.exp_date ASC, i.lot_number ASC, i.rec_date ASC, i.qty_available ASC, i.pallet ASC")
		} else {
			queryInventory = queryInventory.Order("loc_rank.min_rec_date ASC, loc_rank.min_qty ASC, i.location ASC, i.qty_available ASC, i.rec_date ASC")
		}

		if invetoryPolicy.PickingExcludeLocationsUnderCycleCount {
			queryInventory = locationRepo.ExcludeLocationsUnderCycleCount(queryInventory, "i")
		}

		type InventoryWithLocation struct {
			models.Inventory
			LocationCode string
			Row          string
			Bay          string
			Level        string
			Bin          string
			Area         string
		}

		var inventories []InventoryWithLocation

		if err := queryInventory.
			Select(`
				i.*,
				l.location_code as location_code,
				l.row as row,
				l.bay as bay,
				l.level as level,
				l.bin as bin,
				l.area as area
			`).
			Find(&inventories).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf(
					"Failed to fetch inventory for ItemCode: %s (ItemID: %d, Whs: %s, UOM: %s, Owner: %s). Detail: %s",
					outboundDetail.ItemCode,
					outboundDetail.ItemID,
					outboundDetail.WhsCode,
					uomConversion.ToUom,
					outboundHeader.OwnerCode,
					err.Error(),
				),
			})
		}

		if len(inventories) == 0 && invetoryPolicy.UseLotNo && outboundDetail.LotNumber != "" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Insufficient stock available for item " + outboundDetail.ItemCode + " in lot number " + outboundDetail.LotNumber})
		}

		if len(inventories) == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Insufficient stock available for item " + outboundDetail.ItemCode,
			})
		}

		for _, inventory := range inventories {

			if qtyReq < 1 {
				break
			}
			var qtyPick float64 = 0

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
			pickingSheet := models.OutboundPicking{
				InventoryID:      int(inventory.ID),
				OutboundId:       outboundDetail.OutboundID,
				OutboundNo:       outboundDetail.OutboundNo,
				OutboundDetailId: int(outboundDetail.ID),
				OwnerCode:        inventory.OwnerCode,
				ItemID:           inventory.ItemId,
				Barcode:          product.Barcode,
				ItemCode:         product.ItemCode,
				Pallet:           inventory.Pallet,
				Location:         inventory.Location,
				Quantity:         qtyPick,
				Uom:              inventory.Uom,
				RecDate:          inventory.RecDate,
				ExpDate:          inventory.ExpDate,
				LotNumber:        inventory.LotNumber,
				CartonNumber:     inventory.CartonNumber,
				CaseNumber:       inventory.CaseNumber,
				SerialNumber:     inventory.SerialNumber,
				ProdDate:         inventory.ProdDate,
				WhsCode:          inventory.WhsCode,
				QaStatus:         inventory.QaStatus,
				UomDisplay:       outboundDetail.Uom,
				QtyDisplay:       qtyPick / uomConversion.Rate,
				EanDisplay:       uomConversion.Ean,
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

		if qtyReq > 0 && invetoryPolicy.UseLotNo && invetoryPolicy.RequireLotNumber && outboundDetail.LotNumber != "" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Insufficient stock for item " + outboundDetail.ItemCode + " in lot number " + outboundDetail.LotNumber,
			})
		}

		if qtyReq > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Insufficient stock for item " + outboundDetail.ItemCode,
			})
		}
	}

	outboundHeader.Status = "picking"
	outboundHeader.RawStatus = "CONFIRMED"
	outboundHeader.ConfirmTime = time.Now()
	outboundHeader.ConfirmBy = int(ctx.Locals("userID").(float64))
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

func (c *OutboundController) PackingAll(ctx *fiber.Ctx) error {
	outboundIDParam := ctx.Params("id")
	outboundID, err := strconv.Atoi(outboundIDParam)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid outbound ID"})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&outboundHeader, "id = ?", outboundID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Ambil semua picking sheet untuk outbound ini
	var pickings []models.OutboundPicking
	if err := c.DB.Debug().Where("outbound_id = ?", outboundHeader.ID).Find(&pickings).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(pickings) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No picking data found for this outbound"})
	}

	// ---- Group by CartonNumber (kosong dianggap 1 grup sendiri) ----
	cartonGroups := make(map[string][]models.OutboundPicking)
	var cartonKeys []string
	for _, p := range pickings {
		key := p.CartonNumber // bisa kosong, itu tetap valid sebagai 1 grup
		if _, exists := cartonGroups[key]; !exists {
			cartonKeys = append(cartonKeys, key)
		}
		cartonGroups[key] = append(cartonGroups[key], p)
	}

	// ---- Sort ascending (string compare biasa, cuma buat urutan konsisten) ----
	sort.Strings(cartonKeys)

	userID := int(ctx.Locals("userID").(float64))

	txErr := c.DB.Transaction(func(tx *gorm.DB) error {

		// Buat/ambil OutboundPacking dengan PackingNo = OutboundNo
		var packing models.OutboundPacking
		if err := tx.Where("packing_no = ?", outboundHeader.OutboundNo).First(&packing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				packing = models.OutboundPacking{
					PackingNo: outboundHeader.OutboundNo,
					// CreatedAt: time.Now(),
					CreatedBy: userID,
				}
				if err := tx.Create(&packing).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// Index carton dimulai dari 1, sesuai urutan hasil sort
		for i, cartonKey := range cartonKeys {
			packCtnNo := strconv.Itoa(i + 1)
			rows := cartonGroups[cartonKey]

			for _, p := range rows {

				var product models.Product
				if err := tx.Where("id = ?", p.ItemID).First(&product).Error; err != nil {
					return fmt.Errorf("product not found for item_code %s: %w", p.ItemCode, err)
				}

				outboundBarcode := models.OutboundBarcode{
					PackingId:        packing.ID,
					PackingNo:        packing.PackingNo,
					PackCtnNo:        packCtnNo,
					InventoryID:      p.InventoryID,
					OutboundId:       outboundHeader.ID,
					OutboundNo:       outboundHeader.OutboundNo,
					OutboundDetailId: p.OutboundDetailId,
					PickingSheetId:   int(p.ID),
					ItemID:           int(p.ItemID),
					ItemCode:         p.ItemCode,
					Barcode:          p.Barcode,
					Uom:              p.Uom,
					SerialNumber:     p.SerialNumber,
					RecDate:          p.RecDate,
					ProdDate:         p.ProdDate,
					ExpDate:          p.ExpDate,
					LotNumber:        p.LotNumber,
					CaseNumber:       p.CaseNumber,
					Quantity:         p.Quantity,
					Status:           "pending",
					QtyDataScan:      p.Quantity,
					UomScan:          p.Uom,
					IsSerial:         product.HasSerial == "Y",
					CartonID:         0,
					CartonCode:       p.CartonNumber,
					CreatedBy:        userID,
				}

				if err := tx.Create(&outboundBarcode).Error; err != nil {
					return err
				}
			}
		}

		// Update status header via repo yang sudah ada (dipakai juga di mobile scanner)
		pickingRepo := repositories.NewOutboundPickingRepository(tx)
		if err := pickingRepo.ConfirmPacking(outboundHeader.OutboundNo, userID); err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": txErr.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Outbound " + outboundHeader.OutboundNo + " packed successfully",
	})
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

	movementID := uuid.NewString()

	// transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	var outboundHeader models.OutboundHeader
	if err := tx.Where("id = ?", inputBody.OutboundID).First(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get outbound header: " + err.Error()})
	}

	repo := repositories.NewOutboundRepository(tx)

	// Check inventory policy
	var invetoryPolicy models.InventoryPolicy
	if err := tx.Debug().First(&invetoryPolicy, "owner_code = ?", outboundHeader.OwnerCode).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Check outbound item scan complete
	outboundItems, err := repo.GetOutboundItemByID(inputBody.OutboundID)
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(outboundItems) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound scanned not found"})
	}

	for _, outboundItem := range outboundItems {
		if outboundItem.QtyReq != outboundItem.QtyScan {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Scan not complete"})
		}
	}

	var outboundDetails []models.OutboundDetail
	if err := tx.Debug().Where("outbound_id = ?", inputBody.OutboundID).Find(&outboundDetails).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var pickingSheets []models.OutboundPicking
	if err := tx.Debug().Where("outbound_id = ?", inputBody.OutboundID).Find(&pickingSheets).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "message": "Failed to get picking sheets"})
	}

	for _, pickingSheet := range pickingSheets {

		// update inventory
		if err := tx.Debug().
			Model(&models.Inventory{}).
			Where("id = ?", pickingSheet.InventoryID).
			Updates(map[string]interface{}{
				"qty_onhand":    gorm.Expr("qty_onhand - ?", pickingSheet.Quantity),
				"qty_allocated": gorm.Expr("qty_allocated - ?", pickingSheet.Quantity),
				"qty_shipped":   gorm.Expr("qty_shipped + ?", pickingSheet.Quantity),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Record source inventory movement
		sourceMovement := models.InventoryMovement{
			InventoryID:        uint(pickingSheet.InventoryID),
			MovementID:         movementID,
			RefType:            "OUTBOUND COMPLETE",
			RefID:              uint(inputBody.OutboundID),
			ItemID:             pickingSheet.ItemID,
			ItemCode:           pickingSheet.ItemCode,
			QtyOnhandChange:    -pickingSheet.Quantity,
			QtyAvailableChange: -pickingSheet.Quantity,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			FromWhsCode:        pickingSheet.WhsCode,
			FromLocation:       pickingSheet.Location,
			OldQaStatus:        pickingSheet.QaStatus,
			Reason:             outboundHeader.OutboundNo + " COMPLETE",
			CreatedBy:          int(ctx.Locals("userID").(float64)),
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&sourceMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record source movement",
			})
		}

	}

	completeTime := time.Now()
	// UPDATE OUTBOUND STATUS
	if err := tx.Debug().
		Model(&models.OutboundHeader{}).
		Where("id = ?", inputBody.OutboundID).
		Updates(map[string]interface{}{
			"status":        "complete",
			"raw_status":    "COMPLETED",
			"complete_time": completeTime,
			"complete_by":   int(ctx.Locals("userID").(float64)),
			"updated_by":    int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update Outbound Barcodes Status
	if err := tx.Debug().
		Model(&models.OutboundBarcode{}).
		Where("outbound_id = ?", inputBody.OutboundID).
		Updates(map[string]interface{}{
			"status":     "complete",
			"updated_by": int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Trigger email notification (async, tidak memblok response)
	go notification_service.SendNotification(c.DB, "outbound.completed", map[string]interface{}{
		"outbound_no":   outboundHeader.OutboundNo,
		"shipment_id":   outboundHeader.ShipmentID,
		"owner_code":    outboundHeader.OwnerCode,
		"customer_code": outboundHeader.CustomerCode,
		"whs_code":      outboundHeader.WhsCode,
		"picker_name":   outboundHeader.PickerName,
		"deliv_to":      outboundHeader.DelivTo,
		"deliv_address": outboundHeader.DelivAddress,
		"deliv_city":    outboundHeader.DelivCity,
		"driver":        outboundHeader.Driver,
		"truck_no":      outboundHeader.TruckNo,
		"awb_no":        outboundHeader.AwbNo,
		"remarks":       outboundHeader.Remarks,
		"complete_time": completeTime.Format("2006-01-02 15:04:05"),
	})

	// Trigger Integration Hub (async)
	go integration_service.Dispatch(c.DB, c.QueryDB, "outbound.completed", map[string]interface{}{
		"outbound_no":   outboundHeader.OutboundNo,
		"owner_code":    outboundHeader.OwnerCode,
		"customer_code": outboundHeader.CustomerCode,
		"whs_code":      outboundHeader.WhsCode,
		"picker_name":   outboundHeader.PickerName,
		"deliv_to":      outboundHeader.DelivTo,
		"deliv_address": outboundHeader.DelivAddress,
		"deliv_city":    outboundHeader.DelivCity,
		"driver":        outboundHeader.Driver,
		"truck_no":      outboundHeader.TruckNo,
		"awb_no":        outboundHeader.AwbNo,
		"remarks":       outboundHeader.Remarks,
		"complete_time": completeTime.Format("2006-01-02 15:04:05"),
	})

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Picking complete successfully"})
}

func (c *OutboundController) GetKoliDetails(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var outboundHeader models.OutboundHeader
	if err := c.DB.Where("outbound_no = ?", outbound_no).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var koliDetails []models.OutboundScanDetail
	if err := c.DB.Where("outbound_id = ?", outboundHeader.ID).Find(&koliDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound found", "data": koliDetails})

}

func (c *OutboundController) GetOutboundHandlingByID(ctx *fiber.Ctx) error {
	outbound_no := ctx.Params("outbound_no")

	var outbound models.OutboundHeader
	if err := c.DB.Debug().
		Preload("OutboundDetails.Product").
		Preload("OutboundDetails.Handling").
		First(&outbound, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    outbound,
		"message": "Outbound found",
	})
}

type outboundDetailItem struct {
	OutboundID       string   `json:"outbound_id"`
	OutboundNo       string   `json:"outbound_no"`
	OutboundDetailId int      `json:"ID"`
	ItemCode         string   `json:"item_code"`
	Handling         []string `json:"handling"`
}

type outboundDetailRequest struct {
	Items []outboundDetailItem `json:"items"`
}

func (c *OutboundController) UpdateOutboundDetailHandling(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")
	if outboundNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Outbound number is required",
		})
	}

	var req outboundDetailRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON: " + err.Error(),
		})
	}

	if len(req.Items) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Items cannot be empty",
		})
	}

	for _, item := range req.Items {
		if item.OutboundDetailId == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Outbound detail ID is required",
			})
		}

		if item.Handling == nil {
			item.Handling = []string{}
		}

		fmt.Println("Items:", item)
		fmt.Printf("Update detail_id %d handling: %v\n", item.OutboundDetailId, item.Handling)

		// transaction
		tx := c.DB.Begin()
		if tx.Error != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
		}

		var outboundDetail models.OutboundDetail
		if err := tx.Debug().Where("id = ?", item.OutboundDetailId).First(&outboundDetail).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Delete existing Outbound Detail Handlings
		if err := tx.Debug().
			Unscoped().
			Where("outbound_detail_id = ?", outboundDetail.ID).
			Delete(&models.OutboundDetailHandling{}).Error; err != nil {

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if err := tx.Commit().Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Outbound detail handling updated successfully",
	})
}

// DTO / Response struct
type OutboundHandlingResponse struct {
	OutboundNo   string  `json:"outbound_no"`
	ItemCode     string  `json:"item_code"`
	HandlingUsed string  `json:"handling_used"`
	RateIdr      float64 `json:"rate_idr"`
	QtyHandling  int     `json:"qty_handling"`
	TotalPrice   float64 `json:"total_price"`
}

func (c *OutboundController) ViewBillHandlingByOutbound(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	var outboundHandling []OutboundHandlingResponse
	err := c.DB.
		Model(&models.OutboundDetailHandling{}). // model asli
		Select("outbound_no, item_code, handling_used, rate_idr, qty_handling, total_price").
		Where("outbound_no = ?", outboundNo).
		Find(&outboundHandling).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(outboundHandling) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Data not found",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    outboundHandling,
	})
}

func (r *OutboundController) HandleOpen(ctx *fiber.Ctx) error {

	var payload struct {
		OutboundNo string `json:"outbound_no"`
	}

	// Parse JSON payload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	OutboundHeader := models.OutboundHeader{}
	if err := r.DB.Debug().First(&OutboundHeader, "outbound_no = ?", payload.OutboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if OutboundHeader.Status == "open" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound " + payload.OutboundNo + " already in open status", "message": "Outbound " + payload.OutboundNo + " not in picking status"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound is in picking status", "data": OutboundHeader})
}

func (r *OutboundController) ProccesHandleOpen(ctx *fiber.Ctx) error {
	var payload struct {
		Action           string `json:"action"`
		OutboundNo       string `json:"outbound_no"`
		TempLocationName string `json:"temp_location_name"`
		Status           string `json:"status"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	var outboundHeader models.OutboundHeader
	if err := r.DB.First(&outboundHeader, "outbound_no = ?", payload.OutboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if outboundHeader.Status == "complete" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound " + payload.OutboundNo + " already complete", "message": "Outbound " + payload.OutboundNo + " not in picking status"})
	}

	var outboundPickings []models.OutboundPicking
	if err := r.DB.Where("outbound_no = ?", payload.OutboundNo).Find(&outboundPickings).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if len(outboundPickings) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound picking not found"})
	}

	tx := r.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	userID, ok := ctx.Locals("userID").(float64)
	if !ok {
		tx.Rollback()
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user ID"})
	}

	if payload.Action == "temp_location" {
		if payload.TempLocationName == "" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Temp location name is required"})
		}

		for _, picking := range outboundPickings {
			var inventory models.Inventory
			if err := tx.Where("id = ?", picking.InventoryID).First(&inventory).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found"})
			}
			var newInventory models.Inventory
			newInventory.OwnerCode = inventory.OwnerCode
			newInventory.WhsCode = inventory.WhsCode
			newInventory.DivisionCode = inventory.DivisionCode
			newInventory.InboundID = inventory.InboundID
			newInventory.InboundDetailId = inventory.InboundDetailId
			newInventory.RecDate = inventory.RecDate
			newInventory.ProdDate = inventory.ProdDate
			newInventory.ExpDate = inventory.ExpDate
			newInventory.Pallet = payload.TempLocationName
			newInventory.Location = payload.TempLocationName
			newInventory.ItemId = inventory.ItemId
			newInventory.ItemCode = inventory.ItemCode
			newInventory.Barcode = inventory.Barcode
			newInventory.QaStatus = inventory.QaStatus
			newInventory.Uom = inventory.Uom
			newInventory.QtyOrigin = picking.Quantity
			newInventory.QtyOnhand = picking.Quantity
			newInventory.QtyAvailable = picking.Quantity
			newInventory.Trans = "UNPOST " + payload.OutboundNo + ", From INV ID : " + fmt.Sprint(inventory.ID)
			newInventory.IsTransfer = true
			newInventory.TransferFrom = inventory.ID
			newInventory.LotNumber = inventory.LotNumber
			newInventory.CartonNumber = inventory.CartonNumber
			newInventory.CaseNumber = inventory.CaseNumber
			newInventory.SerialNumber = inventory.SerialNumber
			newInventory.CreatedBy = int(userID)
			newInventory.CreatedAt = time.Now()

			if err := tx.Create(&newInventory).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Kurangi inventory lama
			if err := tx.Model(&models.Inventory{}).Where("id = ?", picking.InventoryID).
				Updates(map[string]interface{}{
					"qty_origin":    gorm.Expr("qty_origin - ?", picking.Quantity),
					"qty_onhand":    gorm.Expr("qty_onhand - ?", picking.Quantity),
					"qty_allocated": gorm.Expr("qty_allocated - ?", picking.Quantity),
					"updated_at":    time.Now(),
					"updated_by":    userID,
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	} else {
		// Kalau action return to origin location
		for _, picking := range outboundPickings {
			if err := tx.Debug().Model(&models.Inventory{}).Where("id = ?", picking.InventoryID).
				Updates(map[string]interface{}{
					"qty_available": gorm.Expr("qty_available + ?", picking.Quantity),
					"qty_allocated": gorm.Expr("qty_allocated - ?", picking.Quantity),
					"updated_at":    time.Now(),
					"updated_by":    userID,
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	// Delete outbound picking
	if err := tx.Unscoped().Where("outbound_id = ?", outboundHeader.ID).Delete(&models.OutboundPicking{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Delete outbound barcodes
	if err := tx.Unscoped().Where("outbound_id = ?", outboundHeader.ID).Delete(&models.OutboundBarcode{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Delete scan vas outbound
	if err := tx.Unscoped().Where("outbound_id = ?", outboundHeader.ID).Delete(&models.OutboundVas{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update status
	if err := tx.Model(&models.OutboundHeader{}).Where("id = ?", outboundHeader.ID).
		Updates(map[string]interface{}{
			"status":               payload.Status,
			"raw_status":           "DRAFT",
			"draft_time":           time.Now(),
			"change_to_draft_time": time.Now(),
			"change_to_draft_by":   userID,
			"updated_by":           userID,
			"updated_at":           time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change to " + payload.Status + " successfully"})
}

func (c *OutboundController) HandleCancel(ctx *fiber.Ctx) error {

	var payload struct {
		OutboundNo string `json:"outbound_no"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().First(&outboundHeader, "outbound_no = ?", payload.OutboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if outboundHeader.Status != "complete" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Outbound " + payload.OutboundNo + " is not complete",
			"message": "Cancel only allowed for completed outbound",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound eligible for cancel", "data": outboundHeader})
}

func (c *OutboundController) ProccesHandleCancel(ctx *fiber.Ctx) error {

	fmt.Println("Outbound Cancel Proccess")

	var payload struct {
		OutboundID       int    `json:"outbound_id" validate:"required"`
		Action           string `json:"action"`
		TempLocationName string `json:"temp_location_name"`
		Reason           string `json:"reason"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	if payload.Action == "temp_location" && payload.TempLocationName == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Temp location name is required"})
	}

	userID, ok := ctx.Locals("userID").(float64)
	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user ID"})
	}

	movementID := uuid.NewString()

	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var outboundHeader models.OutboundHeader
	if err := tx.Where("id = ?", payload.OutboundID).First(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get outbound header: " + err.Error()})
	}

	if outboundHeader.Status != "complete" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound " + outboundHeader.OutboundNo + " is not complete"})
	}

	var pickingSheets []models.OutboundPicking
	if err := tx.Debug().Where("outbound_id = ?", payload.OutboundID).Find(&pickingSheets).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "message": "Failed to get picking sheets"})
	}
	if len(pickingSheets) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Picking sheets not found"})
	}

	for _, pickingSheet := range pickingSheets {

		var inventory models.Inventory
		if err := tx.Where("id = ?", pickingSheet.InventoryID).First(&inventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Inventory not found"})
		}

		toLocation := ""

		if payload.Action == "temp_location" {

			// Row asal: cuma reverse qty_shipped, TIDAK menambah onhand di lokasi lama
			if err := tx.Debug().
				Model(&models.Inventory{}).
				Where("id = ?", pickingSheet.InventoryID).
				Updates(map[string]interface{}{
					"qty_origin":  gorm.Expr("qty_origin - ?", pickingSheet.Quantity),
					"qty_shipped": gorm.Expr("qty_shipped - ?", pickingSheet.Quantity),
					"updated_by":  int(userID),
					"updated_at":  time.Now(),
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Row baru di temp location (mirror pattern IsTransfer di HandleOpen)
			newInventory := models.Inventory{
				OwnerCode:       inventory.OwnerCode,
				WhsCode:         inventory.WhsCode,
				DivisionCode:    inventory.DivisionCode,
				InboundID:       inventory.InboundID,
				InboundDetailId: inventory.InboundDetailId,
				RecDate:         inventory.RecDate,
				ProdDate:        inventory.ProdDate,
				ExpDate:         inventory.ExpDate,
				Pallet:          payload.TempLocationName,
				Location:        payload.TempLocationName,
				ItemId:          inventory.ItemId,
				ItemCode:        inventory.ItemCode,
				Barcode:         inventory.Barcode,
				QaStatus:        inventory.QaStatus,
				Uom:             inventory.Uom,
				QtyOrigin:       pickingSheet.Quantity,
				QtyOnhand:       pickingSheet.Quantity,
				QtyAvailable:    pickingSheet.Quantity,
				Trans:           "CANCEL " + outboundHeader.OutboundNo + ", From INV ID : " + fmt.Sprint(inventory.ID),
				IsTransfer:      true,
				TransferFrom:    inventory.ID,
				LotNumber:       inventory.LotNumber,
				CartonNumber:    inventory.CartonNumber,
				CaseNumber:      inventory.CaseNumber,
				SerialNumber:    inventory.SerialNumber,
				CreatedBy:       int(userID),
			}
			if err := tx.Create(&newInventory).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			toLocation = payload.TempLocationName

		} else {
			// Return to origin: reverse langsung di row yang sama
			if err := tx.Debug().
				Model(&models.Inventory{}).
				Where("id = ?", pickingSheet.InventoryID).
				Updates(map[string]interface{}{
					"qty_onhand":    gorm.Expr("qty_onhand + ?", pickingSheet.Quantity),
					"qty_available": gorm.Expr("qty_available + ?", pickingSheet.Quantity),
					"qty_shipped":   gorm.Expr("qty_shipped - ?", pickingSheet.Quantity),
					"updated_by":    int(userID),
					"updated_at":    time.Now(),
				}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			toLocation = inventory.Location
		}

		// Record reverse inventory movement
		reverseMovement := models.InventoryMovement{
			InventoryID:        uint(pickingSheet.InventoryID),
			MovementID:         movementID,
			RefType:            "OUTBOUND CANCEL",
			RefID:              uint(payload.OutboundID),
			ItemID:             pickingSheet.ItemID,
			ItemCode:           pickingSheet.ItemCode,
			QtyOnhandChange:    pickingSheet.Quantity,
			QtyAvailableChange: 0,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   -pickingSheet.Quantity,
			FromWhsCode:        pickingSheet.WhsCode,
			FromLocation:       pickingSheet.Location,
			ToLocation:         toLocation,
			OldQaStatus:        pickingSheet.QaStatus,
			Reason:             outboundHeader.OutboundNo + " CANCEL",
			CreatedBy:          int(userID),
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&reverseMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record reverse movement",
			})
		}
	}

	cancelTime := time.Now()

	// UPDATE OUTBOUND STATUS
	if err := tx.Debug().
		Model(&models.OutboundHeader{}).
		Where("id = ?", payload.OutboundID).
		Updates(map[string]interface{}{
			// "shipment_id": outboundHeader.ShipmentID + " [CANCEL " + outboundHeader.OutboundNo + "]",
			"action_reason": payload.Reason,
			"status":        "cancel",
			"raw_status":    "CANCELLED",
			"cancel_time":   cancelTime,
			"cancel_by":     int(userID),
			"updated_by":    int(userID),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update Outbound Barcodes Status
	if err := tx.Debug().
		Model(&models.OutboundBarcode{}).
		Where("outbound_id = ?", payload.OutboundID).
		Updates(map[string]interface{}{
			"status":     "cancel",
			"updated_by": int(userID),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hard Delete from order_details
	if err := tx.Debug().
		Unscoped().
		Where("outbound_id = ?", payload.OutboundID).
		Delete(&models.OrderDetail{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound cancelled successfully"})
}

func (c *OutboundController) CreatePacking(ctx *fiber.Ctx) error {

	// Mulai transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	repositories := repositories.NewOutboundRepository(tx)

	packingNo, err := repositories.GeneratePackingNumber()

	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate packing number",
			"error":   err.Error(),
		})
	}

	// Create packing
	var packing models.OutboundPacking
	packing.PackingNo = packingNo
	packing.CreatedAt = time.Now()
	packing.CreatedBy = int(ctx.Locals("userID").(float64))
	if err := tx.Create(&packing).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create packing",
			"error":   err.Error(),
		})
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
		"message": "Packing created successfully",
		"data":    packing,
	})
}

func (c *OutboundController) GetAllPacking(ctx *fiber.Ctx) error {

	var outboundRepo = repositories.NewOutboundRepository(c.DB)

	packing, err := outboundRepo.GetPackingSummary()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get packing",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Get packing successfully",
		"data":    packing,
	})
}

func (c *OutboundController) GetPackingItems(ctx *fiber.Ctx) error {
	// ambil outbound_id dari params
	outboundID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid outbound ID"})
	}

	// ambil packing_no dari params URL
	packingNo := ctx.Params("packing_no")
	if packingNo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing packing_no"})
	}

	// call repository
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	items, err := outboundRepo.GetPackingItemsList(outboundID, packingNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Packing Items Found",
		"data": fiber.Map{
			"list": items,
		},
	})
}

func (c *OutboundController) GetSerialNumberList(ctx *fiber.Ctx) error {
	// ambil packing_no dari params URL
	outbound_no := ctx.Params("outbound_no")

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	header, err := outboundRepo.GetOutboundSummary(outbound_no)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	serialNumberList, err := outboundRepo.GetOutboundSerialNumber(header.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Packing Items Found",
		"data":    fiber.Map{"header": header, "items": serialNumberList},
	})
}
func (c *OutboundController) GetOutboundVasSummary(ctx *fiber.Ctx) error {
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	sum, err := outboundRepo.GetOutboundVasSum()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Packing Items Found",
		"data":    sum,
	})
}

func (c *OutboundController) GetOutboundVasByID(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	var outboundVas []models.OutboundVas
	if err := c.DB.Debug().Where("outbound_no = ?", outboundNo).Find(&outboundVas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": outboundVas})
}

func (c *OutboundController) GetOutboundBarcodeByOutboundNo(ctx *fiber.Ctx) error {
	outboundNo := ctx.Params("outbound_no")

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().Where("outbound_no = ?", outboundNo).First(&outboundHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var outboundBarcodes []models.OutboundBarcode
	if err := c.DB.Debug().
		Preload("Product").
		Where("outbound_id = ?", outboundHeader.ID).
		Find(&outboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": outboundBarcodes})
}

//======================================================================
// BEGIN PROCESS UPLOAD OUTBOUND FROM EXCEL
//======================================================================

type ExcelOutboundUploadResponse struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	TotalRows        int               `json:"total_rows"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	OutboundNumbers  []string          `json:"outbound_numbers,omitempty"`
	Errors           []ExcelRowError   `json:"errors,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

type ExcelOutboundHeader struct {
	OutboundDate    string
	ShipmentID      string
	CustomerCode    string
	WhsCode         string
	OwnerCode       string
	Remarks         string
	TransporterCode string
	PickerName      string
	CustAddress     string
	CustCity        string
	PlanPickupDate  string
	PlanPickupTime  string
	RcvDoDate       string
	RcvDoTime       string
	StartPickTime   string
	EndPickTime     string
	DelivTo         string
	DelivAddress    string
	DelivCity       string
	Driver          string
	QtyKoli         string
	QtyKoliSeal     string
	TruckSize       string
	TruckNo         string
}

type ExcelOutboundDetail struct {
	ItemCode  string
	UOM       string
	Quantity  float64
	ExpDate   string
	LotNumber string
	Location  string
	SN        string
	Remarks   string
	VasID     int
	Row       int
}

func (c *OutboundController) CreateOutboundFromExcelFile(ctx *fiber.Ctx) error {
	// Parse uploaded file
	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "No file uploaded or invalid file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Error", Detail: err.Error()},
			},
		})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Invalid file format. Only .xlsx and .xls files are allowed",
		})
	}

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "File size exceeds maximum limit of 10MB",
		})
	}

	// Open uploaded file
	fileHeader, err := file.Open()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to open uploaded file",
			Errors: []ExcelRowError{
				{Row: 0, Message: "File Processing Error", Detail: err.Error()},
			},
		})
	}
	defer fileHeader.Close()

	// Read Excel file
	excelFile, err := excelize.OpenReader(fileHeader)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to read Excel file. Please ensure the file is not corrupted",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Excel Read Error", Detail: err.Error()},
			},
		})
	}
	defer excelFile.Close()

	// Get first sheet
	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Excel file contains no sheets",
		})
	}

	sheetName := sheets[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to read rows from Excel",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Sheet Read Error", Detail: err.Error()},
			},
		})
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Excel file must contain at least header row and one data row",
		})
	}

	// Parse header information from first data row
	headerInfo, err := c.parseOutboundHeaderFromExcel(rows)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to parse header information",
			ValidationErrors: []ValidationError{
				{Field: "Header", Message: err.Error(), Row: 1},
			},
		})
	}

	// Get user ID
	userID := int(ctx.Locals("userID").(float64))

	// Start transaction for validation
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Panic recovered in CreateOutboundFromExcelFile: %v", r)
		}
	}()

	// Validate inventory policy
	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", headerInfo.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Inventory Policy not found for owner: " + headerInfo.OwnerCode,
				Errors: []ExcelRowError{
					{Row: 1, Message: "Inventory Policy Error", Detail: "Owner code: " + headerInfo.OwnerCode},
				},
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to get inventory policy",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Error", Detail: err.Error()},
			},
		})
	}

	// Validate customer exists
	var customer models.Customer
	if err := tx.First(&customer, "customer_code = ?", headerInfo.CustomerCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Customer not found: " + headerInfo.CustomerCode,
				Errors: []ExcelRowError{
					{Row: 1, Message: "Customer Not Found", Detail: "Customer code: " + headerInfo.CustomerCode},
				},
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to validate customer",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Error", Detail: err.Error()},
			},
		})
	}

	// Validate Delivery to is exist
	var customerTo models.Customer
	if err := tx.First(&customerTo, "customer_code = ?", headerInfo.DelivTo).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Delivery to not found: " + headerInfo.DelivTo,
				Errors: []ExcelRowError{
					{Row: 1, Message: "Delivery to Not Found", Detail: "Customer code: " + headerInfo.DelivTo},
				},
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to validate delivery to",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Error", Detail: err.Error()},
			},
		})
	}

	// Validate Transporter is exists
	var transporter models.Transporter
	if err := tx.First(&transporter, "transporter_code = ?", headerInfo.TransporterCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Transporter not found: " + headerInfo.TransporterCode,
				Errors: []ExcelRowError{
					{Row: 1, Message: "Transporter Not Found", Detail: "Transporter code: " + headerInfo.TransporterCode},
				},
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to validate transporter",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Error", Detail: err.Error()},
			},
		})
	}

	// Validate Warehouse Code is exists
	var warehouse models.Warehouse
	if err := tx.First(&warehouse, "code = ?", headerInfo.WhsCode).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Warehouse not found: " + headerInfo.WhsCode,
				Errors: []ExcelRowError{
					{Row: 1, Message: "Warehouse Not Found", Detail: "Warehouse code: " + headerInfo.WhsCode},
				},
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to validate warehouse",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Error", Detail: err.Error()},
			},
		})
	}

	// Parse detail rows
	details, validationErrors := c.parseOutboundDetailsFromExcel(rows, inventoryPolicy)
	if len(validationErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success:          false,
			Message:          fmt.Sprintf("Validation failed with %d errors", len(validationErrors)),
			ValidationErrors: validationErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	if len(details) < 1 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success:   false,
			Message:   "No valid items found in Excel file",
			TotalRows: len(rows) - 1,
		})
	}

	// Check for duplicate items
	duplicateErrors := c.checkDuplicateOutboundItems(details)
	if len(duplicateErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success:          false,
			Message:          "Duplicate items found in Excel file",
			ValidationErrors: duplicateErrors,
			TotalRows:        len(rows) - 1,
		})
	}

	// Validate all products exist and UOM conversions are valid
	productValidationErrors := c.validateOutboundProducts(tx, details)
	if len(productValidationErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(ExcelOutboundUploadResponse{
			Success:          false,
			Message:          "Product validation failed",
			ValidationErrors: productValidationErrors,
			TotalRows:        len(details),
		})
	}

	repositories := repositories.NewOutboundRepository(tx)

	// Generate outbound number
	outboundNo, err := repositories.GenerateOutboundNumber()
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to generate outbound number",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Outbound Generation Error", Detail: err.Error()},
			},
		})
	}

	// Create outbound header
	outboundHeader := models.OutboundHeader{
		OutboundNo:      outboundNo,
		OutboundDate:    headerInfo.OutboundDate,
		CustomerCode:    customer.CustomerCode,
		ShipmentID:      headerInfo.ShipmentID,
		WhsCode:         headerInfo.WhsCode,
		OwnerCode:       headerInfo.OwnerCode,
		Remarks:         headerInfo.Remarks,
		Status:          "open",
		RawStatus:       "DRAFT",
		DraftTime:       time.Now(),
		TransporterCode: transporter.TransporterCode,
		PickerName:      headerInfo.PickerName,
		CustAddress:     customer.CustAddr1,
		CustCity:        customer.CustCity,
		PlanPickupDate:  headerInfo.PlanPickupDate,
		PlanPickupTime:  headerInfo.PlanPickupTime,
		RcvDoDate:       headerInfo.RcvDoDate,
		RcvDoTime:       headerInfo.RcvDoTime,
		StartPickTime:   headerInfo.StartPickTime,
		EndPickTime:     headerInfo.EndPickTime,
		DelivTo:         customerTo.CustomerCode,
		DelivAddress:    customerTo.CustAddr1,
		DelivCity:       customerTo.CustCity,
		Driver:          headerInfo.Driver,
		// QtyKoli:         headerInfo.QtyKoli,
		// QtyKoliSeal:     headerInfo.QtyKoliSeal,
		TruckSize: headerInfo.TruckSize,
		TruckNo:   headerInfo.TruckNo,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	if err := tx.Create(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to create outbound header",
			Errors: []ExcelRowError{
				{Row: 1, Message: "Database Insert Error", Detail: err.Error()},
			},
		})
	}

	successCount := 0

	// Create outbound details
	for _, detail := range details {
		// Get product info
		var product models.Product
		if err := tx.First(&product, "item_code = ?", detail.ItemCode).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Product not found during detail creation",
				Errors: []ExcelRowError{
					{Row: detail.Row, Message: "Product Not Found", Detail: "Item code: " + detail.ItemCode},
				},
			})
		}

		// Get UOM conversion
		var uomConversion models.UomConversion
		if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "UOM conversion not found during detail creation",
				Errors: []ExcelRowError{
					{Row: detail.Row, Message: "UOM Not Found", Detail: fmt.Sprintf("Item: %s, UOM: %s", detail.ItemCode, detail.UOM)},
				},
			})
		}

		// Get VAS info if VasID is provided
		var vas models.Vas
		vasName := ""

		if inventoryPolicy.UseVAS {
			if detail.VasID > 0 {
				if err := tx.First(&vas, "id = ?", detail.VasID).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						tx.Rollback()
						return ctx.Status(fiber.StatusNotFound).JSON(ExcelOutboundUploadResponse{
							Success: false,
							Message: "VAS not found",
							Errors: []ExcelRowError{
								{Row: detail.Row, Message: "VAS Not Found", Detail: fmt.Sprintf("VAS ID: %d", detail.VasID)},
							},
						})
					}
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
						Success: false,
						Message: "Failed to validate VAS",
						Errors: []ExcelRowError{
							{Row: detail.Row, Message: "Database Error", Detail: err.Error()},
						},
					})
				}
				vasName = vas.Name
			}
		}

		// Create outbound detail
		outboundDetail := models.OutboundDetail{
			OutboundNo:   outboundNo,
			OutboundID:   outboundHeader.ID,
			ItemCode:     detail.ItemCode,
			ItemID:       int(product.ID),
			Barcode:      uomConversion.Ean,
			CustomerCode: customer.CustomerCode,
			Uom:          detail.UOM,
			Quantity:     detail.Quantity,
			ExpDate:      detail.ExpDate,
			LotNumber:    detail.LotNumber,
			WhsCode:      headerInfo.WhsCode,
			DivisionCode: "REGULAR",
			Location:     detail.Location,
			QaStatus:     "A",
			SN:           detail.SN,
			SNCheck:      "N",
			OwnerCode:    headerInfo.OwnerCode,
			Remarks:      detail.Remarks,
			VasID:        detail.VasID,
			VasName:      vasName,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}

		if err := tx.Create(&outboundDetail).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
				Success: false,
				Message: "Failed to create outbound detail",
				Errors: []ExcelRowError{
					{Row: detail.Row, Message: "Database Insert Error", Detail: err.Error()},
				},
			})
		}

		successCount++
	}

	// Insert transaction history
	if err := helpers.InsertTransactionHistory(tx, outboundNo, "open", "OUTBOUND", "Created from Excel upload", userID); err != nil {
		log.Printf("Warning: Failed to insert transaction history for %s: %v", outboundNo, err)
		// Don't rollback for history error, just log it
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(ExcelOutboundUploadResponse{
			Success: false,
			Message: "Failed to commit transaction",
			Errors: []ExcelRowError{
				{Row: 0, Message: "Transaction Commit Error", Detail: err.Error()},
			},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(ExcelOutboundUploadResponse{
		Success:         true,
		Message:         fmt.Sprintf("Successfully created outbound %s with %d items", outboundNo, successCount),
		TotalRows:       len(details),
		SuccessCount:    successCount,
		FailedCount:     0,
		OutboundNumbers: []string{outboundNo},
	})
}

// Helper functions
func (c *OutboundController) parseOutboundHeaderFromExcel(rows [][]string) (*ExcelOutboundHeader, error) {
	if len(rows) < 2 {
		return nil, errors.New("no header data found")
	}

	// Parse from first data row (row index 1)
	row := rows[1]
	header := &ExcelOutboundHeader{
		OutboundDate:    strings.TrimSpace(getCell(row, 0)),
		ShipmentID:      strings.TrimSpace(getCell(row, 1)),
		CustomerCode:    strings.TrimSpace(getCell(row, 2)),
		WhsCode:         strings.TrimSpace(getCell(row, 3)),
		OwnerCode:       strings.TrimSpace(getCell(row, 4)),
		TransporterCode: strings.TrimSpace(getCell(row, 5)),
		PickerName:      strings.TrimSpace(getCell(row, 6)),
		CustAddress:     strings.TrimSpace(getCell(row, 7)),
		CustCity:        strings.TrimSpace(getCell(row, 8)),
		PlanPickupDate:  strings.TrimSpace(getCell(row, 9)),
		PlanPickupTime:  strings.TrimSpace(getCell(row, 10)),
		RcvDoDate:       strings.TrimSpace(getCell(row, 11)),
		RcvDoTime:       strings.TrimSpace(getCell(row, 12)),
		DelivTo:         strings.TrimSpace(getCell(row, 13)),
		DelivAddress:    strings.TrimSpace(getCell(row, 14)),
		DelivCity:       strings.TrimSpace(getCell(row, 15)),
		Driver:          strings.TrimSpace(getCell(row, 16)),
		TruckNo:         strings.TrimSpace(getCell(row, 17)),
		TruckSize:       strings.TrimSpace(getCell(row, 18)),
		QtyKoli:         strings.TrimSpace(getCell(row, 19)),
		QtyKoliSeal:     strings.TrimSpace(getCell(row, 20)),
		Remarks:         strings.TrimSpace(getCell(row, 21)),
	}

	// Validate required fields
	if header.CustomerCode == "" {
		return nil, errors.New("customer code is required")
	}
	if header.WhsCode == "" {
		return nil, errors.New("warehouse code is required")
	}
	if header.OwnerCode == "" {
		return nil, errors.New("owner code is required")
	}
	if header.OutboundDate == "" {
		return nil, errors.New("outbound date is required")
	}

	return header, nil
}

func (c *OutboundController) parseOutboundDetailsFromExcel(rows [][]string, policy models.InventoryPolicy) ([]ExcelOutboundDetail, []ValidationError) {
	var details []ExcelOutboundDetail
	var errors []ValidationError

	// Start from row 2 (index 1), assuming row 1 is header
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		// Skip empty rows
		if len(row) == 0 || strings.TrimSpace(getCell(row, 22)) == "" {
			continue
		}

		detail := ExcelOutboundDetail{Row: rowNum}

		// Parse item details starting from column 22 (index 22)
		detail.ItemCode = strings.TrimSpace(getCell(row, 22))
		detail.UOM = strings.TrimSpace(getCell(row, 23))

		qtyStr := strings.TrimSpace(getCell(row, 24))
		if qtyStr != "" {
			qty, err := strconv.ParseFloat(qtyStr, 64)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   "Quantity",
					Message: "Invalid quantity format: " + qtyStr,
					Row:     rowNum,
				})
				continue
			}
			detail.Quantity = qty
		}

		ExpDate, err := getCellAsDateStrict(row, 27)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "ExpDate",
				Message: "Invalid ExpDate format: " + ExpDate,
				Row:     rowNum,
			})
			continue
		}

		detail.Location = strings.TrimSpace(getCell(row, 25))
		detail.LotNumber = strings.TrimSpace(getCell(row, 26))
		// detail.ExpDate = strings.TrimSpace(getCell(row, 27))
		detail.ExpDate = ExpDate
		detail.SN = strings.TrimSpace(getCell(row, 28))

		vasIDStr := strings.TrimSpace(getCell(row, 29))
		if vasIDStr != "" {
			vasID, err := strconv.Atoi(vasIDStr)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   "VasID",
					Message: "Invalid VAS ID format: " + vasIDStr,
					Row:     rowNum,
				})
				continue
			}
			detail.VasID = vasID
		}

		detail.Remarks = strings.TrimSpace(getCell(row, 30))

		// Validate required fields
		if detail.ItemCode == "" {
			errors = append(errors, ValidationError{
				Field:   "ItemCode",
				Message: "Item code cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		if detail.UOM == "" {
			errors = append(errors, ValidationError{
				Field:   "UOM",
				Message: "UOM cannot be empty",
				Row:     rowNum,
			})
			continue
		}

		if detail.Quantity == 0 {
			errors = append(errors, ValidationError{
				Field:   "Quantity",
				Message: "Quantity cannot be zero",
				Row:     rowNum,
			})
			continue
		}

		if detail.Quantity < 0 {
			errors = append(errors, ValidationError{
				Field:   "Quantity",
				Message: "Quantity cannot be negative",
				Row:     rowNum,
			})
			continue
		}

		// Validate based on inventory policy
		if policy.RequireLotNumber && detail.LotNumber == "" {
			errors = append(errors, ValidationError{
				Field:   "LotNumber",
				Message: "Lot number is required by inventory policy",
				Row:     rowNum,
			})
			continue
		}

		details = append(details, detail)
	}

	return details, errors
}

func (c *OutboundController) checkDuplicateOutboundItems(details []ExcelOutboundDetail) []ValidationError {
	var errors []ValidationError
	itemMap := make(map[string]int)

	for _, detail := range details {
		// Key based on ItemCode and UOM only (same as original validation)
		key := fmt.Sprintf("%s|%s", detail.ItemCode, detail.UOM)

		if existingRow, exists := itemMap[key]; exists {
			errors = append(errors, ValidationError{
				Field: "Duplicate",
				Message: fmt.Sprintf("Duplicate item found (same as row %d): Item Code %s, UOM %s",
					existingRow, detail.ItemCode, detail.UOM),
				Row: detail.Row,
			})
		} else {
			itemMap[key] = detail.Row
		}
	}

	return errors
}

func (c *OutboundController) validateOutboundProducts(tx *gorm.DB, details []ExcelOutboundDetail) []ValidationError {
	var errorss []ValidationError

	for _, detail := range details {
		// Check if product exists
		var product models.Product
		if err := tx.First(&product, "item_code = ?", detail.ItemCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				errorss = append(errorss, ValidationError{
					Field:   "ItemCode",
					Message: "Product not found: " + detail.ItemCode,
					Row:     detail.Row,
				})
			} else {
				errorss = append(errorss, ValidationError{
					Field:   "ItemCode",
					Message: "Failed to validate product: " + err.Error(),
					Row:     detail.Row,
				})
			}
			continue
		}

		// Check if UOM conversion exists
		var uomConversion models.UomConversion
		if err := tx.First(&uomConversion, "item_code = ? AND from_uom = ?", product.ItemCode, detail.UOM).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				errorss = append(errorss, ValidationError{
					Field:   "UOM",
					Message: fmt.Sprintf("UOM conversion not found for Item: %s, UOM: %s", detail.ItemCode, detail.UOM),
					Row:     detail.Row,
				})
			} else {
				errorss = append(errorss, ValidationError{
					Field:   "UOM",
					Message: "Failed to validate UOM: " + err.Error(),
					Row:     detail.Row,
				})
			}
			continue
		}

		// Validate VAS if provided
		if detail.VasID > 0 {
			var vas models.Vas
			if err := tx.First(&vas, "id = ?", detail.VasID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					errorss = append(errorss, ValidationError{
						Field:   "VasID",
						Message: fmt.Sprintf("VAS not found with ID: %d", detail.VasID),
						Row:     detail.Row,
					})
				} else {
					errorss = append(errorss, ValidationError{
						Field:   "VasID",
						Message: "Failed to validate VAS: " + err.Error(),
						Row:     detail.Row,
					})
				}
			}
		}
	}

	return errorss
}

//======================================================================
// END PROCESS UPLOAD OUTBOUND FROM EXCEL
//======================================================================

//======================================================================
// BEGIN PROCESS OUTBOUND FROM PDF
//======================================================================

// ParseResult represents the response from Python parser
type ParseResult struct {
	Success bool                `json:"success"`
	Data    *ParsedOutboundData `json:"data,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// ParsedOutboundData represents the extracted data from PDF
type ParsedOutboundData struct {
	DocNo             string `json:"docNo"`
	Vendor            string `json:"vendor"`
	ItemName          string `json:"itemName"`
	SKU               string `json:"sku"`
	BatchNo           string `json:"batchNo"`
	Qty               string `json:"qty"`
	Location          string `json:"location"`
	ConsignmentPeriod string `json:"consignmentPeriod"`
}

// OutboundCreateRequest represents the final request to create outbound
type OutboundCreateRequest struct {
	DocNo             string `json:"docNo" validate:"required"`
	Vendor            string `json:"vendor" validate:"required"`
	ItemName          string `json:"itemName" validate:"required"`
	SKU               string `json:"sku" validate:"required"`
	BatchNo           string `json:"batchNo" validate:"required"`
	Qty               int    `json:"qty" validate:"required,min=1"`
	Location          string `json:"location" validate:"required"`
	ConsignmentPeriod string `json:"consignmentPeriod,omitempty"`
}

// ParseOutboundFromPDFFile parses PDF file and extracts outbound data
func (c *OutboundController) ParseOutboundFromPDFFile(ctx *fiber.Ctx) error {
	// Get uploaded file
	file, err := ctx.FormFile("pdf")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "No PDF file provided",
		})
	}

	// Validate file type
	if filepath.Ext(file.Filename) != ".pdf" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Only PDF files are allowed",
		})
	}

	// Create temp directory if not exists
	tempDir := "./temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create temp directory",
		})
	}

	// Save file to temp location
	timestamp := time.Now().UnixNano()
	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("upload_%d.pdf", timestamp))

	if err := ctx.SaveFile(file, tempFilePath); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to save uploaded file",
		})
	}

	// Clean up temp file after processing
	defer os.Remove(tempFilePath)

	// Call Python parser
	result, err := c.callPythonParser(tempFilePath)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to parse PDF: %s", err.Error()),
		})
	}

	// Return parsed result
	return ctx.JSON(result)
}

// callPythonParser executes Python script and returns parsed data
func (c *OutboundController) callPythonParser(pdfPath string) (*ParseResult, error) {
	// Path to Python script (adjust this path according to your setup)
	pythonScript := "./scripts/parse_consignment_pdf.py"

	// Check if Python script exists
	if _, err := os.Stat(pythonScript); os.IsNotExist(err) {
		return nil, fmt.Errorf("Python parser script not found at %s", pythonScript)
	}

	// Execute Python script
	cmd := exec.Command("python", pythonScript, pdfPath)
	// cmd := exec.Command("python --version")

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Python execution failed: %s, output: %s", err.Error(), string(output))
	}

	// Parse JSON result
	var result ParseResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("Failed to parse Python output: %s, raw output: %s", err.Error(), string(output))
	}

	return &result, nil
}

// OutboundFromPdfPayload represents the data from PDF upload
type OutboundFromPdfPayload struct {
	DocNo             string `form:"docNo"`
	Vendor            string `form:"vendor"`
	ItemName          string `form:"itemName"`
	SKU               string `form:"sku"`
	BatchNo           string `form:"batchNo"`
	Qty               string `form:"qty"`
	Location          string `form:"location"`
	ConsignmentPeriod string `form:"consignmentPeriod"`
}

// CreateOutboundFromPdf handles PDF-based outbound order creation with comprehensive validation
func (c *OutboundController) CreateOutboundFromPdf(ctx *fiber.Ctx) error {
	var payload OutboundFromPdfPayload
	var validationErrors []ValidationError

	// Parse form data
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	fmt.Println("Create Outbound From PDF Payload:", payload)

	// Collect all validation errors before returning
	validationErrors = c.validateOutboundFromPdf(payload)

	// If there are validation errors, return them all at once
	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			"errors":  validationErrors,
		})
	}

	// Convert quantity to integer
	qty, err := strconv.Atoi(payload.Qty)
	if err != nil {
		validationErrors = append(validationErrors, ValidationError{
			Field:   "qty",
			Message: "Quantity must be a valid number",
		})
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid quantity format",
			"errors":  validationErrors,
		})
	}

	// Start database transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	fmt.Println("Start DB Transaction for PDF Outbound")

	ownerCode := "YUWELL" // Assuming owner code is fixed for this example
	// Validate and get inventory policy
	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", ownerCode).First(&inventoryPolicy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "vendor",
				Message: fmt.Sprintf("Vendor/Owner '%s' not found in inventory policy", ownerCode),
			})
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory Policy not found",
				"errors":  validationErrors,
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get inventory policy",
			"error":   err.Error(),
		})
	}

	// Validate product/item exists
	var product models.Product
	if err := tx.Where("item_code = ?", payload.SKU).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "sku",
				Message: fmt.Sprintf("Product with SKU '%s' not found", payload.SKU),
			})
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Product not found",
				"errors":  validationErrors,
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get product",
			"error":   err.Error(),
		})
	}

	// Validate item name matches product
	if product.ItemName != payload.ItemName {
		validationErrors = append(validationErrors, ValidationError{
			Field: "itemName",
			Message: fmt.Sprintf("Item name '%s' does not match product name '%s' for SKU '%s'",
				payload.ItemName, product.ItemName, payload.SKU),
		})
	}

	// Validate vendor matches customer
	var customer models.Customer
	if err := tx.Where("customer_code = ?", payload.Vendor).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "vendor",
				Message: fmt.Sprintf("Vendor/Customer '%s' not found", payload.Vendor),
			})
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Vendor/Customer not found",
				"errors":  validationErrors,
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get customer",
			"error":   err.Error(),
		})
	}

	// Check lot number requirement
	// if inventoryPolicy.RequireLotNumber && payload.BatchNo == "" {
	// 	validationErrors = append(validationErrors, ValidationError{
	// 		Field:   "batchNo",
	// 		Message: "Batch/Lot number is required for this vendor",
	// 	})
	// }

	// If there are validation errors after all checks, return them
	if len(validationErrors) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Validation failed with %d error(s)", len(validationErrors)),
			"errors":  validationErrors,
		})
	}

	// Generate outbound number
	repositories := repositories.NewOutboundRepository(tx)
	outboundNo, err := repositories.GenerateOutboundNumber()
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate outbound number",
			"error":   err.Error(),
		})
	}

	fmt.Println("Generated Outbound No:", outboundNo)

	// Get user ID from context
	userID := int(ctx.Locals("userID").(float64))

	// Create outbound header
	var outboundHeader models.OutboundHeader
	outboundHeader.OutboundNo = outboundNo
	outboundHeader.OutboundDate = time.Now().Format("2006-01-02")
	outboundHeader.ShipmentID = payload.DocNo // Using DocNo as ShipmentID
	outboundHeader.WhsCode = "WH-B"           // You may need to adjust this based on your requirements
	outboundHeader.OwnerCode = ownerCode
	outboundHeader.CustomerCode = customer.CustomerCode
	outboundHeader.CustAddress = customer.CustAddr1
	outboundHeader.CustCity = customer.CustCity
	outboundHeader.DelivTo = customer.CustomerCode
	outboundHeader.DelivAddress = customer.CustAddr1
	outboundHeader.DelivCity = customer.CustCity
	outboundHeader.Remarks = fmt.Sprintf("Created from PDF - Consignment Period: %s", payload.ConsignmentPeriod)
	outboundHeader.CreatedBy = userID
	outboundHeader.UpdatedBy = userID
	outboundHeader.Status = "open"
	outboundHeader.RawStatus = "DRAFT"
	outboundHeader.DraftTime = time.Now()
	outboundHeader.DelivAddress = payload.Location
	outboundHeader.CreatedAt = time.Now()
	outboundHeader.UpdatedAt = time.Now()

	// Insert outbound header
	if err := tx.Create(&outboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create outbound header",
			"error":   err.Error(),
		})
	}

	// Get the default UOM for the product
	var uomConversion models.UomConversion
	if err := tx.Where("item_code = ?", product.ItemCode).First(&uomConversion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErrors = append(validationErrors, ValidationError{
				Field:   "sku",
				Message: fmt.Sprintf("UOM conversion not found for product SKU '%s'", payload.SKU),
			})
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "UOM conversion not found",
				"errors":  validationErrors,
			})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get UOM conversion",
			"error":   err.Error(),
		})
	}

	// Create outbound detail
	var outboundDetail models.OutboundDetail
	outboundDetail.OutboundNo = outboundNo
	outboundDetail.OutboundID = outboundHeader.ID
	outboundDetail.ItemCode = product.ItemCode
	outboundDetail.ItemID = int(product.ID)
	outboundDetail.Barcode = uomConversion.Ean
	outboundDetail.Uom = uomConversion.FromUom
	outboundDetail.Quantity = float64(qty)
	outboundDetail.LotNumber = payload.BatchNo
	outboundDetail.WhsCode = outboundHeader.WhsCode
	outboundDetail.DivisionCode = "REGULAR"
	outboundDetail.Location = payload.Location
	outboundDetail.QaStatus = "A"
	outboundDetail.SNCheck = "N"
	outboundDetail.OwnerCode = "YUWELL"
	outboundDetail.Remarks = fmt.Sprintf("From PDF: %s", payload.DocNo)
	outboundDetail.CreatedBy = userID
	outboundDetail.UpdatedBy = userID

	// Insert outbound detail
	if err := tx.Create(&outboundDetail).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create outbound detail",
			"error":   err.Error(),
		})
	}

	fmt.Println("Outbound created successfully, ID:", outboundHeader.ID)

	// Commit transaction
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
		"message": "Outbound order created successfully from PDF",
		"data": fiber.Map{
			"outbound_id": outboundHeader.ID,
			"outbound_no": outboundNo,
		},
	})
}

// validateOutboundFromPdf performs comprehensive validation and returns all errors at once
func (c *OutboundController) validateOutboundFromPdf(payload OutboundFromPdfPayload) []ValidationError {
	var errors []ValidationError

	// Validate Document Number
	if payload.DocNo == "" {
		errors = append(errors, ValidationError{
			Field:   "docNo",
			Message: "Document number is required",
		})
	} else if len(payload.DocNo) < 3 {
		errors = append(errors, ValidationError{
			Field:   "docNo",
			Message: "Document number must be at least 3 characters",
		})
	}

	// Validate Vendor
	if payload.Vendor == "" {
		errors = append(errors, ValidationError{
			Field:   "vendor",
			Message: "Vendor/Goods Owner is required",
		})
	}

	// Validate Item Name
	if payload.ItemName == "" {
		errors = append(errors, ValidationError{
			Field:   "itemName",
			Message: "Item name is required",
		})
	}

	// Validate SKU
	if payload.SKU == "" {
		errors = append(errors, ValidationError{
			Field:   "sku",
			Message: "SKU is required",
		})
	}

	// Validate Batch Number
	if payload.BatchNo == "" {
		errors = append(errors, ValidationError{
			Field:   "batchNo",
			Message: "Batch number is required",
		})
	}

	// Validate Quantity
	if payload.Qty == "" {
		errors = append(errors, ValidationError{
			Field:   "qty",
			Message: "Quantity is required",
		})
	} else {
		qty, err := strconv.Atoi(payload.Qty)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   "qty",
				Message: "Quantity must be a valid number",
			})
		} else if qty <= 0 {
			errors = append(errors, ValidationError{
				Field:   "qty",
				Message: "Quantity must be greater than zero",
			})
		}
	}

	// Validate Location
	if payload.Location == "" {
		errors = append(errors, ValidationError{
			Field:   "location",
			Message: "Storage location is required",
		})
	} else if len(payload.Location) < 10 {
		errors = append(errors, ValidationError{
			Field:   "location",
			Message: "Storage location must be at least 10 characters",
		})
	}

	return errors
}

//======================================================================
// END PROCESS OUTBOUND FROM PDF
//======================================================================

func createOutboundSerial(
	tx *gorm.DB,
	outboundID uint,
	detailID uint,
	serialNumbers []string,
	userID int,
) error {

	for _, sn := range serialNumbers {

		sn = strings.TrimSpace(sn)

		if sn == "" {
			continue
		}

		serial := models.OutboundSerial{
			OutboundId:       int(outboundID),
			OutboundDetailId: int(detailID),
			SerialNumber:     sn,
			CreatedBy:        userID,
			UpdatedBy:        userID,
		}

		if err := tx.Create(&serial).Error; err != nil {
			return err
		}
	}

	return nil
}
