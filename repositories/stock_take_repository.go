package repositories

import (
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

func (r *StockTakeRepository) GetProgressStockTakeByID(stockTakeID int) ([]ProgressStockTake, error) {

	sql := `WITH data_system AS
	(SELECT a.stock_take_id, a.barcode AS barcode_system, a.location AS location_system,
	SUM(a.system_qty) AS qty_system
	FROM stock_take_items a
	GROUP BY a.stock_take_id, a.barcode, a.location), 

	data_actual AS
	(SELECT a.stock_take_id,
	a.barcode AS barcode_sto, a.location AS location_sto, SUM(a.counted_qty) AS qty_sto
	FROM stock_take_barcodes a
	GROUP BY a.stock_take_id, a.barcode, a.location), 

	data_sto AS
	(SELECT * FROM data_system a
	LEFT JOIN data_actual b ON a.stock_take_id = b.stock_take_id AND a.barcode_system = b.barcode_sto AND a.location_system = b.location_sto 
	WHERE a.stock_take_id = ?
	ORDER BY location_system ASC),

	summary_data AS
	(SELECT barcode_system, COUNT(barcode_system) AS count_barcode_system, COUNT(location_system) AS count_location_system, SUM(qty_system) AS total_qty_system,
	barcode_sto, COUNT(barcode_sto) AS count_barcode_sto, COUNT(location_sto) AS count_location_sto, COALESCE(SUM(qty_sto),0) AS total_qty_sto 
	FROM data_sto
	GROUP BY barcode_system, barcode_sto)

	SELECT *, 
	CASE WHEN count_barcode_system > 0 then (count_barcode_sto/count_barcode_system) * 100 ELSE 0 END AS  progress_barcode,
	CASE WHEN count_location_system > 0 then (count_location_sto/count_location_system) * 100 ELSE 0 END AS progress_location,
	CASE WHEN total_qty_system > 0 then (total_qty_sto/total_qty_system) * 100 ELSE 0 END AS progress_qty
	FROM summary_data`

	var progressStockTake []ProgressStockTake

	if err := r.db.Raw(sql, stockTakeID).Scan(&progressStockTake).Error; err != nil {
		return nil, err
	}

	return progressStockTake, nil

}
