package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type OutboundScan struct {
	gorm.Model
	NoKoli     string               `json:"no_koli"`
	OutboundID uint                 `json:"outbound_id"`
	CreatedBy  int                  `json:"created_by"`
	UpdatedBy  int                  `json:"updated_by"`
	DeletedBy  int                  `json:"deleted_by"`
	Details    []OutboundScanDetail `gorm:"foreignKey:KoliID;references:ID;constraint:OnDelete:CASCADE" json:"details"`
}

type OutboundScanDetail struct {
	gorm.Model
	KoliID           int    `json:"koli_id"`
	NoKoli           string `json:"no_koli"`
	OutboundID       int    `json:"outbound_id"`
	OutboundDetailID int    `json:"outbound_detail_id"`
	InventoryID      int    `json:"inventory_id"`
	ItemID           int    `json:"item_id"`
	PickingSheetID   int    `json:"picking_sheet_id"`
	ItemCode         string `json:"item_code"`
	Barcode          string `json:"barcode"`
	SerialNumber     string `json:"serial_number"`
	Qty              int    `json:"qty"`
	CreatedBy        int    `json:"created_by"`
	UpdatedBy        int    `json:"updated_by"`
	DeletedBy        int    `json:"deleted_by"`
}

// MasterCarton represents the master carton/box configuration
type MasterCarton struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	CartonCode  string `gorm:"uniqueIndex;size:50;not null" json:"carton_code"`
	CartonName  string `gorm:"size:100;not null" json:"carton_name"`
	Description string `gorm:"type:text" json:"description"`

	// Dimensions in cm
	Length float64 `gorm:"type:decimal(10,2);not null" json:"length"`
	Width  float64 `gorm:"type:decimal(10,2);not null" json:"width"`
	Height float64 `gorm:"type:decimal(10,2);not null" json:"height"`

	// Weight limits in kg
	MaxWeight  float64 `gorm:"type:decimal(10,2);not null" json:"max_weight"`
	TareWeight float64 `gorm:"type:decimal(10,2);default:0" json:"tare_weight"` // Empty carton weight

	// Volume in cubic cm (can be auto-calculated)
	Volume float64 `gorm:"type:decimal(10,2)" json:"volume"`

	// Status & Availability
	IsActive  bool `gorm:"default:true;index" json:"is_active"`
	IsDefault bool `gorm:"default:false" json:"is_default"`

	// Additional info
	Material string `gorm:"size:50" json:"material"` // e.g., Cardboard, Plastic, Wood
	Color    string `gorm:"size:30" json:"color"`

	// Audit fields
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	CreatedBy *uint          `json:"created_by,omitempty"`
	UpdatedBy *uint          `json:"updated_by,omitempty"`
}

// TableName specifies the table name for MasterCarton
func (MasterCarton) TableName() string {
	return "master_cartons"
}

// BeforeSave hook to calculate volume automatically
func (mc *MasterCarton) BeforeSave(tx *gorm.DB) error {
	if mc.Length > 0 && mc.Width > 0 && mc.Height > 0 {
		mc.Volume = mc.Length * mc.Width * mc.Height
	}
	return nil
}

// MasterCartonResponse for API responses
type MasterCartonResponse struct {
	ID          uint      `json:"id"`
	CartonCode  string    `json:"carton_code"`
	CartonName  string    `json:"carton_name"`
	Description string    `json:"description"`
	Length      float64   `json:"length"`
	Width       float64   `json:"width"`
	Height      float64   `json:"height"`
	MaxWeight   float64   `json:"max_weight"`
	TareWeight  float64   `json:"tare_weight"`
	Volume      float64   `json:"volume"`
	IsDefault   bool      `json:"is_default"`
	Material    string    `json:"material"`
	CreatedAt   time.Time `json:"created_at"`

	// Formatted display
	Dimensions  string `json:"dimensions"`   // "L x W x H cm"
	DisplayName string `json:"display_name"` // "CartonName (LxWxH)"
}

// ToResponse converts model to response format
func (mc *MasterCarton) ToResponse() MasterCartonResponse {
	return MasterCartonResponse{
		ID:          mc.ID,
		CartonCode:  mc.CartonCode,
		CartonName:  mc.CartonName,
		Description: mc.Description,
		Length:      mc.Length,
		Width:       mc.Width,
		Height:      mc.Height,
		MaxWeight:   mc.MaxWeight,
		TareWeight:  mc.TareWeight,
		Volume:      mc.Volume,
		IsDefault:   mc.IsDefault,
		Material:    mc.Material,
		Dimensions:  formatDimensions(mc.Length, mc.Width, mc.Height),
		DisplayName: formatDisplayName(mc.CartonName, mc.Length, mc.Width, mc.Height),
		CreatedAt:   mc.CreatedAt,
	}
}

func formatDimensions(l, w, h float64) string {
	return fmt.Sprintf("%.0f x %.0f x %.0f cm", l, w, h)
}

func formatDisplayName(name string, l, w, h float64) string {
	return fmt.Sprintf("%s (%.0fx%.0fx%.0f)", name, l, w, h)
}
