package controllers

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type InventoryController struct {
	DB *gorm.DB
}

func NewInventoryController(DB *gorm.DB) *InventoryController {
	return &InventoryController{DB: DB}
}

func (c *InventoryController) GetInventory(ctx *fiber.Ctx) error {

	inventory_repo := repositories.NewInventoryRepository(c.DB)
	inventories, err := inventory_repo.GetInventory()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": fiber.Map{"inventories": inventories}})
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
		ItemId:          itemID,
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
