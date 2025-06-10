package models

import "gorm.io/gorm"

type BusinessUnit struct {
	gorm.Model
	DbName    string `json:"db_name" gorm:"unique"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	CreatedBy int    `json:"created_by"`
	UpdatedBy int    `json:"updated_by"`
	DeletedBy int    `json:"deleted_by"`
}
