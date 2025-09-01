package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type StockTakeController struct {
	DB *gorm.DB
}

func NewStockTakeController(DB *gorm.DB) *StockTakeController {
	return &StockTakeController{DB: DB}
}

func (c *StockTakeController) GenerateStockTakeCode() (string, error) {
	var lastCode models.StockTake

	// Ambil inbound terakhir
	if err := c.DB.Last(&lastCode).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Ambil bulan dan tahun saat ini
	currentYear := time.Now().Format("2006")
	currentMonth := time.Now().Format("01")
	currentDay := time.Now().Format("02")

	// Generate nomor inbound baru
	var stoNo string
	if lastCode.Code != "" {
		lastStoNo := lastCode.Code[len(lastCode.Code)-4:]
		if currentDay != lastCode.Code[8:10] {
			stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, 1)
		} else {
			lastStoNoInt, _ := strconv.Atoi(lastStoNo)
			stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, lastStoNoInt+1)
		}
	} else {
		stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, 1)
	}

	return stoNo, nil
}

func (c *StockTakeController) GenerateDataStockTake(ctx *fiber.Ctx) error {
	// 0. Ambil filter dari body
	type Filters struct {
		Area      string `json:"area"`
		FromRow   string `json:"fromRow"`
		ToRow     string `json:"toRow"`
		FromBay   string `json:"fromBay"`
		ToBay     string `json:"toBay"`
		FromLevel string `json:"fromLevel"`
		ToLevel   string `json:"toLevel"`
		FromBin   string `json:"fromBin"`
		ToBin     string `json:"toBin"`
	}
	var req struct {
		Filters Filters `json:"filters"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// 1. Ambil lokasi yang cocok
	var locations []models.Location
	if err := c.DB.
		// Where("area = ?", req.Filters.Area).
		Where("row >= ? AND row <= ?", req.Filters.FromRow, req.Filters.ToRow).
		Where("bay >= ? AND bay <= ?", req.Filters.FromBay, req.Filters.ToBay).
		Where("level >= ? AND level <= ?", req.Filters.FromLevel, req.Filters.ToLevel).
		Where("bin >= ? AND bin <= ?", req.Filters.FromBin, req.Filters.ToBin).
		Where("is_active = ?", true).
		Find(&locations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get locations",
			"error":   err.Error(),
		})
	}

	// Jika tidak ada lokasi yang ditemukan
	if len(locations) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "No locations found",
		})
	}

	// 2. Ambil LocationCode
	var locationCodes []string
	for _, loc := range locations {
		locationCodes = append(locationCodes, loc.LocationCode)
	}

	// 3. Ambil data dari inventory berdasarkan lokasi yang difilter
	var inventories []models.Inventory
	if err := c.DB.
		Where("location IN ?", locationCodes).
		Where("qty_available > ?", 0).
		Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch inventory data",
			"error":   err.Error(),
		})
	}

	// Jika tidak ada inventory yang ditemukan
	if len(inventories) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "No inventory data found",
		})
	}

	// 4. Buat stock_take baru
	stoNo, err := c.GenerateStockTakeCode()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate stock take code",
			"error":   err.Error(),
		})
	}

	stockTake := models.StockTake{
		Code:      stoNo,
		Status:    "open",
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&stockTake).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create stock take",
			"error":   err.Error(),
		})
	}

	// 5. Konversi ke stock_take_items
	var items []models.StockTakeItem
	for _, inv := range inventories {
		item := models.StockTakeItem{
			StockTakeID: stockTake.ID,
			ItemID:      int64(inv.ItemId),
			InventoryID: int64(inv.ID),
			Location:    inv.Location,
			Pallet:      inv.Pallet,
			Barcode:     inv.Barcode,
			// SerialNumber: inv.SerialNumber,
			SystemQty:  inv.QtyAvailable,
			CountedQty: 0,
			Difference: 0,
			CreatedBy:  int(ctx.Locals("userID").(float64)),
		}
		items = append(items, item)
	}

	if len(items) > 0 {
		if err := c.DB.Create(&items).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to insert stock take items",
				"error":   err.Error(),
			})
		}
	}

	// 6. Return response
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Data stock take generated successfully",
		"data": fiber.Map{
			"stock_take": stockTake,
			"items":      items,
		},
	})
}

// func (c *StockTakeController) GenerateDataStockTake(ctx *fiber.Ctx) error {

// 	stoNo, err := c.GenerateStockTakeCode()
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to generate stock take code",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// 1. Buat stock_take baru
// 	stockTake := models.StockTake{
// 		Code:      stoNo,
// 		Status:    "open",
// 		CreatedBy: int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&stockTake).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to create stock take",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// 2. Ambil data dari inventory
// 	var inventories []models.Inventory
// 	if err := c.DB.Find(&inventories).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to fetch inventory data",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// 3. Konversi ke stock_take_items
// 	var items []models.StockTakeItem
// 	for _, inv := range inventories {
// 		item := models.StockTakeItem{
// 			StockTakeID:  stockTake.ID,
// 			ItemID:       int64(inv.ItemId),
// 			InventoryID:  int64(inv.ID),
// 			Location:     inv.Location,
// 			Pallet:       inv.Pallet,
// 			Barcode:      inv.Barcode,
// 			SerialNumber: inv.SerialNumber,
// 			SystemQty:    inv.QtyAvailable,
// 			CountedQty:   0,
// 			Difference:   0,
// 			CreatedBy:    int(ctx.Locals("userID").(float64)),
// 		}
// 		items = append(items, item)
// 	}

// 	if len(items) > 0 {
// 		if err := c.DB.Create(&items).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"message": "Failed to insert stock take items",
// 				"error":   err.Error(),
// 			})
// 		}
// 	}

// 	// 4. Return response
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Data stock take generated successfully",
// 		"data": fiber.Map{
// 			"stock_take": stockTake,
// 			"items":      items,
// 		},
// 	})
// }

func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
	var stockTakes []models.StockTake
	if err := c.DB.Order("id desc").Find(&stockTakes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    stockTakes,
	})
}

func (c *StockTakeController) GetStockTakeDetail(ctx *fiber.Ctx) error {
	code := ctx.Params("code")
	var stockTake models.StockTake

	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": stockTake.Items})
}

func (c *StockTakeController) ScanStockTake(ctx *fiber.Ctx) error {

	type scanInput struct {
		StockTakeCode string `json:"stock_take_code"`
		Location      string `json:"location"`
		Barcode       string `json:"barcode"`
		Qty           int    `json:"qty"`
	}

	var input scanInput
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Bad request"})
	}

	var stockTake models.StockTake
	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", input.StockTakeCode).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	// insert to StockTakeBarcodes

	var stockTakeBarcode models.StockTakeBarcode
	stockTakeBarcode.StockTakeID = stockTake.ID
	stockTakeBarcode.Barcode = input.Barcode
	stockTakeBarcode.CountedQty = input.Qty
	stockTakeBarcode.Location = input.Location
	stockTakeBarcode.CreatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Create(&stockTakeBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Internal Server Error", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Success", "data": stockTake.Items})
}

func (c *StockTakeController) GetStockTakeBarcodeByCode(ctx *fiber.Ctx) error {

	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	var stockTakeBarcodes []models.StockTakeBarcode
	if err := c.DB.Where("stock_take_id = ?", stockTake.ID).Order("created_at desc").Find(&stockTakeBarcodes).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": stockTakeBarcodes})
}

func (c *StockTakeController) GetProgressStockTakeByCode(ctx *fiber.Ctx) error {

	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repoStockTake := repositories.NewStockTakeRepository(c.DB)
	progress, err := repoStockTake.GetProgressStockTakeByID(int(stockTake.ID))
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": progress})
}

// func (c *StockTakeController) GetCardStockTake(ctx *fiber.Ctx) error {
// 	repoStockTake := repositories.NewStockTakeRepository(c.DB)
// 	cards, err := repoStockTake.GetAllStockCard()
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch stock cards", "error": err.Error()})
// 	}
// 	return ctx.JSON(fiber.Map{"success": true, "data": cards})
// }

func (c *StockTakeController) GetCardStockTake(ctx *fiber.Ctx) error {
	var payload struct {
		Filters models.StockCardFilter `json:"filters"`
	}

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	repoStockTake := repositories.NewStockTakeRepository(c.DB)
	cards, err := repoStockTake.GetFilteredStockCard(payload.Filters)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch stock cards",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    cards,
	})
}

func (c *StockTakeController) LoadLocations(ctx *fiber.Ctx) error {
	var locations []models.Location

	if err := c.DB.Where("is_active = ?", true).Find(&locations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to load locations",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    locations,
	})
}
