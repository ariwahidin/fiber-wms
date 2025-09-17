package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TruckController struct {
	DB *gorm.DB
}

func NewTruckController(db *gorm.DB) *TruckController {
	return &TruckController{DB: db}
}

func (c *TruckController) Create(ctx *fiber.Ctx) error {
	var truck models.Truck
	if err := ctx.BodyParser(&truck); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	truck.CreatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Create(&truck).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Truck created successfully", "data": truck})
}

func (c *TruckController) GetByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var result models.Truck
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Truck not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Truck found", "data": result})
}

func (c *TruckController) Update(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var truck models.Truck
	if err := ctx.BodyParser(&truck); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	truck.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&truck).Where("id = ?", id).Updates(truck).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Truck updated successfully", "data": truck})
}

func (c *TruckController) GetAll(ctx *fiber.Ctx) error {

	var trucks []models.Truck
	if err := c.DB.Find(&trucks).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Truck found", "data": trucks})
}
