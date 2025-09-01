package models

import "gorm.io/gorm"

type Handling struct {
	gorm.Model
	Name      string `json:"name" gorm:"unique"`
	Type      string `json:"type" gorm:"default:'single'"`
	IsKoli    bool   `json:"is_koli" gorm:"default:false"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}

type HandlingRate struct {
	gorm.Model
	HandlingId int
	Name       string
	RateIdr    int `json:"rate_idr" gorm:"default:0"`
	CreatedBy  int
	UpdatedBy  int
	DeletedBy  int
}

type HandlingCombine struct {
	gorm.Model
	HandlingId int
	CreatedBy  int
	UpdatedBy  int
	DeletedBy  int
}

type HandlingCombineDetail struct {
	gorm.Model
	HandlingCombineId int
	HandlingId        int
	CreatedBy         int
	UpdatedBy         int
	DeletedBy         int
}

type HandlingItem struct {
	gorm.Model
	ItemCode  string `json:"item_code"`
	Area      string `json:"area"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int

	Details []HandlingItemDetail `json:"details" gorm:"foreignKey:HandlingItemId;references:ID"`
}

type HandlingItemDetail struct {
	gorm.Model
	HandlingItemId int
	ItemCode       string `json:"item_code"`
	Handling       string
	CreatedBy      int
	UpdatedBy      int
	DeletedBy      int
}
