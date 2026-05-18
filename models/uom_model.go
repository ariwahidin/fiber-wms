package models

import (
	"time"

	"gorm.io/gorm"
)

type UomConversion struct {
	ID             uint    `json:"ID" gorm:"primaryKey"`
	ItemID         uint    `json:"item_id"`
	ItemCode       string  `json:"item_code"`
	Ean            string  `json:"ean"`
	FromUom        string  `json:"from_uom"`
	ToUom          string  `json:"to_uom"`
	ConversionRate float64 `json:"conversion_rate"`
	IsBase         bool    `json:"is_base"`
	IsLocked       bool    `json:"is_locked" default:"false"`
	CreatedAt      time.Time
	CreatedBy      int
	UpdatedAt      time.Time
	UpdatedBy      int
	DeletedAt      gorm.DeletedAt
	DeletedBy      int
}

type UomConversionInput struct {
	ItemCode       string `json:"item_code"`
	Ean            string `json:"ean" required:"true"`
	FromUom        string `json:"from_uom"`
	ToUom          string `json:"to_uom"`
	ConversionRate int    `json:"conversion_rate"`
	IsBase         bool   `json:"is_base"`
}
