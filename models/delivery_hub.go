package models

import "gorm.io/gorm"

type DeliveryHub struct {
	gorm.Model

	Code     string `json:"code" gorm:"type:nvarchar(50);not null;uniqueIndex:ux_delivery_hubs_code"`
	Name     string `json:"name" gorm:"type:nvarchar(150);not null"`
	Address  string `json:"address" gorm:"type:nvarchar(500);not null"`
	City     string `json:"city" gorm:"type:nvarchar(100);not null"`
	IsActive bool   `json:"is_active" gorm:"not null;default:true"`
}
