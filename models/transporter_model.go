package models

import "gorm.io/gorm"

type Transporter struct {
	gorm.Model
	TransporterCode    string `json:"transporter_code" gorm:"unique"`
	TransporterName    string `json:"transporter_name"`
	TransporterAddress string `json:"transporter_address"`
	City               string `json:"city"`
	Pic                string `json:"pic"`
	Phone              string `json:"phone"`
	Email              string `json:"email"`
	Fax                string `json:"fax"`
	IsActive           bool   `json:"is_active" gorm:"default:true"`
	CreatedBy          int
	UpdatedBy          int
	DeletedBy          int
}
