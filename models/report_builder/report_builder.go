package report_builder

import "time"

// =============================================================================
// RPT_REPORTS — definisi report (query-based atau document)
// =============================================================================

type RptReport struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportCode   string    `json:"report_code" gorm:"uniqueIndex;size:50;not null"`
	ReportName   string    `json:"report_name" gorm:"size:100;not null"`
	ReportType   string    `json:"report_type" gorm:"size:20;not null"` // QUERY | DOCUMENT
	BaseQuery    string    `json:"base_query" gorm:"type:text"`
	DocumentType string    `json:"document_type" gorm:"size:50"` // PICKING_LIST | SPK | PACKING_LIST | nil
	IsActive     bool      `json:"is_active" gorm:"default:true"`
	CreatedBy    int       `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedBy    int       `json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relasi
	Fields []RptReportField `json:"fields,omitempty" gorm:"foreignKey:ReportID"`
}

func (RptReport) TableName() string { return "rpt_reports" }

// =============================================================================
// RPT_REPORT_FIELDS — field/kolom yang tersedia per report
// =============================================================================

type RptReportField struct {
	ID           uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID     uint   `json:"report_id" gorm:"not null;index"`
	FieldKey     string `json:"field_key" gorm:"size:100;not null"`   // nama kolom SQL, e.g. "item_code"
	FieldLabel   string `json:"field_label" gorm:"size:100;not null"` // label tampilan, e.g. "Item Code"
	FieldType    string `json:"field_type" gorm:"size:20;not null"`   // STRING | NUMBER | DATE | BOOLEAN
	IsFilterable bool   `json:"is_filterable" gorm:"default:true"`
	IsSortable   bool   `json:"is_sortable" gorm:"default:true"`
	SortOrder    int    `json:"sort_order" gorm:"default:0"`

	// Relasi
	Report RptReport `json:"-" gorm:"foreignKey:ReportID"`
}

func (RptReportField) TableName() string { return "rpt_report_fields" }

// =============================================================================
// RPT_LAYOUTS — layout yang disimpan user
// =============================================================================

type RptLayout struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID   uint      `json:"report_id" gorm:"not null;index"`
	LayoutName string    `json:"layout_name" gorm:"size:100;not null"`
	OwnerCode  string    `json:"owner_code" gorm:"size:50"` // kosong = semua owner
	IsDefault  bool      `json:"is_default" gorm:"default:false"`
	IsPublic   bool      `json:"is_public" gorm:"default:false"` // bisa dipakai user lain
	CreatedBy  int       `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedBy  int       `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relasi
	Report         RptReport          `json:"report,omitempty" gorm:"foreignKey:ReportID"`
	Columns        []RptLayoutColumn  `json:"columns,omitempty" gorm:"foreignKey:LayoutID"`
	Filters        []RptLayoutFilter  `json:"filters,omitempty" gorm:"foreignKey:LayoutID"`
	DocumentConfig *RptDocumentConfig `json:"document_config,omitempty" gorm:"foreignKey:LayoutID"`
}

func (RptLayout) TableName() string { return "rpt_layouts" }

// =============================================================================
// RPT_LAYOUT_COLUMNS — kolom yang dipilih dalam layout + urutannya
// =============================================================================

type RptLayoutColumn struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	LayoutID    uint   `json:"layout_id" gorm:"not null;index"`
	FieldID     uint   `json:"field_id" gorm:"not null"`
	ColumnLabel string `json:"column_label" gorm:"size:100"` // override label
	ColumnOrder int    `json:"column_order" gorm:"not null"`
	ColumnWidth int    `json:"column_width" gorm:"default:120"`
	IsVisible   bool   `json:"is_visible" gorm:"default:true"`

	// Relasi
	Field RptReportField `json:"field,omitempty" gorm:"foreignKey:FieldID"`
}

func (RptLayoutColumn) TableName() string { return "rpt_layout_columns" }

// =============================================================================
// RPT_LAYOUT_FILTERS — filter yang disimpan dalam layout
// =============================================================================

type RptLayoutFilter struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	LayoutID    uint   `json:"layout_id" gorm:"not null;index"`
	FieldID     uint   `json:"field_id" gorm:"not null"`
	Operator    string `json:"operator" gorm:"size:20;not null"` // EQ | LIKE | BETWEEN | IN | GTE | LTE
	FilterValue string `json:"filter_value" gorm:"size:500"`     // default value, bisa di-override saat generate
	IsRequired  bool   `json:"is_required" gorm:"default:false"`

	// Relasi
	Field RptReportField `json:"field,omitempty" gorm:"foreignKey:FieldID"`
}

func (RptLayoutFilter) TableName() string { return "rpt_layout_filters" }

// =============================================================================
// RPT_DOCUMENT_CONFIGS — konfigurasi layout cetak (khusus tipe DOCUMENT)
// =============================================================================

type RptDocumentConfig struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	LayoutID    uint   `json:"layout_id" gorm:"not null;uniqueIndex"`
	PaperSize   string `json:"paper_size" gorm:"size:20;default:'A4'"` // A4 | A5 | LETTER
	Orientation string `json:"orientation" gorm:"size:10;default:'P'"` // P | L
	HeaderJSON  string `json:"header_json" gorm:"type:text"`           // JSON config header
	FooterJSON  string `json:"footer_json" gorm:"type:text"`           // JSON config footer
	ShowPageNum bool   `json:"show_page_num" gorm:"default:true"`
	RowsPerPage int    `json:"rows_per_page" gorm:"default:30"`
}

func (RptDocumentConfig) TableName() string { return "rpt_document_configs" }

// =============================================================================
// DTO — tidak disimpan ke DB, hanya untuk request/response
// =============================================================================

// Request generate report
type ReqGenerateReport struct {
	LayoutID uint              `json:"layout_id" validate:"required"`
	Format   string            `json:"format" validate:"required,oneof=excel pdf csv"` // excel | pdf | csv
	Filters  map[string]string `json:"filters"`                                        // key = field_key, value = filter value
}

// Request simpan layout baru
type ReqSaveLayout struct {
	ReportID   uint                  `json:"report_id" validate:"required"`
	LayoutName string                `json:"layout_name" validate:"required"`
	OwnerCode  string                `json:"owner_code"`
	IsDefault  bool                  `json:"is_default"`
	IsPublic   bool                  `json:"is_public"`
	Columns    []ReqSaveLayoutColumn `json:"columns" validate:"required,min=1"`
	Filters    []ReqSaveLayoutFilter `json:"filters"`
}

type ReqSaveLayoutColumn struct {
	FieldID     uint   `json:"field_id" validate:"required"`
	ColumnLabel string `json:"column_label"`
	ColumnOrder int    `json:"column_order" validate:"required"`
	ColumnWidth int    `json:"column_width"`
	IsVisible   bool   `json:"is_visible"`
}

type ReqSaveLayoutFilter struct {
	FieldID     uint   `json:"field_id" validate:"required"`
	Operator    string `json:"operator" validate:"required"`
	FilterValue string `json:"filter_value"`
	IsRequired  bool   `json:"is_required"`
}

// Response layout dengan detail lengkap
type ResLayoutDetail struct {
	RptLayout
	ReportCode string `json:"report_code"`
	ReportName string `json:"report_name"`
}

// Header/Footer config untuk document (di-parse dari HeaderJSON/FooterJSON)
type DocumentHeaderConfig struct {
	ShowLogo    bool   `json:"show_logo"`
	LogoPath    string `json:"logo_path"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	ShowAddress bool   `json:"show_address"`
	Address     string `json:"address"`
}

type DocumentFooterConfig struct {
	ShowSignature  bool   `json:"show_signature"`
	SignatureLabel string `json:"signature_label"`
	Notes          string `json:"notes"`
}
