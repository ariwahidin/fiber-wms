package models

import "gorm.io/gorm"

type Supplier struct {
	gorm.Model
	SupplierCode string `json:"supplier_code" gorm:"unique"`
	SupplierName string `json:"supplier_name" gorm:"unique"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
