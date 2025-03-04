package models

import "gorm.io/gorm"

type Truck struct {
	gorm.Model
	Name        string `json:"truck_name" gorm:"unique"`
	Description string `json:"truck_description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
