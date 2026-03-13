package controllers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DashboardController struct {
	DB *gorm.DB
}

func NewDashboardController(db *gorm.DB) *DashboardController {
	return &DashboardController{DB: db}
}

// parseFilter mengambil query params dan return kondisi WHERE + args
// Params: date_from, date_to, owner_code
// Semua opsional — jika tidak ada, tidak ada filter
func parseDashboardFilter(ctx *fiber.Ctx) (dateFrom, dateTo, ownerCode string) {
	dateFrom = ctx.Query("date_from")   // format: "2006-01-02"
	dateTo = ctx.Query("date_to")       // format: "2006-01-02"
	ownerCode = ctx.Query("owner_code") // "" = All
	return
}

// buildDateCondition return SQL condition untuk kolom tanggal (SQL Server)
// Jika dateFrom & dateTo kosong, return "" (no filter)
func buildDateCondition(col, dateFrom, dateTo string) string {
	if dateFrom != "" && dateTo != "" {
		return "AND CAST(" + col + " AS DATE) BETWEEN '" + dateFrom + "' AND '" + dateTo + "'"
	}
	if dateFrom != "" {
		return "AND CAST(" + col + " AS DATE) >= '" + dateFrom + "'"
	}
	if dateTo != "" {
		return "AND CAST(" + col + " AS DATE) <= '" + dateTo + "'"
	}
	return ""
}

func buildOwnerCondition(col, ownerCode string) string {
	if ownerCode != "" && ownerCode != "all" {
		return "AND " + col + " = '" + ownerCode + "'"
	}
	return ""
}

// ── GET /dashboard ───────────────────────────────────────────
func (c *DashboardController) GetDashboard(ctx *fiber.Ctx) error {
	dateFrom, dateTo, ownerCode := parseDashboardFilter(ctx)

	ibDateCond := buildDateCondition("ih.inbound_date", dateFrom, dateTo)
	obDateCond := buildDateCondition("oh.outbound_date", dateFrom, dateTo)
	ibOwnerCond := buildOwnerCondition("ih.owner_code", ownerCode)
	obOwnerCond := buildOwnerCondition("oh.owner_code", ownerCode)

	sql := `
		WITH ib AS (
			SELECT ih.id, ih.inbound_no AS no_ref, ih.receipt_id AS reference_no,
			       ih.status, ih.inbound_date AS trans_date, id.tot_item, id.tot_qty
			FROM inbound_headers ih
			INNER JOIN (
				SELECT inbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty
				FROM inbound_details GROUP BY inbound_id
			) id ON ih.id = id.inbound_id
			WHERE ih.status <> 'complete'
			` + ibDateCond + `
			` + ibOwnerCond + `
		),
		ob AS (
			SELECT oh.id, oh.outbound_no AS no_ref, oh.shipment_id AS reference_no,
			       oh.status, oh.outbound_date AS trans_date, od.tot_item, od.tot_qty
			FROM outbound_headers oh
			INNER JOIN (
				SELECT outbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty
				FROM outbound_details GROUP BY outbound_id
			) od ON oh.id = od.outbound_id
			WHERE oh.status <> 'complete'
			` + obDateCond + `
			` + obOwnerCond + `
		)
		SELECT *, 'inbound' AS trans_type FROM ib
		UNION ALL
		SELECT *, 'outbound' AS trans_type FROM ob
		ORDER BY trans_type, no_ref DESC
	`

	var transactions []struct {
		ID          uint   `json:"id"`
		NoRef       string `json:"no_ref"`
		ReferenceNo string `json:"reference_no"`
		Status      string `json:"status"`
		TransDate   string `json:"trans_date"`
		TotItem     int    `json:"tot_item"`
		TotQty      int    `json:"tot_qty"`
		TransType   string `json:"trans_type"`
	}

	if err := c.DB.Raw(sql).Scan(&transactions).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(transactions) == 0 {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "Dashboard found",
			"data":    fiber.Map{"transactions": []interface{}{}},
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Dashboard found",
		"data":    fiber.Map{"transactions": transactions},
	})
}

// ── GET /dashboard/chart ─────────────────────────────────────
func (c *DashboardController) GetDashboardChart(ctx *fiber.Ctx) error {
	dateFrom, dateTo, ownerCode := parseDashboardFilter(ctx)

	// Untuk trend, ada param khusus: period (7, 14, 30)
	// Default 7 hari jika tidak ada date_from/date_to eksplisit
	period := ctx.QueryInt("period", 7)
	if period <= 0 {
		period = 7
	}

	obOwnerCond := buildOwnerCondition("oh.owner_code", ownerCode)
	ibOwnerCond := buildOwnerCondition("ih.owner_code", ownerCode)

	// ── 1. Trend ─────────────────────────────────────────────
	// Jika date_from & date_to ada, gunakan range itu
	// Jika tidak, gunakan period (N hari terakhir)
	var trendSQL string
	if dateFrom != "" && dateTo != "" {
		// Generate date series dari date_from s/d date_to menggunakan recursive CTE
		trendSQL = `
			WITH dates AS (
				SELECT CAST('` + dateFrom + `' AS DATE) AS d
				UNION ALL
				SELECT DATEADD(DAY, 1, d) FROM dates
				WHERE d < CAST('` + dateTo + `' AS DATE)
			),
			ib AS (
				SELECT CAST(inbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
				FROM inbound_headers ih
				WHERE deleted_at IS NULL
				AND CAST(inbound_date AS DATE) BETWEEN '` + dateFrom + `' AND '` + dateTo + `'
				` + ibOwnerCond + `
				GROUP BY CAST(inbound_date AS DATE)
			),
			ob AS (
				SELECT CAST(outbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
				FROM outbound_headers oh
				WHERE deleted_at IS NULL
				AND CAST(outbound_date AS DATE) BETWEEN '` + dateFrom + `' AND '` + dateTo + `'
				` + obOwnerCond + `
				GROUP BY CAST(outbound_date AS DATE)
			)
			SELECT
				CONVERT(VARCHAR(10), dates.d, 23) AS date,
				COALESCE(ib.cnt, 0)               AS inbound,
				COALESCE(ob.cnt, 0)               AS outbound
			FROM dates
			LEFT JOIN ib ON dates.d = ib.trans_date
			LEFT JOIN ob ON dates.d = ob.trans_date
			ORDER BY dates.d ASC
			OPTION (MAXRECURSION 366)
		`
	} else {
		// Default: N hari terakhir
		periodStr := time.Now().AddDate(0, 0, -(period - 1)).Format("2006-01-02")
		trendSQL = `
			WITH dates AS (
				SELECT CAST(DATEADD(DAY, -seq, CAST(GETDATE() AS DATE)) AS DATE) AS d
				FROM (
					SELECT TOP ` + fmt.Sprintf("%d", period) + ` ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) - 1 AS seq
					FROM sys.objects
				) t
			),
			ib AS (
				SELECT CAST(inbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
				FROM inbound_headers ih
				WHERE deleted_at IS NULL
				AND CAST(inbound_date AS DATE) >= '` + periodStr + `'
				` + ibOwnerCond + `
				GROUP BY CAST(inbound_date AS DATE)
			),
			ob AS (
				SELECT CAST(outbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
				FROM outbound_headers oh
				WHERE deleted_at IS NULL
				AND CAST(outbound_date AS DATE) >= '` + periodStr + `'
				` + obOwnerCond + `
				GROUP BY CAST(outbound_date AS DATE)
			)
			SELECT
				CONVERT(VARCHAR(10), dates.d, 23) AS date,
				COALESCE(ib.cnt, 0)               AS inbound,
				COALESCE(ob.cnt, 0)               AS outbound
			FROM dates
			LEFT JOIN ib ON dates.d = ib.trans_date
			LEFT JOIN ob ON dates.d = ob.trans_date
			ORDER BY dates.d ASC
		`
	}

	var trend []struct {
		Date     string `json:"date"`
		Inbound  int    `json:"inbound"`
		Outbound int    `json:"outbound"`
	}

	if err := c.DB.Raw(trendSQL).Scan(&trend).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "trend query failed: " + err.Error(),
		})
	}

	// ── 2. Type Mix ───────────────────────────────────────────
	obDateCond := buildDateCondition("outbound_date", dateFrom, dateTo)

	typeMixSQL := `
		SELECT
			CASE
				WHEN order_type IS NULL OR order_type = '' THEN 'Other'
				ELSE order_type
			END AS name,
			COUNT(*) AS value
		FROM outbound_headers oh
		WHERE deleted_at IS NULL
		` + obDateCond + `
		` + obOwnerCond + `
		GROUP BY
			CASE
				WHEN order_type IS NULL OR order_type = '' THEN 'Other'
				ELSE order_type
			END
		ORDER BY value DESC
	`

	var typeMix []struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	if err := c.DB.Raw(typeMixSQL).Scan(&typeMix).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "type_mix query failed: " + err.Error(),
		})
	}

	// ── 3. Total ──────────────────────────────────────────────
	totalSQL := `
		SELECT
			COUNT(*)        AS total_order,
			SUM(od.tot_qty) AS total_qty
		FROM outbound_headers oh
		INNER JOIN (
			SELECT outbound_id, SUM(quantity) AS tot_qty
			FROM outbound_details
			WHERE deleted_at IS NULL
			GROUP BY outbound_id
		) od ON oh.id = od.outbound_id
		WHERE oh.deleted_at IS NULL
		` + obDateCond + `
		` + obOwnerCond + `
	`

	var total struct {
		TotalOrder int `json:"total_order"`
		TotalQty   int `json:"total_qty"`
	}

	if err := c.DB.Raw(totalSQL).Scan(&total).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "total query failed: " + err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Chart data found",
		"data": fiber.Map{
			"trend":    trend,
			"type_mix": typeMix,
			"total":    total,
		},
	})
}

func (c *DashboardController) GetDashboardPipeline(ctx *fiber.Ctx) error {
	dateFrom, dateTo, ownerCode := parseDashboardFilter(ctx)

	// Kalau tidak ada date range eksplisit, pakai period (default 7 hari)
	period := ctx.QueryInt("period", 7)
	if period <= 0 {
		period = 7
	}

	// Jika tidak ada date filter sama sekali, set date range = N hari terakhir
	// supaya WHERE-nya konsisten dengan chart trend
	resolvedFrom := dateFrom
	resolvedTo := dateTo
	if resolvedFrom == "" && resolvedTo == "" {
		resolvedTo = time.Now().Format("2006-01-02")
		resolvedFrom = time.Now().AddDate(0, 0, -(period - 1)).Format("2006-01-02")
	}

	obDateCond := buildDateCondition("oh.outbound_date", resolvedFrom, resolvedTo)
	obOwnerCond := buildOwnerCondition("oh.owner_code", ownerCode)

	ibDateCond := buildDateCondition("ih.inbound_date", resolvedFrom, resolvedTo)
	ibOwnerCond := buildOwnerCondition("ih.owner_code", ownerCode)

	// ── Outbound pipeline ────────────────────────────────────────────────────
	//
	// [TODO] Sesuaikan:
	//   - nama tabel   : outbound_headers  → ganti jika beda
	//   - kolom status : oh.status         → ganti jika beda
	//   - kolom date   : oh.outbound_date  → sudah di buildDateCondition
	//   - kolom owner  : oh.owner_code     → sudah di buildOwnerCondition
	//
	// SUM(quantity) di bawah adalah DUMMY — ganti dengan join ke detail table
	// yang punya kolom quantity, contoh:
	//   INNER JOIN outbound_details od ON oh.id = od.outbound_id
	//   lalu SUM(od.quantity) AS total_qty
	//
	// Urutan stage di ORDER BY CASE harus kamu sesuaikan dengan nilai status di DB

	outboundSQL := `
WITH OD_SUM AS (
    SELECT 
        outbound_id,
        SUM(quantity) AS quantity
    FROM outbound_details
    GROUP BY outbound_id
),
ORD AS (SELECT b.outbound_id, a.[status] FROM order_headers a inner join order_details b on a.id = b.order_id ),
OHD AS (
    SELECT
        oh.owner_code,
        oh.outbound_date,
        oh.id,
        CASE
            WHEN oh.[status] = 'picking' THEN 'on_picking'
            WHEN oh.[status] = 'packing' THEN 'on_packing'
            WHEN oh.[status] = 'complete' AND (ot.[status] = 'open' OR ot.[status] IS NULL) THEN 'ready_to_ship'
            WHEN oh.[status] = 'complete' AND (ot.[status] = 'loaded') THEN 'shipped'
            ELSE oh.[status]
        END AS status,
        ods.quantity,
        oh.deleted_at
    FROM outbound_headers oh
    LEFT JOIN OD_SUM ods
        ON oh.id = ods.outbound_id
    LEFT JOIN ORD ot
        ON oh.id = ot.outbound_id
)

SELECT
			oh.status                       AS stage_key,
			COUNT(DISTINCT oh.id)           AS total_orders,
			SUM(oh.quantity)                             AS total_qty   -- [TODO] ganti dengan SUM(od.quantity) dari detail table
		FROM OHD oh
		WHERE oh.deleted_at IS NULL
		` + obDateCond + `
		` + obOwnerCond + `
		GROUP BY oh.status
		ORDER BY
			CASE oh.status
				WHEN 'open'     THEN 1
				WHEN 'allocated'     THEN 2
				WHEN 'on_picking'    THEN 3
				WHEN 'on_packing'    THEN 4
				WHEN 'ready_to_ship' THEN 5
				WHEN 'shipped'       THEN 6
				ELSE 99
			END
	`
	// outboundSQL := `
	// 	SELECT
	// 		oh.status                       AS stage_key,
	// 		COUNT(DISTINCT oh.id)           AS total_orders,
	// 		999                             AS total_qty   -- [TODO] ganti dengan SUM(od.quantity) dari detail table
	// 	FROM outbound_headers oh
	// 	WHERE oh.deleted_at IS NULL
	// 	GROUP BY oh.status
	// 	ORDER BY
	// 		CASE oh.status
	// 			WHEN 'confirmed'     THEN 1
	// 			WHEN 'allocated'     THEN 2
	// 			WHEN 'on_picking'    THEN 3
	// 			WHEN 'on_packing'    THEN 4
	// 			WHEN 'ready_to_ship' THEN 5
	// 			WHEN 'shipped'       THEN 6
	// 			ELSE 99
	// 		END
	// `

	// ── Inbound pipeline ─────────────────────────────────────────────────────
	//
	// [TODO] Sesuaikan:
	//   - nama tabel   : inbound_headers   → ganti jika beda
	//   - kolom status : ih.status         → ganti jika beda
	//   - kolom date   : ih.inbound_date   → sudah di buildDateCondition
	//   - kolom owner  : ih.owner_code     → sudah di buildOwnerCondition
	//
	// SUM(quantity) di bawah adalah DUMMY — ganti dengan join ke inbound_details
	// contoh:
	//   INNER JOIN inbound_details id ON ih.id = id.inbound_id
	//   lalu SUM(id.quantity) AS total_qty

	// inboundSQL := `
	// 	SELECT
	// 		ih.status                       AS stage_key,
	// 		COUNT(DISTINCT ih.id)           AS total_orders,
	// 		999                             AS total_qty   -- [TODO] ganti dengan SUM(id.quantity) dari detail table
	// 	FROM inbound_headers ih
	// 	WHERE ih.deleted_at IS NULL
	// 	  AND ih.status NOT IN ('complete', 'cancelled', 'draft')
	// 	` + ibDateCond + `
	// 	` + ibOwnerCond + `
	// 	GROUP BY ih.status
	// 	ORDER BY
	// 		CASE ih.status
	// 			WHEN 'received'   THEN 1
	// 			WHEN 'inspection' THEN 2
	// 			WHEN 'putaway'    THEN 3
	// 			ELSE 99
	// 		END
	// `
	inboundSQL := `WITH ih AS (
SELECT a.id, 
a.inbound_date, a.deleted_at,
SUM(b.quantity) as qty,
CASE a.status
			WHEN 'open'   THEN 'open'
			WHEN 'checking' THEN 'checking'
			WHEN 'partially received'    THEN 'partially_received'
			WHEN 'fully received'    THEN 'fully_received'
			WHEN 'complete'    THEN 'complete' END AS [status]
FROM inbound_headers a
LEFT JOIN inbound_details b ON a.id = b.inbound_id
GROUP BY a.id, a.[status], a.inbound_date, a.deleted_at)

SELECT
	ih.status                       AS stage_key,
	COUNT(DISTINCT ih.id)           AS total_orders,
	SUM(qty)                             AS total_qty
FROM ih 
WHERE ih.deleted_at IS NULL
		` + ibDateCond + `
		` + ibOwnerCond + `
GROUP BY ih.status
ORDER BY
	CASE ih.status
			WHEN 'open'   THEN 1
			WHEN 'checking' THEN 2
			WHEN 'partially_received'    THEN 3
			WHEN 'fully_received'    THEN 4
			WHEN 'complete'    THEN 5
			ELSE 99
	END`

	type PipelineRow struct {
		StageKey    string `json:"stage_key"     gorm:"column:stage_key"`
		TotalOrders int    `json:"total_orders"  gorm:"column:total_orders"`
		TotalQty    int    `json:"total_qty"     gorm:"column:total_qty"`
	}

	var outboundRows []PipelineRow
	var inboundRows []PipelineRow

	fmt.Println("=== OUTBOUND SQL ===")
	fmt.Println(outboundSQL)
	fmt.Println("=== INBOUND SQL ===")
	fmt.Println(inboundSQL)

	if err := c.DB.Raw(outboundSQL).Scan(&outboundRows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "outbound pipeline query failed: " + err.Error(),
		})
	}

	if err := c.DB.Raw(inboundSQL).Scan(&inboundRows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "inbound pipeline query failed: " + err.Error(),
		})
	}

	// Null-safe: kembalikan slice kosong, bukan null
	if outboundRows == nil {
		outboundRows = []PipelineRow{}
	}
	if inboundRows == nil {
		inboundRows = []PipelineRow{}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Pipeline data found",
		"data": fiber.Map{
			"outbound": outboundRows,
			"inbound":  inboundRows,
		},
	})
}
