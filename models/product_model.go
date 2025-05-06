package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	ItemCode       string  `json:"item_code" gorm:"unique"`
	ItemName       string  `json:"item_name"`
	Barcode        string  `json:"barcode" gorm:"unique"`
	GMC            string  `json:"gmc"`
	Width          float64 `json:"width" gorm:"default:0"`
	Length         float64 `json:"length" gorm:"default:0"`
	Height         float64 `json:"height" gorm:"default:0"`
	Uom            string  `json:"uom"`
	BaseUomID      uint    `json:"base_uom_id"` // foreign key ke uoms
	BaseUom        Uom     `gorm:"foreignKey:BaseUomID"`
	Kubikasi       float64 `json:"kubikasi" gorm:"default:0"`
	KubikasiSap    float64 `json:"kubikasi_sap" gorm:"default:0"`
	GrossWeight    float64 `json:"gross_weight" gorm:"default:0"`
	NetWeight      float64 `json:"net_weight" gorm:"default:0"`
	SapCode        string  `json:"sap_code"`
	SapDescription string  `json:"sap_description"`
	CBM            float64 `json:"cbm"`
	Group          string  `json:"group"`
	Category       string  `json:"category"`
	HasWaranty     string  `json:"has_waranty" gorm:"default:'N'"`
	HasSerial      string  `json:"has_serial" gorm:"default:'N'"`
	ManualBook     string  `json:"manual_book" gorm:"default:'N'"`
	HasAdaptor     string  `json:"has_adaptor" gorm:"default:'N'"`
	Remarks        string  `json:"remarks"`
	CreatedBy      int
	UpdatedBy      int
	DeletedBy      int
}

type Uom struct {
	ID   uint   `gorm:"primaryKey"`
	Code string `gorm:"unique" json:"code"` // contoh: "PCS", "BOX", "CTN"
	Name string `json:"name"`               // contoh: "Piece", "Box", "Carton"
}

type UomConversion struct {
	ID        uint    `gorm:"primaryKey"`
	FromUomID uint    `json:"from_uom_id"` // foreign key ke uoms
	ToUomID   uint    `json:"to_uom_id"`   // foreign key ke uoms
	Factor    float64 `json:"factor"`      // contoh: 1 BOX = 12 PCS → factor = 12
	ProductID *uint   `json:"product_id"`  // jika konversi hanya berlaku untuk produk tertentu (opsional)
}
