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
	"github.com/xuri/excelize/v2"
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
			Where("stock_takes.status IN ?", []string{"open", "in_progress"}).
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
		// if len(inventories) == 0 {
		// 	return fiber.NewError(fiber.StatusNotFound, "No inventory data found for the selected locations")
		// }

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
		// var items []models.StockTakeItem
		// for _, inv := range inventories {
		// 	items = append(items, models.StockTakeItem{
		// 		StockTakeID:  stockTake.ID,
		// 		ItemID:       int64(inv.ItemId),
		// 		InventoryID:  int64(inv.ID),
		// 		Location:     inv.Location,
		// 		Pallet:       inv.Pallet,
		// 		Barcode:      inv.Barcode,
		// 		CartonNumber: inv.CartonNumber,
		// 		LotNumber:    inv.LotNumber,
		// 		DivisionCode: inv.DivisionCode,
		// 		OwnerCode:    inv.OwnerCode,
		// 		SystemQty:    int(inv.QtyAvailable),
		// 		CountedQty:   0,
		// 		Difference:   0,
		// 		CreatedBy:    userID,
		// 	})
		// }

		// 6. Konversi ke stock_take_items — sekarang berbasis LOCATIONS, bukan cuma inventories
		invByLocation := make(map[string][]models.Inventory)
		for _, inv := range inventories {
			invByLocation[inv.Location] = append(invByLocation[inv.Location], inv)
		}

		var items []models.StockTakeItem
		for _, loc := range locations {
			invList, hasInventory := invByLocation[loc.LocationCode]

			if hasInventory {
				for _, inv := range invList {
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
				continue
			}

			// Lokasi tanpa inventory -> placeholder item (blind count / cek lokasi kosong)
			items = append(items, models.StockTakeItem{
				StockTakeID:  stockTake.ID,
				ItemID:       0,
				InventoryID:  0,
				Location:     loc.LocationCode,
				Pallet:       "",
				Barcode:      "",
				CartonNumber: "",
				LotNumber:    "",
				DivisionCode: req.Filters.DivisionCode,
				OwnerCode:    req.Filters.OwnerCode,
				SystemQty:    0,
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

// StockTakeListItem membungkus StockTake dengan ringkasan qty untuk tampilan list —
// system qty diambil dari snapshot stock_take_items (saat generate), counted qty
// diambil dari stock_take_barcodes (hasil scan aktual di lapangan).
// type StockTakeListItem struct {
// 	models.StockTake
// 	TotalSystemQty  int `json:"total_system_qty"`
// 	TotalCountedQty int `json:"total_counted_qty"`
// }

type StockTakeListItem struct {
	models.StockTake
	TotalSystemQty  int `json:"total_system_qty"`
	TotalCountedQty int `json:"total_counted_qty"`
	PlannedLocation int `json:"planned_location"`
	CountedLocation int `json:"counted_location"`
	PlannedItem     int `json:"planned_item"`
	CountedItem     int `json:"counted_item"`
}

func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
	statuses := ctx.Query("statuses")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	searchLocation := ctx.Query("search_location")
	code := ctx.Query("code")

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Rebuild the base query fresh each time so Count() and Find() don't
	// accumulate GORM chain state on top of each other.
	buildQuery := func() *gorm.DB {
		q := c.DB.Model(&models.StockTake{})
		if statuses != "" {
			q = q.Where("status IN ?", strings.Split(statuses, ","))
		}
		if startDate != "" && endDate != "" {
			startParsed, errStart := time.Parse("2006-01-02", startDate)
			endParsed, errEnd := time.Parse("2006-01-02", endDate)
			if errStart == nil && errEnd == nil {
				endOfDay := endParsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				q = q.Where("created_at BETWEEN ? AND ?", startParsed, endOfDay)
			}
		}
		if searchLocation != "" {
			q = q.Where(
				"id IN (SELECT stock_take_id FROM stock_take_items WHERE location LIKE ? AND deleted_at IS NULL)",
				"%"+searchLocation+"%",
			)
		}
		if code != "" {
			q = q.Where("UPPER(code) LIKE UPPER(?)", "%"+code+"%")
		}
		return q
	}

	var totalCount int64
	if err := buildQuery().Count(&totalCount).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)
	meta := fiber.Map{
		"page":        page,
		"page_size":   pageSize,
		"total":       totalCount,
		"total_pages": totalPages,
	}

	var stockTakes []models.StockTake
	if err := buildQuery().
		Order("id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&stockTakes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(stockTakes) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"data":    []StockTakeListItem{},
			"meta":    meta,
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

	var systemAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeItem{}).
		Select("stock_take_id, SUM(system_qty) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
		Group("stock_take_id").
		Scan(&systemAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate system qty", "error": err.Error(),
		})
	}

	var countedAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeBarcode{}).
		Select("stock_take_id, SUM(counted_qty) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
		Group("stock_take_id").
		Scan(&countedAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate counted qty", "error": err.Error(),
		})
	}

	var plannedLocationAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeItem{}).
		Select("stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(location)))) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL AND system_qty > 0", stockTakeIDs).
		Group("stock_take_id").
		Scan(&plannedLocationAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate planned location", "error": err.Error(),
		})
	}

	var plannedItemAggs []qtyAgg
	if err := c.DB.Model(&models.StockTakeItem{}).
		Select("stock_take_id, COUNT(DISTINCT item_id) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
		Group("stock_take_id").
		Scan(&plannedItemAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate planned item", "error": err.Error(),
		})
	}

	var countedLocationAggs []qtyAgg
	if err := c.DB.Table("stock_take_items sti").
		Select("sti.stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(sti.location)))) as total").
		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
		Where(`EXISTS (
			SELECT 1 FROM stock_take_barcodes stb
			WHERE stb.stock_take_id = sti.stock_take_id
			AND UPPER(LTRIM(RTRIM(stb.location))) = UPPER(LTRIM(RTRIM(sti.location)))
			AND stb.item_id = sti.item_id
			AND stb.deleted_at IS NULL
		)`).
		Group("sti.stock_take_id").
		Scan(&countedLocationAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate counted location", "error": err.Error(),
		})
	}

	var countedItemAggs []qtyAgg
	if err := c.DB.Table("stock_take_items sti").
		Select("sti.stock_take_id, COUNT(DISTINCT sti.item_id) as total").
		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
		Where(`EXISTS (
			SELECT 1 FROM stock_take_barcodes stb
			WHERE stb.stock_take_id = sti.stock_take_id
			AND stb.item_id = sti.item_id
			AND stb.location = sti.location
			AND stb.division_code = sti.division_code
			AND stb.deleted_at IS NULL
		)`).
		Group("sti.stock_take_id").
		Scan(&countedItemAggs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to aggregate counted item", "error": err.Error(),
		})
	}

	toMap := func(aggs []qtyAgg) map[uint]int {
		m := make(map[uint]int, len(aggs))
		for _, a := range aggs {
			m[a.StockTakeID] = a.Total
		}
		return m
	}

	systemMap := toMap(systemAggs)
	countedMap := toMap(countedAggs)
	plannedLocationMap := toMap(plannedLocationAggs)
	countedLocationMap := toMap(countedLocationAggs)
	plannedItemMap := toMap(plannedItemAggs)
	countedItemMap := toMap(countedItemAggs)

	result := make([]StockTakeListItem, 0, len(stockTakes))
	for _, st := range stockTakes {
		result = append(result, StockTakeListItem{
			StockTake:       st,
			TotalSystemQty:  systemMap[st.ID],
			TotalCountedQty: countedMap[st.ID],
			PlannedLocation: plannedLocationMap[st.ID],
			CountedLocation: countedLocationMap[st.ID],
			PlannedItem:     plannedItemMap[st.ID],
			CountedItem:     countedItemMap[st.ID],
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
		"meta":    meta,
	})
}

func (c *StockTakeController) GetStockTakeStats(ctx *fiber.Ctx) error {
	statuses := ctx.Query("statuses")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	searchLocation := ctx.Query("search_location")
	code := ctx.Query("code")

	buildQuery := func() *gorm.DB {
		q := c.DB.Model(&models.StockTake{})
		if statuses != "" {
			q = q.Where("status IN ?", strings.Split(statuses, ","))
		}
		if startDate != "" && endDate != "" {
			startParsed, errStart := time.Parse("2006-01-02", startDate)
			endParsed, errEnd := time.Parse("2006-01-02", endDate)
			if errStart == nil && errEnd == nil {
				endOfDay := endParsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				q = q.Where("created_at BETWEEN ? AND ?", startParsed, endOfDay)
			}
		}
		if searchLocation != "" {
			q = q.Where(
				"id IN (SELECT stock_take_id FROM stock_take_items WHERE location LIKE ? AND deleted_at IS NULL)",
				"%"+searchLocation+"%",
			)
		}
		if code != "" {
			q = q.Where("UPPER(code) LIKE UPPER(?)", "%"+code+"%")
		}
		return q
	}

	var stockTakeIDs []uint
	if err := buildQuery().Pluck("id", &stockTakeIDs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	stats := fiber.Map{
		"total_sessions":         len(stockTakeIDs),
		"total_system_qty":       0,
		"total_counted_qty":      0,
		"total_planned_location": 0,
		"total_counted_location": 0,
		"total_planned_item":     0,
		"total_counted_item":     0,
	}

	if len(stockTakeIDs) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": stats})
	}

	type sumResult struct{ Total int }
	sumOf := func(q *gorm.DB) int {
		var r sumResult
		q.Scan(&r)
		return r.Total
	}

	stats["total_system_qty"] = sumOf(c.DB.Model(&models.StockTakeItem{}).
		Select("ISNULL(SUM(system_qty), 0) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs))

	stats["total_counted_qty"] = sumOf(c.DB.Model(&models.StockTakeBarcode{}).
		Select("ISNULL(SUM(counted_qty), 0) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs))

	stats["total_planned_location"] = sumOf(c.DB.Model(&models.StockTakeItem{}).
		Select("COUNT(DISTINCT CONCAT(stock_take_id, '|', UPPER(LTRIM(RTRIM(location))))) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL AND system_qty > 0", stockTakeIDs))

	stats["total_planned_item"] = sumOf(c.DB.Model(&models.StockTakeItem{}).
		Select("COUNT(DISTINCT CONCAT(stock_take_id, '|', item_id)) as total").
		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs))

	stats["total_counted_location"] = sumOf(c.DB.Table("stock_take_items sti").
		Select("COUNT(DISTINCT CONCAT(sti.stock_take_id, '|', UPPER(LTRIM(RTRIM(sti.location))))) as total").
		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
		Where(`EXISTS (
			SELECT 1 FROM stock_take_barcodes stb
			WHERE stb.stock_take_id = sti.stock_take_id
			AND UPPER(LTRIM(RTRIM(stb.location))) = UPPER(LTRIM(RTRIM(sti.location)))
			AND stb.item_id = sti.item_id
			AND stb.deleted_at IS NULL
		)`))

	stats["total_counted_item"] = sumOf(c.DB.Table("stock_take_items sti").
		Select("COUNT(DISTINCT CONCAT(sti.stock_take_id, '|', sti.item_id)) as total").
		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
		Where(`EXISTS (
			SELECT 1 FROM stock_take_barcodes stb
			WHERE stb.stock_take_id = sti.stock_take_id
			AND stb.item_id = sti.item_id
			AND stb.location = sti.location
			AND stb.division_code = sti.division_code
			AND stb.deleted_at IS NULL
		)`))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": stats})
}

// func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
// 	statuses := ctx.Query("statuses")
// 	startDate := ctx.Query("start_date")
// 	endDate := ctx.Query("end_date")
// 	searchLocation := ctx.Query("search_location")

// 	query := c.DB.Model(&models.StockTake{}).Order("id desc")

// 	if statuses != "" {
// 		statusList := strings.Split(statuses, ",")
// 		query = query.Where("status IN ?", statusList)
// 	}

// 	if startDate != "" && endDate != "" {
// 		startParsed, errStart := time.Parse("2006-01-02", startDate)
// 		endParsed, errEnd := time.Parse("2006-01-02", endDate)
// 		if errStart == nil && errEnd == nil {
// 			endOfDay := endParsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
// 			query = query.Where("created_at BETWEEN ? AND ?", startParsed, endOfDay)
// 		}
// 	}

// 	if searchLocation != "" {
// 		query = query.Where(
// 			"id IN (SELECT stock_take_id FROM stock_take_items WHERE location LIKE ? AND deleted_at IS NULL)",
// 			"%"+searchLocation+"%",
// 		)
// 	}

// 	var stockTakes []models.StockTake
// 	if err := query.Find(&stockTakes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   err.Error(),
// 		})
// 	}

// 	if len(stockTakes) == 0 {
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 			"success": true,
// 			"data":    []StockTakeListItem{},
// 		})
// 	}

// 	var stockTakeIDs []uint
// 	for _, st := range stockTakes {
// 		stockTakeIDs = append(stockTakeIDs, st.ID)
// 	}

// 	type qtyAgg struct {
// 		StockTakeID uint
// 		Total       int
// 	}

// 	// ── Total qty sistem (snapshot pas generate stock take) ──
// 	var systemAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeItem{}).
// 		Select("stock_take_id, SUM(system_qty) as total").
// 		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&systemAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate system qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Total qty yang sudah dihitung (hasil scan aktual) ──
// 	var countedAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeBarcode{}).
// 		Select("stock_take_id, SUM(counted_qty) as total").
// 		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&countedAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate counted qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Planned Location: distinct location (normalized) dari StockTakeItem, per sesi ──
// 	var plannedLocationAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeItem{}).
// 		Select("stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(location)))) as total").
// 		Where("stock_take_id IN ? AND deleted_at IS NULL AND system_qty > 0", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&plannedLocationAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate planned location",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Planned Item: distinct item_id dari StockTakeItem, per sesi ──
// 	var plannedItemAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeItem{}).
// 		Select("stock_take_id, COUNT(DISTINCT item_id) as total").
// 		Where("stock_take_id IN ? AND deleted_at IS NULL", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&plannedItemAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate planned item",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Counted Location: distinct location (normalized) dari planned yang match ke scan ──
// 	var countedLocationAggs []qtyAgg
// 	if err := c.DB.Table("stock_take_items sti").
// 		Select("sti.stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(sti.location)))) as total").
// 		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
// 		Where(`EXISTS (
// 		SELECT 1 FROM stock_take_barcodes stb
// 		WHERE stb.stock_take_id = sti.stock_take_id
// 		AND UPPER(LTRIM(RTRIM(stb.location))) = UPPER(LTRIM(RTRIM(sti.location)))
// 		AND stb.item_id = sti.item_id
// 		AND stb.deleted_at IS NULL
// 	)`).
// 		Group("sti.stock_take_id").
// 		Scan(&countedLocationAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate counted location",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Counted Item: distinct item_id dari planned yang match composite key (item_id + location + division_code) ──
// 	var countedItemAggs []qtyAgg
// 	if err := c.DB.Table("stock_take_items sti").
// 		Select("sti.stock_take_id, COUNT(DISTINCT sti.item_id) as total").
// 		Where("sti.stock_take_id IN ? AND sti.deleted_at IS NULL", stockTakeIDs).
// 		Where(`EXISTS (
// 			SELECT 1 FROM stock_take_barcodes stb
// 			WHERE stb.stock_take_id = sti.stock_take_id
// 			AND stb.item_id = sti.item_id
// 			AND stb.location = sti.location
// 			AND stb.division_code = sti.division_code
// 			AND stb.deleted_at IS NULL
// 		)`).
// 		Group("sti.stock_take_id").
// 		Scan(&countedItemAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate counted item",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// ── Build lookup maps ──
// 	toMap := func(aggs []qtyAgg) map[uint]int {
// 		m := make(map[uint]int, len(aggs))
// 		for _, a := range aggs {
// 			m[a.StockTakeID] = a.Total
// 		}
// 		return m
// 	}

// 	systemMap := toMap(systemAggs)
// 	countedMap := toMap(countedAggs)
// 	plannedLocationMap := toMap(plannedLocationAggs)
// 	countedLocationMap := toMap(countedLocationAggs)
// 	plannedItemMap := toMap(plannedItemAggs)
// 	countedItemMap := toMap(countedItemAggs)

// 	result := make([]StockTakeListItem, 0, len(stockTakes))
// 	for _, st := range stockTakes {
// 		result = append(result, StockTakeListItem{
// 			StockTake:       st,
// 			TotalSystemQty:  systemMap[st.ID],
// 			TotalCountedQty: countedMap[st.ID],
// 			PlannedLocation: plannedLocationMap[st.ID],
// 			CountedLocation: countedLocationMap[st.ID],
// 			PlannedItem:     plannedItemMap[st.ID],
// 			CountedItem:     countedItemMap[st.ID],
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    result,
// 	})
// }

func (c *StockTakeController) GetAllStockTakeSummary(ctx *fiber.Ctx) error {
	code := strings.TrimSpace(ctx.Query("code"))
	if code == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "code is required",
		})
	}
	if len(code) < 3 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "code must be at least 3 characters",
		})
	}

	var result []StockTakeListItem

	excludedStatuses := []string{"cancelled", "closed"}
	codeLike := "%" + code + "%"

	err := c.DB.Raw(`
		WITH valid_st AS (
			SELECT id FROM stock_takes
			WHERE status NOT IN (?) AND deleted_at IS NULL
			AND UPPER(code) LIKE UPPER(?)
		),
		system_agg AS (
			SELECT sti.stock_take_id, SUM(sti.system_qty) AS total
			FROM stock_take_items sti
			INNER JOIN valid_st v ON v.id = sti.stock_take_id
			WHERE sti.deleted_at IS NULL
			GROUP BY sti.stock_take_id
		),
		counted_agg AS (
			SELECT stb.stock_take_id, SUM(stb.counted_qty) AS total
			FROM stock_take_barcodes stb
			INNER JOIN valid_st v ON v.id = stb.stock_take_id
			WHERE stb.deleted_at IS NULL
			GROUP BY stb.stock_take_id
		),
		planned_location_agg AS (
			SELECT sti.stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(sti.location)))) AS total
			FROM stock_take_items sti
			INNER JOIN valid_st v ON v.id = sti.stock_take_id
			WHERE sti.deleted_at IS NULL AND sti.system_qty > 0
			GROUP BY sti.stock_take_id
		),
		planned_item_agg AS (
			SELECT sti.stock_take_id, COUNT(DISTINCT sti.item_id) AS total
			FROM stock_take_items sti
			INNER JOIN valid_st v ON v.id = sti.stock_take_id
			WHERE sti.deleted_at IS NULL
			GROUP BY sti.stock_take_id
		),
		counted_location_agg AS (
			SELECT sti.stock_take_id, COUNT(DISTINCT UPPER(LTRIM(RTRIM(sti.location)))) AS total
			FROM stock_take_items sti
			INNER JOIN valid_st v ON v.id = sti.stock_take_id
			WHERE sti.deleted_at IS NULL
			AND EXISTS (
				SELECT 1 FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = sti.stock_take_id
				AND UPPER(LTRIM(RTRIM(stb.location))) = UPPER(LTRIM(RTRIM(sti.location)))
				AND stb.item_id = sti.item_id
				AND stb.deleted_at IS NULL
			)
			GROUP BY sti.stock_take_id
		),
		counted_item_agg AS (
			SELECT sti.stock_take_id, COUNT(DISTINCT sti.item_id) AS total
			FROM stock_take_items sti
			INNER JOIN valid_st v ON v.id = sti.stock_take_id
			WHERE sti.deleted_at IS NULL
			AND EXISTS (
				SELECT 1 FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = sti.stock_take_id
				AND stb.item_id = sti.item_id
				AND stb.location = sti.location
				AND stb.division_code = sti.division_code
				AND stb.deleted_at IS NULL
			)
			GROUP BY sti.stock_take_id
		)
		SELECT
			st.*,
			ISNULL(sa.total, 0)  AS total_system_qty,
			ISNULL(ca.total, 0)  AS total_counted_qty,
			ISNULL(pla.total, 0) AS planned_location,
			ISNULL(cla.total, 0) AS counted_location,
			ISNULL(pia.total, 0) AS planned_item,
			ISNULL(cia.total, 0) AS counted_item
		FROM stock_takes st
		LEFT JOIN system_agg sa ON sa.stock_take_id = st.id
		LEFT JOIN counted_agg ca ON ca.stock_take_id = st.id
		LEFT JOIN planned_location_agg pla ON pla.stock_take_id = st.id
		LEFT JOIN counted_location_agg cla ON cla.stock_take_id = st.id
		LEFT JOIN planned_item_agg pia ON pia.stock_take_id = st.id
		LEFT JOIN counted_item_agg cia ON cia.stock_take_id = st.id
		WHERE st.status NOT IN (?) AND st.deleted_at IS NULL
		AND UPPER(st.code) LIKE UPPER(?)
		ORDER BY st.id DESC
	`, excludedStatuses, codeLike, excludedStatuses, codeLike).Scan(&result).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
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

// 	if len(stockTakes) == 0 {
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 			"success": true,
// 			"data":    []StockTakeListItem{},
// 		})
// 	}

// 	var stockTakeIDs []uint
// 	for _, st := range stockTakes {
// 		stockTakeIDs = append(stockTakeIDs, st.ID)
// 	}

// 	type qtyAgg struct {
// 		StockTakeID uint
// 		Total       int
// 	}

// 	// Total qty sistem (snapshot pas generate stock take)
// 	var systemAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeItem{}).
// 		Select("stock_take_id, SUM(system_qty) as total").
// 		Where("stock_take_id IN ?", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&systemAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate system qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Total qty yang sudah dihitung (hasil scan aktual)
// 	var countedAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeBarcode{}).
// 		Select("stock_take_id, SUM(counted_qty) as total").
// 		Where("stock_take_id IN ?", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&countedAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate counted qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	systemMap := make(map[uint]int, len(systemAggs))
// 	for _, a := range systemAggs {
// 		systemMap[a.StockTakeID] = a.Total
// 	}
// 	countedMap := make(map[uint]int, len(countedAggs))
// 	for _, a := range countedAggs {
// 		countedMap[a.StockTakeID] = a.Total
// 	}

// 	result := make([]StockTakeListItem, 0, len(stockTakes))
// 	for _, st := range stockTakes {
// 		result = append(result, StockTakeListItem{
// 			StockTake:       st,
// 			TotalSystemQty:  systemMap[st.ID],
// 			TotalCountedQty: countedMap[st.ID],
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    result,
// 	})
// }

// func (c *StockTakeController) GetAllStockTake(ctx *fiber.Ctx) error {
// 	statuses := ctx.Query("statuses")
// 	startDate := ctx.Query("start_date")
// 	endDate := ctx.Query("end_date")
// 	searchLocation := ctx.Query("search_location")

// 	query := c.DB.Model(&models.StockTake{}).Order("id desc")

// 	if statuses != "" {
// 		statusList := strings.Split(statuses, ",")
// 		query = query.Where("status IN ?", statusList)
// 	}

// 	if startDate != "" && endDate != "" {
// 		startParsed, errStart := time.Parse("2006-01-02", startDate)
// 		endParsed, errEnd := time.Parse("2006-01-02", endDate)
// 		if errStart == nil && errEnd == nil {
// 			endOfDay := endParsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
// 			query = query.Where("created_at BETWEEN ? AND ?", startParsed, endOfDay)
// 		}
// 	}

// 	if searchLocation != "" {
// 		query = query.Where(
// 			"id IN (SELECT stock_take_id FROM stock_take_items WHERE location LIKE ? AND deleted_at IS NULL)",
// 			"%"+searchLocation+"%",
// 		)
// 	}

// 	var stockTakes []models.StockTake
// 	if err := query.Find(&stockTakes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   err.Error(),
// 		})
// 	}

// 	if len(stockTakes) == 0 {
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 			"success": true,
// 			"data":    []StockTakeListItem{},
// 		})
// 	}

// 	var stockTakeIDs []uint
// 	for _, st := range stockTakes {
// 		stockTakeIDs = append(stockTakeIDs, st.ID)
// 	}

// 	type qtyAgg struct {
// 		StockTakeID uint
// 		Total       int
// 	}

// 	// Total qty sistem (snapshot pas generate stock take)
// 	var systemAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeItem{}).
// 		Select("stock_take_id, SUM(system_qty) as total").
// 		Where("stock_take_id IN ?", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&systemAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate system qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	// Total qty yang sudah dihitung (hasil scan aktual)
// 	var countedAggs []qtyAgg
// 	if err := c.DB.Model(&models.StockTakeBarcode{}).
// 		Select("stock_take_id, SUM(counted_qty) as total").
// 		Where("stock_take_id IN ?", stockTakeIDs).
// 		Group("stock_take_id").
// 		Scan(&countedAggs).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"message": "Failed to aggregate counted qty",
// 			"error":   err.Error(),
// 		})
// 	}

// 	systemMap := make(map[uint]int, len(systemAggs))
// 	for _, a := range systemAggs {
// 		systemMap[a.StockTakeID] = a.Total
// 	}
// 	countedMap := make(map[uint]int, len(countedAggs))
// 	for _, a := range countedAggs {
// 		countedMap[a.StockTakeID] = a.Total
// 	}

// 	result := make([]StockTakeListItem, 0, len(stockTakes))
// 	for _, st := range stockTakes {
// 		result = append(result, StockTakeListItem{
// 			StockTake:       st,
// 			TotalSystemQty:  systemMap[st.ID],
// 			TotalCountedQty: countedMap[st.ID],
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data":    result,
// 	})
// }

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
	Division   string `json:"division"`
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
			ISNULL(sti.division_code, '') AS [division],
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
		GROUP BY sti.location, p.item_code, p.item_name, sti.division_code
		ORDER BY sti.location ASC
	`

	var rows []StockTakePrintRow
	if err := c.DB.Debug().Raw(query, args...).Scan(&rows).Error; err != nil {
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

	if input.Qty < 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Quantity must be a positive number"})
	}

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", input.StockTakeCode).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Stock take session not found"})
	}

	if stockTake.Status == "closed" || stockTake.Status == "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot scan — session is already '" + stockTake.Status + "'"})
	}

	if input.CartonNumber != "" && input.Location != "" && stockTake.Status == "in_progress" && input.Sku != "" {
		var existing models.StockTakeBarcode
		err := c.DB.Debug().Where("stock_take_id = ? AND location = ? AND carton_number = ? AND sku = ?",
			stockTake.ID, input.Location, input.CartonNumber, input.Sku).First(&existing).Error

		switch {
		case err == nil:
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Carton number " + input.CartonNumber + " for SKU " + input.Sku + " already scanned in this location"})
		case errors.Is(err, gorm.ErrRecordNotFound):
			// aman, lanjut proses
		default:
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
		}
	}

	// Sesi yang sudah closed/cancelled tidak boleh menerima scan baru
	if stockTake.Status == "closed" || stockTake.Status == "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot scan — session is already '%s'", stockTake.Status),
		})
	}

	// Validate location is matching with stock take item location
	var stockTakeItem models.StockTakeItem
	if err := c.DB.Where("stock_take_id = ? AND location = ?", stockTake.ID, input.Location).First(&stockTakeItem).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Location : " + input.Location + " does not match with session id : " + stockTake.Code})
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
	// c.DB.Model(&stockTake).Where("status = ?", "open").Update("status", "in_progress")

	// Transisi status otomatis begitu scan pertama masuk — no-op kalau statusnya udah bukan "open"
	c.DB.Model(&stockTake).Where("status = ?", "open").Updates(map[string]interface{}{
		"status":     "in_progress",
		"started_at": time.Now(),
		"updated_at": time.Now(),
		"updated_by": int(ctx.Locals("userID").(float64)),
	})

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

	if err := c.DB.Where("stock_take_id = ?", stockTake.ID).Delete(&models.StockTakeBarcode{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete related scan records", "error": err.Error()})
	}

	if err := c.DB.Delete(&stockTake).Error; err != nil {
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

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, barcode.StockTakeID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Stock take session not found",
		})
	}

	if stockTake.Status == "closed" || stockTake.Status == "cancelled" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot delete scan record — session is already '%s'", stockTake.Status),
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

	// if err := c.DB.Model(&stockTake).Update("status", "closed").Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to close stock take", "error": err.Error()})
	// }

	userID := int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&stockTake).Updates(map[string]interface{}{
		"status":     "closed",
		"closed_at":  time.Now(),
		"closed_by":  userID,
		"updated_at": time.Now(),
		"updated_by": userID,
	}).Error; err != nil {
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

	// if err := c.DB.Model(&stockTake).Update("status", "cancelled").Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to cancel stock take", "error": err.Error()})
	// }

	userID := int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&stockTake).Updates(map[string]interface{}{
		"status":     "cancelled",
		"cancel_at":  time.Now(),
		"cancel_by":  userID,
		"updated_at": time.Now(),
		"updated_by": userID,
	}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to close stock take", "error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Stock take cancelled successfully"})
}

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

func (c *StockTakeController) ExportProgressByDivision(ctx *fiber.Ctx) error {
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

	f, err := buildProgressByDivisionExcel(data, code)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to build excel",
			"error":   err.Error(),
		})
	}

	filename := fmt.Sprintf("Report_Daily_Progress_STO_%s.xlsx", code)
	ctx.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))

	return f.Write(ctx.Response().BodyWriter())
}

func buildProgressByDivisionExcel(data *repositories.ProgressByDivisionResult, code string) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Progress"
	f.SetSheetName("Sheet1", sheet)

	// ── Styles ──────────────────────────────────────────────────────────
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Italic: true, Size: 14},
	})
	dateHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	invStockStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7030A0"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	catHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"C00000"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	totalHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Border: borderAll(),
		Font:   &excelize.Font{Bold: true},
	})
	totalRowStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FCE4D6"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	achievementStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"DDEBF7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	totalPctStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"BDD7EE"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// ── Title ───────────────────────────────────────────────────────────
	f.SetCellValue(sheet, "A1", "Report Daily Progress STO")
	f.SetCellStyle(sheet, "A1", "A1", titleStyle)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Code: %s | Generated: %s", code, time.Now().Format("02-Jan-2006")))

	headerRow1 := 4
	headerRow2 := 5
	bodyStartRow := 6

	// Kolom: A=Category, B-C=Inventory Stock (Loc,Qty), lalu tiap tanggal 2 kolom, lalu Total 2 kolom
	col := 2 // mulai dari B (index 2, 1-based)

	f.SetCellValue(sheet, cellRef(1, headerRow1), "Category")
	f.MergeCell(sheet, cellRef(1, headerRow1), cellRef(1, headerRow2))
	f.SetCellStyle(sheet, cellRef(1, headerRow1), cellRef(1, headerRow2), labelStyle)

	// Inventory Stock group
	f.SetCellValue(sheet, cellRef(col, headerRow1), "Inventory Stock")
	f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
	f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), invStockStyle)
	f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
	f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Sum Qty")
	f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)
	col += 2

	dateColStart := make(map[string]int)
	for _, date := range data.Dates {
		t, _ := time.Parse("2006-01-02", date)
		label := t.Format("02-Jan-06")

		f.SetCellValue(sheet, cellRef(col, headerRow1), label)
		f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
		f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), dateHeaderStyle)
		f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
		f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Count Qty")
		f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)

		dateColStart[date] = col
		col += 2
	}

	// Total group
	totalColStart := col
	f.SetCellValue(sheet, cellRef(col, headerRow1), "Total")
	f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
	f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), totalHeaderStyle)
	f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
	f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Count Qty")
	f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)

	// ── Body: 1 baris per division ──────────────────────────────────────
	row := bodyStartRow
	for _, div := range data.Divisions {
		f.SetCellValue(sheet, cellRef(1, row), div.DivisionCode)
		f.SetCellStyle(sheet, cellRef(1, row), cellRef(1, row), labelStyle)

		f.SetCellValue(sheet, cellRef(2, row), div.SystemLocation)
		f.SetCellValue(sheet, cellRef(3, row), div.SystemQty)
		f.SetCellStyle(sheet, cellRef(2, row), cellRef(3, row), cellStyle)

		for _, date := range data.Dates {
			c := dateColStart[date]
			day, ok := div.Daily[date]
			if ok {
				if day.LocationCounted != nil {
					f.SetCellValue(sheet, cellRef(c, row), *day.LocationCounted)
				} else {
					f.SetCellValue(sheet, cellRef(c, row), "-")
				}
				if day.QtyCounted != nil {
					f.SetCellValue(sheet, cellRef(c+1, row), *day.QtyCounted)
				} else {
					f.SetCellValue(sheet, cellRef(c+1, row), "-")
				}
			} else {
				f.SetCellValue(sheet, cellRef(c, row), "-")
				f.SetCellValue(sheet, cellRef(c+1, row), "-")
			}
			f.SetCellStyle(sheet, cellRef(c, row), cellRef(c+1, row), cellStyle)
		}

		f.SetCellValue(sheet, cellRef(totalColStart, row), div.TotalLocationCounted)
		f.SetCellValue(sheet, cellRef(totalColStart+1, row), div.TotalQtyCounted)
		f.SetCellStyle(sheet, cellRef(totalColStart, row), cellRef(totalColStart+1, row), cellStyle)

		row++
	}

	// ── Row: Total (gabungan semua division) ────────────────────────────
	totalRow := row
	f.SetCellValue(sheet, cellRef(1, totalRow), "Total")
	f.SetCellValue(sheet, cellRef(2, totalRow), data.GrandTotal.SystemLocation)
	f.SetCellValue(sheet, cellRef(3, totalRow), data.GrandTotal.SystemQty)
	for _, date := range data.Dates {
		c := dateColStart[date]
		day := data.GrandTotal.Daily[date]
		if day.LocationCounted != nil {
			f.SetCellValue(sheet, cellRef(c, totalRow), *day.LocationCounted)
		}
		if day.QtyCounted != nil {
			f.SetCellValue(sheet, cellRef(c+1, totalRow), *day.QtyCounted)
		}
	}
	f.SetCellValue(sheet, cellRef(totalColStart, totalRow), data.GrandTotal.TotalLocationCounted)
	f.SetCellValue(sheet, cellRef(totalColStart+1, totalRow), data.GrandTotal.TotalQtyCounted)
	f.SetCellStyle(sheet, cellRef(1, totalRow), cellRef(totalColStart+1, totalRow), totalRowStyle)
	row++

	// ── Rows: Achievement per division (%) ───────────────────────────────
	for _, div := range data.Divisions {
		f.SetCellValue(sheet, cellRef(1, row), fmt.Sprintf("Achievement %s", div.DivisionCode))
		for _, date := range data.Dates {
			c := dateColStart[date]
			day, ok := div.Daily[date]
			if ok {
				if day.LocationPercent != nil {
					f.SetCellValue(sheet, cellRef(c, row), fmt.Sprintf("%.2f%%", *day.LocationPercent))
				} else {
					f.SetCellValue(sheet, cellRef(c, row), "-")
				}
				if day.QtyPercent != nil {
					f.SetCellValue(sheet, cellRef(c+1, row), fmt.Sprintf("%.2f%%", *day.QtyPercent))
				} else {
					f.SetCellValue(sheet, cellRef(c+1, row), "-")
				}
			} else {
				f.SetCellValue(sheet, cellRef(c, row), "-")
				f.SetCellValue(sheet, cellRef(c+1, row), "-")
			}
		}
		f.SetCellValue(sheet, cellRef(totalColStart, row), fmt.Sprintf("%.2f%%", div.TotalLocationPercent))
		f.SetCellValue(sheet, cellRef(totalColStart+1, row), fmt.Sprintf("%.2f%%", div.TotalQtyPercent))
		f.SetCellStyle(sheet, cellRef(1, row), cellRef(totalColStart+1, row), achievementStyle)
		row++
	}

	// ── Row: Total % Counting ────────────────────────────────────────────
	f.SetCellValue(sheet, cellRef(1, row), "Total % Counting")
	for _, date := range data.Dates {
		c := dateColStart[date]
		day := data.GrandTotal.Daily[date]
		if day.LocationPercent != nil {
			f.SetCellValue(sheet, cellRef(c, row), fmt.Sprintf("%.2f%%", *day.LocationPercent))
		} else {
			f.SetCellValue(sheet, cellRef(c, row), "-")
		}
		if day.QtyPercent != nil {
			f.SetCellValue(sheet, cellRef(c+1, row), fmt.Sprintf("%.2f%%", *day.QtyPercent))
		} else {
			f.SetCellValue(sheet, cellRef(c+1, row), "-")
		}
	}
	f.SetCellValue(sheet, cellRef(totalColStart, row), fmt.Sprintf("%.2f%%", data.GrandTotal.TotalLocationPercent))
	f.SetCellValue(sheet, cellRef(totalColStart+1, row), fmt.Sprintf("%.2f%%", data.GrandTotal.TotalQtyPercent))
	f.SetCellStyle(sheet, cellRef(1, row), cellRef(totalColStart+1, row), totalPctStyle)

	// Column width biar rapi
	f.SetColWidth(sheet, "A", "A", 18)
	lastCol, _ := excelize.ColumnNumberToName(totalColStart + 1)
	f.SetColWidth(sheet, "B", lastCol, 12)

	f.SetActiveSheet(0)
	return f, nil
}

func borderAll() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
}

func cellRef(col, row int) string {
	name, _ := excelize.ColumnNumberToName(col)
	return fmt.Sprintf("%s%d", name, row)
}

// ─── 3. CONTROLLER METHOD ───────────────────────────────────────────────

func (c *StockTakeController) GetProgressByCategory(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressByCategoryPivot(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by category",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "data": data})
}

// ─── 4. ROUTE ────────────────────────────────────────────────────────────
// Tambahin di file routes, deket-deket route progress-division:
//
//   stockTake.Get("/progress-category/:code", stockTakeController.GetProgressByCategory)
//
// TODO: Export Excel untuk By Category ("/stock-take/export-category/:code")
// BELUM dibuatkan di sini karena saya belum lihat source handler
// export-division-nya (excelize). Share kode itu kalau mau saya bikinin
// versi category-nya juga — tombol Download di frontend sudah saya pasang
// dan siap dihubungkan begitu endpoint-nya ada.

// ============================================================================
// TAMBAHAN: Export Excel untuk "By Category"
// Taruh di file yang sama dengan ExportProgressByDivision / buildProgressByDivisionExcel.
// Reuse helper borderAll() dan cellRef() yang udah ada di file itu — JANGAN
// didefinisikan ulang di sini (bakal duplicate function error).
// ============================================================================

func (c *StockTakeController) ExportProgressByCategory(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var stockTake models.StockTake
	if err := c.DB.First(&stockTake, "code = ?", code).Error; err != nil {
		return ctx.Status(404).JSON(fiber.Map{"success": false, "message": "Not found"})
	}

	repo := repositories.NewStockTakeRepository(c.DB)
	data, err := repo.GetProgressByCategoryPivot(stockTake.ID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch progress by category",
			"error":   err.Error(),
		})
	}

	f, err := buildProgressByCategoryExcel(data, code)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to build excel",
			"error":   err.Error(),
		})
	}

	filename := fmt.Sprintf("Report_Daily_Progress_Category_STO_%s.xlsx", code)
	ctx.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))

	return f.Write(ctx.Response().BodyWriter())
}

func buildProgressByCategoryExcel(data *repositories.ProgressByCategoryResult, code string) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Progress"
	f.SetSheetName("Sheet1", sheet)

	// ── Styles ──────────────────────────────────────────────────────────
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Italic: true, Size: 14},
	})
	dateHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	invStockStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7030A0"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	catHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"C00000"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	totalHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll(),
	})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Border: borderAll(),
		Font:   &excelize.Font{Bold: true},
	})
	totalRowStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FCE4D6"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	achievementStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"DDEBF7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	totalPctStyle, _ := f.NewStyle(&excelize.Style{
		Border:    borderAll(),
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"BDD7EE"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// ── Title ───────────────────────────────────────────────────────────
	f.SetCellValue(sheet, "A1", "Report Daily Progress STO - By Category")
	f.SetCellStyle(sheet, "A1", "A1", titleStyle)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Code: %s | Generated: %s", code, time.Now().Format("02-Jan-2006")))

	headerRow1 := 4
	headerRow2 := 5
	bodyStartRow := 6

	// Kolom: A=Category, B-C=Inventory Stock (Loc,Qty), lalu tiap tanggal 2 kolom, lalu Total 2 kolom
	col := 2 // mulai dari B (index 2, 1-based)

	f.SetCellValue(sheet, cellRef(1, headerRow1), "Category")
	f.MergeCell(sheet, cellRef(1, headerRow1), cellRef(1, headerRow2))
	f.SetCellStyle(sheet, cellRef(1, headerRow1), cellRef(1, headerRow2), labelStyle)

	// Inventory Stock group
	f.SetCellValue(sheet, cellRef(col, headerRow1), "Inventory Stock")
	f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
	f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), invStockStyle)
	f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
	f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Sum Qty")
	f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)
	col += 2

	dateColStart := make(map[string]int)
	for _, date := range data.Dates {
		t, _ := time.Parse("2006-01-02", date)
		label := t.Format("02-Jan-06")

		f.SetCellValue(sheet, cellRef(col, headerRow1), label)
		f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
		f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), dateHeaderStyle)
		f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
		f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Count Qty")
		f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)

		dateColStart[date] = col
		col += 2
	}

	// Total group
	totalColStart := col
	f.SetCellValue(sheet, cellRef(col, headerRow1), "Total")
	f.MergeCell(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1))
	f.SetCellStyle(sheet, cellRef(col, headerRow1), cellRef(col+1, headerRow1), totalHeaderStyle)
	f.SetCellValue(sheet, cellRef(col, headerRow2), "Count Location")
	f.SetCellValue(sheet, cellRef(col+1, headerRow2), "Count Qty")
	f.SetCellStyle(sheet, cellRef(col, headerRow2), cellRef(col+1, headerRow2), catHeaderStyle)

	// ── Body: 1 baris per category ───────────────────────────────────────
	row := bodyStartRow
	for _, cat := range data.Categories {
		f.SetCellValue(sheet, cellRef(1, row), cat.CategoryCode)
		f.SetCellStyle(sheet, cellRef(1, row), cellRef(1, row), labelStyle)

		f.SetCellValue(sheet, cellRef(2, row), cat.SystemLocation)
		f.SetCellValue(sheet, cellRef(3, row), cat.SystemQty)
		f.SetCellStyle(sheet, cellRef(2, row), cellRef(3, row), cellStyle)

		for _, date := range data.Dates {
			c := dateColStart[date]
			day, ok := cat.Daily[date]
			if ok {
				if day.LocationCounted != nil {
					f.SetCellValue(sheet, cellRef(c, row), *day.LocationCounted)
				} else {
					f.SetCellValue(sheet, cellRef(c, row), "-")
				}
				if day.QtyCounted != nil {
					f.SetCellValue(sheet, cellRef(c+1, row), *day.QtyCounted)
				} else {
					f.SetCellValue(sheet, cellRef(c+1, row), "-")
				}
			} else {
				f.SetCellValue(sheet, cellRef(c, row), "-")
				f.SetCellValue(sheet, cellRef(c+1, row), "-")
			}
			f.SetCellStyle(sheet, cellRef(c, row), cellRef(c+1, row), cellStyle)
		}

		f.SetCellValue(sheet, cellRef(totalColStart, row), cat.TotalLocationCounted)
		f.SetCellValue(sheet, cellRef(totalColStart+1, row), cat.TotalQtyCounted)
		f.SetCellStyle(sheet, cellRef(totalColStart, row), cellRef(totalColStart+1, row), cellStyle)

		row++
	}

	// ── Row: Total (gabungan semua category) ─────────────────────────────
	totalRow := row
	f.SetCellValue(sheet, cellRef(1, totalRow), "Total")
	f.SetCellValue(sheet, cellRef(2, totalRow), data.GrandTotal.SystemLocation)
	f.SetCellValue(sheet, cellRef(3, totalRow), data.GrandTotal.SystemQty)
	for _, date := range data.Dates {
		c := dateColStart[date]
		day := data.GrandTotal.Daily[date]
		if day.LocationCounted != nil {
			f.SetCellValue(sheet, cellRef(c, totalRow), *day.LocationCounted)
		}
		if day.QtyCounted != nil {
			f.SetCellValue(sheet, cellRef(c+1, totalRow), *day.QtyCounted)
		}
	}
	f.SetCellValue(sheet, cellRef(totalColStart, totalRow), data.GrandTotal.TotalLocationCounted)
	f.SetCellValue(sheet, cellRef(totalColStart+1, totalRow), data.GrandTotal.TotalQtyCounted)
	f.SetCellStyle(sheet, cellRef(1, totalRow), cellRef(totalColStart+1, totalRow), totalRowStyle)
	row++

	// ── Rows: Achievement per category (%) ────────────────────────────────
	for _, cat := range data.Categories {
		f.SetCellValue(sheet, cellRef(1, row), fmt.Sprintf("Achievement %s", cat.CategoryCode))
		for _, date := range data.Dates {
			c := dateColStart[date]
			day, ok := cat.Daily[date]
			if ok {
				if day.LocationPercent != nil {
					f.SetCellValue(sheet, cellRef(c, row), fmt.Sprintf("%.2f%%", *day.LocationPercent))
				} else {
					f.SetCellValue(sheet, cellRef(c, row), "-")
				}
				if day.QtyPercent != nil {
					f.SetCellValue(sheet, cellRef(c+1, row), fmt.Sprintf("%.2f%%", *day.QtyPercent))
				} else {
					f.SetCellValue(sheet, cellRef(c+1, row), "-")
				}
			} else {
				f.SetCellValue(sheet, cellRef(c, row), "-")
				f.SetCellValue(sheet, cellRef(c+1, row), "-")
			}
		}
		f.SetCellValue(sheet, cellRef(totalColStart, row), fmt.Sprintf("%.2f%%", cat.TotalLocationPercent))
		f.SetCellValue(sheet, cellRef(totalColStart+1, row), fmt.Sprintf("%.2f%%", cat.TotalQtyPercent))
		f.SetCellStyle(sheet, cellRef(1, row), cellRef(totalColStart+1, row), achievementStyle)
		row++
	}

	// ── Row: Total % Counting ────────────────────────────────────────────
	f.SetCellValue(sheet, cellRef(1, row), "Total % Counting")
	for _, date := range data.Dates {
		c := dateColStart[date]
		day := data.GrandTotal.Daily[date]
		if day.LocationPercent != nil {
			f.SetCellValue(sheet, cellRef(c, row), fmt.Sprintf("%.2f%%", *day.LocationPercent))
		} else {
			f.SetCellValue(sheet, cellRef(c, row), "-")
		}
		if day.QtyPercent != nil {
			f.SetCellValue(sheet, cellRef(c+1, row), fmt.Sprintf("%.2f%%", *day.QtyPercent))
		} else {
			f.SetCellValue(sheet, cellRef(c+1, row), "-")
		}
	}
	f.SetCellValue(sheet, cellRef(totalColStart, row), fmt.Sprintf("%.2f%%", data.GrandTotal.TotalLocationPercent))
	f.SetCellValue(sheet, cellRef(totalColStart+1, row), fmt.Sprintf("%.2f%%", data.GrandTotal.TotalQtyPercent))
	f.SetCellStyle(sheet, cellRef(1, row), cellRef(totalColStart+1, row), totalPctStyle)

	// Column width biar rapi
	f.SetColWidth(sheet, "A", "A", 18)
	lastCol, _ := excelize.ColumnNumberToName(totalColStart + 1)
	f.SetColWidth(sheet, "B", lastCol, 12)

	f.SetActiveSheet(0)
	return f, nil
}

// ── Route ────────────────────────────────────────────────────────────────
// Tambahin deket route export-division:
//
//   api.Get("/export-category/:code", stockTakeController.ExportProgressByCategory)
