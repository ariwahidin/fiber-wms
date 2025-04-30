package models

import (
	"gorm.io/gorm"
)

type ListOrderPart struct {
	gorm.Model
	OrderID          uint    `json:"order_id"`
	OrderNo          string  `json:"order_no"`
	OutboundID       uint    `json:"outbound_id"`
	OutboundDetailID uint    `json:"outbound_detail_id"`
	DeliveryNumber   string  `json:"delivery_number"`
	Status           string  `json:"status" gorm:"default:'open'"`
	ItemID           uint    `json:"item_id"`
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	Qty              int     `json:"qty"`
	CustomerID       uint    `json:"customer_id"`
	CustomerCode     string  `json:"customer_code"`
	CustomerName     string  `json:"customer_name"`
	ShipTo           string  `json:"ship_to"`
	ShipToName       string  `json:"ship_to_name"`
	ShipToAddress    string  `json:"ship_to_address"`
	ShipToCity       string  `json:"ship_to_city"`
	ShipToState      string  `json:"ship_to_state"`
	ShipToZipCode    string  `json:"ship_to_zip_code"`
	ShipToCountry    string  `json:"ship_to_country"`
	ShipToPhone      string  `json:"ship_to_phone"`
	ShipToEmail      string  `json:"ship_to_email"`
	DeliveryDate     string  `json:"delivery_date"`
	Volume           float64 `json:"volume"`
	CreatedBy        int     `json:"created_by"`
	UpdatedBy        int     `json:"updated_by"`
	DeletedBy        int     `json:"deleted_by"`
}

type OrderHeader struct {
	gorm.Model
	OrderNo   string `json:"order_no" gorm:"unique"`
	Status    string `json:"status" gorm:"default:'open'"`
	ShipMode  string `json:"ship_mode"`
	OrderType string `json:"order_type"`
	TruckerID uint   `json:"trucker_id"`
	Driver    string `json:"driver"`
	CreatedBy int    `json:"created_by"`
	UpdatedBy int    `json:"updated_by"`
	DeletedBy int    `json:"deleted_by"`
}

type OrderDetail struct {
	gorm.Model
	OrderID         uint    `json:"order_id"`
	ListOrderPartID uint    `json:"list_order_part_id"`
	OrderNo         string  `json:"order_no"`
	ShipTo          string  `json:"ship_to"`
	DestinationCity string  `json:"destination_city"`
	DeliveryNumber  string  `json:"delivery_number"`
	Qty             int     `json:"qty"`
	Volume          float64 `json:"volume"`
	UnitPrice       int     `json:"unit_price"`
	CreatedBy       int     `json:"created_by"`
	UpdatedBy       int     `json:"updated_by"`
	DeletedBy       int     `json:"deleted_by"`
}
