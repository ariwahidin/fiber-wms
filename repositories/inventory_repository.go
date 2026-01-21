package repositories

import (
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type InventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) *InventoryRepository {
	return &InventoryRepository{db}
}

type listInventory struct {
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	Location     string  `json:"location"`
	Barcode      string  `json:"barcode"`
	OwnerCode    string  `json:"owner_code"`
	RecDate      string  `json:"rec_date"`
	Category     string  `json:"category"`
	WhsCode      string  `json:"whs_code"`
	QaStatus     string  `json:"qa_status"`
	QtyIn        int     `json:"qty_in"`
	QtyOnhand    int     `json:"qty_onhand"`
	QtyAvailable int     `json:"qty_available"`
	QtyAllocated int     `json:"qty_allocated"`
	QtyOut       int     `json:"qty_out"`
	CbmPcs       float64 `json:"cbm_pcs"`
	CbmTotal     float64 `json:"cbm_total"`
	Uom          string  `json:"uom"`
}

func (r *InventoryRepository) GetInventory() ([]listInventory, error) {

	sqlInventory := `select a.whs_code, a.location, a.barcode, a.owner_code, a.rec_date, b.category,
	b.item_code, b.item_name, a.qa_status,
	sum(a.qty_origin) as qty_in,
	sum(a.qty_onhand) as qty_onhand,
	sum(a.qty_available) as qty_available,
	sum(a.qty_allocated) as qty_allocated,
	sum(a.qty_shipped) as qty_out,
	b.cbm as cbm_pcs,
	b.cbm * sum(a.qty_available) as cbm_total
	from inventories a
	inner join products b on a.item_id = b.id
	-- where a.qty_available > 0 or a.qty_allocated > 0
	where a.qty_origin > 0
	group by a.whs_code, a.location, b.item_code, b.item_name, a.qa_status,
	a.barcode, a.owner_code, a.rec_date, b.category, a.inbound_detail_id, b.cbm`

	var inventories []listInventory

	if err := r.db.Raw(sqlInventory).Scan(&inventories).Error; err != nil {
		return nil, err
	}

	if len(inventories) == 0 {
		inventories = []listInventory{}
	}

	return inventories, nil
}

func (r *InventoryRepository) GetInventoryByInbound(inbound_id int) ([]listInventory, error) {

	sqlInventory := `select a.whs_code, a.location, a.barcode, a.owner_code, a.rec_date, b.category,
	b.item_code, b.item_name, a.qa_status, a.uom,
	sum(a.qty_origin) as qty_in,
	sum(a.qty_onhand) as qty_onhand,
	sum(a.qty_available) as qty_available,
	sum(a.qty_allocated) as qty_allocated,
	sum(a.qty_shipped) as qty_out,
	b.cbm as cbm_pcs,
	b.cbm * sum(a.qty_available) as cbm_total
	from inventories a
	inner join products b on a.item_id = b.id
	WHERE 
	a.inbound_id = ?
	AND a.qty_origin > 0
	group by a.whs_code, a.location, b.item_code, b.item_name, a.qa_status, a.uom,
	a.barcode, a.owner_code, a.rec_date, b.category, a.inbound_detail_id, b.cbm`

	var inventories []listInventory

	if err := r.db.Raw(sqlInventory, inbound_id).Scan(&inventories).Error; err != nil {
		return nil, err
	}

	if len(inventories) == 0 {
		inventories = []listInventory{}
	}

	return inventories, nil
}

type StockOnHand struct {
	InventoryID       int    `json:"inventory_id"`
	InventoryDetailID int    `json:"inventory_detail_id"`
	InboundDetailID   int    `json:"inbound_detail_id"`
	Location          string `json:"location"`
	ItemID            int    `json:"item_id"`
	ItemCode          string `json:"item_code"`
	WhsCode           string `json:"whs_code"`
	QaStatus          string `json:"qa_status"`
	OnHand            int    `json:"on_hand"`
	Picked            int    `json:"picked"`
	Available         int    `json:"available"`
	RecDate           string `json:"rec_date"`
	SerialNumber      string `json:"serial_number"`
}

func (r *InventoryRepository) GetStockOnHand() ([]StockOnHand, error) {
	var stockOnHand []StockOnHand
	sql := `with ob_cte as 
	(
		select inventory_detail_id, sum(quantity) as picked from outbound_barcodes
		group by inventory_detail_id
	)
	select a.inventory_id, a.id as inventory_detail_id, a.location, a.inbound_detail_id, a.serial_number, a.qa_status,
	b.whs_code,
	b.item_id, b.item_code, c.rec_date, a.quantity as on_hand, isnull(d.picked, 0) as picked, a.quantity - isnull(d.picked, 0) as available
	from
	inventory_details a
	inner join inventories b on a.inventory_id = b.id
	inner join inbound_details c on a.inbound_detail_id = c.id
	left join ob_cte d on a.id = d.inventory_detail_id`

	if err := r.db.Raw(sql).Scan(&stockOnHand).Error; err != nil {
		return nil, err
	}

	return stockOnHand, nil
}

type ResGetStockByRequest struct {
	InventoryID  int    `json:"inventory_id"`
	RecDate      string `json:"rec_date"`
	ItemID       int    `json:"item_id"`
	ItemCode     string `json:"item_code"`
	WhsCode      string `json:"whs_code"`
	Pallet       string `json:"pallet"`
	Location     string `json:"location"`
	QaStatus     string `json:"qa_status"`
	SerialNumber string `json:"serial_number"`
	Stock        int    `json:"stock"`
	Alocated     int    `json:"alocated"`
	Available    int    `json:"available"`
}

func (r *InventoryRepository) GetStockByRequest(inbound_id int) ([]ResGetStockByRequest, error) {
	var stock []ResGetStockByRequest

	sql := `with obd AS(
	select item_id from outbound_details where outbound_id = ?)
	select a.id as inventory_id, a.rec_date, a.item_id, a.item_code, a.whs_code, a.pallet, 
	a.location, a.qa_status, a.serial_number, a.quantity as stock, coalesce(b.quantity, 0) as alocated,
	a.quantity - coalesce(b.quantity, 0) as available
	from inventories a
	left join picking_sheets b on a.id = b.inventory_id
	where a.item_id IN (select item_id from obd)
	order by rec_date desc`

	if err := r.db.Raw(sql, inbound_id).Scan(&stock).Error; err != nil {
		return nil, err
	}

	return stock, nil
}

func (r *InventoryRepository) GeneratePalletID() (string, error) {
	var lastPallet models.Inventory

	// Get the last pallet created today to generate sequence number
	today := time.Now()
	// startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	err := r.db.Where("pallet LIKE ?", fmt.Sprintf("PLT-%s-%%", today.Format("20060102"))).
		Order("created_at DESC").
		First(&lastPallet).Error

	sequence := 1
	if err == nil {
		// Extract sequence number from last pallet ID
		// Format: PLT-YYYYMMDD-XXXX
		parts := strings.Split(lastPallet.Pallet, "-")
		if len(parts) == 3 {
			lastSeq, _ := strconv.Atoi(parts[2])
			sequence = lastSeq + 1
		}
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}

	// Generate new Pallet ID
	// Format: PLT-YYYYMMDD-XXXX
	// PLT = Pallet
	// YYYYMMDD = Date
	// XXXX = Sequential number (4 digits)
	palletID := fmt.Sprintf("PLT-%s-%04d", today.Format("20060102"), sequence)

	return palletID, nil
}
