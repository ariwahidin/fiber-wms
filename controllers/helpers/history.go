package helpers

import (
	"fiber-app/models"
	"time"

	"gorm.io/gorm"
)

// InsertTransactionHistory inserts a new transaction history record.
func InsertTransactionHistory(db *gorm.DB, refNo, status, txType, detail string, actor int) error {
	history := models.TransactionHistory{
		RefNo:     refNo,
		Status:    status,
		Type:      txType,
		Detail:    detail,
		CreatedAt: time.Now(),
		CreatedBy: actor,
		UpdatedAt: time.Now(),
		UpdatedBy: actor,
	}

	if err := db.Create(&history).Error; err != nil {
		return err
	}

	return nil
}
