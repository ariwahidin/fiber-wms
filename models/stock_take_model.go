package models

import (
	"gorm.io/gorm"
)

type StockTake struct {
	gorm.Model
	Code      string          `json:"code" gorm:"unique"`
	Status    string          `json:"status" gorm:"default:'open'"`
	CreatedBy int             `json:"created_by"`
	UpdatedBy int             `json:"updated_by"`
	DeletedBy int             `json:"deleted_by"`
	Items     []StockTakeItem `gorm:"foreignKey:StockTakeID;references:ID;constraint:OnDelete:CASCADE" json:"items"`
}

type StockTakeItem struct {
	gorm.Model
	StockTakeID  uint `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
	ItemID       uint
	InventoryID  uint
	Location     string
	Pallet       string
	Barcode      string
	SerialNumber string
	SystemQty    int
	CountedQty   int
	Difference   int
	Notes        string
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}

type StockTakeBarcode struct {
	gorm.Model
	StockTakeID uint   `gorm:"foreignKey:StockTakeID" json:"stock_take_id"`
	Barcode     string `json:"barcode"`
	Location    string `json:"location"`
	CountedQty  int    `json:"counted_qty"`
	Notes       string `json:"notes"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
