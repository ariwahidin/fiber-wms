package models

import (
	"time"

	"gorm.io/gorm"
)

// Reason codes untuk adjustment
const (
	AdjReasonDamaged    = "DMGD"      // Barang rusak
	AdjReasonExpired    = "EXPD"      // Kadaluarsa
	AdjReasonOpname     = "OPNAME"    // Hasil stock opname
	AdjReasonRecvErr    = "RECV_ERR"  // Receiving error
	AdjReasonSysErr     = "SYS_ERR"   // System correction
	AdjReasonShrinkage  = "SHRINK"    // Unknown loss / shrinkage
	AdjReasonFoundStock = "FOUND"     // Penemuan stok tidak tercatat
	AdjReasonProdLoss   = "PROD_LOSS" // Production / packing loss
)

// Status workflow adjustment
const (
	AdjStatusDraft    = "draft"
	AdjStatusPending  = "pending"
	AdjStatusApproved = "approved"
	AdjStatusRejected = "rejected"
	AdjStatusApplied  = "applied"
)

type InventoryAdjustment struct {
	gorm.Model
	AdjNumber   string `json:"adj_number" gorm:"uniqueIndex;size:50"`
	InventoryID uint   `json:"inventory_id" gorm:"index;not null"`
	OwnerCode   string `json:"owner_code" gorm:"size:20;index"`
	WhsCode     string `json:"whs_code" gorm:"size:20"`
	Location    string `json:"location" gorm:"size:100"`
	ItemID      uint   `json:"item_id"`
	ItemCode    string `json:"item_code" gorm:"size:50"`
	Barcode     string `json:"barcode" gorm:"size:100"`
	LotNumber   string `json:"lot_number" gorm:"size:100"`
	Uom         string `json:"uom" gorm:"size:20"`

	ReasonCode string `json:"reason_code" gorm:"size:20;not null"`
	Notes      string `json:"notes" gorm:"size:500"`

	QtyBefore float64 `json:"qty_before" gorm:"default:0"`
	QtyAdjust float64 `json:"qty_adjust" gorm:"default:0"` // bisa negatif (out) atau positif (in)
	QtyAfter  float64 `json:"qty_after" gorm:"default:0"`

	Status string `json:"status" gorm:"size:20;default:'draft';index"`

	RequestedBy int        `json:"requested_by"`
	RequestedAt time.Time  `json:"requested_at"`
	ApprovedBy  int        `json:"approved_by" gorm:"default:null"`
	ApprovedAt  *time.Time `json:"approved_at" gorm:"default:null"`
	RejectedBy  int        `json:"rejected_by" gorm:"default:null"`
	RejectedAt  *time.Time `json:"rejected_at" gorm:"default:null"`
	RejectNote  string     `json:"reject_note" gorm:"size:500"`
	AppliedAt   *time.Time `json:"applied_at" gorm:"default:null"`

	MovementID *uint `json:"movement_id" gorm:"default:null"` // link ke InventoryMovement setelah applied

	// Relasi
	Inventory Inventory `json:"inventory" gorm:"foreignKey:InventoryID"`
	UpdatedBy int       `json:"updated_by"`
}

func (a *InventoryAdjustment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.AdjNumber == "" {
		var count int64
		tx.Model(&InventoryAdjustment{}).Count(&count)
		now := time.Now()
		a.AdjNumber = "ADJ-" + now.Format("20060102") + "-" + padLeft(int(count+1), 4)
	}
	if a.RequestedAt.IsZero() {
		a.RequestedAt = time.Now()
	}
	if a.Status == "" {
		a.Status = AdjStatusDraft
	}
	return nil
}

func padLeft(n int, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s += "0"
	}
	ns := itoa(n)
	if len(ns) >= width {
		return ns
	}
	return s[len(ns):] + ns
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// AdjustmentReasonCode - master reason codes
type AdjustmentReasonCode struct {
	gorm.Model
	Code        string `json:"code" gorm:"uniqueIndex;size:20"`
	Description string `json:"description" gorm:"size:100"`
	Direction   string `json:"direction" gorm:"size:10"` // "in", "out", "both"
	RequireNote bool   `json:"require_note" gorm:"default:false"`
}
