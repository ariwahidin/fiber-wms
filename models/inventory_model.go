package models

import (
	"fiber-app/types"

	"gorm.io/gorm"
)

type Inventory struct {
	gorm.Model
	ID              types.SnowflakeID `json:"ID" gorm:"primaryKey"`
	OwnerCode       string            `json:"owner_code"`
	WhsCode         string            `json:"whs_code"`
	DivisionCode    string            `json:"division_code"`
	InboundID       types.SnowflakeID `json:"inbound_id" gorm:"default:null"`
	InboundDetailId int               `json:"inbound_detail_id"`
	RecDate         string            `json:"rec_date"`
	Pallet          string            `json:"pallet"`
	Location        string            `json:"location"`
	ItemId          int               `json:"item_id"`
	ItemCode        string            `json:"item_code"`
	Barcode         string            `json:"barcode" gorm:"not null" validate:"required"`
	QaStatus        string            `json:"qa_status"`
	Uom             string            `json:"uom"`
	QtyOrigin       int               `json:"qty_origin" gorm:"default:0"`
	QtyOnhand       int               `json:"qty_onhand" gorm:"default:0"`
	QtyAvailable    int               `json:"qty_available" gorm:"default:0"`
	QtyAllocated    int               `json:"qty_allocated" gorm:"default:0"`
	QtySuspend      int               `json:"qty_suspend" gorm:"default:0"`
	QtyShipped      int               `json:"qty_shipped" gorm:"default:0"`
	Trans           string            `json:"trans"`
	IsTransfer      bool              `json:"is_transfer" gorm:"default:false"`
	TransferFrom    types.SnowflakeID `json:"transfer_from" gorm:"default:null"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
}

// func (i *Inventory) BeforeCreate(tx *gorm.DB) (err error) {
// 	fmt.Println("ID Inventory Before Create:", i.ID)
// 	if i.ID == 0 {
// 		i.ID = types.SnowflakeID(idgen.GenerateID())
// 	}
// 	return nil
// }
