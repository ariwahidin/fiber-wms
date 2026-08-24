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

func (r *InboundRepository) GeneratePalletID(inboundNo string) (string, error) {
	prefix := inboundNo + "-"

	var pallets []string
	if err := r.db.Model(&models.InboundBarcode{}).
		Where("pallet LIKE ?", prefix+"%").
		Pluck("pallet", &pallets).Error; err != nil {
		return "", err
	}

	maxSeq := 0
	for _, p := range pallets {
		seqStr := strings.TrimPrefix(p, prefix)
		if seq, err := strconv.Atoi(seqStr); err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}

	newPalletID := fmt.Sprintf("%s%d", prefix, maxSeq+1)

	// pastiin ngga ada yang sama (jaga-jaga race condition)
	var count int64
	if err := r.db.Model(&models.InboundBarcode{}).
		Where("pallet = ?", newPalletID).
		Count(&count).Error; err != nil {
		return "", err
	}
	if count > 0 {
		return "", fmt.Errorf("pallet ID %s sudah ada", newPalletID)
	}

	return newPalletID, nil
}

func (r *InboundRepository) ProcessPutawayItem(ctx *fiber.Ctx, inboundBarcodeID int, location string) (bool, error) {
	userID, ok := ctx.Locals("userID").(float64)
	movementID := uuid.NewString()
	if !ok {
		return false, errors.New("invalid user ID")
	}

	var barcode models.InboundBarcode
	if err := r.db.Where("id = ?", inboundBarcodeID).Take(&barcode).Error; err != nil {
		return false, err
	}

	var inboundHeader models.InboundHeader
	if err := r.db.Where("id = ?", barcode.InboundId).Take(&inboundHeader).Error; err != nil {
		return false, errors.New("inbound header not found for item: " + barcode.ItemCode)
	}

	var inventoryPolicy models.InventoryPolicy
	if err := r.db.Where("owner_code = ?", inboundHeader.OwnerCode).Take(&inventoryPolicy).Error; err != nil {
		return false, errors.New("inventory policy not found for owner code: " + inboundHeader.OwnerCode)
	}

	if barcode.Status != "pending" {
		return false, fmt.Errorf("item not in pending status")
	}

	var detail models.InboundDetail
	if err := r.db.Where("id = ?", barcode.InboundDetailId).Take(&detail).Error; err != nil {
		return false, errors.New("inbound detail not found for item: " + barcode.ItemCode)
	}

	if location == "" {
		location = barcode.Location
	}

	uomRepo := NewUomRepository(r.db)
	uomConversion, errUom := uomRepo.ConversionQty(barcode.ItemCode, barcode.Quantity, detail.Uom)
	if errUom != nil {
		return false, errUom
	}
	qtyConverted := uomConversion.QtyConverted

	var product models.Product
	if err := r.db.Where("item_code = ?", barcode.ItemCode).Take(&product).Error; err != nil {
		return false, errors.New("product not found for item: " + barcode.ItemCode)
	}

	CartonSerial := ""
	result, _ := r.GetScanData(uint(barcode.ID))
	if result != nil && result.CartonSerial != nil {
		CartonSerial = *result.CartonSerial
	}

	if CartonSerial == "" {
		CartonSerial = barcode.CartonNumber
	}

	// Cek apakah data inventory dengan kombinasi yang sama sudah ada
	// var existingInv models.Inventory
	// invQuery := r.db.Where(`
	// 		inbound_id = ? AND
	// 		inbound_detail_id = ? AND
	// 		item_code = ? AND
	// 		location = ? AND
	// 		barcode = ? AND
	// 		whs_code = ? AND
	// 		qa_status = ? AND
	// 		rec_date = ? AND
	// 		COALESCE(prod_date, '') = COALESCE(?, '') AND
	// 		COALESCE(exp_date, '') = COALESCE(?, '') AND
	// 		COALESCE(lot_number, '') = COALESCE(?, '') AND
	// 		COALESCE(carton_number, '') = COALESCE(?, '')
	// 	`,
	// 	barcode.InboundId,
	// 	barcode.InboundDetailId,
	// 	barcode.ItemCode,
	// 	location,
	// 	product.Barcode,
	// 	barcode.WhsCode,
	// 	barcode.QaStatus,
	// 	barcode.RecDate,
	// 	barcode.ProdDate,
	// 	barcode.ExpDate,
	// 	barcode.LotNumber,
	// 	CartonSerial,
	// ).First(&existingInv)

	var existingInv models.Inventory
	invQuery := r.db.Where(`
        inbound_id = ? AND
        inbound_detail_id = ? AND
        item_code = ? AND
        location = ? AND
        barcode = ? AND
        whs_code = ? AND
        qa_status = ? AND
        rec_date = ? AND
        COALESCE(prod_date, '') = COALESCE(?, '') AND
        COALESCE(exp_date, '') = COALESCE(?, '') AND
        COALESCE(lot_number, '') = COALESCE(?, '') AND
        COALESCE(carton_number, '') = COALESCE(?, '')
    `,
		barcode.InboundId,
		barcode.InboundDetailId,
		barcode.ItemCode,
		location,
		product.Barcode,
		barcode.WhsCode,
		barcode.QaStatus,
		barcode.RecDate,
		barcode.ProdDate,
		barcode.ExpDate,
		barcode.LotNumber,
		CartonSerial,
	)

	// tambahan kondisional
	if inventoryPolicy.UseSerialNumber {
		invQuery = invQuery.Where("serial_number = ?", barcode.SerialNumber)
	}

	err := invQuery.First(&existingInv).Error

	serialNumber := ""
	if inventoryPolicy.UseSerialNumber && barcode.SerialNumber != "" {
		serialNumber = barcode.SerialNumber
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
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
			CartonNumber:    CartonSerial,
			SerialNumber:    serialNumber,
			QaStatus:        barcode.QaStatus,
			Uom:             uomConversion.ToUom,
			QtyOrigin:       qtyConverted,
			QtyOnhand:       qtyConverted,
			QtyAvailable:    qtyConverted,
			ExpDate:         barcode.ExpDate,
			ProdDate:        barcode.ProdDate,
			LotNumber:       barcode.LotNumber,
			Trans:           "INBOUND PUTAWAY",
			CreatedBy:       int(userID),
		}

		if err := r.db.Create(&newInv).Error; err != nil {
			return false, err
		}

		// ledger
		helpers.InsertInventoryMovement(r.db, helpers.InventoryMovementPayload{
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

	} else if err == nil {
		// Sudah ada → Update qty
		if err := r.db.Model(&existingInv).Updates(map[string]interface{}{
			"qty_origin":    existingInv.QtyOrigin + qtyConverted,
			"qty_onhand":    existingInv.QtyOnhand + qtyConverted,
			"qty_available": existingInv.QtyAvailable + qtyConverted,
			"updated_at":    time.Now().UTC(),
			"updated_by":    int(userID),
		}).Error; err != nil {
			return false, err
		}

		// ledger
		helpers.InsertInventoryMovement(r.db, helpers.InventoryMovementPayload{
			InventoryID:        existingInv.ID,
			MovementID:         movementID,
			RefType:            "INBOUND PUTAWAY",
			RefID:              uint(barcode.InboundId),
			ItemID:             product.ID,
			ItemCode:           product.ItemCode,
			ToWhsCode:          existingInv.WhsCode,
			QtyOnhandChange:    qtyConverted,
			QtyAvailableChange: qtyConverted,
			NewQaStatus:        barcode.QaStatus,
			FromLocation:       barcode.Location,
			ToLocation:         location,
			Reason:             detail.InboundNo + " PUTAWAY",
			CreatedBy:          int(userID),
		})
	} else {
		return false, err
	}

	// Update status barcode ke "in stock"
	if err := r.db.Model(&barcode).Updates(map[string]interface{}{
		"status":           "in stock",
		"putaway_location": location,
		"putaway_qty":      barcode.Quantity,
		"putaway_at":       time.Now().UTC(),
		"putaway_by":       int(userID),
		"updated_at":       time.Now().UTC(),
		"updated_by":       int(userID),
	}).Error; err != nil {
		return false, err
	}

	return true, nil
}

type putawayCalc struct {
	barcode      models.InboundBarcode
	product      models.Product
	detail       models.InboundDetail
	location     string
	qtyConverted float64
	toUom        string
	cartonSerial string
	key          InventoryMatchKey
}

// InventoryMatchKey — struct key untuk matching row Inventory. Comparable native,
// nggak ada risiko collision dari string concatenation.
type InventoryMatchKey struct {
	InboundID       int
	InboundDetailId int
	ItemCode        string
	Location        string
	Barcode         string
	WhsCode         string
	QaStatus        string
	RecDate         string
	ProdDate        string
	ExpDate         string
	LotNumber       string
	CartonNumber    string
	SerialNumber    string // cuma dipakai kalau UseSerialNumber aktif
}

func buildMatchKey(usesSerial bool, inboundID, detailID int, itemCode, location, barcodeField, whsCode, qaStatus, recDate, prodDate, expDate, lotNumber, cartonSerial, serialNumber string) InventoryMatchKey {
	k := InventoryMatchKey{
		InboundID:       inboundID,
		InboundDetailId: detailID,
		ItemCode:        itemCode,
		Location:        location,
		Barcode:         barcodeField,
		WhsCode:         whsCode,
		QaStatus:        qaStatus,
		RecDate:         recDate,
		ProdDate:        prodDate,
		ExpDate:         expDate,
		LotNumber:       lotNumber,
		CartonNumber:    cartonSerial,
	}
	if usesSerial {
		k.SerialNumber = serialNumber
	}
	return k
}

func (r *InboundRepository) ProcessPutawayItemsBatch(ctx *fiber.Ctx, barcodeIDs []int) error {
	userIDFloat, ok := ctx.Locals("userID").(float64)
	if !ok {
		return errors.New("invalid user ID")
	}
	userID := int(userIDFloat)

	if len(barcodeIDs) == 0 {
		return nil
	}

	var barcodes []models.InboundBarcode
	if err := helpers.FindInChunks(r.db, "id IN ?", barcodeIDs, &barcodes); err != nil {
		return err
	}

	for _, b := range barcodes {
		if b.Status != "pending" {
			return fmt.Errorf("item not in pending status: barcode id %d", b.ID)
		}
	}

	inboundIDSet := map[int]bool{}
	detailIDSet := map[int]bool{}
	itemCodeSet := map[string]bool{}
	barcodeIDsUint := make([]uint, 0, len(barcodes))
	for _, b := range barcodes {
		inboundIDSet[b.InboundId] = true
		detailIDSet[b.InboundDetailId] = true
		itemCodeSet[b.ItemCode] = true
		barcodeIDsUint = append(barcodeIDsUint, uint(b.ID))
	}

	inboundIDs := toIntSlice(inboundIDSet)
	detailIDs := toIntSlice(detailIDSet)
	itemCodes := toStringSlice(itemCodeSet)

	var headers []models.InboundHeader
	if err := helpers.FindInChunks(r.db, "id IN ?", inboundIDs, &headers); err != nil {
		return err
	}
	headerMap := make(map[int]models.InboundHeader, len(headers))
	ownerCodeSet := map[string]bool{}
	for _, h := range headers {
		headerMap[int(h.ID)] = h
		ownerCodeSet[h.OwnerCode] = true
	}

	var policies []models.InventoryPolicy
	if err := helpers.FindInChunks(r.db, "owner_code IN ?", toStringSlice(ownerCodeSet), &policies); err != nil {
		return err
	}
	policyMap := make(map[string]models.InventoryPolicy, len(policies))
	for _, p := range policies {
		policyMap[p.OwnerCode] = p
	}

	var details []models.InboundDetail
	if err := helpers.FindInChunks(r.db, "id IN ?", detailIDs, &details); err != nil {
		return err
	}
	detailMap := make(map[int]models.InboundDetail, len(details))
	for _, d := range details {
		detailMap[int(d.ID)] = d
	}

	var products []models.Product
	if err := helpers.FindInChunks(r.db, "item_code IN ?", itemCodes, &products); err != nil {
		return err
	}
	productMap := make(map[string]models.Product, len(products))
	for _, p := range products {
		productMap[p.ItemCode] = p
	}

	scanDataMap, err := r.GetScanDataBatch(barcodeIDsUint)
	if err != nil {
		return err
	}

	var existingInvs []models.Inventory
	if err := helpers.FindInChunks(r.db, "inbound_detail_id IN ?", detailIDs, &existingInvs); err != nil {
		return err
	}

	uomRepo := NewUomRepository(r.db)
	conversionCache := make(map[string]UomConversionResult)

	getConversion := func(itemCode string, qty float64, fromUom string) (UomConversionResult, error) {
		cacheKey := itemCode + "|" + fromUom
		if cached, ok := conversionCache[cacheKey]; ok {
			cached.FromQty = qty
			cached.QtyConverted = qty * cached.Rate
			return cached, nil
		}
		res, err := uomRepo.ConversionQty(itemCode, qty, fromUom)
		if err != nil {
			return UomConversionResult{}, err
		}
		conversionCache[cacheKey] = res
		return res, nil
	}

	// pakai FUNCTION package-level buildMatchKey — TIDAK ada closure lokal lagi
	existingInvMap := make(map[InventoryMatchKey]*models.Inventory, len(existingInvs))
	for i := range existingInvs {
		inv := &existingInvs[i]
		policy := policyMap[inv.OwnerCode]
		key := buildMatchKey(policy.UseSerialNumber, inv.InboundID, inv.InboundDetailId, inv.ItemCode, inv.Location, inv.Barcode, inv.WhsCode, inv.QaStatus, inv.RecDate, inv.ProdDate, inv.ExpDate, inv.LotNumber, inv.CartonNumber, inv.SerialNumber)
		existingInvMap[key] = inv
	}

	calcs := make([]putawayCalc, 0, len(barcodes))
	for _, barcode := range barcodes {
		header := headerMap[barcode.InboundId]
		policy := policyMap[header.OwnerCode]
		detail := detailMap[barcode.InboundDetailId]
		product := productMap[barcode.ItemCode]

		location := barcode.Location
		if location == "" {
			location = detail.Location
		}

		uomRes, err := getConversion(barcode.ItemCode, barcode.Quantity, detail.Uom)
		if err != nil {
			return err
		}

		cartonSerial := ""
		if sd, ok := scanDataMap[uint(barcode.ID)]; ok && sd.CartonSerial != nil {
			cartonSerial = *sd.CartonSerial
		}
		if cartonSerial == "" {
			cartonSerial = barcode.CartonNumber
		}

		key := buildMatchKey(policy.UseSerialNumber, barcode.InboundId, barcode.InboundDetailId, barcode.ItemCode, location, product.Barcode, barcode.WhsCode, barcode.QaStatus, barcode.RecDate, barcode.ProdDate, barcode.ExpDate, barcode.LotNumber, cartonSerial, barcode.SerialNumber)

		calcs = append(calcs, putawayCalc{
			barcode: barcode, product: product, detail: detail,
			location: location, qtyConverted: uomRes.QtyConverted, toUom: uomRes.ToUom,
			cartonSerial: cartonSerial, key: key,
		})
	}

	newInvByKey := make(map[InventoryMatchKey]*models.Inventory)
	sumQtyByKey := make(map[InventoryMatchKey]float64)
	for _, c := range calcs {
		sumQtyByKey[c.key] += c.qtyConverted
		if _, exists := existingInvMap[c.key]; exists {
			continue
		}
		if _, exists := newInvByKey[c.key]; !exists {
			newInvByKey[c.key] = &models.Inventory{
				InboundID: c.detail.InboundId, InboundDetailId: int(c.detail.ID), RecDate: c.detail.RecDate,
				ItemId: c.barcode.ItemID, ItemCode: c.barcode.ItemCode, Barcode: c.product.Barcode,
				WhsCode: c.barcode.WhsCode, OwnerCode: c.barcode.OwnerCode, DivisionCode: c.barcode.DivisionCode,
				Pallet: c.barcode.Pallet, Location: c.location, CartonNumber: c.cartonSerial,
				SerialNumber: c.barcode.SerialNumber, QaStatus: c.barcode.QaStatus, Uom: c.toUom,
				ExpDate: c.barcode.ExpDate, ProdDate: c.barcode.ProdDate, LotNumber: c.barcode.LotNumber,
				Trans: "INBOUND PUTAWAY", CreatedBy: userID,
			}
		}
	}

	movements := make([]models.InventoryMovement, 0, len(calcs))

	err = r.db.Transaction(func(tx *gorm.DB) error {

		for key, inv := range existingInvMap {
			add, touched := sumQtyByKey[key]
			if !touched {
				continue
			}
			if err := tx.Model(inv).Updates(map[string]interface{}{
				"qty_origin":    inv.QtyOrigin + add,
				"qty_onhand":    inv.QtyOnhand + add,
				"qty_available": inv.QtyAvailable + add,
				"updated_at":    time.Now().UTC(),
				"updated_by":    userID,
			}).Error; err != nil {
				return err
			}
		}

		if len(newInvByKey) > 0 {
			keys := make([]InventoryMatchKey, 0, len(newInvByKey))
			newInvList := make([]models.Inventory, 0, len(newInvByKey))
			for key, inv := range newInvByKey {
				inv.QtyOrigin = sumQtyByKey[key]
				inv.QtyOnhand = sumQtyByKey[key]
				inv.QtyAvailable = sumQtyByKey[key]
				keys = append(keys, key)
				newInvList = append(newInvList, *inv)
			}
			if err := helpers.CreateInChunks(tx, newInvList); err != nil {
				return err
			}
			for i, key := range keys {
				newInvByKey[key].ID = newInvList[i].ID
			}
		}

		for _, c := range calcs {
			var invID uint
			if inv, ok := existingInvMap[c.key]; ok {
				invID = inv.ID
			} else if inv, ok := newInvByKey[c.key]; ok {
				invID = inv.ID
			}

			movements = append(movements, models.InventoryMovement{
				InventoryID:        invID,
				MovementID:         uuid.NewString(),
				RefType:            "INBOUND PUTAWAY",
				RefID:              uint(c.barcode.InboundId),
				ItemID:             c.product.ID,
				ItemCode:           c.product.ItemCode,
				ToWhsCode:          c.barcode.WhsCode,
				QtyOnhandChange:    c.qtyConverted,
				QtyAvailableChange: c.qtyConverted,
				NewQaStatus:        c.barcode.QaStatus,
				FromLocation:       c.barcode.Location,
				ToLocation:         c.location,
				Reason:             c.detail.InboundNo + " PUTAWAY",
				CreatedBy:          userID,
				CreatedAt:          time.Now(),
			})
		}

		if err := helpers.CreateInChunks(tx, movements); err != nil {
			return err
		}

		now := time.Now().UTC()
		for _, c := range calcs {
			if err := tx.Model(&models.InboundBarcode{}).Where("id = ?", c.barcode.ID).Updates(map[string]interface{}{
				"status":           "in stock",
				"putaway_location": c.location,
				"putaway_qty":      int(c.barcode.Quantity),
				"putaway_at":       now,
				"putaway_by":       userID,
				"updated_at":       now,
				"updated_by":       userID,
			}).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// func (r *InboundRepository) ProcessPutawayItemsBatch(ctx *fiber.Ctx, barcodeIDs []int) error {
// 	userIDFloat, ok := ctx.Locals("userID").(float64)
// 	if !ok {
// 		return errors.New("invalid user ID")
// 	}
// 	userID := int(userIDFloat)

// 	if len(barcodeIDs) == 0 {
// 		return nil
// 	}

// 	// === PATCH 1: barcodes ===
// 	var barcodes []models.InboundBarcode
// 	if err := helpers.FindInChunks(r.db, "id IN ?", barcodeIDs, &barcodes); err != nil {
// 		return err
// 	}

// 	for _, b := range barcodes {
// 		if b.Status != "pending" {
// 			return fmt.Errorf("item not in pending status: barcode id %d", b.ID)
// 		}
// 	}

// 	inboundIDSet := map[int]bool{}
// 	detailIDSet := map[int]bool{}
// 	itemCodeSet := map[string]bool{}
// 	barcodeIDsUint := make([]uint, 0, len(barcodes))
// 	for _, b := range barcodes {
// 		inboundIDSet[b.InboundId] = true
// 		detailIDSet[b.InboundDetailId] = true
// 		itemCodeSet[b.ItemCode] = true
// 		barcodeIDsUint = append(barcodeIDsUint, uint(b.ID))
// 	}

// 	inboundIDs := toIntSlice(inboundIDSet)
// 	detailIDs := toIntSlice(detailIDSet)
// 	itemCodes := toStringSlice(itemCodeSet)

// 	// === PATCH 2: headers ===
// 	var headers []models.InboundHeader
// 	if err := helpers.FindInChunks(r.db, "id IN ?", inboundIDs, &headers); err != nil {
// 		return err
// 	}
// 	headerMap := make(map[int]models.InboundHeader, len(headers))
// 	ownerCodeSet := map[string]bool{}
// 	for _, h := range headers {
// 		headerMap[int(h.ID)] = h
// 		ownerCodeSet[h.OwnerCode] = true
// 	}

// 	// === PATCH 3: policies ===
// 	var policies []models.InventoryPolicy
// 	if err := helpers.FindInChunks(r.db, "owner_code IN ?", toStringSlice(ownerCodeSet), &policies); err != nil {
// 		return err
// 	}
// 	policyMap := make(map[string]models.InventoryPolicy, len(policies))
// 	for _, p := range policies {
// 		policyMap[p.OwnerCode] = p
// 	}

// 	// === PATCH 4: details ===
// 	var details []models.InboundDetail
// 	if err := helpers.FindInChunks(r.db, "id IN ?", detailIDs, &details); err != nil {
// 		return err
// 	}
// 	detailMap := make(map[int]models.InboundDetail, len(details))
// 	for _, d := range details {
// 		detailMap[int(d.ID)] = d
// 	}

// 	// === PATCH 5: products ===
// 	var products []models.Product
// 	if err := helpers.FindInChunks(r.db, "item_code IN ?", itemCodes, &products); err != nil {
// 		return err
// 	}
// 	productMap := make(map[string]models.Product, len(products))
// 	for _, p := range products {
// 		productMap[p.ItemCode] = p
// 	}

// 	// === PATCH 6: scan data — sudah di-chunk di dalam GetScanDataBatch (poin A di atas) ===
// 	scanDataMap, err := r.GetScanDataBatch(barcodeIDsUint)
// 	if err != nil {
// 		return err
// 	}

// 	// === PATCH 7: existing inventory ===
// 	var existingInvs []models.Inventory
// 	if err := helpers.FindInChunks(r.db, "inbound_detail_id IN ?", detailIDs, &existingInvs); err != nil {
// 		return err
// 	}

// 	// ===== UOM conversion cache (tidak berubah — bukan batch query) =====
// 	uomRepo := NewUomRepository(r.db)
// 	conversionCache := make(map[string]UomConversionResult)

// 	getConversion := func(itemCode string, qty float64, fromUom string) (UomConversionResult, error) {
// 		cacheKey := itemCode + "|" + fromUom
// 		if cached, ok := conversionCache[cacheKey]; ok {
// 			cached.FromQty = qty
// 			cached.QtyConverted = qty * cached.Rate
// 			return cached, nil
// 		}
// 		res, err := uomRepo.ConversionQty(itemCode, qty, fromUom)
// 		if err != nil {
// 			return UomConversionResult{}, err
// 		}
// 		conversionCache[cacheKey] = res
// 		return res, nil
// 	}

// 	buildKey := func(usesSerial bool, invID, detailID int, itemCode, location, barcodeField, whsCode, qaStatus, recDate, prodDate, expDate, lotNumber, cartonSerial, serialNumber string) string {
// 		k := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", detailID, itemCode, location, barcodeField, whsCode, qaStatus, recDate, prodDate, expDate, lotNumber, cartonSerial)
// 		if usesSerial {
// 			k += "|" + serialNumber
// 		}
// 		return fmt.Sprintf("%d|%s", invID, k)
// 	}

// 	existingInvMap := make(map[string]*models.Inventory, len(existingInvs))
// 	for i := range existingInvs {
// 		inv := &existingInvs[i]
// 		policy := policyMap[inv.OwnerCode]
// 		key := buildKey(policy.UseSerialNumber, inv.InboundID, inv.InboundDetailId, inv.ItemCode, inv.Location, inv.Barcode, inv.WhsCode, inv.QaStatus, inv.RecDate, inv.ProdDate, inv.ExpDate, inv.LotNumber, inv.CartonNumber, inv.SerialNumber)
// 		existingInvMap[key] = inv
// 	}

// 	// ===== PASS 1: hitung semua barcode =====
// 	calcs := make([]putawayCalc, 0, len(barcodes))
// 	for _, barcode := range barcodes {
// 		header := headerMap[barcode.InboundId]
// 		policy := policyMap[header.OwnerCode]
// 		detail := detailMap[barcode.InboundDetailId]
// 		product := productMap[barcode.ItemCode]

// 		location := barcode.Location
// 		if location == "" {
// 			location = detail.Location
// 		}

// 		uomRes, err := getConversion(barcode.ItemCode, barcode.Quantity, detail.Uom)
// 		if err != nil {
// 			return err
// 		}

// 		cartonSerial := ""
// 		if sd, ok := scanDataMap[uint(barcode.ID)]; ok && sd.CartonSerial != nil {
// 			cartonSerial = *sd.CartonSerial
// 		}
// 		if cartonSerial == "" {
// 			cartonSerial = barcode.CartonNumber
// 		}

// 		key := buildKey(policy.UseSerialNumber, barcode.InboundId, barcode.InboundDetailId, barcode.ItemCode, location, product.Barcode, barcode.WhsCode, barcode.QaStatus, barcode.RecDate, barcode.ProdDate, barcode.ExpDate, barcode.LotNumber, cartonSerial, barcode.SerialNumber)

// 		calcs = append(calcs, putawayCalc{
// 			barcode: barcode, product: product, detail: detail,
// 			location: location, qtyConverted: uomRes.QtyConverted, toUom: uomRes.ToUom,
// 			cartonSerial: cartonSerial, key: key,
// 		})
// 	}

// 	// ===== PASS 2a: accumulate qty per key =====
// 	newInvByKey := make(map[string]*models.Inventory)
// 	sumQtyByKey := make(map[string]float64)
// 	for _, c := range calcs {
// 		sumQtyByKey[c.key] += c.qtyConverted
// 		if _, exists := existingInvMap[c.key]; exists {
// 			continue
// 		}
// 		if _, exists := newInvByKey[c.key]; !exists {
// 			newInvByKey[c.key] = &models.Inventory{
// 				InboundID: c.detail.InboundId, InboundDetailId: int(c.detail.ID), RecDate: c.detail.RecDate,
// 				ItemId: c.barcode.ItemID, ItemCode: c.barcode.ItemCode, Barcode: c.product.Barcode,
// 				WhsCode: c.barcode.WhsCode, OwnerCode: c.barcode.OwnerCode, DivisionCode: c.barcode.DivisionCode,
// 				Pallet: c.barcode.Pallet, Location: c.location, CartonNumber: c.cartonSerial,
// 				SerialNumber: c.barcode.SerialNumber, QaStatus: c.barcode.QaStatus, Uom: c.toUom,
// 				ExpDate: c.barcode.ExpDate, ProdDate: c.barcode.ProdDate, LotNumber: c.barcode.LotNumber,
// 				Trans: "INBOUND PUTAWAY", CreatedBy: userID,
// 			}
// 		}
// 	}

// 	// Update existing rows — masih per-row karena payload beda tiap row (Updates map beda value),
// 	// tapi ini UPDATE murni, bukan Create, jadi TIDAK kena limit 2100 parameter (cuma ~6 param/query)
// 	for key, inv := range existingInvMap {
// 		add, touched := sumQtyByKey[key]
// 		if !touched {
// 			continue
// 		}
// 		if err := r.db.Model(inv).Updates(map[string]interface{}{
// 			"qty_origin":    inv.QtyOrigin + add,
// 			"qty_onhand":    inv.QtyOnhand + add,
// 			"qty_available": inv.QtyAvailable + add,
// 			"updated_at":    time.Now().UTC(),
// 			"updated_by":    userID,
// 		}).Error; err != nil {
// 			return err
// 		}
// 	}

// 	if len(newInvByKey) > 0 {
// 		keys := make([]string, 0, len(newInvByKey))
// 		newInvList := make([]models.Inventory, 0, len(newInvByKey))
// 		for key, inv := range newInvByKey {
// 			inv.QtyOrigin = sumQtyByKey[key]
// 			inv.QtyOnhand = sumQtyByKey[key]
// 			inv.QtyAvailable = sumQtyByKey[key]
// 			keys = append(keys, key)
// 			newInvList = append(newInvList, *inv)
// 		}
// 		if err := helpers.CreateInChunks(r.db, newInvList); err != nil {
// 			return err
// 		}
// 		for i, key := range keys {
// 			newInvByKey[key].ID = newInvList[i].ID
// 		}
// 	}

// 	// ===== PASS 2b: bangun ledger movement =====
// 	movements := make([]models.InventoryMovement, 0, len(calcs))
// 	for _, c := range calcs {
// 		var invID uint
// 		if inv, ok := existingInvMap[c.key]; ok {
// 			invID = inv.ID
// 		} else if inv, ok := newInvByKey[c.key]; ok {
// 			invID = inv.ID
// 		}

// 		movements = append(movements, models.InventoryMovement{
// 			InventoryID:        invID,
// 			MovementID:         uuid.NewString(),
// 			RefType:            "INBOUND PUTAWAY",
// 			RefID:              uint(c.barcode.InboundId),
// 			ItemID:             c.product.ID,
// 			ItemCode:           c.product.ItemCode,
// 			ToWhsCode:          c.barcode.WhsCode,
// 			QtyOnhandChange:    c.qtyConverted,
// 			QtyAvailableChange: c.qtyConverted,
// 			NewQaStatus:        c.barcode.QaStatus,
// 			FromLocation:       c.barcode.Location,
// 			ToLocation:         c.location,
// 			Reason:             c.detail.InboundNo + " PUTAWAY",
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		})
// 	}

// 	// === PATCH 9: insert movements — pakai CreateInChunks ===
// 	if err := helpers.CreateInChunks(r.db, movements); err != nil {
// 		return err
// 	}

// 	// ===== Update status barcode → "in stock" — per row, UPDATE murni, aman dari limit =====
// 	now := time.Now().UTC()
// 	for _, c := range calcs {
// 		if err := r.db.Model(&models.InboundBarcode{}).Where("id = ?", c.barcode.ID).Updates(map[string]interface{}{
// 			"status":           "in stock",
// 			"putaway_location": c.location,
// 			"putaway_qty":      int(c.barcode.Quantity),
// 			"putaway_at":       now,
// 			"putaway_by":       userID,
// 			"updated_at":       now,
// 			"updated_by":       userID,
// 		}).Error; err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// helper kecil
func toIntSlice(m map[int]bool) []int {
	s := make([]int, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}
func toStringSlice(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
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

type ResulInboundBarcodeByOutboundDetailID struct {
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

func (r *InboundRepository) GetInboundBarcodesByDetailIDs(detailIDs []uint) (map[uint]ResulInboundBarcodeByOutboundDetailID, error) {
	if len(detailIDs) == 0 {
		return map[uint]ResulInboundBarcodeByOutboundDetailID{}, nil
	}

	sql := `select a.inbound_detail_id as id, a.item_code, a.item_id, a.status,
		sum(a.quantity) as total_scan, a.qa_status, 
		a.rec_date, a.prod_date, a.lot_number, a.exp_date				
		from inbound_barcodes a
		where inbound_detail_id IN ?
		group by a.inbound_detail_id, a.item_code, a.item_id, a.status, 
		a.qa_status, a.rec_date, a.prod_date, a.lot_number, a.exp_date
		order by a.inbound_detail_id asc;`

	fmt.Println("SQL Query:", sql)

	var results []ResulInboundBarcodeByOutboundDetailID
	if err := r.db.Debug().Raw(sql, detailIDs).Scan(&results).Error; err != nil {
		return nil, err
	}

	resultMap := make(map[uint]ResulInboundBarcodeByOutboundDetailID, len(results))
	for _, res := range results {
		resultMap[res.ID] = res // sesuaikan nama field sesuai struct resulInboundBarcodeByOutboundDetailID kamu
	}

	return resultMap, nil
}

func (r *InboundRepository) GetInboundBarcodeByOutboundDetailID(outboundDetailID uint) (ResulInboundBarcodeByOutboundDetailID, error) {

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

	var result ResulInboundBarcodeByOutboundDetailID
	if err := r.db.Raw(sql, outboundDetailID).Scan(&result).Error; err != nil {
		return ResulInboundBarcodeByOutboundDetailID{}, err
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

	if statusInbound == "checking" && inboundHeader.CheckingAt == nil {
		updateData.CheckingAt = &now
		updateData.CheckingBy = userID
		updateData.PutawayAt = nil
		updateData.PutawayBy = 0
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

type ScanDataResult struct {
	InboundBarcodeID uint       `json:"inbound_barcode_id"`
	InboundID        uint       `json:"inbound_id"`
	InboundDetailID  uint       `json:"inbound_detail_id"`
	LotNumber        *string    `json:"lot_number"`
	RawScanData      *string    `json:"raw_scan_data"`
	ItemType         string     `json:"item_type"`
	SKU              *string    `json:"sku"`
	EAN              *string    `json:"ean"`
	Product          *string    `json:"product"`
	Brand            *string    `json:"brand"`
	Model            *string    `json:"model"`
	Serial           *string    `json:"serial"`
	CartonSerial     *string    `json:"carton_serial"`
	Batch            *string    `json:"batch"`
	MfgDateRaw       *string    `json:"mfg_date_raw"`
	MfgDate          *time.Time `json:"mfg_date"`
	QtyPerCarton     *int       `json:"qty_per_carton"`
}

func (r *InboundRepository) GetScanData(inboundBarcodeID uint) (*ScanDataResult, error) {
	query := `
		SELECT
			ib.id AS inbound_barcode_id,
			ib.inbound_id,
			ib.inbound_detail_id,
			ib.lot_number,
			ib.scan_data AS raw_scan_data,

			CASE 
				WHEN ib.scan_data IS NULL OR LEN(TRIM(ib.scan_data)) = 0 THEN 'INVALID - EMPTY'
				WHEN CHARINDEX('(1)SKU=', ib.scan_data) = 0 THEN 'INVALID - UNKNOWN FORMAT'
				WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 THEN 'CARTON'
				ELSE 'UNIT'
			END AS item_type,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(1)SKU=', ib.scan_data) > 0 
					  AND CHARINDEX('(2)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(1)SKU=', ib.scan_data) + 7,
					CHARINDEX('(2)', ib.scan_data) - CHARINDEX('(1)SKU=', ib.scan_data) - 7)
			END AS sku,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(2)EAN=', ib.scan_data) > 0 
					  AND CHARINDEX('(3)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(2)EAN=', ib.scan_data) + 7,
					CHARINDEX('(3)', ib.scan_data) - CHARINDEX('(2)EAN=', ib.scan_data) - 7)
			END AS ean,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(3)PRODUCT=', ib.scan_data) > 0 
					  AND CHARINDEX('(4)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(3)PRODUCT=', ib.scan_data) + 11,
					CHARINDEX('(4)', ib.scan_data) - CHARINDEX('(3)PRODUCT=', ib.scan_data) - 11)
			END AS product,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(4)BRAND=', ib.scan_data) > 0 
					  AND CHARINDEX('(5)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(4)BRAND=', ib.scan_data) + 9,
					CHARINDEX('(5)', ib.scan_data) - CHARINDEX('(4)BRAND=', ib.scan_data) - 9)
			END AS brand,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(5)MODEL=', ib.scan_data) > 0 
					  AND CHARINDEX('(6)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(5)MODEL=', ib.scan_data) + 9,
					CHARINDEX('(6)', ib.scan_data) - CHARINDEX('(5)MODEL=', ib.scan_data) - 9)
			END AS model,

			CASE WHEN CHARINDEX('(6)SERIAL=', ib.scan_data) > 0 
					  AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(6)SERIAL=', ib.scan_data) + 10,
					CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)SERIAL=', ib.scan_data) - 10)
			END AS serial,

			CASE WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 
					  AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data,
					CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) + 17,
					CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) - 17)
			END AS carton_serial,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(7)BATCH=', ib.scan_data) > 0 
					  AND CHARINDEX('(8)', ib.scan_data) > 0 THEN
				TRIM(SUBSTRING(ib.scan_data,
					CHARINDEX('(7)BATCH=', ib.scan_data) + 9,
					CHARINDEX('(8)', ib.scan_data) - CHARINDEX('(7)BATCH=', ib.scan_data) - 9))
			END AS batch,

			CASE WHEN ib.scan_data IS NOT NULL 
					  AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
				TRIM(CASE
					WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
						SUBSTRING(ib.scan_data,
							CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12,
							CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
					ELSE
						SUBSTRING(ib.scan_data,
							CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12,
							LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
				END)
			END AS mfg_date_raw,

			TRY_CONVERT(DATE,
				CASE WHEN ib.scan_data IS NOT NULL 
						  AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
					TRIM(CASE
						WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
							SUBSTRING(ib.scan_data,
								CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12,
								CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
						ELSE
							SUBSTRING(ib.scan_data,
								CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12,
								LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
					END)
				END
			, 112) AS mfg_date,

			CASE WHEN CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) > 0 THEN
				TRY_CAST(
					TRIM(SUBSTRING(ib.scan_data,
						CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) + 18,
						LEN(ib.scan_data) - CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) - 17))
				AS INT)
			END AS qty_per_carton

		FROM inbound_barcodes ib
		WHERE ib.id = ?
	`

	var result ScanDataResult
	if err := r.db.Raw(query, inboundBarcodeID).Scan(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *InboundRepository) GetScanDataBatch(barcodeIDs []uint) (map[uint]ScanDataResult, error) {
	resultMap := make(map[uint]ScanDataResult)
	if len(barcodeIDs) == 0 {
		return resultMap, nil
	}

	query := `
		SELECT
			ib.id AS inbound_barcode_id,
			ib.inbound_id,
			ib.inbound_detail_id,
			ib.lot_number,
			ib.scan_data AS raw_scan_data,
			CASE 
				WHEN ib.scan_data IS NULL OR LEN(TRIM(ib.scan_data)) = 0 THEN 'INVALID - EMPTY'
				WHEN CHARINDEX('(1)SKU=', ib.scan_data) = 0 THEN 'INVALID - UNKNOWN FORMAT'
				WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 THEN 'CARTON'
				ELSE 'UNIT'
			END AS item_type,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(1)SKU=', ib.scan_data) > 0 AND CHARINDEX('(2)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(1)SKU=', ib.scan_data) + 7, CHARINDEX('(2)', ib.scan_data) - CHARINDEX('(1)SKU=', ib.scan_data) - 7)
			END AS sku,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(2)EAN=', ib.scan_data) > 0 AND CHARINDEX('(3)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(2)EAN=', ib.scan_data) + 7, CHARINDEX('(3)', ib.scan_data) - CHARINDEX('(2)EAN=', ib.scan_data) - 7)
			END AS ean,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(3)PRODUCT=', ib.scan_data) > 0 AND CHARINDEX('(4)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(3)PRODUCT=', ib.scan_data) + 11, CHARINDEX('(4)', ib.scan_data) - CHARINDEX('(3)PRODUCT=', ib.scan_data) - 11)
			END AS product,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(4)BRAND=', ib.scan_data) > 0 AND CHARINDEX('(5)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(4)BRAND=', ib.scan_data) + 9, CHARINDEX('(5)', ib.scan_data) - CHARINDEX('(4)BRAND=', ib.scan_data) - 9)
			END AS brand,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(5)MODEL=', ib.scan_data) > 0 AND CHARINDEX('(6)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(5)MODEL=', ib.scan_data) + 9, CHARINDEX('(6)', ib.scan_data) - CHARINDEX('(5)MODEL=', ib.scan_data) - 9)
			END AS model,
			CASE WHEN CHARINDEX('(6)SERIAL=', ib.scan_data) > 0 AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(6)SERIAL=', ib.scan_data) + 10, CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)SERIAL=', ib.scan_data) - 10)
			END AS serial,
			CASE WHEN CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) > 0 AND CHARINDEX('(7)', ib.scan_data) > 0 THEN
				SUBSTRING(ib.scan_data, CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) + 17, CHARINDEX('(7)', ib.scan_data) - CHARINDEX('(6)CARTON_SERIAL=', ib.scan_data) - 17)
			END AS carton_serial,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(7)BATCH=', ib.scan_data) > 0 AND CHARINDEX('(8)', ib.scan_data) > 0 THEN
				TRIM(SUBSTRING(ib.scan_data, CHARINDEX('(7)BATCH=', ib.scan_data) + 9, CHARINDEX('(8)', ib.scan_data) - CHARINDEX('(7)BATCH=', ib.scan_data) - 9))
			END AS batch,
			CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
				TRIM(CASE WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
					SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
				ELSE
					SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
				END)
			END AS mfg_date_raw,
			TRY_CONVERT(DATE,
				CASE WHEN ib.scan_data IS NOT NULL AND CHARINDEX('(8)MFG_DATE=', ib.scan_data) > 0 THEN
					TRIM(CASE WHEN CHARINDEX('(9)', ib.scan_data) > 0 THEN
						SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, CHARINDEX('(9)', ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 12)
					ELSE
						SUBSTRING(ib.scan_data, CHARINDEX('(8)MFG_DATE=', ib.scan_data) + 12, LEN(ib.scan_data) - CHARINDEX('(8)MFG_DATE=', ib.scan_data) - 11)
					END)
				END
			, 112) AS mfg_date,
			CASE WHEN CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) > 0 THEN
				TRY_CAST(TRIM(SUBSTRING(ib.scan_data, CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) + 18, LEN(ib.scan_data) - CHARINDEX('(9)QTY_PER_CARTON=', ib.scan_data) - 17)) AS INT)
			END AS qty_per_carton
		FROM inbound_barcodes ib
		WHERE ib.id IN ?
	`

	for _, chunk := range helpers.ChunkSlice(barcodeIDs, helpers.ChunkSizeIN) {
		var part []ScanDataResult
		if err := r.db.Raw(query, chunk).Scan(&part).Error; err != nil {
			return nil, err
		}
		for _, res := range part {
			resultMap[res.InboundBarcodeID] = res
		}
	}

	return resultMap, nil
}

type InboundFilterParams struct {
	StartDate  string
	EndDate    string
	Search     string
	SearchItem string
	Statuses   []string
	Types      []string
	Owners     []string
}

func (r *InboundRepository) GetInboundListWithFilter(params InboundFilterParams) ([]ListInbound, error) {
	var listInbound []ListInbound
	var args []interface{}

	// Date range
	startDate := "DATEADD(day, -7, CAST(GETDATE() AS DATE))"
	endDate := "CAST(GETDATE() AS DATE)"
	if params.StartDate != "" {
		startDate = "?"
		args = append(args, params.StartDate)
	}
	if params.EndDate != "" {
		endDate = "?"
		args = append(args, params.EndDate)
	}

	// Status filter
	statusWhere := ""
	if len(params.Statuses) > 0 {
		placeholders := make([]string, len(params.Statuses))
		for i, s := range params.Statuses {
			placeholders[i] = "?"
			args = append(args, s)
		}
		statusWhere = "AND a.status IN (" + strings.Join(placeholders, ", ") + ")"
	}

	// Type filter (IB Type)
	typeWhere := ""
	if len(params.Types) > 0 {
		placeholders := make([]string, len(params.Types))
		for i, s := range params.Types {
			placeholders[i] = "?"
			args = append(args, s)
		}
		typeWhere = "AND a.type IN (" + strings.Join(placeholders, ", ") + ")"
	}

	// Owner filter
	ownerWhere := ""
	if len(params.Owners) > 0 {
		placeholders := make([]string, len(params.Owners))
		for i, s := range params.Owners {
			placeholders[i] = "?"
			args = append(args, s)
		}
		ownerWhere = "AND a.owner_code IN (" + strings.Join(placeholders, ", ") + ")"
	}

	// Header search (di base CTE) — inbound_no, receipt_id
	baseSearch := ""
	if params.Search != "" {
		baseSearch = "AND (a.inbound_no LIKE ? OR a.receipt_id LIKE ?)"
		like := "%" + params.Search + "%"
		args = append(args, like, like)
	}

	// Item search (join + where)
	itemJoin := ""
	itemWhere := ""
	if params.SearchItem != "" {
		itemJoin = `
        INNER JOIN inbound_details id_s ON a.id = id_s.inbound_id
        INNER JOIN products p_s ON id_s.item_code = p_s.item_code`
		itemWhere = "AND (id_s.item_code LIKE ? OR p_s.item_name LIKE ?)"
		like := "%" + params.SearchItem + "%"
		args = append(args, like, like)
	}

	// Outer search (supplier_name, transporter_name) — setelah join
	outerSearch := ""
	if params.Search != "" {
		outerSearch = "OR c.supplier_name LIKE ? OR d.transporter_name LIKE ?"
		like := "%" + params.Search + "%"
		args = append(args, like, like)
	}

	query := `
    WITH base AS (
        SELECT id, inbound_no, receipt_id, owner_code,
               driver, truck_id, no_truck, inbound_date,
               container, origin, arrival_time, start_unloading, end_unloading,
               status, remarks, type, supplier, transporter
        FROM inbound_headers a
        WHERE 1=1
          AND inbound_date >= ` + startDate + `
          AND inbound_date <= ` + endDate + `
          ` + statusWhere + `
          ` + typeWhere + `
          ` + ownerWhere + `
          ` + baseSearch + `
    ),
    detail AS (
        SELECT inbound_id, COUNT(item_code) as total_line, SUM(quantity) total_qty
        FROM inbound_details
        WHERE inbound_id IN (SELECT id FROM base)
        GROUP BY inbound_id
    ),
    inbound_barcode AS (
        SELECT inbound_id, SUM(quantity) as qty_scan
        FROM inbound_barcodes
        WHERE inbound_id IN (SELECT id FROM base)
        GROUP BY inbound_id
    ),
    inbound_putaway AS (
        SELECT inbound_id, SUM(quantity) as qty_scan
        FROM inbound_barcodes
        WHERE status = 'in stock'
          AND inbound_id IN (SELECT id FROM base)
        GROUP BY inbound_id
    )
    SELECT a.id, a.inbound_no, a.receipt_id,
           c.supplier_name, a.owner_code,
           a.driver, a.truck_id, a.no_truck, a.inbound_date,
           a.container,
           a.origin, a.arrival_time, a.start_unloading, a.end_unloading,
           a.status, a.remarks as remarks_header,
           b.total_line, b.total_qty, COALESCE(ib.qty_scan, 0) as qty_scan, COALESCE(ipu.qty_scan, 0) as qty_putaway,
           d.transporter_name, a.type
    FROM base a
    LEFT JOIN detail b ON a.id = b.inbound_id
    LEFT JOIN suppliers c ON a.supplier = c.supplier_code
    LEFT JOIN transporters d ON a.transporter = d.transporter_code
    LEFT JOIN inbound_barcode ib ON a.id = ib.inbound_id
    LEFT JOIN inbound_putaway ipu ON a.id = ipu.inbound_id
    ` + itemJoin + `
    WHERE (1=1 ` + outerSearch + `)
    ` + itemWhere + `
    ORDER BY a.id DESC`

	if err := r.db.Raw(query, args...).Scan(&listInbound).Error; err != nil {
		return nil, err
	}

	for i, inbound := range listInbound {
		var inboundReferences []models.InboundReference
		if err := r.db.Where("inbound_id = ?", inbound.ID).Find(&inboundReferences).Error; err != nil {
			return nil, err
		}
		var refNos []string
		for _, ref := range inboundReferences {
			refNos = append(refNos, ref.RefNo)
		}
		listInbound[i].Invoice = strings.Join(refNos, ", ")
	}

	return listInbound, nil
}
