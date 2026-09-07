package models

import (
	"time"

	"gorm.io/gorm"
)

type StockTakeBatch struct {
	gorm.Model

	Code        string     `json:"code" gorm:"unique;not null"`
	OwnerCode   string     `json:"owner_code"`
	Description string     `json:"description"`
	Status      string     `json:"status" gorm:"default:'open'"`
	CreatedBy   int        `json:"created_by"`
	UpdatedBy   int        `json:"updated_by"`
	StartedAt   *time.Time `json:"started_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	ClosedBy    *int       `json:"closed_by"`
	CancelAt    *time.Time `json:"cancel_at"`
	CancelBy    *int       `json:"cancel_by"`

	StockTakes []StockTake `gorm:"foreignKey:BatchID;references:ID;constraint:OnDelete:CASCADE" json:"stock_takes"`
}

type StockTake struct {
	gorm.Model
	Code      string          `json:"code" gorm:"unique"`
	Status    string          `json:"status" gorm:"default:'open'"`
	CreatedBy int             `json:"created_by"`
	UpdatedBy int             `json:"updated_by"`
	DeletedBy int             `json:"deleted_by"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	StartedAt time.Time       `json:"started_at" gorm:"default:NULL"`
	ClosedAt  time.Time       `json:"closed_at" gorm:"default:NULL"`
	ClosedBy  int             `json:"closed_by" gorm:"default:NULL"`
	CancelAt  time.Time       `json:"cancel_at" gorm:"default:NULL"`
	CancelBy  int             `json:"cancel_by" gorm:"default:NULL"`
	Items     []StockTakeItem `gorm:"foreignKey:StockTakeID;references:ID;constraint:OnDelete:CASCADE" json:"items"`
	BatchID   *uint           `json:"batch_id"`
	Batch     *StockTakeBatch `gorm:"foreignKey:BatchID;references:ID" json:"batch"`
}

type StockTakeItem struct {
	gorm.Model
	StockTakeID  uint   `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
	ItemID       int64  `json:"item_id"`
	InventoryID  int64  `json:"inventory_id"`
	Location     string `json:"location"`
	Pallet       string `json:"pallet"`
	Barcode      string `json:"barcode"`
	CartonNumber string `json:"carton_number"`
	LotNumber    string `json:"lot_number"`
	DivisionCode string `json:"division_code"`
	OwnerCode    string `json:"owner_code"`
	SystemQty    int    `json:"system_qty"`
	CountedQty   int    `json:"counted_qty"`
	Difference   int    `json:"difference"`
	Notes        string `json:"notes"`
	CreatedBy    int    `json:"created_by"`
	UpdatedBy    int    `json:"updated_by"`
	DeletedBy    int    `json:"deleted_by"`
}

type StockTakeBarcode struct {
	gorm.Model
	StockTakeID  uint    `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
	DivisionCode string  `json:"division_code"`
	Location     string  `json:"location"`
	Barcode      string  `json:"barcode"`
	Sku          string  `json:"sku"`
	LotNumber    string  `json:"lot_number"`
	CartonNumber string  `json:"carton_number"`
	CountedQty   int     `json:"counted_qty"`
	QrRaw        string  `json:"qr_raw" gorm:"type:nvarchar(1000)"`
	ItemID       uint    `json:"item_id"`
	Item         Product `json:"item" gorm:"foreignKey:ItemID"`
	CreatedBy    int     `json:"created_by"`
	UpdatedBy    int     `json:"updated_by"`
	DeletedBy    int     `json:"deleted_by"`
}

type StockCardFilter struct {
	FromRow   string `json:"fromRow"`
	ToRow     string `json:"toRow"`
	FromBay   string `json:"fromBay"`
	ToBay     string `json:"toBay"`
	FromLevel string `json:"fromLevel"`
	ToLevel   string `json:"toLevel"`
	FromBin   string `json:"fromBin"`
	ToBin     string `json:"toBin"`
	Area      string `json:"area"`
}
