package report_mailer

import (
	"time"

	"gorm.io/gorm"
)

type RecipientType string

const (
	RecipientTO RecipientType = "TO"
	RecipientCC RecipientType = "CC"
)

type ReportRecipient struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID  uint           `json:"report_id" gorm:"not null;index"`
	Email     string         `json:"email" gorm:"not null"`
	Name      string         `json:"name"`                                 // nama penerima (opsional)
	Type      RecipientType  `json:"type" gorm:"type:varchar(2);not null"` // TO / CC
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
