package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	ID             int     `json:"ID" gorm:"primaryKey"`
	ProductNumber  int     `json:"product_number" gorm:"unique"`
	OwnerCode      string  `json:"owner_code"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	Barcode        string  `json:"barcode"`
	GMC            string  `json:"gmc"`
	Width          float64 `json:"width" gorm:"default:0"`
	Length         float64 `json:"length" gorm:"default:0"`
	Height         float64 `json:"height" gorm:"default:0"`
	Uom            string  `json:"uom"`
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

func (p *Product) BeforeCreate(tx *gorm.DB) (err error) {
	var lastProduct Product

	// Cari product dengan nomor terbesar
	if err := tx.Select("product_number").Order("product_number desc").First(&lastProduct).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			p.ProductNumber = 1 // kalau belum ada data
		} else {
			return err
		}
	} else {
		p.ProductNumber = lastProduct.ProductNumber + 1
	}

	return nil
}

type Category struct {
	ID   uint   `gorm:"primaryKey"`
	Code string `gorm:"unique" json:"code"` // contoh: "A", "B", "C"
	Name string `json:"name"`               // contoh: "A", "B", "C"
}

type Uom struct {
	ID   types.SnowflakeID `gorm:"primaryKey"`
	Code string            `gorm:"unique" json:"code"` // contoh: "PCS", "BOX", "CTN"
	Name string            `json:"name"`               // contoh: "Piece", "Box", "Carton"
}
