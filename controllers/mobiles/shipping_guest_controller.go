package mobiles

import (
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ShippingGuestController struct {
	DB *gorm.DB
}

func NewShippingGuestController(DB *gorm.DB) *ShippingGuestController {
	return &ShippingGuestController{DB: DB}
}

func (c *ShippingGuestController) GetListShippingOpenBySPK(ctx *fiber.Ctx) error {

	spk := ctx.Params("spk")
	if spk == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "spk is required"})
	}

	var orderHeaders []models.OrderHeader
	if err := c.DB.Table("order_headers").Where("order_no = ? AND status = ?", spk, "open").Find(&orderHeaders).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get order headers"})
	}
	if len(orderHeaders) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No open shipping found for the given SPK"})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Getting list of shipping open by SPK successful", "data": orderHeaders})
}

func (c *ShippingGuestController) UpdateShipping(ctx *fiber.Ctx) error {
	type RequestBody struct {
		OrderNo    string  `json:"order_no"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		DriverName string  `json:"driver_name"`
		Remarks    string  `json:"remarks"`
		Status     string  `json:"status"`
	}

	var body RequestBody

	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Optional: Validasi sederhana
	if body.OrderNo == "" || body.DriverName == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "order_no and driver_name are required",
		})
	}

	var orderHeader models.OrderHeader
	if err := c.DB.Where("order_no = ?", body.OrderNo).First(&orderHeader).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Order not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve order header",
		})
	}

	// insert to order console
	orderConsole := models.OrderConsole{
		OrderID:   orderHeader.ID,
		OrderNo:   orderHeader.OrderNo,
		Status:    body.Status,
		Driver:    body.DriverName,
		Longitude: body.Longitude,
		Latitude:  body.Latitude,
		Remarks:   body.Remarks,
	}
	if err := c.DB.Create(&orderConsole).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create order console",
		})
	}

	ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Update shipping successful",
	})
	return nil
}
