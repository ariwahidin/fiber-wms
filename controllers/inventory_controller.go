package controllers

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"net/http"
	"time"

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

func (c *InventoryController) MoveItem(ctx *fiber.Ctx) error {
	movePayload := MovePayload{}
	if err := ctx.BodyParser(&movePayload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Failed to parse JSON" + err.Error()})
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

		var oldInventory models.Inventory
		if err := c.DB.Where("id = ?", item.InventoryID).First(&oldInventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to find source inventory" + err.Error()})
		}

		if oldInventory.QtyAvailable < item.Quantity {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Insufficient quantity in source pallet"})
		}

		// Update the source inventory
		oldInventory.QtyAvailable -= item.Quantity
		oldInventory.QtyOnhand -= item.Quantity
		oldInventory.Quantity -= item.Quantity
		oldInventory.UpdatedBy = int(ctx.Locals("userID").(float64))
		oldInventory.UpdatedAt = time.Now()

		if err := c.DB.Debug().Where("id = ?", oldInventory.ID).
			Select("qty_available", "qty_onhand", "quantity", "updated_by", "updated_at").
			Updates(&oldInventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update source inventory" + err.Error()})
		}

		// Create new inventory for target pallet
		newInventory := models.Inventory{
			InboundDetailId:  oldInventory.InboundDetailId,
			InboundBarcodeId: oldInventory.InboundBarcodeId,
			RecDate:          oldInventory.RecDate,
			ItemId:           item.ItemID,
			ItemCode:         oldInventory.ItemCode,
			WhsCode:          oldInventory.WhsCode,
			Owner:            oldInventory.Owner,
			Pallet:           movePayload.TargetPallet,
			Location:         movePayload.TargetLocation,
			QaStatus:         oldInventory.QaStatus,
			SerialNumber:     oldInventory.SerialNumber,
			Quantity:         item.Quantity,
			QtyOnhand:        item.Quantity,
			QtyAvailable:     item.Quantity,
			QtyAllocated:     0,
			Trans:            "move item",
			CreatedBy:        int(ctx.Locals("userID").(float64)),
		}
		if err := c.DB.Create(&newInventory).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create target inventory" + err.Error()})
		}

	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Items moved successfully"})
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
