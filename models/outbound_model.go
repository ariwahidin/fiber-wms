package models

import (
	"gorm.io/gorm"
)

type OutboundHeader struct {
	gorm.Model
	OutboundNo   string `json:"outbound_no" gorm:"unique"`
	DeliveryNo   string `json:"delivery_no"`
	CustomerCode string `json:"customer_code" validate:"required"`
	CustomerName string `json:"customer_name"`
	OwnerCode    string `json:"owner_code" validate:"required" gorm:"not null"`
	Status       string `json:"status" gorm:"default:'draft'"`
	OutboundDate string `gorm:"type:date" json:"outbound_date"`
	User_Def1    string `json:"user_def1"`
	User_Def2    string `json:"user_def2"`
	User_Def3    string `json:"user_def3"`
	User_Def4    string `json:"user_def4"`
	User_Def5    string `json:"user_def5"`
	Remarks      string `json:"remarks_header"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int

	OutboundDetails []OutboundDetail `gorm:"foreignKey:OutboundID;references:ID;constraint:OnDelete:CASCADE" json:"outbound_details"`
}

type OutboundDetail struct {
	gorm.Model
	OutboundID   int    `json:"outbound_id" gorm:"default:null"`
	OutboundNo   string `json:"outbound_no"`
	CustomerCode string `json:"customer_code"`
	OwnerCode    string `json:"owner_code" required:"required"`
	LineNumber   string `json:"line_number"`
	ItemID       int    `json:"item_id"`
	ItemCode     string `json:"item_code" required:"required"`
	Quantity     int    `json:"quantity" required:"required"`
	Location     string `json:"location" required:"required"`
	Status       string `json:"status" gorm:"default:'draft'"`
	WhsCode      string `json:"whs_code" required:"required"`
	Uom          string `json:"uom" required:"required"`
	HandlingId   int    `json:"handling_id" required:"required"`
	HandlingUsed string `json:"handling_used"`
	Remarks      string `json:"remarks"`
	FileName     string `json:"file_name"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int

	OutboundDetailHandlings []OutboundDetailHandling `gorm:"foreignKey:OutboundDetailId;references:ID;constraint:OnDelete:CASCADE" json:"outbound_detail_handlings"`
	OutboundBarcodes        []OutboundBarcode        `gorm:"foreignKey:OutboundDetailId;references:ID;constraint:OnDelete:CASCADE" json:"outbound_barcodes"`
}

type OutboundFile struct {
	gorm.Model
	DeliveryNo   string `json:"delivery_no"`
	LineNo       string `json:"line_no"`
	CustomerCode string `json:"customer_code"`
	CustomerName string `json:"customer_name"`
	Barcode      string `json:"barcode"`
	ItemCode     string `json:"item_code"`
	Quantity     int    `json:"quantity"`
	Uom          string `json:"uom"`
	OwnerCode    string `json:"owner_code"`
	QaStatus     string `json:"status"`
	User_Def1    string `json:"user_def1"`
	User_Def2    string `json:"user_def2"`
	User_Def3    string `json:"user_def3"`
	User_Def4    string `json:"user_def4"`
	User_Def5    string `json:"user_def5"`
	FileName     string `json:"file_name"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}

type OutboundDetailHandling struct {
	gorm.Model
	OutboundDetailId  int `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	HandlingCombineId int
	HandlingId        int
	HandlingUsed      string
	OriginHandlingId  int
	OriginHandling    string
	RateId            int
	RateIdr           int
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
}

type PickingSheet struct {
	gorm.Model
	InventoryID       int    `json:"inventory_id"`
	InventoryDetailID int    `json:"inventory_detail_id"`
	OutboundId        int    `json:"outbound_id"`
	OutboundDetailId  int    `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	ItemID            int    `json:"item_id"`
	ItemCode          string `json:"item_code"`
	ScanType          string `json:"scan_type"`
	ScanData          string `json:"scan_data"`
	Barcode           string `json:"barcode"`
	SerialNumber      string `json:"serial_number"`
	Location          string `json:"location"`
	Quantity          int    `json:"quantity"`
	WhsCode           string `json:"whs_code"`
	QaStatus          string `json:"qa_status"`
	Status            string `json:"status" gorm:"default:'pending'"`
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
}

type OutboundBarcode struct {
	gorm.Model
	InvetoryID        int    `json:"inventory_id"`
	InventoryDetailID int    `json:"inventory_detail_id"`
	OutboundId        int    `json:"outbound_id"`
	OutboundDetailId  int    `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	SeqBox            int    `json:"seq_box"`
	ItemID            int    `json:"item_id"`
	ItemCode          string `json:"item_code"`
	ScanType          string `json:"scan_type"`
	ScanData          string `json:"scan_data"`
	Barcode           string `json:"barcode"`
	SerialNumber      string `json:"serial_number"`
	Location          string `json:"location"`
	Quantity          int    `json:"quantity"`
	WhsCode           string `json:"whs_code"`
	QaStatus          string `json:"qa_status"`
	Status            string `json:"status" gorm:"default:'pending'"`
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
}
