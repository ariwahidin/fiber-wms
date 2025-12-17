package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username    string       `json:"username" gorm:"unique"`
	Password    string       `json:"password"`
	Name        string       `json:"name"`
	Email       string       `json:"email" gorm:"unique"`
	Role        string       `json:"role"`
	BaseRoute   string       `json:"base_route"`
	Roles       []Role       `gorm:"many2many:user_roles;"`
	Permissions []Permission `gorm:"many2many:user_permissions;"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

type UserDashboard struct {
	gorm.Model
	Username  string `json:"username" gorm:"unique"`
	Password  string `json:"password"`
	Name      string `json:"name"`
	Email     string `json:"email" gorm:"unique"`
	CreatedBy int
	UpdatedBy int
	DeletedBy int
}

// Role Model
type Role struct {
	gorm.Model
	Name        string       `json:"name" gorm:"unique"`
	Description string       `json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}

// Permission Model
type Permission struct {
	gorm.Model
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int

	Menus []Menu `gorm:"many2many:menu_permissions;"`
}

type LoginLog struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        *uint64    `gorm:"column:user_id"`
	Username      string     `gorm:"column:username;size:50"`
	CustomerID    *uint64    `gorm:"column:customer_id"`
	LoginAt       *time.Time `gorm:"column:login_at"`
	LogoutAt      *time.Time `gorm:"column:logout_at"`
	IPAddress     string     `gorm:"column:ip_address;size:45"`
	UserAgent     string     `gorm:"column:user_agent;size:255"`
	Browser       string     `gorm:"column:browser;size:50"`
	OS            string     `gorm:"column:os;size:50"`
	DeviceType    string     `gorm:"column:device_type;size:20"`
	LoginStatus   string     `gorm:"column:login_status;size:10"`
	FailureReason *string    `gorm:"column:failure_reason;size:50"`
	SessionID     string     `gorm:"column:session_id;size:100"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
}
