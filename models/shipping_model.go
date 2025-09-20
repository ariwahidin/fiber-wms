package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type ListOrderPart struct {
	gorm.Model
	OrderID          uint              `json:"order_id"`
	OrderNo          string            `json:"order_no"`
	OutboundID       types.SnowflakeID `json:"outbound_id"`
	OutboundDetailID uint              `json:"outbound_detail_id"`
	DeliveryNumber   string            `json:"delivery_number"`
	Status           string            `json:"status" gorm:"default:'open'"`
	ItemID           uint              `json:"item_id"`
	ItemCode         string            `json:"item_code"`
	ItemName         string            `json:"item_name"`
	Qty              int               `json:"qty"`
	CustomerID       uint              `json:"customer_id"`
	CustomerCode     string            `json:"customer_code"`
	CustomerName     string            `json:"customer_name"`
	ShipTo           string            `json:"ship_to"`
	ShipToName       string            `json:"ship_to_name"`
	ShipToAddress    string            `json:"ship_to_address"`
	ShipToCity       string            `json:"ship_to_city"`
	ShipToState      string            `json:"ship_to_state"`
	ShipToZipCode    string            `json:"ship_to_zip_code"`
	ShipToCountry    string            `json:"ship_to_country"`
	ShipToPhone      string            `json:"ship_to_phone"`
	ShipToEmail      string            `json:"ship_to_email"`
	DeliveryDate     string            `json:"delivery_date"`
	Volume           float64           `json:"volume"`
	CreatedBy        int               `json:"created_by"`
	UpdatedBy        int               `json:"updated_by"`
	DeletedBy        int               `json:"deleted_by"`
}

type OrderHeader struct {
	gorm.Model
	OrderNo         string        `json:"order_no" gorm:"unique"`
	Status          string        `json:"status" gorm:"default:'open'"`
	OrderDate       string        `json:"order_date"`
	DeliveryDate    string        `json:"delivery_date"`
	LoadDate        string        `json:"load_date"`
	OrderType       string        `json:"order_type"`
	Driver          string        `json:"driver"`
	TruckType       string        `json:"truck_type"`
	TruckSize       string        `json:"truck_size"`
	TruckNo         string        `json:"truck_no"`
	TransporterCode string        `json:"transporter_code"`
	TransporterName string        `json:"transporter_name"`
	LoadStartTime   string        `json:"load_start_time"`
	LoadEndTime     string        `json:"load_end_time"`
	Remarks         string        `json:"remarks"`
	CreatedBy       int           `json:"created_by"`
	UpdatedBy       int           `json:"updated_by"`
	DeletedBy       int           `json:"deleted_by"`
	Items           []OrderDetail `json:"items" gorm:"foreignKey:OrderID;references:ID"`
}

// type OrderDetailItem struct {
// 	OutboundID  int     `json:"outbound_id"`
// 	OutboundNo  string  `json:"outbound_no"`
// 	DelivTo     string  `json:"deliv_to"`
// 	DelivToName string  `json:"deliv_to_name"`
// 	DelivCity   string  `json:"deliv_city"`
// 	ShipmentID  string  `json:"shipment_id"`
// 	TotalKoli   int     `json:"total_koli"`
// 	Remarks     string  `json:"remarks"`
// 	ItemCode    string  `json:"item_code"`
// 	Quantity    int     `json:"quantity"`
// 	CBM         float64 `json:"cbm"`
// 	TotalCBM    float64 `json:"total_cbm"`
// }

type OrderDetail struct {
	gorm.Model
	OrderID      uint              `json:"order_id"`
	OrderNo      string            `json:"order_no"`
	ShipmentID   string            `json:"shipment_id"`
	OutboundID   types.SnowflakeID `json:"outbound_id"`
	OutboundNo   string            `json:"outbound_no"`
	QtyKoli      int               `json:"qty_koli"`
	VasKoli      int               `json:"vas_koli"`
	TotalItem    int               `json:"total_item"`
	TotalQty     int               `json:"total_qty"`
	TotalCBM     float64           `json:"total_cbm"`
	DelivTo      string            `json:"deliv_to"`
	DelivToName  string            `json:"deliv_to_name"`
	DelivAddress string            `json:"deliv_address"`
	DelivCity    string            `json:"deliv_city"`
	Status       string            `json:"status" gorm:"default:'open'"`
	Remarks      string            `json:"remarks"`
	CreatedBy    int               `json:"created_by"`
	UpdatedBy    int               `json:"updated_by"`
	DeletedBy    int               `json:"deleted_by"`
}

type OrderConsole struct {
	gorm.Model
	OrderID   uint    `json:"order_id"`
	OrderNo   string  `json:"order_no"`
	Status    string  `json:"status" gorm:"default:'open'"`
	Driver    string  `json:"driver"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Remarks   string  `json:"remarks"`
}
