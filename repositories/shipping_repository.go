package repositories

import (
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
			COALESCE(kd.qty_pack, 0) AS total_qty,
            cs.customer_name,
			a.deliv_to,
			cd.customer_name as deliv_to_name,
			cd.cust_addr1 as deliv_address,
			cd.cust_city as deliv_city,
			a.qty_koli,
			od.total_cbm * od.qty_req as total_cbm,
			od.total_item,
			ps.qty_plan as total_qty,
			odt.outbound_id as odt_id
            from outbound_headers a
            left join od on a.id = od.outbound_id
            LEFT JOIN ps ON a.id = ps.outbound_id
            LEFT JOIN kd ON a.id = kd.outbound_id
            LEFT JOIN customers cs ON a.customer_code = cs.customer_code
			LEFT JOIN customers cd ON a.deliv_to = cd.customer_code
			LEFT JOIN order_details odt ON a.id = odt.outbound_id
			WHERE a.status <> 'open'
			AND odt.outbound_id IS NULL
			order by a.id desc`

	if err := r.db.Raw(sql).Scan(&outboundList).Error; err != nil {
		return nil, err
	}

	return outboundList, nil
}

type OrderList struct {
	ID              int     `json:"ID"`
	OrderNo         string  `json:"order_no"`
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
	SELECT a.order_id, count(shipment_id) as total_do, sum(qty_koli) as total_koli,
	sum(total_item) as total_item, sum(total_cbm) as total_cbm, SUM(op.quantity) as total_qty
	FROM order_details a
	LEFT JOIN outbound_pickings op on a.outbound_id = op.outbound_id 
	GROUP BY
	a.order_id
), dlv AS (
	SELECT a.order_id, count( distinct a.deliv_to) as total_drop
	FROM order_details a
	GROUP BY
	a.order_id
	-- select * from order_details where order_id = 14
)

SELECT oh.id,
oh.order_no,
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
obh.total_cbm,
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
