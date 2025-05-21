package models

import "gorm.io/gorm"

// Menu Model
// type Menu struct {
// 	gorm.Model
// 	Name        string       `json:"name"`                        // Nama menu (misal: Dashboard)
// 	Path        string       `json:"path"`                        // URL path (misal: /dashboard)
// 	Icon        string       `json:"icon"`                        // Nama ikon (opsional)
// 	Order       int          `json:"order"`                       // Urutan tampil
// 	ParentID    *uint        `json:"parent_id"`                   // Self-referencing untuk submenu
// 	Parent      *Menu        `gorm:"foreignKey:ParentID"`         // Referensi ke parent
// 	Children    []Menu       `gorm:"foreignKey:ParentID"`         // Daftar submenu
// 	Permissions []Permission `gorm:"many2many:menu_permissions;"` // Relasi ke permission
// 	CreatedBy   int
// 	UpdatedBy   int
// 	DeletedBy   int
// }

type Menu struct {
	gorm.Model
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Path        string       `json:"path"`
	Icon        string       `json:"icon"`
	MenuOrder   int          `json:"menu_order" gorm:"column:menu_order"`
	ParentID    *uint        `json:"parent_id"`
	Parent      *Menu        `gorm:"foreignKey:ParentID"`
	Children    []Menu       `gorm:"foreignKey:ParentID"`
	Permissions []Permission `gorm:"many2many:menu_permissions;"`
	CreatedBy   int
	UpdatedBy   int
	DeletedBy   int
}
