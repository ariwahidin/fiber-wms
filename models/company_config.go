package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ─── JSON Helper Type ─────────────────────────────────────────────────────────

type JSONSlides []SlideItem

type SlideItem struct {
	ImageURL string `json:"image_url"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
}

func (s JSONSlides) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *JSONSlides) Scan(value interface{}) error {
	var raw []byte
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return errors.New("unsupported type for JSONSlides")
	}
	return json.Unmarshal(raw, s)
}

// ─── Model ────────────────────────────────────────────────────────────────────

// CompanyConfig menyimpan konfigurasi perusahaan yang bisa dipakai
// di login page, dokumen, laporan, header, footer, dsb.
type CompanyConfig struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`

	// Identitas Perusahaan
	CompanyName  string `gorm:"type:nvarchar(200);not null" json:"company_name"`
	CompanyShort string `gorm:"type:nvarchar(100)" json:"company_short"`
	Address      string `gorm:"type:nvarchar(500)" json:"address"`
	Phone        string `gorm:"type:nvarchar(50)" json:"phone"`
	Email        string `gorm:"type:nvarchar(100)" json:"email"`
	Website      string `gorm:"type:nvarchar(200)" json:"website"`

	// Aplikasi
	AppName string `gorm:"type:nvarchar(100);not null" json:"app_name"`
	Tagline string `gorm:"type:nvarchar(200)" json:"tagline"`
	LogoURL string `gorm:"type:nvarchar(500)" json:"logo_url"`

	// Branding
	PrimaryColor string `gorm:"type:nvarchar(20);default:'#041F5F'" json:"primary_color"`
	AccentColor  string `gorm:"type:nvarchar(20);default:'#1A50C8'" json:"accent_color"`

	// Login Page
	LoginTheme  string     `gorm:"type:nvarchar(50);default:'ThemeModern'" json:"login_theme"`
	LoginSlides JSONSlides `gorm:"type:nvarchar(max)" json:"login_slides"`

	// Audit
	UpdatedBy int       `gorm:"type:int" json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}
