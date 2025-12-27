package controllers

import (
	"fiber-app/middleware"
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ItemPackagingController struct {
	DB *gorm.DB
}

type ItemPackaging struct {
	ID       uint   `json:"id" gorm:"primaryKey autoincrement;not null"`
	ItemID   uint   `json:"item_id" gorm:"not null;index"`
	ItemCode string `json:"item_code" gorm:"not null"`
	UOM      string `json:"uom" gorm:"not null"`
	Ean      string `json:"ean" gorm:"not null"`

	// Dimensi (cm)
	LengthCM float64 `json:"length_cm" gorm:"type:decimal(10,2);not null;default:0"`
	WidthCM  float64 `json:"width_cm" gorm:"type:decimal(10,2);not null;default:0"`
	HeightCM float64 `json:"height_cm" gorm:"type:decimal(10,2);not null;default:0"`

	// Berat (kg)
	NetWeightKG   float64 `json:"net_weight_kg" gorm:"type:decimal(10,3);not null;default:0"`
	GrossWeightKG float64 `json:"gross_weight_kg" gorm:"type:decimal(10,3);not null;default:0"`

	// Flag tambahan
	IsActive bool `json:"is_active" gorm:"not null;default:true"`

	CreatedBy int
	CreatedAt time.Time
	UpdatedBy int
	UpdatedAt time.Time
}

type UomConversion struct {
	ID             int64          `json:"id" gorm:"primaryKey"`
	ItemCode       string         `json:"item_code"`
	Ean            string         `json:"ean"`
	FromUom        string         `json:"from_uom"`
	ToUom          string         `json:"to_uom"`
	ConversionRate float64        `json:"conversion_rate"`
	IsBase         bool           `json:"is_base"`
	IsLocked       bool           `json:"is_locked" default:"false"`
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `json:"-"`
}

type ItemCodeOption struct {
	ItemCode string `json:"item_code"`
	Ean      string `json:"ean"`
	UOM      string `json:"uom"`
}

func NewItemPackagingController(db *gorm.DB) *ItemPackagingController {
	return &ItemPackagingController{DB: db}
}

// GetAll - Get all item packaging with pagination
func (ctrl *ItemPackagingController) GetAll(c *fiber.Ctx) error {
	var items []ItemPackaging
	var total int64

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	offset := (page - 1) * limit

	query := ctrl.DB.Model(&ItemPackaging{})

	if search != "" {
		query = query.Where("item_code LIKE ? OR ean LIKE ? OR uom LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch item packaging",
		})
	}

	return c.JSON(fiber.Map{
		"data": items,
		"meta": fiber.Map{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetByID - Get single item packaging by ID
func (ctrl *ItemPackagingController) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	var item ItemPackaging

	if err := ctrl.DB.First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Item packaging not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch item packaging",
		})
	}

	return c.JSON(fiber.Map{
		"data": item,
	})
}

// GetItemCodeOptions - Get unique item codes from uom_conversion
func (ctrl *ItemPackagingController) GetItemCodeOptions(c *fiber.Ctx) error {
	var options []ItemCodeOption

	if err := ctrl.DB.Model(&UomConversion{}).
		Select("DISTINCT item_code, ean, from_uom as uom").
		Where("deleted_at IS NULL").
		Order("item_code ASC").
		Find(&options).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch item code options",
		})
	}

	return c.JSON(fiber.Map{
		"data": options,
	})
}

// Create - Create new item packaging
func (ctrl *ItemPackagingController) Create(c *fiber.Ctx) error {
	var item ItemPackaging

	if err := c.BodyParser(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var product models.Product
	if err := ctrl.DB.Where("item_code = ?", item.ItemCode).First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product with given item code not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch product",
		})
	}

	// Set created timestamp
	item.ItemID = uint(product.ID)
	item.CreatedAt = time.Now()
	item.IsActive = true

	if err := ctrl.DB.Create(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create item packaging",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Item packaging created successfully",
		"data":    item,
	})
}

// Update - Update existing item packaging
func (ctrl *ItemPackagingController) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var item ItemPackaging

	if err := ctrl.DB.First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Item packaging not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch item packaging",
		})
	}

	var updateData ItemPackaging
	if err := c.BodyParser(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var product models.Product
	if err := ctrl.DB.Where("item_code = ?", item.ItemCode).First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product with given item code not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch product",
		})
	}

	userID := int(c.Locals("userID").(float64))

	// Update fields
	item.ItemID = uint(product.ID)
	item.ItemCode = updateData.ItemCode
	item.UOM = updateData.UOM
	item.Ean = updateData.Ean
	item.LengthCM = updateData.LengthCM
	item.WidthCM = updateData.WidthCM
	item.HeightCM = updateData.HeightCM
	item.NetWeightKG = updateData.NetWeightKG
	item.GrossWeightKG = updateData.GrossWeightKG
	item.IsActive = updateData.IsActive
	item.UpdatedBy = userID
	item.UpdatedAt = time.Now()

	if err := ctrl.DB.Save(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update item packaging",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Item packaging updated successfully",
		"data":    item,
	})
}

// Delete - Delete item packaging
func (ctrl *ItemPackagingController) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	var item ItemPackaging

	if err := ctrl.DB.First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Item packaging not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch item packaging",
		})
	}

	if err := ctrl.DB.Delete(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete item packaging",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Item packaging deleted successfully",
	})
}

type ItemPackagingUploadResult struct {
	TotalRows     int      `json:"total_rows"`
	SuccessCount  int      `json:"success_count"`
	SkippedCount  int      `json:"skipped_count"`
	ErrorCount    int      `json:"error_count"`
	SkippedItems  []string `json:"skipped_items"`
	ErrorMessages []string `json:"error_messages"`
}

func (ctrl *ItemPackagingController) CreateItemPackagingFromExcel(c *fiber.Ctx) error {
	// Get file from request
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "File is required",
		})
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".xls") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Only Excel files (.xlsx, .xls) are allowed",
		})
	}

	// Open uploaded file
	fileContent, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to open file",
		})
	}
	defer fileContent.Close()

	// Read Excel file
	f, err := excelize.OpenReader(fileContent)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read Excel file",
		})
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "No sheets found in Excel file",
		})
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read rows",
		})
	}

	if len(rows) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Excel file must contain header and at least one data row",
		})
	}

	result := ItemPackagingUploadResult{
		TotalRows:     len(rows) - 1,
		SuccessCount:  0,
		SkippedCount:  0,
		ErrorCount:    0,
		SkippedItems:  []string{},
		ErrorMessages: []string{},
	}

	userID := int(c.Locals("userID").(float64))

	// Cache for product validation
	productCache := make(map[string]models.Product)
	uomCache := make(map[string]bool)

	// Start transaction
	tx := ctrl.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Process each row (skip header)
	for i, row := range rows[1:] {
		rowNum := i + 2 // Excel row number (header is row 1)

		// Skip empty rows
		if len(row) == 0 || (len(row) > 0 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		// Ensure minimum columns
		if len(row) < 8 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Insufficient columns (expected 8)", rowNum))
			continue
		}

		// Sanitize and normalize input
		itemCode := strings.ToUpper(strings.TrimSpace(row[0]))
		uom := strings.ToUpper(strings.TrimSpace(row[1]))
		ean := strings.ToUpper(strings.TrimSpace(row[2]))
		lengthCM := parseFloat(row[3])
		widthCM := parseFloat(row[4])
		heightCM := parseFloat(row[5])
		netWeightKG := parseFloat(row[6])
		grossWeightKG := parseFloat(row[7])

		// Validate required fields
		if itemCode == "" || uom == "" || ean == "" {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Missing required fields (ITEM_CODE, UOM, EAN)", rowNum))
			continue
		}

		// Validate dimensions and weights
		if lengthCM <= 0 || widthCM <= 0 || heightCM <= 0 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Dimensions must be greater than 0", rowNum))
			continue
		}

		if netWeightKG <= 0 || grossWeightKG <= 0 {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Weights must be greater than 0", rowNum))
			continue
		}

		if netWeightKG > grossWeightKG {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Net weight cannot be greater than gross weight", rowNum))
			continue
		}

		// Validate Product exists (with cache)
		product, exists := productCache[itemCode]
		if !exists {
			if err := tx.Where("item_code = ?", itemCode).First(&product).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: Product '%s' not found", rowNum, itemCode))
				continue
			}
			productCache[itemCode] = product
		}

		// Validate UOM exists (with cache)
		if _, exists := uomCache[uom]; !exists {
			var uomModel models.Uom
			if err := tx.Where("code = ?", uom).First(&uomModel).Error; err != nil {
				result.ErrorCount++
				result.ErrorMessages = append(result.ErrorMessages,
					fmt.Sprintf("Row %d: UOM '%s' not found", rowNum, uom))
				continue
			}
			uomCache[uom] = true
		}

		// Check for duplicate (item_code + uom + ean)
		var existingPackaging ItemPackaging
		err := tx.Where("item_code = ? AND uom = ? AND ean = ?", itemCode, uom, ean).
			First(&existingPackaging).Error

		if err == nil {
			result.SkippedCount++
			result.SkippedItems = append(result.SkippedItems,
				fmt.Sprintf("%s (%s - %s)", itemCode, uom, ean))
			continue
		}

		// Create Item Packaging
		itemPackaging := ItemPackaging{
			ItemID:        uint(product.ID),
			ItemCode:      itemCode,
			UOM:           uom,
			Ean:           ean,
			LengthCM:      lengthCM,
			WidthCM:       widthCM,
			HeightCM:      heightCM,
			NetWeightKG:   netWeightKG,
			GrossWeightKG: grossWeightKG,
			IsActive:      true,
			CreatedBy:     userID,
			CreatedAt:     time.Now(),
			UpdatedBy:     userID,
			UpdatedAt:     time.Now(),
		}

		if err := tx.Create(&itemPackaging).Error; err != nil {
			result.ErrorCount++
			result.ErrorMessages = append(result.ErrorMessages,
				fmt.Sprintf("Row %d: Failed to create item packaging - %s", rowNum, err.Error()))
			continue
		}

		result.SuccessCount++
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to commit transaction",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Upload completed: %d success, %d skipped, %d errors",
			result.SuccessCount, result.SkippedCount, result.ErrorCount),
		"data": result,
	})
}

// SetupRoutes - Setup routes for item packaging
func (ctrl *ItemPackagingController) SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1", middleware.AuthMiddleware)

	api.Get("/product/item-packaging", ctrl.GetAll)
	api.Get("/product/item-packaging/item-codes", ctrl.GetItemCodeOptions)
	api.Get("/product/item-packaging/:id", ctrl.GetByID)
	api.Post("/product/item-packaging", ctrl.Create)
	api.Post("/product/item-packaging/upload-excel", ctrl.CreateItemPackagingFromExcel)
	api.Put("/product/item-packaging/:id", ctrl.Update)
	api.Delete("/product/item-packaging/:id", ctrl.Delete)
}
