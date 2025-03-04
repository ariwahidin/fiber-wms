package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	ItemCode  string  `json:"item_code" gorm:"unique"`
	ItemName  string  `json:"item_name"`
	CBM       float64 `json:"cbm"`
	GMC       string  `json:"gmc" gorm:"unique"`
	Group     string  `json:"group"`
	Category  string  `json:"category"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}
