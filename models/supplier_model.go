package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type Supplier struct {
	gorm.Model
	ID           types.SnowflakeID `json:"ID" gorm:"primaryKey"`
	SupplierCode string            `json:"supplier_code" gorm:"unique"`
	SupplierName string            `json:"supplier_name"`
	OwnerCode    string            `json:"owner_code"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
