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
	// SeedDivision(db)
	SeedMasterCartons(db)
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
				u.ID = uint(types.SnowflakeID(idgen.GenerateID()))
				db.Create(&u)
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
			Path:      "/wms/master/product",
			Icon:      "Box",
			MenuOrder: 1,
			ParentID:  getMenuIDByName(db, "Master Data"), // ambil ID parent
		},
		{
			Name:      "Supplier",
			Path:      "/wms/master/supplier",
			Icon:      "Truck",
			MenuOrder: 2,
			ParentID:  getMenuIDByName(db, "Master Data"),
		},
		{
			Name:      "Handling",
			Path:      "/wms/master/handling",
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

func SeedMasterCartons(db *gorm.DB) error {
	masterCartons := []models.MasterCarton{
		{
			CartonCode:  "CTN-01",
			CartonName:  "Carton 01",
			Description: "Small carton for lightweight items",
			Length:      30,
			Width:       20,
			Height:      15,
			MaxWeight:   5,
			TareWeight:  0.5,
			IsActive:    true,
			IsDefault:   false,
			Material:    "Cardboard",
			Color:       "Brown",
		},
		// {
		// 	CartonCode:  "CTN-MEDIUM",
		// 	CartonName:  "Medium Box",
		// 	Description: "Standard medium-sized carton",
		// 	Length:      40,
		// 	Width:       30,
		// 	Height:      25,
		// 	MaxWeight:   10,
		// 	TareWeight:  0.8,
		// 	IsActive:    true,
		// 	IsDefault:   true, // Default carton
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-LARGE",
		// 	CartonName:  "Large Box",
		// 	Description: "Large carton for bulky items",
		// 	Length:      60,
		// 	Width:       40,
		// 	Height:      40,
		// 	MaxWeight:   20,
		// 	TareWeight:  1.2,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-XLARGE",
		// 	CartonName:  "Extra Large Box",
		// 	Description: "Extra large carton for very bulky items",
		// 	Length:      80,
		// 	Width:       60,
		// 	Height:      50,
		// 	MaxWeight:   30,
		// 	TareWeight:  2.0,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-CUSTOM",
		// 	CartonName:  "Custom Box",
		// 	Description: "Custom sized carton",
		// 	Length:      50,
		// 	Width:       35,
		// 	Height:      30,
		// 	MaxWeight:   15,
		// 	TareWeight:  1.0,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "White",
		// },
	}

	for _, carton := range masterCartons {
		// Check if carton already exists
		var existing models.MasterCarton
		err := db.Where("carton_code = ?", carton.CartonCode).First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			// Create new carton
			if err := db.Create(&carton).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// RunSeeders - Run all seeders
// func RunSeeders(db *gorm.DB) error {
// 	if err := SeedMasterCartons(db); err != nil {
// 		return err
// 	}
// 	return nil
// }
