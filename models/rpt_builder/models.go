package rpt_builder

import "time"

// =============================================================================
// RPT2_TEMPLATES — definisi report (kepala)
// =============================================================================

type Rpt2Template struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:50;not null"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Description string    `json:"description" gorm:"size:255"`
	Category    string    `json:"category" gorm:"size:50"` // INBOUND | OUTBOUND | INVENTORY | dll
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedBy   int       `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedBy   int       `json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relasi
	Sheets []Rpt2Sheet `json:"sheets,omitempty" gorm:"foreignKey:TemplateID;references:ID"`
	Params []Rpt2Param `json:"params,omitempty" gorm:"foreignKey:TemplateID;references:ID"`
}

func (Rpt2Template) TableName() string { return "rpt2_templates" }

// =============================================================================
// RPT2_SHEETS — sheet Excel per template, masing-masing punya query sendiri
// =============================================================================

type Rpt2Sheet struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID uint      `json:"template_id" gorm:"not null;index"`
	SheetName  string    `json:"sheet_name" gorm:"size:50;not null"`
	SqlQuery   string    `json:"sql_query" gorm:"type:text;not null"`
	SheetOrder int       `json:"sheet_order" gorm:"default:0"`
	CreatedBy  int       `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedBy  int       `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relasi
	Template Rpt2Template `json:"-" gorm:"foreignKey:TemplateID"`
	Columns  []Rpt2Column `json:"columns,omitempty" gorm:"foreignKey:SheetID;references:ID"`
}

func (Rpt2Sheet) TableName() string { return "rpt2_sheets" }

// =============================================================================
// RPT2_COLUMNS — konfigurasi kolom default per sheet
// =============================================================================

type Rpt2Column struct {
	ID               uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID       uint   `json:"template_id" gorm:"not null;index"`
	SheetID          uint   `json:"sheet_id" gorm:"not null;index"`
	ColumnKey        string `json:"column_key" gorm:"size:100;not null"`   // nama kolom dari SQL
	DisplayName      string `json:"display_name" gorm:"size:100;not null"` // label friendly
	ColumnOrder      int    `json:"column_order" gorm:"default:0"`
	ExcelWidth       int    `json:"excel_width" gorm:"default:120"`
	ExcelFormat      string `json:"excel_format" gorm:"size:50"` // DATE | NUMBER | TEXT
	IsDefaultVisible bool   `json:"is_default_visible" gorm:"default:true"`
	IsToggleable     bool   `json:"is_toggleable" gorm:"default:true"`

	// Relasi
	Sheet Rpt2Sheet `json:"-" gorm:"foreignKey:SheetID"`
}

func (Rpt2Column) TableName() string { return "rpt2_columns" }

// =============================================================================
// RPT2_PARAMS — parameter input yang diisi user sebelum run report
// =============================================================================

type Rpt2Param struct {
	ID               uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID       uint   `json:"template_id" gorm:"not null;index"`
	ParamKey         string `json:"param_key" gorm:"size:100;not null"` // key yang dipakai di SQL: :start_date
	Label            string `json:"label" gorm:"size:100;not null"`
	ParamType        string `json:"param_type" gorm:"size:20;not null"` // DATE | DATERANGE | SELECT | TEXT | NUMBER
	DefaultValue     string `json:"default_value" gorm:"size:255"`
	OptionsQuery     string `json:"options_query" gorm:"type:text"`     // query untuk dropdown dinamis
	ApplicableSheets string `json:"applicable_sheets" gorm:"type:text"` // JSON array sheet IDs, kosong = semua
	IsRequired       bool   `json:"is_required" gorm:"default:false"`
	ParamOrder       int    `json:"param_order" gorm:"default:0"`

	// Relasi
	Template Rpt2Template `json:"-" gorm:"foreignKey:TemplateID"`
}

func (Rpt2Param) TableName() string { return "rpt2_params" }

// =============================================================================
// RPT2_USER_COLUMN_PREFS — preferensi kolom per user per sheet
// auto-insert dari default saat pertama kali download
// =============================================================================

type Rpt2UserColumnPref struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID  uint      `json:"template_id" gorm:"not null;index"`
	SheetID     uint      `json:"sheet_id" gorm:"not null;index"`
	UserID      uint      `json:"user_id" gorm:"not null;index"`
	ColumnKey   string    `json:"column_key" gorm:"size:100;not null"`
	IsVisible   bool      `json:"is_visible" gorm:"default:true"`
	ColumnOrder int       `json:"column_order" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Rpt2UserColumnPref) TableName() string { return "rpt2_user_column_prefs" }

// =============================================================================
// RPT2_DOWNLOAD_LOGS — audit trail download
// =============================================================================

type Rpt2DownloadLog struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID uint      `json:"template_id" gorm:"not null;index"`
	UserID     uint      `json:"user_id" gorm:"not null;index"`
	ParamsJSON string    `json:"params_json" gorm:"type:text"` // snapshot parameter saat download
	RowCount   int       `json:"row_count"`
	DownloadAt time.Time `json:"download_at"`
}

func (Rpt2DownloadLog) TableName() string { return "rpt2_download_logs" }

// =============================================================================
// DTOs
// =============================================================================

type ReqCreateTemplate struct {
	Code        string `json:"code" validate:"required,min=3,max=50"`
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type ReqUpdateTemplate struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsActive    bool   `json:"is_active"`
}

type ReqSaveSheet struct {
	SheetName  string          `json:"sheet_name" validate:"required,max=50"`
	SqlQuery   string          `json:"sql_query" validate:"required"`
	SheetOrder int             `json:"sheet_order"`
	Columns    []ReqSaveColumn `json:"columns" validate:"required,min=1"`
}

type ReqSaveColumn struct {
	ColumnKey        string `json:"column_key" validate:"required"`
	DisplayName      string `json:"display_name" validate:"required"`
	ColumnOrder      int    `json:"column_order"`
	ExcelWidth       int    `json:"excel_width"`
	ExcelFormat      string `json:"excel_format"`
	IsDefaultVisible bool   `json:"is_default_visible"`
	IsToggleable     bool   `json:"is_toggleable"`
}

type ReqSaveParam struct {
	ParamKey         string `json:"param_key" validate:"required"`
	Label            string `json:"label" validate:"required"`
	ParamType        string `json:"param_type" validate:"required,oneof=DATE DATERANGE SELECT TEXT NUMBER"`
	DefaultValue     string `json:"default_value"`
	OptionsQuery     string `json:"options_query"`
	ApplicableSheets string `json:"applicable_sheets"`
	IsRequired       bool   `json:"is_required"`
	ParamOrder       int    `json:"param_order"`
}

type ReqGenerateReport struct {
	TemplateID uint              `json:"template_id" validate:"required"`
	Params     map[string]string `json:"params"`
	// ColumnPrefs per sheet: key = sheet_id (string), value = list column_key yang visible
	ColumnPrefs map[string][]string `json:"column_prefs"`
}

type ReqUpdateColumnPrefs struct {
	SheetID     uint                `json:"sheet_id" validate:"required"`
	ColumnPrefs []ReqColumnPrefItem `json:"column_prefs" validate:"required,min=1"`
}

type ReqColumnPrefItem struct {
	ColumnKey   string `json:"column_key" validate:"required"`
	IsVisible   bool   `json:"is_visible"`
	ColumnOrder int    `json:"column_order"`
}
