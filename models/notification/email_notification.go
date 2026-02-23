package notification

import (
	report_mailer "fiber-app/models/report_mailer"
	"time"

	"gorm.io/gorm"
)

type EmailNotification struct {
	ID            uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string `json:"name" gorm:"not null"`
	EventKey      string `json:"event_key" gorm:"not null;index"` // cth: "outbound.completed"
	Description   string `json:"description"`
	EmailConfigID uint   `json:"email_config_id" gorm:"not null"`
	EmailSubject  string `json:"email_subject" gorm:"not null"`
	EmailHeader   string `json:"email_header" gorm:"type:text"`
	EmailBody     string `json:"email_body" gorm:"type:text"`
	EmailFooter   string `json:"email_footer" gorm:"type:text"`
	HeaderColor   string `json:"header_color" gorm:"default:'#1E40AF'"`

	WithAttachment   bool   `json:"with_attachment" gorm:"default:false"`
	AttachmentName   string `json:"attachment_name"`
	AttachmentFields string `json:"attachment_fields" gorm:"type:text"`
	AttachmentSource string `json:"attachment_source" gorm:"default:'event'"`
	AttachmentQuery  string `json:"attachment_query" gorm:"type:text"`

	IsActive  bool           `json:"is_active" gorm:"default:true"`
	CreatedBy int            `json:"created_by"`
	UpdatedBy int            `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relasi
	EmailConfig report_mailer.EmailConfig `json:"email_config" gorm:"foreignKey:EmailConfigID"`
	Recipients  []NotificationRecipient   `json:"recipients" gorm:"foreignKey:NotificationID"`
}
