package report_mailer

import (
	"time"
)

type SendStatus string

const (
	SendStatusSuccess SendStatus = "success"
	SendStatusFailed  SendStatus = "failed"
)

type SendHistory struct {
	ID          uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID    uint       `json:"report_id" gorm:"not null;index"`
	Status      SendStatus `json:"status" gorm:"type:varchar(10);not null"`
	Message     string     `json:"message" gorm:"type:text"` // error message kalau gagal
	SentAt      time.Time  `json:"sent_at"`
	TriggeredBy string     `json:"triggered_by" gorm:"type:varchar(20)"` // "scheduler" / "manual"

	// Relasi
	Report Report `json:"report" gorm:"foreignKey:ReportID"`
}
