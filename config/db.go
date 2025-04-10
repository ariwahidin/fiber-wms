package config

import (
	"fiber-app/models"
	"fmt"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// ConnectDB membuat koneksi ke database menggunakan Gorm
func ConnectDB() (*gorm.DB, error) {

	dsn := "sqlserver://" + DBUser + ":" + DBPassword + "@" + DBHost + ":" + DBPort + "?database=" + DBName
	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{})

	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return nil, err
	}

	fmt.Println("🤣 Connected to database")
	db.AutoMigrate(&models.User{})
	db.AutoMigrate(&models.Role{})
	db.AutoMigrate(&models.Permission{})

	db.AutoMigrate(&models.Product{})
	db.AutoMigrate(&models.Customer{})
	db.AutoMigrate(&models.Supplier{})
	db.AutoMigrate(&models.Handling{})
	db.AutoMigrate(&models.HandlingRate{})
	db.AutoMigrate(&models.HandlingCombine{})
	db.AutoMigrate(&models.HandlingCombineDetail{})
	db.AutoMigrate(&models.InboundHeader{})
	db.AutoMigrate(&models.InboundDetail{})
	db.AutoMigrate(&models.InboundDetailHandling{})
	db.AutoMigrate(&models.Transporter{})
	db.AutoMigrate(&models.Truck{})
	db.AutoMigrate(&models.Origin{})
	db.AutoMigrate(&models.Inventory{})
	db.AutoMigrate(&models.InventoryDetail{})
	db.AutoMigrate(&models.WarehouseCode{})
	db.AutoMigrate(&models.QaStatus{})
	db.AutoMigrate(&models.InboundBarcode{})

	db.AutoMigrate(&models.Receiving{})
	db.AutoMigrate(&models.FileLog{})

	db.AutoMigrate(&models.OutboundHeader{})
	db.AutoMigrate(&models.OutboundDetail{})
	db.AutoMigrate(&models.OutboundDetailHandling{})
	db.AutoMigrate(&models.OutboundBarcode{})
	db.AutoMigrate(&models.PickingSheet{})
	db.AutoMigrate(&models.OutboundFile{})

	return db, nil
}
