// package controllers

// import (
// 	"errors"
// 	"fiber-app/models"
// 	"fiber-app/repositories"
// 	"fmt"
// 	"strconv"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"gorm.io/gorm"
// )

// type StockTakeController struct {
// 	DB *gorm.DB
// }

// func NewStockTakeController(DB *gorm.DB) *StockTakeController {
// 	return &StockTakeController{DB: DB}
// }

// func (c *StockTakeController) GenerateStockTakeCode() (string, error) {
// 	var lastCode models.StockTake

// 	// Ambil inbound terakhir
// 	if err := c.DB.Last(&lastCode).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
// 		return "", err
// 	}

// 	// Ambil bulan dan tahun saat ini
// 	currentYear := time.Now().Format("2006")
// 	currentMonth := time.Now().Format("01")
// 	currentDay := time.Now().Format("02")

// 	// Generate nomor inbound baru
// 	var stoNo string
// 	if lastCode.Code != "" {
// 		lastStoNo := lastCode.Code[len(lastCode.Code)-4:]
// 		if currentDay != lastCode.Code[8:10] {
// 			stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, 1)
// 		} else {
// 			lastStoNoInt, _ := strconv.Atoi(lastStoNo)
// 			stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, lastStoNoInt+1)
// 		}
// 	} else {
// 		stoNo = fmt.Sprintf("ST%s%s%s%04d", currentYear, currentMonth, currentDay, 1)
// 	}

// 	return stoNo, nil
// }

// func (c *StockTakeController) GenerateDataStockTake(ctx *fiber.Ctx) error {
// 	// 0. Ambil filter dari body
// 	type Filters struct {
// 		Area      string `json:"area"`
// 		FromRow   string `json:"fromRow"`
// 		ToRow     string `json:"toRow"`
// 		FromBay   string `json:"fromBay"`
// 		ToBay     string `json:"toBay"`
// 		FromLevel string `json:"fromLevel"`
// 		ToLevel   string `json:"toLevel"`
// 		FromBin   string `json:"fromBin"`
// 		ToBin     string `json:"toBin"`
// 	}
// 	var req struct {
// 		Filters Filters `json:"filters"`
// 	}
// 	if err := ctx.BodyParser(&req); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Invalid request body",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// 1. Ambil lokasi yang cocok
// 	var locations []models.Location
// 	if err := c.DB.
// 		// Where("area = ?", req.Filters.Area).
// 		Where("row >= ? AND row <= ?", req.Filters.FromRow, req.Filters.ToRow).
// 		Where("bay >= ? AND bay <= ?", req.Filters.FromBay, req.Filters.ToBay).
// 		Where("level >= ? AND level <= ?", req.Filters.FromLevel, req.Filters.ToLevel).
// 		Where("bin >= ? AND bin <= ?", req.Filters.FromBin, req.Filters.ToBin).
// 		Where("is_active = ?", true).
// 		Find(&locations).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to get locations",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Jika tidak ada lokasi yang ditemukan
// 	if len(locations) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"success": false,
// 			"message": "No locations found",
// 		})
// 	}

// 	// 2. Ambil LocationCode
// 	var locationCodes []string
// 	for _, loc := range locations {
// 		locationCodes = append(locationCodes, loc.LocationCode)
// 	}

// 	// 3. Ambil data dari inventory berdasarkan lokasi yang difilter
// 	var inventories []models.Inventory
// 	if err := c.DB.
// 		Where("location IN ?", locationCodes).
// 		Where("qty_available > ?", 0).
// 		Find(&inventories).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to fetch inventory data",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Jika tidak ada inventory yang ditemukan
// 	if len(inventories) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"success": false,
// 			"message": "No inventory data found",
// 		})
// 	}

// 	// 4. Buat stock_take baru
// 	stoNo, err := c.GenerateStockTakeCode()
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to generate stock take code",
// 			"error":   err.Error(),
// 		})
// 	}

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

// 	// 5. Konversi ke stock_take_items
// 	var items []models.StockTakeItem
// 	for _, inv := range inventories {
// 		item := models.StockTakeItem{
// 			StockTakeID: stockTake.ID,
// 			ItemID:      int64(inv.ItemId),
// 			InventoryID: int64(inv.ID),
// 			Location:    inv.Location,
// 			Pallet:      inv.Pallet,
// 			Barcode:     inv.Barcode,
// 			// SerialNumber: inv.SerialNumber,
// 			SystemQty:  int(inv.QtyAvailable),
// 			CountedQty: 0,
// 			Difference: 0,
// 			CreatedBy:  int(ctx.Locals("userID").(float64)),
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

// 	// 6. Return response
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Data stock take generated successfully",
// 		"data": fiber.Map{
// 			"stock_take": stockTake,
// 			"items":      items,
// 		},
// 	})
// }

// func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
// 	var stockTakes []models.StockTake
// 	if err := c.DB.Order("id desc").Find(&stockTakes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    stockTakes,
// 	})
// }

// func (c *StockTakeController) GetStockTakeDetail(ctx *fiber.Ctx) error {
// 	code := ctx.Params("code")
// 	var stockTake models.StockTake

// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "data": stockTake.Items})
// }

// func (c *StockTakeController) ScanStockTake(ctx *fiber.Ctx) error {

// 	type scanInput struct {
// 		StockTakeCode string `json:"stock_take_code"`
// 		Location      string `json:"location"`
// 		Barcode       string `json:"barcode"`
// 		Qty           int    `json:"qty"`
// 	}

// 	var input scanInput
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Bad request"})
// 	}

// 	var stockTake models.StockTake
// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", input.StockTakeCode).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	// insert to StockTakeBarcodes

// 	var stockTakeBarcode models.StockTakeBarcode
// 	stockTakeBarcode.StockTakeID = stockTake.ID
// 	stockTakeBarcode.Barcode = input.Barcode
// 	stockTakeBarcode.CountedQty = input.Qty
// 	stockTakeBarcode.Location = input.Location
// 	stockTakeBarcode.CreatedBy = int(ctx.Locals("userID").(float64))
// 	if err := c.DB.Create(&stockTakeBarcode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Internal Server Error", "error": err.Error()})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "message": "Success", "data": stockTake.Items})
// }

// func (c *StockTakeController) GetStockTakeBarcodeByCode(ctx *fiber.Ctx) error {

// 	code := ctx.Params("code")

// 	var stockTake models.StockTake
// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	var stockTakeBarcodes []models.StockTakeBarcode
// 	if err := c.DB.Where("stock_take_id = ?", stockTake.ID).Order("created_at desc").Find(&stockTakeBarcodes).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "data": stockTakeBarcodes})
// }

// func (c *StockTakeController) GetProgressStockTakeByCode(ctx *fiber.Ctx) error {

// 	code := ctx.Params("code")

// 	var stockTake models.StockTake
// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	repoStockTake := repositories.NewStockTakeRepository(c.DB)
// 	progress, err := repoStockTake.GetProgressStockTakeByID(int(stockTake.ID))
// 	if err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "data": progress})
// }

// // func (c *StockTakeController) GetCardStockTake(ctx *fiber.Ctx) error {
// // 	repoStockTake := repositories.NewStockTakeRepository(c.DB)
// // 	cards, err := repoStockTake.GetAllStockCard()
// // 	if err != nil {
// // 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch stock cards", "error": err.Error()})
// // 	}
// // 	return ctx.JSON(fiber.Map{"success": true, "data": cards})
// // }

// func (c *StockTakeController) GetCardStockTake(ctx *fiber.Ctx) error {
// 	var payload struct {
// 		Filters models.StockCardFilter `json:"filters"`
// 	}

// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Invalid request body",
// 			"error":   err.Error(),
// 		})
// 	}

// 	repoStockTake := repositories.NewStockTakeRepository(c.DB)
// 	cards, err := repoStockTake.GetFilteredStockCard(payload.Filters)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to fetch stock cards",
// 			"error":   err.Error(),
// 		})
// 	}

// 	return ctx.JSON(fiber.Map{
// 		"success": true,
// 		"data":    cards,
// 	})
// }

// func (c *StockTakeController) LoadLocations(ctx *fiber.Ctx) error {
// 	var locations []models.Location

// 	if err := c.DB.Where("is_active = ?", true).Find(&locations).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to load locations",
// 			"error":   err.Error(),
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    locations,
// 	})
// }

package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"strings"
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

// generateStockTakeCodeTx generates a new sequential code within an existing transaction.
// Must be called with a locking read to avoid race conditions on concurrent requests.
func generateStockTakeCodeTx(tx *gorm.DB) (string, error) {
	currentYear := time.Now().Format("2006")
	currentMonth := time.Now().Format("01")
	currentDay := time.Now().Format("02")
	datePrefix := fmt.Sprintf("ST%s%s%s", currentYear, currentMonth, currentDay)

	var lastCode models.StockTake
	err := tx.Raw(`
		SELECT TOP 1 * FROM stock_takes WITH (UPDLOCK, ROWLOCK)
		WHERE code LIKE ? AND deleted_at IS NULL
		ORDER BY id DESC
	`, datePrefix+"%").Scan(&lastCode).Error

	if err != nil {
		return "", err
	}

	if lastCode.Code == "" {
		return fmt.Sprintf("%s%04d", datePrefix, 1), nil
	}

	lastSeq := lastCode.Code[len(lastCode.Code)-4:]
	lastSeqInt, convErr := strconv.Atoi(lastSeq)
	if convErr != nil {
		return "", fmt.Errorf("failed to parse sequence from code %s: %w", lastCode.Code, convErr)
	}

	return fmt.Sprintf("%s%04d", datePrefix, lastSeqInt+1), nil
}

func (c *StockTakeController) GenerateDataStockTake(ctx *fiber.Ctx) error {
	type Filters struct {
		Area         string `json:"area"`
		FromRow      string `json:"fromRow"`
		ToRow        string `json:"toRow"`
		FromBay      string `json:"fromBay"`
		ToBay        string `json:"toBay"`
		FromLevel    string `json:"fromLevel"`
		ToLevel      string `json:"toLevel"`
		FromBin      string `json:"fromBin"`
		ToBin        string `json:"toBin"`
		DivisionCode string `json:"divisionCode"`
		OwnerCode    string `json:"ownerCode"`
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

	userID := int(ctx.Locals("userID").(float64))

	var resultStockTake models.StockTake
	var resultItems []models.StockTakeItem

	txErr := c.DB.Transaction(func(tx *gorm.DB) error {

		// Validasi: customer/owner wajib dipilih sebelum generate stock take
		if req.Filters.OwnerCode == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Please select a customer before generating a stock take")
		}

		// Pastikan owner-nya valid/exist
		var owner models.Owner
		if err := tx.Where("code = ?", req.Filters.OwnerCode).First(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(fiber.StatusBadRequest, "Selected customer was not found")
			}
			return fmt.Errorf("failed to validate owner: %w", err)
		}

		// 1. Ambil lokasi yang cocok — conditional where, skip filter kosong
		locationQuery := tx.Where("is_active = ?", true)

		if req.Filters.Area != "" {
			locationQuery = locationQuery.Where("area = ?", req.Filters.Area)
		}
		if req.Filters.FromRow != "" && req.Filters.ToRow != "" {
			locationQuery = locationQuery.Where("row >= ? AND row <= ?", req.Filters.FromRow, req.Filters.ToRow)
		}
		if req.Filters.FromBay != "" && req.Filters.ToBay != "" {
			locationQuery = locationQuery.Where("bay >= ? AND bay <= ?", req.Filters.FromBay, req.Filters.ToBay)
		}
		if req.Filters.FromLevel != "" && req.Filters.ToLevel != "" {
			locationQuery = locationQuery.Where("level >= ? AND level <= ?", req.Filters.FromLevel, req.Filters.ToLevel)
		}
		if req.Filters.FromBin != "" && req.Filters.ToBin != "" {
			locationQuery = locationQuery.Where("bin >= ? AND bin <= ?", req.Filters.FromBin, req.Filters.ToBin)
		}

		var locations []models.Location
		if err := locationQuery.Find(&locations).Error; err != nil {
			return fmt.Errorf("failed to get locations: %w", err)
		}
		if len(locations) == 0 {
			return fiber.NewError(fiber.StatusNotFound, "No locations found matching the given filters")
		}

		var locationCodes []string
		for _, loc := range locations {
			locationCodes = append(locationCodes, loc.LocationCode)
		}

		// 2. Cek overlap: lokasi ini masih ada di session yang belum closed
		var overlapCount int64
		if err := tx.Model(&models.StockTakeItem{}).
			Joins("JOIN stock_takes ON stock_takes.id = stock_take_items.stock_take_id").
			Where("stock_take_items.location IN ?", locationCodes).
			Where("stock_takes.status IN ?", []string{"open", "in progress"}).
			Where("stock_takes.deleted_at IS NULL").
			Count(&overlapCount).Error; err != nil {
			return fmt.Errorf("failed to check overlapping sessions: %w", err)
		}
		if overlapCount > 0 {
			return fiber.NewError(fiber.StatusBadRequest,
				"Some of the selected locations already have an open or in-progress stock take session. Please close it first before generating a new one.")
		}

		// 3. Ambil data dari inventory berdasarkan lokasi yang difilter
		inventoryQuery := tx.
			Where("location IN ?", locationCodes).
			Where("owner_code = ?", req.Filters.OwnerCode). // wajib, bukan lagi conditional
			Where("qty_available > ?", 0)

		if req.Filters.DivisionCode != "" {
			inventoryQuery = inventoryQuery.Where("division_code = ?", req.Filters.DivisionCode)
		}
		if req.Filters.OwnerCode != "" {
			inventoryQuery = inventoryQuery.Where("owner_code = ?", req.Filters.OwnerCode)
		}

		var inventories []models.Inventory
		if err := inventoryQuery.Find(&inventories).Error; err != nil {
			return fmt.Errorf("failed to fetch inventory data: %w", err)
		}
		if len(inventories) == 0 {
			return fiber.NewError(fiber.StatusNotFound, "No inventory data found for the selected locations")
		}

		// 4. Generate code (locked read, aman dari race condition)
		stoNo, err := generateStockTakeCodeTx(tx)
		if err != nil {
			return fmt.Errorf("failed to generate stock take code: %w", err)
		}

		// 5. Buat stock_take baru
		stockTake := models.StockTake{
			Code:      stoNo,
			Status:    "open",
			CreatedBy: userID,
		}
		if err := tx.Create(&stockTake).Error; err != nil {
			return fmt.Errorf("failed to create stock take: %w", err)
		}

		// 6. Konversi ke stock_take_items
		var items []models.StockTakeItem
		for _, inv := range inventories {
			items = append(items, models.StockTakeItem{
				StockTakeID:  stockTake.ID,
				ItemID:       int64(inv.ItemId),
				InventoryID:  int64(inv.ID),
				Location:     inv.Location,
				Pallet:       inv.Pallet,
				Barcode:      inv.Barcode,
				CartonNumber: inv.CartonNumber,
				LotNumber:    inv.LotNumber,
				DivisionCode: inv.DivisionCode,
				OwnerCode:    inv.OwnerCode,
				SystemQty:    int(inv.QtyAvailable),
				CountedQty:   0,
				Difference:   0,
				CreatedBy:    userID,
			})
		}

		if len(items) > 0 {
			// Batch size dihitung supaya aman di bawah limit 2100 parameter SQL Server.
			// StockTakeItem punya ~17 kolom, jadi 100 rows/batch = ~1700 parameter, masih aman.
			const batchSize = 100
			if err := tx.CreateInBatches(&items, batchSize).Error; err != nil {
				return fmt.Errorf("failed to insert stock take items: %w", err)
			}
		}

		resultStockTake = stockTake
		resultItems = items
		return nil
	})

	if txErr != nil {
		if fe, ok := txErr.(*fiber.Error); ok {
			return ctx.Status(fe.Code).JSON(fiber.Map{
				"success": false,
				"message": fe.Message,
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate stock take",
			"error":   txErr.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Data stock take generated successfully",
		"data": fiber.Map{
			"stock_take": resultStockTake,
			"items":      resultItems,
		},
	})
}

// func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
// 	var stockTakes []models.StockTake
// 	if err := c.DB.Order("id desc").Find(&stockTakes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   err.Error(),
// 		})
// 	}
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    stockTakes,
// 	})
// }

// StockTakeListItem membungkus StockTake dengan ringkasan qty untuk tampilan list —
// system qty diambil dari snapshot stock_take_items (saat generate), counted qty
// diambil dari stock_take_barcodes (hasil scan aktual di lapangan).
type StockTakeListItem struct {
	models.StockTake
	TotalSystemQty  int `json:"total_system_qty"`
	TotalCountedQty int `json:"total_counted_qty"`
}

func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
	var stockTakes []models.StockTake
	if err := c.DB.Order("id desc").Find(&stockTakes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(stockTakes) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"data":    []StockTakeListItem{},
		})
	}

	var stockTakeIDs []uint
	for _, st := range stockTakes {
		stockTakeIDs = append(stockTakeIDs, st.ID)
	}

	type qtyAgg struct {
		StockTakeID uint
		Total       int
	}

	// Total qty sistem (snapshot pas generate stock take)
	var systemAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeItem{}).
		Select("stock_take_id, SUM(system_qty) as total").
		Where("stock_take_id IN ?", stockTakeIDs).
		Group("stock_take_id").
		Scan(&systemAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to aggregate system qty",
			"error":   err.Error(),
		})
	}

	// Total qty yang sudah dihitung (hasil scan aktual)
	var countedAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeBarcode{}).
		Select("stock_take_id, SUM(counted_qty) as total").
		Where("stock_take_id IN ?", stockTakeIDs).
		Group("stock_take_id").
		Scan(&countedAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to aggregate counted qty",
			"error":   err.Error(),
		})
	}

	systemMap := make(map[uint]int, len(systemAggs))
	for _, a := range systemAggs {
		systemMap[a.StockTakeID] = a.Total
	}
	countedMap := make(map[uint]int, len(countedAggs))
	for _, a := range countedAggs {
		countedMap[a.StockTakeID] = a.Total
	}

	result := make([]StockTakeListItem, 0, len(stockTakes))
	for _, st := range stockTakes {
		result = append(result, StockTakeListItem{
			StockTake:       st,
			TotalSystemQty:  systemMap[st.ID],
			TotalCountedQty: countedMap[st.ID],
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// ================== Detail (halaman utama) ==================

// StockTakeDetailRow adalah hasil join + group by stock_take_items x products.
// Di-group per location + item, qty di-SUM.
type StockTakeDetailRow struct {
	ItemID       uint   `json:"item_id"`
	ItemCode     string `json:"item_code"`
	ItemName     string `json:"item_name"`
	UnitModel    string `json:"unit_model"`
	Location     string `json:"location"`
	DivisionCode string `json:"division_code"`
	SystemQty    int    `json:"system_qty"`
	CountedQty   int    `json:"counted_qty"`
	Difference   int    `json:"difference"`
}

func (c *StockTakeController) GetStockTakeDetail(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.Select("id, code, created_at, updated_at, status").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	var rows []StockTakeDetailRow

	query := `
		SELECT
			sti.item_id,
			ISNULL(p.item_code, '')  AS item_code,
			ISNULL(p.item_name, '')  AS item_name,
			ISNULL(p.unit_model, '') AS unit_model,
			sti.location,
			sti.division_code,
			SUM(sti.system_qty)      AS system_qty,
			SUM(sti.counted_qty)     AS counted_qty,
			SUM(sti.difference)      AS difference
		FROM stock_take_items sti
		LEFT JOIN products p ON p.id = sti.item_id
		WHERE sti.stock_take_id = ?
		  AND sti.deleted_at IS NULL
		GROUP BY
			sti.item_id,
			sti.location,
			sti.division_code,
			p.item_code,
			p.item_name,
			p.unit_model
		ORDER BY sti.location ASC, item_code ASC
	`

	if err := c.DB.Raw(query, stockTake.ID).Scan(&rows).Error; err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch stock take detail",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"stock_take": stockTake,
		"rows":       rows,
	}})
}

// ================== Print (by Row) ==================

// StockTakePrintRow adalah hasil untuk halaman print, di-filter by Row terpilih.
type StockTakePrintRow struct {
	Location   string `json:"location"`
	ItemCode   string `json:"item_code"`
	ItemName   string `json:"item_name"`
	SystemQty  int    `json:"system_qty"`
	CountedQty int    `json:"counted_qty"`
	Difference int    `json:"difference"`
}

// GetStockTakePrintDetail dipanggil dari halaman print.
// Query param: ?rows=A,B,C  (Row yang dipilih user di modal)
func (c *StockTakeController) GetStockTakePrintDetail(ctx *fiber.Ctx) error {
	code := ctx.Params("code")
	rowsParam := ctx.Query("rows")

	var stockTake models.StockTake
	if err := c.DB.Select("id, code, created_at, updated_at").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	var selectedRows []string
	if rowsParam != "" {
		for _, r := range strings.Split(rowsParam, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				selectedRows = append(selectedRows, r)
			}
		}
	}

	args := []interface{}{stockTake.ID}

	query := `
		SELECT
			sti.location,
			ISNULL(p.item_code, '') AS item_code,
			ISNULL(p.item_name, '') AS item_name,
			SUM(sti.system_qty)     AS system_qty,
			SUM(sti.counted_qty)    AS counted_qty,
			SUM(sti.difference)     AS difference
		FROM stock_take_items sti
		LEFT JOIN products p ON p.id = sti.item_id
		LEFT JOIN locations l ON l.location_code = sti.location
		WHERE sti.stock_take_id = ?
		  AND sti.deleted_at IS NULL
	`

	// Kalau user pilih Row tertentu, filter di sini. Kalau kosong (misal
	// dipanggil tanpa modal), tampilkan semua row.
	if len(selectedRows) > 0 {
		placeholders := make([]string, len(selectedRows))
		for i, r := range selectedRows {
			placeholders[i] = "?"
			args = append(args, r)
		}
		query += " AND l.row IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += `
		GROUP BY sti.location, p.item_code, p.item_name
		ORDER BY sti.location ASC
	`

	var rows []StockTakePrintRow
	if err := c.DB.Raw(query, args...).Scan(&rows).Error; err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch print data",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success":    true,
		"stock_take": stockTake,
		"data":       rows,
		"rows":       selectedRows, // dikirim balik biar FE gampang render header "Row: A, B, C"
	})
}

// func (c *StockTakeController) GetStockTakeDetail(ctx *fiber.Ctx) error {
// 	code := ctx.Params("code")
// 	var stockTake models.StockTake

// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "data": stockTake.Items})
// }

// func (c *StockTakeController) ScanStockTake(ctx *fiber.Ctx) error {
// 	type scanInput struct {
// 		StockTakeCode string `json:"stock_take_code"`
// 		Location      string `json:"location"`
// 		Barcode       string `json:"barcode"`
// 		Qty           int    `json:"qty"`
// 	}

// 	var input scanInput
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Bad request"})
// 	}

// 	var stockTake models.StockTake
// 	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", input.StockTakeCode).Error; err != nil {
// 		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
// 	}

// 	var stockTakeBarcode models.StockTakeBarcode
// 	stockTakeBarcode.StockTakeID = stockTake.ID
// 	stockTakeBarcode.Barcode = input.Barcode
// 	stockTakeBarcode.CountedQty = input.Qty
// 	stockTakeBarcode.Location = input.Location
// 	stockTakeBarcode.CreatedBy = int(ctx.Locals("userID").(float64))
// 	if err := c.DB.Create(&stockTakeBarcode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Internal Server Error", "error": err.Error()})
// 	}

// 	return ctx.JSON(fiber.Map{"success": true, "message": "Success", "data": stockTake.Items})
// }

func (c *StockTakeController) ScanStockTake(ctx *fiber.Ctx) error {
	type scanInput struct {
		StockTakeCode string `json:"stock_take_code"`
		DivisionCode  string `json:"division_code"`
		Location      string `json:"location"`
		Barcode       string `json:"barcode"`
		Sku           string `json:"sku"`
		LotNumber     string `json:"lot_number"`
		CartonNumber  string `json:"carton_number"`
		Qty           int    `json:"qty"`
		QrRaw         string `json:"qr_raw"`
	}

	var input scanInput
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Bad request"})
	}

	if input.DivisionCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Please select a division before scanning"})
	}
	if input.Location == "" || (input.Barcode == "" && input.Sku == "") {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Location and barcode/SKU are required"})
	}

	if len(input.Location) > 8 || len(input.Location) < 8 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Location must be 8 characters long"})
	}

	if input.Qty < 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Quantity must be at least 1"})
	}

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", input.StockTakeCode).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Stock take session not found"})
	}

	// Sesi yang sudah closed/cancelled tidak boleh menerima scan baru
	if stockTake.Status == "closed" || stockTake.Status == "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot scan — session is already '%s'", stockTake.Status),
		})
	}

	// QR mode -> validate by SKU (item_code). EAN mode -> validate by barcode.
	var product models.Product
	var lookupErr error
	if input.Sku != "" {
		lookupErr = c.DB.Where("item_code = ?", input.Sku).First(&product).Error
	} else {
		lookupErr = c.DB.Where("barcode = ?", input.Barcode).First(&product).Error
	}
	if lookupErr != nil {
		identifier := input.Sku
		if identifier == "" {
			identifier = input.Barcode
		}
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Product not found for SKU/Barcode: %s", identifier),
		})
	}

	// If scan came from QR (SKU-based), fall back barcode field to product's EAN.
	barcode := input.Barcode
	if barcode == "" {
		barcode = product.Barcode
	}

	stockTakeBarcode := models.StockTakeBarcode{
		StockTakeID:  stockTake.ID,
		DivisionCode: input.DivisionCode,
		Location:     input.Location,
		Barcode:      barcode,
		Sku:          product.ItemCode,
		LotNumber:    input.LotNumber,
		CartonNumber: input.CartonNumber,
		CountedQty:   input.Qty,
		QrRaw:        input.QrRaw,
		ItemID:       product.ID,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&stockTakeBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Internal Server Error", "error": err.Error()})
	}

	// Transisi status otomatis begitu scan pertama masuk — no-op kalau statusnya udah bukan "open"
	c.DB.Model(&stockTake).Where("status = ?", "open").Update("status", "in_progress")

	c.DB.Preload("Product").First(&stockTakeBarcode, stockTakeBarcode.ID)

	return ctx.JSON(fiber.Map{"success": true, "message": "Scan recorded successfully", "data": stockTakeBarcode})
}
func (c *StockTakeController) GetStockTakeBarcodeByCode(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.Preload("Items").First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	var stockTakeBarcodes []models.StockTakeBarcode
	if err := c.DB.Preload("Item").
		Where("stock_take_id = ?", stockTake.ID).
		Order("created_at desc").
		Find(&stockTakeBarcodes).Error; err != nil {
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

// DeleteStockTake soft-deletes a stock take session.
// Only allowed when status is still "open" (nothing counted yet).
func (c *StockTakeController) DeleteStockTake(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	if stockTake.Status != "open" && stockTake.Status != "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Only sessions with status 'open' or 'cancelled' can be deleted",
		})
	}

	if err := c.DB.Unscoped().Where("stock_take_id = ?", stockTake.ID).Delete(&models.StockTakeBarcode{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete related scan records", "error": err.Error()})
	}

	if err := c.DB.Unscoped().Delete(&stockTake).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete stock take", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Stock take deleted successfully"})
}

func (c *StockTakeController) DeleteStockTakeBarcode(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid ID",
		})
	}

	var barcode models.StockTakeBarcode
	if err := c.DB.First(&barcode, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Scan record not found",
		})
	}

	if err := c.DB.Unscoped().Delete(&barcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete scan record",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Scan record deleted successfully",
	})
}

func (c *StockTakeController) GetStockTakeBarcode(ctx *fiber.Ctx) error {
	stoCode := ctx.Params("sto")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", stoCode).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Stock take session not found"})
	}

	var data []models.StockTakeBarcode
	if err := c.DB.Preload("Product").
		Where("stock_take_id = ?", stockTake.ID).
		Order("id desc").
		Find(&data).Error; err != nil {
		return ctx.Status(500).JSON(fiber.Map{"success": false, "message": "Internal Server Error", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}

func (c *StockTakeController) CloseStockTake(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	if stockTake.Status != "in_progress" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Only sessions with status 'in_progress' can be closed",
		})
	}

	if err := c.DB.Model(&stockTake).Update("status", "closed").Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to close stock take", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Stock take closed successfully"})
}

func (c *StockTakeController) CancelStockTake(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	if stockTake.Status == "closed" || stockTake.Status == "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot cancel a session that is already '%s'", stockTake.Status),
		})
	}

	if err := c.DB.Model(&stockTake).Update("status", "cancelled").Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to cancel stock take", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Stock take cancelled successfully"})
}

// Tambahkan di StockTakeController (file controller yang sama, taruh di bawah GetProgressStockTakeByCode)

func (c *StockTakeController) GetProgressBySKU(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressBySKU(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by SKU",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}

func (c *StockTakeController) GetProgressByDivision(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressByDivisionPivot(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by division",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}

func (c *StockTakeController) GetProgressByLocation(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressByLocation(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by location",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}

func (c *StockTakeController) GetProgressByPic(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressByPic(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by PIC",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}
