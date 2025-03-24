package models

import (
	"time"

	"gorm.io/gorm"
)

type FileLog struct {
	gorm.Model
	ID           uint   `gorm:"primaryKey"`
	Filename     string `gorm:"unique;not null"`
	DateModified time.Time
}

type Receiving struct {
	ID            uint `gorm:"primaryKey"`
	InboundID     string
	Supplier      string
	PO_Number     string
	Material      string
	Description   string
	Quantity      int
	UOM           string
	Warehouse     string
	ReceivingDate string
	Filename      string
}
