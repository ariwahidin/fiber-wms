package repositories

import (
	"fiber-app/models"
	"time"

	"gorm.io/gorm"
)

type AdjustmentRepository struct {
	DB *gorm.DB
}

func NewAdjustmentRepository(db *gorm.DB) *AdjustmentRepository {
	return &AdjustmentRepository{DB: db}
}

type AdjustmentFilter struct {
	Status    string
	OwnerCode string
	WhsCode   string
	ItemCode  string
	DateFrom  string
	DateTo    string
	Search    string
	Page      int
	PageSize  int
}

type AdjustmentListResult struct {
	Data       []models.InventoryAdjustment
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

func (r *AdjustmentRepository) GetAll(filter AdjustmentFilter) (*AdjustmentListResult, error) {
	query := r.DB.Model(&models.InventoryAdjustment{}).
		Preload("Inventory").
		Preload("Inventory.Product")

	if filter.Status != "" {
		query = query.Where("inventory_adjustments.status = ?", filter.Status)
	}
	if filter.OwnerCode != "" {
		query = query.Where("inventory_adjustments.owner_code = ?", filter.OwnerCode)
	}
	if filter.WhsCode != "" {
		query = query.Where("inventory_adjustments.whs_code = ?", filter.WhsCode)
	}
	if filter.ItemCode != "" {
		query = query.Where("inventory_adjustments.item_code = ?", filter.ItemCode)
	}
	if filter.DateFrom != "" {
		query = query.Where("inventory_adjustments.requested_at >= ?", filter.DateFrom)
	}
	if filter.DateTo != "" {
		query = query.Where("inventory_adjustments.requested_at <= ?", filter.DateTo+" 23:59:59")
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		query = query.Where(
			"inventory_adjustments.adj_number LIKE ? OR inventory_adjustments.item_code LIKE ? OR inventory_adjustments.reason_code LIKE ?",
			like, like, like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	offset := (filter.Page - 1) * filter.PageSize
	var records []models.InventoryAdjustment
	if err := query.
		Order("inventory_adjustments.requested_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / filter.PageSize
	if int(total)%filter.PageSize > 0 {
		totalPages++
	}

	return &AdjustmentListResult{
		Data:       records,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (r *AdjustmentRepository) GetByID(id uint) (*models.InventoryAdjustment, error) {
	var adj models.InventoryAdjustment
	err := r.DB.
		Preload("Inventory").
		Preload("Inventory.Product").
		First(&adj, id).Error
	if err != nil {
		return nil, err
	}
	return &adj, nil
}

func (r *AdjustmentRepository) Create(adj *models.InventoryAdjustment) error {
	return r.DB.Create(adj).Error
}

func (r *AdjustmentRepository) UpdateStatus(tx *gorm.DB, id uint, status string, userID int, rejectNote string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_by": userID,
	}

	now := time.Now()
	switch status {
	case models.AdjStatusApproved:
		updates["approved_by"] = userID
		updates["approved_at"] = now
	case models.AdjStatusRejected:
		updates["rejected_by"] = userID
		updates["rejected_at"] = now
		updates["reject_note"] = rejectNote
	case models.AdjStatusApplied:
		updates["applied_at"] = now
	}

	return tx.Model(&models.InventoryAdjustment{}).Where("id = ?", id).Updates(updates).Error
}

func (r *AdjustmentRepository) GetReasonCodes() ([]models.AdjustmentReasonCode, error) {
	var codes []models.AdjustmentReasonCode
	err := r.DB.Order("code ASC").Find(&codes).Error
	return codes, err
}
