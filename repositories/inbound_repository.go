package repositories

import (
	"errors"
	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InboundRepository struct {
	db *gorm.DB
}

type ListInbound struct {
	ID              uint   `json:"id"`
	InboundNo       string `json:"inbound_no"`
	ReceiptID       string `json:"receipt_id"`
	SupplierID      string `json:"supplier_id"`
	SupplierName    string `json:"supplier_name"`
	Status          string `json:"status"`
	Invoice         string `json:"invoice"`
	TransporterID   string `json:"transporter_id"`
	DriverName      string `json:"driver_name"`
	TruckID         string `json:"truck_id"`
	NoTruck         string `json:"no_truck"`
	Type            string `json:"type"`
	InboundDate     string `json:"inbound_date"`
	Container       string `json:"container"`
	Origin          string `json:"origin"`
	OwnerCode       string `json:"owner_code"`
	ArrivalTime     string `json:"arrival_time"`
	StartUnloading  string `json:"start_unloading"`
	EndUnloading    string `json:"end_unloading"`
	RemarksHeader   string `json:"remarks_header"`
	TotalLine       int    `json:"total_line"`
	TotalQty        int    `json:"total_qty"`
	QtyScan         int    `json:"qty_scan"`
	QtyPutaway      int    `json:"qty_putaway"`
	TransporterName string `json:"transporter_name"`
}

type HeaderInbound struct {
	InboundID      int    `json:"inbound_id"`
	InboundNo      string `json:"inbound_no"`
	SupplierID     int    `json:"supplier_id"`
	SupplierName   string `json:"supplier_name"`
	Invoice        string `json:"invoice"`
	TransporterID  int    `json:"transporter_id"`
	Driver         string `json:"driver"`
	TruckSize      string `json:"truck_size"`
	NoTruck        string `json:"no_truck"`
	InboundDate    string `json:"inbound_date"`
	Container      string `json:"container"`
	Origin         int    `json:"origin"`
	ArrivalTime    string `json:"arrival_time"`
	StartUnloading string `json:"start_unloading"`
	EndUnloading   string `json:"end_unloading"`
	Remarks        string `json:"remarks_header"`
	TotalLine      int    `json:"total_line"`
	TotalQty       int    `json:"total_qty"`
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
func (r *InboundRepository) CreateInboundDetail(data *models.InboundDetail, handlingUsed []HandlingDetailUsed) (int64, error) {
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

	return int64(inboundDetailID), nil
}

func (r *InboundRepository) UpdateInboundDetail(data *models.InboundDetail, handlingUsed []HandlingDetailUsed) (int64, error) {
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

	return int64(inboundDetailID), nil
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
	),
	inbound_putaway AS(
			select inbound_id, sum(quantity) as qty_scan from inbound_barcodes
			where status = 'in stock'
			group by inbound_id
	)

			SELECT a.id, a.inbound_no, a.receipt_id,
			c.supplier_name, a.owner_code,
			a.driver, a.truck_id, a.no_truck, a.inbound_date,
			a.container,
			a.origin, a.arrival_time, a.start_unloading, a.end_unloading,
			a.status, a.inbound_date, a.remarks as remarks_header,
			b.total_line, b.total_qty, COALESCE(ib.qty_scan, 0) as qty_scan, COALESCE(ipu.qty_scan, 0) as qty_putaway, 
			c.supplier_name, a.status, d.transporter_name, a.type
			FROM 
			inbound_headers a
			LEFT JOIN detail b ON a.id = b.inbound_id
			LEFT JOIN suppliers c ON a.supplier = c.supplier_code
			LEFT JOIN transporters d ON a.transporter = d.transporter_code
			LEFT JOIN inbound_barcode ib ON a.id = ib.inbound_id
			LEFT JOIN inbound_putaway ipu ON a.id = ipu.inbound_id
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
	a.driver, a.truck_id, a.no_truck, a.inbound_date,
	a.container,
	a.origin, a.arrival_time, a.start_unloading, a.end_unloading,
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

func (r *InboundRepository) ProcessPutawayItem(ctx *fiber.Ctx, inboundBarcodeID int, location string) (bool, error) {
	userID, ok := ctx.Locals("userID").(float64)
	movementID := uuid.NewString()
	if !ok {
		return false, errors.New("invalid user ID")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var barcode models.InboundBarcode
		if err := tx.Where("id = ?", inboundBarcodeID).Take(&barcode).Error; err != nil {
			return err
		}

		if barcode.Status != "pending" {
			return fmt.Errorf("item not in pending status")
		}

		var detail models.InboundDetail
		if err := tx.Where("id = ?", barcode.InboundDetailId).Take(&detail).Error; err != nil {
			return errors.New("inbound detail not found for item: " + barcode.ItemCode)
		}

		if location == "" {
			location = barcode.Location
		}

		uomRepo := NewUomRepository(tx)
		uomConversion, errUom := uomRepo.ConversionQty(barcode.ItemCode, barcode.Quantity, detail.Uom)
		if errUom != nil {
			return errUom
		}
		qtyConverted := uomConversion.QtyConverted

		var product models.Product
		if err := tx.Where("item_code = ?", barcode.ItemCode).Take(&product).Error; err != nil {
			return errors.New("product not found for item: " + barcode.ItemCode)
		}

		// Cek apakah data inventory dengan kombinasi yang sama sudah ada
		var existingInv models.Inventory
		invQuery := tx.Debug().Where(`
			inbound_id = ? AND
			inbound_detail_id = ? AND
			item_code = ? AND
			location = ? AND
			barcode = ? AND
			whs_code = ? AND
			qa_status = ? AND
			rec_date = ? AND
			prod_date = ? AND
			exp_date = ? AND
			lot_number = ?`,
			barcode.InboundId,
			barcode.InboundDetailId,
			barcode.ItemCode,
			location,
			product.Barcode,
			barcode.WhsCode,
			barcode.QaStatus,
			detail.RecDate,
			detail.ProdDate,
			barcode.ExpDate,
			barcode.LotNumber,
		).First(&existingInv)

		if errors.Is(invQuery.Error, gorm.ErrRecordNotFound) {
			// Tidak ada data → Insert baru
			newInv := models.Inventory{
				InboundID:       detail.InboundId,
				InboundDetailId: int(detail.ID),
				RecDate:         detail.RecDate,
				ItemId:          barcode.ItemID,
				ItemCode:        barcode.ItemCode,
				Barcode:         product.Barcode,
				WhsCode:         barcode.WhsCode,
				OwnerCode:       barcode.OwnerCode,
				DivisionCode:    barcode.DivisionCode,
				Pallet:          barcode.Pallet,
				Location:        location,
				QaStatus:        barcode.QaStatus,
				Uom:             uomConversion.ToUom,
				QtyOrigin:       qtyConverted,
				QtyOnhand:       qtyConverted,
				QtyAvailable:    qtyConverted,
				ExpDate:         barcode.ExpDate,
				ProdDate:        barcode.ProdDate,
				LotNumber:       barcode.LotNumber,
				Trans:           "putaway",
				CreatedBy:       int(userID),
			}

			if err := tx.Create(&newInv).Error; err != nil {
				return err
			}

			// ledger
			helpers.InsertInventoryMovement(tx, helpers.InventoryMovementPayload{
				InventoryID:        newInv.ID,
				MovementID:         movementID,
				RefType:            "INBOUND PUTAWAY",
				RefID:              uint(barcode.InboundId),
				ItemID:             product.ID,
				ItemCode:           product.ItemCode,
				ToWhsCode:          newInv.WhsCode,
				QtyOnhandChange:    qtyConverted,
				QtyAvailableChange: qtyConverted,
				FromLocation:       barcode.Location,
				NewQaStatus:        barcode.QaStatus,
				ToLocation:         location,
				Reason:             detail.InboundNo + " PUTAWAY",
				CreatedBy:          int(userID),
			})

		} else if invQuery.Error == nil {
			// Sudah ada → Update qty
			if err := tx.Model(&existingInv).Updates(map[string]interface{}{
				"qty_origin":    existingInv.QtyOrigin + qtyConverted,
				"qty_onhand":    existingInv.QtyOnhand + qtyConverted,
				"qty_available": existingInv.QtyAvailable + qtyConverted,
				"updated_at":    time.Now().UTC(),
				"updated_by":    int(userID),
			}).Error; err != nil {
				return err
			}

			// ledger
			helpers.InsertInventoryMovement(tx, helpers.InventoryMovementPayload{
				InventoryID:        existingInv.ID,
				MovementID:         movementID,
				RefType:            "INBOUND PUTAWAY",
				RefID:              uint(barcode.InboundId),
				ItemID:             product.ID,
				ItemCode:           product.ItemCode,
				ToWhsCode:          existingInv.WhsCode,
				QtyOnhandChange:    existingInv.QtyOnhand + qtyConverted,
				QtyAvailableChange: existingInv.QtyAvailable + qtyConverted,
				NewQaStatus:        barcode.QaStatus,
				FromLocation:       barcode.Location,
				ToLocation:         location,
				Reason:             detail.InboundNo + " PUTAWAY",
				CreatedBy:          int(userID),
			})
		} else {
			return invQuery.Error
		}

		// Update status barcode ke "in stock"
		if err := tx.Model(&barcode).Updates(map[string]interface{}{
			"status":           "in stock",
			"putaway_location": location,
			"putaway_qty":      barcode.Quantity,
			"putaway_at":       time.Now().UTC(),
			"putaway_by":       int(userID),
			"updated_at":       time.Now().UTC(),
			"updated_by":       int(userID),
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

type resultDetail struct {
	// di isi nanti
	ID        uint   `json:"id"`
	ItemCode  string `json:"item_code"`
	Barcode   string `json:"barcode"`
	ItemName  string `json:"item_name"`
	Uom       string `json:"uom"`
	Quantity  int    `json:"quantity"`
	QtyScan   int    `json:"qty_scan"`
	QaStatus  string `json:"qa_status"`
	ProdDate  string `json:"prod_date"`
	RecDate   string `json:"rec_date"`
	ExpDate   string `json:"exp_date"`
	LotNumber string `json:"lot_number"`
}

func (r *InboundRepository) GetInboundDetailByInboundID(inboundID uint) ([]resultDetail, error) {

	sql := `with inb_barcode as (
			select item_id, inbound_id, inbound_detail_id, sum(quantity) as qty_scan  
			from inbound_barcodes
			group by item_id, inbound_id, inbound_detail_id
		)
		select 
		a.id,
		a.item_code,
		a.barcode,
		b.item_name,
		a.uom,
		a.quantity,
		coalesce(c.qty_scan, 0) as qty_scan,
		a.qa_status,
		a.prod_date,
		a.rec_date,
		a.exp_date,
		a.lot_number
		from inbound_details a
		left join products b on a.item_id = b.id 
		left join inb_barcode c on a.id = c.inbound_detail_id
		where a.inbound_id = ?
		order by a.id asc;`

	var result []resultDetail
	if err := r.db.Raw(sql, inboundID).Scan(&result).Error; err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return []resultDetail{}, nil
	}

	return result, nil
}

type resulInboundBarcodeByOutboundDetailID struct {
	ID        uint   `json:"id"`
	ItemCode  string `json:"item_code"`
	ItemID    uint   `json:"item_id"`
	Status    string `json:"status"`
	TotalScan int    `json:"total_scan"`
	QaStatus  string `json:"qa_status"`
	RecDate   string `json:"rec_date"`
	ProdDate  string `json:"prod_date"`
	LotNumber string `json:"lot_number"`
	ExpDate   string `json:"exp_date"`
}

func (r *InboundRepository) GetInboundBarcodeByOutboundDetailID(outboundDetailID uint) (resulInboundBarcodeByOutboundDetailID, error) {

	// sql := `select a.inbound_detail_id, a.item_code, a.item_id, a.status,
	// 	sum(a.quantity) as total_scan, a.qa_status, a.rec_date, a.prod_date, a.lot_number, a.exp_date
	// 	from inbound_barcodes a
	// 	where inbound_detail_id = ?
	// 	group by a.inbound_detail_id, a.item_code, a.item_id, a.status, a.qa_status, a.rec_date, a.prod_date, a.lot_number, a.exp_date
	// 	order by a.inbound_detail_id asc;`

	sql := `select a.inbound_detail_id, a.item_code, a.item_id, a.status,
		sum(a.quantity) as total_scan, a.qa_status, 
		a.rec_date, a.prod_date, a.lot_number, a.exp_date
		from inbound_barcodes a
		where inbound_detail_id = ?
		group by a.inbound_detail_id, a.item_code, a.item_id, a.status, 
		a.qa_status, a.rec_date, a.prod_date, a.lot_number, a.exp_date
		order by a.inbound_detail_id asc;`

	var result resulInboundBarcodeByOutboundDetailID
	if err := r.db.Raw(sql, outboundDetailID).Scan(&result).Error; err != nil {
		return resulInboundBarcodeByOutboundDetailID{}, err
	}

	return result, nil
}

func (r *InboundRepository) UpdateStatusInbound(ctx *fiber.Ctx, inboundHeaderID uint) error {

	type CheckResult struct {
		InboundNo       string `json:"inbound_no"`
		InboundDetailId int    `json:"inbound_detail_id"`
		ItemId          int    `json:"item_id"`
		Quantity        int    `json:"quantity"`
		QtyScan         int    `json:"qty_scan"`
	}

	var inboundHeader models.InboundHeader
	if err := r.db.First(&inboundHeader, inboundHeaderID).Error; err != nil {
		return errors.New(err.Error())
	}

	sqlCheck := `WITH ib AS
	(
		SELECT inbound_id, inbound_detail_id, item_id, SUM(quantity) AS qty_scan, status
		FROM inbound_barcodes WHERE inbound_id = ? AND status = 'in stock'
		GROUP BY inbound_id, inbound_detail_id, item_id, status
	)

	SELECT a.id, a.inbound_no, a.inbound_id, a.item_id, a.quantity, COALESCE(ib.qty_scan, 0) AS qty_scan
	FROM inbound_details a
	LEFT JOIN ib ON a.id = ib.inbound_detail_id
	WHERE a.inbound_id = ?`

	var checkResult []CheckResult
	if err := r.db.Raw(sqlCheck, inboundHeaderID, inboundHeaderID).Scan(&checkResult).Error; err != nil {
		return errors.New(err.Error())
	}

	qtyRequest := 0
	qtyReceived := 0
	for _, result := range checkResult {
		qtyRequest += result.Quantity
		qtyReceived += result.QtyScan
	}

	statusInbound := "fully received"
	if qtyRequest != qtyReceived {
		if qtyReceived == 0 {
			statusInbound = "checking"
		} else {
			statusInbound = "partially received"
		}
	}

	userID := int(ctx.Locals("userID").(float64))

	now := time.Now()
	updateData := models.InboundHeader{
		Status:    statusInbound,
		PutawayAt: &now,
		PutawayBy: userID,
	}
	if err := r.db.Debug().Model(&models.InboundHeader{}).
		Where("id = ?", inboundHeaderID).
		Updates(updateData).Error; err != nil {
		return errors.New(err.Error())
	}

	errHistory := helpers.InsertTransactionHistory(
		r.db,
		inboundHeader.InboundNo,
		statusInbound,
		"INBOUND",
		"",
		userID,
	)
	if errHistory != nil {
		log.Println("Gagal insert history:", errHistory)
		return errors.New(errHistory.Error())
	}

	return nil
}
