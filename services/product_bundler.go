package services

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"math"

	"gorm.io/gorm"
	// Sesuaikan path ini dengan module project kamu.
	// Contoh:
	// "fiber-wms/models"
)

// ProductBundleService menangani business logic untuk Product Bundle.
//
// Tanggung jawab:
// - Resolve / expand bundle menjadi component SKU
// - Validasi konfigurasi bundle
// - Menghitung available bundle berdasarkan inventory component
//
// Service ini TIDAK melakukan:
// - Create Outbound
// - Allocation inventory
// - Deduct QtyAvailable
// - Picking
type ProductBundleService struct {
	DB *gorm.DB
}

// NewProductBundleService membuat instance ProductBundleService.
func NewProductBundleService(db *gorm.DB) *ProductBundleService {
	return &ProductBundleService{
		DB: db,
	}
}

// BundleComponent adalah hasil expansion sebuah bundle.
//
// Contoh:
//
// # BUNDLE-001 x 2
//
// akan menghasilkan:
//
// SKU-A x 2
// SKU-B x 4
// SKU-C x 2
type BundleComponent struct {
	ItemID   uint    `json:"item_id"`
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Qty      float64 `json:"qty"`
	UOM      string  `json:"uom,omitempty"`
}

// ExpandBundle melakukan expansion Product Bundle menjadi component SKU.
//
// Contoh:
//
// Bundle:
//
// BUNDLE-001
// ├── SKU-A x 1
// ├── SKU-B x 2
// └── SKU-C x 1
//
// Request:
//
// ExpandBundle(tx, 100, 3)
//
// Result:
//
// SKU-A x 3
// SKU-B x 6
// SKU-C x 3
//
// productID adalah ID Product Bundle.
// quantity adalah jumlah bundle yang diminta.
func (s *ProductBundleService) ExpandBundle(
	tx *gorm.DB,
	productID uint,
	quantity float64,
) ([]BundleComponent, error) {

	if tx == nil {
		tx = s.DB
	}

	if tx == nil {
		return nil, errors.New("database connection is required")
	}

	if productID == 0 {
		return nil, errors.New("bundle product ID is required")
	}

	if quantity <= 0 {
		return nil, errors.New("bundle quantity must be greater than zero")
	}

	// ---------------------------------------------------------
	// 1. Get bundle product
	// ---------------------------------------------------------

	var bundleProduct models.Product

	if err := tx.
		Where("id = ?", productID).
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf(
				"bundle product with ID %d not found",
				productID,
			)
		}

		return nil, fmt.Errorf(
			"failed to get bundle product: %w",
			err,
		)
	}

	// Product harus ditandai sebagai bundle.
	if bundleProduct.IsBundle != "Y" {
		return nil, fmt.Errorf(
			"product %s is not a bundle",
			bundleProduct.ItemCode,
		)
	}

	// ---------------------------------------------------------
	// 2. Get bundle components
	// ---------------------------------------------------------

	var bundleItems []models.ProductBundle

	if err := tx.
		Where("bundle_product_id = ?", productID).
		Order("id ASC").
		Find(&bundleItems).Error; err != nil {

		return nil, fmt.Errorf(
			"failed to get bundle components: %w",
			err,
		)
	}

	if len(bundleItems) == 0 {
		return nil, fmt.Errorf(
			"bundle %s has no component items",
			bundleProduct.ItemCode,
		)
	}

	// ---------------------------------------------------------
	// 3. Resolve component products
	// ---------------------------------------------------------

	result := make([]BundleComponent, 0, len(bundleItems))

	for _, bundleItem := range bundleItems {

		if bundleItem.ItemId == 0 {
			return nil, fmt.Errorf(
				"bundle %s contains invalid component with empty item_id",
				bundleProduct.ItemCode,
			)
		}

		if bundleItem.Qty <= 0 {
			return nil, fmt.Errorf(
				"bundle %s contains component %d with invalid quantity %.4f",
				bundleProduct.ItemCode,
				bundleItem.ItemId,
				bundleItem.Qty,
			)
		}

		// Prevent direct self-reference.
		if bundleItem.ItemId == productID {
			return nil, fmt.Errorf(
				"bundle %s cannot contain itself as a component",
				bundleProduct.ItemCode,
			)
		}

		var component models.Product

		if err := tx.
			Where("id = ?", bundleItem.ItemId).
			First(&component).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf(
					"component product ID %d in bundle %s not found",
					bundleItem.ItemId,
					bundleProduct.ItemCode,
				)
			}

			return nil, fmt.Errorf(
				"failed to get component product ID %d: %w",
				bundleItem.ItemId,
				err,
			)
		}

		// -----------------------------------------------------
		// Important:
		// Untuk tahap sekarang nested bundle tidak di-expand.
		//
		// Bundle:
		//   BUNDLE-A
		//      └── BUNDLE-B
		//
		// sebaiknya dicegah di master configuration.
		// -----------------------------------------------------

		if component.IsBundle == "Y" {
			return nil, fmt.Errorf(
				"nested bundle is not supported: component %s is also a bundle",
				component.ItemCode,
			)
		}

		result = append(result, BundleComponent{
			ItemID:   component.ID,
			ItemCode: component.ItemCode,
			ItemName: component.ItemName,
			Qty:      bundleItem.Qty * quantity,
			UOM:      component.Uom,
		})
	}

	return result, nil
}

// ValidateBundle melakukan validasi konfigurasi bundle.
//
// Validasi yang dilakukan:
//
// 1. Product harus ada.
// 2. Product harus IsBundle = Y.
// 3. Bundle harus mempunyai component.
// 4. Component harus valid.
// 5. Component tidak boleh dirinya sendiri.
// 6. Component tidak boleh bundle lain.
// 7. Quantity component harus > 0.
// 8. Tidak boleh ada duplicate component.
func (s *ProductBundleService) ValidateBundle(
	tx *gorm.DB,
	productID uint,
) error {

	if tx == nil {
		tx = s.DB
	}

	if tx == nil {
		return errors.New("database connection is required")
	}

	if productID == 0 {
		return errors.New("bundle product ID is required")
	}

	// ---------------------------------------------------------
	// Get bundle
	// ---------------------------------------------------------

	var bundleProduct models.Product

	if err := tx.
		Where("id = ?", productID).
		First(&bundleProduct).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf(
				"bundle product with ID %d not found",
				productID,
			)
		}

		return fmt.Errorf(
			"failed to get bundle product: %w",
			err,
		)
	}

	if bundleProduct.IsBundle != "Y" {
		return fmt.Errorf(
			"product %s is not marked as bundle",
			bundleProduct.ItemCode,
		)
	}

	// ---------------------------------------------------------
	// Get components
	// ---------------------------------------------------------

	var bundleItems []models.ProductBundle

	if err := tx.
		Where("bundle_product_id = ?", productID).
		Order("id ASC").
		Find(&bundleItems).Error; err != nil {

		return fmt.Errorf(
			"failed to get bundle components: %w",
			err,
		)
	}

	if len(bundleItems) == 0 {
		return fmt.Errorf(
			"bundle %s must have at least one component",
			bundleProduct.ItemCode,
		)
	}

	// ---------------------------------------------------------
	// Duplicate detection
	// ---------------------------------------------------------

	seen := make(map[uint]struct{})

	for _, item := range bundleItems {

		if item.ItemId == 0 {
			return fmt.Errorf(
				"bundle %s contains component with empty item_id",
				bundleProduct.ItemCode,
			)
		}

		if item.Qty <= 0 {
			return fmt.Errorf(
				"bundle %s contains component %d with quantity %.4f; quantity must be greater than zero",
				bundleProduct.ItemCode,
				item.ItemId,
				item.Qty,
			)
		}

		if item.ItemId == productID {
			return fmt.Errorf(
				"bundle %s cannot contain itself",
				bundleProduct.ItemCode,
			)
		}

		if _, exists := seen[item.ItemId]; exists {
			return fmt.Errorf(
				"bundle %s contains duplicate component product ID %d",
				bundleProduct.ItemCode,
				item.ItemId,
			)
		}

		seen[item.ItemId] = struct{}{}

		// -----------------------------------------------------
		// Check component product
		// -----------------------------------------------------

		var component models.Product

		if err := tx.
			Where("id = ?", item.ItemId).
			First(&component).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf(
					"component product ID %d in bundle %s not found",
					item.ItemId,
					bundleProduct.ItemCode,
				)
			}

			return fmt.Errorf(
				"failed to get component product ID %d: %w",
				item.ItemId,
				err,
			)
		}

		// Nested bundle belum kita support.
		if component.IsBundle == "Y" {
			return fmt.Errorf(
				"component %s is also a bundle; nested bundle is not supported",
				component.ItemCode,
			)
		}
	}

	return nil
}

// GetBundleAvailableQty menghitung berapa banyak bundle yang
// dapat dibuat berdasarkan QtyAvailable dari inventory component.
//
// Contoh:
//
// Bundle:
//
// SKU-A x 1
// SKU-B x 2
//
// Inventory:
//
// SKU-A available = 10
// SKU-B available = 5
//
// Result:
//
// SKU-A -> 10 bundle
// SKU-B -> 2 bundle
//
// Available Bundle = 2
//
// ownerCode dan whsCode digunakan untuk membatasi inventory.
//
// divisionCode boleh dikosongkan jika tidak ingin membatasi division.
func (s *ProductBundleService) GetBundleAvailableQty(
	tx *gorm.DB,
	productID uint,
	ownerCode string,
	whsCode string,
	divisionCode string,
) (float64, error) {

	if tx == nil {
		tx = s.DB
	}

	if tx == nil {
		return 0, errors.New("database connection is required")
	}

	if productID == 0 {
		return 0, errors.New("bundle product ID is required")
	}

	// ---------------------------------------------------------
	// Validate bundle terlebih dahulu
	// ---------------------------------------------------------

	if err := s.ValidateBundle(tx, productID); err != nil {
		return 0, err
	}

	// ---------------------------------------------------------
	// Get bundle components
	// ---------------------------------------------------------

	var bundleItems []models.ProductBundle

	if err := tx.
		Where("bundle_product_id = ?", productID).
		Order("id ASC").
		Find(&bundleItems).Error; err != nil {

		return 0, fmt.Errorf(
			"failed to get bundle components: %w",
			err,
		)
	}

	// ---------------------------------------------------------
	// Calculate available bundle
	// ---------------------------------------------------------

	availableBundle := math.Inf(1)

	for _, bundleItem := range bundleItems {

		var component models.Product

		if err := tx.
			Where("id = ?", bundleItem.ItemId).
			First(&component).Error; err != nil {

			return 0, fmt.Errorf(
				"failed to get component product %d: %w",
				bundleItem.ItemId,
				err,
			)
		}

		// -----------------------------------------------------
		// Build inventory query
		// -----------------------------------------------------

		query := tx.
			Model(&models.Inventory{}).
			Where("item_id = ?", component.ID).
			Where("qty_available > ?", 0)

		if ownerCode != "" {
			query = query.Where(
				"owner_code = ?",
				ownerCode,
			)
		}

		if whsCode != "" {
			query = query.Where(
				"whs_code = ?",
				whsCode,
			)
		}

		if divisionCode != "" {
			query = query.Where(
				"division_code = ?",
				divisionCode,
			)
		}

		var availableQty float64

		if err := query.
			Select("COALESCE(SUM(qty_available), 0)").
			Scan(&availableQty).Error; err != nil {

			return 0, fmt.Errorf(
				"failed to calculate available stock for component %s: %w",
				component.ItemCode,
				err,
			)
		}

		// Berapa bundle yang dapat dibuat dari component ini.
		componentBundleQty :=
			availableQty / bundleItem.Qty

		if componentBundleQty < availableBundle {
			availableBundle = componentBundleQty
		}
	}

	// Tidak ada component / inventory.
	if math.IsInf(availableBundle, 1) {
		return 0, nil
	}

	// Jangan return pecahan bundle.
	//
	// Misalnya:
	//
	// availableBundle = 2.5
	//
	// berarti hanya 2 bundle penuh.
	availableBundle = math.Floor(availableBundle)

	if availableBundle < 0 {
		return 0, nil
	}

	return availableBundle, nil
}
