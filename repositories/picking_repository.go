package repositories

import (
	"errors"
	"fiber-app/models"
	"fmt"

	"gorm.io/gorm"
)

type OutboundPickingRepository struct {
	DB *gorm.DB
}

func NewOutboundPickingRepository(DB *gorm.DB) *OutboundPickingRepository {
	return &OutboundPickingRepository{DB: DB}
}

// ─── GetPickingSheet ──────────────────────────────────────────────────────────
// Ambil list OutboundPicking (picking sheet) untuk satu outbound beserta
// progress scan (qty_picked dihitung dari outbound_picking_scans).

type PickingSheetRow struct {
	OutboundPickingID int     `json:"outbound_picking_id"`
	OutboundDetailID  int     `json:"outbound_detail_id"`
	OutboundNo        string  `json:"outbound_no"`
	ItemID            uint    `json:"item_id"`
	ItemCode          string  `json:"item_code"`
	ItemName          string  `json:"item_name"`
	Barcode           string  `json:"barcode"`
	Location          string  `json:"location"`
	Pallet            string  `json:"pallet"`
	Uom               string  `json:"uom"`
	QtyRequired       float64 `json:"qty_required"`
	QtyPicked         float64 `json:"qty_picked"`
	IsComplete        bool    `json:"is_complete"`
	LotNumber         string  `json:"lot_number"`
	ProdDate          string  `json:"prod_date"`
	ExpDate           string  `json:"exp_date"`
}

func (r *OutboundPickingRepository) GetPickingSheet(outboundNo string) ([]PickingSheetRow, error) {
	sql := `
		SELECT
			op.id                                        AS outbound_picking_id,
			op.outbound_detail_id,
			op.outbound_no,
			op.item_id,
			op.item_code,
			ISNULL(p.item_name, '')                      AS item_name,
			op.barcode,
			op.location,
			op.pallet,
			op.uom,
			op.quantity                                  AS qty_required,
			ISNULL(SUM(ops.quantity), 0)                 AS qty_picked,
			CASE WHEN ISNULL(SUM(ops.quantity), 0) >= op.quantity THEN 1 ELSE 0 END AS is_complete,
			ISNULL(op.lot_number, '')                    AS lot_number,
			ISNULL(op.prod_date, '')                     AS prod_date,
			ISNULL(op.exp_date, '')                      AS exp_date
		FROM outbound_pickings op
		LEFT JOIN products p ON op.item_id = p.id
		LEFT JOIN outbound_picking_scans ops
			ON ops.outbound_picking_id = op.id
			AND ops.deleted_at IS NULL
		WHERE op.outbound_no = ?
		  AND op.deleted_at IS NULL
		GROUP BY
			op.id, op.outbound_detail_id, op.outbound_no,
			op.item_id, op.item_code, p.item_name,
			op.barcode, op.location, op.pallet, op.uom,
			op.quantity, op.lot_number, op.prod_date, op.exp_date
		ORDER BY op.id ASC`

	var rows []PickingSheetRow
	if err := r.DB.Raw(sql, outboundNo).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ─── GetScans ─────────────────────────────────────────────────────────────────
// Ambil semua scan untuk outbound_picking_id tertentu.

func (r *OutboundPickingRepository) GetScans(outboundPickingID int) ([]models.OutboundPickingScan, error) {
	var scans []models.OutboundPickingScan
	if err := r.DB.
		Where("outbound_picking_id = ? AND deleted_at IS NULL", outboundPickingID).
		Order("id ASC").
		Find(&scans).Error; err != nil {
		return nil, err
	}
	return scans, nil
}

// ─── GetQtyPicked ─────────────────────────────────────────────────────────────
// Total qty yang sudah di-scan untuk satu OutboundPicking.

func (r *OutboundPickingRepository) GetQtyPicked(outboundPickingID int) (float64, error) {
	var total float64
	err := r.DB.Model(&models.OutboundPickingScan{}).
		Where("outbound_picking_id = ? AND deleted_at IS NULL", outboundPickingID).
		Select("ISNULL(SUM(quantity), 0)").
		Scan(&total).Error
	return total, err
}

// ─── FindPickingByBarcode ─────────────────────────────────────────────────────
// Cari OutboundPicking yang cocok berdasarkan outbound_no + barcode.
// Dipakai untuk matching saat scan.

func (r *OutboundPickingRepository) FindPickingByBarcode(outboundNo, barcode string) (*models.OutboundPicking, error) {
	var picking models.OutboundPicking
	err := r.DB.
		Where("outbound_no = ? AND barcode = ? AND deleted_at IS NULL", outboundNo, barcode).
		First(&picking).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &picking, nil
}

// ─── FindPickingByItemCode ────────────────────────────────────────────────────
// Fallback: cari berdasarkan item_code jika barcode tidak match.

func (r *OutboundPickingRepository) FindPickingByItemCode(outboundNo, itemCode string) (*models.OutboundPicking, error) {
	var picking models.OutboundPicking
	err := r.DB.
		Where("outbound_no = ? AND item_code = ? AND deleted_at IS NULL", outboundNo, itemCode).
		First(&picking).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &picking, nil
}

// ─── CreateScan ───────────────────────────────────────────────────────────────

func (r *OutboundPickingRepository) CreateScan(scan *models.OutboundPickingScan) error {
	return r.DB.Create(scan).Error
}

// ─── DeleteScan ───────────────────────────────────────────────────────────────
// Soft delete satu scan record (gorm.Model sudah handle deleted_at).

func (r *OutboundPickingRepository) DeleteScan(scanID uint) error {
	result := r.DB.Delete(&models.OutboundPickingScan{}, scanID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("scan record %d not found", scanID)
	}
	return nil
}

// ─── IsPickingComplete ────────────────────────────────────────────────────────
// Cek apakah semua item di picking sheet sudah complete (qty_picked >= qty_required).

func (r *OutboundPickingRepository) IsPickingComplete(outboundNo string) (bool, error) {
	sql := `
		SELECT COUNT(*) FROM outbound_pickings op
		WHERE op.outbound_no = ?
		  AND op.deleted_at IS NULL
		  AND op.quantity > (
		      SELECT ISNULL(SUM(ops.quantity), 0)
		      FROM outbound_picking_scans ops
		      WHERE ops.outbound_picking_id = op.id
		        AND ops.deleted_at IS NULL
		  )`
	var incompleteCount int64
	if err := r.DB.Raw(sql, outboundNo).Scan(&incompleteCount).Error; err != nil {
		return false, err
	}
	return incompleteCount == 0, nil
}

// ─── ConfirmPicking ───────────────────────────────────────────────────────────
// Update status outbound_header → 'packing' dan semua scan → 'confirmed'.

func (r *OutboundPickingRepository) ConfirmPicking(outboundNo string, userID int) error {
	tx := r.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Update semua scan jadi confirmed
	if err := tx.Model(&models.OutboundPickingScan{}).
		Where("outbound_no = ? AND deleted_at IS NULL", outboundNo).
		Updates(map[string]interface{}{
			"status":     "confirmed",
			"updated_by": userID,
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Update outbound_header status
	if err := tx.Model(&models.OutboundHeader{}).
		Where("outbound_no = ?", outboundNo).
		Updates(map[string]interface{}{
			"status":     "packing",
			"raw_status": "PACKING",
			"updated_by": userID,
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
