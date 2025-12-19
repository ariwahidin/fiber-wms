package models

import (
	"gorm.io/gorm"
)

type Inventory struct {
	gorm.Model
	InventoryNumber int     `json:"inventory_number" gorm:"unique"`
	OwnerCode       string  `json:"owner_code"`
	WhsCode         string  `json:"whs_code"`
	DivisionCode    string  `json:"division_code"`
	InboundID       int     `json:"inbound_id" gorm:"default:null"`
	InboundDetailId int     `json:"inbound_detail_id"`
	RecDate         string  `json:"rec_date" gorm:"default:null"`
	ProdDate        string  `json:"prod_date" gorm:"default:null"`
	ExpDate         string  `json:"exp_date" gorm:"default:null"`
	LotNumber       string  `json:"lot_number" gorm:"default:null"`
	Pallet          string  `json:"pallet"`
	Location        string  `json:"location"`
	ItemId          int     `json:"item_id"`
	ItemCode        string  `json:"item_code"`
	Barcode         string  `json:"barcode" gorm:"not null" validate:"required"`
	QaStatus        string  `json:"qa_status"`
	Uom             string  `json:"uom"`
	QtyOrigin       float64 `json:"qty_origin" gorm:"default:0"`
	QtyOnhand       float64 `json:"qty_onhand" gorm:"default:0"`
	QtyAvailable    float64 `json:"qty_available" gorm:"default:0"`
	QtyAllocated    float64 `json:"qty_allocated" gorm:"default:0"`
	QtySuspend      float64 `json:"qty_suspend" gorm:"default:0"`
	QtyShipped      float64 `json:"qty_shipped" gorm:"default:0"`
	Trans           string  `json:"trans"`
	IsTransfer      bool    `json:"is_transfer" gorm:"default:false"`
	TransferFrom    uint    `json:"transfer_from" gorm:"default:null"`
	CreatedBy       int
	UpdatedBy       int
	DeletedBy       int
}

func (p *Inventory) BeforeCreate(tx *gorm.DB) (err error) {
	var lastInventory Inventory

	if err := tx.Select("inventory_number").Order("inventory_number desc").First(&lastInventory).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			p.InventoryNumber = 1
		} else {
			return err
		}
	} else {
		p.InventoryNumber = lastInventory.InventoryNumber + 1
	}

	return nil
}

type InventoryPolicy struct {
	gorm.Model
	OwnerCode               string `gorm:"not null" validate:"required" json:"owner_code" `
	UseLotNo                bool   `gorm:"default:false" json:"use_lot_no"`
	UseFIFO                 bool   `gorm:"default:false" json:"use_fifo"`
	UseFEFO                 bool   `gorm:"default:false" json:"use_fefo"`
	UseVAS                  bool   `gorm:"default:false" json:"use_vas"`
	UseProductionDate       bool   `gorm:"default:false" json:"use_production_date"`
	UseReceiveLocation      bool   `gorm:"default:false" json:"use_receive_location"`
	ShowRecDate             bool   `gorm:"default:false" json:"show_rec_date"`
	RequireExpiryDate       bool   `gorm:"default:false" json:"require_expiry_date"`
	RequireLotNumber        bool   `gorm:"default:false" json:"require_lot_number"`
	RequireScanPickLocation bool   `gorm:"default:false" json:"require_scan_pick_location"`
	AllowMixedLot           bool   `gorm:"default:false" json:"allow_mixed_lot"`
	AllowNegativeStock      bool   `gorm:"default:false" json:"allow_negative_stock"`
	ValidationSN            bool   `gorm:"default:false" json:"validation_sn"`
	CreatedBy               int
	UpdatedBy               int
	DeletedBy               int
}

// func (i *Inventory) BeforeCreate(tx *gorm.DB) (err error) {
// 	fmt.Println("ID Inventory Before Create:", i.ID)
// 	if i.ID == 0 {
// 		i.ID = types.SnowflakeID(idgen.GenerateID())
// 	}
// 	return nil
// }
