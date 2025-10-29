package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProductController struct {
	DB *gorm.DB
}

func NewProductController(DB *gorm.DB) *ProductController {
	return &ProductController{DB: DB}
}

var productInput struct {
	ID         uint    `json:"id"`
	ItemCode   string  `json:"item_code" validate:"required,min=3"`
	ItemName   string  `json:"item_name" validate:"required,min=3"`
	CBM        float64 `json:"cbm" validate:"required"`
	GMC        string  `json:"gmc" validate:"required,min=6"`
	Width      float64 `json:"width"`
	Length     float64 `json:"length"`
	Height     float64 `json:"height"`
	Group      string  `json:"group" validate:"required,min=3"`
	Category   string  `json:"category" validate:"required,min=3"`
	Serial     string  `json:"serial" validate:"required,min=1"`
	Waranty    string  `json:"waranty" validate:"required,min=1"`
	Adaptor    string  `json:"adaptor" validate:"required,min=1"`
	ManualBook string  `json:"manual_book" validate:"required,min=1"`
	Uom        string  `json:"uom" validate:"required,min=3"`
	OwnerCode  string  `json:"owner_code" validate:"required,min=3"`
}

func (c *ProductController) CreateProduct(ctx *fiber.Ctx) error {

	// Parse Body
	if err := ctx.BodyParser(&productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(productInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	Uom := models.Uom{}
	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
	if Uom.ID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	// Membuat user dengan memasukkan data ke struct models.Product
	product := models.Product{
		ItemCode:   productInput.ItemCode,
		ItemName:   productInput.ItemName,
		CBM:        productInput.CBM,
		Barcode:    productInput.GMC,
		GMC:        productInput.GMC,
		Width:      productInput.Width,
		Length:     productInput.Length,
		Height:     productInput.Height,
		Group:      productInput.Group,
		Category:   productInput.Category,
		HasSerial:  productInput.Serial,
		HasWaranty: productInput.Waranty,
		HasAdaptor: productInput.Adaptor,
		ManualBook: productInput.ManualBook,
		Uom:        productInput.Uom,
		OwnerCode:  productInput.OwnerCode,
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&product).Error; err != nil {
		c.DB.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	uomConversion := models.UomConversion{
		ItemCode:       product.ItemCode,
		FromUom:        product.Uom,
		ToUom:          product.Uom,
		IsBase:         true,
		ConversionRate: 1,
		CreatedBy:      int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&uomConversion).Error; err != nil {
		// Jika terjadi error saat membuat UomConversion, rollback perubahan pada Product
		c.DB.Rollback()
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

	Uom := models.Uom{}
	c.DB.Where("code = ?", productInput.Uom).First(&Uom)
	if Uom.ID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Uom not found"})
	}

	// product.ItemCode = productInput.ItemCode
	// product.ItemName = productInput.ItemName
	// product.CBM = productInput.CBM
	// product.Barcode = productInput.GMC
	// product.GMC = productInput.GMC
	// product.Group = productInput.Group
	// product.Category = productInput.Category
	// product.HasSerial = productInput.Serial
	// product.Uom = productInput.Uom
	// product.UpdatedBy = int(ctx.Locals("userID").(float64))

	// if err := c.DB.Save(&product).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if err := c.DB.Debug().
		Model(&models.Product{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"item_code":   productInput.ItemCode,
			"item_name":   productInput.ItemName,
			"cbm":         productInput.CBM,
			"gmc":         productInput.GMC,
			"barcode":     productInput.GMC,
			"group":       productInput.Group,
			"category":    productInput.Category,
			"width":       productInput.Width,
			"length":      productInput.Length,
			"height":      productInput.Height,
			"has_serial":  productInput.Serial,
			"has_waranty": productInput.Waranty,
			"has_adaptor": productInput.Adaptor,
			"manual_book": productInput.ManualBook,
			"uom":         productInput.Uom,
			"owner_code":  productInput.OwnerCode,
			"updated_at":  time.Now(),
			"updated_by":  int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Product updated successfully", "data": product})

}

func (c *ProductController) GetAllProducts(ctx *fiber.Ctx) error {

	if ctx.Query("owner") != "" {
		var products []models.Product
		if err := c.DB.Where("owner_code = ?", ctx.Query("owner")).Order("item_code ASC").Find(&products).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
	}

	var products []models.Product
	if err := c.DB.Order("item_code ASC").Find(&products).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Products found", "data": products})
}

func (c *ProductController) GetAllCategory(ctx *fiber.Ctx) error {

	var categories []models.Category
	if err := c.DB.Find(&categories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Categories found", "data": categories})
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
