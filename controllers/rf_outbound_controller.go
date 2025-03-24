package controllers

import (
	"fiber-app/repositories"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type RfOutboundController struct {
	DB *gorm.DB
}

func NewRfOutboundController(DB *gorm.DB) *RfOutboundController {
	return &RfOutboundController{DB: DB}
}

// Fungsi untuk mengambil struktur dari struct dalam bentuk JSON
func GetStructFields(s interface{}) []map[string]string {
	t := reflect.TypeOf(s)
	var fields []map[string]string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fields = append(fields, map[string]string{
			"name": field.Name,
			"type": field.Type.String(), // Mendapatkan tipe data (string, int, dll.)
			"json": field.Tag.Get("json"),
		})
	}
	return fields
}

func (c *RfOutboundController) ScanForm(ctx *fiber.Ctx) error {
	type ScanForm struct {
		ScanType         string `json:"scan_type"`
		ScanData         string `json:"scan_data"`
		OutboundID       string `json:"outbound_id"`
		OutboundNo       string `json:"outbound_no"`
		DeliveryNo       string `json:"delivery_no"`
		Barcode          string `json:"barcode"`
		StatusOutbound   string `json:"status_outbound"`
		CustomerCode     string `json:"customer_code"`
		CustomerName     string `json:"customer_name"`
		OutboundDate     string `json:"outbound_date"`
		OutboundDetailID int    `json:"outbound_detail_id"`
		ItemID           int    `json:"item_id"`
		ItemCode         string `json:"item_code"`
		ItemName         string `json:"item_name"`
		Quantity         int    `json:"quantity"`
		ReqQty           int    `json:"req_qty"`
		ScannedQty       int    `json:"scanned_qty"`
		Uom              string `json:"uom"`
		ItemHasSerial    string `json:"item_has_serial"`
	}

	var scanForm ScanForm
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundOpenList, err := outboundRepo.GetOutboundOpen()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get outbound open list",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Scan Form",
		"data": fiber.Map{
			"scan_form":     scanForm,
			"list_outbound": outboundOpenList,
		},
	})
}

func (c *RfOutboundController) GetAllListOutboundOpen(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundList, err := outboundRepo.GetOutboundOpen()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get outbound list",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Get All List Outbound Open",
		"data": fiber.Map{
			"outbound_list": outboundList,
		},
	})
}

func (c *RfOutboundController) GetAllListOutbound(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundList, err := outboundRepo.GetAllOutboundList()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get outbound list",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Get All List Outbound",
		"data": fiber.Map{
			"outbound_list": outboundList,
		},
	})
}

func (c *RfOutboundController) GetOutboundByOutboundID(ctx *fiber.Ctx) error {
	outbound_id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid outbound_id",
		})
	}

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundDetailList, err := outboundRepo.GetOutboundDetailList(outbound_id)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get outbound detail list",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Get Inbound By Outbound ID",
		"data": fiber.Map{
			"outbound_id":          outbound_id,
			"outbound_detail_list": outboundDetailList,
		},
	})
}
