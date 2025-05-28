package controllers

import (
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type WarehouseController struct {
	DB *gorm.DB
}

func SetuWarehouseController(DB *gorm.DB) *WarehouseController {
	return &WarehouseController{DB}
}

func (c *WarehouseController) GetAllWarehouses(ctx *fiber.Ctx) error {
	var warehouses []models.Warehouse
	if err := c.DB.Find(&warehouses).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    warehouses,
	})
}
