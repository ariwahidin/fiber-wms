package integration

import (
	"time"

	"gorm.io/gorm"
)

type IntegrationConnection struct {
	ID            uint `json:"id" gorm:"primaryKey;autoIncrement"`
	IntegrationID uint `json:"integration_id" gorm:"not null;uniqueIndex"`

	// SFTP / FTP
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RemotePath string `json:"remote_path"` // path tujuan upload

	// REST API
	URL          string `json:"url"`
	Method       string `json:"method" gorm:"default:'POST'"` // POST / GET / PUT
	Headers      string `json:"headers" gorm:"type:text"`     // JSON string: {"key":"value"}
	AuthType     string `json:"auth_type"`                    // none / basic / bearer / api_key
	AuthValue    string `json:"auth_value"`                   // token / key / "user:pass"
	TimeoutSec   int    `json:"timeout_sec" gorm:"default:30"`
	BodyTemplate string `json:"body_template" gorm:"type:text"` // template JSON body untuk API, support {{placeholder}}

	// File output
	OutputPath string `json:"output_path"` // direktori tujuan simpan file

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
