package mobiles

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobilePackingController struct {
	DB *gorm.DB
}

func NewMobilePackingController(DB *gorm.DB) *MobilePackingController {
	return &MobilePackingController{DB: DB}
}

func (c *MobilePackingController) GenerateKoli(ctx *fiber.Ctx) error {

	// Parse request body
	var requestBody struct {
		OutboundNo string `json:"outbound_no"`
	}

	if err := ctx.BodyParser(&requestBody); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Cek apakah OutboundNo ada
	var outboundHeader models.OutboundHeader
	if err := c.DB.Where("outbound_no = ?", requestBody.OutboundNo).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Outbound not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Ambil max no_koli yang sudah ada
	var maxKoliNo string
	err := c.DB.Table("koli_headers").
		Select("COALESCE(MAX(no_koli), '') as max_koli_no").
		Where("outbound_id = ?", outboundHeader.ID).
		Scan(&maxKoliNo).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Generate new koli number
	var newKoliNo string
	if maxKoliNo == "" {
		// Belum ada koli
		newKoliNo = fmt.Sprintf("%s%04d", outboundHeader.OutboundNo, 1)
	} else {
		// Ambil 4 digit terakhir dari maxKoliNo
		lastFour := maxKoliNo[len(maxKoliNo)-4:]
		lastNumber, err := strconv.Atoi(lastFour)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Invalid existing KoliNo format",
			})
		}
		newKoliNo = fmt.Sprintf("%s%04d", outboundHeader.OutboundNo, lastNumber+1)
	}

	// Simpan ke database
	koliHeader := models.KoliHeader{
		NoKoli:     newKoliNo,
		OutboundID: outboundHeader.ID,
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&koliHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Koli header created successfully",
		"data":    koliHeader,
	})
}

func (c *MobilePackingController) GetKoliByOutbound(ctx *fiber.Ctx) error {

	outboundNo := ctx.Params("outbound_no")

	var outboundHeader models.OutboundHeader
	if err := c.DB.Where("outbound_no = ?", outboundNo).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Outbound not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var koliHeaders []models.KoliHeader
	if err := c.DB.Preload("Details").Where("outbound_id = ?", outboundHeader.ID).Find(&koliHeaders).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    koliHeaders,
	})
}

func (c *MobilePackingController) AddToKoli(ctx *fiber.Ctx) error {
	var requestBody struct {
		OutboundNo       string `json:"outbound_no"`
		Barcode          string `json:"barcode"`
		KoliID           int    `json:"koli_id"`
		NoKoli           string `json:"no_koli"`
		OutboundDetailID int    `json:"outbound_detail_id"`
		Qty              int    `json:"qty"`
		ScanType         string `json:"scan_type"`
		SerialNumber     string `json:"serial_number"`
		SerialNumber2    string `json:"serial_number2"`
	}

	if err := ctx.BodyParser(&requestBody); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if requestBody.OutboundNo == "" || requestBody.Barcode == "" || requestBody.KoliID == 0 || requestBody.NoKoli == "" || requestBody.Qty == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing required fields",
		})
	}

	var outboundHeader models.OutboundHeader
	if err := c.DB.Where("outbound_no = ?", requestBody.OutboundNo).First(&outboundHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Outbound not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var outboundDetail models.OutboundDetail
	if err := c.DB.Debug().Where("barcode = ? AND outbound_id = ?", requestBody.Barcode, outboundHeader.ID).First(&outboundDetail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Item not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	fmt.Println("requestBody:", requestBody)

	// return nil

	if requestBody.ScanType == "SERIAL" {
		var koliDetails []models.KoliDetail
		if err := c.DB.Debug().Where("barcode = ? AND serial_number = ?", requestBody.Barcode, requestBody.SerialNumber).Find(&koliDetails).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if len(koliDetails) > 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Item already scanned",
			})
		}
	}

	// return nil

	var totalQtyRequest int
	err := c.DB.Debug().Model(&models.OutboundDetail{}).
		Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, requestBody.Barcode).
		Select("COALESCE(SUM(quantity),0) as total_qty_request").
		Scan(&totalQtyRequest).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var totalQtyPack int
	err = c.DB.Debug().Model(&models.KoliDetail{}).
		Where("outbound_id = ? AND barcode = ?", outboundHeader.ID, requestBody.Barcode).
		Select("COALESCE(SUM(qty),0) as total_qty_pack").
		Scan(&totalQtyPack).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if totalQtyPack+requestBody.Qty > totalQtyRequest {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Quantity exceed",
		})
	}

	var product models.Product
	if err := c.DB.Where("barcode = ?", requestBody.Barcode).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Product " + requestBody.Barcode + " not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var pickingSheets []models.PickingSheet

	serialMandatroy := false

	if !serialMandatroy {

		// START OF CODE IF SERIAL NUMBER IS NOT MANDATORY
		if err := c.DB.Debug().
			Where("barcode = ? AND outbound_id = ?", requestBody.Barcode, outboundHeader.ID).
			Find(&pickingSheets).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if len(pickingSheets) == 0 {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Picking sheet not found",
			})
		}

		qtyReq := requestBody.Qty
		for _, sheet := range pickingSheets {
			qtyPicking := sheet.QtyAvailable
			if qtyReq < qtyPicking {
				qtyPicking = qtyReq
			}

			var koliDetail models.KoliDetail
			koliDetail.KoliID = requestBody.KoliID
			koliDetail.NoKoli = requestBody.NoKoli
			koliDetail.PickingSheetID = int(sheet.ID)
			koliDetail.OutboundDetailID = int(outboundDetail.ID)
			koliDetail.ItemCode = sheet.ItemCode
			koliDetail.Barcode = requestBody.Barcode
			koliDetail.SerialNumber = requestBody.SerialNumber
			koliDetail.Qty = qtyPicking
			koliDetail.ItemID = int(product.ID)
			koliDetail.InventoryID = sheet.InventoryID
			koliDetail.OutboundID = int(outboundHeader.ID)
			koliDetail.CreatedBy = int(ctx.Locals("userID").(float64))

			// create new koli detail
			if err := c.DB.Debug().Create(&koliDetail).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			// update picking sheet
			if err := c.DB.Model(&models.PickingSheet{}).Where("id = ?", sheet.ID).
				Updates(map[string]interface{}{
					"qty_available": sheet.QtyAvailable - qtyPicking,
					"qty_allocated": sheet.QtyAllocated + qtyPicking,
					"updated_by":    int(ctx.Locals("userID").(float64)),
					"updated_at":    time.Now(),
				}).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			qtyReq -= qtyPicking
			if qtyReq <= 0 {
				break
			}
		}
		// END OF CODE IF SERIAL NUMBER IS NOT MANDATORY

	} else {

		// START OF CODE IF SERIAL NUMBER IS MANDATORY

		if err := c.DB.Debug().
			Where("barcode = ? AND serial_number = ? AND outbound_id = ?", requestBody.Barcode, requestBody.SerialNumber, outboundHeader.ID).
			Find(&pickingSheets).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if len(pickingSheets) == 0 {

			var inventories []models.Inventory

			if err := c.DB.Debug().Where("item_id = ? AND whs_code = ? AND barcode = ? AND serial_number = ? AND qty_available > 0", outboundDetail.ItemID, outboundDetail.WhsCode, requestBody.Barcode, requestBody.SerialNumber).Find(&inventories).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			if len(inventories) == 0 {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Item " + outboundDetail.ItemCode + " not found in inventory with whs code " + outboundDetail.WhsCode,
				})
			}

			qtyReq := requestBody.Qty
			for _, inventory := range inventories {
				qtyPicking := inventory.QtyAvailable
				if qtyReq < qtyPicking {
					qtyPicking = qtyReq
				}

				var koliDetail models.KoliDetail
				koliDetail.KoliID = requestBody.KoliID
				koliDetail.NoKoli = requestBody.NoKoli
				koliDetail.OutboundDetailID = int(outboundDetail.ID)
				koliDetail.ItemCode = inventory.ItemCode
				koliDetail.Barcode = inventory.Barcode
				koliDetail.SerialNumber = inventory.SerialNumber
				koliDetail.Qty = qtyPicking
				koliDetail.ItemID = int(product.ID)
				koliDetail.InventoryID = int(inventory.ID)
				koliDetail.OutboundID = int(outboundHeader.ID)
				koliDetail.CreatedBy = int(ctx.Locals("userID").(float64))

				// create new koli detail
				if err := c.DB.Debug().Create(&koliDetail).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				}

				// update inventory
				if err := c.DB.Debug().Model(&models.Inventory{}).Where("id = ?", inventory.ID).
					Updates(map[string]interface{}{
						"qty_available": inventory.QtyAvailable - qtyPicking,
						"qty_allocated": inventory.QtyAllocated + qtyPicking,
						"updated_by":    int(ctx.Locals("userID").(float64)),
						"updated_at":    time.Now(),
					}).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": err.Error(),
					})
				}

				qtyReq -= qtyPicking
				if qtyReq <= 0 {
					break
				}
			}

			return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
				"success": true,
				"message": "Item added to koli from inventory successfully",
			})
		}

		qtyReq := requestBody.Qty
		for _, sheet := range pickingSheets {
			qtyPicking := sheet.QtyAvailable
			if qtyReq < qtyPicking {
				qtyPicking = qtyReq
			}

			var koliDetail models.KoliDetail
			koliDetail.KoliID = requestBody.KoliID
			koliDetail.NoKoli = requestBody.NoKoli
			koliDetail.PickingSheetID = int(sheet.ID)
			koliDetail.OutboundDetailID = int(outboundDetail.ID)
			koliDetail.ItemCode = sheet.ItemCode
			koliDetail.Barcode = requestBody.Barcode
			koliDetail.SerialNumber = requestBody.SerialNumber
			koliDetail.Qty = qtyPicking
			koliDetail.ItemID = int(product.ID)
			koliDetail.InventoryID = sheet.InventoryID
			koliDetail.OutboundID = int(outboundHeader.ID)
			koliDetail.CreatedBy = int(ctx.Locals("userID").(float64))

			// create new koli detail
			if err := c.DB.Debug().Create(&koliDetail).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			// update picking sheet
			if err := c.DB.Model(&models.PickingSheet{}).Where("id = ?", sheet.ID).
				Updates(map[string]interface{}{
					"qty_available": sheet.QtyAvailable - qtyPicking,
					"qty_allocated": sheet.QtyAllocated + qtyPicking,
					"updated_by":    int(ctx.Locals("userID").(float64)),
					"updated_at":    time.Now(),
				}).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": err.Error(),
				})
			}

			qtyReq -= qtyPicking
			if qtyReq <= 0 {
				break
			}
		}

		// END OF CODE IF SERIAL NUMBER IS MANDATORY

	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Item added to koli from picking sheets successfully",
	})

}

func (c *MobilePackingController) RemoveItemFromKoli(ctx *fiber.Ctx) error {

	id := ctx.Params("id")
	var koliDetail models.KoliDetail
	if err := c.DB.Debug().Where("id = ?", id).First(&koliDetail).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Koli detail not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var inventory models.Inventory
	if err := c.DB.Debug().Where("id = ?", koliDetail.InventoryID).First(&inventory).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Inventory not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if koliDetail.PickingSheetID > 0 {

		var pickingSheet models.PickingSheet
		if err := c.DB.Debug().Where("id = ?", koliDetail.PickingSheetID).First(&pickingSheet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Picking sheet not found",
				})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Jika data ditemukan, lanjut update
		if err := c.DB.Debug().Model(&models.PickingSheet{}).Where("id = ?", pickingSheet.ID).
			Updates(map[string]interface{}{
				"qty_available": pickingSheet.QtyAvailable + koliDetail.Qty,
				"qty_allocated": pickingSheet.QtyAllocated - koliDetail.Qty,
				"updated_by":    int(ctx.Locals("userID").(float64)),
				"updated_at":    time.Now(),
			}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

	} else {
		// updating inventory
		if err := c.DB.Debug().Model(&models.Inventory{}).Where("id = ?", inventory.ID).Updates(map[string]interface{}{
			"qty_available": inventory.QtyAvailable + koliDetail.Qty,
			"qty_allocated": inventory.QtyAllocated - koliDetail.Qty,
			"updated_by":    int(ctx.Locals("userID").(float64)),
			"updated_at":    time.Now(),
		}).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	//  hard delete

	if err := c.DB.Debug().Unscoped().Delete(&koliDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Koli detail removed successfully",
	})
}

func (c *MobilePackingController) RemoveKoliByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var koliHeader models.KoliHeader
	if err := c.DB.Where("id = ?", id).First(&koliHeader).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Koli header not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var koliDetails []models.KoliDetail
	if err := c.DB.Where("koli_id = ?", koliHeader.ID).Find(&koliDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if len(koliDetails) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot remove Koli header with associated Koli details",
		})
	}

	// hard delete
	if err := c.DB.Debug().Unscoped().Delete(&koliHeader).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Koli header removed successfully",
	})
}
