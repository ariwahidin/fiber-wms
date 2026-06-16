package qr_parser

import (
	"errors"
	"fmt"
	"strings"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var ErrNoMatch = errors.New("no fields could be parsed from QR string")

// ─── Types ────────────────────────────────────────────────────────────────────

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

type Config struct {
	PatternType   string     `json:"pattern_type"`    // "bracket_kv", "positional", "key_value"
	Delimiter     string     `json:"delimiter"`       // "|", ",", ";"
	MfgDateFormat string     `json:"mfg_date_format"` // "YYYYMMDD", "DDMMYYYY"
	QtyStripUnit  bool       `json:"qty_strip_unit"`
	FieldMap      QRFieldMap `json:"field_map"`
}

type ParsedQRData struct {
	SKU                   string   `json:"sku"`
	EAN                   string   `json:"ean"`
	Product               string   `json:"product"`
	Brand                 string   `json:"brand"`
	Model                 string   `json:"model"`
	Serial                string   `json:"serial"`
	CartonSerial          string   `json:"carton_serial"`
	Batch                 string   `json:"batch"`
	MfgDate               string   `json:"mfg_date"`
	QtyPerCarton          *int     `json:"qty_per_carton"`
	LabelType             string   `json:"label_type"` // "UNIT", "CARTON", "UNKNOWN"
	InnerSerialStart      string   `json:"inner_serial_start"`
	InnerSerialEnd        string   `json:"inner_serial_end"`
	InnerSerials          []string `json:"inner_serials"`
	InnerSerialRangeError string   `json:"inner_serial_range_error"`
}

// ─── Default Config ───────────────────────────────────────────────────────────

func DefaultConfig() Config {
	return Config{
		PatternType:   "bracket_kv",
		Delimiter:     "",
		MfgDateFormat: "YYYYMMDD",
		QtyStripUnit:  true,
		FieldMap: QRFieldMap{
			SKU:              "SKU",
			EAN:              "EAN",
			Serial:           "SERIAL",
			CartonSerial:     "CARTON_SERIAL",
			Batch:            "BATCH",
			MfgDate:          "MFG_DATE",
			QtyPerCarton:     "QTY_PER_CARTON",
			InnerSerialStart: "INNER_SERIAL_START",
			InnerSerialEnd:   "INNER_SERIAL_END",
			Product:          "PRODUCT",
			Brand:            "BRAND",
			Model:            "MODEL",
		},
	}
}

// ─── Entry Point ──────────────────────────────────────────────────────────────

func Parse(raw string, config Config) (ParsedQRData, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedQRData{}, errors.New("raw QR string is empty")
	}

	switch config.PatternType {
	case "bracket_kv":
		return parseBracketKV(raw, config)
	case "positional":
		return parsePositional(raw, config)
	case "key_value":
		return parseKeyValue(raw, config)
	default:
		return ParsedQRData{}, fmt.Errorf("unknown pattern type: %s", config.PatternType)
	}
}
