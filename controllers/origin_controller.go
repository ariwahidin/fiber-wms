package controllers

import (
	"fiber-app/models"
	"fmt"

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

	fmt.Println("Payload Data Origin : ", string(ctx.Body()))
	// return nil

	var origin models.Origin

	if err := ctx.BodyParser(&origin); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	origin.CreatedBy = int(ctx.Locals("userID").(float64))

	fmt.Println("Payload Data : ", origin)

	// return nil

	if err := c.DB.Create(&origin).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter created successfully", "data": origin})
}

func (c *OriginController) GetAll(ctx *fiber.Ctx) error {

	var origins []models.Origin
	if err := c.DB.Find(&origins).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporters found", "data": origins})
}

// func (c *SupplierController) GetSupplierByID(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	var result models.Supplier
// 	if err := c.DB.First(&result, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier found", "data": result})
// }

// func (c *SupplierController) UpdateSupplier(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	if err := ctx.BodyParser(&supplierInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	supplier := models.Supplier{
// 		SupplierCode: supplierInput.SupplierCode,
// 		SupplierName: supplierInput.SupplierName,
// 		UpdatedBy:    int(ctx.Locals("userID").(float64)),
// 	}

// 	// Hanya menyimpan field yang dipilih dengan menggunakan Select
// 	if err := c.DB.Select("supplier_code", "supplier_name", "updated_by").Where("id = ?", id).Updates(&supplier).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier updated successfully", "data": supplier})
// }

// func (c *SupplierController) DeleteSupplier(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	var supplier models.Supplier
// 	if err := c.DB.First(&supplier, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Add SupplierID to DeletedBy field
// 	supplier.DeletedBy = int(ctx.Locals("userID").(float64))

// 	// Hanya menyimpan field yang dipilih dengan menggunakan Select
// 	if err := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&supplier).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if err := c.DB.Delete(&supplier).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier deleted successfully", "data": supplier})
// }
