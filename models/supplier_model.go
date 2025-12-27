package models

import (
	"gorm.io/gorm"
)

type Supplier struct {
	gorm.Model
	ID           uint   `json:"ID" gorm:"primaryKey"`
	SupplierCode string `json:"supplier_code" gorm:"unique"`
	SupplierName string `json:"supplier_name"`
	OwnerCode    string `json:"owner_code"`
	SuppAddr1    string `json:"supp_addr1"`
	SuppCity     string `json:"supp_city"`
	SuppCountry  string `json:"supp_country"`
	SuppPhone    string `json:"supp_phone"`
	SuppEmail    string `json:"supp_email"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
