// database/migrate.go
package database

import (
	"fiber-app/models"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.Product{},
		&models.Customer{},
		&models.Supplier{},
		&models.Handling{},
		&models.HandlingRate{},
		&models.HandlingCombine{},
		&models.HandlingCombineDetail{},
		&models.InboundHeader{},
		&models.InboundDetail{},
		&models.InboundDetailHandling{},
		&models.Transporter{},
		&models.Truck{},
		&models.Origin{},
		&models.Inventory{},
		&models.Warehouse{},
		&models.QaStatus{},
		&models.InboundBarcode{},
		&models.Receiving{},
		&models.FileLog{},
		&models.OutboundHeader{},
		&models.OutboundDetail{},
		&models.OutboundDetailHandling{},
		&models.OutboundBarcode{},
		&models.PickingSheet{},
		&models.OutboundFile{},
		&models.ListOrderPart{},
		&models.OrderHeader{},
		&models.OrderDetail{},
		&models.Uom{},
		&models.StockTake{},
		&models.StockTakeItem{},
		&models.StockTakeBarcode{},
		&models.Menu{},
		&models.OrderConsole{},
		&models.KoliHeader{},
		&models.KoliDetail{},
	)
}
