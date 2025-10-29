package models

import "time"

type IntegrationLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	LogTime      time.Time `gorm:"autoCreateTime" json:"log_time"`
	SourceSystem string    `gorm:"size:100" json:"source_system"`
	ProcessName  string    `gorm:"size:100;not null" json:"process_name"`
	FileName     string    `gorm:"size:255" json:"file_name"`
	RecordKey    string    `gorm:"size:100" json:"record_key"`
	LogLevel     string    `gorm:"size:10;not null" json:"log_level"`
	Message      string    `gorm:"type:text;not null" json:"message"`
	CreatedBy    string    `gorm:"size:100;default:'SYSTEM'" json:"created_by"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}
