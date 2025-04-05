package controllers

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type RfOutboundController struct {
	DB *gorm.DB
}

type ScanForm struct {
	ScanType         string `json:"scan_type"`
	ScanData         string `json:"scan_data"`
	OutboundID       int    `json:"outbound_id"`
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
	Koli             int    `json:"koli"`
	Quantity         int    `json:"quantity"`
	ReqQty           int    `json:"req_qty"`
	ScannedQty       int    `json:"scanned_qty"`
	Uom              string `json:"uom"`
	ItemHasSerial    string `json:"item_has_serial"`
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

	var scanForm ScanForm
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundOpenList, err := outboundRepo.GetOutboundPicking()
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

func (c *RfOutboundController) GetAllListOutboundPicking(ctx *fiber.Ctx) error {

	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundList, err := outboundRepo.GetOutboundPicking()
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

func (c *RfOutboundController) PostScanForm(ctx *fiber.Ctx) error {
	// fmt.Println("PostScanForm : ", string(ctx.Body()))
	// return nil

	var scanForm ScanForm
	if err := ctx.BodyParser(&scanForm); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse JSON" + err.Error(),
		})
	}

	// Check Outbound Detail
	outboundRepo := repositories.NewOutboundRepository(c.DB)
	outboundDetailItem, err := outboundRepo.GetOutboundDetailItem(scanForm.OutboundID, scanForm.OutboundDetailID)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get outbound detail item",
		})
	}

	if scanForm.Quantity < 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Quantity must be greater than 0",
		})
	}

	qtyScanPredict := scanForm.Quantity + outboundDetailItem.QtyScan
	if qtyScanPredict > outboundDetailItem.QtyReq {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Quantity scan is more than quantity request",
		})
	}

	// Check Stock On Hand
	inventoryRepo := repositories.NewInventoryRepository(c.DB)
	stock, err := inventoryRepo.GetStockOnHand()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock on hand",
		})
	}

	var selectedStock repositories.StockOnHand

	fmt.Println("stock : ", stock)

	var stockFound bool
	for _, s := range stock {

		fmt.Println("Item Code : ", s.ItemCode)
		fmt.Println("SF Item Code : ", scanForm.ItemCode)

		fmt.Println("Serial Number : ", s.SerialNumber)
		fmt.Println("SF Serial Number : ", scanForm.ScanData)

		fmt.Println("Stock Available : ", s.Available)
		fmt.Println("Qty Scan Predict : ", qtyScanPredict)

		if s.ItemCode == scanForm.ItemCode && s.Available >= scanForm.Quantity && s.SerialNumber == scanForm.ScanData && s.WhsCode == outboundDetailItem.WhsCode {
			selectedStock = s
			stockFound = true
			break
		}
	}

	if !stockFound {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Stock not found",
		})
	}

	if scanForm.Koli < 1 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Koli must be greater than 0",
		})
	}

	// var maxSeqBox int
	// errs := c.DB.Model(&models.OutboundBarcode{}).Select("MAX(seq_box)").Where("outbound_id = ?", scanForm.OutboundID).Scan(&maxSeqBox).Error
	// if errs != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Failed to get max seq box",
	// 	})
	// }
	// var nextSeqBox int = maxSeqBox + 1
	// if scanForm.Koli > nextSeqBox {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Max koli is " + strconv.Itoa(nextSeqBox),
	// 	})
	// }
	// fmt.Println("selectedStock : ", selectedStock)

	var outboundBarcode models.OutboundBarcode
	outboundBarcode.OutboundId = scanForm.OutboundID
	outboundBarcode.OutboundDetailId = scanForm.OutboundDetailID
	outboundBarcode.ItemID = outboundDetailItem.ItemID
	outboundBarcode.SeqBox = scanForm.Koli
	outboundBarcode.ItemCode = outboundDetailItem.ItemCode
	outboundBarcode.ScanType = scanForm.ScanType
	outboundBarcode.ScanData = scanForm.ScanData
	outboundBarcode.Barcode = scanForm.Barcode
	outboundBarcode.SerialNumber = scanForm.ScanData
	outboundBarcode.Quantity = scanForm.Quantity
	outboundBarcode.Status = "picked"
	outboundBarcode.InvetoryID = selectedStock.InventoryID
	outboundBarcode.InventoryDetailID = selectedStock.InventoryDetailID
	outboundBarcode.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&outboundBarcode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create outbound barcode",
		})
	}

	outboundDetailItem.QtyScan = qtyScanPredict

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Post Scan Form",
		"data": fiber.Map{
			"scan_form":         scanForm,
			"outbound_detail":   outboundDetailItem,
			"outbound_detailID": scanForm.OutboundDetailID,
		},
	})
}
