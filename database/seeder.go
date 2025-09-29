// database/seeder.go
package database

import (
	"errors"
	"fiber-app/config"
	"fiber-app/controllers/idgen"
	"fiber-app/models"
	"fiber-app/types"
	"log"

	"gorm.io/gorm"
)

func RunSeeders(db *gorm.DB) {
	SeedMenus(db)
	SeedUoms(db)
	// SeedWarehouse(db)
	SeedUserMaster(db)
	SeedCategory(db)
	SeedDivision(db)
}

func SeedUnit(db *gorm.DB) {
	unit := models.BusinessUnit{
		DbName: config.DBUnit,
	}

	var existing models.BusinessUnit
	err := db.Where("db_name = ?", unit.DbName).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&unit).Error; err != nil {
				log.Fatalf("Failed to create unit: %v", err)
			}
		} else {
			log.Fatalf("Unexpected DB error: %v", err)
		}
	}
}

func SeedCategory(db *gorm.DB) {
	categories := []models.Category{
		{
			Code: "BOOK",
			Name: "BOOK",
		},
		{
			Code: "INSTRUMENT",
			Name: "INSTRUMENT",
		},
	}

	for _, c := range categories {
		var existing models.Category
		if err := db.Where("name = ?", c.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&c)
			}
		}
	}

}

func SeedUoms(db *gorm.DB) {
	uoms := []models.Uom{
		{Code: "PCS", Name: "PCS"},
	}

	for _, u := range uoms {
		var existing models.Uom
		if err := db.Where("code = ?", u.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				u.ID = types.SnowflakeID(idgen.GenerateID())
				db.Create(&u)
			}
		}
	}
}

// func SeedWarehouse(db *gorm.DB) {
// 	warehouses := []models.Warehouse{
// 		{Code: "CKY", Name: "Warehouse 1", Description: "Warehouse Cakung"},
// 		{Code: "NGK", Name: "Warehouse 2", Description: "Warehouse Nagrak"},
// 	}

// 	for _, w := range warehouses {
// 		var existing models.Warehouse
// 		if err := db.Where("code = ?", w.Code).First(&existing).Error; err != nil {
// 			if err == gorm.ErrRecordNotFound {
// 				w.ID = types.SnowflakeID(idgen.GenerateID())
// 				db.Create(&w)
// 			}
// 		}
// 	}
// }

func SeedDivision(db *gorm.DB) {
	divisions := []models.Division{
		{Code: "REGULAR", Name: "REGULAR", Description: "REGULAR"},
	}

	for _, d := range divisions {
		var existing models.Division
		if err := db.Where("code = ?", d.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				d.ID = types.SnowflakeID(idgen.GenerateID())
				db.Create(&d)
			}
		}
	}
}

func SeedMenus(db *gorm.DB) error {
	menus := []models.Menu{
		{
			Name:      "Master Data",
			Path:      "#",
			Icon:      "Database",
			MenuOrder: 1,
		},
		{
			Name:      "Product",
			Path:      "/master/product",
			Icon:      "Box",
			MenuOrder: 1,
			ParentID:  getMenuIDByName(db, "Master Data"), // ambil ID parent
		},
		{
			Name:      "Supplier",
			Path:      "/master/supplier",
			Icon:      "Truck",
			MenuOrder: 2,
			ParentID:  getMenuIDByName(db, "Master Data"),
		},
		{
			Name:      "Handling",
			Path:      "/master/handling",
			Icon:      "Truck",
			MenuOrder: 3,
			ParentID:  getMenuIDByName(db, "Master Data"),
		},
	}

	for _, menu := range menus {
		var existing models.Menu
		err := db.Where("name = ? AND path = ?", menu.Name, menu.Path).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&menu).Error; err != nil {
				log.Println("Gagal insert menu:", menu.Name, err)
			} else {
				log.Println("Insert menu:", menu.Name)
			}
		}
	}

	return nil
}

func SeedUserMaster(db *gorm.DB) {
	users := []models.User{
		{
			Username:  "admin",
			Password:  "admin",
			Name:      "Admin",
			Email:     "admin@example.com",
			BaseRoute: "/dashboard",
			// Role:     "admin",
		},
	}

	for _, user := range users {
		var existing models.User
		err := db.Where("email = ?", user.Email).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&user).Error; err != nil {
				log.Println("Gagal insert user:", user.Username, err)
			} else {
				log.Println("Insert user:", user.Username)
			}
		}
	}
}

func getMenuIDByName(db *gorm.DB, name string) *uint {
	var parent models.Menu
	err := db.Where("name = ?", name).First(&parent).Error
	if err == nil {
		id := uint(parent.ID)
		return &id
	}
	return nil
}
