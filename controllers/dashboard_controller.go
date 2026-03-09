// package controllers

// import (
// 	"github.com/gofiber/fiber/v2"
// 	"gorm.io/gorm"
// )

// type DashboardController struct {
// 	DB *gorm.DB
// }

// func NewDashboardController(db *gorm.DB) *DashboardController {
// 	return &DashboardController{DB: db}
// }

// func (c *DashboardController) GetDashboard(ctx *fiber.Ctx) error {

// 	sql := `WITH ib AS (
// 			SELECT ih.id, ih.inbound_no AS no_ref,ih.receipt_id AS reference_no, ih.status, ih.inbound_date AS trans_date, id.tot_item, id.tot_qty
// 			FROM inbound_headers ih
// 			INNER JOIN (
// 				SELECT inbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty FROM inbound_details GROUP BY inbound_id
// 			) id ON ih.id = id.inbound_id
// 			WHERE ih.status <> 'complete'
// 		), ob AS (
// 			SELECT oh.id, oh.outbound_no AS no_ref, oh.shipment_id AS reference_no, oh.status, oh.outbound_date AS trans_date, od.tot_item, od.tot_qty
// 			FROM outbound_headers oh
// 			INNER JOIN (
// 				SELECT outbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty FROM outbound_details GROUP BY outbound_id
// 			) od ON oh.id = od.outbound_id
// 			WHERE oh.status <> 'complete'
// 		)

// 		SELECT *, 'inbound' AS trans_type FROM ib
// 		UNION ALL
// 		SELECT *, 'outbound' AS trans_type FROM ob ORDER BY trans_type, no_ref DESC`

// 	var transactions []struct {
// 		ID          uint   `json:"id"`
// 		NoRef       string `json:"no_ref"`
// 		ReferenceNo string `json:"reference_no"`
// 		Status      string `json:"status"`
// 		TransDate   string `json:"trans_date"`
// 		TotItem     int    `json:"tot_item"`
// 		TotQty      int    `json:"tot_qty"`
// 		TransType   string `json:"trans_type"`
// 	}

// 	if err := c.DB.Raw(sql).Scan(&transactions).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(transactions) == 0 {
// 		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Dashboard found", "data": fiber.Map{"transactions": []interface{}{}}})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Dashboard found", "data": fiber.Map{"transactions": transactions}})
// }

// func (c *DashboardController) GetDashboardChart(ctx *fiber.Ctx) error {

// 	// ── 1. Transaction Trend (7 hari terakhir) ───────────────
// 	trendSQL := `
// 		WITH dates AS (
// 			SELECT CAST(DATEADD(DAY, -seq, CAST(GETDATE() AS DATE)) AS DATE) AS d
// 			FROM (
// 				SELECT 0 seq UNION SELECT 1 UNION SELECT 2 UNION SELECT 3
// 				UNION SELECT 4 UNION SELECT 5 UNION SELECT 6
// 			) t
// 		),
// 		ib AS (
// 			SELECT CAST(inbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
// 			FROM inbound_headers
// 			WHERE CAST(inbound_date AS DATE) >= CAST(DATEADD(DAY, -6, GETDATE()) AS DATE)
// 			  AND deleted_at IS NULL
// 			GROUP BY CAST(inbound_date AS DATE)
// 		),
// 		ob AS (
// 			SELECT CAST(outbound_date AS DATE) AS trans_date, COUNT(*) AS cnt
// 			FROM outbound_headers
// 			WHERE CAST(outbound_date AS DATE) >= CAST(DATEADD(DAY, -6, GETDATE()) AS DATE)
// 			  AND deleted_at IS NULL
// 			GROUP BY CAST(outbound_date AS DATE)
// 		)
// 		SELECT
// 			CONVERT(VARCHAR(10), dates.d, 23) AS date,
// 			COALESCE(ib.cnt, 0)               AS inbound,
// 			COALESCE(ob.cnt, 0)               AS outbound
// 		FROM dates
// 		LEFT JOIN ib ON dates.d = ib.trans_date
// 		LEFT JOIN ob ON dates.d = ob.trans_date
// 		ORDER BY dates.d ASC
// 	`

// 	var trend []struct {
// 		Date     string `json:"date"`
// 		Inbound  int    `json:"inbound"`
// 		Outbound int    `json:"outbound"`
// 	}

// 	if err := c.DB.Raw(trendSQL).Scan(&trend).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "trend query failed: " + err.Error(),
// 		})
// 	}

// 	// ── 2. Transaction Type Mix ──────────────────────────────
// 	typeMixSQL := `
// 		SELECT
// 			CASE
// 				WHEN order_type IS NULL OR order_type = '' THEN 'Other'
// 				ELSE order_type
// 			END AS name,
// 			COUNT(*) AS value
// 		FROM outbound_headers
// 		WHERE deleted_at IS NULL
// 		GROUP BY
// 			CASE
// 				WHEN order_type IS NULL OR order_type = '' THEN 'Other'
// 				ELSE order_type
// 			END
// 		ORDER BY value DESC
// 	`
// 	// Note: SQL Server tidak bisa GROUP BY alias, jadi CASE harus diulang

// 	var typeMix []struct {
// 		Name  string `json:"name"`
// 		Value int    `json:"value"`
// 	}

// 	if err := c.DB.Raw(typeMixSQL).Scan(&typeMix).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "type_mix query failed: " + err.Error(),
// 		})
// 	}

// 	totalSQL := `
//     SELECT
//         COUNT(*)        AS total_order,
//         SUM(od.tot_qty) AS total_qty
//     FROM outbound_headers oh
//     INNER JOIN (
//         SELECT outbound_id, SUM(quantity) AS tot_qty
//         FROM outbound_details
//         WHERE deleted_at IS NULL
//         GROUP BY outbound_id
//     ) od ON oh.id = od.outbound_id
//     WHERE oh.deleted_at IS NULL
// `

// 	var total struct {
// 		TotalOrder int `json:"total_order"`
// 		TotalQty   int `json:"total_qty"`
// 	}

// 	if err := c.DB.Raw(totalSQL).Scan(&total).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "total query failed: " + err.Error(),
// 		})
// 	}

// 	// Tambah total ke response data:
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Chart data found",
// 		"data": fiber.Map{
// 			"trend":    trend,
// 			"type_mix": typeMix,
// 			"total":    total, // <-- tambah ini
// 		},
// 	})
// }

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
