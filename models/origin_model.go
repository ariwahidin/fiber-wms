package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type Origin struct {
	gorm.Model
	// ID        types.SnowflakeID `json:"id" gorm:"primaryKey"`
	Country   string `json:"country" gorm:"unique"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}

type Warehouse struct {
	gorm.Model
	ID          types.SnowflakeID `json:"id" gorm:"primaryKey"`
	Code        string            `json:"code" gorm:"unique"`
	Name        string            `json:"name" gorm:"unique"`
	Description string            `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

type Division struct {
	gorm.Model
	ID          types.SnowflakeID `json:"id" gorm:"primaryKey"`
	Code        string            `json:"code" gorm:"unique"`
	Name        string            `json:"name" gorm:"unique"`
	Description string            `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

type QaStatus struct {
	gorm.Model
	ID        types.SnowflakeID `json:"id" gorm:"primaryKey"`
	QaStatus  string            `json:"qa_status" gorm:"unique"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}
