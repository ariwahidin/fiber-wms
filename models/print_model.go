package models

import "time"

type PrintLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	DocumentType string `gorm:"size:50;not null;index:idx_print_document" json:"document_type"`
	DocumentNo   string `gorm:"size:100;not null;index:idx_print_document" json:"document_no"`
	Module       string `gorm:"size:50;not null" json:"module"`

	PrintAction string `gorm:"size:20;not null" json:"print_action"`
	PrintMethod string `gorm:"size:20;not null" json:"print_method"`
	PrintFormat string `gorm:"size:20" json:"print_format"`

	CopyCount   int    `gorm:"not null;default:1" json:"copy_count"`
	PrinterName string `gorm:"size:150" json:"printer_name"`

	Status       string `gorm:"size:20;not null" json:"status"`
	Reason       string `gorm:"type:nvarchar(max)" json:"reason"`
	ErrorMessage string `gorm:"type:nvarchar(max)" json:"error_message"`

	RequestedBy uint       `gorm:"not null" json:"requested_by"`
	RequestedAt time.Time  `gorm:"not null" json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at"`

	IPAddress   string `gorm:"size:50" json:"ip_address"`
	Workstation string `gorm:"size:100" json:"workstation"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PrintLog) TableName() string {
	return "print_logs"
}
