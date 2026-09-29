package item_controller

import (
	"errors"
	"strings"

	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProductBundleController struct {
	DB *gorm.DB
}

func NewProductBundleController(DB *gorm.DB) *ProductBundleController {
	return &ProductBundleController{
		DB: DB,
	}
}

// ============================================================
// Request / Response Struct
// ============================================================

type productBundleItemInput struct {
	ItemId uint    `json:"item_id" validate:"required"`
	Qty    float64 `json:"qty" validate:"gt=0"`
}

type createProductBundleInput struct {
	BundleProductId uint                     `json:"bundle_product_id" validate:"required"`
	Items           []productBundleItemInput `json:"items" validate:"required,min=1"`
}

type updateProductBundleInput struct {
	Items []productBundleItemInput `json:"items" validate:"required,min=1"`
}

type productBundleResponse struct {
	ID              uint    `json:"id"`
	BundleProductId uint    `json:"bundle_product_id"`
	BundleItemCode  string  `json:"bundle_item_code"`
	BundleItemName  string  `json:"bundle_item_name"`
	ItemId          uint    `json:"item_id"`
	ItemCode        string  `json:"item_code"`
	ItemName        string  `json:"item_name"`
	Qty             float64 `json:"qty"`
}

// ============================================================
// GET ALL BUNDLES
// ============================================================
//
// GET /products/bundles
//
// Optional:
// ?owner=OWNER001
func (c *ProductBundleController) GetAllBundles(ctx *fiber.Ctx) error {
	type bundleRow struct {
		ID              uint
		BundleProductId uint
		BundleItemCode  string
		BundleItemName  string
		ItemId          uint
		ItemCode        string
		ItemName        string
		Qty             float64
	}

	var rows []bundleRow

	query := c.DB.Table("product_bundles AS pb").
		Select(`
			pb.id,
			pb.bundle_product_id,
			bp.item_code AS bundle_item_code,
			bp.item_name AS bundle_item_name,
			pb.item_id,
			ip.item_code,
			ip.item_name,
			pb.qty
		`).
		Joins("INNER JOIN products AS bp ON bp.id = pb.bundle_product_id AND bp.deleted_at IS NULL").
		Joins("INNER JOIN products AS ip ON ip.id = pb.item_id AND ip.deleted_at IS NULL").
		Where("pb.deleted_at IS NULL").
		Order("bp.item_code ASC").
		Order("ip.item_code ASC")

	if owner := strings.TrimSpace(ctx.Query("owner")); owner != "" {
		query = query.Where("bp.owner_code = ?", owner)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get product bundles",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product bundles found",
		"data":    rows,
	})
}

// ============================================================
// GET BUNDLE BY PRODUCT ID
// ============================================================
//
// GET /products/bundles/:id
//
// :id = Product ID dari bundle
func (c *ProductBundleController) GetBundleByProductID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid bundle product ID",
		})
	}

	var bundleProduct models.Product

	if err := c.DB.
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get bundle product",
			"error":   err.Error(),
		})
	}

	if strings.ToUpper(bundleProduct.IsBundle) != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Product is not a bundle",
		})
	}

	var items []productBundleResponse

	err = c.DB.Table("product_bundles AS pb").
		Select(`
			pb.id,
			pb.bundle_product_id,
			bp.item_code AS bundle_item_code,
			bp.item_name AS bundle_item_name,
			pb.item_id,
			ip.item_code,
			ip.item_name,
			pb.qty
		`).
		Joins("INNER JOIN products AS bp ON bp.id = pb.bundle_product_id AND bp.deleted_at IS NULL").
		Joins("INNER JOIN products AS ip ON ip.id = pb.item_id AND ip.deleted_at IS NULL").
		Where("pb.bundle_product_id = ?", id).
		Where("pb.deleted_at IS NULL").
		Order("ip.item_code ASC").
		Scan(&items).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get bundle items",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Bundle found",
		"data": fiber.Map{
			"bundle": bundleProduct,
			"items":  items,
		},
	})
}

// ============================================================
// CREATE BUNDLE
// ============================================================
//
// POST /products/bundles
//
// Body:
//
//	{
//	    "bundle_product_id": 100,
//	    "items": [
//	        {
//	            "item_id": 101,
//	            "qty": 1
//	        },
//	        {
//	            "item_id": 102,
//	            "qty": 2
//	        }
//	    ]
//	}
func (c *ProductBundleController) CreateBundle(ctx *fiber.Ctx) error {
	var input createProductBundleInput

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Basic validation
	if input.BundleProductId == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "bundle_product_id is required",
		})
	}

	if len(input.Items) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Bundle must contain at least one item",
		})
	}

	// Get user ID
	userID := int(ctx.Locals("userID").(float64))

	// --------------------------------------------------------
	// Validate Bundle Product
	// --------------------------------------------------------

	var bundleProduct models.Product

	if err := c.DB.
		Where("id = ?", input.BundleProductId).
		Where("deleted_at IS NULL").
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to validate bundle product",
			"error":   err.Error(),
		})
	}

	if strings.ToUpper(bundleProduct.IsBundle) != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Selected product is not marked as bundle",
		})
	}

	// --------------------------------------------------------
	// Validate Duplicate Items
	// --------------------------------------------------------

	itemIDs := make(map[uint]bool)

	for _, item := range input.Items {

		if item.ItemId == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item ID is required",
			})
		}

		if item.Qty <= 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item quantity must be greater than 0",
			})
		}

		// Bundle tidak boleh memasukkan dirinya sendiri
		if item.ItemId == input.BundleProductId {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product cannot be one of its own items",
			})
		}

		// Item tidak boleh duplicate
		if itemIDs[item.ItemId] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found in bundle",
			})
		}

		itemIDs[item.ItemId] = true
	}

	// --------------------------------------------------------
	// Get Component Products
	// --------------------------------------------------------

	var products []models.Product

	if err := c.DB.
		Where("id IN ?", itemIDsToSlice(itemIDs)).
		Where("deleted_at IS NULL").
		Find(&products).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to validate bundle items",
			"error":   err.Error(),
		})
	}

	if len(products) != len(input.Items) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "One or more bundle items were not found",
		})
	}

	// --------------------------------------------------------
	// Check Existing Bundle
	// --------------------------------------------------------

	var existingCount int64

	if err := c.DB.Model(&models.ProductBundle{}).
		Where("bundle_product_id = ?", input.BundleProductId).
		Where("deleted_at IS NULL").
		Count(&existingCount).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check existing bundle",
			"error":   err.Error(),
		})
	}

	if existingCount > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Bundle already exists",
		})
	}

	// --------------------------------------------------------
	// Transaction
	// --------------------------------------------------------

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	for _, inputItem := range input.Items {

		var product models.Product

		if err := tx.First(&product, inputItem.ItemId).Error; err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Bundle item not found",
			})
		}

		bundleItem := models.ProductBundle{
			BundleProductId: input.BundleProductId,
			ItemId:          inputItem.ItemId,
			ItemCode:        product.ItemCode,
			Qty:             inputItem.Qty,
			CreatedBy:       userID,
			UpdatedBy:       userID,
		}

		if err := tx.Create(&bundleItem).Error; err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create bundle item",
				"error":   err.Error(),
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit bundle",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Product bundle created successfully",
	})
}

// ============================================================
// UPDATE BUNDLE
// ============================================================
//
// PUT /products/bundles/:id
//
// :id = bundle product ID
func (c *ProductBundleController) UpdateBundle(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid bundle product ID",
		})
	}

	var input updateProductBundleInput

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	if len(input.Items) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Bundle must contain at least one item",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	// --------------------------------------------------------
	// Validate Bundle Product
	// --------------------------------------------------------

	var bundleProduct models.Product

	if err := c.DB.
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get bundle product",
			"error":   err.Error(),
		})
	}

	if strings.ToUpper(bundleProduct.IsBundle) != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Product is not a bundle",
		})
	}

	// --------------------------------------------------------
	// Validate Items
	// --------------------------------------------------------

	itemIDs := make(map[uint]bool)

	for _, item := range input.Items {

		if item.ItemId == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item ID is required",
			})
		}

		if item.Qty <= 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Item quantity must be greater than 0",
			})
		}

		if item.ItemId == uint(id) {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product cannot be one of its own items",
			})
		}

		if itemIDs[item.ItemId] {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Duplicate item found in bundle",
			})
		}

		itemIDs[item.ItemId] = true
	}

	// --------------------------------------------------------
	// Validate All Products Exist
	// --------------------------------------------------------

	var products []models.Product

	if err := c.DB.
		Where("id IN ?", itemIDsToSlice(itemIDs)).
		Where("deleted_at IS NULL").
		Find(&products).Error; err != nil {

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to validate bundle items",
			"error":   err.Error(),
		})
	}

	if len(products) != len(input.Items) {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "One or more bundle items were not found",
		})
	}

	// --------------------------------------------------------
	// Transaction
	// --------------------------------------------------------

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Soft delete existing bundle details
	if err := tx.
		Where("bundle_product_id = ?", uint(id)).
		Delete(&models.ProductBundle{}).Error; err != nil {

		tx.Rollback()

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to remove existing bundle items",
			"error":   err.Error(),
		})
	}

	// Insert new bundle details
	for _, inputItem := range input.Items {

		var product models.Product

		if err := tx.First(&product, inputItem.ItemId).Error; err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Bundle item not found",
			})
		}

		bundleItem := models.ProductBundle{
			BundleProductId: uint(id),
			ItemId:          inputItem.ItemId,
			ItemCode:        product.ItemCode,
			Qty:             inputItem.Qty,
			CreatedBy:       userID,
			UpdatedBy:       userID,
		}

		if err := tx.Create(&bundleItem).Error; err != nil {
			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create bundle item",
				"error":   err.Error(),
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit bundle update",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product bundle updated successfully",
	})
}

// ============================================================
// DELETE BUNDLE
// ============================================================
//
// DELETE /products/bundles/:id
//
// :id = bundle product ID
func (c *ProductBundleController) DeleteBundle(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid bundle product ID",
		})
	}

	var bundleProduct models.Product

	if err := c.DB.
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Bundle product not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get bundle product",
			"error":   err.Error(),
		})
	}

	if strings.ToUpper(bundleProduct.IsBundle) != "Y" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Product is not a bundle",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Soft delete all bundle details
	if err := tx.
		Where("bundle_product_id = ?", uint(id)).
		Delete(&models.ProductBundle{}).Error; err != nil {

		tx.Rollback()

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete bundle items",
			"error":   err.Error(),
		})
	}

	// Optional: jangan delete Product-nya.
	// Kita hanya menghapus konfigurasi bundle.
	if err := tx.Model(&models.Product{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_bundle":  "N",
			"updated_by": userID,
		}).Error; err != nil {

		tx.Rollback()

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to reset bundle product",
			"error":   err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit bundle deletion",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product bundle deleted successfully",
	})
}

// ============================================================
// Helper
// ============================================================

func itemIDsToSlice(items map[uint]bool) []uint {
	result := make([]uint, 0, len(items))

	for id := range items {
		result = append(result, id)
	}

	return result
}
