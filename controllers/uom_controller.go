package controllers

import (
	"errors"
	"fiber-app/models"
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

	if uomPayolad.ItemCode == "" || uomPayolad.Ean == "" || uomPayolad.FromUom == "" || uomPayolad.ToUom == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item, Ean, FromUom, and ToUom are required", "message": "Please provide all required fields"})
	}

	uomExists := false
	errCheck1 := c.DB.Where("item_code = ? AND ean = ? AND from_uom = ? AND to_uom = ?", uomPayolad.ItemCode, uomPayolad.Ean, uomPayolad.FromUom, uomPayolad.ToUom).First(&models.UomConversion{}).Error
	if errCheck1 == nil {
		uomExists = true
	}

	errCheck2 := c.DB.Where("item_code = ? AND ean = ? ", uomPayolad.ItemCode, uomPayolad.Ean).First(&models.UomConversion{}).Error
	if errCheck2 == nil {
		uomExists = true
	}

	if uomExists {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "UOM conversion already exists", "message": "UOM already exists"})
	}

	uom := models.UomConversion{
		ItemCode:       uomPayolad.ItemCode,
		Ean:            uomPayolad.Ean,
		FromUom:        uomPayolad.FromUom,
		ToUom:          uomPayolad.ToUom,
		ConversionRate: uomPayolad.ConversionRate,
		IsBase:         uomPayolad.IsBase,
		CreatedBy:      int(ctx.Locals("userID").(float64)),
		CreatedAt:      time.Now(),
		UpdatedBy:      int(ctx.Locals("userID").(float64)),
		UpdatedAt:      time.Now(),
	}

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

	if uom.IsLocked {
		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "UOM is locked", "message": "UOM is locked and cannot be updated"})
	}

	uom.ItemCode = uomPayload.ItemCode
	uom.Ean = uomPayload.Ean
	uom.FromUom = uomPayload.FromUom
	uom.ToUom = uomPayload.ToUom
	uom.ConversionRate = float64(uomPayload.ConversionRate)
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

func (c *UomController) GetUomConversionByItemCodeAndFromUom(ctx *fiber.Ctx) error {
	var payload struct {
		ItemCode string `json:"item_code" validate:"required"`
		FromUom  string `json:"from_uom" validate:"required"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "message": "Invalid request payload"})
	}

	item_code := payload.ItemCode

	var uoms models.UomConversion

	if err := c.DB.Where("item_code = ? AND from_uom = ?", item_code, payload.FromUom).First(&uoms).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM not found for item: " + item_code + " from UoM: " + payload.FromUom})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "UOM retrieved successfully", "data": uoms})
}
