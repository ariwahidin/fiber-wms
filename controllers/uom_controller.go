package controllers

import (
	"fiber-app/models"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UomController struct {
	DB *gorm.DB
}

func NewUomController(DB *gorm.DB) *UomController {
	return &UomController{DB: DB}
}

func (c *UomController) CreateUom(ctx *fiber.Ctx) error {
	var uomPayolad models.UomConversion
	if err := ctx.BodyParser(&uomPayolad); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	if uomPayolad.ItemCode == "" || uomPayolad.FromUom == "" || uomPayolad.ToUom == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item, FromUom, and ToUom are required", "message": "Please provide all required fields"})
	}

	fmt.Println("Received UOM data:", uomPayolad)

	uom := models.UomConversion{
		ItemCode:       uomPayolad.ItemCode,
		FromUom:        uomPayolad.FromUom,
		ToUom:          uomPayolad.ToUom,
		ConversionRate: uomPayolad.ConversionRate,
		IsBase:         uomPayolad.IsBase,
		CreatedBy:      int(ctx.Locals("userID").(float64)),
		CreatedAt:      time.Now(),
		UpdatedBy:      int(ctx.Locals("userID").(float64)),
		UpdatedAt:      time.Now(),
	}

	fmt.Println("UOM to be created:", uom)

	if err := c.DB.Debug().Create(&uom).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "UOM created successfully",
		"data":    uom,
	})
}

func (c *UomController) GetAllUOMConversion(ctx *fiber.Ctx) error {
	var uoms []models.UomConversion
	if err := c.DB.Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(uoms) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "No UOMs found"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "UOMs retrieved successfully",
		"data":    uoms,
	})
}

func (c *UomController) UpdateUOMConversion(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID is required"})
	}

	var uomPayload models.UomConversionInput

	if err := ctx.BodyParser(&uomPayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	var uom models.UomConversion
	if err := c.DB.Debug().First(&uom, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM not found", "message": "No UOM found with the provided ID"})
	}

	uom.ItemCode = uomPayload.ItemCode
	uom.FromUom = uomPayload.FromUom
	uom.ToUom = uomPayload.ToUom
	uom.ConversionRate = uomPayload.ConversionRate
	uom.IsBase = uomPayload.IsBase
	uom.UpdatedAt = time.Now()
	uom.UpdatedBy = int(ctx.Locals("userID").(float64))

	c.DB.Save(&uom)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM updated successfully", "data": uom})
}

func (c *UomController) GetUomByItemCode(ctx *fiber.Ctx) error {
	var payload struct {
		ItemCode string `json:"item_code" validate:"required"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	item_code := payload.ItemCode

	var uoms []models.UomConversion
	// if err := c.DB.Where("item_code = ? AND is_base = ?", item_code, true).Find(&uoms).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if err := c.DB.Where("item_code = ?", item_code).Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(uoms) == 0 {
		uoms = []models.UomConversion{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM retrieved successfully", "data": uoms})
}

func (c *UomController) GetAllUOM(ctx *fiber.Ctx) error {
	var uoms []models.Uom
	if err := c.DB.Find(&uoms).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Uoms found", "data": uoms})
}
