package controllers

import (
	"fiber-app/models"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ShippingController struct {
	DB *gorm.DB
}

type ListDNOpen struct {
	OutboundID     int     `json:"outbound_id"`
	DeliveryNumber string  `json:"delivery_number"`
	CustomerName   string  `json:"customer_name"`
	TotalItem      int     `json:"total_item"`
	TotalQty       int     `json:"total_qty"`
	Kubikasi       float64 `json:"kubikasi"`
	Volume         float64 `json:"volume"`
}

func NewShippingController(DB *gorm.DB) *ShippingController {
	return &ShippingController{DB: DB}
}

func (c *ShippingController) GetListOrderPart(ctx *fiber.Ctx) error {

	var listOrderParts []models.ListOrderPart
	if err := c.DB.Where("status = ?", "open").Find(&listOrderParts).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listOrderParts})
}

func (c *ShippingController) GetListDNOpen(ctx *fiber.Ctx) error {

	var listDNOpen []ListDNOpen
	sql := `with dn as 
	(select outbound_id,
	delivery_number, customer_name,
	count(a.item_id) as total_item,
	SUM(qty) as total_qty,
	SUM(qty) * b.kubikasi as volume
	from list_order_parts a
	inner join products b on a.item_id = b.id
	where a.status = 'open'
	GROUP BY outbound_id, delivery_number, customer_name, b.kubikasi)
	select outbound_id, delivery_number, customer_name,
	SUM(total_item) as total_item,
	SUM(total_qty) as total_qty,
	SUM(volume) as volume
	from dn
	group by outbound_id, delivery_number, customer_name`

	if err := c.DB.Raw(sql).Scan(&listDNOpen).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listDNOpen})
}

func (c *ShippingController) CreateOrder(ctx *fiber.Ctx) error {

	var request []ListDNOpen
	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Generate Order No
	orderNo, err := GenerateOrderNo(c.DB)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// start DB transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	orderHeader := models.OrderHeader{
		OrderNo:   orderNo,
		Status:    "open",
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := tx.Create(&orderHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Create Order Details
	for _, item := range request {

		var listOrderParts []models.ListOrderPart

		if err := tx.Where("outbound_id = ?", item.OutboundID).Find(&listOrderParts).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if len(listOrderParts) == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ListOrderPart not found"})
		}

		for _, do := range listOrderParts {
			orderDetail := models.OrderDetail{
				OrderID:         orderHeader.ID,
				OrderNo:         orderNo,
				DeliveryNumber:  do.DeliveryNumber,
				ListOrderPartID: do.ID,
				ShipTo:          do.ShipTo,
				Volume:          do.Volume,
				Qty:             do.Qty,
				UnitPrice:       0,
				DestinationCity: do.CustomerName,
				CreatedBy:       int(ctx.Locals("userID").(float64)),
			}

			if err := tx.Create(&orderDetail).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if err := tx.Model(&models.ListOrderPart{}).
				Where("id = ?", do.ID).
				Updates(map[string]interface{}{
					"order_id":   orderHeader.ID,
					"order_no":   orderNo,
					"status":     "order",
					"updated_by": int(ctx.Locals("userID").(float64)),
					"updated_at": time.Now(),
				}).
				Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order created successfully"})
}

func GenerateOrderNo(db *gorm.DB) (string, error) {
	prefix := "SPK"
	companyCode := "YM"
	now := time.Now()

	year := now.Format("06")  // 2 digit tahun
	month := now.Format("01") // 2 digit bulan

	// Hitung range awal dan akhir bulan ini
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	firstOfNextMonth := firstOfMonth.AddDate(0, 1, 0)

	// Cari order terakhir yang dibuat di bulan ini
	var lastOrder models.OrderHeader
	err := db.
		Where("created_at >= ? AND created_at < ?", firstOfMonth, firstOfNextMonth).
		Order("created_at DESC").
		First(&lastOrder).Error

	var sequence int
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			sequence = 1
		} else {
			return "", err
		}
	} else {
		lastOrderNo := lastOrder.OrderNo
		if len(lastOrderNo) >= 4 {
			var lastSequence int
			fmt.Sscanf(lastOrderNo[len(lastOrderNo)-4:], "%d", &lastSequence)
			sequence = lastSequence + 1
		} else {
			sequence = 1
		}
	}

	orderNo := fmt.Sprintf("%s%s%s%s%04d", prefix, companyCode, year, month, sequence)
	return orderNo, nil
}

func (c *ShippingController) GetListOrder(ctx *fiber.Ctx) error {

	var orderHeaders []models.OrderHeader
	if err := c.DB.
		Order("created_at desc"). // Example: Order by the created_at field in descending order
		Find(&orderHeaders).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order found", "data": orderHeaders})
}

func (c *ShippingController) GetOrderByID(ctx *fiber.Ctx) error {

	var orderHeader models.OrderHeader
	if err := c.DB.Where("order_no = ?", ctx.Params("order_no")).First(&orderHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var orderDetails []models.OrderDetail
	if err := c.DB.Where("order_id = ?", orderHeader.ID).Find(&orderDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order found", "data": orderDetails})
}

func (c *ShippingController) UnGroupOrder(ctx *fiber.Ctx) error {
	var ReqOrderDetails []models.OrderDetail
	if err := ctx.BodyParser(&ReqOrderDetails); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// start DB transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// update ListOrderPart
	for _, item := range ReqOrderDetails {
		if err := tx.Model(&models.ListOrderPart{}).
			Where("id = ?", item.ListOrderPartID).
			Updates(map[string]interface{}{
				"order_id":   0,
				"order_no":   "",
				"status":     "open",
				"updated_by": int(ctx.Locals("userID").(float64)),
				"updated_at": time.Now(),
			}).
			Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		sqlDelete := "DELETE FROM order_details WHERE list_order_part_id = ?"

		// Delete corresponding record from order_details
		if err := tx.Exec(sqlDelete, item.ListOrderPartID).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Ungroup Order successfully"})
}
