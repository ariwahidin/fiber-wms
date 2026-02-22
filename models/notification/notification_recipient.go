package notification

import (
	"time"

	"gorm.io/gorm"
)

type RecipientType string

const (
	RecipientTO RecipientType = "TO"
	RecipientCC RecipientType = "CC"
)

type NotificationRecipient struct {
	ID             uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	NotificationID uint           `json:"notification_id" gorm:"not null;index"`
	Email          string         `json:"email" gorm:"not null"`
	Name           string         `json:"name"`
	Type           RecipientType  `json:"type" gorm:"type:varchar(5);default:'TO'"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}
