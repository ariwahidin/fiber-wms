package controllers

import (
	"fmt"
	"strings"
	"time"

	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type sjPackageSelection struct {
	OutboundNo    string   `json:"outbound_no"`
	OrderDetailID string   `json:"order_detail_id"`
	PackageIDs    []uint64 `json:"package_ids"`
}
type sjPayload struct {
	SJDate          string               `json:"sj_date"`
	Driver          string               `json:"driver"`
	DriverPhone     string               `json:"driver_phone"`
	TruckType       string               `json:"truck_type"`
	TruckSize       string               `json:"truck_size"`
	TruckNo         string               `json:"truck_no"`
	TransporterCode string               `json:"transporter_code"`
	TransporterName string               `json:"transporter_name"`
	Remarks         string               `json:"remarks"`
	Items           []sjPackageSelection `json:"items"`
}

func sjUserID(c *fiber.Ctx) int {
	v, ok := c.Locals("userID").(float64)
	if !ok {
		return 0
	}
	return int(v)
}
func sjID(v interface{}) string { return fmt.Sprint(v) }

// nextSJNo must be called inside the same SQL Server transaction used to insert the header.
// The unique SJNo index plus UPDLOCK/HOLDLOCK serializes the daily sequence allocation.
func nextSJNo(tx *gorm.DB, now time.Time) (string, error) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	date := now.In(loc).Format("20060102")
	prefix := "YIID-PILOT-" + date + "-"
	var last string
	err = tx.Raw("SELECT TOP 1 sj_no FROM surat_jalan_headers WITH (UPDLOCK, HOLDLOCK) WHERE sj_no >= ? AND sj_no < ? ORDER BY sj_no DESC", prefix, prefix+"~").Scan(&last).Error
	if err != nil {
		return "", err
	}
	next := 1
	if last != "" {
		var n int
		if _, scanErr := fmt.Sscanf(last, prefix+"%d", &n); scanErr == nil {
			next = n + 1
		}
	}
	return fmt.Sprintf("%s%04d", prefix, next), nil
}
func sjError(c *fiber.Ctx, code int, msg string, err error) error {
	out := fiber.Map{"success": false, "message": msg}
	if err != nil {
		out["error"] = err.Error()
	}
	return c.Status(code).JSON(out)
}

// GetSuratJalanListByOrderNo returns all SJ documents belonging to an SPK.
func (h *ShippingController) GetSuratJalanListByOrderNo(c *fiber.Ctx) error {
	orderNo := strings.TrimSpace(c.Params("order_no"))
	var order models.OrderHeader
	if err := h.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return sjError(c, 404, "SPK not found", err)
	}
	var rows []models.SuratJalanHeader
	if err := h.DB.Where("order_no = ?", orderNo).Order("id DESC").Find(&rows).Error; err != nil {
		return sjError(c, 500, "Failed to get SJ list", err)
	}
	return c.JSON(fiber.Map{"success": true, "data": rows})
}

// GetAvailablePackages lists package rows from outbound_barcodes not reserved by any active SJ.
func (h *ShippingController) GetAvailablePackages(c *fiber.Ctx) error {
	orderNo := strings.TrimSpace(c.Params("order_no"))
	var order models.OrderHeader
	if err := h.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return sjError(c, 404, "SPK not found", err)
	}
	var details []models.OrderDetail
	if err := h.DB.Where("order_id = ?", order.ID).Find(&details).Error; err != nil {
		return sjError(c, 500, "Failed to get SPK DO list", err)
	}
	outboundNos := make([]string, 0, len(details))
	validDO := map[string]bool{}
	for _, d := range details {
		outboundNos = append(outboundNos, d.OutboundNo)
		validDO[d.OutboundNo] = true
	}
	if len(outboundNos) == 0 {
		return c.JSON(fiber.Map{"success": true, "data": []interface{}{}})
	}
	reserved := h.DB.Table("surat_jalan_packages AS sjp").Select("sjp.outbound_barcode_id").Where("sjp.is_active = ?", true)
	if sjNo := strings.TrimSpace(c.Query("sj_no")); sjNo != "" {
		reserved = reserved.Joins("JOIN surat_jalan_details AS sjd ON sjd.id = sjp.sj_detail_id").Joins("JOIN surat_jalan_headers AS sjh ON sjh.id = sjd.sj_id").Where("sjh.sj_no <> ?", sjNo)
	}
	var packages []models.OutboundBarcode
	err := h.DB.Preload("Product").
		Preload("OutboundHeader").
		Where("outbound_no IN ?", outboundNos).
		Where("id NOT IN (?)", reserved).Order("outbound_no, carton_code, id").Find(&packages).Error
	if err != nil {
		return sjError(c, 500, "Failed to get available packages", err)
	}
	return c.JSON(fiber.Map{"success": true, "data": packages})
}

// CreateSuratJalan creates a draft and reserves all selected physical package records atomically.
func (h *ShippingController) CreateSuratJalan(c *fiber.Ctx) error {
	orderNo := strings.TrimSpace(c.Params("order_no"))
	var p sjPayload
	if err := c.BodyParser(&p); err != nil {
		return sjError(c, 400, "Invalid payload", err)
	}
	return h.saveSJ(c, orderNo, "", p)
}

func (h *ShippingController) UpdateSuratJalan(c *fiber.Ctx) error {
	var p sjPayload
	if err := c.BodyParser(&p); err != nil {
		return sjError(c, 400, "Invalid payload", err)
	}
	var header models.SuratJalanHeader
	if err := h.DB.Where("sj_no = ?", c.Params("sj_no")).First(&header).Error; err != nil {
		return sjError(c, 404, "SJ not found", err)
	}
	return h.saveSJ(c, header.OrderNo, header.SJNo, p)
}

func (h *ShippingController) saveSJ(c *fiber.Ctx, orderNo, sjNo string, p sjPayload) error {
	if len(p.Items) == 0 {
		return sjError(c, 400, "Select at least one DO and package", nil)
	}
	tx := h.DB.Begin()
	if tx.Error != nil {
		return sjError(c, 500, "Could not start transaction", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var order models.OrderHeader
	if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		tx.Rollback()
		return sjError(c, 404, "SPK not found", err)
	}
	var header models.SuratJalanHeader
	if sjNo != "" {
		if err := tx.Where("sj_no = ?", sjNo).First(&header).Error; err != nil {
			tx.Rollback()
			return sjError(c, 404, "SJ not found", err)
		}
		if header.Status != "DRAFT" {
			tx.Rollback()
			return sjError(c, 500, "Only DRAFT SJ can be edited", nil)
		}
		if err := tx.Unscoped().Where("sj_detail_id IN (SELECT id FROM surat_jalan_details WHERE sj_id = ?)", header.ID).Delete(&models.SuratJalanPackage{}).Error; err != nil {
			tx.Rollback()
			return sjError(c, 500, "Failed to replace package allocations", err)
		}
		if err := tx.Unscoped().Where("sj_id = ?", header.ID).Delete(&models.SuratJalanDetail{}).Error; err != nil {
			tx.Rollback()
			return sjError(c, 500, "Failed to replace SJ details", err)
		}
	} else {
		sjNoGenerated, genErr := nextSJNo(tx, time.Now())
		if genErr != nil {
			tx.Rollback()
			return sjError(c, 500, "Failed to generate SJ number", genErr)
		}
		header = models.SuratJalanHeader{SJNo: sjNoGenerated, OrderID: sjID(order.ID), OrderNo: order.OrderNo, Status: "DRAFT", CreatedBy: sjUserID(c), CreatedAt: time.Now()}
	}
	date := time.Now()
	if p.SJDate != "" {
		parsed, err := time.Parse("2006-01-02", p.SJDate)
		if err != nil {
			tx.Rollback()
			return sjError(c, 400, "sj_date must use YYYY-MM-DD", err)
		}
		date = parsed
	}
	header.SJDate = date
	header.Driver = p.Driver
	header.DriverPhone = p.DriverPhone
	header.TruckType = p.TruckType
	header.TruckSize = p.TruckSize
	header.TruckNo = p.TruckNo
	header.TransporterCode = p.TransporterCode
	header.TransporterName = p.TransporterName
	header.Remarks = p.Remarks
	header.UpdatedBy = sjUserID(c)
	header.UpdatedAt = time.Now()
	if sjNo == "" {
		if err := tx.Create(&header).Error; err != nil {
			tx.Rollback()
			return sjError(c, 500, "Failed to create SJ header", err)
		}
	} else if err := tx.Save(&header).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to update SJ header", err)
	}
	var orderDetails []models.OrderDetail
	if err := tx.Where("order_id = ?", order.ID).Find(&orderDetails).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to load SPK DOs", err)
	}
	detailByID := map[string]models.OrderDetail{}
	for _, d := range orderDetails {
		detailByID[sjID(d.ID)] = d
	}
	seen := map[uint64]bool{}
	for _, selection := range p.Items {
		detail, ok := detailByID[selection.OrderDetailID]
		if !ok || detail.OutboundNo != selection.OutboundNo {
			tx.Rollback()
			return sjError(c, 400, "Selected DO does not belong to this SPK", nil)
		}
		if len(selection.PackageIDs) == 0 {
			tx.Rollback()
			return sjError(c, 400, "Each selected DO must contain at least one package", nil)
		}
		sd := models.SuratJalanDetail{SJID: header.ID, OrderDetailID: selection.OrderDetailID, OutboundID: sjID(detail.OutboundID), OutboundNo: detail.OutboundNo, ShipmentID: detail.ShipmentID, DeliveryTo: detail.DelivTo, DeliveryToName: detail.DelivToName, DeliveryAddress: detail.DelivAddress, DeliveryCity: detail.DelivCity, CreatedAt: time.Now()}
		if err := tx.Create(&sd).Error; err != nil {
			tx.Rollback()
			return sjError(c, 500, "Failed to create SJ DO detail", err)
		}
		for _, packageID := range selection.PackageIDs {
			if seen[packageID] {
				tx.Rollback()
				return sjError(c, 400, "Duplicate package ID in request", nil)
			}
			seen[packageID] = true
			var barcode models.OutboundBarcode
			if err := tx.Where("id = ? AND outbound_no = ?", packageID, detail.OutboundNo).First(&barcode).Error; err != nil {
				tx.Rollback()
				return sjError(c, 400, "Package does not belong to selected DO", err)
			}
			allocation := models.SuratJalanPackage{SJDetailID: sd.ID, OutboundBarcodeID: uint64(barcode.ID), IsActive: true, CreatedAt: time.Now()}
			if err := tx.Create(&allocation).Error; err != nil {
				tx.Rollback()
				return sjError(c, 500, "Package already reserved by another active SJ", err)
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		return sjError(c, 500, "Failed to commit SJ", err)
	}
	return c.Status(201).JSON(fiber.Map{"success": true, "message": "SJ draft saved; packages reserved", "data": fiber.Map{"sj_no": header.SJNo, "status": header.Status}})
}

// DeleteSuratJalan permanently deletes a DRAFT SJ and its child rows.
func (h *ShippingController) DeleteSuratJalan(c *fiber.Ctx) error {
	tx := h.DB.Begin()
	if tx.Error != nil {
		return sjError(c, 500, "Could not start transaction", tx.Error)
	}
	var header models.SuratJalanHeader
	if err := tx.Where("sj_no = ?", c.Params("sj_no")).First(&header).Error; err != nil {
		tx.Rollback()
		return sjError(c, 404, "SJ not found", err)
	}
	if header.Status != "DRAFT" {
		tx.Rollback()
		return sjError(c, 409, "Only DRAFT SJ can be deleted", nil)
	}
	if err := tx.Unscoped().Where("sj_detail_id IN (SELECT id FROM surat_jalan_details WHERE sj_id = ?)", header.ID).Delete(&models.SuratJalanPackage{}).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to delete package allocations", err)
	}
	if err := tx.Unscoped().Where("sj_id = ?", header.ID).Delete(&models.SuratJalanDetail{}).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to delete SJ details", err)
	}
	if err := tx.Unscoped().Delete(&header).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to delete SJ header", err)
	}
	if err := tx.Commit().Error; err != nil {
		return sjError(c, 500, "Failed to commit deletion", err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "SJ permanently deleted"})
}

func (h *ShippingController) GetSuratJalanDetail(c *fiber.Ctx) error {
	var header models.SuratJalanHeader
	if err := h.DB.Where("sj_no = ?", c.Params("sj_no")).First(&header).Error; err != nil {
		return sjError(c, 404, "SJ not found", err)
	}
	var details []models.SuratJalanDetail
	if err := h.DB.Where("sj_id = ?", header.ID).Order("id").Find(&details).Error; err != nil {
		return sjError(c, 500, "Failed to get SJ details", err)
	}
	type row struct {
		models.SuratJalanDetail
		Packages []models.OutboundBarcode `json:"packages"`
	}
	rows := make([]row, 0, len(details))
	for _, d := range details {
		var allocations []models.SuratJalanPackage
		if err := h.DB.Where("sj_detail_id = ? AND is_active = 1", d.ID).Find(&allocations).Error; err != nil {
			return sjError(c, 500, "Failed to get package allocations", err)
		}
		ids := make([]uint64, 0, len(allocations))
		for _, a := range allocations {
			ids = append(ids, a.OutboundBarcodeID)
		}
		var pkgs []models.OutboundBarcode
		if len(ids) > 0 {
			if err := h.DB.Preload("Product").Where("id IN ?", ids).Find(&pkgs).Error; err != nil {
				return sjError(c, 500, "Failed to load package rows", err)
			}
		}
		rows = append(rows, row{SuratJalanDetail: d, Packages: pkgs})
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"header": header, "items": rows}})
}

func (h *ShippingController) ReleaseSuratJalan(c *fiber.Ctx) error {
	var hrow models.SuratJalanHeader
	if err := h.DB.Where("sj_no = ?", c.Params("sj_no")).First(&hrow).Error; err != nil {
		return sjError(c, 404, "SJ not found", err)
	}
	if hrow.Status != "DRAFT" {
		return sjError(c, 500, "Only DRAFT SJ can be released", nil)
	}
	res := h.DB.Model(&models.SuratJalanHeader{}).Where("id = ? AND status = ?", hrow.ID, "DRAFT").Updates(map[string]interface{}{"status": "RELEASED", "updated_at": time.Now(), "updated_by": sjUserID(c)})
	if res.Error != nil {
		return sjError(c, 500, "Failed to release SJ", res.Error)
	}
	if res.RowsAffected == 0 {
		return sjError(c, 500, "SJ status changed; reload and retry", nil)
	}
	return c.JSON(fiber.Map{"success": true, "message": "SJ released"})
}

func (h *ShippingController) CancelSuratJalan(c *fiber.Ctx) error {
	tx := h.DB.Begin()
	var header models.SuratJalanHeader
	if err := tx.Where("sj_no = ?", c.Params("sj_no")).First(&header).Error; err != nil {
		tx.Rollback()
		return sjError(c, 404, "SJ not found", err)
	}
	if header.Status == "DELIVERED" {
		tx.Rollback()
		return sjError(c, 500, "Delivered SJ requires a reversal workflow", nil)
	}
	if header.Status == "CANCELLED" {
		tx.Rollback()
		return c.JSON(fiber.Map{"success": true, "message": "SJ already cancelled"})
	}
	if err := tx.Unscoped().Where("sj_detail_id IN (SELECT id FROM surat_jalan_details WHERE sj_id = ?)", header.ID).Delete(&models.SuratJalanPackage{}).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to release package reservations", err)
	}
	if err := tx.Model(&models.SuratJalanHeader{}).Where("id = ?", header.ID).Updates(map[string]interface{}{"status": "CANCELLED", "updated_at": time.Now(), "updated_by": sjUserID(c)}).Error; err != nil {
		tx.Rollback()
		return sjError(c, 500, "Failed to cancel SJ", err)
	}
	if err := tx.Commit().Error; err != nil {
		return sjError(c, 500, "Failed to commit cancellation", err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "SJ cancelled; package reservations released"})
}

// MarkSuratJalanDelivered marks a released SJ as delivered. Allocations remain active permanently.
func (h *ShippingController) MarkSuratJalanDelivered(c *fiber.Ctx) error {
	res := h.DB.Model(&models.SuratJalanHeader{}).
		Where("sj_no = ? AND status = ?", c.Params("sj_no"), "RELEASED").
		Updates(map[string]interface{}{"status": "DELIVERED", "updated_at": time.Now(), "updated_by": sjUserID(c)})
	if res.Error != nil {
		return sjError(c, 500, "Failed to mark SJ delivered", res.Error)
	}
	if res.RowsAffected == 0 {
		return sjError(c, 500, "Only RELEASED SJ can be marked delivered", nil)
	}
	return c.JSON(fiber.Map{"success": true, "message": "SJ marked as delivered"})
}
