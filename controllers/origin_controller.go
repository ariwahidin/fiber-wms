package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type OriginController struct {
	DB *gorm.DB
}

func NewOriginController(db *gorm.DB) *OriginController {
	return &OriginController{DB: db}
}

func (c *OriginController) Create(ctx *fiber.Ctx) error {
	var origin models.Origin

	if err := ctx.BodyParser(&origin); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	origin.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&origin).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Origin created successfully", "data": origin})
}

func (c *OriginController) GetAll(ctx *fiber.Ctx) error {

	var origins []models.Origin
	if err := c.DB.Find(&origins).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Origins found", "data": origins})
}

func (c *OriginController) GetByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var result models.Origin
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Origin not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Origin found", "data": result})
}

func (c *OriginController) Update(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var origin models.Origin
	if err := ctx.BodyParser(&origin); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	origin.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&origin).Where("id = ?", id).Updates(origin).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Origin updated successfully", "data": origin})
}
