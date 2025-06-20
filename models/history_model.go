package models

import (
	"fiber-app/controllers/idgen"
	"time"

	"gorm.io/gorm"
)

type TransactionHistory struct {
	ID        int64  `json:"ID" gorm:"primaryKey"`
	RefNo     string `json:"ref_no"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Detail    string `json:"detail"`
	CreatedAt time.Time
	CreatedBy int
	UpdatedAt time.Time
	UpdatedBy int
	DeletedAt gorm.DeletedAt
	DeletedBy int
}

func (u *TransactionHistory) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = idgen.GenerateID()
	return
}
