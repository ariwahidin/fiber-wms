package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	integration_service "fiber-app/services/integration_service"
	"fiber-app/types"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ShippingController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
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

func NewShippingController(DB *gorm.DB, queryDB *gorm.DB) *ShippingController {
	return &ShippingController{DB: DB, QueryDB: queryDB}
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
	VasKoli      int               `json:"vas_koli"`
	TotalItem    int               `json:"total_item"`
	TotalQty     int               `json:"total_qty"`
	TotalCBM     float64           `json:"total_cbm"`
	Remarks      string            `json:"remarks"`
	OrderType    string            `json:"order_type"`
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

	shippingRepo := repositories.NewShippingRepository(tx)

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
		orderItem.VasKoli = item.VasKoli
		orderItem.TotalItem = item.TotalItem
		orderItem.TotalQty = item.TotalQty
		orderItem.TotalCBM = item.TotalCBM
		orderItem.Remarks = item.Remarks
		orderItem.OrderType = item.OrderType
		orderItem.CreatedBy = int(ctx.Locals("userID").(float64))
		orderItem.CreatedAt = time.Now()
		orderItem.UpdatedAt = time.Now()
		if err := tx.Debug().Create(&orderItem).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert order item",
				"error":   err.Error(),
			})
		}

		vasCalculated, err := shippingRepo.CalculatVasOutbound(int(item.OutboundID))
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if len(vasCalculated) > 0 {
			for _, vas_item := range vasCalculated {
				newOutboundVas := models.OutboundVas{
					OutboundID:   vas_item.OutboundID,
					OutboundNo:   vas_item.OutboundNo,
					OutboundDate: vas_item.OutboundDate,
					QtyItem:      vas_item.QtyItem,
					QtyKoli:      vas_item.VasKoli,
					MainVasName:  vas_item.MainVasName,
					DefaultPrice: vas_item.DefaultPrice,
					IsKoli:       vas_item.IsKoli,
					TotalPrice:   vas_item.TotalPrice,
					CreatedBy:    int(ctx.Locals("userID").(float64)),
					UpdatedBy:    int(ctx.Locals("userID").(float64)),
				}

				if err := tx.Debug().Create(&newOutboundVas).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
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
				VasKoli:      item.VasKoli,
				TotalItem:    item.TotalItem,
				TotalQty:     item.TotalQty,
				TotalCBM:     item.TotalCBM,
				OrderType:    item.OrderType,
				CreatedBy:    userID,
			}
			if err := tx.Create(&newItem).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

		} else if err == nil {
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
			orderItem.VasKoli = item.VasKoli
			orderItem.TotalItem = item.TotalItem
			orderItem.TotalQty = item.TotalQty
			orderItem.TotalCBM = item.TotalCBM
			orderItem.Remarks = item.Remarks
			orderItem.OrderType = item.OrderType
			orderItem.CreatedBy = int(ctx.Locals("userID").(float64))
			orderItem.CreatedAt = time.Now()
			orderItem.UpdatedAt = time.Now()

			if err := tx.Save(&orderItem).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		} else {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	orderDetailNewest := []models.OrderDetail{}
	if err := tx.Debug().Where("order_no = ?", order_no).Order("created_at desc").Find(&orderDetailNewest).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	shippingRepo := repositories.NewShippingRepository(tx)

	if len(orderDetailNewest) > 0 {
		for _, item := range orderDetailNewest {

			// Delete Outbound Vas existing
			if err := tx.Where("outbound_id = ?", item.OutboundID).Unscoped().Delete(&models.OutboundVas{}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			vasCalculated, err := shippingRepo.CalculatVasOutbound(int(item.OutboundID))
			if err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			if len(vasCalculated) > 0 {
				for _, vas_item := range vasCalculated {
					newOutboundVas := models.OutboundVas{
						OutboundID:   vas_item.OutboundID,
						OutboundNo:   vas_item.OutboundNo,
						OutboundDate: vas_item.OutboundDate,
						QtyItem:      vas_item.QtyItem,
						QtyKoli:      vas_item.VasKoli,
						MainVasName:  vas_item.MainVasName,
						DefaultPrice: vas_item.DefaultPrice,
						IsKoli:       vas_item.IsKoli,
						TotalPrice:   vas_item.TotalPrice,
						CreatedBy:    int(ctx.Locals("userID").(float64)),
						UpdatedBy:    int(ctx.Locals("userID").(float64)),
					}

					if err := tx.Debug().Create(&newOutboundVas).Error; err != nil {
						tx.Rollback()
						return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
					}
				}
			}
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

	// Hard Delete Order Header
	if err := c.DB.Where("id = ?", id).Unscoped().Delete(&models.OrderDetail{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Delete Order successfully"})
}

// UpdateOrderStatus - Update status satu atau banyak order sekaligus
func (c *ShippingController) UpdateOrderStatus(ctx *fiber.Ctx) error {
	type StatusPayload struct {
		OrderNos []string `json:"order_nos"` // bisa satu atau banyak
		Status   string   `json:"status"`    // "loaded", "open", dst
	}

	var payload StatusPayload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	if len(payload.OrderNos) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No order numbers provided",
		})
	}

	allowedStatuses := map[string]bool{
		"open":   true,
		"loaded": true,
	}
	if !allowedStatuses[payload.Status] {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid status. Allowed: open, loaded",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	result := c.DB.Model(&models.OrderHeader{}).
		Where("order_no IN ?", payload.OrderNos).
		Updates(map[string]interface{}{
			"status":     payload.Status,
			"updated_by": userID,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update status",
			"error":   result.Error.Error(),
		})
	}

	if payload.Status == "loaded" {
		go func() {
			for _, orderNo := range payload.OrderNos {
				integration_service.Dispatch(c.DB, c.QueryDB, "order.loaded", map[string]interface{}{
					"order_no":   orderNo,
					"status":     payload.Status,
					"updated_by": userID,
				})
			}
		}()
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"message":       fmt.Sprintf("%d order(s) updated to '%s'", result.RowsAffected, payload.Status),
		"rows_affected": result.RowsAffected,
	})
}

// OrderSummaryFilterRow — struct hasil query ringkasan order untuk list page.
// Sesuaikan field dengan kolom aktual di tabel order_headers dan order_details.
type OrderSummaryFilterRow struct {
	ID              interface{} `json:"ID"`
	OrderNo         string      `json:"order_no"`
	OrderDate       string      `json:"order_date"`
	Status          string      `json:"status"`
	Driver          string      `json:"driver"`
	TransporterCode string      `json:"transporter_code"`
	TransporterName string      `json:"transporter_name"`
	TruckType       string      `json:"truck_type"`
	TruckSize       string      `json:"truck_size"`
	TruckNo         string      `json:"truck_no"`
	LoadDate        string      `json:"load_date"`
	OrderType       string      `json:"order_type"`
	Remarks         string      `json:"remarks"`
	TotalDO         int         `json:"total_do"`   // COUNT(DISTINCT order_details.id)
	TotalDrop       int         `json:"total_drop"` // COUNT(DISTINCT deliv_to)
	TotalKoli       int         `json:"total_koli"`
	TotalItem       int         `json:"total_item"`
	TotalQty        int         `json:"total_qty"`
	TotalCBM        float64     `json:"total_cbm"`
}

func (c *ShippingController) GetListOrderFilter(ctx *fiber.Ctx) error {
	// ── 1. Parse query params ─────────────────────────────────────────────────

	const layout = "2006-01-02"
	now := time.Now()

	// Default: 7 hari ke belakang
	startDate := now.AddDate(0, 0, -7).Format(layout)
	endDate := now.Format(layout)

	if v := ctx.Query("start_date"); v != "" {
		startDate = v
	}
	if v := ctx.Query("end_date"); v != "" {
		endDate = v
	}

	search := strings.TrimSpace(ctx.Query("search"))      // order header fields
	searchDO := strings.TrimSpace(ctx.Query("search_do")) // DO No di dalam items

	// statuses: "open,loaded" → ["open","loaded"]
	var statusList []string
	if v := ctx.Query("statuses"); v != "" {
		for _, s := range strings.Split(v, ",") {
			if t := strings.TrimSpace(s); t != "" {
				statusList = append(statusList, t)
			}
		}
	}

	// ── 2. Build query ────────────────────────────────────────────────────────
	//
	// Kita query langsung ke DB pakai raw GORM builder agar fleksibel.
	// order_headers  → oh
	// order_details  → od
	//
	// Jika search_do aktif: filter ke order yang punya setidaknya 1 baris
	// di order_details dengan shipment_id LIKE '%search_do%'.

	db := c.DB

	// SELECT clause — aggregasi dari order_details
	query := db.Table("order_headers oh").
		Select(`
			oh.id                  AS id,
			oh.order_no            AS order_no,
			oh.order_date          AS order_date,
			oh.status              AS status,
			oh.driver              AS driver,
			oh.transporter_code    AS transporter_code,
			oh.transporter_name    AS transporter_name,
			oh.truck_type          AS truck_type,
			oh.truck_size          AS truck_size,
			oh.truck_no            AS truck_no,
			oh.load_date           AS load_date,
			oh.order_type          AS order_type,
			oh.remarks             AS remarks,
			COUNT(DISTINCT od.id)        AS total_do,
			COUNT(DISTINCT od.deliv_to)  AS total_drop,
			COALESCE(SUM(od.qty_koli),  0) AS total_koli,
			COALESCE(SUM(od.total_item),0) AS total_item,
			COALESCE(SUM(od.total_qty), 0) AS total_qty,
			COALESCE(SUM(od.total_cbm), 0) AS total_cbm
		`).
		Joins("LEFT JOIN order_details od ON od.order_id = oh.id AND od.deleted_at IS NULL").
		Where("oh.deleted_at IS NULL").
		// Date range — filter by order_date (cast ke DATE agar jam tidak pengaruh)
		Where("CAST(oh.order_date AS DATE) BETWEEN ? AND ?", startDate, endDate).
		Group("oh.id, oh.order_no, oh.order_date, oh.status, oh.driver, oh.transporter_code, oh.transporter_name, oh.truck_type, oh.truck_size, oh.truck_no, oh.load_date, oh.order_type, oh.remarks").
		Order("oh.order_date DESC, oh.id DESC")

	// Filter status (multi)
	if len(statusList) > 0 {
		query = query.Where("oh.status IN ?", statusList)
	}

	// Filter search header (order_no, transporter, truck_no, driver)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"oh.order_no LIKE ? OR oh.transporter_name LIKE ? OR oh.truck_no LIKE ? OR oh.driver LIKE ?",
			like, like, like, like,
		)
	}

	// ── Filter search_do ──────────────────────────────────────────────────────
	// Gunakan EXISTS subquery agar tidak menggandakan baris & tetap bisa
	// GROUP BY dengan benar.
	// shipment_id di order_details = DO No.
	if searchDO != "" {
		like := "%" + searchDO + "%"
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM order_details od2
				WHERE od2.order_id    = oh.id
				  AND od2.deleted_at  IS NULL
				  AND od2.shipment_id LIKE ?
			)`,
			like,
		)
	}

	// ── 3. Execute ────────────────────────────────────────────────────────────

	var results []OrderSummaryFilterRow
	if err := query.Scan(&results).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch orders",
			"error":   err.Error(),
		})
	}

	// Kembalikan slice kosong bukan null agar frontend tidak perlu nil-check
	if results == nil {
		results = []OrderSummaryFilterRow{}
	}

	// Tambahkan no urut (sama seperti endpoint lama)
	type OrderSummaryWithNo struct {
		OrderSummaryFilterRow
		No int `json:"no"`
	}
	withNo := make([]OrderSummaryWithNo, len(results))
	for i, r := range results {
		withNo[i] = OrderSummaryWithNo{OrderSummaryFilterRow: r, No: i + 1}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Orders found",
		"data":    withNo,
	})
}
