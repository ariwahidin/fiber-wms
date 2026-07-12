package models

import (
	"time"

	"gorm.io/gorm"
)

type StockTake struct {
	gorm.Model
	Code      string          `json:"code" gorm:"unique"`
	Status    string          `json:"status" gorm:"default:'open'"`
	CreatedBy int             `json:"created_by"`
	UpdatedBy int             `json:"updated_by"`
	DeletedBy int             `json:"deleted_by"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Items     []StockTakeItem `gorm:"foreignKey:StockTakeID;references:ID;constraint:OnDelete:CASCADE" json:"items"`
}

// type StockTakeItem struct {
// 	gorm.Model
// 	StockTakeID  uint `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
// 	ItemID       int64
// 	InventoryID  int64
// 	Location     string
// 	Pallet       string
// 	Barcode      string
// 	SerialNumber string
// 	SystemQty    int
// 	CountedQty   int
// 	Difference   int
// 	Notes        string
// 	CreatedBy    int
// 	UpdatedBy    int
// 	DeletedBy    int
// }

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

// type StockTakeBarcode struct {
// 	gorm.Model
// 	StockTakeID uint   `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
// 	Barcode     string `json:"barcode"`
// 	Location    string `json:"location"`
// 	CountedQty  int    `json:"counted_qty"`
// 	Notes       string `json:"notes"`
// 	CreatedBy   int
// 	UpdatedBy   int
// 	DeletedBy   int
// }

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
