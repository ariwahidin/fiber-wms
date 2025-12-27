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

type InventoryMovementPayload struct {
	InventoryID uint

	RefType string
	RefID   uint

	QtyOnhandChange    float64
	QtyAvailableChange float64
	QtyAllocatedChange float64
	QtySuspendChange   float64
	QtyShippedChange   float64

	FromWhsCode  string
	ToWhsCode    string
	FromLocation string
	ToLocation   string
	OldQaStatus  string
	NewQaStatus  string

	Reason    string
	CreatedBy int
}

func InsertInventoryMovement(
	tx *gorm.DB,
	p InventoryMovementPayload,
) error {

	movement := models.InventoryMovement{
		InventoryID:        p.InventoryID,
		RefType:            p.RefType,
		RefID:              p.RefID,
		QtyOnhandChange:    p.QtyOnhandChange,
		QtyAvailableChange: p.QtyAvailableChange,
		QtyAllocatedChange: p.QtyAllocatedChange,
		QtySuspendChange:   p.QtySuspendChange,
		QtyShippedChange:   p.QtyShippedChange,
		FromWhsCode:        p.FromWhsCode,
		ToWhsCode:          p.ToWhsCode,
		FromLocation:       p.FromLocation,
		ToLocation:         p.ToLocation,
		OldQaStatus:        p.OldQaStatus,
		NewQaStatus:        p.NewQaStatus,
		Reason:             p.Reason,
		CreatedBy:          p.CreatedBy,
		CreatedAt:          time.Now(),
	}

	return tx.Create(&movement).Error
}
