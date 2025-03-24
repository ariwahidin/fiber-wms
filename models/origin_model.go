package models

import "gorm.io/gorm"

type Origin struct {
	gorm.Model
	Country   string `json:"country" gorm:"unique"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}

type WarehouseCode struct {
	gorm.Model
	WarehouseCode string `json:"warehouse_code" gorm:"unique"`
	CreatedBy     int
	UpdatedBy     int
	DeletedBy     int
}

type QaStatus struct {
	gorm.Model
	QaStatus  string `json:"qa_status" gorm:"unique"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}
