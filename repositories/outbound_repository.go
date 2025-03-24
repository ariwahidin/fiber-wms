package repositories

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

type OutboundRepository struct {
	db *gorm.DB
}

type OutboundList struct {
	ID           uint   `json:"ID"`
	OutboundNo   string `json:"outbound_no"`
	DeliveryNo   string `json:"delivery_no"`
	OutboundDate string `json:"outbound_date"`
	CustomerCode string `json:"customer_code"`
	CustomerName string `json:"customer_name"`
	TotalLine    int    `json:"total_line"`
	TotalQtyReq  int    `json:"total_qty_req"`
	TotalQtyScan int    `json:"total_qty_scan"`
	Status       string `json:"status"`
}

type OutboundDetailList struct {
	OutboundDetailID int    `json:"outbound_detail_id"`
	OutboundID       int    `json:"outbound_id"`
	OutboundNo       string `json:"outbound_no"`
	DeliveryNo       string `json:"delivery_no"`
	CustomerCode     string `json:"customer_code"`
	CustomerName     string `json:"customer_name"`
	ItemID           int    `json:"item_id"`
	ItemCode         string `json:"item_code"`
	ItemName         string `json:"item_name"`
	HasSerial        string `json:"has_serial"`
	QtyReq           int    `json:"qty_req"`
	QtyScan          int    `json:"qty_scan"`
	Uom              string `json:"uom"`
}

func NewOutboundRepository(db *gorm.DB) *OutboundRepository {
	return &OutboundRepository{db}
}

func (r *OutboundRepository) GenerateOutboundNumber() (string, error) {
	var lastOutbound models.OutboundHeader

	// Ambil outbound terakhir
	if err := r.db.Last(&lastOutbound).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Ambil bulan dan tahun saat ini
	currentYear := time.Now().Format("2006")
	currentMonth := time.Now().Format("01")

	// Generate nomor inbound baru
	var outboundNo string
	if lastOutbound.OutboundNo != "" {
		lastOutboundNo := lastOutbound.OutboundNo[len(lastOutbound.OutboundNo)-4:] // Ambil 4 digit terakhir
		if currentMonth != lastOutbound.OutboundNo[8:10] {                         // Jika bulan berbeda
			outboundNo = fmt.Sprintf("OB-%s-%s-%04d", currentYear, currentMonth, 1)
		} else {
			lastOutboundNoInt, _ := strconv.Atoi(lastOutboundNo)
			outboundNo = fmt.Sprintf("OB-%s-%s-%04d", currentYear, currentMonth, lastOutboundNoInt+1)
		}
	} else {
		outboundNo = fmt.Sprintf("OB-%s-%s-%04d", currentYear, currentMonth, 1)
	}

	return outboundNo, nil
}

func (r *OutboundRepository) CreateItemOutbound(header *models.OutboundHeader, data *models.OutboundDetail, handlingUsed []HandlingDetailUsed) (uint, error) {

	fmt.Println("data Item : ", data)

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

	if data.ID > 0 {
		if err := tx.Save(data).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	} else {
		if err := tx.Create(data).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Insert ke Outbound Detail
	// if err := tx.Create(data).Error; err != nil {
	// 	tx.Rollback()
	// 	return 0, err
	// }

	sqlDelete := `DELETE FROM outbound_detail_handlings WHERE outbound_detail_id = ?`
	if err := tx.Exec(sqlDelete, data.ID).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Ambil ID yang baru saja diinsert
	outboundDetailID := data.ID

	// Insert ke Inbound Detail Handlings
	for _, handling := range handlingUsed {
		inboundDetailHandling := models.OutboundDetailHandling{
			OutboundDetailId:  int(outboundDetailID),
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

	return data.ID, nil
}

func (r *OutboundRepository) GetAllOutboundList() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `with details as
		(select outbound_id, count(outbound_id) as total_line,
		sum(quantity) as total_qty_req
		from outbound_details od
		group by outbound_id)

		select a.id, a.outbound_no, a.delivery_no, a.status,
		a.outbound_date, a.customer_code, c.customer_name,
		b.total_line, b.total_qty_req
		from outbound_headers a
		inner join details b on a.id = b.outbound_id
		inner join customers c on a.customer_code = c.customer_code
		order by a.id desc`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

func (r *OutboundRepository) GetOutboundOpen() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `with details as
		(select outbound_id, count(outbound_id) as total_line,
		sum(quantity) as total_qty_req
		from outbound_details od
		group by outbound_id)

		select a.id, a.outbound_no, a.delivery_no, a.status,
		a.outbound_date, a.customer_code, c.customer_name,
		b.total_line, b.total_qty_req
		from outbound_headers a
		inner join details b on a.id = b.outbound_id
		inner join customers c on a.customer_code = c.customer_code
		where a.status = 'open'`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

func (r *OutboundRepository) GetOutboundDetailList(outbound_id int) ([]OutboundDetailList, error) {

	var outboundDetailList []OutboundDetailList

	sql := `with cte_outbound_barcodes as (
				select outbound_id, outbound_detail_id, sum(quantity) as qty_scan 
				from outbound_barcodes
				group by outbound_id, outbound_detail_id
			)
			select a.id as outbound_detail_id, a.outbound_id, a.outbound_no, c.delivery_no, c.customer_code, d.customer_name,
			a.item_id, a.item_code, b.item_name, b.has_serial,
			a.quantity as qty_req, a.uom, isnull(e.qty_scan, 0) as qty_scan
			from outbound_details a
			inner join products b on a.item_id = b.id
			inner join outbound_headers c on a.outbound_id = c.id
			inner join customers d on c.customer_code = d.customer_code
			left join cte_outbound_barcodes e on a.id = e.outbound_detail_id
			where a.outbound_id = ?`

	if err := r.db.Raw(sql, outbound_id).Scan(&outboundDetailList).Error; err != nil {
		return nil, err
	}

	return outboundDetailList, nil
}
