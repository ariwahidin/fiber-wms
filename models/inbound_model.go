package models

import (
	"fiber-app/types"
	"time"

	"gorm.io/gorm"
)

type InboundHeader struct {
	gorm.Model
	InboundNo         string    `json:"inbound_no" gorm:"unique"`
	OwnerCode         string    `json:"owner_code" required:"required"`
	WhsCode           string    `json:"whs_code" required:"required"`
	ReceiptID         string    `json:"receipt_id" required:"required" gorm:"unique"`
	SupplierId        int       `json:"supplier_id"`
	Supplier          string    `json:"supplier"`
	Status            string    `json:"status" gorm:"default:'draft'"`
	RawStatus         string    `json:"raw_status" gorm:"default:'DRAFT'"`
	DraftTime         time.Time `json:"draft_time" gorm:"default:null"`
	ConfirmTime       time.Time `json:"confirm_time" gorm:"default:null"`
	ConfirmBy         int       `json:"confirm_by" gorm:"default:null"`
	CompleteTime      time.Time `json:"complete_time" gorm:"default:null"`
	CompleteBy        int       `json:"complete_by" gorm:"default:null"`
	ChangeToDraftTime time.Time `json:"change_to_draft_time" gorm:"default:null"`
	ChangeToDraftBy   int       `json:"change_to_draft_by" gorm:"default:null"`
	InboundDate       string    `json:"inbound_date"`
	Transporter       string    `json:"transporter"`
	Driver            string    `json:"driver"`
	TruckId           int       `json:"truck_id"`
	NoTruck           string    `json:"no_truck"`
	Type              string    `json:"type"`
	Container         string    `json:"container"`
	Origin            string    `json:"origin"`
	PoDate            string    `json:"po_date"`
	ArrivalTime       string    `json:"arrival_time"`
	StartUnloading    string    `json:"start_unloading"`
	EndUnloading      string    `json:"end_unloading"`
	TruckSize         string    `json:"truck_size"`
	BLNo              string    `json:"bl_no"`
	Koli              int       `json:"koli"`
	Remarks           string    `json:"remarks"`
	Integration       bool      `json:"integration" gorm:"default:false"`
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
	CheckingAt        *time.Time `json:"checking_at" gorm:"type:datetime"`
	CheckingBy        int
	CancelAt          *time.Time `json:"cancel_at" gorm:"type:datetime"`
	CancelBy          int
	PutawayAt         *time.Time `json:"putaway_at" gorm:"type:datetime"`
	PutawayBy         int
	CompleteAt        *time.Time `json:"complete_at" gorm:"type:datetime"`

	// Relations
	InboundReferences []InboundReference `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"references"`
	Details           []InboundDetail    `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"details"`
	Received          []InboundBarcode   `gorm:"foreignKey:InboundId;references:ID;constraint:OnDelete:CASCADE" json:"received"`
}

type InboundReference struct {
	gorm.Model
	InboundId uint   `json:"inbound_id" gorm:"default:null"`
	RefNo     string `json:"ref_no" gorm:"unique"`
}

type InboundDetail struct {
	gorm.Model
	OwnerCode     string  `json:"owner_code" required:"required"`
	WhsCode       string  `json:"whs_code" required:"required"`
	DivisionCode  string  `json:"division_code" required:"required" gorm:"default:REGULAR"`
	InboundId     int     `json:"inbound_id" gorm:"default:null"`
	InboundNo     string  `json:"inbound_no"`
	ItemId        uint    `json:"item_id" required:"required"`
	ProductNumber int     `json:"product_number"`
	ItemCode      string  `json:"item_code" required:"required"`
	Barcode       string  `json:"barcode"`
	Quantity      float64 `json:"quantity" required:"required"`
	RcvLocation   string  `json:"rcv_location"`
	QaStatus      string  `json:"qa_status" gorm:"default:'pending'"`
	Location      string  `json:"location" required:"required"`
	Status        string  `json:"status" gorm:"default:'draft'"`
	RecDate       string  `json:"rec_date" gorm:"default:null"`
	ProdDate      string  `json:"prod_date" gorm:"default:null"`
	ExpDate       string  `json:"exp_date" gorm:"default:null"`
	LotNumber     string  `json:"lot_number" gorm:"default:null"`
	CaseNumber    string  `json:"case_number" gorm:"default:null"`
	Uom           string  `json:"uom" required:"required"`
	IsSerial      string  `json:"is_serial"`
	SN            string  `json:"sn"`
	HandlingId    int     `json:"handling_id" required:"required"`
	HandlingUsed  string  `json:"handling_used"`
	TotalVas      int     `json:"total_vas"`
	Remarks       string  `json:"remarks"`
	RefId         int     `json:"ref_id"`
	RefNo         string  `json:"ref_no"`
	CreatedBy     int
	UpdatedBy     int
	DeletedBy     int
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
	ID              types.SnowflakeID `gorm:"primaryKey" json:"ID"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
	InboundId       int            `json:"inbound_id" gorm:"default:null"`
	InboundDetailId int            `gorm:"foreignKey:InboundDetailId" json:"inbound_detail_id"`
	ItemID          uint           `json:"item_id" required:"required"`
	Product         Product        `gorm:"foreignKey:ItemID;references:ID" json:"product"`
	ItemCode        string         `json:"item_code"`
	ScanType        string         `json:"scan_type"`
	ScanData        string         `json:"scan_data"`
	Barcode         string         `json:"barcode"`
	SerialNumber    string         `json:"serial_number"`
	Pallet          string         `json:"pallet"`
	Location        string         `json:"location"`
	PutawayLocation string         `json:"putaway_location"`
	PutawayQty      int            `json:"putaway_qty"`
	RecDate         string         `json:"rec_date" gorm:"default:null"`
	ProdDate        string         `json:"prod_date" gorm:"default:null"`
	ExpDate         string         `json:"exp_date" gorm:"default:null"`
	LotNumber       string         `json:"lot_number" gorm:"default:null"`
	CaseNumber      string         `json:"case_number" gorm:"default:null"`
	Quantity        float64        `json:"quantity"`
	Uom             string         `json:"uom"`
	WhsCode         string         `json:"whs_code"`
	OwnerCode       string         `json:"owner_code"`
	DivisionCode    string         `json:"division_code"`
	QaStatus        string         `json:"qa_status"`
	Status          string         `json:"status" gorm:"default:'pending'"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
	PutawayBy       int       `json:"putaway_by" gorm:"default:null"`
	PutawayAt       time.Time `json:"putaway_at" gorm:"default:null"`
}

// func (i *InboundBarcode) BeforeCreate(tx *gorm.DB) (err error) {
// 	fmt.Println("ID Inbound Barcode Before Create : ", i.ID)
// 	if i.ID == 0 {
// 		i.ID = types.SnowflakeID(idgen.GenerateID())
// 	}
// 	return nil
// }

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

type InboundDetailView struct {
	ID            uint   `json:"ID"`
	OwnerCode     string `json:"owner_code" required:"required"`
	WhsCode       string `json:"whs_code" required:"required"`
	DivisionCode  string `json:"division_code" required:"required"`
	InboundId     int    `json:"inbound_id" gorm:"default:null"`
	InboundNo     string `json:"inbound_no"`
	ItemId        int    `json:"item_id" required:"required"`
	ProductNumber int    `json:"product_number"`
	ItemCode      string `json:"item_code" required:"required"`
	Barcode       string `json:"barcode"`
	Quantity      int    `json:"quantity" required:"required"`
	RcvLocation   string `json:"rcv_location"`
	QaStatus      string `json:"qa_status" gorm:"default:'pending'"`
	Location      string `json:"location" required:"required"`
	Status        string `json:"status" gorm:"default:'draft'"`
	RecDate       string `json:"rec_date" gorm:"default:null"`
	ProdDate      string `json:"prod_date" gorm:"default:null"`
	ExpDate       string `json:"exp_date" gorm:"default:null"`
	LotNumber     string `json:"lot_number" gorm:"default:null"`
	Uom           string `json:"uom" required:"required"`
	IsSerial      string `json:"is_serial"`
	SN            string `json:"sn"`
	HandlingId    int    `json:"handling_id" required:"required"`
	HandlingUsed  string `json:"handling_used"`
	TotalVas      int    `json:"total_vas"`
	Remarks       string `json:"remarks"`
	RefId         int    `json:"ref_id"`
	RefNo         string `json:"ref_no"`
}
