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
