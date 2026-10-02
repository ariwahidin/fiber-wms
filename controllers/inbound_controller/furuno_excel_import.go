package inbound_controller

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/services"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func (c *InboundController) expandInboundItems(tx *gorm.DB, items []InboundItem) ([]InboundItem, error) {
	bundleService := services.NewProductBundleService(tx)
	var expanded []InboundItem
	for _, item := range items {
		var product models.Product
		if err := tx.First(&product, "item_code = ?", strings.TrimSpace(item.ItemCode)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("product not found: %s", item.ItemCode)
			}
			return nil, err
		}
		if product.IsBundle != "Y" {
			item.ItemCode = product.ItemCode
			if item.BundleProductID == 0 {
				item.BundleProductCode = ""
				item.BundleQuantity = 0
			}
			expanded = append(expanded, item)
			continue
		}
		components, err := bundleService.ExpandBundle(tx, product.ID, item.Quantity)
		if err != nil {
			return nil, fmt.Errorf("failed to expand bundle %s: %w", product.ItemCode, err)
		}
		if len(components) == 0 {
			return nil, fmt.Errorf("bundle %s has no components", product.ItemCode)
		}
		for _, component := range components {
			child := item
			child.ID = 0
			child.ItemCode = component.ItemCode
			child.Quantity = component.Qty
			child.UOM = component.UOM
			child.BundleProductID = int(product.ID)
			child.BundleProductCode = product.ItemCode
			child.BundleQuantity = item.Quantity
			// Parent-level serial is not copied to every component unless the caller already supplied component serials.
			expanded = append(expanded, child)
		}
	}
	return expanded, nil
}

func (c *InboundController) CreateInbound(ctx *fiber.Ctx) error {
	var payload Inbound
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid payload", "error": err.Error()})
	}
	if payload.ReceiptID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Receipt ID cannot be empty", "error": "Receipt ID cannot be empty"})
	}

	var policy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", payload.OwnerCode).First(&policy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to get inventory policy", "error": err.Error()})
	}
	for _, item := range payload.Items {
		if item.Quantity == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Quantity cannot be zero", "error": "Quantity cannot be zero"})
		}
		if item.UOM == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "UOM cannot be empty", "error": "UOM cannot be empty"})
		}
		if item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Item code cannot be empty", "error": "Item code cannot be empty"})
		}
		// if policy.UseReceiveLocation && item.Location == "" {
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Receive location cannot be empty", "error": "Receive location cannot be empty"})
		// }
		// if policy.UseCartonNumber && item.CartonNumber == "" {
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Carton number cannot be empty", "error": "Carton number cannot be empty"})
		// }
		// if policy.UseCaseNumber && item.CaseNumber == "" {
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Case number cannot be empty", "error": "Case number cannot be empty"})
		// }
	}

	// Expand bundle before duplicate validation / insert. The parent bundle is
	// never lost: every component carries the parent reference.
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("receipt_id = ?", payload.ReceiptID).First(&models.InboundHeader{}).Error; err == nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Receipt ID already exists", "error": "Receipt ID already exists: " + payload.ReceiptID})
	}

	repo := repositories.NewInboundRepository(tx)
	inboundNo, err := repo.GenerateInboundNo()
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate inbound no", "error": err.Error()})
	}
	userID := int(ctx.Locals("userID").(float64))

	var supplier models.Supplier
	if err := tx.First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Supplier not found", "error": "Supplier not found : " + payload.Supplier})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundHeader := models.InboundHeader{
		InboundNo: inboundNo, InboundDate: payload.InboundDate, ReceiptID: payload.ReceiptID,
		Supplier: payload.Supplier, SupplierId: int(supplier.ID), Status: "open", RawStatus: "DRAFT",
		DraftTime: time.Now(), Transporter: payload.Transporter, NoTruck: payload.NoTruck, Driver: payload.Driver,
		Container: payload.Container, Remarks: payload.Remarks, Type: payload.Type, WhsCode: payload.WhsCode,
		OwnerCode: payload.OwnerCode, Origin: payload.Origin, PoDate: payload.PoDate, ArrivalTime: payload.ArrivalTime,
		StartUnloading: payload.StartUnloading, EndUnloading: payload.EndUnloading, TruckSize: payload.TruckSize,
		BLNo: payload.BLNo, Koli: payload.Koli, CreatedBy: userID, UpdatedBy: userID,
	}
	if err := tx.Create(&inboundHeader).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to insert inbound header", "error": err.Error()})
	}

	for _, ref := range payload.References {
		if ref.RefNo == "" {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invoice no cannot be empty"})
		}
		if err := tx.Create(&models.InboundReference{InboundId: inboundHeader.ID, RefNo: ref.RefNo}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	expandedItems, err := c.expandInboundItems(tx, payload.Items)
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Failed to expand inbound items", "error": err.Error()})
	}
	if len(expandedItems) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No items after bundle expansion", "error": "No items after bundle expansion"})
	}

	// Prevent duplicate component lines after expansion.
	seen := make(map[string]bool)
	for _, item := range expandedItems {
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%d|%s", item.ItemCode, item.RecDate, item.ExpDate, item.LotNumber, item.ProdDate, item.Location, item.UOM, item.DivisionCode, item.BundleProductID, item.SerialNumber)
		if seen[key] {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Duplicate item found: " + item.ItemCode, "error": "Duplicate item found: " + item.ItemCode})
		}
		seen[key] = true
	}

	for _, item := range expandedItems {
		var product models.Product
		if err := tx.First(&product, "item_code = ?", item.ItemCode).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found: " + item.ItemCode})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var uom models.UomConversion
		if err := tx.First(&uom, "item_code = ? AND from_uom = ?", product.ItemCode, item.UOM).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found: " + item.ItemCode + " / " + item.UOM})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if !policy.UseLotNo {
			item.LotNumber = inboundHeader.InboundNo
		}
		inputQty := item.Quantity
		if len(item.SerialNumbers) > 0 {
			inputQty = float64(len(item.SerialNumbers))
		}
		refNo, refID := item.RefNo, item.RefId
		if len(payload.References) == 1 {
			var ref models.InboundReference
			if err := tx.First(&ref, "ref_no = ?", payload.References[0].RefNo).Error; err == nil {
				refNo = ref.RefNo
				refID = int(ref.ID)
			}
		}
		if refID == 0 && refNo != "" {
			var ref models.InboundReference
			if err := tx.First(&ref, "ref_no = ?", refNo).Error; err == nil {
				refID = int(ref.ID)
				refNo = ref.RefNo
			}
		}

		detail := models.InboundDetail{
			InboundNo: inboundHeader.InboundNo, InboundId: int(inboundHeader.ID), ItemCode: item.ItemCode, ItemId: product.ID,
			ProductNumber: product.ProductNumber, Barcode: uom.Ean, Uom: item.UOM, Quantity: inputQty, Location: item.Location,
			QaStatus: item.QaStatus, WhsCode: inboundHeader.WhsCode, RecDate: item.RecDate, ProdDate: item.ProdDate, ExpDate: item.ExpDate,
			LotNumber: item.LotNumber, SerialNumber: item.SerialNumber, CartonNumber: item.CartonNumber, CaseNumber: item.CaseNumber,
			RefNo: refNo, RefId: refID, IsSerial: product.HasSerial, OwnerCode: inboundHeader.OwnerCode, DivisionCode: item.DivisionCode,
			BundleProductID: item.BundleProductID, BundleProductCode: item.BundleProductCode, BundleQuantity: item.BundleQuantity,
			CreatedBy: userID, UpdatedBy: userID,
		}
		if err := tx.Create(&detail).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to insert inbound detail", "error": err.Error()})
		}

		if len(item.SerialNumbers) > 0 {
			seenSN := map[string]bool{}
			if int(item.Quantity) != len(item.SerialNumbers) {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Total serial number not match with quantity for item " + item.ItemCode})
			}
			for _, sn := range item.SerialNumbers {
				sn = strings.TrimSpace(sn)
				if sn == "" {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Serial number cannot be empty"})
				}
				if seenSN[sn] {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Duplicate serial number: " + sn})
				}
				seenSN[sn] = true
				var existing models.InboundSerial
				e := tx.Where("serial_number = ?", sn).First(&existing).Error
				if e == nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Duplicate serial number: " + sn})
				}
				if !errors.Is(e, gorm.ErrRecordNotFound) {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": e.Error()})
				}
				if err := tx.Create(&models.InboundSerial{InboundId: int(inboundHeader.ID), InboundDetailId: int(detail.ID), SerialNumber: sn, CreatedBy: userID, UpdatedBy: userID}).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}
	}

	if err := helpers.InsertTransactionHistory(tx, inboundHeader.InboundNo, "open", "INBOUND", "", userID); err != nil {
		log.Println("Gagal insert history:", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to commit transaction", "error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Inbound created successfully", "data": fiber.Map{"inbound_id": inboundHeader.ID}})
}

func (c *InboundController) UpdateInboundByID(ctx *fiber.Ctx) error {
	inboundNo := ctx.Params("inbound_no")
	var payload Inbound
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var policy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", payload.OwnerCode).First(&policy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to get inventory policy", "error": err.Error()})
	}
	for _, item := range payload.Items {
		if item.Quantity == 0 {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity cannot be zero"})
		}
		if item.UOM == "" || item.ItemCode == "" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item code and UOM cannot be empty"})
		}
		// if policy.UseReceiveLocation && item.Location == "" {
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Receive location cannot be empty"})
		// }
	}
	userID := int(ctx.Locals("userID").(float64))
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var header models.InboundHeader
	if err := tx.First(&header, "inbound_no = ?", inboundNo).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if header.Status == "complete" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + inboundNo + " is already complete", "message": "Inbound is already complete"})
	}
	var supplier models.Supplier
	if err := tx.First(&supplier, "supplier_code = ?", payload.Supplier).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	header.InboundDate = payload.InboundDate
	header.Supplier = payload.Supplier
	header.SupplierId = int(supplier.ID)
	header.ReceiptID = payload.ReceiptID
	header.Type = payload.Type
	header.Remarks = payload.Remarks
	header.UpdatedBy = userID
	header.Transporter = payload.Transporter
	header.NoTruck = payload.NoTruck
	header.Driver = payload.Driver
	header.Container = payload.Container
	header.WhsCode = payload.WhsCode
	header.OwnerCode = payload.OwnerCode
	header.Origin = payload.Origin
	header.PoDate = payload.PoDate
	header.ArrivalTime = payload.ArrivalTime
	header.StartUnloading = payload.StartUnloading
	header.EndUnloading = payload.EndUnloading
	header.TruckSize = payload.TruckSize
	header.BLNo = payload.BLNo
	header.Koli = payload.Koli
	if err := tx.Save(&header).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	for _, ref := range payload.References {
		if ref.ID > 0 {
			var r models.InboundReference
			if err := tx.First(&r, "id = ?", ref.ID).Error; err == nil {
				r.RefNo = ref.RefNo
				if err := tx.Save(&r).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
				continue
			}
		}
		if err := tx.Create(&models.InboundReference{InboundId: header.ID, RefNo: ref.RefNo}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	expanded, err := c.expandInboundItems(tx, payload.Items)
	if err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Failed to expand inbound items", "error": err.Error()})
	}
	var existing []models.InboundDetail
	if err := tx.Where("inbound_no = ?", inboundNo).Find(&existing).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	byID := map[uint]*models.InboundDetail{}
	byBundle := map[string][]*models.InboundDetail{}
	for i := range existing {
		d := &existing[i]
		byID[d.ID] = d
		key := fmt.Sprintf("%d|%s", d.BundleProductID, d.BundleProductCode)
		if d.BundleProductID > 0 && d.BundleProductCode != "" {
			byBundle[key] = append(byBundle[key], d)
		}
	}
	barcodeMap := map[uint]repositories.ResulInboundBarcodeByOutboundDetailID{}
	ids := make([]uint, 0, len(existing))
	for _, d := range existing {
		ids = append(ids, d.ID)
	}
	if len(ids) > 0 {
		barcodeMap, err = repositories.NewInboundRepository(tx).GetInboundBarcodesByDetailIDs(ids)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	var stock []models.InboundBarcode
	if len(ids) > 0 {
		if err := tx.Where("inbound_detail_id IN ? AND status = ?", ids, "in stock").Find(&stock).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	putaway := map[uint]map[string]bool{}
	for _, b := range stock {
		if _, ok := putaway[b.InboundDetailId]; !ok {
			putaway[b.InboundDetailId] = map[string]bool{}
		}
		if strings.TrimSpace(b.SerialNumber) != "" {
			putaway[b.InboundDetailId][strings.TrimSpace(b.SerialNumber)] = true
		}
	}
	serialMap := map[uint][]models.InboundSerial{}
	if len(ids) > 0 {
		var ss []models.InboundSerial
		if err := tx.Where("inbound_detail_id IN ?", ids).Find(&ss).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		for _, s := range ss {
			serialMap[uint(s.InboundDetailId)] = append(serialMap[uint(s.InboundDetailId)], s)
		}
	}

	used := map[uint]bool{}
	usedBundle := map[string]bool{}
	for _, item := range expanded {
		product, ok := func() (models.Product, bool) {
			p, ok := models.Product{}, false
			var x models.Product
			if tx.First(&x, "item_code = ?", item.ItemCode).Error == nil {
				p = x
				ok = true
			}
			return p, ok
		}()
		if !ok {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found: " + item.ItemCode})
		}
		var uom models.UomConversion
		if err := tx.First(&uom, "item_code = ? AND from_uom = ?", item.ItemCode, item.UOM).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "UOM conversion not found: " + item.ItemCode + " / " + item.UOM})
		}
		var d *models.InboundDetail
		if item.ID > 0 {
			d = byID[uint(item.ID)]
		}
		if d == nil && item.BundleProductID > 0 {
			key := fmt.Sprintf("%d|%s", item.BundleProductID, item.BundleProductCode)
			for _, cand := range byBundle[key] {
				if !used[cand.ID] && cand.ItemCode == item.ItemCode {
					d = cand
					break
				}
			}
		}
		if d == nil {
			for i := range existing {
				cand := &existing[i]
				if !used[cand.ID] && cand.BundleProductID == 0 && item.BundleProductID == 0 && cand.ItemCode == item.ItemCode && cand.Uom == item.UOM {
					d = cand
					break
				}
			}
		}
		if d == nil {
			d = &models.InboundDetail{InboundNo: header.InboundNo, InboundId: int(header.ID), CreatedBy: userID}
		}
		if d.ID > 0 {
			used[d.ID] = true
		}
		if item.BundleProductID > 0 {
			usedBundle[fmt.Sprintf("%d|%s", item.BundleProductID, item.BundleProductCode)] = true
		}
		if !policy.UseLotNo {
			item.LotNumber = header.InboundNo
		}
		qty := item.Quantity
		if len(item.SerialNumbers) > 0 {
			qty = float64(len(item.SerialNumbers))
		}
		// Scanned quantity may never be reduced.
		if b, ok := barcodeMap[d.ID]; ok && b.TotalScan > int(qty) {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Quantity update for item " + item.ItemCode + " is less than the total scanned quantity"})
		}
		for sn := range putaway[d.ID] {
			found := false
			for _, in := range item.SerialNumbers {
				if strings.TrimSpace(in) == sn {
					found = true
					break
				}
			}
			if !found {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "serial number " + sn + " already putaway and cannot be removed"})
			}
		}
		d.ItemId = product.ID
		d.ProductNumber = product.ProductNumber
		d.ItemCode = item.ItemCode
		d.Barcode = uom.Ean
		d.Quantity = qty
		d.Location = item.Location
		d.WhsCode = header.WhsCode
		d.RecDate = item.RecDate
		d.ProdDate = item.ProdDate
		d.ExpDate = item.ExpDate
		d.LotNumber = item.LotNumber
		d.Uom = item.UOM
		d.IsSerial = product.HasSerial
		d.SerialNumber = item.SerialNumber
		d.CartonNumber = item.CartonNumber
		d.CaseNumber = item.CaseNumber
		d.RefNo = item.RefNo
		d.RefId = item.RefId
		d.OwnerCode = header.OwnerCode
		d.QaStatus = item.QaStatus
		d.DivisionCode = item.DivisionCode
		d.BundleProductID = item.BundleProductID
		d.BundleProductCode = item.BundleProductCode
		d.BundleQuantity = item.BundleQuantity
		d.UpdatedBy = userID
		if d.ID == 0 {
			if err := tx.Create(d).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		} else if err := tx.Save(d).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if len(item.SerialNumbers) > 0 {
			if int(item.Quantity) != len(item.SerialNumbers) {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Jumlah serial number tidak sesuai quantity untuk item " + item.ItemCode})
			}
			seenSN := map[string]bool{}
			for _, sn := range item.SerialNumbers {
				sn = strings.TrimSpace(sn)
				if sn == "" || seenSN[sn] {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid or duplicate serial number: " + sn})
				}
				seenSN[sn] = true
				for _, old := range serialMap[d.ID] {
					if old.SerialNumber == sn {
						continue
					}
				}
				// var other models.InboundSerial
				// if e := tx.Where("serial_number = ? AND inbound_detail_id != ?", sn, d.ID).First(&other).Error; e == nil {
				// 	tx.Rollback()
				// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "serial number is already in use: " + sn})
				// } else if !errors.Is(e, gorm.ErrRecordNotFound) {
				// 	tx.Rollback()
				// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": e.Error()})
				// }
			}
			if err := tx.Where("inbound_detail_id = ?", d.ID).Delete(&models.InboundSerial{}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			for _, sn := range item.SerialNumbers {
				if err := tx.Create(&models.InboundSerial{InboundId: int(header.ID), InboundDetailId: int(d.ID), SerialNumber: strings.TrimSpace(sn), CreatedBy: userID, UpdatedBy: userID}).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}
	}
	// Remove old details no longer present. Bundle details are treated as one logical group.
	for _, d := range existing {
		if used[d.ID] {
			continue
		}
		if d.BundleProductID > 0 && d.BundleProductCode != "" {
			key := fmt.Sprintf("%d|%s", d.BundleProductID, d.BundleProductCode)
			if usedBundle[key] {
				if b, ok := barcodeMap[d.ID]; ok && b.TotalScan > 0 {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Bundle " + d.BundleProductCode + " contains scanned item and cannot remove component " + d.ItemCode})
				}
				if len(putaway[d.ID]) > 0 {
					tx.Rollback()
					return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Bundle " + d.BundleProductCode + " contains putaway item and cannot remove component " + d.ItemCode})
				}
				if err := tx.Where("inbound_detail_id = ?", d.ID).Delete(&models.InboundSerial{}).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
				if err := tx.Unscoped().Delete(d).Error; err != nil {
					tx.Rollback()
					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
				continue
			}
		} else {
			if b, ok := barcodeMap[d.ID]; ok && b.TotalScan > 0 {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + d.ItemCode + " is already scanned and cannot be deleted"})
			}
			if len(putaway[d.ID]) > 0 {
				tx.Rollback()
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + d.ItemCode + " is already putaway and cannot be deleted"})
			}
			if err := tx.Where("inbound_detail_id = ?", d.ID).Delete(&models.InboundSerial{}).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			if err := tx.Unscoped().Delete(d).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Update Inbound " + header.InboundNo + " successfully"})
}

func (c *InboundController) DeleteItem(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var detail models.InboundDetail
	if err := tx.First(&detail, "id = ?", id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var header models.InboundHeader
	if err := tx.First(&header, "inbound_no = ?", detail.InboundNo).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if header.Status != "open" {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inbound " + detail.InboundNo + " is not open", "message": "Inbound not open"})
	}
	var targets []models.InboundDetail
	if detail.BundleProductID > 0 && detail.BundleProductCode != "" {
		if err := tx.Where("inbound_no = ? AND bundle_product_id = ? AND bundle_product_code = ?", detail.InboundNo, detail.BundleProductID, detail.BundleProductCode).Find(&targets).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		targets = []models.InboundDetail{detail}
	}
	for _, d := range targets {
		var b repositories.ResulInboundBarcodeByOutboundDetailID
		var err error
		b, err = repositories.NewInboundRepository(tx).GetInboundBarcodeByOutboundDetailID(uint(d.ID))
		if err == nil && b.ItemID == d.ItemId {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Bundle/item " + d.ItemCode + " is already scanned", "message": "Item already scanned"})
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var stock int64
		if err := tx.Model(&models.InboundBarcode{}).Where("inbound_detail_id = ? AND status = ?", d.ID, "in stock").Count(&stock).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if stock > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item " + d.ItemCode + " is already putaway"})
		}
	}
	for _, d := range targets {
		if err := tx.Unscoped().Where("inbound_detail_id = ?", d.ID).Delete(&models.InboundSerial{}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if err := tx.Unscoped().Delete(&d).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if detail.BundleProductID > 0 && detail.BundleProductCode != "" {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Bundle " + detail.BundleProductCode + " deleted successfully"})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item deleted successfully"})
}

const furunoInboundSheetName = "Receive Item Detail"

type FurunoInboundUploadResponse struct {
	Success          bool                           `json:"success"`
	Message          string                         `json:"message"`
	TotalRows        int                            `json:"total_rows"`
	ProcessedRows    int                            `json:"processed_rows"`
	SuccessCount     int                            `json:"success_count"`
	FailedCount      int                            `json:"failed_count"`
	InboundNumbers   []string                       `json:"inbound_numbers,omitempty"`
	SkippedReceipts  []FurunoInboundSkippedReceipt  `json:"skipped_receipts,omitempty"`
	ValidationErrors []FurunoInboundValidationError `json:"validation_errors,omitempty"`
	Errors           []FurunoInboundExcelRowError   `json:"errors,omitempty"`
}

type FurunoInboundSkippedReceipt struct {
	ReceiptID string `json:"receipt_id"`
	Reason    string `json:"reason"`
}

type FurunoInboundValidationError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type FurunoInboundExcelRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ============================================================================
// INTERNAL DATA STRUCT
// ============================================================================

type FurunoInboundRow struct {
	Row int

	ReceiptID    string
	InboundDate  string
	ItemCode     string
	PartCode     string
	ItemName     string
	ModelName    string
	Quantity     float64
	Unit         string
	SupplierID   string
	Supplier     string
	SerialNumber string
}

// ============================================================================
// HEADER CONFIGURATION
// ============================================================================

type FurunoInboundHeaderMap map[string]int

var furunoInboundRequiredHeaders = []string{
	"receive no receive item",
	"date",
	"code#",
	"part code item",
	"item name",
	"model name",
	"quantity",
	"unit",
	"supplier id supplier receive item",
	"supplier",
	"serial/production number",
}

// ============================================================================
// MAIN HANDLER
// ============================================================================

func (c *InboundController) CreateInboundFromFurunoExcelFile(
	ctx *fiber.Ctx,
) error {

	// =========================================================================
	// 1. FORM PARAMETER
	// =========================================================================

	ownerCode := strings.TrimSpace(
		ctx.FormValue("owner_code"),
	)

	if ownerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Owner code is required",
			},
		)
	}

	whsCode := strings.TrimSpace(
		ctx.FormValue("whs_code"),
	)

	if whsCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Warehouse code is required",
			},
		)
	}

	handlingIDRaw := strings.TrimSpace(
		ctx.FormValue("handling_id"),
	)

	handlingID, err := strconv.Atoi(handlingIDRaw)

	// =========================================================================
	// 2. GET USER ID
	// =========================================================================

	userIDValue := ctx.Locals("userID")

	if userIDValue == nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "User ID not found",
			},
		)
	}

	currentUserID := 0

	switch value := userIDValue.(type) {

	case float64:
		currentUserID = int(value)

	case int:
		currentUserID = value

	case uint:
		currentUserID = int(value)

	case string:

		userID, err := strconv.Atoi(value)

		if err != nil {
			return ctx.Status(fiber.StatusUnauthorized).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Invalid user ID",
				},
			)
		}

		currentUserID = userID

	default:

		return ctx.Status(fiber.StatusUnauthorized).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Invalid user ID",
			},
		)
	}

	if currentUserID <= 0 {
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Invalid user ID",
			},
		)
	}

	// =========================================================================
	// 3. GET FILE
	// =========================================================================

	file, err := ctx.FormFile("file")

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "No file uploaded or invalid file",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "File Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	// =========================================================================
	// 4. VALIDATE EXTENSION
	// =========================================================================

	lowerName := strings.ToLower(
		file.Filename,
	)

	if !strings.HasSuffix(lowerName, ".xlsx") &&
		!strings.HasSuffix(lowerName, ".xls") {

		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Invalid file format. Only .xlsx and .xls files are allowed",
			},
		)
	}

	// =========================================================================
	// 5. VALIDATE SIZE
	// =========================================================================

	if file.Size > 10*1024*1024 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "File size exceeds maximum limit of 10MB",
			},
		)
	}

	// =========================================================================
	// 6. OPEN EXCEL
	// =========================================================================

	fileHeader, err := file.Open()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to open uploaded file",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "File Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	defer fileHeader.Close()

	excelFile, err := excelize.OpenReader(fileHeader)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to read Excel file. Please ensure the file is not corrupted",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Excel Read Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	defer excelFile.Close()

	// =========================================================================
	// 7. FIND SHEET
	// =========================================================================

	sheetName, err := findFurunoInboundSheet(
		excelFile,
		furunoInboundSheetName,
	)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: fmt.Sprintf(
					"Sheet '%s' not found. Available sheets: %s",
					furunoInboundSheetName,
					strings.Join(
						excelFile.GetSheetList(),
						", ",
					),
				),
			},
		)
	}

	// =========================================================================
	// 8. READ ROWS
	// =========================================================================

	rows, err := excelFile.GetRows(sheetName)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to read rows from Excel",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Sheet Read Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	if len(rows) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Excel file contains no data rows",
			},
		)
	}

	// =========================================================================
	// 9. BUILD HEADER MAP
	// =========================================================================

	headerMap, err := buildFurunoInboundHeaderMap(
		rows[0],
	)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: err.Error(),
			},
		)
	}

	// =========================================================================
	// 10. PARSE EXCEL
	// =========================================================================

	inboundRows, validationErrors :=
		parseFurunoInboundRows(
			rows,
			headerMap,
		)

	if len(validationErrors) > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: fmt.Sprintf(
					"Validation failed with %d error(s)",
					len(validationErrors),
				),
				TotalRows:        len(rows) - 1,
				ProcessedRows:    len(inboundRows),
				ValidationErrors: validationErrors,
			},
		)
	}

	if len(inboundRows) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			FurunoInboundUploadResponse{
				Success:       false,
				Message:       "No valid rows found",
				TotalRows:     len(rows) - 1,
				ProcessedRows: 0,
			},
		)
	}

	// =========================================================================
	// 11. GROUP BY RECEIPT ID
	// =========================================================================

	receiptMap := groupFurunoInboundRowsByReceiptID(
		inboundRows,
	)

	receiptIDs := make(
		[]string,
		0,
		len(receiptMap),
	)

	for receiptID := range receiptMap {
		receiptIDs = append(
			receiptIDs,
			receiptID,
		)
	}

	sort.Strings(receiptIDs)

	// =========================================================================
	// 12. START TRANSACTION
	// =========================================================================

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to start database transaction",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Transaction Error",
						Detail:  tx.Error.Error(),
					},
				},
			},
		)
	}

	defer func() {
		if r := recover(); r != nil {

			tx.Rollback()

			log.Printf(
				"Panic recovered in CreateInboundFromFurunoExcelFile: %v",
				r,
			)
		}
	}()

	// =========================================================================
	// 13. VALIDATE OWNER
	// =========================================================================

	var inventoryPolicy models.InventoryPolicy

	if err := tx.
		Where("owner_code = ?", ownerCode).
		First(&inventoryPolicy).
		Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Owner not found: " + ownerCode,
				},
			)
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to validate owner",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Owner Validation Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	// =========================================================================
	// 14. VALIDATE WAREHOUSE
	// =========================================================================

	var warehouse models.Warehouse

	if err := tx.
		Where("code = ?", whsCode).
		First(&warehouse).
		Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Warehouse not found: " + whsCode,
				},
			)
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to validate warehouse",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Warehouse Validation Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	// =========================================================================
	// 15. CHECK DUPLICATE RECEIPT ID
	// =========================================================================

	duplicateReceipts := make(
		map[string]string,
	)

	for _, receiptID := range receiptIDs {

		var count int64

		err := tx.
			Model(&models.InboundHeader{}).
			Where(
				"receipt_id = ?",
				receiptID,
			).
			Count(&count).
			Error

		if err != nil {

			tx.Rollback()

			return ctx.Status(fiber.StatusInternalServerError).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Failed to check duplicate receipt ID",
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     0,
							Message: "Duplicate Check Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		if count > 0 {
			duplicateReceipts[receiptID] =
				fmt.Sprintf(
					"Receipt ID '%s' already exists",
					receiptID,
				)
		}
	}

	// =========================================================================
	// 16. FILTER VALID RECEIPTS
	// =========================================================================

	validReceiptIDs := make(
		[]string,
		0,
	)

	skippedReceipts := make(
		[]FurunoInboundSkippedReceipt,
		0,
	)

	for _, receiptID := range receiptIDs {

		if reason, exists :=
			duplicateReceipts[receiptID]; exists {

			skippedReceipts = append(
				skippedReceipts,
				FurunoInboundSkippedReceipt{
					ReceiptID: receiptID,
					Reason:    reason,
				},
			)

			continue
		}

		validReceiptIDs = append(
			validReceiptIDs,
			receiptID,
		)
	}

	if len(validReceiptIDs) == 0 {

		tx.Rollback()

		return ctx.Status(fiber.StatusOK).JSON(
			FurunoInboundUploadResponse{
				Success:         false,
				Message:         "No valid inbound to process. All receipt IDs already exist.",
				TotalRows:       len(rows) - 1,
				ProcessedRows:   len(inboundRows),
				SuccessCount:    0,
				FailedCount:     len(skippedReceipts),
				SkippedReceipts: skippedReceipts,
				InboundNumbers:  []string{},
			},
		)
	}

	// =========================================================================
	// 17. CREATE INBOUND
	// =========================================================================

	repo := repositories.NewInboundRepository(tx)

	var inboundNumbers []string

	totalSuccessItems := 0

	for _, receiptID := range validReceiptIDs {

		items := receiptMap[receiptID]

		if len(items) == 0 {
			continue
		}

		firstItem := items[0]

		// =====================================================================
		// VALIDATE SUPPLIER
		// =====================================================================

		supplierName := strings.TrimSpace(
			firstItem.Supplier,
		)

		if supplierName == "" {

			tx.Rollback()

			return ctx.Status(fiber.StatusBadRequest).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: fmt.Sprintf(
						"Supplier is required for receipt %s",
						receiptID,
					),
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Supplier Validation Error",
							Detail:  "Supplier from Excel is empty",
						},
					},
				},
			)
		}

		var supplier models.Supplier

		if err := tx.
			Where(
				"LOWER(LTRIM(RTRIM(supplier_name))) = LOWER(LTRIM(RTRIM(?)))",
				supplierName,
			).
			Where("owner_code = ?", ownerCode).
			Where("is_active = ?", true).
			First(&supplier).
			Error; err != nil {

			tx.Rollback()

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.Status(fiber.StatusBadRequest).JSON(
					FurunoInboundUploadResponse{
						Success: false,
						Message: fmt.Sprintf(
							"Supplier not found or inactive: %s",
							supplierName,
						),
						Errors: []FurunoInboundExcelRowError{
							{
								Row:     firstItem.Row,
								Message: "Supplier Not Found",
								Detail: fmt.Sprintf(
									"Supplier '%s' does not exist, is inactive, or does not belong to owner '%s'",
									supplierName,
									ownerCode,
								),
							},
						},
					},
				)
			}

			return ctx.Status(fiber.StatusInternalServerError).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Failed to validate supplier",
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Supplier Validation Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		// =====================================================================
		// GENERATE INBOUND NUMBER
		// =====================================================================

		inboundNo, err :=
			repo.GenerateInboundNo()

		if err != nil {

			tx.Rollback()

			return ctx.Status(
				fiber.StatusInternalServerError,
			).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Failed to generate inbound number",
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Generation Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		// =====================================================================
		// CURRENT TIME
		// =====================================================================

		now := time.Now()

		nowTime :=
			now.Format("15:04")

		// =====================================================================
		// INBOUND HEADER
		// =====================================================================

		inboundHeader := models.InboundHeader{

			InboundNo: inboundNo,

			OwnerCode: ownerCode,

			WhsCode: whsCode,

			ReceiptID: receiptID,

			SupplierId: int(supplier.ID),
			Supplier:   supplier.SupplierCode,

			Status: "open",

			RawStatus: "DRAFT",

			DraftTime: now,

			InboundDate: firstItem.InboundDate,

			Type: "FURUNO",

			Remarks: fmt.Sprintf(
				"FURUNO | Receive No: %s",
				receiptID,
			),

			Integration: false,

			ArrivalTime: nowTime,

			CreatedBy: currentUserID,

			UpdatedBy: currentUserID,
		}

		// =====================================================================
		// INSERT HEADER
		// =====================================================================

		if err := tx.
			Create(&inboundHeader).
			Error; err != nil {

			tx.Rollback()

			return ctx.Status(
				fiber.StatusInternalServerError,
			).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: fmt.Sprintf(
						"Failed to create inbound header for receipt %s",
						receiptID,
					),
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Header Insert Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		// =====================================================================
		// INSERT REFERENCE
		// =====================================================================

		inboundReference := models.InboundReference{
			InboundId: uint(inboundHeader.ID),
			RefNo:     receiptID,
		}

		if err := tx.
			Create(&inboundReference).
			Error; err != nil {

			tx.Rollback()

			return ctx.Status(
				fiber.StatusInternalServerError,
			).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: fmt.Sprintf(
						"Failed to insert inbound reference for receipt %s",
						receiptID,
					),
					Errors: []FurunoInboundExcelRowError{
						{
							Row:     firstItem.Row,
							Message: "Reference Insert Error",
							Detail:  err.Error(),
						},
					},
				},
			)
		}

		// =====================================================================
		// EXPAND BUNDLE
		// =====================================================================

		expandedItems, err := c.expandFurunoInboundItems(tx, items)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "Failed to expand Furuno inbound items",
					Errors: []FurunoInboundExcelRowError{{
						Row:     firstItem.Row,
						Message: "Bundle Expansion Error",
						Detail:  err.Error(),
					}},
				},
			)
		}

		if len(expandedItems) == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(
				FurunoInboundUploadResponse{
					Success: false,
					Message: "No items after bundle expansion",
				},
			)
		}

		// =====================================================================
		// INSERT EXPANDED DETAILS
		// =====================================================================

		for _, item := range expandedItems {

			// ================================================================
			// SOURCE PRODUCT / COMPONENT PRODUCT
			// ================================================================

			var product models.Product

			if err := tx.
				Where(
					"item_code = ?",
					strings.TrimSpace(item.ItemCode),
				).
				First(&product).
				Error; err != nil {

				tx.Rollback()

				if errors.Is(
					err,
					gorm.ErrRecordNotFound,
				) {

					return ctx.Status(
						fiber.StatusNotFound,
					).JSON(
						FurunoInboundUploadResponse{
							Success: false,
							Message: fmt.Sprintf(
								"Product not found: %s",
								item.ItemCode,
							),
							Errors: []FurunoInboundExcelRowError{
								{
									Row:     item.Row,
									Message: "Product Not Found",
									Detail: fmt.Sprintf(
										"SKU: %s",
										item.ItemCode,
									),
								},
							},
						},
					)
				}

				return ctx.Status(
					fiber.StatusInternalServerError,
				).JSON(
					FurunoInboundUploadResponse{
						Success: false,
						Message: "Failed to lookup product",
						Errors: []FurunoInboundExcelRowError{
							{
								Row:     item.Row,
								Message: "Product Lookup Error",
								Detail:  err.Error(),
							},
						},
					},
				)
			}

			// ================================================================
			// UOM CONVERSION
			// ================================================================

			var uomConversion models.UomConversion

			if err := tx.
				Where(
					"item_code = ? AND factor = 1",
					product.ItemCode,
				).
				First(&uomConversion).
				Error; err != nil {

				if err2 := tx.
					Where(
						"item_code = ?",
						product.ItemCode,
					).
					First(&uomConversion).
					Error; err2 != nil {

					tx.Rollback()

					return ctx.Status(
						fiber.StatusNotFound,
					).JSON(
						FurunoInboundUploadResponse{
							Success: false,
							Message: fmt.Sprintf(
								"UOM conversion not found for SKU: %s",
								item.ItemCode,
							),
							Errors: []FurunoInboundExcelRowError{
								{
									Row:     item.Row,
									Message: "UOM Not Found",
									Detail: fmt.Sprintf(
										"SKU: %s",
										item.ItemCode,
									),
								},
							},
						},
					)
				}
			}

			// ================================================================
			// RESOLVE UOM
			// ================================================================

			detailUOM :=
				uomConversion.FromUom

			if strings.TrimSpace(item.Unit) != "" {

				var excelUOMConversion models.UomConversion

				errUOM := tx.
					Where(
						"item_code = ? AND from_uom = ?",
						product.ItemCode,
						strings.TrimSpace(
							item.Unit,
						),
					).
					First(
						&excelUOMConversion,
					).
					Error

				if errUOM == nil {

					uomConversion =
						excelUOMConversion

					detailUOM =
						excelUOMConversion.FromUom
				}
			}

			// ================================================================
			// BUNDLE REFERENCE
			// ================================================================

			bundleProductID := item.BundleProductID
			bundleProductCode := item.BundleProductCode
			bundleQuantity := item.BundleQuantity

			// ================================================================
			// INSERT DETAIL
			// ================================================================

			inboundDetail :=
				models.InboundDetail{
					OwnerCode:    ownerCode,
					WhsCode:      whsCode,
					DivisionCode: "REGULAR",
					InboundId: int(
						inboundHeader.ID,
					),
					InboundNo:     inboundNo,
					ItemId:        product.ID,
					ProductNumber: product.ProductNumber,
					ItemCode:      product.ItemCode,
					Barcode:       uomConversion.Ean,
					Quantity:      item.Quantity,
					RcvLocation:   "",
					QaStatus:      "A",
					Location:      "",
					Status:        "draft",
					RecDate:       item.InboundDate,
					Uom:           detailUOM,
					RefId: int(
						inboundReference.ID,
					),
					RefNo:        inboundReference.RefNo,
					IsSerial:     product.HasSerial,
					HandlingId:   handlingID,
					HandlingUsed: "",
					// PartCode Excel -> Remarks.
					Remarks: item.PartCode,
					// Bundle reference.
					BundleProductID:   bundleProductID,
					BundleProductCode: bundleProductCode,
					BundleQuantity:    bundleQuantity,
					CreatedBy:         currentUserID,
					UpdatedBy:         currentUserID,
				}

			// ================================================================
			// SERIAL FLAG
			// ================================================================

			serialNumbers :=
				splitFurunoInboundSerials(
					item.SerialNumber,
				)

			if len(serialNumbers) > 0 {

				inboundDetail.IsSerial = "Y"

				inboundDetail.SerialNumber = ""
			}

			// ================================================================
			// INSERT DETAIL
			// ================================================================

			if err := tx.
				Create(&inboundDetail).
				Error; err != nil {

				tx.Rollback()

				return ctx.Status(
					fiber.StatusInternalServerError,
				).JSON(
					FurunoInboundUploadResponse{
						Success: false,
						Message: fmt.Sprintf(
							"Failed to create inbound detail for SKU: %s",
							item.ItemCode,
						),
						Errors: []FurunoInboundExcelRowError{
							{
								Row:     item.Row,
								Message: "Detail Insert Error",
								Detail:  err.Error(),
							},
						},
					},
				)
			}

			// ================================================================
			// INSERT SERIAL
			// ================================================================

			for _, serialNumber := range serialNumbers {

				serialNumber =
					strings.TrimSpace(
						serialNumber,
					)

				if serialNumber == "" {
					continue
				}

				inboundSerial :=
					models.InboundSerial{

						InboundId: int(
							inboundHeader.ID,
						),

						InboundDetailId: int(
							inboundDetail.ID,
						),

						SerialNumber: serialNumber,

						CreatedBy: currentUserID,

						UpdatedBy: currentUserID,
					}

				if err := tx.
					Create(&inboundSerial).
					Error; err != nil {

					tx.Rollback()

					return ctx.Status(
						fiber.StatusInternalServerError,
					).JSON(
						FurunoInboundUploadResponse{
							Success: false,
							Message: "Failed to insert inbound serial",
							Errors: []FurunoInboundExcelRowError{
								{
									Row:     item.Row,
									Message: "Serial Insert Error",
									Detail:  err.Error(),
								},
							},
						},
					)
				}
			}

			totalSuccessItems++
		}

		// =====================================================================
		// TRANSACTION HISTORY
		// =====================================================================

		if err := helpers.InsertTransactionHistory(
			tx,
			inboundNo,
			"open",
			"INBOUND",
			fmt.Sprintf(
				"Created from FURUNO Excel - Receive No: %s",
				receiptID,
			),
			currentUserID,
		); err != nil {

			log.Printf(
				"Warning: Failed to insert transaction history for %s: %v",
				inboundNo,
				err,
			)
		}

		inboundNumbers =
			append(
				inboundNumbers,
				inboundNo,
			)
	}

	// =========================================================================
	// 18. COMMIT
	// =========================================================================

	if err := tx.Commit().Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(
			FurunoInboundUploadResponse{
				Success: false,
				Message: "Failed to commit transaction",
				Errors: []FurunoInboundExcelRowError{
					{
						Row:     0,
						Message: "Transaction Commit Error",
						Detail:  err.Error(),
					},
				},
			},
		)
	}

	// =========================================================================
	// 19. RESPONSE
	// =========================================================================

	message := fmt.Sprintf(
		"Created %d inbound(s) with %d item(s) from FURUNO Excel",
		len(inboundNumbers),
		totalSuccessItems,
	)

	if len(skippedReceipts) > 0 {

		message += fmt.Sprintf(
			". %d receipt(s) skipped because already exist",
			len(skippedReceipts),
		)
	}

	return ctx.Status(
		fiber.StatusOK,
	).JSON(
		FurunoInboundUploadResponse{

			Success: true,

			Message: message,

			TotalRows: len(rows) - 1,

			ProcessedRows: len(inboundRows),

			SuccessCount: totalSuccessItems,

			FailedCount: len(skippedReceipts),

			InboundNumbers: inboundNumbers,

			SkippedReceipts: skippedReceipts,
		},
	)
}

type FurunoInboundExpandedRow struct {
	FurunoInboundRow
	BundleProductID   int
	BundleProductCode string
	BundleQuantity    float64
}

func (c *InboundController) expandFurunoInboundItems(
	tx *gorm.DB,
	rows []FurunoInboundRow,
) ([]FurunoInboundExpandedRow, error) {
	bundleService := services.NewProductBundleService(tx)
	expanded := make([]FurunoInboundExpandedRow, 0)

	for _, row := range rows {
		var product models.Product
		if err := tx.Where("item_code = ?", strings.TrimSpace(row.ItemCode)).First(&product).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("product not found: %s", row.ItemCode)
			}
			return nil, err
		}

		if product.IsBundle != "Y" {
			expanded = append(expanded, FurunoInboundExpandedRow{FurunoInboundRow: row})
			continue
		}

		components, err := bundleService.ExpandBundle(tx, product.ID, row.Quantity)
		if err != nil {
			return nil, fmt.Errorf("failed to expand bundle %s: %w", product.ItemCode, err)
		}
		if len(components) == 0 {
			return nil, fmt.Errorf("bundle %s has no components", product.ItemCode)
		}

		for _, component := range components {
			child := row
			child.ItemCode = component.ItemCode
			child.Quantity = component.Qty
			child.Unit = component.UOM
			// Excel PartCode remains unchanged and is stored only as Remarks.
			expanded = append(expanded, FurunoInboundExpandedRow{
				FurunoInboundRow:  child,
				BundleProductID:   int(product.ID),
				BundleProductCode: product.ItemCode,
				BundleQuantity:    row.Quantity,
			})
		}
	}

	return expanded, nil
}

func buildFurunoInboundHeaderMap(
	headerRow []string,
) (FurunoInboundHeaderMap, error) {

	headerMap :=
		make(FurunoInboundHeaderMap)

	for index, header := range headerRow {

		normalized :=
			normalizeFurunoInboundHeader(
				header,
			)

		if normalized == "" {
			continue
		}

		if _, exists :=
			headerMap[normalized]; exists {

			return nil, fmt.Errorf(
				"duplicate Excel header found: '%s'",
				header,
			)
		}

		headerMap[normalized] =
			index
	}

	// Validate required headers.

	for _, required := range furunoInboundRequiredHeaders {

		if _, exists :=
			headerMap[required]; !exists {

			return nil, fmt.Errorf(
				"required Excel header '%s' not found",
				required,
			)
		}
	}

	return headerMap, nil
}

func parseFurunoInboundRows(
	rows [][]string,
	headerMap FurunoInboundHeaderMap,
) (
	[]FurunoInboundRow,
	[]FurunoInboundValidationError,
) {

	var result []FurunoInboundRow

	var errs []FurunoInboundValidationError

	// Row 0 = header.

	for i := 1; i < len(rows); i++ {

		row := rows[i]

		rowNum := i + 1

		// =====================================================================
		// EMPTY ROW
		// =====================================================================

		if furunoInboundRowIsEmpty(row) {
			continue
		}

		// =====================================================================
		// RECEIVE NO
		// =====================================================================

		receiptID :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"receive no receive item",
				),
			)

		if receiptID == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:     rowNum,
					Field:   "Receive No Receive Item",
					Message: "Receive No cannot be empty",
				},
			)

			continue
		}

		// =====================================================================
		// ITEM CODE
		// =====================================================================

		itemCodeRaw :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"code#",
				),
			)

		if itemCodeRaw == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:     rowNum,
					Field:   "Code#",
					Message: "Code# cannot be empty",
				},
			)

			continue
		}

		itemCode :=
			cleanFurunoInboundItemCode(
				itemCodeRaw,
			)

		// =====================================================================
		// PART CODE
		// =====================================================================

		partCode :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"part code item",
				),
			)

		// =====================================================================
		// ITEM NAME
		// =====================================================================

		itemName :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"item name",
				),
			)

		// =====================================================================
		// MODEL NAME
		// =====================================================================

		modelName :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"model name",
				),
			)

		// =====================================================================
		// QUANTITY
		// =====================================================================

		qtyRaw :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"quantity",
				),
			)

		if qtyRaw == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:     rowNum,
					Field:   "Quantity",
					Message: "Quantity cannot be empty",
				},
			)

			continue
		}

		qty, err :=
			strconv.ParseFloat(
				qtyRaw,
				64,
			)

		if err != nil || qty <= 0 {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:   rowNum,
					Field: "Quantity",
					Message: fmt.Sprintf(
						"Invalid quantity: %s",
						qtyRaw,
					),
				},
			)

			continue
		}

		// =====================================================================
		// UNIT
		// =====================================================================

		unit :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"unit",
				),
			)

			// =====================================================================
			// DATE
			// =====================================================================

			// dateRaw :=
			// 	strings.TrimSpace(
			// 		getFurunoInboundCell(
			// 			row,
			// 			headerMap,
			// 			"date",
			// 		),
			// 	)

			// if dateRaw == "" {

			// 	errs = append(
			// 		errs,
			// 		FurunoInboundValidationError{
			// 			Row:     rowNum,
			// 			Field:   "Date",
			// 			Message: "Date cannot be empty",
			// 		},
			// 	)

			// 	continue
			// }

		nowDate := time.Now().Format("2006-01-02")
		// dateRaw := strings.TrimSpace(
		// 	getFurunoCell(
		// 		row,
		// 		headerMap,
		// 		"date",
		// 	),
		// )

		parsedDate :=
			parseFurunoInboundDate(
				nowDate,
			)

		// Hard Code Date Format: 2023-08-15
		// parsedDate :=

		if parsedDate == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:   rowNum,
					Field: "Date",
					Message: fmt.Sprintf(
						"Invalid date format: %s",
						nowDate,
					),
				},
			)

			continue
		}

		// =====================================================================
		// SUPPLIER ID
		// =====================================================================

		supplierID :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"supplier id supplier receive item",
				),
			)

		if supplierID == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:     rowNum,
					Field:   "Supplier ID Supplier Receive Item",
					Message: "Supplier ID cannot be empty",
				},
			)

			continue
		}

		// =====================================================================
		// SUPPLIER
		// =====================================================================

		supplier :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"supplier",
				),
			)

		if supplier == "" {

			errs = append(
				errs,
				FurunoInboundValidationError{
					Row:     rowNum,
					Field:   "Supplier",
					Message: "Supplier cannot be empty",
				},
			)

			continue
		}

		// =====================================================================
		// SERIAL
		// =====================================================================

		serialNumber :=
			strings.TrimSpace(
				getFurunoInboundCell(
					row,
					headerMap,
					"serial/production number",
				),
			)

		// =====================================================================
		// APPEND
		// =====================================================================

		result = append(
			result,
			FurunoInboundRow{

				Row: rowNum,

				ReceiptID: receiptID,

				InboundDate: parsedDate,

				ItemCode: itemCode,

				PartCode: partCode,

				ItemName: itemName,

				ModelName: modelName,

				Quantity: qty,

				Unit: unit,

				SupplierID: supplierID,

				Supplier: supplier,

				SerialNumber: serialNumber,
			},
		)
	}

	return result, errs
}

func groupFurunoInboundRowsByReceiptID(
	rows []FurunoInboundRow,
) map[string][]FurunoInboundRow {

	result :=
		make(
			map[string][]FurunoInboundRow,
		)

	for _, row := range rows {

		result[row.ReceiptID] =
			append(
				result[row.ReceiptID],
				row,
			)
	}

	return result
}

func getFurunoInboundCell(
	row []string,
	headerMap FurunoInboundHeaderMap,
	header string,
) string {

	index, exists :=
		headerMap[normalizeFurunoInboundHeader(
			header,
		)]

	if !exists {
		return ""
	}

	if index < 0 ||
		index >= len(row) {
		return ""
	}

	return row[index]
}

func normalizeFurunoInboundHeader(
	header string,
) string {

	header =
		strings.TrimSpace(
			header,
		)

	header =
		strings.ToLower(
			header,
		)

	header =
		strings.Join(
			strings.Fields(header),
			" ",
		)

	return header
}

func furunoInboundRowIsEmpty(
	row []string,
) bool {

	for _, cell := range row {

		if strings.TrimSpace(cell) != "" {
			return false
		}
	}

	return true
}

func cleanFurunoInboundItemCode(
	raw string,
) string {

	raw =
		strings.TrimSpace(
			raw,
		)

	if idx :=
		strings.Index(
			raw,
			".",
		); idx != -1 {

		decimal :=
			raw[idx+1:]

		allZero := true

		for _, ch := range decimal {

			if ch != '0' {

				allZero = false

				break
			}
		}

		if allZero {
			return raw[:idx]
		}
	}

	return raw
}

func parseFurunoInboundDate(
	raw string,
) string {

	raw =
		strings.TrimSpace(
			raw,
		)

	if raw == "" {
		return ""
	}

	// =====================================================================
	// Excel serial number
	// =====================================================================

	if days, err :=
		strconv.ParseFloat(
			raw,
			64,
		); err == nil &&
		days > 40000 {

		excelEpoch :=
			time.Date(
				1899,
				12,
				30,
				0,
				0,
				0,
				0,
				time.UTC,
			)

		date :=
			excelEpoch.Add(
				time.Duration(
					days,
				) * 24 * time.Hour,
			)

		return date.Format(
			"2006-01-02",
		)
	}

	// =====================================================================
	// STRING FORMATS
	// =====================================================================

	dateFormats := []string{

		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",

		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",

		"01/02/2006",

		"2/1/2006",
		"1/2/2006",

		"02-01-06",
		"02-01-2006",

		"2-Jan-06",
		"02-Jan-06",
		"2-January-06",
		"02-January-06",

		"2-Jan-2006",
		"02-Jan-2006",
		"2-January-2006",
		"02-January-2006",

		"2 Jan 2006",
		"02 Jan 2006",
	}

	for _, format := range dateFormats {

		if t, err :=
			time.Parse(
				format,
				raw,
			); err == nil {

			return t.Format(
				"2006-01-02",
			)
		}
	}

	return ""
}

func splitFurunoInboundSerials(
	value string,
) []string {

	value =
		strings.TrimSpace(
			value,
		)

	if value == "" {
		return nil
	}

	parts :=
		strings.Split(
			value,
			",",
		)

	result :=
		make(
			[]string,
			0,
			len(parts),
		)

	for _, part := range parts {

		part =
			strings.TrimSpace(
				part,
			)

		if part == "" {
			continue
		}

		result =
			append(
				result,
				part,
			)
	}

	return result
}

func findFurunoInboundSheet(
	file *excelize.File,
	expected string,
) (string, error) {

	expectedNormalized :=
		normalizeFurunoInboundHeader(
			expected,
		)

	for _, sheet := range file.GetSheetList() {

		if normalizeFurunoInboundHeader(sheet) ==
			expectedNormalized {

			return sheet, nil
		}
	}

	return "",
		fmt.Errorf(
			"sheet '%s' not found",
			expected,
		)
}
