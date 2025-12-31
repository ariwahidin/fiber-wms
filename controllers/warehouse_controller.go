package controllers

import (
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type WarehouseController struct {
	DB *gorm.DB
}

func SetupWarehouseController(DB *gorm.DB) *WarehouseController {
	return &WarehouseController{DB}
}

// GetAllWarehouses - Mendapatkan semua warehouse
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

// GetWarehouseByID - Mendapatkan warehouse berdasarkan ID
func (c *WarehouseController) GetWarehouseByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var warehouse models.Warehouse

	if err := c.DB.First(&warehouse, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Warehouse not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    warehouse,
	})
}

// CreateWarehouse - Membuat warehouse baru
func (c *WarehouseController) CreateWarehouse(ctx *fiber.Ctx) error {
	var warehouse models.Warehouse

	if err := ctx.BodyParser(&warehouse); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	if err := c.DB.Create(&warehouse).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Warehouse created successfully",
		"data":    warehouse,
	})
}

// UpdateWarehouse - Mengupdate warehouse berdasarkan ID
func (c *WarehouseController) UpdateWarehouse(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var warehouse models.Warehouse

	// Cek apakah warehouse ada
	if err := c.DB.First(&warehouse, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Warehouse not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse request body
	if err := ctx.BodyParser(&warehouse); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Update warehouse
	if err := c.DB.Save(&warehouse).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Warehouse updated successfully",
		"data":    warehouse,
	})
}

// DeleteWarehouse - Menghapus warehouse berdasarkan ID
func (c *WarehouseController) DeleteWarehouse(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var warehouse models.Warehouse

	// Cek apakah warehouse ada
	if err := c.DB.First(&warehouse, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Warehouse not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hapus warehouse
	if err := c.DB.Delete(&warehouse).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Warehouse deleted successfully",
	})
}

// package controllers

// import (
// 	"fiber-app/models"

// 	"github.com/gofiber/fiber/v2"
// 	"gorm.io/gorm"
// )

// type WarehouseController struct {
// 	DB *gorm.DB
// }

// func SetuWarehouseController(DB *gorm.DB) *WarehouseController {
// 	return &WarehouseController{DB}
// }

// func (c *WarehouseController) GetAllWarehouses(ctx *fiber.Ctx) error {
// 	var warehouses []models.Warehouse
// 	if err := c.DB.Find(&warehouses).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}
// 	return ctx.JSON(fiber.Map{
// 		"success": true,
// 		"data":    warehouses,
// 	})
// }
