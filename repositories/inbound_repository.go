package repositories

import (
	"errors"
	"fiber-app/models"

	"gorm.io/gorm"
)

type InboundRepository struct {
	db *gorm.DB
}

func NewInboundRepository(db *gorm.DB) *InboundRepository {
	return &InboundRepository{db}
}

// CreateInboundDetail function dengan transaction
func (r *InboundRepository) CreateInboundDetail(data *models.InboundDetail, handlingUsed []HandlingDetailUsed) (uint, error) {
	// Mulai transaksi
	tx := r.db.Begin()
	if tx.Error != nil {
		return 0, errors.New("failed to start transaction")
	}

	// Jika terjadi panic, rollback transaksi
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Insert ke Inbound Detail
	if err := tx.Create(data).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Ambil ID yang baru saja diinsert
	inboundDetailID := data.ID

	// Insert ke Inbound Detail Handlings
	for _, handling := range handlingUsed {
		inboundDetailHandling := models.InboundDetailHandling{
			InboundDetailId:   int(inboundDetailID),
			HandlingId:        handling.HandlingID,
			HandlingUsed:      handling.HandlingUsed,
			HandlingCombineId: handling.HandlingCombineID,
			OriginHandlingId:  handling.OriginHandlingID,
			OriginHandling:    handling.OriginHandling,
			RateId:            handling.RateID,
			RateIdr:           handling.RateIDR,
			CreatedBy:         int(data.CreatedBy),
		}

		if err := tx.Create(&inboundDetailHandling).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return inboundDetailID, nil
}

func (r *InboundRepository) UpdateInboundDetail(data *models.InboundDetail, handlingUsed []HandlingDetailUsed) (uint, error) {
	// Mulai transaksi
	tx := r.db.Begin()
	if tx.Error != nil {
		return 0, errors.New("failed to start transaction")
	}

	// Jika terjadi panic, rollback transaksi
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Insert ke Inbound Detail
	if err := tx.Create(data).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Ambil ID yang baru saja diinsert
	inboundDetailID := data.ID

	// Insert ke Inbound Detail Handlings
	for _, handling := range handlingUsed {
		inboundDetailHandling := models.InboundDetailHandling{
			InboundDetailId:   int(inboundDetailID),
			HandlingId:        handling.HandlingID,
			HandlingUsed:      handling.HandlingUsed,
			HandlingCombineId: handling.HandlingCombineID,
			OriginHandlingId:  handling.OriginHandlingID,
			OriginHandling:    handling.OriginHandling,
			RateId:            handling.RateID,
			RateIdr:           handling.RateIDR,
			CreatedBy:         int(data.CreatedBy),
		}

		if err := tx.Create(&inboundDetailHandling).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return inboundDetailID, nil
}
