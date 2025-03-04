package models

import "gorm.io/gorm"

type Handling struct {
	gorm.Model
	Name      string `json:"name" gorm:"unique"`
	Type      string `json:"type" gorm:"default:'single'"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}

type HandlingRate struct {
	gorm.Model
	HandlingId int
	Name       string
	RateIdr    int `json:"rate_idr" gorm:"default:0"`
	CreatedBy  int
	UpdatedBy  int
	DeletedBy  int
}

type HandlingCombine struct {
	gorm.Model
	HandlingId int
	CreatedBy  int
	UpdatedBy  int
	DeletedBy  int
}

type HandlingCombineDetail struct {
	gorm.Model
	HandlingCombineId int
	HandlingId        int
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
}
