package integration

import (
	"time"

	"gorm.io/gorm"
)

// ─── Recipient ────────────────────────────────────────────────────────────────

type IntegrationRecipientType string

const (
	RecipientTO IntegrationRecipientType = "TO"
	RecipientCC IntegrationRecipientType = "CC"
)

type IntegrationRecipient struct {
	ID            uint                     `json:"id" gorm:"primaryKey;autoIncrement"`
	IntegrationID uint                     `json:"integration_id" gorm:"not null;index"`
	Email         string                   `json:"email" gorm:"not null"`
	Name          string                   `json:"name"`
	Type          IntegrationRecipientType `json:"type" gorm:"type:varchar(5);default:'TO'"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	DeletedAt     gorm.DeletedAt           `json:"-" gorm:"index"`
}

// ─── History ──────────────────────────────────────────────────────────────────

type IntegrationStatus string

const (
	StatusSuccess IntegrationStatus = "success"
	StatusFailed  IntegrationStatus = "failed"
)

type IntegrationHistory struct {
	ID             uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	IntegrationID  uint              `json:"integration_id" gorm:"not null;index"`
	EventKey       string            `json:"event_key"`
	ChannelType    ChannelType       `json:"channel_type" gorm:"type:varchar(20)"`
	Status         IntegrationStatus `json:"status" gorm:"type:varchar(10)"`
	Message        string            `json:"message" gorm:"type:text"`
	FileName       string            `json:"file_name"`                        // nama file yang dikirim (kalau ada)
	PayloadSummary string            `json:"payload_summary" gorm:"type:text"` // ringkasan data
	TriggeredBy    string            `json:"triggered_by"`                     // "event" / "scheduler"
	SentAt         time.Time         `json:"sent_at"`
	DeletedAt      gorm.DeletedAt    `json:"-" gorm:"index"`

	// Relasi
	Integration Integration `json:"integration" gorm:"foreignKey:IntegrationID"`
}
