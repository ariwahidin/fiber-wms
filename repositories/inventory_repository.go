package repositories

import (
	"gorm.io/gorm"
)

type InventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) *InventoryRepository {
	return &InventoryRepository{db}
}

type listInventory struct {
	ItemCode string `json:"item_code"`
	ItemName string `json:"item_name"`
	Location string `json:"location"`
	WhsCode  string `json:"whs_code"`
	QaStatus string `json:"qa_status"`
	Quantity int    `json:"quantity"`
}

func (r *InventoryRepository) GetInventory() ([]listInventory, error) {

	sqlInventory := `select b.whs_code, a.inventory_id, a.location, b.item_code, c.item_name,
	a.qa_status, sum(a.quantity) as quantity
	from inventory_details a
	inner join inventories b on a.inventory_id = b.id
	inner join products c on b.item_id = c.id
	group by a.inventory_id, a.location, a.qa_status,
	b.item_code, c.item_name, b.whs_code
	`

	var inventories []listInventory

	if err := r.db.Raw(sqlInventory).Scan(&inventories).Error; err != nil {
		return nil, err
	}

	return inventories, nil
}
