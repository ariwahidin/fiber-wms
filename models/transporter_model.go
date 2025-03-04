package models

import "gorm.io/gorm"

type Transporter struct {
	gorm.Model
	TransporterCode    string `json:"transporter_code" gorm:"unique"`
	TransporterName    string `json:"transporter_name" gorm:"unique"`
	TransporterAddress string `json:"transporter_address"`
	CreatedBy          int
	UpdatedBy          int
	DeletedBy          int
}
