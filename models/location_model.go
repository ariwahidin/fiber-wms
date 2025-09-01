package models

import (
	"gorm.io/gorm"
)

type Location struct {
	gorm.Model
	LocationCode string `json:"location_code" gorm:"unique"`
	Row          string `json:"row"`
	Bay          string `json:"bay"`
	Level        string `json:"level"`
	Bin          string `json:"bin"`
	Area         string `json:"area"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
