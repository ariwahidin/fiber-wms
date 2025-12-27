package models

import (
	"fiber-app/types"
	"time"

	"gorm.io/gorm"
)

type OutboundHeader struct {
	gorm.Model
	ID                uint      `json:"ID"`
	OutboundNo        string    `json:"outbound_no" gorm:"unique"`
	OutboundDate      string    `json:"outbound_date"`
	OwnerCode         string    `json:"owner_code" validate:"required" gorm:"not null"`
	ShipmentID        string    `json:"shipment_id" gorm:"unique"`
	CustomerCode      string    `json:"customer_code"`
	WhsCode           string    `json:"whs_code"`
	Status            string    `json:"status" gorm:"default:'draft'"`
	RawStatus         string    `json:"raw_status" gorm:"default:'DRAFT'"`
	DraftTime         time.Time `json:"draft_time" gorm:"default:null"`
	ConfirmTime       time.Time `json:"confirm_time" gorm:"default:null"`
	ConfirmBy         int       `json:"confirm_by" gorm:"default:null"`
	CompleteTime      time.Time `json:"complete_time" gorm:"default:null"`
	CompleteBy        int       `json:"complete_by" gorm:"default:null"`
	ChangeToDraftTime time.Time `json:"change_to_draft_time" gorm:"default:null"`
	ChangeToDraftBy   int       `json:"change_to_draft_by"`
	User_Def1         string    `json:"user_def1"`
	User_Def2         string    `json:"user_def2"`
	User_Def3         string    `json:"user_def3"`
	User_Def4         string    `json:"user_def4"`
	User_Def5         string    `json:"user_def5"`
	Remarks           string    `json:"remarks"`
	PickerName        string    `json:"picker_name"`
	CustAddress       string    `json:"cust_address"`
	CustCity          string    `json:"cust_city"`
	PlanPickupDate    string    `json:"plan_pickup_date"`
	PlanPickupTime    string    `json:"plan_pickup_time"`
	RcvDoDate         string    `json:"rcv_do_date"`
	RcvDoTime         string    `json:"rcv_do_time"`
	StartPickTime     string    `json:"start_pick_time"`
	EndPickTime       string    `json:"end_pick_time"`
	DelivTo           string    `json:"deliv_to"`
	DelivAddress      string    `json:"deliv_address"`
	DelivCity         string    `json:"deliv_city"`
	Driver            string    `json:"driver"`
	QtyKoli           int       `json:"qty_koli"`
	QtyKoliSeal       int       `json:"qty_koli_seal"`
	TruckSize         string    `json:"truck_size"`
	TruckNo           string    `json:"truck_no"`
	TransporterCode   string    `json:"transporter_code"`
	Integration       bool      `json:"integration" gorm:"default:false"`
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
	OutboundDetails   []OutboundDetail `gorm:"foreignKey:OutboundID;references:ID;constraint:OnDelete:CASCADE" json:"items"`
}

type OutboundDetail struct {
	gorm.Model
	OutboundID   uint    `json:"outbound_id" gorm:"default:null"`
	OutboundNo   string  `json:"outbound_no"`
	CustomerCode string  `json:"customer_code"`
	OwnerCode    string  `json:"owner_code" required:"required"`
	WhsCode      string  `json:"whs_code" required:"required"`
	DivisionCode string  `json:"division_code" required:"required" gorm:"default:'REGULAR'"`
	ItemID       int     `json:"item_id"`
	ItemCode     string  `json:"item_code" required:"required"`
	Barcode      string  `json:"barcode"`
	Quantity     float64 `json:"quantity" required:"required"`
	Pallet       string  `json:"pallet"`
	RecDate      string  `json:"rec_date" gorm:"default:null"`
	ExpDate      string  `json:"exp_date" gorm:"default:null"`
	LotNumber    string  `json:"lot_number" gorm:"default:null"`
	ScanQty      int     `json:"scan_qty" gorm:"default:0"`
	Location     string  `json:"location" required:"required"`
	Status       string  `json:"status" gorm:"default:'draft'"`
	Uom          string  `json:"uom" required:"required"`
	QaStatus     string  `json:"qa_status" gorm:"default:'A'"`
	SN           string  `json:"sn"`
	SNCheck      string  `json:"sn_check" gorm:"default:'N'"`
	VasID        int     `json:"vas_id"`
	VasName      string  `json:"vas_name"`
	VasPrice     float64 `json:"vas_price"`
	Remarks      string  `json:"remarks"`
	FileName     string  `json:"file_name"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int

	// Relationship
	Product                 Product                  `gorm:"foreignKey:ItemID;references:ID" json:"product"`
	OutboundDetailHandlings []OutboundDetailHandling `gorm:"foreignKey:OutboundDetailId;references:ID;constraint:OnDelete:CASCADE" json:"outbound_detail_handlings"`
	OutboundPickings        []OutboundPicking        `gorm:"foreignKey:OutboundDetailId;references:ID;constraint:OnDelete:CASCADE" json:"picking_sheets"`
	Handling                []OutboundDetailHandling `gorm:"foreignKey:OutboundDetailId;references:ID;" json:"handling"`
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
	OutboundID       types.SnowflakeID `json:"outbound_id" gorm:"default:null"`
	OutboundNo       string            `json:"outbound_no"`
	OutboundDetailId int               `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	ItemCode         string            `json:"item_code"`
	HandlingId       int
	HandlingUsed     string
	RateIdr          int
	IsKoli           bool
	QtyHandling      int
	TotalPrice       int
	CreatedBy        int
	UpdatedBy        int
	DeletedBy        int
}

type OutboundHandling struct {
	gorm.Model
	OutboundID       types.SnowflakeID `json:"outbound_id" gorm:"default:null"`
	OutboundNo       string            `json:"outbound_no"`
	OutboundDetailId int               `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	ItemCode         string            `json:"item_code"`
	Quantity         int               `json:"quantity" gorm:"default:0"`
	Koli             int               `json:"koli" gorm:"default:0"`
	CreatedBy        int
	UpdatedBy        int
	DeletedBy        int
}

type OutboundPicking struct {
	gorm.Model
	InventoryID      int     `json:"inventory_id"`
	OutboundId       uint    `json:"outbound_id"`
	OutboundNo       string  `json:"outbound_no"`
	OutboundDetailId int     `gorm:"foreignKey:OutboundDetailId" json:"outbound_detail_id"`
	OwnerCode        string  `json:"owner_code"`
	WhsCode          string  `json:"whs_code"`
	DivisionCode     string  `json:"division_code" gorm:"default:'REGULAR'"`
	ItemID           int     `json:"item_id"`
	ItemCode         string  `json:"item_code"`
	Barcode          string  `json:"barcode"`
	Pallet           string  `json:"pallet"`
	Location         string  `json:"location"`
	Quantity         float64 `json:"quantity"`
	QaStatus         string  `json:"qa_status"`
	RecDate          string  `json:"rec_date"`
	ExpDate          string  `json:"exp_date"`
	ProdDate         string  `json:"prod_date"`
	LotNumber        string  `json:"lot_number"`
	Uom              string  `json:"uom"`
	Reason           string  `json:"reason"`
	QtyDisplay       float64 `json:"qty_display"`
	UomDisplay       string  `json:"uom_display"`
	EanDisplay       string  `json:"ean_display"`
	CreatedBy        int
	UpdatedBy        int
	DeletedBy        int
}

type OutboundBarcode struct {
	gorm.Model
	ID               uint `json:"ID"`
	PackingId        uint
	PackingNo        string  `json:"packing_no" gorm:"size:50"` // sama persis dengan parent
	InventoryID      int     `json:"inventory_id"`
	OutboundId       uint    `json:"outbound_id"`
	OutboundNo       string  `json:"outbound_no"`
	OutboundDetailId int     `json:"outbound_detail_id"`
	PickingSheetId   int     `json:"picking_sheet_id" gorm:"default:0"`
	ItemID           int     `json:"item_id"`
	ItemCode         string  `json:"item_code"`
	Barcode          string  `json:"barcode"`
	Uom              string  `json:"uom"`
	SerialNumber     string  `json:"serial_number"`
	Quantity         float64 `json:"quantity"`
	Status           string  `json:"status" gorm:"default:'pending'"`
	BarcodeDataScan  string  `json:"barcode_data_scan"` // data barcode yang di scan
	QtyDataScan      float64 `json:"qty_data_scan"`     // data qty yang di scan
	LocationScan     string  `json:"location_scan"`
	UomScan          string  `json:"uom_scan"`
	IsSerial         bool    `json:"is_serial"`
	CreatedBy        int
	UpdatedBy        int
	DeletedBy        int

	OutboundHeader OutboundHeader `json:"Outbound" gorm:"foreignKey:OutboundId;references:ID"`
}

type OutboundPacking struct {
	gorm.Model
	PackingNo string
	CreatedBy int
	UpdatedBy int
	DeletedBy int
	// Orders    []OutboundBarcode `json:"orders" gorm:"foreignKey:PackingId;references:ID"`
}

type OutboundVas struct {
	gorm.Model
	OutboundID   int     `json:"outbound_id"`
	OutboundNo   string  `json:"outbound_no"`
	OutboundDate string  `json:"outbound_date"`
	MainVasName  string  `json:"main_vas_name"`
	IsKoli       bool    `json:"is_koli"`
	DefaultPrice float64 `json:"default_price"`
	QtyItem      int     `json:"qty_item"`
	QtyKoli      int     `json:"qty_koli"`
	TotalPrice   float64 `json:"total_price"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}

// type OrderHeader struct {
// 	gorm.Model
// 	ID              types.SnowflakeID `json:"ID"`
// }
