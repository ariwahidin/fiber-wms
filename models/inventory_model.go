package models

import (
	"time"

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
	ItemId          uint    `json:"item_id"`
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
	Product         Product `json:"product" gorm:"foreignKey:ItemId;references:ID"`
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
	RequirePickingScan      bool   `gorm:"default:false" json:"require_picking_scan"`
	RequirePackingScan      bool   `gorm:"default:false" json:"require_packing_scan"`
	PickingSingleScan       bool   `gorm:"default:false" json:"picking_single_scan"`
	RequireReceiveScan      bool   `gorm:"default:false" json:"require_receive_scan"`
	CreatedBy               int
	UpdatedBy               int
	DeletedBy               int
}

type InventoryMovement struct {
	ID         uint `gorm:"primaryKey"`
	MovementID string

	InventoryID uint `gorm:"index;not null"`

	// Referensi barang
	ItemID   uint
	ItemCode string

	// Referensi proses
	RefType string `gorm:"size:50;index"` // inbound, outbound, allocate, release, transfer, adjust, qc
	RefID   uint   `gorm:"index"`

	// Perubahan kuantitas (DELTA)
	QtyOnhandChange    float64 `gorm:"default:0"`
	QtyAvailableChange float64 `gorm:"default:0"`
	QtyAllocatedChange float64 `gorm:"default:0"`
	QtySuspendChange   float64 `gorm:"default:0"`
	QtyShippedChange   float64 `gorm:"default:0"`

	// Konteks whs_code, lokasi & status
	FromWhsCode  string `gorm:"size:20"`
	ToWhsCode    string `gorm:"size:20"`
	FromLocation string `gorm:"size:100"`
	ToLocation   string `gorm:"size:100"`
	OldQaStatus  string `gorm:"size:50"`
	NewQaStatus  string `gorm:"size:50"`

	// Metadata
	Reason    string `gorm:"size:255"`
	CreatedBy int
	CreatedAt time.Time
}

// func (i *Inventory) BeforeCreate(tx *gorm.DB) (err error) {
// 	fmt.Println("ID Inventory Before Create:", i.ID)
// 	if i.ID == 0 {
// 		i.ID = types.SnowflakeID(idgen.GenerateID())
// 	}
// 	return nil
// }

// qa_status_change_request table
type QAStatusChangeRequest struct {
	gorm.Model
	InventoryID     int     `json:"inventory_id"`
	InventoryNumber int     `json:"inventory_number"`
	CurrentStatus   string  `json:"current_status"`
	RequestedStatus string  `json:"requested_status"`
	Quantity        float64 `json:"quantity"`
	ReasonCode      string  `json:"reason_code"`
	ReasonNotes     string  `json:"reason_notes"`
	RequestedBy     string  `json:"requested_by"`
	RequestedDate   string  `json:"requested_date"`
	ApprovalStatus  string  `json:"approval_status"` // PENDING, APPROVED, REJECTED
	ApprovedBy      string  `json:"approved_by"`
	ApprovedDate    string  `json:"approved_date"`
	ApprovalNotes   string  `json:"approval_notes"`
}

// qa_status_change_reason (master table)
type QAStatusChangeReason struct {
	gorm.Model
	Code             string `json:"code" gorm:"unique"`
	Description      string `json:"description"`
	FromStatus       string `json:"from_status"`
	ToStatus         string `json:"to_status"`
	RequiresApproval bool   `json:"requires_approval"`
}
