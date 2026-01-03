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
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type OutboundController struct {
	DB *gorm.DB
}

func NewOutboundController(DB *gorm.DB) *OutboundController {
	return &OutboundController{DB: DB}
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
	ID         int               `json:"ID"`
	OutboundID types.SnowflakeID `json:"outbound_id"`
	ItemCode   string            `json:"item_code"`
	Quantity   float64           `json:"quantity"`
	UOM        string            `json:"uom"`
	SN         string            `json:"sn"`
	Location   string            `json:"location"`
	Remarks    string            `json:"remarks"`
	Mode       string            `json:"mode"`
	VasID      int               `json:"vas_id"`
	ExpDate    string            `json:"exp_date"`
	LotNumber  string            `json:"lot_number"`
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

		key := fmt.Sprintf("%s|%s", item.ItemCode, item.UOM)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item with code %s,  uom %s",
					item.ItemCode, item.UOM),
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
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert outbound header",
			"error":   res.Error.Error(),
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
		OutboundDetail.WhsCode = OutboundHeader.WhsCode
		OutboundDetail.DivisionCode = "REGULAR"
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
		Preload("OutboundDetails.Product"). // ✅ ambil product termasuk item_name
		First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": OutboundHeader, "message": "Outbound found"})
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

		// if InventoryPolicy.UseLotNo {
		// 	if item.LotNumber == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Lot number cannot be empty",
		// 			"error":   "Lot number cannot be empty",
		// 		})
		// 	}
		// }

		// if InventoryPolicy.UseProductionDate {
		// 	if item.ProdDate == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Production date cannot be empty",
		// 			"error":   "Production date cannot be empty",
		// 		})
		// 	}
		// }

		// if InventoryPolicy.UseReceiveLocation {
		// 	if item.Location == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Receive location cannot be empty",
		// 			"error":   "Receive location cannot be empty",
		// 		})
		// 	}
		// }

		// if InventoryPolicy.UseFEFO {
		// 	if item.ExpDate == "" {
		// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 			"success": false,
		// 			"message": "Expiration date cannot be empty",
		// 			"error":   "Expiration date cannot be empty",
		// 		})
		// 	}
		// }

		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item code cannot be empty",
				"error":   "Item code cannot be empty",
			})
		}

		// if itemCodes[item.ItemCode] {
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 		"success": false,
		// 		"message": "Duplicate item code found: " + item.ItemCode,
		// 		"error":   "Duplicate item code for item code " + item.ItemCode,
		// 	})
		// }

		// itemCodes[item.ItemCode] = true // tandai sebagai sudah ditemukan

		key := fmt.Sprintf("%s|%s", item.ItemCode, item.UOM)

		if itemCodes[key] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found: " + item.ItemCode,
				"error": fmt.Sprintf("Duplicate item with code %s,  uom %s",
					item.ItemCode, item.UOM),
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
					Location:     item.Location,
					WhsCode:      OutboundHeader.WhsCode,
					OwnerCode:    OutboundHeader.OwnerCode,
					CustomerCode: customer.CustomerCode,
					Uom:          item.UOM,
					DivisionCode: "REGULAR",
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
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}

			}

		} else if err == nil {
			// ✅ Ditemukan → update

			if OutboundHeader.Status == "open" {
				outboundDetail.OutboundID = OutboundHeader.ID
				outboundDetail.ItemID = int(product.ID)
				outboundDetail.ItemCode = item.ItemCode
				outboundDetail.Barcode = uomConversion.Ean
				outboundDetail.Uom = item.UOM
				outboundDetail.WhsCode = OutboundHeader.WhsCode
				outboundDetail.OwnerCode = OutboundHeader.OwnerCode
				outboundDetail.DivisionCode = "REGULAR"
				outboundDetail.CustomerCode = customer.CustomerCode
				outboundDetail.QaStatus = "A"
				outboundDetail.ExpDate = item.ExpDate
				outboundDetail.LotNumber = item.LotNumber
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
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var invetoryPolicy models.InventoryPolicy
	if err := tx.Debug().First(&invetoryPolicy, "owner_code = ?", outboundHeader.OwnerCode).Error; err != nil {
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
		for _, item := range outboundDetails {
			if item.LotNumber == "" {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Lot number is required",
					"error":   "Lot number is required",
				})
			}
		}
	}

	uomRepo := repositories.NewUomRepository(tx)

	// Proses picking by FIFO
	for _, outboundDetail := range outboundDetails {

		// Convert to base UOM
		uomConversion, err := uomRepo.ConversionQty(outboundDetail.ItemCode, outboundDetail.Quantity, outboundDetail.Uom)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "UOM Conversion Error: " + err.Error()})
		}

		// qtyReq := outboundDetail.Quantity
		qtyReq := uomConversion.QtyConverted

		fmt.Println("Picking Query")

		queryInventory := tx.Debug().
			Where("item_id = ? AND whs_code = ? AND qty_available > 0 AND uom = ? AND owner_code = ?",
				outboundDetail.ItemID, outboundDetail.WhsCode, uomConversion.ToUom, outboundHeader.OwnerCode)

		if invetoryPolicy.UseFEFO && invetoryPolicy.UseLotNo && invetoryPolicy.RequireLotNumber {
			queryInventory = queryInventory.Where("lot_number = ?", outboundDetail.LotNumber).
				Order("rec_date, pallet, location ASC")
		} else if invetoryPolicy.UseFEFO {
			queryInventory = queryInventory.Order("exp_date, rec_date, qty_available, pallet, location ASC")
		} else {
			queryInventory = queryInventory.Order("rec_date, qty_available, pallet, location ASC")
		}

		var inventories []models.Inventory

		if err := queryInventory.Find(&inventories).Error; err != nil {
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

		// if err := tx.Debug().
		// 	Where("item_id = ? AND whs_code = ? AND qty_available > 0 AND uom = ? AND owner_code = ?",
		// 		outboundDetail.ItemID, outboundDetail.WhsCode, uomConversion.ToUom, outboundHeader.OwnerCode).
		// 	Order("rec_date, pallet, location ASC").
		// 	Find(&inventories).Error; err != nil {
		// 	tx.Rollback()
		// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		// }

		if len(inventories) == 0 {
			tx.Rollback()
			// return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			// 	"error": "Item " + outboundDetail.ItemCode + " not found",
			// })
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf(
					"Failed to fetch inventory for ItemCode: %s (ItemID: %d, Whs: %s, UOM: %s, Owner: %s). Detail: %s",
					outboundDetail.ItemCode,
					outboundDetail.ItemID,
					outboundDetail.WhsCode,
					uomConversion.ToUom,
					outboundHeader.OwnerCode,
					"Insufficient stock available",
				),
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

	if invetoryPolicy.RequirePickingScan {
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
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Scan picking not complete"})
			}
		}
	} else {

		err := repo.InsertIntoOutboundBarcodeFromOutboundPicking(tx, ctx, outboundHeader.ID) // insert into outbound barcodes
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "message": "Failed to insert into outbound barcodes"})
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
			// ToWhsCode:          input.ToWhsCode,
			FromLocation: pickingSheet.Location,
			// ToLocation:         input.ToLocation,
			OldQaStatus: pickingSheet.QaStatus,
			// NewQaStatus:        newQaStatus,
			Reason:    outboundHeader.OutboundNo + " COMPLETE",
			CreatedBy: int(ctx.Locals("userID").(float64)),
			CreatedAt: time.Now(),
		}

		if err := tx.Create(&sourceMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record source movement",
			})
		}

	}

	// UPDATE OUTBOUND STATUS
	if err := tx.Debug().
		Model(&models.OutboundHeader{}).
		Where("id = ?", inputBody.OutboundID).
		Updates(map[string]interface{}{
			"status":        "complete",
			"raw_status":    "COMPLETED",
			"complete_time": time.Now(),
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

		// for _, handling := range item.Handling {

		// 	var handlingRate models.HandlingRate
		// 	if err := tx.Debug().
		// 		Where("name = ?", handling).
		// 		Order("handling_rates.id DESC").
		// 		Take(&handlingRate).Error; err != nil {

		// 		tx.Rollback()
		// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		// 			"error": err.Error(),
		// 		})
		// 	}

		// 	var handlingSelected models.Handling
		// 	if err := tx.Debug().
		// 		Where("id = ?", handlingRate.HandlingId).
		// 		Take(&handlingSelected).Error; err != nil {

		// 		tx.Rollback()
		// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		// 			"error": err.Error(),
		// 		})
		// 	}

		// 	var totalPrice = 0
		// 	var qtyHandling = 0

		// 	if handlingSelected.IsKoli {
		// 		qtyHandling = 1
		// 		totalPrice = handlingRate.RateIdr * qtyHandling
		// 	} else {
		// 		qtyHandling = outboundDetail.Quantity
		// 		totalPrice = handlingRate.RateIdr * qtyHandling
		// 	}

		// 	// Insert ke Outbound Detail Handlings
		// 	outboundDetailHandling := models.OutboundDetailHandling{
		// 		OutboundID:       outboundDetail.OutboundID,
		// 		OutboundNo:       outboundDetail.OutboundNo,
		// 		OutboundDetailId: int(outboundDetail.ID),
		// 		ItemCode:         outboundDetail.ItemCode,
		// 		HandlingUsed:     handlingRate.Name,
		// 		HandlingId:       handlingRate.HandlingId,
		// 		RateIdr:          handlingRate.RateIdr,
		// 		IsKoli:           handlingSelected.IsKoli,
		// 		QtyHandling:      qtyHandling,
		// 		TotalPrice:       totalPrice,
		// 		CreatedBy:        int(ctx.Locals("userID").(float64)),
		// 	}
		// 	if err := tx.Create(&outboundDetailHandling).Error; err != nil {
		// 		tx.Rollback()
		// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		// 	}

		// }

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
			newInventory.ExpDate = inventory.ExpDate
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

	// if err := tx.Model(&models.OutboundHeader{}).Where("id = ?", outboundHeader.ID).
	// 	Update("status", payload.Status).Error; err != nil {
	// 	tx.Rollback()
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Change to " + payload.Status + " successfully"})
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
	packing.CreatedBy = 1
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
		"message": "Packcing created successfully",
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
	items, err := outboundRepo.GetPackingItems(outboundID, packingNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Packing Items Found",
		"data":    items,
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

		detail.Location = strings.TrimSpace(getCell(row, 25))
		detail.LotNumber = strings.TrimSpace(getCell(row, 26))
		detail.ExpDate = strings.TrimSpace(getCell(row, 27))
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
