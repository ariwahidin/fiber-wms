package notification

import (
	"time"

	"gorm.io/gorm"
)

type NotifStatus string

const (
	NotifStatusSuccess NotifStatus = "success"
	NotifStatusFailed  NotifStatus = "failed"
)

type NotificationHistory struct {
	ID             uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	NotificationID uint           `json:"notification_id" gorm:"not null;index"`
	EventKey       string         `json:"event_key" gorm:"not null"`
	Status         NotifStatus    `json:"status" gorm:"type:varchar(10)"`
	Message        string         `json:"message" gorm:"type:text"`
	Payload        string         `json:"payload" gorm:"type:text"` // JSON string dari data event
	SentAt         time.Time      `json:"sent_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	// Relasi
	Notification EmailNotification `json:"notification" gorm:"foreignKey:NotificationID"`
}
