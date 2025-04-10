package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"reflect"
	"time"

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
	fmt.Println("PostScanForm : ", string(ctx.Body()))
	// return nil

	var scanForm ScanForm
	if err := ctx.BodyParser(&scanForm); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse JSON" + err.Error(),
		})
	}

	outboundID := scanForm.OutboundID
	outboundDetailID := scanForm.OutboundDetailID
	ItemID := scanForm.ItemID
	itemCode := scanForm.ItemCode
	scanData := scanForm.ScanData
	qtyScan := scanForm.Quantity

	// start DB transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
		})
	}

	var pickingSheet models.PickingSheet

	if err := tx.Debug().Where("outbound_id = ? AND outbound_detail_id = ? AND item_code = ? AND item_id = ? AND serial_number = ? AND is_suggestion = ?", outboundID, outboundDetailID, itemCode, ItemID, scanData, "Y").First(&pickingSheet).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to get picking sheet",
			})
		}
	}

	var outboundDetail models.OutboundDetail
	if err := tx.Debug().
		Where("id = ?", outboundDetailID).
		First(&outboundDetail).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Outbound detail not found",
			})
		}

		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to retrieve outbound detail",
		})
	}

	// totalScanned := pickingSheet.ScannedQty + qtyScan
	// if totalScanned > pickingSheet.Quantity {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Scan quantity is not enough",
	// 	})
	// }

	var originQty struct {
		ReqQty  int
		ScanQty int
	}

	sqlScan := `SELECT 
		SUM(quantity) AS req_qty,
		(
			SELECT SUM(scanned_qty) 
			FROM picking_sheets
			WHERE outbound_id = ? AND item_id = ?
		) AS scan_qty
		FROM outbound_details
		WHERE outbound_id = ? AND item_id = ?`

	if err := tx.Raw(sqlScan, outboundID, ItemID, outboundID, ItemID).Scan(&originQty).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get picking summary",
		})
	}

	if originQty.ScanQty+qtyScan > originQty.ReqQty {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Scan Qty > Req Qty",
		})
	}

	if pickingSheet.ID != 0 {

		totalScanned := pickingSheet.ScannedQty + qtyScan

		if totalScanned > pickingSheet.Quantity {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Stock Not Available",
			})
		}

		// Update existing record

		pickingSheet.ScannedQty += qtyScan
		pickingSheet.UpdatedBy = int(ctx.Locals("userID").(float64))
		pickingSheet.UpdatedAt = time.Now()

		if err := tx.Debug().Model(&models.PickingSheet{}).Where("id = ?", pickingSheet.ID).
			Updates(map[string]interface{}{
				"scanned_qty": pickingSheet.ScannedQty,
				"updated_by":  pickingSheet.UpdatedBy,
				"updated_at":  pickingSheet.UpdatedAt,
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update picking sheet",
			})
		}
	} else {

		// Cari di Inventory dengan ItemID, Serial Number, dan QtyAvailable > 0, order by Rec Date, Pallet, Location agar fifo
		var inventory models.Inventory
		if err := tx.Debug().
			Where("item_id = ? AND serial_number = ? AND qty_available > 0", ItemID, scanForm.ScanData).
			Order("rec_date, pallet, location DESC").
			First(&inventory).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"success": false,
					"message": "Inventory data not found",
				})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to retrieve inventory data",
			})
		}

		if inventory.QtyAvailable < qtyScan {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Stock Not Enough",
			})
		}

		var product models.Product

		if err := tx.Debug().
			Where("id = ?", ItemID).
			First(&product).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"success": false,
					"message": "Product not found",
				})
			}

			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to retrieve product data",
			})
		}

		// create New Picking Sheet
		newPicking := models.PickingSheet{
			InventoryID:      int(inventory.ID),
			OutboundId:       outboundDetail.OutboundID,
			OutboundDetailId: int(outboundDetail.ID),
			ItemID:           ItemID,
			Barcode:          product.Barcode,
			ItemCode:         scanForm.ItemCode,
			SerialNumber:     scanData,
			Pallet:           inventory.Pallet,
			Location:         inventory.Location,
			Quantity:         qtyScan,
			ScannedQty:       qtyScan,
			WhsCode:          inventory.WhsCode,
			QaStatus:         inventory.QaStatus,
			Status:           "scanned",
			IsSuggestion:     "N",
			CreatedBy:        int(ctx.Locals("userID").(float64)),
		}

		if err := tx.Create(&newPicking).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create new picking sheet",
			})
		}

		// Update Inventory
		if err := tx.Debug().
			Model(&models.Inventory{}).
			Where("id = ?", inventory.ID).
			Updates(map[string]interface{}{
				"qty_available": gorm.Expr("qty_available - ?", qtyScan),
				"qty_allocated": gorm.Expr("qty_allocated + ?", qtyScan),
				"updated_by":    int(ctx.Locals("userID").(float64)),
				"updated_at":    time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update inventory",
			})
		}

	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Post Scan Form",
	})

	// // Check Outbound Detail
	// outboundRepo := repositories.NewOutboundRepository(c.DB)
	// outboundDetailItem, err := outboundRepo.GetOutboundDetailItem(scanForm.OutboundID, scanForm.OutboundDetailID)
	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Failed to get outbound detail item",
	// 	})
	// }

	// if scanForm.Quantity < 1 {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Quantity must be greater than 0",
	// 	})
	// }

	// qtyScanPredict := scanForm.Quantity + outboundDetailItem.QtyScan
	// if qtyScanPredict > outboundDetailItem.QtyReq {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Quantity scan is more than quantity request",
	// 	})
	// }

	// // Check Stock On Hand
	// inventoryRepo := repositories.NewInventoryRepository(c.DB)
	// stock, err := inventoryRepo.GetStockOnHand()
	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Failed to get stock on hand",
	// 	})
	// }

	// var selectedStock repositories.StockOnHand

	// fmt.Println("stock : ", stock)

	// var stockFound bool
	// for _, s := range stock {

	// 	if s.ItemCode == scanForm.ItemCode && s.Available >= scanForm.Quantity && s.SerialNumber == scanForm.ScanData && s.WhsCode == outboundDetailItem.WhsCode {
	// 		selectedStock = s
	// 		stockFound = true
	// 		break
	// 	}
	// }

	// if !stockFound {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Stock not found",
	// 	})
	// }

	// if scanForm.Koli < 1 {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Koli must be greater than 0",
	// 	})
	// }

	// var outboundBarcode models.OutboundBarcode
	// outboundBarcode.OutboundId = scanForm.OutboundID
	// outboundBarcode.OutboundDetailId = scanForm.OutboundDetailID
	// outboundBarcode.ItemID = outboundDetailItem.ItemID
	// outboundBarcode.SeqBox = scanForm.Koli
	// outboundBarcode.ItemCode = outboundDetailItem.ItemCode
	// outboundBarcode.ScanType = scanForm.ScanType
	// outboundBarcode.ScanData = scanForm.ScanData
	// outboundBarcode.Barcode = scanForm.Barcode
	// outboundBarcode.SerialNumber = scanForm.ScanData
	// outboundBarcode.Quantity = scanForm.Quantity
	// outboundBarcode.Status = "picked"
	// outboundBarcode.InvetoryID = selectedStock.InventoryID
	// outboundBarcode.InventoryDetailID = selectedStock.InventoryDetailID
	// outboundBarcode.CreatedBy = int(ctx.Locals("userID").(float64))

	// if err := c.DB.Create(&outboundBarcode).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Failed to create outbound barcode",
	// 	})
	// }

	// outboundDetailItem.QtyScan = qtyScanPredict

	// return ctx.JSON(fiber.Map{
	// 	"success": true,
	// 	"message": "Post Scan Form",
	// 	"data": fiber.Map{
	// 		"scan_form":         scanForm,
	// 		"outbound_detail":   outboundDetailItem,
	// 		"outbound_detailID": scanForm.OutboundDetailID,
	// 	},
	// })
}
