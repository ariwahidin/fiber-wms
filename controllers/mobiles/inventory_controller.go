package mobiles

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
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
			InboundDetailId: rand.Intn(1000),
			// InboundBarcodeId: rand.Intn(1000),
			RecDate:   now.Format("2006-01-02"),
			OwnerCode: fmt.Sprintf("Owner%d", rand.Intn(100)),
			WhsCode:   fmt.Sprintf("WHS%d", rand.Intn(10)),
			Pallet:    fmt.Sprintf("Pallet%d", rand.Intn(100)),
			Location:  fmt.Sprintf("Loc%d", rand.Intn(50)),
			ItemId:    rand.Intn(1000),
			ItemCode:  fmt.Sprintf("ITEMCODE%d", rand.Intn(10000)),
			Barcode:   fmt.Sprintf("BARCODE%d", rand.Intn(99999)),
			// SerialNumber:     fmt.Sprintf("SN%d", rand.Intn(99999)),
			QaStatus: "A",
			// QtyOrigin:    rand.Intn(100),
			QtyOnhand:    rand.Intn(100),
			QtyAvailable: rand.Intn(100),
			QtyAllocated: rand.Intn(100),
			QtySuspend:   rand.Intn(100),
			QtyShipped:   rand.Intn(100),
			Trans:        "dummy",
			CreatedBy:    1,
			UpdatedBy:    1,
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
	// if err := c.DB.Where("location = ? AND qty_available > 0", req.Barcode, req.Location).Find(&inventories).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	if req.Barcode != "" {
		if err := c.DB.Where("location = ? AND barcode = ? AND qty_available > 0", req.Location, req.Barcode).Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if err := c.DB.Where("location = ? AND qty_available > 0 AND qty_allocated = 0", req.Location).Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	totalAllocated := 0
	for _, inv := range inventories {
		totalAllocated += inv.QtyAllocated
	}

	fmt.Println("Total Allocated : ", totalAllocated)

	// if totalAllocated > 0 {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Item already allocated"})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inventories})
}

func (c *MobileInventoryController) ConfirmTransferByLocationAndBarcode(ctx *fiber.Ctx) error {

	var input struct {
		FromLocation  string `json:"from_location"`
		ToLocation    string `json:"to_location"`
		ListInventory []struct {
			ID string `json:"id"`
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

	// check ToLocation is registered
	var location models.Location
	if err := c.DB.Where("location_code = ?", input.ToLocation).First(&location).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "To Location is not registered"})
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

		// if inventory.QtyAllocated > 0 {
		// 	tx.Rollback()
		// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory already allocated"})
		// }

		if inventory.Location != input.FromLocation {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
		}

		var newInventory models.Inventory
		newInventory.OwnerCode = inventory.OwnerCode
		newInventory.DivisionCode = inventory.DivisionCode
		newInventory.Uom = inventory.Uom
		newInventory.InboundID = inventory.InboundID
		newInventory.InboundDetailId = inventory.InboundDetailId
		newInventory.RecDate = inventory.RecDate
		newInventory.ItemId = inventory.ItemId
		newInventory.ItemCode = inventory.ItemCode
		newInventory.Barcode = inventory.Barcode
		newInventory.WhsCode = inventory.WhsCode
		newInventory.Pallet = input.ToLocation
		newInventory.Location = input.ToLocation
		newInventory.QaStatus = inventory.QaStatus
		newInventory.QtyOrigin = inventory.QtyAvailable
		newInventory.QtyOnhand = inventory.QtyAvailable
		newInventory.QtyAvailable = inventory.QtyAvailable
		newInventory.Trans = fmt.Sprintf("transfer from inventory_id : %d", inventory.ID)
		newInventory.IsTransfer = true
		newInventory.TransferFrom = inventory.ID
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

func (c *MobileInventoryController) ConfirmTransferByInventoryID(ctx *fiber.Ctx) error {

	var input struct {
		FromLocation string `json:"from_location"`
		ToLocation   string `json:"to_location"`
		InventoryID  string `json:"inventory_id"`
		QtyTransfer  int    `json:"qty_transfer"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if input.FromLocation == "" || input.ToLocation == "" || input.InventoryID == "" || input.QtyTransfer == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location, To Location, Inventory ID and Qty Transfer are required"})
	}

	if input.FromLocation == input.ToLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location and To Location cannot be the same"})
	}

	// convert InventoryID to int
	inventoryID, err := strconv.Atoi(input.InventoryID)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Inventory ID"})
	}

	if input.FromLocation == "" || input.ToLocation == "" || inventoryID == 0 || input.QtyTransfer == 0 {
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

	// validate to location already exists on master locations
	var toLocation models.Location
	if err := tx.Where("location_code = ?", input.ToLocation).First(&toLocation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "To Location is not registered"})
		}
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inventory models.Inventory
	if err := tx.Where("id = ? AND location = ? AND qty_available > 0", inventoryID, input.FromLocation).First(&inventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

	if inventory.Location != input.FromLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

	// if inventory.QtyAllocated > 0 {
	// 	tx.Rollback()
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory already allocated"})
	// }

	if inventory.QtyAvailable < input.QtyTransfer {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Qty Transfer is greater than available quantity"})
	}

	var newInventory models.Inventory
	newInventory.OwnerCode = inventory.OwnerCode
	newInventory.DivisionCode = inventory.DivisionCode
	newInventory.Uom = inventory.Uom
	newInventory.InboundID = inventory.InboundID
	newInventory.InboundDetailId = inventory.InboundDetailId
	newInventory.RecDate = inventory.RecDate
	newInventory.ItemId = inventory.ItemId
	newInventory.ItemCode = inventory.ItemCode
	newInventory.Barcode = inventory.Barcode
	newInventory.WhsCode = inventory.WhsCode
	newInventory.Pallet = input.ToLocation
	newInventory.Location = input.ToLocation
	newInventory.QaStatus = inventory.QaStatus
	newInventory.QtyOrigin = input.QtyTransfer
	newInventory.QtyOnhand = input.QtyTransfer
	newInventory.QtyAvailable = input.QtyTransfer
	newInventory.Trans = fmt.Sprintf("transfer from inventory_id : %d", inventory.ID)
	newInventory.IsTransfer = true
	newInventory.TransferFrom = inventory.ID
	newInventory.CreatedAt = time.Now()
	newInventory.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := tx.Create(&newInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var oldInventory models.Inventory
	if err := tx.Where("id = ?", inventoryID).First(&oldInventory).Error; err != nil {
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transfer successful"})
}

type LocationRequest struct {
	NewLocation string `json:"new_location" validate:"required"`
}

// CREATE
func (lc *MobileInventoryController) CreateLocation(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var newLocation LocationRequest
	if err := ctx.BodyParser(&newLocation); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// validate location length must 9
	if len(newLocation.NewLocation) != 9 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid location length, must be 9 characters"})
	}

	// check lokasi sudah ada
	var existingLocation models.Location
	if err := lc.DB.Where("location_code = ?", newLocation.NewLocation).First(&existingLocation).Error; err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location already exists"})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Example YMK49B102

	row := newLocation.NewLocation[0:3]   // "YMK"
	bay := newLocation.NewLocation[3:5]   // "49"
	level := newLocation.NewLocation[5:7] // "B1"
	bin := newLocation.NewLocation[7:]    // "02"

	var location models.Location
	location.Row = row
	location.Bay = bay
	location.Level = level
	location.Bin = bin
	location.LocationCode = newLocation.NewLocation

	bayInt, err := strconv.Atoi(location.Bay)
	if err != nil {
		location.Area = "Unknown"
	} else {
		if bayInt%2 != 0 {
			location.Area = "ganjil"
		} else {
			location.Area = "genap"
		}
	}

	location.CreatedBy = userID
	location.UpdatedBy = userID

	if err := lc.DB.Create(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "New location created successfully",
		"data":    location,
	})
}

type Payload struct {
	Barcode string `json:"barcode"`
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func (c *MobileInventoryController) GetItemsByBarcode(ctx *fiber.Ctx) error {
	barcode := ctx.Params("barcode")

	if barcode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid barcode",
		})
	}

	type InventoryResult struct {
		ItemName     string  `json:"item_name"`
		ItemCode     string  `json:"item_code"`
		Barcode      string  `json:"barcode"`
		Location     string  `json:"location"`
		WhsCode      string  `json:"whs_code"`
		RecDate      string  `json:"rec_date"`
		QtyAvailable float64 `json:"qty_available"`
	}

	var results []InventoryResult

	query := `
		SELECT 
			p.item_name,
			inv.item_code,
			inv.barcode,
			inv.location,
			inv.whs_code,
			inv.rec_date,
			SUM(inv.qty_available) AS qty_available
		FROM inventories inv
		INNER JOIN products p ON inv.item_id = p.id
		WHERE (inv.barcode = ? OR inv.item_code = ?) AND inv.qty_available > 0
		GROUP BY
			p.item_name,
			inv.item_code,
			inv.barcode,
			inv.location,
			inv.whs_code,
			inv.rec_date
	`

	if err := c.DB.Raw(query, barcode, barcode).Scan(&results).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(results) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Item not found",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}
