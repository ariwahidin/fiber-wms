package repositories

import (
	"fiber-app/models"

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

// func (r *StockTakeRepository) GetFilteredStockCard(filter models.StockCardFilter) ([]ViewModelCardStockTake, error) {
// 	sql := `
// 	WITH inv AS (
// 		SELECT
// 			a.location, a.item_code, a.barcode, a.whs_code, SUM(a.qty_onhand) as quantity
// 		FROM inventories a
// 		WHERE a.qty_onhand > 0
// 		GROUP BY a.location, a.item_code, a.barcode, a.whs_code
// 	)
// 	SELECT DISTINCT
// 		inv.location, inv.item_code, inv.barcode, inv.quantity,
// 		loc.row, loc.bay, loc.level, loc.bin, loc.area,
// 		itm.item_name, inv.whs_code
// 	FROM inv
// 	INNER JOIN locations loc ON inv.location = loc.location_code
// 	INNER JOIN products itm ON inv.item_code = itm.item_code
// 	WHERE
// 		loc.row >= @fromRow AND loc.row <= @toRow AND
// 		loc.bay >= @fromBay AND loc.bay <= @toBay AND
// 		loc.level >= @fromLevel AND loc.level <= @toLevel AND
// 		loc.bin >= @fromBin AND loc.bin <= @toBin
// 	`

// 	// Tambahkan filter area jika diberikan
// 	if strings.TrimSpace(filter.Area) != "" {
// 		sql += " AND loc.area = @area"
// 	}

// 	var stockCards []ViewModelCardStockTake
// 	tx := r.db.Raw(sql,
// 		sql.Named("fromRow", filter.FromRow),
// 		sql.Named("toRow", filter.ToRow),
// 		sql.Named("fromBay", filter.FromBay),
// 		sql.Named("toBay", filter.ToBay),
// 		sql.Named("fromLevel", filter.FromLevel),
// 		sql.Named("toLevel", filter.ToLevel),
// 		sql.Named("fromBin", filter.FromBin),
// 		sql.Named("toBin", filter.ToBin),
// 		sql.Named("area", filter.Area),
// 	).Scan(&stockCards)

// 	if tx.Error != nil {
// 		return nil, tx.Error
// 	}

// 	return stockCards, nil
// }
