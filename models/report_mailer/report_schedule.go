package report_mailer

import (
	"time"

	"gorm.io/gorm"
)

type ScheduleFrequency string

const (
	FrequencyDaily   ScheduleFrequency = "daily"
	FrequencyWeekly  ScheduleFrequency = "weekly"
	FrequencyMonthly ScheduleFrequency = "monthly"
)

type ReportSchedule struct {
	ID            uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID      uint              `json:"report_id" gorm:"not null;uniqueIndex"`      // 1 report = 1 schedule
	Frequency     ScheduleFrequency `json:"frequency" gorm:"type:varchar(10);not null"` // daily/weekly/monthly
	DayOfWeek     *int              `json:"day_of_week"`                                // 0=Minggu ... 6=Sabtu, dipakai kalau weekly
	DayOfMonth    *int              `json:"day_of_month"`                               // 1-31, dipakai kalau monthly
	Hour          int               `json:"hour"`                                       // 0-23
	Minute        int               `json:"minute"`                                     // 0-59
	IsActive      bool              `json:"is_active" gorm:"default:true"`
	LastRunAt     *time.Time        `json:"last_run_at"`
	NextRunAt     *time.Time        `json:"next_run_at"`
	MaxRetry      int               `json:"max_retry" gorm:"default:3"`   // ← tambah
	RetryCount    int               `json:"retry_count" gorm:"default:0"` // ← tambah
	RetryDelayMin int               `json:"retry_delay_min" gorm:"default:5"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     gorm.DeletedAt    `json:"-" gorm:"index"`
}
