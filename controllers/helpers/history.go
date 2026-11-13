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
	MovementID  string

	RefType string
	RefID   uint

	ItemID   uint
	ItemCode string

	QtyOnhandChange    float64
	QtyAvailableChange float64
	QtyAllocatedChange float64
	QtySuspendChange   float64
	QtyShippedChange   float64

	FromDivision string
	ToDivision   string

	FromProdDate string
	ToProdDate   string

	FromLotNumber string
	ToLotNumber   string

	FromPallet string
	ToPallet   string

	FromWhsCode  string
	ToWhsCode    string
	FromLocation string
	ToLocation   string
	OldQaStatus  string
	NewQaStatus  string

	Reason    string
	CreatedBy int

	ActionType string
}

func InsertInventoryMovement(
	tx *gorm.DB,
	p InventoryMovementPayload,
) error {

	movement := models.InventoryMovement{
		InventoryID:        p.InventoryID,
		MovementID:         p.MovementID,
		RefType:            p.RefType,
		RefID:              p.RefID,
		ItemID:             p.ItemID,
		ItemCode:           p.ItemCode,
		QtyOnhandChange:    p.QtyOnhandChange,
		QtyAvailableChange: p.QtyAvailableChange,
		QtyAllocatedChange: p.QtyAllocatedChange,
		QtySuspendChange:   p.QtySuspendChange,
		QtyShippedChange:   p.QtyShippedChange,
		FromWhsCode:        p.FromWhsCode,
		ToWhsCode:          p.ToWhsCode,
		FromLocation:       p.FromLocation,
		ToLocation:         p.ToLocation,

		FromPallet: p.FromPallet,
		ToPallet:   p.ToPallet,

		OldQaStatus:   p.OldQaStatus,
		NewQaStatus:   p.NewQaStatus,
		FromDivision:  p.FromDivision,
		ToDivision:    p.ToDivision,
		FromLotNumber: p.FromLotNumber,
		ToLotNumber:   p.ToLotNumber,
		FromProdDate:  p.FromProdDate,
		ToProdDate:    p.ToProdDate,
		Reason:        p.Reason,
		ActionType:    p.ActionType,
		CreatedBy:     p.CreatedBy,
		CreatedAt:     time.Now(),
	}

	return tx.Create(&movement).Error
}
