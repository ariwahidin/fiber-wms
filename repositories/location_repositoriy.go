// repositories/location_repository.go
package repositories

import (
	"fmt"

	"gorm.io/gorm"
)

type LocationRepository struct {
	DB *gorm.DB
}

func NewLocationRepository(db *gorm.DB) *LocationRepository {
	return &LocationRepository{DB: db}
}

// activeStockTakeStatuses - status yang dianggap "sedang berjalan".
// TODO: konfirmasi apakah ada status lain selain 'open' yang juga harus di-block.
var activeStockTakeStatuses = []string{"open", "in_progress"}

// IsLocationUnderCycleCount - cek satu lokasi spesifik (whs_code + location),
// apakah sedang ada stock take aktif di lokasi itu.
func (r *LocationRepository) IsLocationUnderCycleCount(whsCode, location string) (bool, error) {
	var count int64
	err := r.DB.
		Table("stock_take_items as sti").
		Joins("JOIN stock_takes as st ON st.id = sti.stock_take_id").
		Joins("JOIN inventories as inv ON inv.id = sti.inventory_id").
		Where(`
			inv.whs_code = ?
			AND sti.location = ?
			AND st.status IN (?)
			AND sti.deleted_at IS NULL
			AND st.deleted_at IS NULL
		`, whsCode, location, activeStockTakeStatuses).
		Count(&count).Error

	return count > 0, err
}

// ExcludeLocationsUnderCycleCount - dipanggil setelah query inventory dibuat.
// inventoryAlias harus sama dengan alias tabel inventories di query pemanggil
// (mis. "i" pada Table("inventories as i")).
func (r *LocationRepository) ExcludeLocationsUnderCycleCount(query *gorm.DB, inventoryAlias string) *gorm.DB {
	subquery := fmt.Sprintf(`
		NOT EXISTS (
			SELECT 1
			FROM stock_take_items sti WITH (NOLOCK)
			JOIN stock_takes st WITH (NOLOCK) ON st.id = sti.stock_take_id
			JOIN inventories inv2 WITH (NOLOCK) ON inv2.id = sti.inventory_id
			WHERE inv2.whs_code = %[1]s.whs_code
			AND sti.location = %[1]s.location
			AND st.status IN ('open', 'in_progress')
			AND sti.deleted_at IS NULL
			AND st.deleted_at IS NULL
		)
	`, inventoryAlias)
	return query.Where(subquery)
}
