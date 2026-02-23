package migration

import (
	"fiber-app/models"
	"fiber-app/models/integration"
	"fiber-app/models/notification"
	"fiber-app/models/report_mailer"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.BusinessUnit{},
	)
}

func MigrateBusinessUnit(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.Product{},
		&models.Customer{},
		&models.Supplier{},
		&models.InboundHeader{},
		&models.InboundDetail{},
		&models.InboundReference{},
		&models.Transporter{},
		&models.Truck{},
		&models.Origin{},
		&models.Inventory{},
		&models.InventoryMovement{},
		&models.Warehouse{},
		&models.QaStatus{},
		&models.InboundBarcode{},
		&models.Receiving{},
		&models.FileLog{},
		&models.OutboundHeader{},
		&models.OutboundDetail{},
		&models.OutboundDetailHandling{},
		&models.OutboundPicking{},
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
		&models.OutboundBarcode{},
		&models.OutboundPacking{},
		&models.Category{},
		&models.TransactionHistory{},
		&models.UomConversion{},
		&models.Division{},
		&models.Location{},
		&models.Owner{},

		&models.MainVas{},
		&models.VasRate{},
		&models.Vas{},
		&models.VasDetail{},
		&models.OutboundVas{},
		&models.InventoryPolicy{},
		&models.IntegrationLog{},
		&models.LoginLog{},
		&models.ItemPackaging{},
		&models.UserOwner{},
		&models.UserSession{},
		&models.LoginConflict{},

		&models.ProductRegister{},
		&models.MasterCarton{},
		&report_mailer.EmailConfig{},
		&report_mailer.Report{},
		&report_mailer.ReportRecipient{},
		&report_mailer.ReportSchedule{},
		&report_mailer.SendHistory{},
		&report_mailer.ReportQuery{},

		&notification.EmailNotification{},
		&notification.NotificationRecipient{},
		&notification.NotificationHistory{},

		&integration.Integration{},
		&integration.IntegrationConnection{},
		&integration.IntegrationRecipient{},
		&integration.IntegrationHistory{},
	)
}
