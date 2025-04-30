package mobiles

import (
	"fiber-app/models"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/exp/rand"
	"gorm.io/gorm"
)

type MobileInventoryController struct {
	DB *gorm.DB
}

func NewMobileInventoryController(DB *gorm.DB) *MobileInventoryController {
	return &MobileInventoryController{DB: DB}
}

func (c *MobileInventoryController) GetItemsByLocation(ctx *fiber.Ctx) error {

	location := ctx.Params("location")

	if location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid location"})
	}

	var inventories []models.Inventory

	if err := c.DB.Where("location = ?", location).Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inventories})
}

func (c *MobileInventoryController) CreateDummyInventory(ctx *fiber.Ctx) error {
	// Ambil jumlah dari query param (default 100)
	count := ctx.QueryInt("count", 100)

	var inventories []models.Inventory

	for i := 0; i < count; i++ {
		fmt.Println("Loop ke-", i, "Data : ", inventories)
		now := time.Now()
		inventory := models.Inventory{
			InboundDetailId:  rand.Intn(1000),
			InboundBarcodeId: rand.Intn(1000),
			RecDate:          now.Format("2006-01-02"),
			Owner:            fmt.Sprintf("Owner%d", rand.Intn(100)),
			WhsCode:          fmt.Sprintf("WHS%d", rand.Intn(10)),
			Pallet:           fmt.Sprintf("Pallet%d", rand.Intn(100)),
			Location:         fmt.Sprintf("Loc%d", rand.Intn(50)),
			ItemId:           rand.Intn(1000),
			ItemCode:         fmt.Sprintf("ITEMCODE%d", rand.Intn(10000)),
			Barcode:          fmt.Sprintf("BARCODE%d", rand.Intn(99999)),
			SerialNumber:     fmt.Sprintf("SN%d", rand.Intn(99999)),
			QaStatus:         "A",
			QtyOrigin:        rand.Intn(100),
			QtyOnhand:        rand.Intn(100),
			QtyAvailable:     rand.Intn(100),
			QtyAllocated:     rand.Intn(100),
			QtySuspend:       rand.Intn(100),
			QtyShipped:       rand.Intn(100),
			Trans:            "dummy",
			CreatedBy:        1,
			UpdatedBy:        1,
		}
		inventories = append(inventories, inventory)
	}

	// Batch Insert
	if err := c.DB.Create(&inventories).Error; err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"error": "Failed to insert dummy data to database, error: " + err.Error(),
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"success": true,
		"data":    inventories})
}

func (c *MobileInventoryController) GetItemsByLocationAndBarcode(ctx *fiber.Ctx) error {

	type request struct {
		Location string `json:"location" validate:"required"`
		Barcode  string `json:"barcode"`
	}

	var req request
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location is required"})
	}

	var inventories []models.Inventory
	if err := c.DB.Where("location = ? AND barcode = ? AND qty_available > 0", req.Location, req.Barcode).Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inventories})
}

func (c *MobileInventoryController) ConfirmTransferByLocationAndBarcode(ctx *fiber.Ctx) error {

	var input struct {
		FromLocation  string `json:"from_location"`
		ToLocation    string `json:"to_location"`
		ListInventory []struct {
			ID int `json:"id"`
		} `json:"list_inventory"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Input : ", input)

	if input.FromLocation == "" || input.ToLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location and To Location are required"})
	}

	if input.FromLocation == input.ToLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location and To Location cannot be the same"})
	}

	if len(input.ListInventory) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "List Inventory is required"})
	}

	// start db transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, inv := range input.ListInventory {

		var inventory models.Inventory
		if err := tx.Where("id = ? AND location = ? AND qty_available > 0", inv.ID, input.FromLocation).First(&inventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
		}

		if inventory.Location != input.FromLocation {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
		}

		var newInventory models.Inventory
		newInventory.InboundDetailId = inventory.InboundDetailId
		newInventory.InboundBarcodeId = inventory.InboundBarcodeId
		newInventory.RecDate = inventory.RecDate
		newInventory.ItemId = inventory.ItemId
		newInventory.ItemCode = inventory.ItemCode
		newInventory.Barcode = inventory.Barcode
		newInventory.WhsCode = inventory.WhsCode
		newInventory.Pallet = input.ToLocation
		newInventory.Location = input.ToLocation
		newInventory.QaStatus = inventory.QaStatus
		newInventory.SerialNumber = inventory.SerialNumber
		newInventory.QtyOrigin = inventory.QtyOrigin
		newInventory.QtyOnhand = inventory.QtyOnhand
		newInventory.QtyAvailable = inventory.QtyAvailable
		newInventory.QtyAllocated = 0
		newInventory.QtySuspend = 0
		newInventory.QtyShipped = 0
		newInventory.Trans = "transfer"
		newInventory.CreatedBy = int(ctx.Locals("userID").(float64))
		newInventory.UpdatedBy = int(ctx.Locals("userID").(float64))

		if err := tx.Create(&newInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		var oldInventory models.Inventory
		if err := tx.Where("id = ?", inv.ID).First(&oldInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		oldInventory.QtyOrigin = oldInventory.QtyOrigin - inventory.QtyAvailable
		oldInventory.QtyOnhand = oldInventory.QtyOnhand - inventory.QtyAvailable
		oldInventory.QtyAvailable = oldInventory.QtyAvailable - inventory.QtyAvailable
		oldInventory.UpdatedAt = time.Now()
		oldInventory.UpdatedBy = int(ctx.Locals("userID").(float64))

		if err := tx.Select("qty_origin", "qty_onhand", "qty_available", "updated_at", "updated_by").Updates(&oldInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Confirm putaway successfully"})
}

func (c *MobileInventoryController) ConfirmTransferBySerial(ctx *fiber.Ctx) error {

	var input struct {
		FromLocation string `json:"from_location"`
		ToLocation   string `json:"to_location"`
		InvetoryID   int    `json:"inventory_id"`
		QtyTransfer  int    `json:"qty_transfer"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if input.FromLocation == "" || input.ToLocation == "" || input.InvetoryID == 0 || input.QtyTransfer == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location, To Location, Inventory ID and Qty Transfer are required"})
	}

	// start db transaction
	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": tx.Error.Error()})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var inventory models.Inventory
	if err := tx.Where("id = ? AND location = ? AND qty_available > 0", input.InvetoryID, input.FromLocation).First(&inventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

	if inventory.Location != input.FromLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

	var newInventory models.Inventory
	newInventory.InboundDetailId = inventory.InboundDetailId
	newInventory.InboundBarcodeId = inventory.InboundBarcodeId
	newInventory.RecDate = inventory.RecDate
	newInventory.ItemId = inventory.ItemId
	newInventory.ItemCode = inventory.ItemCode
	newInventory.Barcode = inventory.Barcode
	newInventory.WhsCode = inventory.WhsCode
	newInventory.Pallet = input.ToLocation
	newInventory.Location = input.ToLocation
	newInventory.QaStatus = inventory.QaStatus
	newInventory.SerialNumber = inventory.SerialNumber
	newInventory.QtyOrigin = inventory.QtyOrigin
	newInventory.QtyOnhand = inventory.QtyOnhand
	newInventory.QtyAvailable = inventory.QtyAvailable
	newInventory.QtyAllocated = 0
	newInventory.QtySuspend = 0
	newInventory.QtyShipped = 0
	newInventory.Trans = "transfer by serial"
	newInventory.CreatedBy = inventory.CreatedBy
	newInventory.UpdatedBy = inventory.UpdatedBy
	newInventory.DeletedBy = inventory.DeletedBy

	if err := tx.Create(&newInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var oldInventory models.Inventory
	if err := tx.Where("id = ?", input.InvetoryID).First(&oldInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	oldInventory.QtyOrigin = oldInventory.QtyOrigin - input.QtyTransfer
	oldInventory.QtyOnhand = oldInventory.QtyOnhand - input.QtyTransfer
	oldInventory.QtyAvailable = oldInventory.QtyAvailable - input.QtyTransfer
	oldInventory.UpdatedAt = time.Now()
	oldInventory.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := tx.Select("qty_origin", "qty_onhand", "qty_available", "updated_at", "updated_by").Updates(&oldInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Confirm transfer by serial successfully"})
}
