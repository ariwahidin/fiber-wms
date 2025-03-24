package models

import (
	"gorm.io/gorm"
)

// Inventory: Tabel utama yang menyimpan informasi stok barang
type Inventory struct {
	gorm.Model
	InboundDetailId int    `json:"inbound_detail_id"`
	ItemId          int    `json:"item_id"`
	ItemCode        string `json:"item_code"`
	WhsCode         string `json:"whs_code"`
	Quantity        int    `json:"quantity"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int

	// Relasi One-to-Many: Inventory ke InventoryDetail
	Details []InventoryDetail `gorm:"foreignKey:InventoryId;references:ID;constraint:OnDelete:CASCADE" json:"details"`
}

// InventoryDetail: Detail dari setiap item di dalam inventory
type InventoryDetail struct {
	gorm.Model
	InventoryId     int    `gorm:"foreignKey:InventoryId;references:ID" json:"inventory_id"`
	Location        string `json:"location"`
	InboundDetailId int    `json:"inbound_detail_id"`
	SerialNumber    string `json:"serial_number"`
	Quantity        int    `json:"quantity"`
	QaStatus        string `json:"qa_status"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
}
