package models

import (
	"gorm.io/gorm"
)

// Inventory: Tabel utama yang menyimpan informasi stok barang
type Inventory struct {
	gorm.Model
	InboundDetailId  int    `json:"inbound_detail_id"`
	InboundBarcodeId int    `json:"inbound_barcode_id"`
	RecDate          string `json:"rec_date"`
	Owner            string `json:"owner"`
	WhsCode          string `json:"whs_code"`
	Pallet           string `json:"pallet"`
	Location         string `json:"location"`
	ItemId           int    `json:"item_id"`
	ItemCode         string `json:"item_code"`
	Barcode          string `json:"barcode" gorm:"not null" validate:"required"`
	SerialNumber     string `json:"serial_number"`
	QaStatus         string `json:"qa_status"`
	Uom              string `json:"uom"`
	QtyOrigin        int    `json:"qty_origin" gorm:"default:0"`
	QtyOnhand        int    `json:"qty_onhand" gorm:"default:0"`
	QtyAvailable     int    `json:"qty_available" gorm:"default:0"`
	QtyAllocated     int    `json:"qty_allocated" gorm:"default:0"`
	QtySuspend       int    `json:"qty_suspend" gorm:"default:0"`
	QtyShipped       int    `json:"qty_shipped" gorm:"default:0"`
	Trans            string `json:"trans"`
	CreatedBy        int
	UpdatedBy        int
	DeletedBy        int
}

// InventoryDetail: Detail dari setiap item di dalam inventory
// type InventoryDetail struct {
// 	gorm.Model
// 	InventoryId     int    `gorm:"foreignKey:InventoryId;references:ID" json:"inventory_id"`
// 	Location        string `json:"location"`
// 	InboundDetailId int    `json:"inbound_detail_id"`
// 	SerialNumber    string `json:"serial_number"`
// 	Quantity        int    `json:"quantity"`
// 	QaStatus        string `json:"qa_status"`
// 	CreatedBy       int
// 	UpdatedBy       int
// 	DeletedBy       int
// }
