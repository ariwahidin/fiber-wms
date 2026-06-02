package repositories

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type ShippingRepository struct {
	db *gorm.DB
}

func NewShippingRepository(db *gorm.DB) *ShippingRepository {
	return &ShippingRepository{db: db}
}

// type OutboundList struct {
// 	ID           uint    `json:"ID"`
// 	OutboundNo   string  `json:"outbound_no"`
// 	ShipmentID   string  `json:"shipment_id"`
// 	OwnerCode    string  `json:"owner_code"`
// 	OutboundDate string  `json:"outbound_date"`
// 	CustomerCode string  `json:"customer_code"`
// 	CustomerName string  `json:"customer_name"`
// 	TotalItem    int     `json:"total_item"`
// 	QtyReq       int     `json:"qty_req"`
// 	QtyPlan      int     `json:"qty_plan"`
// 	QtyPack      int     `json:"qty_pack"`
// 	Status       string  `json:"status"`
// 	TotalPrice   int     `json:"total_price"`
// 	DelivTo      string  `json:"deliv_to"`
// 	DelivToName  string  `json:"deliv_to_name"`
// 	DelivAddress string  `json:"deliv_address"`
// 	DelivCity    string  `json:"deliv_city"`
// 	QtyKoli      int     `json:"qty_koli"`
// 	TotalCBM     float64 `json:"total_cbm"`
// }

func (r *ShippingRepository) GetAllOutboundList() ([]OutboundList, error) {
	var outboundList []OutboundList

	sql := `WITH od AS 
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
		SELECT outbound_id, SUM(quantity) AS qty_pack, count( distinct pack_ctn_no) as total_ctn
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
			COALESCE(kd.qty_pack, 0) AS total_qty,
            cs.customer_name,
			a.deliv_to,
			cd.customer_name as deliv_to_name,
			cd.cust_addr1 as deliv_address,
			cd.cust_city as deliv_city,
			COALESCE(kd.total_ctn, 0) as qty_koli,
			ROUND(od.total_cbm, 4) as total_cbm,
			od.total_item,
			ps.qty_plan as total_qty,
			odt.outbound_id as odt_id,
			inpo.use_vas, 
			a.order_type
            from outbound_headers a
            left join od on a.id = od.outbound_id
            LEFT JOIN ps ON a.id = ps.outbound_id
            LEFT JOIN kd ON a.id = kd.outbound_id
            LEFT JOIN customers cs ON a.customer_code = cs.customer_code
			LEFT JOIN customers cd ON a.deliv_to = cd.customer_code
			LEFT JOIN order_details odt ON a.id = odt.outbound_id
			LEFT JOIN inventory_policies inpo ON a.owner_code = inpo.owner_code
			WHERE a.status = 'complete'
			AND odt.outbound_id IS NULL
			order by a.id desc;`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	if len(outboundList) == 0 {
		return []OutboundList{}, nil
	}

	return outboundList, nil
}

type OrderList struct {
	ID              int     `json:"ID"`
	OrderNo         string  `json:"order_no"`
	Status          string  `json:"status"`
	OrderDate       string  `json:"order_date"`
	OrderType       string  `json:"order_type"`
	Driver          string  `json:"driver"`
	TruckNo         string  `json:"truck_no"`
	TruckSize       string  `json:"truck_size"`
	TransporterName string  `json:"transporter_name"`
	TotalDO         int     `json:"total_do"`
	TotalKoli       int     `json:"total_koli"`
	TotalItem       int     `json:"total_item"`
	TotalQty        int     `json:"total_qty"`
	TotalCBM        float64 `json:"total_cbm"`
	TotalDrop       int     `json:"total_drop"`
}

func (r *ShippingRepository) GetOrderSummaryList() ([]OrderList, error) {
	var orderList []OrderList
	sql := `WITH obh AS
(
		SELECT a.order_id, 
	count(a.shipment_id) as total_do, 
	sum(a.qty_koli) as total_koli,
	sum(a.total_item) as total_item, 
	sum(a.total_cbm) as total_cbm, 
	sum(op.quantity) as total_qty
	FROM order_details a
	LEFT JOIN (select outbound_id, sum(quantity) as quantity from outbound_pickings group by outbound_id) op 
	on a.outbound_id = op.outbound_id 
	GROUP BY a.order_id
), dlv AS (
	SELECT a.order_id, count( distinct a.deliv_to) as total_drop
	FROM order_details a
	GROUP BY
	a.order_id
	-- select * from order_details where order_id = 14
)

SELECT oh.id,
oh.order_no,
oh.status,
oh.order_date,
oh.order_type,
oh.driver,
oh.truck_no,
oh.truck_size,
oh.transporter_name,
obh.total_do,
obh.total_koli,
obh.total_item,
obh.total_qty,
ROUND(obh.total_cbm, 4) as total_cbm,
dlv.total_drop
FROM order_headers oh
LEFT JOIN obh ON oh.id = obh.order_id
LEFT JOIN dlv ON oh.id = dlv.order_id
order by oh.order_no DESC`

	if err := r.db.Raw(sql).Scan(&orderList).Error; err != nil {
		return nil, err
	}

	return orderList, nil
}

type OrderDetailItem struct {
	OutboundID  int     `json:"outbound_id"`
	OutboundNo  string  `json:"outbound_no"`
	DelivTo     string  `json:"deliv_to"`
	DelivToName string  `json:"deliv_to_name"`
	DelivCity   string  `json:"deliv_city"`
	ShipmentID  string  `json:"shipment_id"`
	TotalKoli   int     `json:"total_koli"`
	Remarks     string  `json:"remarks"`
	ItemCode    string  `json:"item_code"`
	Quantity    int     `json:"quantity"`
	CBM         float64 `json:"cbm"`
	TotalCBM    float64 `json:"total_cbm"`
}

func (r *ShippingRepository) GetOrderDetailItem(outboundID int) ([]OrderDetailItem, error) {
	var orderDetailItem []OrderDetailItem
	sql := `SELECT od.outbound_id,
	od.outbound_no,
	od.deliv_to,
	od.deliv_to_name,
	od.deliv_city,
	od.shipment_id,
	od.qty_koli as total_koli,
	od.remarks,
	odt.item_code,
	odt.quantity,
	p.cbm,
	ROUND(p.cbm * odt.quantity, 4) as total_cbm
	-- p.cbm * odt.quantity as total_cbm
	FROM order_details od
	INNER JOIN outbound_details odt ON od.outbound_id = odt.outbound_id
	LEFT JOIN products p ON odt.item_id = p.id
	WHERE order_id = ?`

	if err := r.db.Raw(sql, outboundID).Scan(&orderDetailItem).Error; err != nil {
		return nil, err
	}

	if len(orderDetailItem) == 0 {
		return []OrderDetailItem{}, nil
	}

	return orderDetailItem, nil
}

func (r *ShippingRepository) CalculatVasOutbound(outboundID int) ([]VasCalculate, error) {
	var result []VasCalculate

	sql := `WITH vas_sum AS
	(SELECT v.id as vas_id, v.name as vas_name,
	vd.main_vas_id, mv.name as main_vas_name, mv.default_price, mv.is_koli
	FROM vas v
	INNER JOIN vas_detail vd ON v.id = vd.vas_id
	INNER JOIN main_vas mv ON mv.id = vd.main_vas_id),
	vas_ob_item AS (
		SELECT 
		od.id as outbound_detail_id,
		od.outbound_id,
		od.outbound_no,
		oh.outbound_date,
		od.item_id,
		od.item_code,
		od.barcode,
		od.quantity as qty_item,
		ordt.vas_koli,
		od.vas_id ob_vas_id,
		od.vas_name ob_vas_name,
		vs.main_vas_id,
		vs.main_vas_name,
		vs.default_price,
		vs.is_koli,
		CASE WHEN vs.is_koli = 0 THEN od.quantity * vs.default_price ELSE ordt.vas_koli * vs.default_price END AS total_price
		FROM
		outbound_details od
		inner join outbound_headers oh ON od.outbound_id = oh.id
		inner join vas_sum vs ON od.vas_id = vs.vas_id
		inner join order_details ordt ON oh.id = ordt.outbound_id
		WHERE od.outbound_id = ?
		),
	vas_ob_sum AS( 
		select
		vb.outbound_id, vb.outbound_no, vb.outbound_date,
		vb.main_vas_name, vb.is_koli, vb.default_price,
		sum(vb.qty_item) as qty_item, 
		vb.vas_koli
		from 
		vas_ob_item vb
		where vb.is_koli = 1
		GROUP BY 
		vb.outbound_id,
		vb.outbound_no,
		vb.outbound_date,
		vb.is_koli,
		vb.main_vas_name,
		vb.default_price,
		vb.vas_koli
		UNION ALL
		select
		vb.outbound_id, vb.outbound_no, vb.outbound_date,
		vb.main_vas_name, vb.is_koli, vb.default_price,vb.qty_item, vb.vas_koli
		from 
		vas_ob_item vb
		where vb.is_koli = 0)
	SELECT
	vos.outbound_id,
	vos.outbound_no,
	vos.outbound_date,
	vos.main_vas_name,
	vos.is_koli,
	vos.default_price,
	vos.qty_item,
	vos.vas_koli,
	CASE WHEN vos.is_koli = 1 THEN vos.default_price * vas_koli ELSE vos.default_price * vos.qty_item END AS total_price
	FROM vas_ob_sum vos`

	if err := r.db.Debug().Raw(sql, outboundID).Scan(&result).Error; err != nil {
		return result, err
	}

	if len(result) == 0 {
		result = []VasCalculate{}
	}

	return result, nil
}

// OrderListFilter — sama persis dengan OrderList yang sudah ada,
// ditambah field ShipmentIDs agar frontend bisa tahu DO mana yang match.
type OrderListFilter struct {
	ID              int     `json:"ID"`
	OrderNo         string  `json:"order_no"`
	Status          string  `json:"status"`
	OrderDate       string  `json:"order_date"`
	OrderType       string  `json:"order_type"`
	Driver          string  `json:"driver"`
	TruckNo         string  `json:"truck_no"`
	TruckSize       string  `json:"truck_size"`
	TransporterName string  `json:"transporter_name"`
	TotalDO         int     `json:"total_do"`
	TotalKoli       int     `json:"total_koli"`
	TotalItem       int     `json:"total_item"`
	TotalQty        int     `json:"total_qty"`
	TotalCBM        float64 `json:"total_cbm"`
	TotalDrop       int     `json:"total_drop"`
	// Comma-separated list of shipment_id yang match search_do (kosong jika tidak search)
	MatchedDOs string `json:"matched_dos,omitempty"`
}

func (r *ShippingRepository) GetOrderSummaryListFilter(
	startDate, endDate, search, searchDO string,
	statuses []string,
) ([]OrderListFilter, error) {
	var orderList []OrderListFilter

	// ── Bangun klausa WHERE dinamis ───────────────────────────────────────────
	// Kita kumpulkan kondisi dan args terpisah agar SQL tetap bersih
	// dan aman dari SQL injection (pakai placeholder ?).

	whereClauses := []string{
		"oh.deleted_at IS NULL",
		"CAST(oh.order_date AS DATE) BETWEEN ? AND ?",
	}
	args := []interface{}{startDate, endDate}

	// Filter status (multi)
	if len(statuses) > 0 {
		// Bangun IN (?, ?, ...)
		placeholders := make([]string, len(statuses))
		for i, s := range statuses {
			placeholders[i] = "?"
			args = append(args, s)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("oh.status IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter search header
	if search != "" {
		like := "%" + search + "%"
		whereClauses = append(whereClauses,
			"(oh.order_no LIKE ? OR oh.transporter_name LIKE ? OR oh.truck_no LIKE ? OR oh.driver LIKE ?)")
		args = append(args, like, like, like, like)
	}

	// Filter search_do — EXISTS subquery ke order_details
	// shipment_id di order_details = nomor DO
	if searchDO != "" {
		like := "%" + searchDO + "%"
		whereClauses = append(whereClauses, `EXISTS (
			SELECT 1
			FROM order_details od_do
			WHERE od_do.order_id   = oh.id
			  AND od_do.deleted_at IS NULL
			  AND od_do.shipment_id LIKE ?
		)`)
		args = append(args, like)
	}

	whereSQL := strings.Join(whereClauses, "\n    AND ")

	// ── SELECT matched_dos ────────────────────────────────────────────────────
	// Jika searchDO aktif: tampilkan shipment_id yang match di field matched_dos
	// agar frontend bisa render info "DO ini ditemukan di SPK ini".
	// Jika tidak aktif: kolom tetap ada tapi kosong (GROUP_CONCAT return NULL → "").
	matchedDOsSelect := ""
	var matchedArgs []interface{}
	if searchDO != "" {
		like := "%" + searchDO + "%"
		matchedDOsSelect = `,
	(
		SELECT GROUP_CONCAT(DISTINCT od_m.shipment_id ORDER BY od_m.shipment_id SEPARATOR ', ')
		FROM order_details od_m
		WHERE od_m.order_id    = oh.id
		  AND od_m.deleted_at  IS NULL
		  AND od_m.shipment_id LIKE ?
	) AS matched_dos`
		matchedArgs = append(matchedArgs, like)
	} else {
		matchedDOsSelect = `,'' AS matched_dos`
	}

	// ── Full SQL ──────────────────────────────────────────────────────────────
	// Ikuti pola GetOrderSummaryList yang sudah ada:
	//   CTE obh  → aggregasi per order dari order_details + outbound_pickings
	//   CTE dlv  → count distinct deliv_to (total drop point)
	// Tambahan: WHERE dinamis + matched_dos

	sql := fmt.Sprintf(`
WITH obh AS (
	SELECT
		a.order_id,
		COUNT(a.shipment_id)               AS total_do,
		SUM(a.qty_koli)                    AS total_koli,
		SUM(a.total_item)                  AS total_item,
		ROUND(SUM(a.total_cbm), 4)         AS total_cbm,
		SUM(op.quantity)                   AS total_qty
	FROM order_details a
	LEFT JOIN (
		SELECT outbound_id, SUM(quantity) AS quantity
		FROM outbound_pickings
		GROUP BY outbound_id
	) op ON a.outbound_id = op.outbound_id
	WHERE a.deleted_at IS NULL
	GROUP BY a.order_id
),
dlv AS (
	SELECT
		a.order_id,
		COUNT(DISTINCT a.deliv_to) AS total_drop
	FROM order_details a
	WHERE a.deleted_at IS NULL
	GROUP BY a.order_id
)
 
SELECT
	oh.id,
	oh.order_no,
	oh.status,
	oh.order_date,
	oh.order_type,
	oh.driver,
	oh.truck_no,
	oh.truck_size,
	oh.transporter_name,
	COALESCE(obh.total_do,   0)    AS total_do,
	COALESCE(obh.total_koli, 0)    AS total_koli,
	COALESCE(obh.total_item, 0)    AS total_item,
	COALESCE(obh.total_qty,  0)    AS total_qty,
	COALESCE(obh.total_cbm,  0)    AS total_cbm,
	COALESCE(dlv.total_drop, 0)    AS total_drop
	%s
FROM order_headers oh
LEFT JOIN obh ON oh.id = obh.order_id
LEFT JOIN dlv ON oh.id = dlv.order_id
WHERE %s
ORDER BY oh.order_no DESC`,
		matchedDOsSelect,
		whereSQL,
	)

	// ── Gabungkan args: matchedArgs (subquery SELECT) dulu, lalu WHERE args ──
	// Urutan harus sesuai dengan posisi ? dalam SQL:
	// 1. ? di matched_dos subquery (SELECT clause) — sebelum WHERE
	// 2. ? di WHERE clause
	finalArgs := append(matchedArgs, args...)

	if err := r.db.Raw(sql, finalArgs...).Scan(&orderList).Error; err != nil {
		return nil, err
	}

	if orderList == nil {
		orderList = []OrderListFilter{}
	}

	return orderList, nil
}
