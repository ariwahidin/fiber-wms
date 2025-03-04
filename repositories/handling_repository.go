package repositories

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Handling struct {
	CombineID  int       `gorm:"column:combine_id" json:"combine_id"`
	HandlingID int       `gorm:"column:handling_id" json:"handling_id"`
	Name       string    `gorm:"column:name" json:"name"`
	Type       string    `gorm:"column:type" json:"type"`
	RateIDR    int       `gorm:"column:rate_idr" json:"rate_idr"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

type HandlingDetailUsed struct {
	HandlingCombineID int    `gorm:"column:handling_combine_id" json:"handling_combine_id"`
	HandlingID        int    `gorm:"column:handling_id" json:"handling_id"`
	HandlingUsed      string `gorm:"column:handling_used" json:"handling_used"`
	OriginHandlingID  int    `gorm:"column:origin_handling_id" json:"origin_handling_id"`
	OriginHandling    string `gorm:"column:origin_handling" json:"origin_handling"`
	RateID            int    `gorm:"column:rate_id" json:"rate_id"`
	RateIDR           int    `gorm:"column:rate_idr" json:"rate_idr"`
}

type HandlingRepository struct {
	db *gorm.DB
}

func NewHandlingRepository(db *gorm.DB) *HandlingRepository {
	return &HandlingRepository{db}
}

func (r *HandlingRepository) GetHandlingRates() ([]Handling, error) {
	var results []Handling

	query := `
	WITH handling_current AS (
		SELECT 
			a.id, a.handling_id, c.name, c.type, b.handling_id AS id_combine,
			d.rate_id, d.rate_idr, c.updated_at
		FROM handling_combines a
		INNER JOIN handling_combine_details b ON a.id = b.handling_combine_id
		INNER JOIN handlings c ON a.handling_id = c.id
		INNER JOIN (
			SELECT id AS rate_id, handling_id, name, rate_idr
			FROM handling_rates
			WHERE id IN (
				SELECT MAX(id) 
				FROM handling_rates 
				GROUP BY handling_id
			)
		) d ON b.handling_id = d.handling_id
	)
	SELECT id as combine_id, handling_id, name, type, SUM(rate_idr) as rate_idr, updated_at
	FROM handling_current
	GROUP BY id, handling_id, name, type, updated_at`

	err := r.db.Raw(query).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	fmt.Println("results: ", results)

	return results, nil
}

func (r *HandlingRepository) GetHandlingUsed(handlingID int) ([]HandlingDetailUsed, error) {
	var result []HandlingDetailUsed

	query := `	SELECT a.id as handling_combine_id, a.handling_id, c.name as handling_used, 
	b.handling_id as origin_handling_id, d.name as origin_handling,
	e.rate_id, e.rate_idr
	FROM handling_combines a
	INNER JOIN handling_combine_details b ON a.id = b.handling_combine_id
	INNER JOIN handlings c ON a.handling_id = c.id
	INNER JOIN handlings d ON b.handling_id = d.id
	INNER JOIN (
			SELECT id AS rate_id, handling_id, name, rate_idr
				FROM handling_rates
				WHERE id IN (
					SELECT MAX(id) 
					FROM handling_rates 
					GROUP BY handling_id
				)
			) e ON b.handling_id = e.handling_id
	WHERE a.handling_id = ?`

	err := r.db.Raw(query, handlingID).Scan(&result).Error
	if err != nil {
		return result, err
	}

	return result, nil
}
