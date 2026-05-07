package report_mailer

import (
	"time"

	"gorm.io/gorm"
)

type ReportQuery struct {
	ID            uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID      uint           `json:"report_id" gorm:"not null;index"`
	Name          string         `json:"name" gorm:"not null"` // nama sheet / nama file
	Query         string         `json:"query" gorm:"type:text;not null"`
	ExcelTitle    string         `json:"excel_title" gorm:"default:''"`
	ExcelSubtitle string         `json:"excel_subtitle" gorm:"default:''"`
	SortOrder     int            `json:"sort_order" gorm:"default:0"` // urutan sheet/file
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}
