package models

import (
	"fiber-app/controllers/idgen"

	"gorm.io/gorm"
)

type InboundHeader struct {
	gorm.Model
	ID              int64  `json:"id" gorm:"primary_key"`
	InboundNo       string `json:"inbound_no" gorm:"unique"`
	SupplierId      int    `json:"supplier_id"`
	Supplier        string `json:"supplier"`
	Status          string `json:"status" gorm:"default:'draft'"`
	InboundDate     string `json:"inbound_date"`
	Transporter     string `json:"transporter"`
	Driver          string `json:"driver"`
	TruckId         int    `json:"truck_id"`
	NoTruck         string `json:"no_truck"`
	Type            string `json:"type"`
	Container       string `json:"container"`
	PoNumber        string `json:"po_number"`
	Invoice         string `json:"invoice"`
	PoDate          string `gorm:"type:date" json:"po_date"`
	OriginId        int    `json:"origin_id"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	Remarks         string `json:"remarks_header"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int

	// Relations
	InboundReferences []InboundReference `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"references"`
	Details           []InboundDetail    `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"details"`
	Received          []InboundBarcode   `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"received"`
}

func (u *InboundHeader) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = idgen.GenerateID()
	return
}

type InboundReference struct {
	gorm.Model
	InboundId uint   `json:"inbound_id" gorm:"default:null"`
	RefNo     string `json:"ref_no" gorm:"unique"`
}

type InboundDetail struct {
	gorm.Model
	InboundId    uint   `json:"inbound_id" gorm:"default:null"`
	InboundNo    string `json:"inbound_no"`
	ItemId       int    `json:"item_id"`
	ItemCode     string `json:"item_code" required:"required"`
	Barcode      string `json:"barcode"`
	Quantity     int    `json:"quantity" required:"required"`
	Location     string `json:"location" required:"required"`
	Status       string `json:"status" gorm:"default:'draft'"`
	WhsCode      string `json:"whs_code" required:"required"`
	RecDate      string `json:"rec_date" required:"required"`
	Uom          string `json:"uom" required:"required"`
	IsSerial     string `json:"is_serial"`
	HandlingId   int    `json:"handling_id" required:"required"`
	HandlingUsed string `json:"handling_used"`
	TotalVas     int    `json:"total_vas"`
	Remarks      string `json:"remarks"`
	RefId        int    `json:"ref_id"`
	RefNo        string `json:"ref_no"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}

type InboundDetailHandling struct {
	gorm.Model
	InboundDetailId   int `gorm:"foreignKey:InboundDetailId" json:"inbound_detail_id"`
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

type InboundBarcode struct {
	gorm.Model
	InboundId       int    `json:"inbound_id" gorm:"default:null"`
	InboundDetailId int    `gorm:"foreignKey:InboundDetailId" json:"inbound_detail_id"`
	ItemID          int    `json:"item_id"`
	ItemCode        string `json:"item_code"`
	ScanType        string `json:"scan_type"`
	ScanData        string `json:"scan_data"`
	Barcode         string `json:"barcode"`
	SerialNumber    string `json:"serial_number"`
	Pallet          string `json:"pallet"`
	Location        string `json:"location"`
	Quantity        int    `json:"quantity"`
	WhsCode         string `json:"whs_code"`
	QaStatus        string `json:"qa_status"`
	Status          string `json:"status" gorm:"default:'pending'"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
}

type FormItemInbound struct {
	InboundDetailID int    `json:"inbound_detail_id"`
	InboundID       int    `json:"inbound_id"`
	InboundNo       string `json:"inbound_no"`
	ItemID          int    `json:"item_id" validate:"required"`
	ItemName        string `json:"item_name"`
	Barcode         string `json:"barcode"`
	ItemCode        string `json:"item_code"`
	Quantity        int    `json:"quantity" validate:"required,min=1" `
	Uom             string `json:"uom"`
	RecDate         string `json:"rec_date"`
	WhsCode         string `json:"whs_code"`
	HandlingID      int    `json:"handling_id"`
	HandlingUsed    string `json:"handling_used"`
	Remarks         string `json:"remarks"`
	Location        string `json:"location"`
	TotalVas        int    `json:"total_vas"`
}
