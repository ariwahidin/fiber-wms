package integration

import (
	"time"

	"gorm.io/gorm"
)

type ChannelType string
type FileFormat string
type Timing string
type SourceType string

const (
	ChannelSFTP         ChannelType = "sftp"
	ChannelFTP          ChannelType = "ftp"
	ChannelAPI          ChannelType = "api"
	ChannelFile         ChannelType = "file"
	ChannelGoogleSheets ChannelType = "google_sheets"

	FormatCSV   FileFormat = "csv"
	FormatExcel FileFormat = "excel"
	FormatJSON  FileFormat = "json"
	FormatTXT   FileFormat = "txt"

	TimingRealtime  Timing = "realtime"
	TimingScheduled Timing = "scheduled"

	SourceQuery SourceType = "query" // SQL query
	SourceEvent SourceType = "event" // data dari event payload
)

type Direction string

const (
	DirectionOutbound Direction = "outbound"
	DirectionInbound  Direction = "inbound"
)

type Integration struct {
	ID              uint        `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string      `json:"name" gorm:"not null"`
	EventKey        string      `json:"event_key" gorm:"not null;index"` // cth: "outbound.completed"
	Description     string      `json:"description"`
	ChannelType     ChannelType `json:"channel_type" gorm:"type:varchar(20);not null"`
	FileFormat      FileFormat  `json:"file_format" gorm:"type:varchar(10)"` // kosong kalau API
	SourceType      SourceType  `json:"source_type" gorm:"type:varchar(10);not null"`
	Query           string      `json:"query" gorm:"type:text"` // diisi kalau source_type = query
	FilenamePattern string      `json:"filename_pattern"`       // cth: "outbound_{{outbound_no}}_{{date}}"
	Timing          Timing      `json:"timing" gorm:"type:varchar(15);default:'realtime'"`

	// Schedule (diisi kalau timing = scheduled)
	ScheduleFreq       string     `json:"schedule_freq"` // daily/weekly/monthly
	ScheduleDayOfWeek  *int       `json:"schedule_day_of_week"`
	ScheduleDayOfMonth *int       `json:"schedule_day_of_month"`
	ScheduleHour       int        `json:"schedule_hour"`
	ScheduleMinute     int        `json:"schedule_minute"`
	LastRunAt          *time.Time `json:"last_run_at"`
	NextRunAt          *time.Time `json:"next_run_at"`

	// Notifikasi email
	EmailConfigID      *uint  `json:"email_config_id"`
	NotifyOnSuccess    bool   `json:"notify_on_success" gorm:"default:false"`
	NotifyOnFailure    bool   `json:"notify_on_failure" gorm:"default:true"`
	NotifyEmailSubject string `json:"notify_email_subject"`
	NotifyEmailBody    string `json:"notify_email_body" gorm:"type:text"`

	IsActive bool `json:"is_active" gorm:"default:true"`

	Direction     Direction `json:"direction" gorm:"type:varchar(10);default:'outbound'"`
	ColumnMapping string    `json:"column_mapping" gorm:"type:text"`         // JSON: {"wms_field": "file_column"}
	Action        string    `json:"action" gorm:"default:'create_outbound'"` // create_outbound | create_inbound
	SourcePath    string    `json:"source_path"`                             // path folder/remote yg dibaca
	ArchivePath   string    `json:"archive_path"`                            // file dipindah ke sini kalau sukses
	ErrorPath     string    `json:"error_path"`                              // file dipindah ke sini kalau gagal

	CreatedBy int            `json:"created_by"`
	UpdatedBy int            `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relasi
	Connection *IntegrationConnection `json:"connection" gorm:"foreignKey:IntegrationID"`
	Recipients []IntegrationRecipient `json:"recipients" gorm:"foreignKey:IntegrationID"`
}
