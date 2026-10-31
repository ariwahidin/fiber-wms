package mobiles

import "time"

type VerificationSummary struct {
	OrderID uint   `json:"order_id"`
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`

	TotalExpected int64 `json:"total_expected"`
	TotalScanned  int64 `json:"total_scanned"`
	TotalPending  int64 `json:"total_pending"`

	IsComplete bool `json:"is_complete"`
}

type VerificationItem struct {
	OrderDetailID uint   `json:"order_detail_id"`
	ShipmentID    string `json:"shipment_id"`
	OutboundID    string `json:"outbound_id"`
	OutboundNo    string `json:"outbound_no"`

	QtyKoli   int `json:"qty_koli"`
	TotalItem int `json:"total_item"`
	TotalQty  int `json:"total_qty"`

	DelivTo     string `json:"deliv_to"`
	DelivToName string `json:"deliv_to_name"`

	IsScanned bool       `json:"is_scanned"`
	ScannedBy *int       `json:"scanned_by"`
	ScannedAt *time.Time `json:"scanned_at"`
}

type VerificationHistoryItem struct {
	ID            uint      `json:"ID"`
	OrderNo       string    `json:"order_no"`
	ShipmentID    string    `json:"shipment_id"`
	OrderDetailID uint      `json:"order_detail_id"`
	ScannedBy     int       `json:"scanned_by"`
	ScannedAt     time.Time `json:"scanned_at"`
	Status        string    `json:"status"`
	Remarks       string    `json:"remarks"`
}

type ScanShipmentRequest struct {
	ShipmentID string `json:"shipment_id"`
	Remarks    string `json:"remarks"`
}

type VerificationDetailResponse struct {
	Summary VerificationSummary `json:"summary"`
	Items   []VerificationItem  `json:"items"`
}
