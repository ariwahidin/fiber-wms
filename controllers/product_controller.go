package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProductController struct {
	DB *gorm.DB
}

var productInput struct {
	ID       uint    `json:"id"`
	ItemCode string  `json:"item_code" validate:"required,min=3"`
	ItemName string  `json:"item_name" validate:"required,min=3"`
	CBM      float64 `json:"cbm" validate:"required"`
	GMC      string  `json:"gmc" validate:"required,min=6"`
	Group    string  `json:"group" validate:"required,min=3"`
	Category string  `json:"category" validate:"required,min=3"`
	Serial   string  `json:"serial" validate:"required,min=1"`
}

func NewProductController(DB *gorm.DB) *ProductController {
	return &ProductController{DB: DB}
}

func (c *ProductController) CreateProduct(ctx *fiber.Ctx) error {

	// fmt.Println("Payload Data : ", string(ctx.Body()))
	// return nil

	// Parse Body
	if err := ctx.BodyParser(&productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Membuat user dengan memasukkan data ke struct models.Product
	product := models.Product{
		ItemCode:  productInput.ItemCode,
		ItemName:  productInput.ItemName,
		CBM:       productInput.CBM,
		Barcode:   productInput.GMC,
		GMC:       productInput.GMC,
		Group:     productInput.Group,
		Category:  productInput.Category,
		HasSerial: productInput.Serial,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&product).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Product created successfully", "data": product})

}

func (c *ProductController) GetProductByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Periksa apakah user dengan ID tersebut ada
	var result models.Product
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product found", "data": result})
}

func (c *ProductController) UpdateProduct(ctx *fiber.Ctx) error {

	fmt.Println("Payload Edit Data : ", string(ctx.Body()))
	// return nil

	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Check if the product exists
	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse Body
	if err := ctx.BodyParser(&productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Membuat user dengan memasukkan data ke struct models.Product
	product.ItemCode = productInput.ItemCode
	product.ItemName = productInput.ItemName
	product.CBM = productInput.CBM
	product.GMC = productInput.GMC
	product.Barcode = productInput.GMC
	product.Group = productInput.Group
	product.Category = productInput.Category
	product.HasSerial = productInput.Serial
	product.UpdatedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("item_code", "item_name", "cbm", "gmc", "barcode", "group", "category", "has_serial", "updated_by").Where("id = ?", id).Updates(&product)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product updated successfully", "data": product})

}

func (c *ProductController) GetAllProducts(ctx *fiber.Ctx) error {
	var products []models.Product
	if err := c.DB.Find(&products).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
}

func (c *ProductController) DeleteProduct(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Periksa apakah user dengan ID tersebut ada
	var product models.Product
	if err := c.DB.First(&product, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&product)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Hapus user
	result = c.DB.Delete(&product)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product deleted successfully", "data": product})
}
