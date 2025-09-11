package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/types"
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
	Quantity   int               `json:"quantity"`
	UOM        string            `json:"uom"`
	SN         string            `json:"sn"`
	Location   string            `json:"location"`
	Remarks    string            `json:"remarks"`
	Mode       string            `json:"mode"`
	VasID      int               `json:"vas_id"`
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

	var outboundID int
	if res.RowsAffected == 1 {
		outboundID = int(OutboundHeader.ID)
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

		if err := tx.Debug().First(&vas, "id = ?", item.VasID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Vas not found"})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var OutboundDetail models.OutboundDetail
		OutboundDetail.OutboundNo = payload.OutboundNo
		OutboundDetail.OutboundID = types.SnowflakeID(outboundID)
		OutboundDetail.ItemCode = item.ItemCode
		OutboundDetail.ItemID = int(product.ID)
		OutboundDetail.Barcode = product.Barcode
		OutboundDetail.CustomerCode = OutboundHeader.CustomerCode
		OutboundDetail.Uom = item.UOM
		OutboundDetail.Quantity = item.Quantity
		OutboundDetail.WhsCode = OutboundHeader.WhsCode
		OutboundDetail.DivisionCode = "REGULAR"
		OutboundDetail.Location = item.Location
		OutboundDetail.QaStatus = "A"
		OutboundDetail.SN = item.SN
		OutboundDetail.SNCheck = "N"
		OutboundDetail.OwnerCode = OutboundHeader.OwnerCode
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
		// Preload("OutboundDetails").
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

	fmt.Println("payload: ", payload)
	// return nil

	// Mulai transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	userID := int(ctx.Locals("userID").(float64))
	var OutboundHeader models.OutboundHeader
	if err := tx.Debug().First(&OutboundHeader, "outbound_no = ?", outbound_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Outbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var customer models.Customer
	if err := tx.Debug().First(&customer, "customer_code = ?", payload.CustomerCode).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
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
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var vas models.Vas

		if err := tx.Debug().First(&vas, "id = ?", item.VasID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Vas not found"})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Coba cari berdasarkan ID
		err := tx.Debug().First(&outboundDetail, "id = ?", item.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// ❌ Tidak ditemukan → insert baru
			newDetail := models.OutboundDetail{
				OutboundID:   OutboundHeader.ID,
				OutboundNo:   OutboundHeader.OutboundNo,
				ItemID:       int(product.ID),
				ItemCode:     item.ItemCode,
				Barcode:      product.Barcode,
				Quantity:     item.Quantity,
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
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		} else if err == nil {
			// ✅ Ditemukan → update
			outboundDetail.OutboundID = OutboundHeader.ID
			outboundDetail.ItemID = int(product.ID)
			outboundDetail.ItemCode = item.ItemCode
			outboundDetail.Barcode = product.Barcode
			outboundDetail.Uom = item.UOM
			outboundDetail.WhsCode = OutboundHeader.WhsCode
			outboundDetail.OwnerCode = OutboundHeader.OwnerCode
			outboundDetail.DivisionCode = "REGULAR"
			outboundDetail.CustomerCode = customer.CustomerCode
			outboundDetail.QaStatus = "A"
			outboundDetail.Quantity = item.Quantity
			outboundDetail.Location = item.Location
			outboundDetail.Remarks = item.Remarks
			outboundDetail.SN = item.SN
			outboundDetail.SNCheck = "N"
			outboundDetail.VasID = item.VasID
			outboundDetail.VasName = vas.Name
			outboundDetail.UpdatedBy = int(ctx.Locals("userID").(float64))
			outboundDetail.UpdatedAt = time.Now()

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
			pickingSheet := models.OutboundPicking{
				InventoryID:      int(inventory.ID),
				OutboundId:       outboundDetail.OutboundID,
				OutboundNo:       outboundDetail.OutboundNo,
				OutboundDetailId: int(outboundDetail.ID),
				OwnerCode:        outboundDetail.OwnerCode,
				ItemID:           outboundDetail.ItemID,
				Barcode:          product.Barcode,
				ItemCode:         product.ItemCode,
				Pallet:           inventory.Pallet,
				Location:         inventory.Location,
				Quantity:         qtyPick,
				WhsCode:          inventory.WhsCode,
				QaStatus:         inventory.QaStatus,
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

	repo := repositories.NewOutboundRepository(tx)

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
		if outboundItem.QtyReq != outboundItem.QtyPack {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Packing not complete"})
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
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
	}

	// UPDATE OUTBOUND STATUS
	if err := tx.Debug().
		Model(&models.OutboundHeader{}).
		Where("id = ?", inputBody.OutboundID).
		Updates(map[string]interface{}{
			"status":     "complete",
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

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Pick and Pack Outbound Success"})
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

	if OutboundHeader.Status != "picking" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Outbound " + payload.OutboundNo + " not in picking status", "message": "Outbound " + payload.OutboundNo + " not in picking status"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Outbound is in picking status", "data": OutboundHeader})
}

func (r *OutboundController) ProccesHandleOpen(ctx *fiber.Ctx) error {
	var payload struct {
		Action           string `json:"action"`
		OutboundNo       string `json:"outbound_no"`
		TempLocationName string `json:"temp_location_name"`
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

			// Buat inventory baru (jumlah = picking.Quantity)
			newInventory := inventory
			newInventory.ID = 0
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
			newInventory.QaStatus = inventory.QaStatus
			newInventory.Uom = inventory.Uom
			newInventory.QtyOnhand = picking.Quantity
			newInventory.QtyAvailable = picking.Quantity
			newInventory.QtyAllocated = 0
			newInventory.QtySuspend = 0
			newInventory.QtyShipped = 0
			newInventory.Trans = "unpost " + payload.OutboundNo
			newInventory.CreatedBy = int(userID)
			newInventory.CreatedAt = time.Now()

			if err := tx.Create(&newInventory).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			// Kurangi inventory lama
			if err := tx.Model(&models.Inventory{}).Where("id = ?", picking.InventoryID).
				Updates(map[string]interface{}{
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
		// Kalau action selain return to origin location
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

	// Delete scan detail
	// if err := tx.Unscoped().Where("outbound_id = ?", outboundHeader.ID).Delete(&models.OutboundScanDetail{}).Error; err != nil {
	// 	tx.Rollback()
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

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

	// Update status
	if err := tx.Model(&models.OutboundHeader{}).Where("id = ?", outboundHeader.ID).
		Update("status", "open").Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Changed to Open successfully"})
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
