package models

import (
	"encoding/json"
	"fiber-app/controllers/idgen"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

type UomConversion struct {
	ID             int64   `json:"ID" gorm:"primaryKey"`
	ItemCode       string  `json:"item_code"`
	FromUom        string  `json:"from_uom"`
	ToUom          string  `json:"to_uom"`
	ConversionRate float64 `json:"conversion_rate"`
	IsBase         bool    `json:"is_base"`
	CreatedAt      time.Time
	CreatedBy      int
	UpdatedAt      time.Time
	UpdatedBy      int
	DeletedAt      gorm.DeletedAt
	DeletedBy      int
}

type UomConversionInput struct {
	ItemCode       string `json:"item_code"`
	FromUom        string `json:"from_uom"`
	ToUom          string `json:"to_uom"`
	ConversionRate int    `json:"conversion_rate"`
	IsBase         bool   `json:"is_base"`
}

func (u *UomConversion) BeforeCreate(tx *gorm.DB) (err error) {
	fmt.Println("🔥 ID being generated...")
	u.ID = idgen.GenerateID()
	fmt.Println("✅ ID generated:", u.ID)
	return nil
}

// Custom JSON output (convert ID to string)
func (u UomConversion) MarshalJSON() ([]byte, error) {
	type Alias UomConversion
	return json.Marshal(&struct {
		ID string `json:"ID"`
		Alias
	}{
		ID:    strconv.FormatInt(u.ID, 10),
		Alias: (Alias)(u),
	})
}
