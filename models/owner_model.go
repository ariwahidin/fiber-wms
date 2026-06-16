package models

import (
	"gorm.io/gorm"
)

type Owner struct {
	gorm.Model
	Code        string `json:"code" gorm:"unique"`
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

type QRFieldMap struct {
	SKU              string `json:"sku"`
	EAN              string `json:"ean"`
	Serial           string `json:"serial"`
	CartonSerial     string `json:"carton_serial"`
	Batch            string `json:"batch"`
	MfgDate          string `json:"mfg_date"`
	QtyPerCarton     string `json:"qty_per_carton"`
	InnerSerialStart string `json:"inner_serial_start"`
	InnerSerialEnd   string `json:"inner_serial_end"`
	Product          string `json:"product"`
	Brand            string `json:"brand"`
	Model            string `json:"model"`
}

type OwnerQRConfig struct {
	gorm.Model
	OwnerID       uint       `json:"owner_id" gorm:"uniqueIndex"`
	PatternType   string     `json:"pattern_type"`    // "bracket_kv", "positional", "key_value"
	Delimiter     string     `json:"delimiter"`       // "|", ",", ";"
	MfgDateFormat string     `json:"mfg_date_format"` // "YYYYMMDD", "DDMMYYYY"
	QtyStripUnit  bool       `json:"qty_strip_unit"`
	FieldMap      QRFieldMap `json:"field_map" gorm:"serializer:json"`
}
