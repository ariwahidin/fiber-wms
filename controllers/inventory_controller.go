package controllers

import (
	"fiber-app/repositories"
	"fmt"
	"net/http"

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
	f.SetCellValue(sheet, "F1", "Quantity")

	// Isi data ke dalam sheet
	for i, item := range inventories {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", i+2), item.WhsCode)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", i+2), item.ItemCode)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", i+2), item.ItemName)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", i+2), item.Location)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", i+2), item.QaStatus)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", i+2), item.Quantity)
	}

	// Simpan file ke dalam response
	ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Set("Content-Disposition", `attachment; filename="report.xlsx"`)

	if err := f.Write(ctx.Response().BodyWriter()); err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Gagal generate Excel")
	}

	return nil
}
