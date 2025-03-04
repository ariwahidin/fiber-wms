package models

import "gorm.io/gorm"

type Customer struct {
	gorm.Model
	CustomerCode string `json:"customer_code" gorm:"unique"`
	CustomerName string `json:"customer_name" gorm:"unique"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
