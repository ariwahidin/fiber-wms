package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type OutboundScan struct {
	gorm.Model
	NoKoli     string               `json:"no_koli"`
	OutboundID types.SnowflakeID    `json:"outbound_id"`
	CreatedBy  int                  `json:"created_by"`
	UpdatedBy  int                  `json:"updated_by"`
	DeletedBy  int                  `json:"deleted_by"`
	Details    []OutboundScanDetail `gorm:"foreignKey:KoliID;references:ID;constraint:OnDelete:CASCADE" json:"details"`
}

type OutboundScanDetail struct {
	gorm.Model
	KoliID           int    `json:"koli_id"`
	NoKoli           string `json:"no_koli"`
	OutboundID       int    `json:"outbound_id"`
	OutboundDetailID int    `json:"outbound_detail_id"`
	InventoryID      int    `json:"inventory_id"`
	ItemID           int    `json:"item_id"`
	PickingSheetID   int    `json:"picking_sheet_id"`
	ItemCode         string `json:"item_code"`
	Barcode          string `json:"barcode"`
	SerialNumber     string `json:"serial_number"`
	Qty              int    `json:"qty"`
	CreatedBy        int    `json:"created_by"`
	UpdatedBy        int    `json:"updated_by"`
	DeletedBy        int    `json:"deleted_by"`
}
