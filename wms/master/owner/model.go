package owner

import (
	"gorm.io/gorm"
)

type Owner struct {
	gorm.Model
	// ID          types.SnowflakeID `json:"id" gorm:"primaryKey"`
	Code        string `json:"code" gorm:"unique"`
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
