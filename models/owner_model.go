package models

import (
	"gorm.io/gorm"
)

type Owner struct {
	gorm.Model
	Code        string `json:"code" gorm:"unique"`
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
