package models

import "gorm.io/gorm"

type ShopeeConfig struct {
	gorm.Model
	ID           uint   `json:"id" gorm:"primaryKey"`
	PartnerID    int64  `json:"partner_id"`
	PartnerKey   string `json:"partner_key"`
	ShopID       int64  `json:"shop_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	BaseURL      string `json:"base_url"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	Environment  string `json:"environment" gorm:"default:'sandbox'"` // sandbox | production
	PushURL      string `json:"push_url"`                             // URL untuk menerima webhook dari Shopee
	CreatedBy    int    `json:"created_by"`
	UpdatedBy    int    `json:"updated_by"`
}
