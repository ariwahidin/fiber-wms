package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username    string       `json:"username" gorm:"unique"`
	Password    string       `json:"password"`
	Name        string       `json:"name"`
	Email       string       `json:"email" gorm:"unique"`
	Role        string       `json:"role"`
	Roles       []Role       `gorm:"many2many:user_roles;"`
	Permissions []Permission `gorm:"many2many:user_permissions;"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

// Role Model
type Role struct {
	gorm.Model
	Name        string
	Description string
	Permissions []Permission `gorm:"many2many:role_permissions;"`
}

// Permission Model
type Permission struct {
	gorm.Model
	Name        string
	Description string
}
