// package report_mailer

// import (
// 	"time"

// 	"gorm.io/gorm"
// )

// type Report struct {
// 	ID            uint           `json:"id" gorm:"primaryKey;autoIncrement"`
// 	Name          string         `json:"name" gorm:"not null"` // nama report
// 	Description   string         `json:"description"`
// 	Query         string         `json:"query" gorm:"type:text;not null"` // raw SQL query
// 	EmailConfigID uint           `json:"email_config_id" gorm:"not null"`
// 	ExcelTitle    string         `json:"excel_title" gorm:"not null"` // baris A1
// 	ExcelSubtitle string         `json:"excel_subtitle"`              // baris A2
// 	IsActive      bool           `json:"is_active" gorm:"default:true"`
// 	CreatedBy     int            `json:"created_by"`
// 	UpdatedBy     int            `json:"updated_by"`
// 	CreatedAt     time.Time      `json:"created_at"`
// 	UpdatedAt     time.Time      `json:"updated_at"`
// 	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

// 	// Relasi (preload)
// 	EmailConfig EmailConfig       `json:"email_config" gorm:"foreignKey:EmailConfigID"`
// 	Recipients  []ReportRecipient `json:"recipients" gorm:"foreignKey:ReportID"`
// 	Schedule    *ReportSchedule   `json:"schedule" gorm:"foreignKey:ReportID"`
// }

// PATCH: update models/report_mailer/report.go
// Ganti seluruh isi file dengan versi ini:

package report_mailer

import (
	"time"

	"gorm.io/gorm"
)

type OutputMode string

const (
	OutputModeSingleFile OutputMode = "single_file" // 1 Excel, banyak sheet
	OutputModeMultiFile  OutputMode = "multi_file"  // banyak Excel, 1 email
)

type Report struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	// Query         string         `json:"query" gorm:"type:text;not null"` // raw SQL query
	OutputMode    OutputMode `json:"output_mode" gorm:"type:varchar(20);default:'single_file'"`
	EmailConfigID uint       `json:"email_config_id" gorm:"not null"`
	ExcelTitle    string     `json:"excel_title" gorm:"not null"`
	ExcelSubtitle string     `json:"excel_subtitle"`

	EmailSubject string `json:"email_subject" gorm:"default:null"`
	EmailHeader  string `json:"email_header" gorm:"type:text;default:null"`
	EmailBody    string `json:"email_body" gorm:"type:text;default:null"`
	EmailFooter  string `json:"email_footer" gorm:"type:text;default:null"`
	HeaderColor  string `json:"header_color" gorm:"default:'#1E40AF'"`

	AlertEmail string         `json:"alert_email" gorm:"default:null"`
	IsActive   bool           `json:"is_active" gorm:"default:true"`
	CreatedBy  int            `json:"created_by"`
	UpdatedBy  int            `json:"updated_by"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`

	// Relasi
	EmailConfig EmailConfig       `json:"email_config" gorm:"foreignKey:EmailConfigID"`
	Recipients  []ReportRecipient `json:"recipients" gorm:"foreignKey:ReportID"`
	Schedule    *ReportSchedule   `json:"schedule" gorm:"foreignKey:ReportID"`
	Queries     []ReportQuery     `json:"queries" gorm:"foreignKey:ReportID;orderBy:sort_order asc"`
}
