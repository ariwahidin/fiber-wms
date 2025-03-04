package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type SupplierController struct {
	DB *gorm.DB
}

var supplierInput struct {
	SupplierCode string `json:"supplier_code" gorm:"unique"`
	SupplierName string `json:"supplier_name" gorm:"unique"`
}

func NewSupplierController(db *gorm.DB) *SupplierController {
	return &SupplierController{DB: db}
}

func (c *SupplierController) CreateSupplier(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&supplierInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	supplier := models.Supplier{
		SupplierCode: supplierInput.SupplierCode,
		SupplierName: supplierInput.SupplierName,
	}

	// Add SupplierID to CreatedBy field
	supplier.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier created successfully", "data": supplier})
}

func (c *SupplierController) GetAllSuppliers(ctx *fiber.Ctx) error {
	var suppliers []models.Supplier
	if err := c.DB.Find(&suppliers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Suppliers found", "data": suppliers})
}

func (c *SupplierController) GetSupplierByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Supplier
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier found", "data": result})
}

func (c *SupplierController) UpdateSupplier(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := ctx.BodyParser(&supplierInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	supplier := models.Supplier{
		SupplierCode: supplierInput.SupplierCode,
		SupplierName: supplierInput.SupplierName,
		UpdatedBy:    int(ctx.Locals("userID").(float64)),
	}

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	if err := c.DB.Select("supplier_code", "supplier_name", "updated_by").Where("id = ?", id).Updates(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier updated successfully", "data": supplier})
}

func (c *SupplierController) DeleteSupplier(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var supplier models.Supplier
	if err := c.DB.First(&supplier, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Add SupplierID to DeletedBy field
	supplier.DeletedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	if err := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Delete(&supplier).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Supplier deleted successfully", "data": supplier})
}
