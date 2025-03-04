package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username  string `json:"username" gorm:"unique"`
	Password  string `json:"password"`
	Name      string `json:"name"`
	Email     string `json:"email" gorm:"unique"`
	Role      string `json:"role"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}
