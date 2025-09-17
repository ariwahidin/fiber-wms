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

type OutboundDetailList struct {
	OutboundDetailID int    `json:"outbound_detail_id"`
	OutboundID       int    `json:"outbound_id"`
	OutboundNo       string `json:"outbound_no"`
	DeliveryNo       string `json:"shipment_id"`
	CustomerCode     string `json:"customer_code"`
	CustomerName     string `json:"customer_name"`
	ItemID           int    `json:"item_id"`
	ItemCode         string `json:"item_code"`
	ItemName         string `json:"item_name"`
	HasSerial        string `json:"has_serial"`
	QtyReq           int    `json:"qty_req"`
	QtyScan          int    `json:"qty_scan"`
	WhsCode          string `json:"whs_code"`
	Uom              string `json:"uom"`
}

func NewOutboundRepository(db *gorm.DB) *OutboundRepository {
	return &OutboundRepository{db: db}
}

// func (r *OutboundRepository) GenerateOutboundNumber() (string, error) {
// 	var lastOutbound models.OutboundHeader

// 	// Ambil outbound terakhir
// 	if err := r.db.Last(&lastOutbound).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
// 		return "", err
// 	}

// 	// Ambil bulan dan tahun saat ini
// 	currentYear := time.Now().Format("2006")
// 	currentMonth := time.Now().Format("01")

// 	// Generate nomor inbound baru
// 	var outboundNo string
// 	if lastOutbound.OutboundNo != "" {
// 		lastOutboundNo := lastOutbound.OutboundNo[len(lastOutbound.OutboundNo)-4:] // Ambil 4 digit terakhir
// 		if currentMonth != lastOutbound.OutboundNo[6:8] {                          // Jika bulan berbeda
// 			outboundNo = fmt.Sprintf("OB%s%s%04d", currentYear, currentMonth, 1)
// 		} else {
// 			lastOutboundNoInt, _ := strconv.Atoi(lastOutboundNo)
// 			outboundNo = fmt.Sprintf("OB%s%s%04d", currentYear, currentMonth, lastOutboundNoInt+1)
// 		}
// 	} else {
// 		outboundNo = fmt.Sprintf("OB%s%s%04d", currentYear, currentMonth, 1)
// 	}

// 	return outboundNo, nil
// }

func (r *OutboundRepository) GenerateOutboundNumber() (string, error) {
	var lastOutbound models.OutboundHeader

	// Ambil outbound terakhir
	if err := r.db.Last(&lastOutbound).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Ambil tanggal sekarang dalam format YYMMDD
	now := time.Now()
	currentDate := now.Format("060102") // 06=YY, 01=MM, 02=DD

	var outboundNo string
	if lastOutbound.OutboundNo != "" && len(lastOutbound.OutboundNo) >= 12 {
		lastDatePart := lastOutbound.OutboundNo[2:8]
		lastSequenceStr := lastOutbound.OutboundNo[len(lastOutbound.OutboundNo)-4:]

		if currentDate != lastDatePart {
			// Tanggal berubah → reset nomor urut ke 1
			outboundNo = fmt.Sprintf("OB%s%04d", currentDate, 1)
		} else {
			// Tanggal sama → tambahkan nomor urut
			lastSequenceInt, _ := strconv.Atoi(lastSequenceStr)
			outboundNo = fmt.Sprintf("OB%s%04d", currentDate, lastSequenceInt+1)
		}
	} else {
		// Tidak ada outbound sebelumnya
		outboundNo = fmt.Sprintf("OB%s%04d", currentDate, 1)
	}

	return outboundNo, nil
}

func (r *OutboundRepository) GeneratePackingNumber() (string, error) {
	var lastPacking models.OutboundPacking

	// Ambil outbound terakhir
	if err := r.db.Last(&lastPacking).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Ambil tanggal sekarang dalam format YYMMDD
	now := time.Now()
	currentDate := now.Format("060102") // 06=YY, 01=MM, 02=DD

	var packingNo string
	if lastPacking.PackingNo != "" && len(lastPacking.PackingNo) >= 12 {
		lastDatePart := lastPacking.PackingNo[2:8]
		lastSequenceStr := lastPacking.PackingNo[len(lastPacking.PackingNo)-4:]

		if currentDate != lastDatePart {
			// Tanggal berubah → reset nomor urut ke 1
			packingNo = fmt.Sprintf("PA%s%04d", currentDate, 1)
		} else {
			// Tanggal sama → tambahkan nomor urut
			lastSequenceInt, _ := strconv.Atoi(lastSequenceStr)
			packingNo = fmt.Sprintf("PA%s%04d", currentDate, lastSequenceInt+1)
		}
	} else {
		// Tidak ada outbound sebelumnya
		packingNo = fmt.Sprintf("PA%s%04d", currentDate, 1)
	}

	return packingNo, nil
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
	total_idr := 0
	for _, handling := range handlingUsed {
		inboundDetailHandling := models.OutboundDetailHandling{
			OutboundDetailId: int(outboundDetailID),
			// HandlingId:        handling.HandlingID,
			HandlingUsed: handling.HandlingUsed,
			// HandlingCombineId: handling.HandlingCombineID,
			// OriginHandlingId:  handling.OriginHandlingID,
			// OriginHandling:    handling.OriginHandling,
			// RateId:            handling.RateID,
			RateIdr:   handling.RateIDR,
			CreatedBy: int(data.CreatedBy),
		}

		total_idr += handling.RateIDR

		if err := tx.Create(&inboundDetailHandling).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Update total_idr di outbound_detail
	if err := tx.Model(data).Where("id = ?", outboundDetailID).Update("total_vas", total_idr).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return data.ID, nil
}

type OutboundList struct {
	ID           uint    `json:"ID"`
	OutboundNo   string  `json:"outbound_no"`
	ShipmentID   string  `json:"shipment_id"`
	OwnerCode    string  `json:"owner_code"`
	OutboundDate string  `json:"outbound_date"`
	CustomerCode string  `json:"customer_code"`
	CustomerName string  `json:"customer_name"`
	TotalItem    int     `json:"total_item"`
	QtyReq       int     `json:"qty_req"`
	QtyPlan      int     `json:"qty_plan"`
	QtyPack      int     `json:"qty_pack"`
	TotalQty     int     `json:"total_qty"`
	Status       string  `json:"status"`
	TotalPrice   int     `json:"total_price"`
	DelivTo      string  `json:"deliv_to"`
	DelivToName  string  `json:"deliv_to_name"`
	DelivAddress string  `json:"deliv_address"`
	DelivCity    string  `json:"deliv_city"`
	QtyKoli      int     `json:"qty_koli"`
	TotalCBM     float64 `json:"total_cbm"`
}

func (r *OutboundRepository) GetAllOutboundList() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := ` WITH od AS 
	 (select outbound_id, count(outbound_id) as total_item, sum(p.cbm) as total_cbm,
    sum(quantity) as qty_req
    from outbound_details od
	inner join products as p on od.item_id = p.id
    group by outbound_id),
   ps AS(
		SELECT outbound_id, COUNT(item_id) AS total_item,  
		SUM(quantity) AS qty_plan
		FROM outbound_pickings
		GROUP BY outbound_id
	),
	kd AS(
		SELECT outbound_id, SUM(quantity) AS qty_pack
		FROM outbound_barcodes
		GROUP BY outbound_id
	)
   select a.id, a.outbound_no, 
			a.shipment_id, 
			a.status, a.owner_code, 
			a.shipment_id,
            a.outbound_date, a.customer_code,
            od.total_item, od.qty_req, COALESCE(ps.qty_plan, 0) AS qty_plan,
            COALESCE(kd.qty_pack, 0) AS qty_pack,
            cs.customer_name,
			a.deliv_to,
			cd.customer_name as deliv_to_name,
			cd.cust_addr1 as deliv_address,
			cd.cust_city as deliv_city,
			a.qty_koli,
			od.total_cbm,
			od.total_item
            from outbound_headers a
            left join od on a.id = od.outbound_id
            LEFT JOIN ps ON a.id = ps.outbound_id
            LEFT JOIN kd ON a.id = kd.outbound_id
            LEFT JOIN customers cs ON a.customer_code = cs.customer_code
			LEFT JOIN customers cd ON a.deliv_to = cd.customer_code
			order by a.id desc`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}
func (r *OutboundRepository) GetAllOutboundListComplete() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `WITH od AS 
	 (select outbound_id, count(outbound_id) as total_item,
    sum(quantity) as qty_req
    from outbound_details od
    group by outbound_id),
   ps AS(
		SELECT outbound_id, COUNT(item_id) AS total_item,  
		SUM(quantity) AS qty_plan
		FROM outbound_pickings
		GROUP BY outbound_id
	),
	kd AS(
		SELECT outbound_id, SUM(qty) AS qty_pack
		FROM outbound_scan_details
		GROUP BY outbound_id
	)
   select a.id, a.outbound_no, a.shipment_id, a.status, a.owner_code, a.shipment_id,
            a.outbound_date, a.customer_code,
            od.total_item, od.qty_req, COALESCE(ps.qty_plan, 0) AS qty_plan,
            COALESCE(kd.qty_pack, 0) AS qty_pack,
            cs.customer_name
            from outbound_headers a
            left join od on a.id = od.outbound_id
            LEFT JOIN ps ON a.id = ps.outbound_id
            LEFT JOIN kd ON a.id = kd.outbound_id
            LEFT JOIN customers cs ON a.customer_code = cs.customer_code
			WHERE a.status = 'complete'
			order by a.id desc`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

func (r *OutboundRepository) GetAllOutboundListOutboundHandling() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `WITH od AS 
	 (select outbound_id, count(outbound_id) as total_item,
    sum(quantity) as qty_req
    from outbound_details od
    group by outbound_id),
   ps AS(
		SELECT outbound_id, COUNT(item_id) AS total_item,  
		SUM(quantity) AS qty_plan
		FROM outbound_pickings
		GROUP BY outbound_id
	),
	kd AS(
		SELECT outbound_id, SUM(qty) AS qty_pack
		FROM outbound_scan_details
		GROUP BY outbound_id
	),
	hd AS (
		SELECT outbound_no,
		SUM(total_price) as total_price
		FROM outbound_detail_handlings
		GROUP BY outbound_no
	)
   select a.id, a.outbound_no, a.shipment_id, a.status, a.owner_code, a.shipment_id,
            a.outbound_date, a.customer_code,
            od.total_item, od.qty_req, COALESCE(ps.qty_plan, 0) AS qty_plan,
            COALESCE(kd.qty_pack, 0) AS qty_pack, hd.total_price,
            cs.customer_name
            from outbound_headers a
            left join od on a.id = od.outbound_id
            LEFT JOIN ps ON a.id = ps.outbound_id
            LEFT JOIN kd ON a.id = kd.outbound_id
            LEFT JOIN customers cs ON a.customer_code = cs.customer_code
			LEFT JOIN hd ON a.outbound_no = hd.outbound_no
			WHERE a.status = 'complete'
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

		select a.id, a.outbound_no, a.shipment_id, a.status,
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

func (r *OutboundRepository) GetOutboundPicking() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `with details as
		(select outbound_id, count(outbound_id) as total_line,
		sum(quantity) as total_qty_req
		from outbound_details od
		group by outbound_id)

		select a.id, a.outbound_no, a.shipment_id, a.status,
		a.outbound_date, a.customer_code, c.customer_name,
		b.total_line, b.total_qty_req
		from outbound_headers a
		inner join details b on a.id = b.outbound_id
		inner join customers c on a.customer_code = c.customer_code
		where a.status = 'picking'`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

// type PaperPickingSheet struct {
// 	OutboundNo   string  `json:"outbound_no"`
// 	InventoryID  int     `json:"inventory_id"`
// 	ItemID       int     `json:"item_id"`
// 	ItemCode     string  `json:"item_code"`
// 	Quantity     int     `json:"quantity"`
// 	Barcode      string  `json:"barcode"`
// 	ItemName     string  `json:"item_name"`
// 	Pallet       string  `json:"pallet"`
// 	Location     string  `json:"location"`
// 	Cbm          float64 `json:"cbm"`
// 	WhsCode      string  `json:"whs_code"`
// 	RecDate      string  `json:"rec_date"`
// 	OutboundDate string  `json:"outbound_date"`
// 	ShipmentID   string  `json:"shipment_id"`
// 	CustomerCode string  `json:"customer_code"`
// 	CustomerName string  `json:"customer_name"`
// }

type PaperPickingSheet struct {
	OutboundNo      string  `json:"outbound_no"`
	InventoryID     int     `json:"inventory_id,omitempty"` // gak ada di select, bisa dihapus kalau gak dipakai
	ItemID          int     `json:"item_id"`
	ItemCode        string  `json:"item_code"`
	Quantity        int     `json:"quantity"`
	Barcode         string  `json:"barcode"`
	ItemName        string  `json:"item_name"`
	Pallet          string  `json:"pallet"`
	Location        string  `json:"location"`
	Cbm             float64 `json:"cbm"`
	WhsCode         string  `json:"whs_code"`
	RecDate         string  `json:"rec_date"`
	OutboundDate    string  `json:"outbound_date"`
	ShipmentID      string  `json:"shipment_id"`
	CustomerCode    string  `json:"customer_code"`
	CustomerName    string  `json:"customer_name"`
	DelivTo         string  `json:"deliv_to"`
	DelivToName     string  `json:"deliv_to_name"`
	CustAddress     string  `json:"cust_address"`
	CustCity        string  `json:"cust_city"`
	DelivAddress    string  `json:"deliv_address"`
	DelivCity       string  `json:"deliv_city"`
	QtyKoli         int     `json:"qty_koli"`
	QtyKoliSeal     int     `json:"qty_koli_seal"`
	Remarks         string  `json:"remarks"`
	PickerName      string  `json:"picker_name"`
	PlanPickupDate  string  `json:"plan_pickup_date"`
	PlanPickupTime  string  `json:"plan_pickup_time"`
	TransporterCode string  `json:"transporter_code"`
}

func (r *OutboundRepository) GetPickingSheet(outbound_id int) ([]PaperPickingSheet, error) {
	var outboundList []PaperPickingSheet

	sql := `select
	e.cust_address,
	e.cust_city,
	e.deliv_to,
	e.deliv_address,
	e.deliv_city,
	e.qty_koli,
	e.qty_koli_seal,
	e.remarks,
	e.picker_name,
	e.plan_pickup_date,
	e.plan_pickup_time,
	a.item_id, 
	a.item_code, 
	sum(a.quantity) as quantity, 
	a.pallet, a.location,
	b.barcode, b.item_name, b.cbm, 
	c.rec_date, 
	c.whs_code,
	b.cbm,
	b.item_name,
	e.outbound_no, 
	e.customer_code, 
	e.outbound_date, 
	e.shipment_id,
	f.customer_name,
	g.customer_name as deliv_to_name,
	h.transporter_code
	from outbound_pickings a
	inner join products b on a.item_id = b.id
	inner join inventories c on a.inventory_id = c.id
	inner join outbound_headers e on a.outbound_id = e.id
	inner join customers f on e.customer_code = f.customer_code
	inner join customers g on e.deliv_to = g.customer_code
	left join transporters h on e.transporter_code = h.transporter_code
	where a.outbound_id = ?
	group by a.location, a.pallet, a.item_id, a.item_code,
	b.barcode, b.item_name, b.cbm, c.rec_date, c.whs_code,
	e.outbound_no, e.customer_code, f.customer_name, e.outbound_date, e.shipment_id,
	e.cust_address,
	e.cust_city,
	e.deliv_to,
	e.deliv_address,
	e.deliv_city,
	e.qty_koli,
	e.qty_koli_seal,
	e.remarks,
	e.picker_name,
	e.plan_pickup_date,
	e.plan_pickup_time,
	g.customer_name,
	h.transporter_code
	Order By a.[location] ASC`

	if err := r.db.Debug().Raw(sql, outbound_id).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

func (r *OutboundRepository) GetOutboundDetailList(outbound_id int) ([]OutboundDetailList, error) {

	var outboundDetailList []OutboundDetailList

	sql := `with cte_outbound_barcodes as (
				select outbound_id, outbound_detail_id, sum(quantity) as qty_scan 
				from outbound_barcodes
				where status = 'picked'
				group by outbound_id, outbound_detail_id
			)
			select a.id as outbound_detail_id, a.outbound_id, a.outbound_no, c.shipment_id, c.customer_code, d.customer_name,
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

func (r *OutboundRepository) GetOutboundDetailItem(outbound_id int, outbound_detail_id int) (OutboundDetailList, error) {

	var outboundDetailList OutboundDetailList

	sql := `with cte_outbound_barcodes as (
				select outbound_id, outbound_detail_id, sum(quantity) as qty_scan 
				from outbound_barcodes
				where status = 'picked'
				group by outbound_id, outbound_detail_id
			)
			select a.id as outbound_detail_id, a.outbound_id, a.outbound_no, c.shipment_id, c.customer_code, d.customer_name,
			a.item_id, a.item_code, b.item_name, b.has_serial, a.whs_code,
			a.quantity as qty_req, a.uom, isnull(e.qty_scan, 0) as qty_scan
			from outbound_details a
			inner join products b on a.item_id = b.id
			inner join outbound_headers c on a.outbound_id = c.id
			inner join customers d on c.customer_code = d.customer_code
			left join cte_outbound_barcodes e on a.id = e.outbound_detail_id
			where a.outbound_id = ? and a.id = ?`

	if err := r.db.Debug().Raw(sql, outbound_id, outbound_detail_id).Scan(&outboundDetailList).Error; err != nil {
		return outboundDetailList, err
	}

	return outboundDetailList, nil
}

type OutboundItem struct {
	OutboundDetailID int    `json:"outbound_detail_id"`
	OutboundID       int    `json:"outbound_id"`
	ItemID           int    `json:"item_id"`
	QtyReq           int    `json:"qty_req"`
	QtyScan          int    `json:"qty_scan"`
	Status           string `json:"status"`
	OutboundNo       string `json:"outbound_no"`
	// QtyPack          int    `json:"qty_pack"`
}

func (r *OutboundRepository) GetOutboundItemByID(outbound_id int) ([]OutboundItem, error) {

	var outboundItems []OutboundItem

	sql := `WITH od AS (
    SELECT
            id AS outbound_detail_id,
            outbound_id,
            item_id,
            SUM(quantity) AS qty_req,
            -- SUM(scan_qty) AS scan_qty,
            status,
            outbound_no
    FROM outbound_details
    GROUP BY outbound_id, item_id, id, status, outbound_no
),
kd AS (
    SELECT
            outbound_id,
            item_id,
            SUM(quantity) AS qty_scan,
            outbound_detail_id
    FROM outbound_barcodes
    GROUP BY outbound_id, item_id, outbound_detail_id
)
SELECT
    od.outbound_detail_id,
    od.outbound_id,
    od.item_id,
    od.qty_req,
    kd.qty_scan,
    od.status,
    od.outbound_no
    -- kd.qty_pack
FROM od
LEFT JOIN kd
    ON od.outbound_id = kd.outbound_id
    AND od.item_id = kd.item_id
    AND od.outbound_detail_id = kd.outbound_detail_id
WHERE od.outbound_id = ?`

	if err := r.db.Debug().Raw(sql, outbound_id).Scan(&outboundItems).Error; err != nil {
		return nil, err
	}

	return outboundItems, nil

}

type PickingItem struct {
	OutboundNo  string `json:"outbound_no"`
	InventoryID int    `json:"inventory_id,omitempty"` // gak ada di select, bisa dihapus kalau gak dipakai
	ItemID      int    `json:"item_id"`
	ItemCode    string `json:"item_code"`
	Quantity    int    `json:"quantity"`
	Barcode     string `json:"barcode"`
	ItemName    string `json:"item_name"`
	Pallet      string `json:"pallet"`
}

func (r *OutboundRepository) CheckPickingItem(outbound_id int, barcode string) ([]PickingItem, error) {
	var outboundList []PickingItem

	sql := `select
	e.cust_address,
	e.cust_city,
	e.deliv_to,
	e.deliv_address,
	e.deliv_city,
	e.qty_koli,
	e.qty_koli_seal,
	e.remarks,
	e.picker_name,
	e.plan_pickup_date,
	e.plan_pickup_time,
	a.item_id, 
	a.item_code, 
	sum(a.quantity) as quantity, 
	a.pallet, a.location,
	b.barcode, b.item_name, b.cbm, 
	c.rec_date, 
	c.whs_code,
	b.cbm,
	b.item_name,
	e.outbound_no, 
	e.customer_code, 
	e.outbound_date, 
	e.shipment_id,
	f.customer_name,
	g.customer_name as deliv_to_name,
	h.transporter_code
	from outbound_pickings a
	inner join products b on a.item_id = b.id
	inner join inventories c on a.inventory_id = c.id
	inner join outbound_headers e on a.outbound_id = e.id
	inner join customers f on e.customer_code = f.customer_code
	inner join customers g on e.deliv_to = g.customer_code
	left join transporters h on e.transporter_code = h.transporter_code
	where a.outbound_id = ?
	group by a.location, a.pallet, a.item_id, a.item_code,
	b.barcode, b.item_name, b.cbm, c.rec_date, c.whs_code,
	e.outbound_no, e.customer_code, f.customer_name, e.outbound_date, e.shipment_id,
	e.cust_address,
	e.cust_city,
	e.deliv_to,
	e.deliv_address,
	e.deliv_city,
	e.qty_koli,
	e.qty_koli_seal,
	e.remarks,
	e.picker_name,
	e.plan_pickup_date,
	e.plan_pickup_time,
	g.customer_name,
	h.transporter_code
	Order By a.[location] ASC`

	if err := r.db.Debug().Raw(sql, outbound_id).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

type PackingSummary struct {
	ID               int       `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	PackingNo        string    `json:"packing_no"`
	TotItem          int       `json:"tot_item"`
	TotQty           int       `json:"tot_qty"`
	OutboundNo       string    `json:"outbound_no"`
	OutboundID       int       `json:"outbound_id"`
	CustomerCode     string    `json:"customer_code"`
	CustAddress      string    `json:"cust_address"`
	CustCity         string    `json:"cust_city"`
	DelivTo          string    `json:"deliv_to"`
	DelivAddress     string    `json:"deliv_address"`
	DelivCity        string    `json:"deliv_city"`
	CustomerName     string    `json:"customer_name"`
	CustomerDelivery string    `json:"customer_delivery"`
}

func (r *OutboundRepository) GetPackingSummary() ([]PackingSummary, error) {
	var result []PackingSummary

	sql := `WITH ob AS (
			SELECT count(item_id) as tot_item, sum(quantity) as tot_qty, outbound_no, outbound_id, 
			packing_id, packing_no
			FROM outbound_barcodes
			WHERE packing_id <> 0
			GROUP BY outbound_id, outbound_no, packing_id, packing_no
		)

		SELECT op.id, op.created_at, op.packing_no, ob.tot_item, ob.tot_qty, ob.outbound_no, ob.outbound_id,
		oh.customer_code, oh.cust_address, oh.cust_city,
		oh.deliv_to, oh.deliv_address, oh.deliv_city, cs.customer_name, cd.customer_name as customer_delivery
		FROM 
		outbound_packings op
		LEFT JOIN ob on op.id = ob.packing_id
		LEFT JOIN outbound_headers oh on ob.outbound_id = oh.id
		LEFT JOIN customers cs on oh.customer_code = cs.customer_code
		LEFT JOIN customers cd on cd.customer_code = oh.deliv_to
		ORDER BY op.created_at DESC
	`

	if err := r.db.Debug().Raw(sql).Scan(&result).Error; err != nil {
		return nil, err
	}

	return result, nil
}

type PackingItem struct {
	PackingNo       string    `json:"packing_no"`
	PackingDate     time.Time `json:"packing_date"`
	CustAddress     string    `json:"cust_address"`
	CustCity        string    `json:"cust_city"`
	DelivTo         string    `json:"deliv_to"`
	DelivAddress    string    `json:"deliv_address"`
	DelivCity       string    `json:"deliv_city"`
	QtyKoli         int       `json:"qty_koli"`
	QtyKoliSeal     int       `json:"qty_koli_seal"`
	Remarks         string    `json:"remarks"`
	PickerName      string    `json:"picker_name"`
	PlanPickupDate  string    `json:"plan_pickup_date"`
	PlanPickupTime  string    `json:"plan_pickup_time"`
	ItemID          int       `json:"item_id"`
	ItemCode        string    `json:"item_code"`
	Quantity        int       `json:"quantity"`
	Barcode         string    `json:"barcode"`
	ItemName        string    `json:"item_name"`
	CBM             float64   `json:"cbm"`
	SerialNumber    string    `json:"serial_number"`
	OutboundNo      string    `json:"outbound_no"`
	CustomerCode    string    `json:"customer_code"`
	OutboundDate    string    `json:"outbound_date"`
	ShipmentID      string    `json:"shipment_id"`
	CustomerName    string    `json:"customer_name"`
	DelivToName     string    `json:"deliv_to_name"`
	TransporterCode string    `json:"transporter_code"`
}

func (r *OutboundRepository) GetPackingItems(outboundID int, packingNo string) ([]PackingItem, error) {
	var result []PackingItem

	sql := `
	SELECT
		a.packing_no,
		c.created_at as packing_date,
		e.cust_address,
		e.cust_city,
		e.deliv_to,
		e.deliv_address,
		e.deliv_city,
		e.qty_koli,
		e.qty_koli_seal,
		e.remarks,
		e.picker_name,
		e.plan_pickup_date,
		e.plan_pickup_time,
		a.item_id, 
		a.item_code, 
		sum(a.quantity) as quantity, 
		b.barcode, 
		b.item_name, 
		b.cbm, 
		a.serial_number,
		e.outbound_no, 
		e.customer_code, 
		e.outbound_date, 
		e.shipment_id,
		f.customer_name,
		g.customer_name as deliv_to_name,
		h.transporter_code
	FROM outbound_barcodes a
	INNER JOIN products b ON a.item_id = b.id
	INNER JOIN outbound_packings c ON a.packing_id = c.id
	INNER JOIN outbound_headers e ON a.outbound_id = e.id
	INNER JOIN customers f ON e.customer_code = f.customer_code
	INNER JOIN customers g ON e.deliv_to = g.customer_code
	LEFT JOIN transporters h ON e.transporter_code = h.transporter_code
	WHERE a.outbound_id = ? AND a.packing_no = ?
	GROUP BY 
		a.item_id, 
		a.item_code,
		b.barcode, 
		b.item_name, 
		b.cbm,
		e.outbound_no, 
		e.customer_code, 
		f.customer_name, 
		e.outbound_date, 
		e.shipment_id,
		e.cust_address,
		e.cust_city,
		e.deliv_to,
		e.deliv_address,
		e.deliv_city,
		e.qty_koli,
		e.qty_koli_seal,
		e.remarks,
		e.picker_name,
		e.plan_pickup_date,
		e.plan_pickup_time,
		g.customer_name,
		h.transporter_code,
		a.serial_number,
		a.packing_no,
		c.created_at
	ORDER BY a.item_code ASC
	`

	if err := r.db.Debug().Raw(sql, outboundID, packingNo).Scan(&result).Error; err != nil {
		return nil, err
	}

	return result, nil
}

type OutboundSummary struct {
	ID              int    `json:"id"`
	OutboundNo      string `json:"outbound_no"`
	ShipmentID      string `json:"shipment_id"`
	OutboundDate    string `json:"outbound_date"`
	CustomerCode    string `json:"customer_code"`
	CustomerName    string `json:"customer_name"`
	CustomerAddress string `json:"customer_address"`
	CustomerCity    string `json:"customer_city"`
	DelivTo         string `json:"deliv_to"`
	DelivToName     string `json:"deliv_to_name"`
	DelivAddress    string `json:"deliv_address"`
	DelivCity       string `json:"deliv_city"`
	TransporterCode string `json:"transporter_code"`
	TransporterName string `json:"transporter_name"`
}

func (r *OutboundRepository) GetOutboundSummary(outboundNo string) (OutboundSummary, error) {
	var result OutboundSummary

	sql := `SELECT 
		oh.id,
		oh.outbound_no, 
		oh.shipment_id,
		oh.outbound_date,
		oh.customer_code,
		cs.customer_name,
		cs.cust_addr1 as customer_address,
		cs.cust_city as customer_city,
		oh.deliv_to,
		cd.customer_name as deliv_to_name,
		cd.cust_addr1 as deliv_address,
		cd.cust_city as deliv_city,
		oh.transporter_code,
		tr.transporter_name
		FROM outbound_headers oh
		LEFT JOIN customers cs ON oh.customer_code = cs.customer_code
		LEFT JOIN customers cd ON oh.deliv_to = cd.customer_code
		LEFT JOIN transporters tr ON oh.transporter_code = tr.transporter_code
		WHERE oh.outbound_no = ?`

	if err := r.db.Debug().Raw(sql, outboundNo).Scan(&result).Error; err != nil {
		return result, err
	}

	return result, nil
}

type SerialNumberList struct {
	ItemCode     string `json:"item_code"`
	ItemName     string `json:"item_name"`
	Barcode      string `json:"barcode"`
	SerialNumber string `json:"serial_number"`
	HasSerial    string `json:"has_serial"`
}

func (r *OutboundRepository) GetOutboundSerialNumber(outboundID int) ([]SerialNumberList, error) {
	var result []SerialNumberList

	sql := `select 
	ob.item_code,
	p.item_name,
	p.barcode,
	ob.serial_number,
	p.has_serial
	from outbound_barcodes ob
	left join products p on ob.item_code = p.item_code
	where ob.outbound_id = ?
	AND p.has_serial = 'Y'`

	if err := r.db.Debug().Raw(sql, outboundID).Scan(&result).Error; err != nil {
		return result, err
	}

	if len(result) == 0 {
		result = []SerialNumberList{}
	}

	return result, nil
}

type VasCalculate struct {
	OutboundID   int     `json:"outbound_id"`
	OutboundNo   string  `json:"outbound_no"`
	OutboundDate string  `json:"outbound_date"`
	MainVasName  string  `json:"main_vas_name"`
	IsKoli       bool    `json:"is_koli"`
	DefaultPrice float64 `json:"default_price"`
	QtyItem      int     `json:"qty_item"`
	QtyKoli      int     `json:"qty_koli"`
	TotalPrice   float64 `json:"total_price"`
}

// func (r *OutboundRepository) CalculatVasOutbound(outboundID int) ([]VasCalculate, error) {
// 	var result []VasCalculate

// 	sql := `WITH vas_sum AS
// 	(SELECT v.id as vas_id, v.name as vas_name,
// 	vd.main_vas_id, mv.name as main_vas_name, mv.default_price, mv.is_koli
// 	FROM vas v
// 	INNER JOIN vas_detail vd ON v.id = vd.vas_id
// 	INNER JOIN main_vas mv ON mv.id = vd.main_vas_id),
// 	vas_ob_item AS (
// 		SELECT
// 		od.id as outbound_detail_id,
// 		od.outbound_id,
// 		od.outbound_no,
// 		oh.outbound_date,
// 		od.item_id,
// 		od.item_code,
// 		od.barcode,
// 		od.quantity as qty_item,
// 		ordt.qty_koli,
// 		od.vas_id ob_vas_id,
// 		od.vas_name ob_vas_name,
// 		vs.main_vas_id,
// 		vs.main_vas_name,
// 		vs.default_price,
// 		vs.is_koli,
// 		CASE WHEN vs.is_koli = 0 THEN od.quantity * vs.default_price ELSE oh.qty_koli * vs.default_price END AS total_price
// 		FROM
// 		outbound_details od
// 		inner join outbound_headers oh ON od.outbound_id = oh.id
// 		inner join vas_sum vs ON od.vas_id = vs.vas_id
// 		inner join order_details ordt ON oh.id = ordt.outbound_id
// 		WHERE od.outbound_id = ?
// 		),
// 	vas_ob_sum AS(
// 		select
// 		vb.outbound_id, vb.outbound_no, vb.outbound_date,
// 		vb.main_vas_name, vb.is_koli, vb.default_price,
// 		sum(vb.qty_item) as qty_item,
// 		vb.qty_koli
// 		from
// 		vas_ob_item vb
// 		where vb.is_koli = 1
// 		GROUP BY
// 		vb.outbound_id,
// 		vb.outbound_no,
// 		vb.outbound_date,
// 		vb.is_koli,
// 		vb.main_vas_name,
// 		vb.default_price,
// 		vb.qty_koli
// 		UNION ALL
// 		select
// 		vb.outbound_id, vb.outbound_no, vb.outbound_date,
// 		vb.main_vas_name, vb.is_koli, vb.default_price,vb.qty_item, vb.qty_koli
// 		from
// 		vas_ob_item vb
// 		where vb.is_koli = 0)
// 	SELECT
// 	vos.outbound_id,
// 	vos.outbound_no,
// 	vos.outbound_date,
// 	vos.main_vas_name,
// 	vos.is_koli,
// 	vos.default_price,
// 	vos.qty_item,
// 	vos.qty_koli,
// 	CASE WHEN vos.is_koli = 1 THEN vos.default_price * qty_koli ELSE vos.default_price * vos.qty_item END AS total_price
// 	FROM vas_ob_sum vos
// 	`

// 	if err := r.db.Debug().Raw(sql, outboundID).Scan(&result).Error; err != nil {
// 		return result, err
// 	}

// 	if len(result) == 0 {
// 		result = []VasCalculate{}
// 	}

// 	return result, nil
// }

type OutboundVasSum struct {
	OutboundID   int     `json:"outbound_id"`
	OutboundNo   string  `json:"outbound_no"`
	OutboundDate string  `json:"outbound_date"`
	TotalQty     int     `json:"total_qty"`
	GrandTotal   float64 `json:"grand_total"`
}

func (r *OutboundRepository) GetOutboundVasSum() ([]OutboundVasSum, error) {
	var result []OutboundVasSum

	sql := `SELECT ob.outbound_id, ob.outbound_date,
	ob.outbound_no, 
	sum(qty_item) as total_qty,
	sum(ob.total_price) as grand_total
	FROM 
	outbound_vas ob
	group by
	ob.outbound_date,
	ob.outbound_id,  
	ob.outbound_no`

	if err := r.db.Debug().Raw(sql).Scan(&result).Error; err != nil {
		return result, err
	}

	if len(result) == 0 {
		result = []OutboundVasSum{}
	}

	return result, nil
}
