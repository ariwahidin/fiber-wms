package controllers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DashboardController struct {
	DB *gorm.DB
}

func NewDashboardController(db *gorm.DB) *DashboardController {
	return &DashboardController{DB: db}
}

func (c *DashboardController) GetDashboard(ctx *fiber.Ctx) error {

	sql := `WITH ib AS (
			SELECT ih.id, ih.inbound_no AS no_ref,ih.receipt_id AS reference_no, ih.status, ih.inbound_date AS trans_date, id.tot_item, id.tot_qty
			FROM inbound_headers ih
			INNER JOIN (
				SELECT inbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty FROM inbound_details GROUP BY inbound_id
			) id ON ih.id = id.inbound_id
			WHERE ih.status <> 'complete'
		), ob AS (
			SELECT oh.id, oh.outbound_no AS no_ref, oh.shipment_id AS reference_no, oh.status, oh.outbound_date AS trans_date, od.tot_item, od.tot_qty
			FROM outbound_headers oh
			INNER JOIN (
				SELECT outbound_id, COUNT(item_code) AS tot_item, SUM(quantity) AS tot_qty FROM outbound_details GROUP BY outbound_id
			) od ON oh.id = od.outbound_id
			WHERE oh.status <> 'complete'
		)

		SELECT *, 'inbound' AS trans_type FROM ib
		UNION ALL
		SELECT *, 'outbound' AS trans_type FROM ob ORDER BY trans_type, no_ref DESC`

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
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Dashboard found", "data": fiber.Map{"transactions": []interface{}{}}})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Dashboard found", "data": fiber.Map{"transactions": transactions}})
}
