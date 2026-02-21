package report_mailer

import (
	"time"

	"gorm.io/gorm"
)

type EmailConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string         `json:"name" gorm:"not null"` // label config, misal "SMTP Gmail Production"
	Host      string         `json:"host" gorm:"not null"` // smtp.gmail.com
	Port      int            `json:"port" gorm:"not null"` // 587, 465, 25
	Username  string         `json:"username" gorm:"not null"`
	Password  string         `json:"password" gorm:"not null"`
	FromName  string         `json:"from_name" gorm:"not null"`  // nama pengirim
	FromEmail string         `json:"from_email" gorm:"not null"` // email pengirim
	UseTLS    bool           `json:"use_tls" gorm:"default:true"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	CreatedBy int            `json:"created_by"`
	UpdatedBy int            `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
