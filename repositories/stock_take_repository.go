package repositories

import (
	"fiber-app/models"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"
)

type StockTakeRepository struct {
	db *gorm.DB
}

func NewStockTakeRepository(db *gorm.DB) *StockTakeRepository {
	return &StockTakeRepository{db}
}

type ProgressStockTake struct {
	BarcodeSystem       string  `json:"barcode_system"`
	CountBarcodeSystem  int     `json:"count_barcode_system"`
	CountLocationSystem int     `json:"count_location_system"`
	TotalQtySystem      int     `json:"total_qty_system"`
	BarcodeSto          string  `json:"barcode_sto"`
	CountBarcodeSto     int     `json:"count_barcode_sto"`
	CountLocationSto    int     `json:"count_location_sto"`
	TotalQtySto         int     `json:"total_qty_sto"`
	ProgressBarcode     float64 `json:"progress_barcode"`
	ProgressLocation    float64 `json:"progress_location"`
	ProgressQty         float64 `json:"progress_qty"`
}

type ViewModelCardStockTake struct {
	Location string `json:"location"`
	ItemCode string `json:"item_code"`
	ItemName string `json:"item_name"`
	Barcode  string `json:"barcode"`
	Quantity int    `json:"quantity"`
	Row      string `json:"row"`
	Bay      string `json:"bay"`
	Level    string `json:"level"`
	Bin      string `json:"bin"`
	WhsCode  string `json:"whs_code"`
}

func (r *StockTakeRepository) GetProgressStockTakeByID(stockTakeID int) ([]ProgressStockTake, error) {

	sql := `WITH data_system AS (
        SELECT 
            a.stock_take_id, 
            a.barcode AS barcode_system, 
            a.location AS location_system,
            SUM(a.system_qty) AS qty_system
        FROM stock_take_items a
        GROUP BY a.stock_take_id, a.barcode, a.location
    ),

    data_actual AS (
        SELECT 
            a.stock_take_id,
            a.barcode AS barcode_sto, 
            a.location AS location_sto, 
            SUM(a.counted_qty) AS qty_sto
        FROM stock_take_barcodes a
        GROUP BY a.stock_take_id, a.barcode, a.location
    ),

    data_sto AS (
        SELECT 
            a.stock_take_id,
            a.barcode_system, 
            a.location_system, 
            a.qty_system,
            b.barcode_sto,
            b.location_sto,
            b.qty_sto
        FROM data_system a
        LEFT JOIN data_actual b 
            ON a.stock_take_id = b.stock_take_id 
            AND a.barcode_system = b.barcode_sto 
            AND a.location_system = b.location_sto
        WHERE a.stock_take_id = ?
    ),
    summary_data AS (
        SELECT 
            barcode_system, 
            COUNT(DISTINCT barcode_system) AS count_barcode_system, 
            COUNT(DISTINCT location_system) AS count_location_system, 
            SUM(qty_system) AS total_qty_system,
            barcode_sto, 
            COUNT(DISTINCT barcode_sto) AS count_barcode_sto, 
            COUNT(DISTINCT location_sto) AS count_location_sto, 
            COALESCE(SUM(qty_sto), 0) AS total_qty_sto
        FROM data_sto
        GROUP BY barcode_system, barcode_sto
    )
    SELECT *,
        CASE 
            WHEN count_barcode_system > 0 THEN (count_barcode_sto * 1.0 / count_barcode_system) * 100 
            ELSE 0 
        END AS progress_barcode,
        CASE 
            WHEN count_location_system > 0 THEN (count_location_sto * 1.0 / count_location_system) * 100 
            ELSE 0 
        END AS progress_location,
        CASE 
            WHEN total_qty_system > 0 THEN (total_qty_sto * 1.0 / total_qty_system) * 100 
            ELSE 0 
        END AS progress_qty
    FROM summary_data;
    `

	var progressStockTake []ProgressStockTake

	if err := r.db.Raw(sql, stockTakeID).Scan(&progressStockTake).Error; err != nil {
		return nil, err
	}

	return progressStockTake, nil

}

func (r *StockTakeRepository) GetAllStockCard() ([]ViewModelCardStockTake, error) {
	sql := `WITH inv AS
    (select a.location, a.item_code, a.barcode, a.whs_code, SUM(a.qty_onhand) as quantity 
    from inventories a
    where a.qty_onhand > 0
    group by a.location, a.item_code, a.barcode, a.whs_code)
    SELECT distinct location, inv.item_code, inv.barcode, quantity, row, bay, level, bin, itm.item_name, inv.whs_code
    FROM inv 
    INNER JOIN locations loc ON inv.location = loc.location_code
    INNER JOIN products itm ON inv.item_code = itm.item_code`
	var stockCards []ViewModelCardStockTake
	if err := r.db.Raw(sql).Scan(&stockCards).Error; err != nil {
		return nil, err
	}

	if len(stockCards) == 0 {
		return []ViewModelCardStockTake{}, nil
	}
	return stockCards, nil
}

func (r *StockTakeRepository) GetFilteredStockCard(filter models.StockCardFilter) ([]ViewModelCardStockTake, error) {
	sql := `
	WITH inv AS (
		SELECT 
			a.location, a.item_code, a.barcode, a.whs_code, SUM(a.qty_onhand) as quantity 
		FROM inventories a
		WHERE a.qty_onhand > 0
		GROUP BY a.location, a.item_code, a.barcode, a.whs_code
	)
	SELECT DISTINCT 
		inv.location, inv.item_code, inv.barcode, inv.quantity,
		loc.row, loc.bay, loc.level, loc.bin, loc.area,
		itm.item_name, inv.whs_code
	FROM inv 
	INNER JOIN locations loc ON inv.location = loc.location_code
	INNER JOIN products itm ON inv.item_code = itm.item_code
	WHERE 
		loc.row >= ? AND loc.row <= ? AND
		loc.bay >= ? AND loc.bay <= ? AND
		loc.level >= ? AND loc.level <= ? AND
		loc.bin >= ? AND loc.bin <= ?
	`

	var args []interface{} = []interface{}{
		filter.FromRow, filter.ToRow,
		filter.FromBay, filter.ToBay,
		filter.FromLevel, filter.ToLevel,
		filter.FromBin, filter.ToBin,
	}

	// if strings.TrimSpace(filter.Area) != "" {
	// 	sql += " AND loc.area = ?"
	// 	args = append(args, filter.Area)
	// }

	var stockCards []ViewModelCardStockTake
	if err := r.db.Raw(sql, args...).Scan(&stockCards).Error; err != nil {
		return nil, err
	}

	return stockCards, nil
}

// ─── By SKU (dengan Item Name & SKU) ───────────────────────────────────────

type ProgressBySKU struct {
	ItemID              int64   `json:"item_id"`
	ItemName            string  `json:"item_name"`
	Sku                 string  `json:"sku"`
	CountLocationSystem int     `json:"count_location_system"`
	TotalQtySystem      int     `json:"total_qty_system"`
	CountLocationSto    int     `json:"count_location_sto"`
	TotalQtySto         int     `json:"total_qty_sto"`
	ProgressLocation    float64 `json:"progress_location"`
	ProgressQty         float64 `json:"progress_qty"`
}

// GetProgressBySKU meng-agregasi progress per item (SKU), lengkap dengan
// nama item — dipakai untuk tab "By SKU" di dashboard progress.
func (r *StockTakeRepository) GetProgressBySKU(stockTakeID uint) ([]ProgressBySKU, error) {
	type aggRow struct {
		ItemID        int64
		LocationCount int
		Total         int
	}

	// Sisi rencana (system): snapshot dari stock_take_items saat generate STO
	var systemAggs []aggRow
	if err := r.db.Model(&models.StockTakeItem{}).
		Select("item_id, COUNT(DISTINCT location) as location_count, SUM(system_qty) as total").
		Where("stock_take_id = ?", stockTakeID).
		Group("item_id").
		Scan(&systemAggs).Error; err != nil {
		return nil, err
	}

	// Sisi hasil scan aktual: dari stock_take_barcodes
	var stoAggs []aggRow
	if err := r.db.Model(&models.StockTakeBarcode{}).
		Select("item_id, COUNT(DISTINCT location) as location_count, SUM(counted_qty) as total").
		Where("stock_take_id = ?", stockTakeID).
		Group("item_id").
		Scan(&stoAggs).Error; err != nil {
		return nil, err
	}

	systemMap := make(map[int64]aggRow, len(systemAggs))
	for _, a := range systemAggs {
		systemMap[a.ItemID] = a
	}
	stoMap := make(map[int64]aggRow, len(stoAggs))
	for _, a := range stoAggs {
		stoMap[a.ItemID] = a
	}

	// Union item_id dari kedua sisi — jaga-jaga ada barang ke-scan yang
	// sebenarnya di luar rencana system (misal salah SKU pas scan).
	itemIDSet := make(map[int64]struct{})
	for id := range systemMap {
		itemIDSet[id] = struct{}{}
	}
	for id := range stoMap {
		itemIDSet[id] = struct{}{}
	}
	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}

	var products []models.Product
	if len(itemIDs) > 0 {
		if err := r.db.Where("id IN ?", itemIDs).Find(&products).Error; err != nil {
			return nil, err
		}
	}
	productMap := make(map[int64]models.Product, len(products))
	for _, p := range products {
		productMap[int64(p.ID)] = p
	}

	result := make([]ProgressBySKU, 0, len(itemIDs))
	for _, id := range itemIDs {
		sys := systemMap[id]
		sto := stoMap[id]
		product := productMap[id]

		row := ProgressBySKU{
			ItemID:              id,
			ItemName:            product.ItemName,
			Sku:                 product.ItemCode,
			CountLocationSystem: sys.LocationCount,
			TotalQtySystem:      sys.Total,
			CountLocationSto:    sto.LocationCount,
			TotalQtySto:         sto.Total,
		}

		if sys.LocationCount > 0 {
			row.ProgressLocation = math.Round(float64(sto.LocationCount)/float64(sys.LocationCount)*10000) / 100
		}
		if sys.Total > 0 {
			row.ProgressQty = math.Round(float64(sto.Total)/float64(sys.Total)*10000) / 100
		}

		result = append(result, row)
	}

	// Urutkan by Item Name biar enak dibaca (bukan urutan random dari map)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ItemName < result[j].ItemName
	})

	return result, nil
}

// ─── By Division (pivot per tanggal) ───────────────────────────────────────

type DivisionDayProgress struct {
	LocationCounted *int     `json:"location_counted"`
	QtyCounted      *int     `json:"qty_counted"`
	ProgressPercent *float64 `json:"progress_percent"`
}

type DivisionPivotRow struct {
	DivisionCode   string                         `json:"division_code"`
	SystemQty      int                            `json:"system_qty"`
	SystemLocation int                            `json:"system_location"`
	Daily          map[string]DivisionDayProgress `json:"daily"`
}

type ProgressByDivisionResult struct {
	Dates     []string           `json:"dates"`
	Divisions []DivisionPivotRow `json:"divisions"`
}

// GetProgressByDivisionPivot meng-agregasi progress per division, dipecah
// per tanggal (dari created_at hasil scan), dengan progress % cumulative
// (running total s.d. tanggal itu, dibagi total qty system division tsb).
func (r *StockTakeRepository) GetProgressByDivisionPivot(stockTakeID uint) (*ProgressByDivisionResult, error) {
	// 1. Snapshot system per division (statis, gak ada dimensi tanggal)
	type systemAgg struct {
		DivisionCode  string
		LocationCount int
		Total         int
	}
	var systemAggs []systemAgg
	if err := r.db.Model(&models.StockTakeItem{}).
		Select("division_code, COUNT(DISTINCT location) as location_count, SUM(system_qty) as total").
		Where("stock_take_id = ?", stockTakeID).
		Group("division_code").
		Scan(&systemAggs).Error; err != nil {
		return nil, err
	}

	// 2. Breakdown harian per division dari hasil scan aktual.
	// CAST(created_at AS DATE) dipakai (bukan DATE()) karena target DB SQL Server.
	type dailyAgg struct {
		DivisionCode  string
		ScanDate      time.Time
		LocationCount int
		Total         int
	}
	var dailyAggs []dailyAgg
	if err := r.db.Model(&models.StockTakeBarcode{}).
		Select("division_code, CAST(created_at AS DATE) as scan_date, COUNT(DISTINCT location) as location_count, SUM(counted_qty) as total").
		Where("stock_take_id = ?", stockTakeID).
		Group("division_code, CAST(created_at AS DATE)").
		Order("division_code, scan_date").
		Scan(&dailyAggs).Error; err != nil {
		return nil, err
	}

	// Union tanggal (dari semua division) — biar kolom pivot konsisten
	dateSet := make(map[string]struct{})
	for _, d := range dailyAggs {
		dateSet[d.ScanDate.Format("2006-01-02")] = struct{}{}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// Grouping daily per division — tetap urut tanggal (query sudah ORDER BY scan_date)
	dailyByDivision := make(map[string][]dailyAgg)
	for _, d := range dailyAggs {
		dailyByDivision[d.DivisionCode] = append(dailyByDivision[d.DivisionCode], d)
	}

	systemMap := make(map[string]systemAgg, len(systemAggs))
	for _, s := range systemAggs {
		systemMap[s.DivisionCode] = s
	}

	// Union division_code dari kedua sisi
	divSet := make(map[string]struct{})
	for _, s := range systemAggs {
		divSet[s.DivisionCode] = struct{}{}
	}
	for _, d := range dailyAggs {
		divSet[d.DivisionCode] = struct{}{}
	}

	divisions := make([]DivisionPivotRow, 0, len(divSet))
	for divCode := range divSet {
		sys := systemMap[divCode]
		row := DivisionPivotRow{
			DivisionCode:   divCode,
			SystemQty:      sys.Total,
			SystemLocation: sys.LocationCount,
			Daily:          make(map[string]DivisionDayProgress),
		}

		// Cumulative qty berjalan, dihitung urut tanggal per division.
		// Tanggal yang divisionnya gak ada aktivitas SENGAJA tidak dimasukkan
		// ke map Daily — di frontend ini artinya cell kosong ("-"), bukan 0.
		var cumulativeQty int
		for _, d := range dailyByDivision[divCode] {
			dateKey := d.ScanDate.Format("2006-01-02")
			cumulativeQty += d.Total

			locCount := d.LocationCount
			qtyCount := d.Total

			dayProgress := DivisionDayProgress{
				LocationCounted: &locCount,
				QtyCounted:      &qtyCount,
			}
			if sys.Total > 0 {
				pct := math.Round(float64(cumulativeQty)/float64(sys.Total)*10000) / 100
				dayProgress.ProgressPercent = &pct
			}
			row.Daily[dateKey] = dayProgress
		}

		divisions = append(divisions, row)
	}

	sort.Slice(divisions, func(i, j int) bool {
		return divisions[i].DivisionCode < divisions[j].DivisionCode
	})

	return &ProgressByDivisionResult{
		Dates:     dates,
		Divisions: divisions,
	}, nil
}

type LocationItemDetail struct {
	ItemID     int64  `json:"item_id"`
	ItemName   string `json:"item_name"`
	Sku        string `json:"sku"`
	SystemQty  int    `json:"system_qty"`
	CountedQty int    `json:"counted_qty"`
	Variance   int    `json:"variance"`
	Status     string `json:"status"` // matched, over, under, not_counted, extra
}

type LocationPivotRow struct {
	Location         string               `json:"location"`
	DivisionCode     string               `json:"division_code"`
	SystemItemCount  int                  `json:"system_item_count"`
	SystemQty        int                  `json:"system_qty"`
	CountedItemCount int                  `json:"counted_item_count"`
	CountedQty       int                  `json:"counted_qty"`
	Status           string               `json:"status"` // not_started, partial, done
	HasDiscrepancy   bool                 `json:"has_discrepancy"`
	Items            []LocationItemDetail `json:"items"`
}

// GetProgressByLocation meng-agregasi progress per lokasi fisik (rak/bin),
// dengan status coverage (belum/sebagian/selesai) dan flag diskrepansi qty,
// plus detail per item untuk drill-down di frontend.
func (r *StockTakeRepository) GetProgressByLocation(stockTakeID uint) ([]LocationPivotRow, error) {
	type aggRow struct {
		Location     string
		DivisionCode string
		ItemID       int64
		Qty          int
	}

	var systemAggs []aggRow
	if err := r.db.Model(&models.StockTakeItem{}).
		Select("location, division_code, item_id, SUM(system_qty) as qty").
		Where("stock_take_id = ?", stockTakeID).
		Group("location, division_code, item_id").
		Scan(&systemAggs).Error; err != nil {
		return nil, err
	}

	var stoAggs []aggRow
	if err := r.db.Model(&models.StockTakeBarcode{}).
		Select("location, division_code, item_id, SUM(counted_qty) as qty").
		Where("stock_take_id = ?", stockTakeID).
		Group("location, division_code, item_id").
		Scan(&stoAggs).Error; err != nil {
		return nil, err
	}

	type key struct {
		Location string
		ItemID   int64
	}
	systemMap := make(map[key]aggRow, len(systemAggs))
	locationDivision := make(map[string]string)
	for _, a := range systemAggs {
		systemMap[key{a.Location, a.ItemID}] = a
		locationDivision[a.Location] = a.DivisionCode
	}
	stoMap := make(map[key]aggRow, len(stoAggs))
	for _, a := range stoAggs {
		stoMap[key{a.Location, a.ItemID}] = a
		if _, ok := locationDivision[a.Location]; !ok {
			locationDivision[a.Location] = a.DivisionCode
		}
	}

	keySet := make(map[key]struct{})
	for k := range systemMap {
		keySet[k] = struct{}{}
	}
	for k := range stoMap {
		keySet[k] = struct{}{}
	}

	itemIDSet := make(map[int64]struct{})
	for k := range keySet {
		itemIDSet[k.ItemID] = struct{}{}
	}
	itemIDs := make([]int64, 0, len(itemIDSet))
	for id := range itemIDSet {
		itemIDs = append(itemIDs, id)
	}

	var products []models.Product
	if len(itemIDs) > 0 {
		if err := r.db.Where("id IN ?", itemIDs).Find(&products).Error; err != nil {
			return nil, err
		}
	}
	productMap := make(map[int64]models.Product, len(products))
	for _, p := range products {
		productMap[int64(p.ID)] = p
	}

	locItems := make(map[string][]LocationItemDetail)
	for k := range keySet {
		sys, hasSys := systemMap[k]
		sto, hasSto := stoMap[k]
		product := productMap[k.ItemID]

		systemQty := 0
		if hasSys {
			systemQty = sys.Qty
		}
		countedQty := 0
		if hasSto {
			countedQty = sto.Qty
		}

		var status string
		switch {
		case !hasSto:
			status = "not_counted"
		case !hasSys:
			status = "extra" // discan tapi gak ada di rencana system (lokasi/SKU salah scan)
		case countedQty == systemQty:
			status = "matched"
		case countedQty > systemQty:
			status = "over"
		default:
			status = "under"
		}

		locItems[k.Location] = append(locItems[k.Location], LocationItemDetail{
			ItemID:     k.ItemID,
			ItemName:   product.ItemName,
			Sku:        product.ItemCode,
			SystemQty:  systemQty,
			CountedQty: countedQty,
			Variance:   countedQty - systemQty,
			Status:     status,
		})
	}

	result := make([]LocationPivotRow, 0, len(locItems))
	for loc, items := range locItems {
		sort.Slice(items, func(i, j int) bool { return items[i].ItemName < items[j].ItemName })

		row := LocationPivotRow{Location: loc, DivisionCode: locationDivision[loc], Items: items}

		systemItemCount, countedItemCount := 0, 0
		for _, it := range items {
			if it.Status != "extra" {
				row.SystemQty += it.SystemQty
				systemItemCount++
			}
			if it.Status != "not_counted" {
				row.CountedQty += it.CountedQty
				countedItemCount++
			}
			if it.Status == "over" || it.Status == "under" || it.Status == "extra" {
				row.HasDiscrepancy = true
			}
		}
		row.SystemItemCount = systemItemCount
		row.CountedItemCount = countedItemCount

		switch {
		case countedItemCount == 0:
			row.Status = "not_started"
		case systemItemCount > 0 && countedItemCount >= systemItemCount:
			row.Status = "done"
		default:
			row.Status = "partial"
		}

		result = append(result, row)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].DivisionCode != result[j].DivisionCode {
			return result[i].DivisionCode < result[j].DivisionCode
		}
		return result[i].Location < result[j].Location
	})

	return result, nil
}

type PicQualityBreakdown struct {
	Matched int `json:"matched"`
	Over    int `json:"over"`
	Under   int `json:"under"`
	Extra   int `json:"extra"`
}

type PicProgress struct {
	UserID        int                 `json:"user_id"`
	UserName      string              `json:"user_name"`
	LocationCount int                 `json:"location_count"`
	ItemCount     int                 `json:"item_count"`
	ScanCount     int                 `json:"scan_count"`
	QtyCounted    int                 `json:"qty_counted"`
	FirstScanAt   *time.Time          `json:"first_scan_at"`
	LastScanAt    *time.Time          `json:"last_scan_at"`
	Quality       PicQualityBreakdown `json:"quality"`
}

// GetProgressByPic meng-agregasi aktivitas & kualitas scan per petugas (PIC).
// Status kualitas (matched/over/under/extra) dihitung di level lokasi+item
// (gabungan semua scan di titik itu), lalu tiap baris scan individual
// mewarisi status tersebut untuk ditotal per user — jadi ini sinyal
// "PIC ini banyak berada di titik yang bermasalah", bukan vonis akurat 1:1.
func (r *StockTakeRepository) GetProgressByPic(stockTakeID uint) ([]PicProgress, error) {
	type locItemKey struct {
		Location string
		ItemID   uint
	}

	// 1. System qty per lokasi+item (buat nentuin status matched/over/under)
	type sysAgg struct {
		Location string
		ItemID   uint
		Qty      int
	}
	var sysAggs []sysAgg
	if err := r.db.Model(&models.StockTakeItem{}).
		Select("location, item_id, SUM(system_qty) as qty").
		Where("stock_take_id = ?", stockTakeID).
		Group("location, item_id").
		Scan(&sysAggs).Error; err != nil {
		return nil, err
	}
	sysMap := make(map[locItemKey]int, len(sysAggs))
	for _, s := range sysAggs {
		sysMap[locItemKey{s.Location, s.ItemID}] = s.Qty
	}

	// 2. Total counted qty per lokasi+item (gabungan semua PIC)
	type stoAgg struct {
		Location string
		ItemID   uint
		Qty      int
	}
	var stoAggs []stoAgg
	if err := r.db.Model(&models.StockTakeBarcode{}).
		Select("location, item_id, SUM(counted_qty) as qty").
		Where("stock_take_id = ?", stockTakeID).
		Group("location, item_id").
		Scan(&stoAggs).Error; err != nil {
		return nil, err
	}

	locItemStatus := make(map[locItemKey]string, len(stoAggs))
	for _, s := range stoAggs {
		sysQty, hasSys := sysMap[locItemKey{s.Location, s.ItemID}]
		switch {
		case !hasSys:
			locItemStatus[locItemKey{s.Location, s.ItemID}] = "extra"
		case s.Qty == sysQty:
			locItemStatus[locItemKey{s.Location, s.ItemID}] = "matched"
		case s.Qty > sysQty:
			locItemStatus[locItemKey{s.Location, s.ItemID}] = "over"
		default:
			locItemStatus[locItemKey{s.Location, s.ItemID}] = "under"
		}
	}

	// 3. Baris scan mentah per PIC — join manual ke users (CreatedBy bukan FK tertyped)
	type rawScan struct {
		CreatedBy  int
		UserName   string
		Location   string
		ItemID     uint
		CountedQty int
		CreatedAt  time.Time
	}
	var rawScans []rawScan
	if err := r.db.Table("stock_take_barcodes AS b").
		Select("b.created_by, u.name as user_name, b.location, b.item_id, b.counted_qty, b.created_at").
		Joins("LEFT JOIN users AS u ON u.id = b.created_by").
		Where("b.stock_take_id = ? AND b.deleted_at IS NULL", stockTakeID).
		Scan(&rawScans).Error; err != nil {
		return nil, err
	}

	type picAgg struct {
		userName   string
		locations  map[string]struct{}
		items      map[uint]struct{}
		scanCount  int
		qtyCounted int
		firstScan  time.Time
		lastScan   time.Time
		quality    PicQualityBreakdown
	}
	picMap := make(map[int]*picAgg)

	for _, s := range rawScans {
		p, ok := picMap[s.CreatedBy]
		if !ok {
			p = &picAgg{
				userName:  s.UserName,
				locations: make(map[string]struct{}),
				items:     make(map[uint]struct{}),
				firstScan: s.CreatedAt,
				lastScan:  s.CreatedAt,
			}
			picMap[s.CreatedBy] = p
		}

		p.locations[s.Location] = struct{}{}
		p.items[s.ItemID] = struct{}{}
		p.scanCount++
		p.qtyCounted += s.CountedQty
		if s.CreatedAt.Before(p.firstScan) {
			p.firstScan = s.CreatedAt
		}
		if s.CreatedAt.After(p.lastScan) {
			p.lastScan = s.CreatedAt
		}

		switch locItemStatus[locItemKey{s.Location, s.ItemID}] {
		case "matched":
			p.quality.Matched++
		case "over":
			p.quality.Over++
		case "under":
			p.quality.Under++
		case "extra":
			p.quality.Extra++
		}
	}

	result := make([]PicProgress, 0, len(picMap))
	for userID, p := range picMap {
		firstScan := p.firstScan
		lastScan := p.lastScan
		result = append(result, PicProgress{
			UserID:        userID,
			UserName:      p.userName,
			LocationCount: len(p.locations),
			ItemCount:     len(p.items),
			ScanCount:     p.scanCount,
			QtyCounted:    p.qtyCounted,
			FirstScanAt:   &firstScan,
			LastScanAt:    &lastScan,
			Quality:       p.quality,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].QtyCounted > result[j].QtyCounted
	})

	return result, nil
}
