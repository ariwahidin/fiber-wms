package repositories

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type InboundRepository struct {
	db *gorm.DB
}

type ListInbound struct {
	ID              uint   `json:"id"`
	InboundNo       string `json:"inbound_no"`
	PONumber        string `json:"po_number"`
	SupplierID      string `json:"supplier_id"`
	SupplierName    string `json:"supplier_name"`
	Status          string `json:"status"`
	Invoice         string `json:"invoice"`
	TransporterID   string `json:"transporter_id"`
	DriverName      string `json:"driver_name"`
	TruckID         string `json:"truck_id"`
	TruckNo         string `json:"truck_no"`
	Type            string `json:"type"`
	InboundDate     string `json:"inbound_date"`
	ContainerNo     string `json:"container_no"`
	PoDate          string `json:"po_date"`
	OriginID        string `json:"origin_id"`
	Origin          string `json:"origin"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	RemarksHeader   string `json:"remarks_header"`
	TotalLine       int    `json:"total_line"`
	TotalQty        int    `json:"total_qty"`
	QtyScan         int    `json:"qty_scan"`
	TransporterName string `json:"transporter_name"`
}

type HeaderInbound struct {
	InboundID       int    `json:"inbound_id"`
	InboundNo       string `json:"inbound_no"`
	SupplierID      int    `json:"supplier_id"`
	SupplierName    string `json:"supplier_name"`
	Invoice         string `json:"invoice"`
	TransporterID   int    `json:"transporter_id"`
	Driver          string `json:"driver"`
	TruckSize       string `json:"truck_size"`
	TruckNo         string `json:"truck_no"`
	InboundDate     string `json:"inbound_date"`
	ContainerNo     string `json:"container_no"`
	PoDate          string `json:"po_date"`
	OriginID        int    `json:"origin_id"`
	TimeArrival     string `json:"time_arrival"`
	StartUnloading  string `json:"start_unloading"`
	FinishUnloading string `json:"finish_unloading"`
	Remarks         string `json:"remarks_header"`
	TotalLine       int    `json:"total_line"`
	TotalQty        int    `json:"total_qty"`
}

type DetailItem struct {
	ID           uint    `json:"id"`
	InboundId    int     `json:"inbound_id"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	CBM          float64 `json:"cbm"`
	GMC          string  `json:"gmc"`
	Barcode      string  `json:"barcode"`
	Quantity     int     `json:"quantity"`
	WhsCode      string  `json:"whs_code"`
	RecDate      string  `json:"rec_date"`
	Uom          string  `json:"uom"`
	Remarks      string  `json:"remarks"`
	HandlingId   int     `json:"handling_id"`
	HandlingUsed string  `json:"handling_used"`
	Location     string  `json:"location"`
	SumRateIdr   int     `json:"sum_rate_idr"`
}

func NewInboundRepository(db *gorm.DB) *InboundRepository {
	return &InboundRepository{db: db}
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

	if data.ID > 0 {
		if err := tx.Save(data).Error; err != nil {
			tx.Rollback()
			return 0, err
		}

		sqlDelete := `DELETE FROM inbound_detail_handlings WHERE inbound_detail_id = ?`
		if err := tx.Exec(sqlDelete, data.ID).Error; err != nil {
			tx.Rollback()
			return 0, err
		}

	} else {
		if err := tx.Create(data).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Ambil ID yang baru saja diinsert
	inboundDetailID := data.ID

	var total_vas int

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

		total_vas = total_vas + handling.RateIDR

		if err := tx.Create(&inboundDetailHandling).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	data.TotalVas = total_vas

	if err := tx.Save(data).Error; err != nil {
		tx.Rollback()
		return 0, err
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

func (r *InboundRepository) GetAllInbound() ([]ListInbound, error) {
	var listInbound []ListInbound
	sql := `WITH detail AS (
				SELECT inbound_id, COUNT(item_code) as total_line,SUM(quantity) total_qty 
				FROM inbound_details GROUP BY inbound_id
			),
	inbound_barcode AS(
			select inbound_id, sum(quantity) as qty_scan from inbound_barcodes
			group by inbound_id
	)
			SELECT a.id, a.inbound_no, a.supplier_id,
			c.supplier_name, 
			a.invoice, a.transporter_id,
			a.driver, a.truck_id, a.truck_no, a.inbound_date,
			a.container_no,a.po_date,
			a.origin_id, a.time_arrival, a.start_unloading, a.finish_unloading,
			a.status, a.inbound_date, a.remarks as remarks_header,
			b.total_line, b.total_qty, COALESCE(ib.qty_scan, 0) as qty_scan,
			c.supplier_name, a.status, d.transporter_name, a.po_number, a.type
			FROM 
			inbound_headers a
			LEFT JOIN detail b ON a.id = b.inbound_id
			LEFT JOIN suppliers c ON a.supplier_id = c.id
			LEFT JOIN transporters d ON a.transporter_id = d.id
			LEFT JOIN inbound_barcode ib ON a.id = ib.inbound_id
			ORDER BY a.created_at DESC`

	if err := r.db.Raw(sql).Scan(&listInbound).Error; err != nil {
		return nil, err
	}

	for i, inbound := range listInbound {
		inboundRefereces := []models.InboundReference{}

		if err := r.db.Where("inbound_id = ?", inbound.ID).Find(&inboundRefereces).Error; err != nil {
			return nil, err
		}

		var refNos []string
		for _, ref := range inboundRefereces {
			refNos = append(refNos, ref.RefNo)
		}

		// Gabungkan semua RefNo dengan koma
		listInbound[i].Invoice = strings.Join(refNos, ", ")
	}

	return listInbound, nil
}

func (r *InboundRepository) GetInboundHeaderByInboundID(inbound_id int) (HeaderInbound, error) {

	var result HeaderInbound

	sql := `WITH detail AS (
		SELECT inbound_id, COUNT(item_code) as total_line,SUM(quantity) total_qty 
		FROM inbound_details GROUP BY inbound_id
	)
	SELECT a.id as inbound_id, a.inbound_no, a.supplier_id, 
	a.invoice, a.transporter_id,
	a.driver, a.truck_id, a.truck_no, a.inbound_date,
	a.container_no, a.po_date,
	a.origin_id, a.time_arrival, a.start_unloading, a.finish_unloading,
	a.status, a.inbound_date, a.remarks,
	b.total_line, b.total_qty,
	c.supplier_name, a.status
	FROM 
	inbound_headers a
	LEFT JOIN detail b ON a.id = b.inbound_id
	LEFT JOIN suppliers c ON a.supplier_id = c.id
	WHERE a.id = ?`

	if err := r.db.Raw(sql, inbound_id).Scan(&result).Error; err != nil {
		return result, err
	}

	return result, nil
}

func (r *InboundRepository) GetDetailItemByInboundID(inbound_id int) ([]models.FormItemInbound, error) {
	var result []models.FormItemInbound

	sql := `SELECT 
		b.id as inbound_detail_id,
		a.id as inbound_id,
		a.inbound_no as inbound_no,
		b.item_id,
		p.item_name, 
		p.barcode,
		b.item_code,
		b.quantity,
		b.uom,
		b.rec_date,
		b.whs_code,
		b.handling_id,
		b.remarks,
		b.location,
		c.name as handling_used,
		b.total_vas
        FROM
        inbound_headers a
        INNER JOIN inbound_details b ON a.id = b.inbound_id
		INNER JOIN products p on p.id = b.item_id
		LEFT JOIN handlings c ON b.handling_id = c.id
        WHERE a.id = ?
		ORDER BY b.id ASC`

	if err := r.db.Debug().Raw(sql, inbound_id).Scan(&result).Error; err != nil {
		return nil, err
	}

	return result, nil
}

type InboundDetailScanned struct {
	ID           uint    `json:"id"`
	InboundId    int     `json:"inbound_id"`
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	CBM          float64 `json:"cbm"`
	GMC          string  `json:"gmc"`
	HasSerial    string  `json:"has_serial"`
	Barcode      string  `json:"barcode"`
	Quantity     int     `json:"quantity"`
	QtyScan      int     `json:"qty_scan"`
	WhsCode      string  `json:"whs_code"`
	RecDate      string  `json:"rec_date"`
	Uom          string  `json:"uom"`
	Remarks      string  `json:"remarks"`
	HandlingId   int     `json:"handling_id"`
	HandlingUsed string  `json:"handling_used"`
	Location     string  `json:"location"`
	SumRateIdr   int     `json:"sum_rate_idr"`
}

func (r *InboundRepository) GetDetailInbound(inbound_id int, inbound_detail_id int) (InboundDetailScanned, error) {
	var result InboundDetailScanned

	sql := `WITH detail_handling AS
	(
		SELECT inbound_detail_id, SUM(rate_idr) as sum_rate_idr 
		FROM inbound_detail_handlings
		GROUP BY inbound_detail_id
	), 
	inbound_barcode AS
	(
		SELECT inbound_id, inbound_detail_id, SUM(quantity) AS qty_scan
		FROM inbound_barcodes
		WHERE inbound_id = ? AND inbound_detail_id = ?
		GROUP BY inbound_id, inbound_detail_id
	)
	SELECT a.id, a.inbound_id, a.item_code, a.quantity , isnull(d.qty_scan, 0) as qty_scan,
	b.item_name, b.cbm, b.gmc, b.barcode, a.whs_code, a.rec_date, a.uom, a.remarks, a.location, e.has_serial,
	a.handling_id, a.handling_used, c.sum_rate_idr
	FROM inbound_details a
	INNER JOIN products b ON a.item_code = b.item_code
	LEFT JOIN detail_handling c ON a.id = c.inbound_detail_id
	LEFT JOIN inbound_barcode d ON a.id = d.inbound_detail_id
	LEFT JOIN products e ON a.item_id = e.id
	WHERE a.inbound_id = ? AND a.id = ?`

	if err := r.db.Raw(sql, inbound_id, inbound_detail_id, inbound_id, inbound_detail_id).Scan(&result).Error; err != nil {
		return result, err
	}

	return result, nil
}

type InboundBarcode struct {
	ID              uint   `json:"id"`
	InboundId       int    `json:"inbound_id"`
	InboundNo       string `json:"inbound_no"`
	InboundDetailId int    `json:"inbound_detail_id"`
	ItemCode        string `json:"item_code"`
	Barcode         string `json:"bracode"`
	ItemName        string `json:"item_name"`
	SerialNumber    string `json:"serial_number"`
	Location        string `json:"location"`
	Quantity        int    `json:"quantity"`
	Status          string `json:"status"`
}

func (r *InboundRepository) GetInboundBarcode(inbound_id int) ([]InboundBarcode, error) {

	var result []InboundBarcode
	sql := `select a.id, a.inbound_id, c.code as inbound_no, a.inbound_detail_id,
	a.item_code, a.barcode, a.quantity,
	b.item_name, a.serial_number, a.location, a.quantity
	from inbound_barcodes a
	inner join products b ON a.item_code = b.item_code
	inner join inbound_headers c ON a.inbound_id = c.id
	WHERE inbound_id = ?`
	if err := r.db.Raw(sql, inbound_id).Scan(&result).Error; err != nil {
		return result, err
	}

	return result, nil
}

func (r *InboundRepository) GetInboundBarcodeDetail(inbound_id int, inbound_detail_id int) ([]InboundBarcode, error) {

	var result []InboundBarcode
	sql := `select a.id, a.inbound_id, c.inbound_no, a.inbound_detail_id,
	a.item_code, a.barcode, a.quantity,
	b.item_name, a.serial_number, a.location, a.quantity, a.status
	from inbound_barcodes a
	inner join products b ON a.item_code = b.item_code
	inner join inbound_headers c ON a.inbound_id = c.id
	WHERE inbound_id = ? AND a.inbound_detail_id = ?`
	if err := r.db.Raw(sql, inbound_id, inbound_detail_id).Scan(&result).Error; err != nil {
		return result, err
	}

	return result, nil
}

type InboundBarcodeScanned struct {
	ReferenceCode   string `json:"reference_code"`
	InboundID       int    `json:"inbound_id"`
	InboundDetailID int    `json:"inbound_detail_id"`
	ItemCode        string `json:"item_code"`
	Barcode         string `json:"barcode"`
	ItemName        string `json:"item_name"`
	Expect          int    `json:"expect"`
	QtyScan         int    `json:"qty_scan"`
	RemainingQty    int    `json:"remaining_qty"`
}

func (r *InboundRepository) GetAllInboundScannedByInboundID(inbound_id int) ([]InboundBarcodeScanned, error) {
	sqlSelect := `WITH barcode AS (
    SELECT inbound_id, inbound_detail_id, item_code, barcode, SUM(quantity) as qty_scan
    FROM inbound_barcodes
    WHERE inbound_id = ?
    GROUP BY inbound_id, inbound_detail_id, item_code, barcode
)
	SELECT

		i.inbound_no,
		a.inbound_id,
		a.id as inbound_detail_id,
		a.item_code,
		c.barcode,
		c.item_name,
		a.quantity AS expect,
		COALESCE(b.qty_scan, 0) AS qty_scan,
		(a.quantity - COALESCE(b.qty_scan, 0)) AS remaining_qty
	FROM inbound_details a
	INNER JOIN inbound_headers i ON a.inbound_id = i.id
	LEFT JOIN barcode b ON a.inbound_id = b.inbound_id AND a.id = b.inbound_detail_id
	LEFT JOIN products c ON a.item_code = c.item_code
	WHERE a.inbound_id = ?
`
	var result []InboundBarcodeScanned
	if err := r.db.Raw(sqlSelect, inbound_id, inbound_id).Scan(&result).Error; err != nil {
		return result, err
	}
	return result, nil
}

func (r *InboundRepository) GenerateInboundNo() (string, error) {
	var lastInbound models.InboundHeader

	// Ambil inbound terakhir
	if err := r.db.Last(&lastInbound).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Ambil tanggal sekarang dalam format YYMMDD
	now := time.Now()
	currentDate := now.Format("060102") // 06=YY, 01=MM, 02=DD

	// Generate nomor inbound baru
	var inboundNo string
	if lastInbound.InboundNo != "" && len(lastInbound.InboundNo) >= 12 {
		lastDatePart := lastInbound.InboundNo[2:8]
		lastSequenceStr := lastInbound.InboundNo[len(lastInbound.InboundNo)-4:]

		if currentDate != lastDatePart {
			// Tanggal berbeda → reset sequence ke 1
			inboundNo = fmt.Sprintf("IN%s%04d", currentDate, 1)
		} else {
			// Tanggal sama → increment sequence
			lastSequenceInt, _ := strconv.Atoi(lastSequenceStr)
			inboundNo = fmt.Sprintf("IN%s%04d", currentDate, lastSequenceInt+1)
		}
	} else {
		// Tidak ada record sebelumnya → mulai dari 1
		inboundNo = fmt.Sprintf("IN%s%04d", currentDate, 1)
	}

	return inboundNo, nil
}

func (r *InboundRepository) PutawayItem(ctx *fiber.Ctx, inboundBarcodeID int, location string) (bool, error) {
	userID, ok := ctx.Locals("userID").(float64)
	if !ok {
		return false, errors.New("invalid user ID")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var barcode models.InboundBarcode
		if err := tx.Where("id = ?", inboundBarcodeID).Take(&barcode).Error; err != nil {
			return err
		}

		if barcode.Status != "pending" {
			return fmt.Errorf("barcode not in pending status")
		}

		var detail models.InboundDetail
		if err := tx.Where("id = ?", barcode.InboundDetailId).Take(&detail).Error; err != nil {
			return errors.New("inbound detail not found for item: " + barcode.ItemCode)
		}

		if location == "" {
			location = barcode.Location
		}

		uomRepo := NewUomRepository(tx)
		uomConversion, err := uomRepo.ConversionQty(barcode.ItemCode, barcode.Quantity, detail.Uom)
		if err != nil {
			return err
		}
		qtyConverted := uomConversion.QtyConverted

		inventory := models.Inventory{
			InboundDetailId:  int(detail.ID),
			InboundBarcodeId: int(barcode.ID),
			RecDate:          detail.RecDate,
			ItemId:           int(barcode.ItemID),
			ItemCode:         barcode.ItemCode,
			Barcode:          barcode.Barcode,
			WhsCode:          barcode.WhsCode,
			Pallet:           barcode.Pallet,
			Location:         location,
			QaStatus:         barcode.QaStatus,
			SerialNumber:     barcode.ScanData,
			Uom:              uomConversion.ToUom,
			QtyOrigin:        qtyConverted,
			QtyOnhand:        qtyConverted,
			QtyAvailable:     qtyConverted,
			QtyAllocated:     0,
			Trans:            "putaway",
			CreatedBy:        int(userID),
		}

		if err := tx.Create(&inventory).Error; err != nil {
			return err
		}

		if err := tx.Model(&barcode).Updates(map[string]interface{}{
			"status":     "in stock",
			"updated_at": time.Now().UTC(),
			"updated_by": int(userID),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return false, err
	}

	return true, nil
}
