package inbound_controller

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ---------- 1. CHECK ALL (insert ke InboundBarcode) ----------

func (c *InboundController) CheckPutawayByInboundNo(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var invPolicy models.InventoryPolicy
	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var warehouse models.Warehouse
	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if invPolicy.RequirePutawayScan {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
	}

	var inboundDetail []models.InboundDetail
	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if !invPolicy.RequireReceiveScan {

		var allRcvLocationIsFilled bool = true
		for _, detail := range inboundDetail {
			if detail.Location == "" {
				allRcvLocationIsFilled = false
				break
			}
		}

		if !allRcvLocationIsFilled {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
		}

		for _, detail := range inboundDetail {

			var inboundBarcodesCheck []models.InboundBarcode
			if err := c.DB.Debug().Where("inbound_detail_id = ?", detail.ID).Find(&inboundBarcodesCheck).Error; err != nil {
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			totalQtyReq := detail.Quantity
			totalQtyScanned := float64(0)
			newQtyScanned := float64(0)

			for _, inboundBarcode := range inboundBarcodesCheck {
				totalQtyScanned += inboundBarcode.Quantity
			}

			newQtyScanned = totalQtyReq - totalQtyScanned

			inputSerialNumber := detail.SerialNumber
			if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
				inputSerialNumber = detail.SerialNumber
			}

			var location models.Location
			if err := c.DB.Debug().First(&location, "location_code = ?", detail.Location).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location " + detail.Location + " not registered in system"})
				}
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			if newQtyScanned > 0 {
				newInboundBarcode := models.InboundBarcode{
					InboundId:       int(inboundHeader.ID),
					InboundDetailId: int(detail.ID),
					ItemCode:        detail.ItemCode,
					ItemID:          detail.ItemId,
					ScanData:        detail.Barcode,
					Barcode:         detail.Barcode,
					CartonNumber:    detail.CartonNumber,
					CaseNumber:      detail.CaseNumber,
					SerialNumber:    inputSerialNumber,
					Pallet:          palletID,
					Location:        location.LocationCode,
					Quantity:        newQtyScanned,
					WhsCode:         detail.WhsCode,
					OwnerCode:       detail.OwnerCode,
					DivisionCode:    detail.DivisionCode,
					QaStatus:        detail.QaStatus,
					Status:          "pending",
					Uom:             detail.Uom,
					RecDate:         detail.RecDate,
					ProdDate:        detail.ProdDate,
					ExpDate:         detail.ExpDate,
					LotNumber:       detail.LotNumber,
					CreatedBy:       int(ctx.Locals("userID").(float64)),
				}

				if err := c.DB.Debug().Create(&newInboundBarcode).Error; err != nil {
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}
	}

	// ambil semua yang pending, buat ditampilin ke FE sebagai preview sebelum confirm
	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"inbound_no": inboundHeader.InboundNo,
		"pallet":     palletID,
		"items":      inboundBarcodes,
	})
}

// ---------- 2. CONFIRM PUTAWAY (jalanin servicePutawayPerItem) ----------

func (c *InboundController) ConfirmPutawayByInboundNo(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
	}

	for _, barcode := range inboundBarcodes {
		barcodeIDStr := strconv.Itoa(int(barcode.ID))

		if err := c.servicePutawayPerItem(ctx, barcodeIDStr); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   err.Error(),
				"message": err.Error(),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Putaway inbound " + inboundHeader.InboundNo + " successfully"})
}
