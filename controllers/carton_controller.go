package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MasterCartonController struct {
	DB *gorm.DB
}

func NewMasterCartonController(db *gorm.DB) *MasterCartonController {
	return &MasterCartonController{DB: db}
}

func (c *MasterCartonController) Create(ctx *fiber.Ctx) error {
	var carton models.MasterCarton

	if err := ctx.BodyParser(&carton); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var userID uint
	userID = uint(ctx.Locals("userID").(float64))
	carton.CreatedBy = &userID

	// Calculate volume automatically
	carton.Volume = carton.Length * carton.Width * carton.Height

	if err := c.DB.Create(&carton).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Master Carton created successfully",
		"data":    carton.ToResponse(),
	})
}

func (c *MasterCartonController) GetAll(ctx *fiber.Ctx) error {
	var cartons []models.MasterCarton
	if err := c.DB.Find(&cartons).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Convert to response format
	responses := make([]models.MasterCartonResponse, len(cartons))
	for i, carton := range cartons {
		responses[i] = carton.ToResponse()
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Master Cartons found",
		"data":    responses,
	})
}

func (c *MasterCartonController) GetByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var carton models.MasterCarton
	if err := c.DB.First(&carton, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Master Carton not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Master Carton found",
		"data":    carton.ToResponse(),
	})
}

func (c *MasterCartonController) Update(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var carton models.MasterCarton
	if err := ctx.BodyParser(&carton); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// carton.UpdatedBy = int(ctx.Locals("userID").(float64))
	var userID uint
	userID = uint(ctx.Locals("userID").(float64))
	carton.UpdatedBy = &userID

	// Recalculate volume
	carton.Volume = carton.Length * carton.Width * carton.Height

	if err := c.DB.Model(&carton).Where("id = ?", id).Updates(carton).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Fetch updated data
	if err := c.DB.First(&carton, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Master Carton updated successfully",
		"data":    carton.ToResponse(),
	})
}

func (c *MasterCartonController) Delete(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var carton models.MasterCarton
	if err := c.DB.Delete(&carton, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Master Carton deleted successfully",
	})
}
