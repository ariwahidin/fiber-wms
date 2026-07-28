package item_controller

import (
	"errors"
	"fiber-app/models"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CategoryController struct {
	DB *gorm.DB
}

func NewCategoryController(DB *gorm.DB) *CategoryController {
	return &CategoryController{DB: DB}
}

type categoryInputStruct struct {
	Code    string `json:"code" validate:"required,min=2"`
	Name    string `json:"name" validate:"required,min=3"`
	Remarks string `json:"remarks"`
}

func (c *CategoryController) CreateCategory(ctx *fiber.Ctx) error {
	var input categoryInputStruct
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	code := strings.ToUpper(strings.TrimSpace(input.Code))

	var existing models.Category
	if err := c.DB.Where("code = ?", code).First(&existing).Error; err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Category code already exists"})
	}

	userID := int(ctx.Locals("userID").(float64))

	category := models.Category{
		Code:      code,
		Name:      strings.TrimSpace(input.Name),
		Remarks:   input.Remarks,
		CreatedBy: userID,
	}

	if err := c.DB.Create(&category).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Category created successfully", "data": category})
}

func (c *CategoryController) GetAllCategory(ctx *fiber.Ctx) error {
	var categories []models.Category
	if err := c.DB.Order("code ASC").Find(&categories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Categories found", "data": categories})
}

func (c *CategoryController) GetCategoryByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var category models.Category
	if err := c.DB.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Category found", "data": category})
}

func (c *CategoryController) UpdateCategory(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var category models.Category
	if err := c.DB.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var input categoryInputStruct
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	newCode := strings.ToUpper(strings.TrimSpace(input.Code))

	// Kalau code diubah, cek duplikat ke row lain
	if newCode != category.Code {
		var dupe models.Category
		if err := c.DB.Where("code = ? AND id != ?", newCode, id).First(&dupe).Error; err == nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Category code already exists"})
		}

		// Kalau code lama sudah dipakai di Product, ikutan update biar konsisten
		c.DB.Model(&models.Product{}).Where("category = ?", category.Code).Updates(map[string]interface{}{
			"category": newCode,
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	if err := c.DB.Model(&models.Category{}).Where("id = ?", id).Updates(map[string]interface{}{
		"code":       newCode,
		"name":       strings.TrimSpace(input.Name),
		"remarks":    input.Remarks,
		"updated_at": time.Now(),
		"updated_by": userID,
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	c.DB.First(&category, id)
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Category updated successfully", "data": category})
}

func (c *CategoryController) DeleteCategory(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var category models.Category
	if err := c.DB.First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Category not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Block delete kalau masih dipakai di Product
	var usageCount int64
	if err := c.DB.Model(&models.Product{}).Where("category = ?", category.Code).Count(&usageCount).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if usageCount > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Category is still used by existing products"})
	}

	if err := c.DB.Delete(&category).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Category deleted successfully"})
}
