package models

import "gorm.io/gorm"

type Truck struct {
	gorm.Model
	Name        string  `json:"name" gorm:"unique"`
	Description string  `json:"description"`
	CBM         float64 `json:"cbm"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
