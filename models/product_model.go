package models

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	ProductNumber int     `json:"product_number" gorm:"unique"`
	OwnerCode     string  `json:"owner_code"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	UnitModel     string  `json:"unit_model"`
	Barcode       string  `json:"barcode"`
	QtyPerCarton  int     `json:"qty_per_carton" gorm:"default:0"`
	GMC           string  `json:"gmc"`
	Width         float64 `json:"width" gorm:"default:0"`
	Length        float64 `json:"length" gorm:"default:0"`
	Height        float64 `json:"height" gorm:"default:0"`
	Weight        float64 `json:"weight" gorm:"default:0"`
	Color         string  `json:"color"`
	Uom           string  `json:"uom"`
	CBM           float64 `json:"cbm"`
	Group         string  `json:"group"`
	Category      string  `json:"category"`
	HasWaranty    string  `json:"has_waranty" gorm:"default:'N'"`
	HasSerial     string  `json:"has_serial" gorm:"default:'N'"`
	ManualBook    string  `json:"manual_book" gorm:"default:'N'"`
	HasAdaptor    string  `json:"has_adaptor" gorm:"default:'N'"`
	Remarks       string  `json:"remarks"`
	UserDef1      string  `json:"user_def1"`
	UserDef2      string  `json:"user_def2"`
	UserDef3      string  `json:"user_def3"`
	CreatedBy     int
	UpdatedBy     int
	DeletedBy     int

	//Additional fields
	ItemCode2       string  `json:"item_code2"`
	ItemCode3       string  `json:"item_code3"`
	PcsPeBox        int     `json:"pcs_per_box" gorm:"default:0"`
	PcsPerDozen     int     `json:"pcs_per_dozen" gorm:"default:0"`
	PcsPerSet       int     `json:"pcs_per_set" gorm:"default:0"`
	PcsPerPack      int     `json:"pcs_per_pack" gorm:"default:0"`
	PcsPerCase      int     `json:"pcs_per_case" gorm:"default:0"`
	PcsPerGross     int     `json:"pcs_per_gross" gorm:"default:0"`
	CbmPerCarton    float64 `json:"cbm_per_carton" gorm:"default:0"`
	CbmPerPcs       float64 `json:"cbm_per_pcs" gorm:"default:0"`
	TonagePerCarton float64 `json:"tonnage_per_carton" gorm:"default:0"`
	TonagePerPcs    float64 `json:"tonnage_per_pcs" gorm:"default:0"`
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

type Uom struct {
	ID   uint   `gorm:"primaryKey"`
	Code string `gorm:"unique" json:"code"` // contoh: "PCS", "BOX", "CTN"
	Name string `json:"name"`               // contoh: "Piece", "Box", "Carton"
}

type ItemPackaging struct {
	ID       uint   `json:"id" gorm:"primaryKey autoincrement;not null"`
	ItemID   uint   `gorm:"not null;index"`
	ItemCode string `json:"item_code" gorm:"not null"`
	UOM      string `json:"uom" gorm:"not null"`
	Ean      string `json:"ean" gorm:"not null"`

	// Dimensi (cm)
	LengthCM float64 `json:"length_cm" gorm:"type:decimal(10,2);not null;default:0"`
	WidthCM  float64 `json:"width_cm" gorm:"type:decimal(10,2);not null;default:0"`
	HeightCM float64 `json:"height_cm" gorm:"type:decimal(10,2);not null;default:0"`

	// Berat (kg)
	NetWeightKG   float64 `json:"net_weight_kg" gorm:"type:decimal(10,3);not null;default:0"`
	GrossWeightKG float64 `json:"gross_weight_kg" gorm:"type:decimal(10,3);not null;default:0"`

	// Flag tambahan
	IsActive bool `gorm:"not null;default:true"`

	CreatedBy int
	CreatedAt time.Time
	UpdatedBy int
	UpdatedAt time.Time
}

type ProductRegister struct {
	gorm.Model
	OwnerCode     string `json:"owner_code"`
	SKU           string `json:"sku"`
	Description   string `json:"description"`
	UnitModel     string `json:"unit_model"`
	Ean           string `json:"ean"`
	Uom           string `json:"uom"`
	CreatedBy     int
	CreatedByName string `json:"created_by_name" gorm:"column:created_by_name;->"`
	CreatedAt     time.Time
	UpdatedBy     int
	UpdatedAt     time.Time
}
