package controllers

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type InventoryController struct {
	DB *gorm.DB
}

func NewInventoryController(DB *gorm.DB) *InventoryController {
	return &InventoryController{DB: DB}
}

func (c *InventoryController) GetAllInventoryAvailable(ctx *fiber.Ctx) error {
	var inventories []models.Inventory

	// Query inventory dengan qty_available > 0
	if err := c.DB.
		Preload("Product").
		Where("qty_available > ?", 0).
		Order("item_code ASC, whs_code ASC, location ASC").
		Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch inventories",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"inventories": inventories,
			"total":       len(inventories),
		},
	})
}

func (c *InventoryController) GetInventory(ctx *fiber.Ctx) error {

	inventory_repo := repositories.NewInventoryRepository(c.DB)
	inventories, err := inventory_repo.GetInventory()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"inventories": inventories}})
}

func (c *InventoryController) GetInventoryPolicy(ctx *fiber.Ctx) error {
	ownerCode := ctx.Query("owner")
	if ownerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "owner_code is required"})
	}

	inventoryPolicy := models.InventoryPolicy{}
	err := c.DB.Where("owner_code = ?", ownerCode).First(&inventoryPolicy).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inventory policy not found for owner_code: " + ownerCode})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"inventory_policy": inventoryPolicy}})
}

type ReqPallet struct {
	Pallet   string `json:"pallet"`
	Location string `json:"location"`
}

type ResPallet struct {
	ItemId       int    `json:"item_id"`
	ItemCode     string `json:"item_code"`
	QtyAvailable int    `json:"qty_available"`
}

func (c *InventoryController) GetInventoryByPalletAndLocation(ctx *fiber.Ctx) error {

	scanForm := ReqPallet{}

	if err := ctx.BodyParser(&scanForm); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Failed to parse JSON" + err.Error()})
	}

	var inventories []models.Inventory
	if err := c.DB.Where("pallet = ? AND location = ? AND qty_available > 0 AND qty_allocated = 0", scanForm.Pallet, scanForm.Location).Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to find inventory" + err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Get Pallet Location", "data": fiber.Map{"inventories": inventories}})

}

type Item struct {
	InventoryID int `json:"inventory_id"`
	ItemID      int `json:"item_id"`
	Quantity    int `json:"quantity"`
}

type MovePayload struct {
	SourcePallet   string    `json:"sourcePallet"`
	SourceLocation string    `json:"sourceLocation"`
	TargetPallet   string    `json:"targetPallet"`
	TargetLocation string    `json:"targetLocation"`
	Items          []Item    `json:"items"`
	Timestamp      time.Time `json:"timestamp"`
}

// 🔹 Helper untuk update inventory existing
func (c *InventoryController) updateInventoryQuantity(ctx *fiber.Ctx, inv *models.Inventory, qty int) error {
	inv.QtyAvailable -= float64(qty)
	inv.QtyOnhand -= float64(qty)
	// inv.QtyOrigin -= qty
	inv.UpdatedBy = int(ctx.Locals("userID").(float64))
	inv.UpdatedAt = time.Now()

	if err := c.DB.Model(inv).
		Select("qty_available", "qty_onhand", "qty_origin", "updated_by", "updated_at").
		Where("id = ?", inv.ID).
		Updates(inv).Error; err != nil {
		return err
	}
	return nil
}

// 🔹 Helper untuk create inventory baru (target pallet)
func (c *InventoryController) createNewInventory(ctx *fiber.Ctx, oldInv *models.Inventory, targetPallet, targetLocation string, itemID, qty int) error {
	newInventory := models.Inventory{
		InboundDetailId: oldInv.InboundDetailId,
		RecDate:         oldInv.RecDate,
		ItemId:          uint(itemID),
		ItemCode:        oldInv.ItemCode,
		WhsCode:         oldInv.WhsCode,
		DivisionCode:    oldInv.DivisionCode,
		InboundID:       oldInv.InboundID,
		OwnerCode:       oldInv.OwnerCode,
		Pallet:          targetPallet,
		Location:        targetLocation,
		QaStatus:        oldInv.QaStatus,
		// QtyOrigin:       qty,
		QtyOnhand:    float64(qty),
		QtyAvailable: float64(qty),
		QtyAllocated: 0,
		Trans:        "move item",
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&newInventory).Error; err != nil {
		return err
	}
	return nil
}

// 🔹 Function utama untuk move item
func (c *InventoryController) MoveItem(ctx *fiber.Ctx) error {
	movePayload := MovePayload{}
	if err := ctx.BodyParser(&movePayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Failed to parse JSON: " + err.Error()})
	}

	if movePayload.SourcePallet == "" || movePayload.SourceLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Source pallet and location are required"})
	}

	if movePayload.TargetPallet == "" || movePayload.TargetLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Target pallet and location are required"})
	}

	if movePayload.SourceLocation == movePayload.TargetLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Source and target locations cannot be the same"})
	}

	for _, item := range movePayload.Items {
		// cari inventory lama
		var oldInventory models.Inventory
		if err := c.DB.Where("id = ?", item.InventoryID).First(&oldInventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to find source inventory: " + err.Error()})
		}

		// validasi stock cukup
		if oldInventory.QtyAvailable < float64(item.Quantity) {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Insufficient quantity in source pallet"})
		}

		// update inventory lama
		if err := c.updateInventoryQuantity(ctx, &oldInventory, item.Quantity); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update source inventory: " + err.Error(),
			})
		}

		// create inventory baru di target
		if err := c.createNewInventory(ctx, &oldInventory, movePayload.TargetPallet, movePayload.TargetLocation, item.ItemID, item.Quantity); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create target inventory: " + err.Error(),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item moved successfully"})
}

// Handler untuk generate dan kirim file Excel
func (c *InventoryController) ExportExcel(ctx *fiber.Ctx) error {

	inventory_repo := repositories.NewInventoryRepository(c.DB)
	inventories, err := inventory_repo.GetInventory()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Buat file Excel baru
	f := excelize.NewFile()
	sheet := "Sheet1"

	// Buat header
	f.SetCellValue(sheet, "A1", "Whs Code")
	f.SetCellValue(sheet, "B1", "Item Code")
	f.SetCellValue(sheet, "C1", "Item Name")
	f.SetCellValue(sheet, "D1", "Location")
	f.SetCellValue(sheet, "E1", "Qa Status")
	f.SetCellValue(sheet, "F1", "Qty Onhand")

	// Isi data ke dalam sheet
	for i, item := range inventories {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), item.WhsCode)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), item.ItemCode)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), item.ItemName)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), item.Location)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), item.QaStatus)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), item.QtyOnhand)
	}

	// Simpan file ke dalam response
	ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set("Content-Disposition", `attachment; filename="report.xlsx"`)

	if err := f.Write(ctx.Response().BodyWriter()); err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Gagal generate Excel")
	}

	return nil
}

type ItemPayload struct {
	ItemCode  string `json:"item_code"`
	Location  string `json:"location"`
	OwnerCode string `json:"owner_code"`
	WhsCode   string `json:"whs_code"`
	QaStatus  string `json:"qa_status"`
}

type TransferRequest struct {
	Items       []ItemPayload `json:"items" validate:"required"`
	NewWhsCode  string        `json:"new_whs_code" validate:"required"`
	NewQaStatus string        `json:"new_qa_status" validate:"required"`
}

func (c *InventoryController) ChangeStatusInventory(ctx *fiber.Ctx) error {
	var req TransferRequest

	// Parse body JSON
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request: " + err.Error(),
		})
	}

	// Validasi
	if err := validator.New().Struct(req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request: " + err.Error(),
		})
	}

	if len(req.Items) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Items tidak boleh kosong",
		})
	}

	var updatedCount int64
	var notFoundItems []string

	tx := c.DB.Begin()

	for _, item := range req.Items {
		var inv models.Inventory

		// 1️⃣ SELECT dulu berdasarkan kombinasi kolom
		err := tx.Where("location = ? AND owner_code = ? AND item_code = ? AND whs_code = ? AND qa_status = ?",
			item.Location, item.OwnerCode, item.ItemCode, item.WhsCode, item.QaStatus).
			First(&inv).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				notFoundItems = append(notFoundItems, item.ItemCode)
				continue // skip item yang gak ada
			} else {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Gagal query item " + item.ItemCode + ": " + err.Error(),
				})
			}
		}

		fmt.Println(inv)

		// validasi dulu
		if inv.QtyAvailable == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Item " + item.ItemCode + " not found or already transferred",
				"message": "Item " + item.ItemCode + " not found or already transferred",
			})
		}

		if inv.WhsCode == req.NewWhsCode && inv.QaStatus == req.NewQaStatus {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Item " + item.ItemCode + " already has the same status",
				"message": "Item " + item.ItemCode + " already has the same status",
			})
		}

		var newInventory models.Inventory
		newInventory.OwnerCode = inv.OwnerCode
		newInventory.DivisionCode = inv.DivisionCode
		newInventory.Uom = inv.Uom
		newInventory.InboundID = inv.InboundID
		newInventory.InboundDetailId = inv.InboundDetailId
		newInventory.RecDate = inv.RecDate
		newInventory.ItemId = inv.ItemId
		newInventory.ItemCode = inv.ItemCode
		newInventory.Barcode = inv.Barcode
		newInventory.WhsCode = req.NewWhsCode
		newInventory.Pallet = inv.Pallet
		newInventory.Location = inv.Location
		newInventory.QaStatus = req.NewQaStatus
		newInventory.QtyOrigin = inv.QtyAvailable
		newInventory.QtyOnhand = inv.QtyAvailable
		newInventory.QtyAvailable = inv.QtyAvailable
		newInventory.Trans = fmt.Sprintf("change from inventory_id : %d", inv.ID)
		newInventory.IsTransfer = true
		newInventory.TransferFrom = inv.ID
		newInventory.CreatedAt = time.Now()
		newInventory.CreatedBy = int(ctx.Locals("userID").(float64))

		if err := tx.Create(&newInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var oldInventory models.Inventory
		if err := tx.Where("id = ?", inv.ID).First(&oldInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		oldInventory.QtyOrigin = oldInventory.QtyOrigin - inv.QtyAvailable
		oldInventory.QtyOnhand = oldInventory.QtyOnhand - inv.QtyAvailable
		oldInventory.QtyAvailable = oldInventory.QtyAvailable - inv.QtyAvailable
		oldInventory.UpdatedAt = time.Now()
		oldInventory.UpdatedBy = int(ctx.Locals("userID").(float64))

		if err := tx.Select("qty_origin", "qty_onhand", "qty_available", "updated_at", "updated_by").Updates(&oldInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		updatedCount++
	}

	tx.Commit()

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":        true,
		"message":        "Change status inventory successfully",
		"not_found_list": notFoundItems,
		"updated_count":  updatedCount,
	})
}

// Create - Membuat inventory policy baru
func (c *InventoryController) CreateInvetoryPolicy(ctx *fiber.Ctx) error {
	var policy models.InventoryPolicy

	if err := ctx.BodyParser(&policy); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validasi owner_code
	if policy.OwnerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Owner code wajib diisi",
		})
	}

	// Cek apakah owner_code sudah ada
	var existingPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ?", policy.OwnerCode).First(&existingPolicy).Error; err == nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Owner code sudah terdaftar",
		})
	}

	policy.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&policy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal membuat inventory policy",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Inventory policy berhasil dibuat",
		"data":    policy,
	})
}

// GetAll - Mendapatkan semua inventory policies
func (c *InventoryController) GetAllInventoryPolicy(ctx *fiber.Ctx) error {
	var policies []models.InventoryPolicy

	if err := c.DB.Find(&policies).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengambil data inventory policy",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Data inventory policy berhasil diambil",
		"data":    policies,
	})
}

// Update - Mengupdate inventory policy
func (c *InventoryController) UpdateInventoryPolicy(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var policy models.InventoryPolicy

	// Cek apakah data ada
	if err := c.DB.First(&policy, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory policy tidak ditemukan",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengambil data inventory policy",
			"error":   err.Error(),
		})
	}

	var updateData models.InventoryPolicy
	if err := ctx.BodyParser(&updateData); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validasi owner_code
	if updateData.OwnerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Owner code wajib diisi",
		})
	}

	// Cek apakah owner_code sudah digunakan oleh data lain
	var existingPolicy models.InventoryPolicy
	if err := c.DB.Where("owner_code = ? AND id != ?", updateData.OwnerCode, id).First(&existingPolicy).Error; err == nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Owner code sudah terdaftar",
		})
	}

	// Update data
	policy.OwnerCode = updateData.OwnerCode
	policy.UseLotNo = updateData.UseLotNo
	policy.UseFIFO = updateData.UseFIFO
	policy.UseFEFO = updateData.UseFEFO
	policy.UseVAS = updateData.UseVAS
	policy.UseProductionDate = updateData.UseProductionDate
	policy.UseReceiveLocation = updateData.UseReceiveLocation
	policy.ShowRecDate = updateData.ShowRecDate
	policy.RequireExpiryDate = updateData.RequireExpiryDate
	policy.RequireLotNumber = updateData.RequireLotNumber
	policy.RequireScanPickLocation = updateData.RequireScanPickLocation
	policy.AllowMixedLot = updateData.AllowMixedLot
	policy.AllowNegativeStock = updateData.AllowNegativeStock
	policy.ValidationSN = updateData.ValidationSN
	policy.RequirePickingScan = updateData.RequirePickingScan
	policy.RequirePackingScan = updateData.RequirePackingScan
	policy.PickingSingleScan = updateData.PickingSingleScan
	policy.RequireReceiveScan = updateData.RequireReceiveScan
	policy.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Save(&policy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengupdate inventory policy",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inventory policy berhasil diupdate",
		"data":    policy,
	})
}

// HardDelete - Menghapus inventory policy secara permanen
func (c *InventoryController) HardDelete(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var policy models.InventoryPolicy

	// Cek apakah data ada (termasuk yang sudah soft delete)
	if err := c.DB.Unscoped().First(&policy, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Inventory policy tidak ditemukan",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengambil data inventory policy",
			"error":   err.Error(),
		})
	}

	// Hard delete
	if err := c.DB.Unscoped().Delete(&policy).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal menghapus inventory policy secara permanen",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inventory policy berhasil dihapus secara permanen",
	})
}

//===================================================================
// BEGIN INTERNAL TRANSFER
// ==================================================================

type TransferInventoryInput struct {
	InventoryID   uint    `json:"inventory_id" validate:"required"`
	FromWhsCode   string  `json:"from_whs_code" validate:"required"`
	ToWhsCode     string  `json:"to_whs_code" validate:"required"`
	FromLocation  string  `json:"from_location" validate:"required"`
	ToLocation    string  `json:"to_location" validate:"required"`
	OldQaStatus   string  `json:"old_qa_status"`
	NewQaStatus   string  `json:"new_qa_status"`
	RecDate       string  `json:"rec_date"`
	ProdDate      string  `json:"prod_date"`
	ExpDate       string  `json:"exp_date"`
	LotNumber     string  `json:"lot_number"`
	Pallet        string  `json:"pallet"`
	QtyToTransfer float64 `json:"qty_to_transfer" validate:"required,gt=0"`
	Reason        string  `json:"reason"`
	DivisionCode  string  `json:"division_code" validate:"required"`
}

func (c *InventoryController) TransferInventory(ctx *fiber.Ctx) error {
	var input TransferInventoryInput
	movementID := uuid.NewString()

	// Parse body
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate input
	if input.InventoryID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Inventory ID is required",
		})
	}

	if input.QtyToTransfer <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Quantity to transfer must be greater than 0",
		})
	}

	if input.FromWhsCode == "" || input.ToWhsCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "From and To warehouse codes are required",
		})
	}

	if input.FromLocation == "" || input.ToLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "From and To locations are required",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Start transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get source inventory
	var sourceInventory models.Inventory
	if err := tx.Where("id = ?", input.InventoryID).First(&sourceInventory).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Source inventory not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch source inventory",
		})
	}

	// Validate warehouse and location match
	if sourceInventory.WhsCode != input.FromWhsCode {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Source warehouse mismatch. Expected: %s, Got: %s", sourceInventory.WhsCode, input.FromWhsCode),
		})
	}

	if sourceInventory.Location != input.FromLocation {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Source location mismatch. Expected: %s, Got: %s", sourceInventory.Location, input.FromLocation),
		})
	}

	// Validate sufficient quantity
	if sourceInventory.QtyAvailable < input.QtyToTransfer {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Insufficient quantity. Available: %.2f, Requested: %.2f", sourceInventory.QtyAvailable, input.QtyToTransfer),
		})
	}

	// Use existing QA status if not provided
	newQaStatus := input.NewQaStatus
	if newQaStatus == "" {
		newQaStatus = sourceInventory.QaStatus
	}

	// Check if destination inventory exists with same attributes
	var destInventory models.Inventory
	destQuery := tx.Where("whs_code = ? AND location = ? AND item_code = ? AND barcode = ? AND qa_status = ? AND lot_number = ?",
		input.ToWhsCode,
		input.ToLocation,
		sourceInventory.ItemCode,
		sourceInventory.Barcode,
		newQaStatus,
		input.LotNumber,
	)

	// Add optional filters if provided
	if input.RecDate != "" {
		destQuery = destQuery.Where("rec_date = ?", input.RecDate)
	}
	if input.ProdDate != "" {
		destQuery = destQuery.Where("prod_date = ?", input.ProdDate)
	}
	if input.ExpDate != "" {
		destQuery = destQuery.Where("exp_date = ?", input.ExpDate)
	}
	if input.Pallet != "" {
		destQuery = destQuery.Where("pallet = ?", input.Pallet)
	}

	err := destQuery.First(&destInventory).Error
	isNewDestination := err == gorm.ErrRecordNotFound

	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to check destination inventory",
		})
	}

	// Update source inventory - deduct quantity
	sourceInventory.QtyOrigin -= input.QtyToTransfer
	sourceInventory.QtyOnhand -= input.QtyToTransfer
	sourceInventory.QtyAvailable -= input.QtyToTransfer
	sourceInventory.UpdatedBy = userID

	if err := tx.Save(&sourceInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update source inventory",
		})
	}

	// Record source inventory movement
	sourceMovement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        sourceInventory.ID,
		RefType:            "TRANSFER",
		RefID:              0, // Will be updated with destination inventory ID
		ItemID:             sourceInventory.ItemId,
		ItemCode:           sourceInventory.ItemCode,
		QtyOnhandChange:    -input.QtyToTransfer,
		QtyAvailableChange: -input.QtyToTransfer,
		QtyAllocatedChange: 0,
		QtySuspendChange:   0,
		QtyShippedChange:   0,
		FromWhsCode:        input.FromWhsCode,
		ToWhsCode:          input.ToWhsCode,
		FromLocation:       input.FromLocation,
		ToLocation:         input.ToLocation,
		OldQaStatus:        sourceInventory.QaStatus,
		NewQaStatus:        newQaStatus,
		Reason:             input.Reason,
		CreatedBy:          userID,
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(&sourceMovement).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to record source movement",
		})
	}

	var destInventoryID uint

	if input.Pallet == sourceInventory.Pallet {
		input.Pallet = input.ToLocation
	}

	if isNewDestination {
		// Create new destination inventory
		newInventory := models.Inventory{
			OwnerCode:       sourceInventory.OwnerCode,
			WhsCode:         input.ToWhsCode,
			InboundID:       sourceInventory.InboundID,
			InboundDetailId: sourceInventory.InboundDetailId,
			DivisionCode:    input.DivisionCode,
			RecDate:         input.RecDate,
			ProdDate:        input.ProdDate,
			ExpDate:         input.ExpDate,
			LotNumber:       input.LotNumber,
			Pallet:          input.Pallet,
			Location:        input.ToLocation,
			ItemId:          sourceInventory.ItemId,
			ItemCode:        sourceInventory.ItemCode,
			Barcode:         sourceInventory.Barcode,
			QaStatus:        newQaStatus,
			Uom:             sourceInventory.Uom,
			QtyOrigin:       input.QtyToTransfer,
			QtyOnhand:       input.QtyToTransfer,
			QtyAvailable:    input.QtyToTransfer,
			QtyAllocated:    0,
			QtySuspend:      0,
			QtyShipped:      0,
			Trans:           "TRANSFER",
			IsTransfer:      true,
			TransferFrom:    sourceInventory.ID,
			CreatedBy:       userID,
			UpdatedBy:       userID,
		}

		if err := tx.Create(&newInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to create destination inventory",
			})
		}

		destInventoryID = newInventory.ID

		// Record destination inventory movement
		destMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        newInventory.ID,
			RefType:            "TRANSFER",
			RefID:              sourceInventory.ID,
			ItemID:             sourceInventory.ItemId,
			ItemCode:           sourceInventory.ItemCode,
			QtyOnhandChange:    input.QtyToTransfer,
			QtyAvailableChange: input.QtyToTransfer,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			FromWhsCode:        input.FromWhsCode,
			ToWhsCode:          input.ToWhsCode,
			FromLocation:       input.FromLocation,
			ToLocation:         input.ToLocation,
			OldQaStatus:        sourceInventory.QaStatus,
			NewQaStatus:        newQaStatus,
			Reason:             input.Reason,
			CreatedBy:          userID,
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&destMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record destination movement",
			})
		}

	} else {
		// Update existing destination inventory
		destInventory.QtyOnhand += input.QtyToTransfer
		destInventory.QtyAvailable += input.QtyToTransfer
		destInventory.UpdatedBy = userID

		if err := tx.Save(&destInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to update destination inventory",
			})
		}

		destInventoryID = destInventory.ID

		// Record destination inventory movement
		destMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        destInventory.ID,
			RefType:            "TRANSFER",
			RefID:              sourceInventory.ID,
			ItemID:             sourceInventory.ItemId,
			ItemCode:           sourceInventory.ItemCode,
			QtyOnhandChange:    input.QtyToTransfer,
			QtyAvailableChange: input.QtyToTransfer,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			FromWhsCode:        input.FromWhsCode,
			ToWhsCode:          input.ToWhsCode,
			FromLocation:       input.FromLocation,
			ToLocation:         input.ToLocation,
			OldQaStatus:        sourceInventory.QaStatus,
			NewQaStatus:        newQaStatus,
			Reason:             input.Reason,
			CreatedBy:          userID,
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&destMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record destination movement",
			})
		}
	}

	// Update source movement with destination ref
	sourceMovement.RefID = destInventoryID
	if err := tx.Save(&sourceMovement).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to update source movement reference",
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to commit transaction",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Successfully transferred %.2f units from %s to %s", input.QtyToTransfer, input.FromLocation, input.ToLocation),
		"data": fiber.Map{
			"source_inventory_id":      sourceInventory.ID,
			"destination_inventory_id": destInventoryID,
			"quantity_transferred":     input.QtyToTransfer,
			"is_new_destination":       isNewDestination,
		},
	})
}

//===================================================================
// END INTERNAL TRANSFER
//===================================================================

//===================================================================
// BEGIN GET INVENTORY GROUPED
//===================================================================

// GetAllInventoryAvailableGrouped returns inventory grouped by location
func (c *InventoryController) GetAllInventoryAvailableGrouped(ctx *fiber.Ctx) error {
	type InventoryGrouped struct {
		Location          string  `json:"location"`
		ItemCode          string  `json:"item_code"`
		ItemName          string  `json:"item_name"`
		Barcode           string  `json:"barcode"`
		Category          string  `json:"category"`
		Group             string  `json:"group"`
		QaStatus          string  `json:"qa_status"`
		Uom               string  `json:"uom"`
		TotalQtyAvailable float64 `json:"total_qty_available"`
		TotalQtyOnhand    float64 `json:"total_qty_onhand"`
		TotalQtyAllocated float64 `json:"total_qty_allocated"`
		InventoryCount    int     `json:"inventory_count"`
	}

	// Get query parameters for filtering
	location := ctx.Query("location")
	category := ctx.Query("category")
	group := ctx.Query("group")
	qaStatus := ctx.Query("qa_status")
	search := ctx.Query("search")

	// Build query
	query := c.DB.Table("inventories").
		Select(`
			inventories.location,
			inventories.item_code,
			products.item_name as item_name,
			inventories.barcode,
			products.category,
			products.[group],
			inventories.qa_status,
			inventories.uom,
			SUM(inventories.qty_available) as total_qty_available,
			SUM(inventories.qty_onhand) as total_qty_onhand,
			SUM(inventories.qty_allocated) as total_qty_allocated,
			COUNT(*) as inventory_count
		`).
		Joins("LEFT JOIN products ON inventories.item_id = products.id").
		Where("inventories.deleted_at IS NULL").
		Where("inventories.qty_onhand > ?", 0)

	// Apply filters
	if location != "" {
		query = query.Where("inventories.location = ?", location)
	}
	if category != "" {
		query = query.Where("products.category = ?", category)
	}
	if group != "" {
		query = query.Where("products.[group] = ?", group)
	}
	if qaStatus != "" {
		query = query.Where("inventories.qa_status = ?", qaStatus)
	}
	if search != "" {
		query = query.Where(
			"inventories.item_code LIKE ? OR products.item_name LIKE ? OR inventories.barcode LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%",
		)
	}

	var inventories []InventoryGrouped
	result := query.
		Group("inventories.location, inventories.item_code, inventories.barcode, inventories.qa_status, inventories.uom, products.item_name, products.category, products.[group]").
		Order("inventories.location ASC, inventories.item_code ASC").
		Find(&inventories)

	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to retrieve inventory",
			"error":   result.Error.Error(),
		})
	}

	// Get filter options for frontend
	var locations []string
	c.DB.Table("inventories").
		Where("deleted_at IS NULL AND qty_onhand > 0").
		Distinct("location").
		Order("location ASC").
		Pluck("location", &locations)

	var categories []string
	c.DB.Table("products").
		Where("deleted_at IS NULL").
		Distinct("category").
		Order("category ASC").
		Pluck("category", &categories)

	var groups []string
	c.DB.Table("products").
		Where("deleted_at IS NULL").
		Distinct("[group]").
		Order("[group] ASC").
		Pluck("[group]", &groups)

	qaStatuses := []string{"A", "R", "Q"}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inventory retrieved successfully",
		"data":    inventories,
		"total":   len(inventories),
		"filters": fiber.Map{
			"locations":   locations,
			"categories":  categories,
			"groups":      groups,
			"qa_statuses": qaStatuses,
		},
	})
}

//===================================================================
// END GET INVENTORY GROUPED
//===================================================================

//===================================================================
// BEGIN GET INVENTORY MOVEMENT
//===================================================================

// GetInventoryMovements returns paginated inventory movements with filters
func (c *InventoryController) GetInventoryMovements(ctx *fiber.Ctx) error {
	type InventoryMovementResponse struct {
		ID         uint   `json:"id"`
		MovementID string `json:"movement_id"`
		ItemCode   string `json:"item_code"`
		ItemName   string `json:"item_name"`
		RefType    string `json:"ref_type"`
		RefID      uint   `json:"ref_id"`

		QtyOnhandChange    float64 `json:"qty_onhand_change"`
		QtyAvailableChange float64 `json:"qty_available_change"`
		QtyAllocatedChange float64 `json:"qty_allocated_change"`
		QtySuspendChange   float64 `json:"qty_suspend_change"`
		QtyShippedChange   float64 `json:"qty_shipped_change"`

		FromWhsCode  string `json:"from_whs_code"`
		ToWhsCode    string `json:"to_whs_code"`
		FromLocation string `json:"from_location"`
		ToLocation   string `json:"to_location"`
		OldQaStatus  string `json:"old_qa_status"`
		NewQaStatus  string `json:"new_qa_status"`

		Reason    string    `json:"reason"`
		CreatedBy int       `json:"created_by"`
		CreatedAt time.Time `json:"created_at"`
	}

	// Pagination parameters
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 10 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// Filter parameters
	refType := ctx.Query("ref_type")
	itemCode := ctx.Query("item_code")
	location := ctx.Query("location")
	whsCode := ctx.Query("whs_code")
	qaStatus := ctx.Query("qa_status")
	search := ctx.Query("search")
	dateFrom := ctx.Query("date_from")
	dateTo := ctx.Query("date_to")

	// Build query
	query := c.DB.Table("inventory_movements").
		Select(`
			inventory_movements.id,
			inventory_movements.movement_id,
			inventory_movements.item_code,
			products.item_name,
			inventory_movements.ref_type,
			inventory_movements.ref_id,
			inventory_movements.qty_onhand_change,
			inventory_movements.qty_available_change,
			inventory_movements.qty_allocated_change,
			inventory_movements.qty_suspend_change,
			inventory_movements.qty_shipped_change,
			inventory_movements.from_whs_code,
			inventory_movements.to_whs_code,
			inventory_movements.from_location,
			inventory_movements.to_location,
			inventory_movements.old_qa_status,
			inventory_movements.new_qa_status,
			inventory_movements.reason,
			inventory_movements.created_by,
			inventory_movements.created_at
		`).
		Joins("LEFT JOIN products ON inventory_movements.item_id = products.id")

	// Apply filters
	if refType != "" {
		query = query.Where("inventory_movements.ref_type = ?", refType)
	}
	if itemCode != "" {
		query = query.Where("inventory_movements.item_code = ?", itemCode)
	}
	if location != "" {
		query = query.Where(
			"inventory_movements.from_location = ? OR inventory_movements.to_location = ?",
			location, location,
		)
	}
	if whsCode != "" {
		query = query.Where(
			"inventory_movements.from_whs_code = ? OR inventory_movements.to_whs_code = ?",
			whsCode, whsCode,
		)
	}
	if qaStatus != "" {
		query = query.Where(
			"inventory_movements.old_qa_status = ? OR inventory_movements.new_qa_status = ?",
			qaStatus, qaStatus,
		)
	}
	if search != "" {
		query = query.Where(
			"inventory_movements.movement_id LIKE ? OR inventory_movements.item_code LIKE ? OR products.item_name LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%",
		)
	}
	if dateFrom != "" {
		query = query.Where("inventory_movements.created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("inventory_movements.created_at <= ?", dateTo+" 23:59:59")
	}

	// Count total records (for pagination)
	var totalRecords int64
	countQuery := query
	countQuery.Count(&totalRecords)

	// Get paginated data
	var movements []InventoryMovementResponse
	result := query.
		Order("inventory_movements.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&movements)

	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to retrieve inventory movements",
			"error":   result.Error.Error(),
		})
	}

	// Get filter options
	var refTypes []string
	c.DB.Table("inventory_movements").
		Distinct("ref_type").
		Order("ref_type ASC").
		Pluck("ref_type", &refTypes)

	var locations []string
	c.DB.Table("inventory_movements").
		Select("DISTINCT COALESCE(from_location, to_location) as location").
		Where("COALESCE(from_location, to_location) != ''").
		Order("location ASC").
		Pluck("location", &locations)

	var whsCodes []string
	c.DB.Table("inventory_movements").
		Select("DISTINCT COALESCE(from_whs_code, to_whs_code) as whs_code").
		Where("COALESCE(from_whs_code, to_whs_code) != ''").
		Order("whs_code ASC").
		Pluck("whs_code", &whsCodes)

	qaStatuses := []string{"A", "R", "Q"}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(pageSize)))

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Inventory movements retrieved successfully",
		"data":    movements,
		"pagination": fiber.Map{
			"page":          page,
			"page_size":     pageSize,
			"total_records": totalRecords,
			"total_pages":   totalPages,
		},
		"filters": fiber.Map{
			"ref_types":   refTypes,
			"locations":   locations,
			"whs_codes":   whsCodes,
			"qa_statuses": qaStatuses,
		},
	})
}

//===================================================================
// END GET INVENTORY MOVEMENT
//===================================================================
