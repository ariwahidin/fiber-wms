package models

import "time"

// SuratJalanHeader represents one dispatch document under an SPK.
type SuratJalanHeader struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SJNo            string    `gorm:"size:40;not null;uniqueIndex" json:"sj_no"`
	OrderID         string    `gorm:"size:64;not null;index" json:"order_id"`
	OrderNo         string    `gorm:"size:50;not null;index" json:"order_no"`
	SJDate          time.Time `gorm:"not null" json:"sj_date"`
	Status          string    `gorm:"size:20;not null;index" json:"status"` // DRAFT, RELEASED, DELIVERED, CANCELLED
	Driver          string    `gorm:"size:150" json:"driver"`
	DriverPhone     string    `gorm:"size:50" json:"driver_phone"`
	TruckType       string    `gorm:"size:50" json:"truck_type"`
	TruckSize       string    `gorm:"size:50" json:"truck_size"`
	TruckNo         string    `gorm:"size:50" json:"truck_no"`
	TransporterCode string    `gorm:"size:50" json:"transporter_code"`
	TransporterName string    `gorm:"size:150" json:"transporter_name"`
	Remarks         string    `gorm:"type:nvarchar(max)" json:"remarks"`
	CreatedBy       int       `json:"created_by"`
	UpdatedBy       int       `json:"updated_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SuratJalanHeader) TableName() string { return "surat_jalan_headers" }

type SuratJalanDetail struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SJID            uint64    `gorm:"not null;index" json:"sj_id"`
	OrderID         uint64    `json:"order_id"`
	OrderDetailID   string    `gorm:"size:64;not null" json:"order_detail_id"`
	OutboundID      string    `gorm:"size:64;not null;index" json:"outbound_id"`
	OutboundNo      string    `gorm:"size:50;not null;index" json:"outbound_no"`
	ShipmentID      string    `gorm:"size:100" json:"shipment_id"`
	DeliveryTo      string    `gorm:"size:50" json:"delivery_to"`
	DeliveryToName  string    `gorm:"size:200" json:"delivery_to_name"`
	DeliveryAddress string    `gorm:"type:nvarchar(500)" json:"delivery_address"`
	DeliveryCity    string    `gorm:"size:100" json:"delivery_city"`
	CreatedAt       time.Time `json:"created_at"`
}

func (SuratJalanDetail) TableName() string { return "surat_jalan_details" }

type SuratJalanPackage struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID           uint64     `json:"order_id"`
	SJDetailID        uint64     `gorm:"not null" json:"sj_detail_id"`
	OutboundBarcodeID uint64     `gorm:"not null;uniqueIndex" json:"outbound_barcode_id"`
	OutboundID        uint64     `json:"outbound_id"`
	ItemID            uint64     `json:"item_id"`
	Quantity          float64    `gorm:"type:decimal(18,4)" json:"quantity"`
	IsActive          bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
	ReleasedAt        *time.Time `json:"released_at,omitempty"`
}

func (SuratJalanPackage) TableName() string { return "surat_jalan_packages" }
