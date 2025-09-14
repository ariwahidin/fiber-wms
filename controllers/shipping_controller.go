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

type OrderDetail struct {
	OrderID         int    `json:"order_id"`
	DeliveryNumber  string `json:"delivery_number"`
	DestinationCity string `json:"destination_city"`
	TotalQty        int    `json:"total_qty"`
	TotalItem       int    `json:"total_item"`
}

func NewShippingController(DB *gorm.DB) *ShippingController {
	return &ShippingController{DB: DB}
}

// func (c *ShippingController) GetListOrderPart(ctx *fiber.Ctx) error {

// 	var listOrderParts []models.ListOrderPart
// 	if err := c.DB.Where("status = ?", "open").Find(&listOrderParts).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listOrderParts})
// }

// func (c *ShippingController) GetListDNOpen(ctx *fiber.Ctx) error {

// 	var listDNOpen []ListDNOpen
// 	sql := `with dn as
// 	(select outbound_id,
// 	delivery_number, customer_name,
// 	count(a.item_id) as total_item,
// 	SUM(qty) as total_qty,
// 	SUM(qty) * b.kubikasi as volume
// 	from list_order_parts a
// 	inner join products b on a.item_id = b.id
// 	where a.status = 'open'
// 	GROUP BY outbound_id, delivery_number, customer_name, b.kubikasi)
// 	select outbound_id, delivery_number, customer_name,
// 	SUM(total_item) as total_item,
// 	SUM(total_qty) as total_qty,
// 	SUM(volume) as volume
// 	from dn
// 	group by outbound_id, delivery_number, customer_name`

// 	if err := c.DB.Raw(sql).Scan(&listDNOpen).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(listDNOpen) == 0 {
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": []ListDNOpen{}})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listDNOpen})
// }

// func (c *ShippingController) CreateOrder(ctx *fiber.Ctx) error {

// 	var request []ListDNOpen
// 	if err := ctx.BodyParser(&request); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(request) == 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
// 	}

// 	listDeliveryNumber := make([]string, len(request))
// 	for i, item := range request {
// 		listDeliveryNumber[i] = item.DeliveryNumber
// 	}

// 	// Generate Order No
// 	orderNo, err := GenerateOrderNo(c.DB)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// start DB transaction
// 	tx := c.DB.Begin()
// 	if tx.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
// 	}

// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	orderHeader := models.OrderHeader{
// 		OrderNo:   orderNo,
// 		Status:    "open",
// 		CreatedBy: int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := tx.Create(&orderHeader).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	type orderOpenSelected struct {
// 		DeliveryNumber string
// 		Status         string
// 		CustomerCode   string
// 		ShipTo         string
// 	}

// 	var orderOpenSelecteds []orderOpenSelected

// 	sql := `SELECT delivery_number, status, customer_code, ship_to
// 	FROM
// 	list_order_parts
// 	WHERE status = 'open'
// 	AND delivery_number IN (?)
// 	GROUP BY delivery_number, status, customer_code, ship_to`

// 	if err := tx.Debug().Raw(sql, listDeliveryNumber).Scan(&orderOpenSelecteds).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(orderOpenSelecteds) == 0 {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ListOrderPart not found"})
// 	}

// 	// Create Order Details
// 	for _, item := range orderOpenSelecteds {
// 		orderDetail := models.OrderDetail{
// 			OrderID:        orderHeader.ID,
// 			OrderNo:        orderNo,
// 			DeliveryNumber: item.DeliveryNumber,
// 			Customer:       item.CustomerCode,
// 			ShipTo:         item.ShipTo,
// 			CreatedBy:      int(ctx.Locals("userID").(float64)),
// 		}

// 		if err := tx.Create(&orderDetail).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	// Update ListOrderPart
// 	for _, item := range orderOpenSelecteds {
// 		if err := tx.Model(&models.ListOrderPart{}).
// 			Where("status = 'open' AND delivery_number = ?", item.DeliveryNumber).
// 			Updates(map[string]interface{}{
// 				"order_id":   orderHeader.ID,
// 				"order_no":   orderNo,
// 				"status":     "order",
// 				"updated_by": int(ctx.Locals("userID").(float64)),
// 				"updated_at": time.Now(),
// 			}).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	// Update Total Order In Order Header
// 	if err := tx.Model(&models.OrderHeader{}).
// 		Where("id = ?", orderHeader.ID).
// 		Updates(map[string]interface{}{
// 			"total_order": len(orderOpenSelecteds),
// 			"updated_by":  int(ctx.Locals("userID").(float64)),
// 			"updated_at":  time.Now(),
// 		}).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order created successfully"})
// }

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
	err := db.Debug().
		Where("created_at >= ? AND created_at < ?", firstOfMonth, firstOfNextMonth).
		Order("order_no DESC").
		First(&lastOrder).Error

	fmt.Println("lastOrder:", lastOrder)

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

type OrderItem struct {
	ID           int               `json:"ID"`
	OutboundID   types.SnowflakeID `json:"outbound_id"`
	OutboundNo   string            `json:"outbound_no"`
	ShipmentID   string            `json:"shipment_id"`
	DelivTo      string            `json:"deliv_to"`
	DelivToName  string            `json:"deliv_to_name"`
	DelivAddress string            `json:"deliv_address"`
	DelivCity    string            `json:"deliv_city"`
	QtyKoli      int               `json:"qty_koli"`
	TotalItem    int               `json:"total_item"`
	TotalQty     int               `json:"total_qty"`
	TotalCBM     float64           `json:"total_cbm"`
}

type Order struct {
	ID              types.SnowflakeID `json:"ID"`
	Driver          string            `json:"driver"`
	OrderDate       string            `json:"order_date"`
	OrderNo         string            `json:"order_no"`
	TransporterCode string            `json:"transporter_code"`
	TransporterName string            `json:"transporter_name"`
	TruckType       string            `json:"truck_type"`
	TruckSize       string            `json:"truck_size"`
	TruckNo         string            `json:"truck_no"`
	LoadDate        string            `json:"load_date"`
	LoadStartTime   string            `json:"load_start_time"`
	LoadEndTime     string            `json:"load_end_time"`
	OrderType       string            `json:"order_type"`
	Remarks         string            `json:"remarks"`
	Items           []OrderItem       `json:"items"`
}

func (c *ShippingController) GetOutboundList(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewShippingRepository(c.DB)
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

func (c *ShippingController) CreateOrder(ctx *fiber.Ctx) error {
	var payload Order

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

	// Generate Order No
	orderNo, err := GenerateOrderNo(tx)
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate order no",
			"error":   err.Error(),
		})
	}

	// Insert To Order Header

	var orderHeader models.OrderHeader
	orderHeader.OrderNo = orderNo
	orderHeader.Driver = payload.Driver
	orderHeader.OrderDate = payload.OrderDate
	orderHeader.TransporterCode = payload.TransporterCode
	orderHeader.TransporterName = payload.TransporterName
	orderHeader.TruckNo = payload.TruckNo
	orderHeader.TruckSize = payload.TruckSize
	orderHeader.TruckType = payload.TruckType
	orderHeader.LoadDate = payload.LoadDate
	orderHeader.LoadStartTime = payload.LoadStartTime
	orderHeader.LoadEndTime = payload.LoadEndTime
	orderHeader.OrderType = payload.OrderType
	orderHeader.Remarks = payload.Remarks
	orderHeader.CreatedBy = int(ctx.Locals("userID").(float64))
	orderHeader.CreatedAt = time.Now()
	orderHeader.UpdatedAt = time.Now()
	if err := tx.Create(&orderHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to insert order header",
			"error":   err.Error(),
		})
	}

	// Insert To Order Items
	for _, item := range payload.Items {
		var orderItem models.OrderDetail
		orderItem.OrderID = orderHeader.ID
		orderItem.OrderNo = orderHeader.OrderNo
		orderItem.OutboundID = item.OutboundID
		orderItem.OutboundNo = item.OutboundNo
		orderItem.ShipmentID = item.ShipmentID
		orderItem.DelivTo = item.DelivTo
		orderItem.DelivToName = item.DelivToName
		orderItem.DelivAddress = item.DelivAddress
		orderItem.DelivCity = item.DelivCity
		orderItem.QtyKoli = item.QtyKoli
		orderItem.TotalItem = item.TotalItem
		orderItem.TotalQty = item.TotalQty
		orderItem.TotalCBM = item.TotalCBM
		orderItem.CreatedBy = int(ctx.Locals("userID").(float64))
		orderItem.CreatedAt = time.Now()
		orderItem.UpdatedAt = time.Now()
		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert order item",
				"error":   err.Error(),
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
			"order_no": orderNo,
		},
	})
}

func (c *ShippingController) GetListOrder(ctx *fiber.Ctx) error {
	orderRepo := repositories.NewShippingRepository(c.DB)
	orderList, err := orderRepo.GetOrderSummaryList()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order found", "data": orderList})
}

func (c *ShippingController) GetOrderByNo(ctx *fiber.Ctx) error {
	order_no := ctx.Params("order_no")
	var OrderHeader models.OrderHeader
	if err := c.DB.Debug().
		Preload("Items").
		First(&OrderHeader, "order_no = ?", order_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": OrderHeader, "message": "Order found"})
}

func (c *ShippingController) GetOrderAndDetailByNo(ctx *fiber.Ctx) error {
	order_no := ctx.Params("order_no")
	var OrderHeader models.OrderHeader
	if err := c.DB.Debug().
		Preload("Items").
		First(&OrderHeader, "order_no = ?", order_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	shippingRepo := repositories.NewShippingRepository(c.DB)

	orderDetailItems, err := shippingRepo.GetOrderDetailItem(int(OrderHeader.ID))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"order": OrderHeader, "detail_items": orderDetailItems}, "message": "Order found"})
}

func (c *ShippingController) UpdateOrderByID(ctx *fiber.Ctx) error {
	order_no := ctx.Params("order_no")

	var payload Order

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Mulai transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	userID := int(ctx.Locals("userID").(float64))
	var orderHeader models.OrderHeader
	if err := tx.Debug().First(&orderHeader, "order_no = ?", order_no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	orderHeader.Driver = payload.Driver
	orderHeader.OrderDate = payload.OrderDate
	orderHeader.TransporterCode = payload.TransporterCode
	orderHeader.TransporterName = payload.TransporterName
	orderHeader.TruckSize = payload.TruckSize
	orderHeader.TruckNo = payload.TruckNo
	orderHeader.TruckType = payload.TruckType
	orderHeader.LoadDate = payload.LoadDate
	orderHeader.LoadStartTime = payload.LoadStartTime
	orderHeader.LoadEndTime = payload.LoadEndTime
	orderHeader.OrderType = payload.OrderType
	orderHeader.Remarks = payload.Remarks
	orderHeader.CreatedBy = userID
	orderHeader.CreatedAt = time.Now()
	orderHeader.UpdatedAt = time.Now()

	if err := tx.Model(&models.OrderHeader{}).Where("id = ?", orderHeader.ID).Updates(orderHeader).Error; err != nil {
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

		var orderItem models.OrderDetail

		// Coba cari berdasarkan ID
		err := tx.Debug().First(&orderItem, "id = ?", item.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {

			newItem := models.OrderDetail{
				OrderID:      orderHeader.ID,
				OrderNo:      orderHeader.OrderNo,
				OutboundID:   item.OutboundID,
				OutboundNo:   item.OutboundNo,
				ShipmentID:   item.ShipmentID,
				DelivTo:      item.DelivTo,
				DelivToName:  item.DelivToName,
				DelivAddress: item.DelivAddress,
				DelivCity:    item.DelivCity,
				QtyKoli:      item.QtyKoli,
				TotalItem:    item.TotalItem,
				TotalQty:     item.TotalQty,
				TotalCBM:     item.TotalCBM,
				CreatedBy:    userID,
			}
			if err := tx.Create(&newItem).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

		} else if err == nil {
			// ✅ Ditemukan → update

			orderItem.OrderID = orderHeader.ID
			orderItem.OrderNo = orderHeader.OrderNo
			orderItem.OutboundID = item.OutboundID
			orderItem.OutboundNo = item.OutboundNo
			orderItem.ShipmentID = item.ShipmentID
			orderItem.DelivTo = item.DelivTo
			orderItem.DelivToName = item.DelivToName
			orderItem.DelivAddress = item.DelivAddress
			orderItem.DelivCity = item.DelivCity
			orderItem.QtyKoli = item.QtyKoli
			orderItem.TotalItem = item.TotalItem
			orderItem.TotalQty = item.TotalQty
			orderItem.TotalCBM = item.TotalCBM
			orderItem.CreatedBy = int(ctx.Locals("userID").(float64))
			orderItem.CreatedAt = time.Now()
			orderItem.UpdatedAt = time.Now()

			if err := tx.Save(&orderItem).Error; err != nil {
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Order successfully", "data": orderHeader})
}

func (c *ShippingController) DeleteItemOrderByID(ctx *fiber.Ctx) error {

	id := ctx.Params("id")

	// if err := c.DB.Where("id = ?", id).Delete(&models.OrderDetail{}).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	// Hard Delete Order Header
	if err := c.DB.Where("id = ?", id).Unscoped().Delete(&models.OrderDetail{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Delete Order successfully"})
}

// func (c *ShippingController) GetOrderByID(ctx *fiber.Ctx) error {

// 	var orderHeader models.OrderHeader
// 	if err := c.DB.Where("order_no = ?", ctx.Params("order_no")).First(&orderHeader).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	sql := `WITH lop AS (
// 				SELECT
// 					order_id,
// 					delivery_number,
// 					COUNT(item_id) AS total_item,
// 					SUM(qty) AS total_qty,
// 					customer_code,
// 					customer_name
// 				FROM list_order_parts
// 				WHERE order_id = ?
// 				GROUP BY order_id, customer_code, customer_name, delivery_number
// 			)
// 			SELECT
// 				a.id,
// 				a.order_id,
// 				a.delivery_number,
// 				a.customer,
// 				a.ship_to,
// 				b.total_qty,
// 				b.total_item
// 			FROM order_details a
// 			INNER JOIN lop b ON a.order_id = b.order_id AND a.delivery_number = b.delivery_number
// 			WHERE a.order_id = ?`

// 	type OrderDetail struct {
// 		ID             int    `json:"id"`
// 		OrderID        int    `json:"order_id"`
// 		DeliveryNumber string `json:"delivery_number"`
// 		TotalItem      int    `json:"total_item"`
// 		TotalQty       int    `json:"total_qty"`
// 		Customer       string `json:"customer"`
// 		ShipTo         string `json:"ship_to"`
// 	}

// 	var orderDetails []OrderDetail
// 	if err := c.DB.Raw(sql, orderHeader.ID, orderHeader.ID).Scan(&orderDetails).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(orderDetails) == 0 {
// 		orderDetails = []OrderDetail{}
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Order found", "data": fiber.Map{"order_header": orderHeader, "order_details": orderDetails}})
// }

// func (c *ShippingController) UnGroupOrder(ctx *fiber.Ctx) error {

// 	fmt.Println(ctx.Body())

// 	// return nil

// 	var ReqOrderDetails []OrderDetail
// 	if err := ctx.BodyParser(&ReqOrderDetails); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// start DB transaction
// 	tx := c.DB.Begin()
// 	if tx.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
// 	}

// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	// update ListOrderPart
// 	for _, item := range ReqOrderDetails {
// 		if err := tx.Model(&models.ListOrderPart{}).
// 			Where("order_id = ?", item.OrderID).
// 			Updates(map[string]interface{}{
// 				"order_id":   0,
// 				"order_no":   "",
// 				"status":     "open",
// 				"updated_by": int(ctx.Locals("userID").(float64)),
// 				"updated_at": time.Now(),
// 			}).
// 			Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}

// 		sqlDelete := "DELETE FROM order_details WHERE order_id = ?"

// 		// Delete corresponding record from order_details
// 		if err := tx.Exec(sqlDelete, item.OrderID).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Ungroup Order successfully"})
// }

// func (c *ShippingController) UpdateOrderDetailByID(ctx *fiber.Ctx) error {
// 	type UpdateOrderRequest struct {
// 		ID         int    `json:"id"`
// 		DeliveryNo string `json:"delivery_number"`
// 		Customer   string `json:"customer"`
// 		ShipTo     string `json:"ship_to"`
// 		TotalItem  int    `json:"total_item"`
// 		TotalQty   int    `json:"total_qty"`
// 	}

// 	id := ctx.Params("id") // misal URL: /shipping/order/:id

// 	var req UpdateOrderRequest
// 	if err := ctx.BodyParser(&req); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Invalid request payload",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Update the order in the database
// 	result := c.DB.Model(&models.OrderDetail{}).Where("id = ?", id).
// 		Updates(map[string]interface{}{
// 			"delivery_number": req.DeliveryNo,
// 			"customer":        req.Customer,
// 			"ship_to":         req.ShipTo,
// 			"updated_by":      int(ctx.Locals("userID").(float64)),
// 			"updated_at":      time.Now(),
// 		})

// 	if result.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to update order",
// 			"error":   result.Error.Error(),
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Order updated successfully",
// 	})
// }

// func (c *ShippingController) UpdateOrderHeaderByID(ctx *fiber.Ctx) error {
// 	id := ctx.Params("id") // misal URL: /shipping/order/:id

// 	var req models.OrderHeader
// 	if err := ctx.BodyParser(&req); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Invalid request payload",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Update the order in the database
// 	result := c.DB.Model(&models.OrderHeader{}).Where("id = ?", id).
// 		Updates(map[string]interface{}{
// 			"driver":        req.Driver,
// 			"truck_no":      req.TruckNo,
// 			"transporter":   req.Transporter,
// 			"delivery_date": req.DeliveryDate,
// 			"updated_by":    int(ctx.Locals("userID").(float64)),
// 			"updated_at":    time.Now(),
// 		})

// 	if result.Error != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to update order",
// 			"error":   result.Error.Error(),
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Order updated successfully",
// 	})
// }

// func (c *ShippingController) GetOrderDetailItemsByOrderNo(ctx *fiber.Ctx) error {
// 	order_no := ctx.Params("order_no") // misal URL: /shipping/order/:id

// 	orderHeader := models.OrderHeader{}
// 	if err := c.DB.Where("order_no = ?", order_no).First(&orderHeader).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if orderHeader.ID == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
// 	}

// 	var orderDetails []models.OrderDetail
// 	// pake preload
// 	if err := c.DB.Where("order_id = ?", orderHeader.ID).Find(&orderDetails).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var orderDetailItems []models.ListOrderPart
// 	if err := c.DB.Where("order_id = ?", orderHeader.ID).Find(&orderDetailItems).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var orderConsoles []models.OrderConsole
// 	if err := c.DB.Where("order_id = ?", orderHeader.ID).Find(&orderConsoles).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"order_header": orderHeader, "order_details": orderDetails, "order_detail_items": orderDetailItems, "order_consoles": orderConsoles}})
// }
