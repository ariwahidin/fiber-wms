package seed

import (
	"fiber-app/models"

	"gorm.io/gorm"
)

func SeedUoms(db *gorm.DB) {
	uoms := []models.Uom{
		{Code: "PCS", Name: "Piece"},
		{Code: "BOX", Name: "Box"},
		{Code: "CTN", Name: "Carton"},
	}

	for _, u := range uoms {
		var existing models.Uom
		if err := db.Where("code = ?", u.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&u)
			}
		}
	}
}

func SeedWarehouse(db *gorm.DB) {
	warehouses := []models.Warehouse{
		{Code: "CKY", Name: "Warehouse 1", Description: "Warehouse Cakung"},
		{Code: "NGK", Name: "Warehouse 2", Description: "Warehouse Nagrak"},
	}

	for _, w := range warehouses {
		var existing models.Warehouse
		if err := db.Where("code = ?", w.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&w)
			}
		}
	}
}
