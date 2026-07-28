package models

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	Code      string `json:"code" gorm:"unique"`
	Name      string `json:"name"`
	Remarks   string `json:"remarks"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}
