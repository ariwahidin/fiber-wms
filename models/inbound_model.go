package models

import (
	"gorm.io/gorm"
)

type InboundHeader struct {
	gorm.Model
	Code            string `json:"code" gorm:"unique"`
	SupplierCode    string `json:"supplier_code" validate:"required" gorm:"not null"`
	Status          string `json:"status" gorm:"default:'draft'"`
	InboundDate     string `gorm:"type:date" json:"inbound_date"`
	InvoiceNo       string `json:"invoice"`
	Transporter     string `json:"transporter_code"`
	DriverName      string `json:"driver_name"`
	TruckSize       string `json:"truck_size"`
	TruckNo         string `json:"truck_no"`
	ContainerNo     string `json:"container_no"`
	BlNo            string `json:"bl_no"`
	PoNo            string `json:"po_no"`
	PoDate          string `gorm:"type:date" json:"po_date"`
	SjNo            string `json:"sj_no"`
	Origin          string `json:"origin"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	Remarks         string `json:"remarks_header"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
}

type InboundDetail struct {
	gorm.Model
	InboundId     int    `json:"inbound_id"`
	ReferenceCode string `json:"reference_code"`
	ItemCode      string `json:"item_code" required:"required"`
	Quantity      int    `json:"quantity" required:"required"`
	Status        string `json:"status" gorm:"default:'draft'"`
	WhsCode       string `json:"whs_code" required:"required"`
	RecDate       string `json:"rec_date" required:"required"`
	Uom           string `json:"uom" required:"required"`
	HandlingId    int    `json:"handling_id" required:"required"`
	HandlingUsed  string `json:"handling_used"`
	Remarks       string `json:"remarks"`
	CreatedBy     int
	UpdatedBy     int
	DeletedBy     int
}

type InboundDetailHandling struct {
	gorm.Model
	InboundDetailId   int
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

type DetailResponse struct {
	ID           uint    `json:"id"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	CBM          float64 `json:"cbm"`
	GMC          string  `json:"gmc"`
	Quantity     int     `json:"quantity"`
	WhsCode      string  `json:"whs_code"`
	RecDate      string  `json:"rec_date"`
	Uom          string  `json:"uom"`
	Remarks      string  `json:"remarks"`
	HandlingId   int     `json:"handling_id"`
	HandlingUsed string  `json:"handling_used"`
}
