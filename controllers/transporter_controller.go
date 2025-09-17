package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TransporterController struct {
	DB *gorm.DB
}

func NewTransporterController(db *gorm.DB) *TransporterController {
	return &TransporterController{DB: db}
}

func (c *TransporterController) CreateTransporter(ctx *fiber.Ctx) error {

	var transporter models.Transporter

	if err := ctx.BodyParser(&transporter); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	transporter.CreatedBy = int(ctx.Locals("userID").(float64))

	fmt.Println("Payload Data : ", transporter)

	// return nil

	if err := c.DB.Create(&transporter).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter created successfully", "data": transporter})
}

func (c *TransporterController) GetAllTransporter(ctx *fiber.Ctx) error {

	var transporters []models.Transporter
	if err := c.DB.Find(&transporters).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporters found", "data": transporters})
}

func (c *TransporterController) GetTransporterByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Transporter
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Transporter not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter found", "data": result})
}

func (c *TransporterController) UpdateTransporter(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var transporter models.Transporter
	if err := ctx.BodyParser(&transporter); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	transporter.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&transporter).Where("id = ?", id).Updates(transporter).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transporter updated successfully", "data": transporter})
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
