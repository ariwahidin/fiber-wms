package mobiles

import (
	"errors"
	"fiber-app/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobileOutboundController struct {
	DB *gorm.DB
}

func NewMobileOutboundController(DB *gorm.DB) *MobileOutboundController {
	return &MobileOutboundController{DB: DB}
}

func (c *MobileOutboundController) GetListOutbound(ctx *fiber.Ctx) error {
	type listOutboundResponse struct {
		ID           uint      `json:"id"`
		OutboundNo   string    `json:"outbound_no"`
		CustomerName string    `json:"customer_name"`
		Status       string    `json:"status"`
		ShipmentID   string    `json:"shipment_id"`
		QtyReq       int       `json:"qty_req"`
		QtyScan      int       `json:"qty_scan"`
		QtyPack      int       `json:"qty_pack"`
		UpdatedAt    time.Time `json:"updated_at"`
	}

	sql := `WITH od AS
	(SELECT outbound_id, SUM(quantity) qty_req, SUM(scan_qty) as scan_qty 
	FROM outbound_details
	GROUP BY outbound_id),
	kd AS (
	SELECT outbound_id, SUM(qty) AS qty_pack
	FROM outbound_scan_details
	GROUP BY outbound_id
	)

	SELECT a.id, a.outbound_no, b.customer_name,
	a.shipment_id, od.qty_req, od.scan_qty, kd.qty_pack,
	a.status, a.updated_at
	FROM outbound_headers a
	INNER JOIN customers b ON a.customer_code = b.customer_code
	LEFT JOIN od ON a.id = od.outbound_id
	LEFT JOIN kd ON a.id = kd.outbound_id
	WHERE a.status = 'picking'
	ORDER BY a.id DESC`
	var listOutbound []listOutboundResponse
	if err := c.DB.Raw(sql).Scan(&listOutbound).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": listOutbound})
}

func (c *MobileOutboundController) GetListOutboundDetail(ctx *fiber.Ctx) error {

	outbound_no := ctx.Params("outbound_no")

	if outbound_no == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "outbound_no is required"})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Debug().Where("outbound_no = ?", outbound_no).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound_no not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var listOutboundDetails []models.OutboundDetail
	if err := c.DB.Debug().Where("outbound_id = ?", outboundHeader.ID).Find(&listOutboundDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": listOutboundDetails})
}
