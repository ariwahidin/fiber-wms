package mobiles

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

// func (c *MobileInventoryController) CreateDummyInventory(ctx *fiber.Ctx) error {
// 	// Ambil jumlah dari query param (default 100)
// 	count := ctx.QueryInt("count", 100)

// 	var inventories []models.Inventory

// 	for i := 0; i < count; i++ {
// 		fmt.Println("Loop ke-", i, "Data : ", inventories)
// 		now := time.Now()
// 		inventory := models.Inventory{
// 			InboundDetailId: rand.Intn(1000),
// 			// InboundBarcodeId: rand.Intn(1000),
// 			RecDate:   now.Format("2006-01-02"),
// 			OwnerCode: fmt.Sprintf("Owner%d", rand.Intn(100)),
// 			WhsCode:   fmt.Sprintf("WHS%d", rand.Intn(10)),
// 			Pallet:    fmt.Sprintf("Pallet%d", rand.Intn(100)),
// 			Location:  fmt.Sprintf("Loc%d", rand.Intn(50)),
// 			ItemId:    rand.Intn(1000),
// 			ItemCode:  fmt.Sprintf("ITEMCODE%d", rand.Intn(10000)),
// 			Barcode:   fmt.Sprintf("BARCODE%d", rand.Intn(99999)),
// 			// SerialNumber:     fmt.Sprintf("SN%d", rand.Intn(99999)),
// 			QaStatus: "A",
// 			// QtyOrigin:    rand.Intn(100),
// 			QtyOnhand:    rand.Intn(100),
// 			QtyAvailable: rand.Intn(100),
// 			QtyAllocated: rand.Intn(100),
// 			QtySuspend:   rand.Intn(100),
// 			QtyShipped:   rand.Intn(100),
// 			Trans:        "dummy",
// 			CreatedBy:    1,
// 			UpdatedBy:    1,
// 		}
// 		inventories = append(inventories, inventory)
// 	}

// 	// Batch Insert
// 	if err := c.DB.Create(&inventories).Error; err != nil {
// 		return ctx.Status(500).JSON(fiber.Map{
// 			"error": "Failed to insert dummy data to database, error: " + err.Error(),
// 		})
// 	}

// 	return ctx.Status(200).JSON(fiber.Map{
// 		"success": true,
// 		"data":    inventories})
// }

func (c *MobileInventoryController) GetItemsByLocationAndBarcode(ctx *fiber.Ctx) error {

	type request struct {
		Location string `json:"location" validate:"required"`
		Barcode  string `json:"barcode"`
	}

	type resultInventory struct {
		ID              int64   `json:"ID"`
		InboundID       int64   `json:"inbound_id"`
		InboundDetailID int64   `json:"inbound_detail_id"`
		Barcode         string  `json:"barcode"`
		SerialNumber    string  `json:"serial_number"`
		Pallet          string  `json:"pallet"`
		Location        string  `json:"location"`
		QaStatus        string  `json:"qa_status"`
		WhsCode         string  `json:"whs_code"`
		QtyAvailable    float64 `json:"qty_available"`
		QtyAllocated    float64 `json:"qty_allocated"`
		RecDate         string  `json:"rec_date"`
		LotNumber       string  `json:"lot_number"`
		ProdDate        string  `json:"prod_date"`
		ExpDate         string  `json:"exp_date"`
		Uom             string  `json:"uom"`
		QtyDisplay      float64 `json:"qty_display"`
		UomDisplay      string  `json:"uom_display"`
		EanDisplay      string  `json:"ean_display"`
		OwnerCode       string  `json:"owner_code"`
	}

	var req request
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location is required"})
	}

	var inventories []resultInventory
	uomRepo := repositories.NewUomRepository(c.DB)
	if req.Barcode != "" {

		uomConvByBarcode, err := uomRepo.GetUomConversionByEan(req.Barcode)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if err := c.DB.
			Table("inventories").
			Select("inventories.*, qty_available / ? AS qty_display, ? AS uom_display, ? AS ean_display", uomConvByBarcode.Rate, uomConvByBarcode.Uom, req.Barcode).
			Where("location = ? AND barcode = ? AND qty_available > 0", req.Location, uomConvByBarcode.BaseEan).Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if err := c.DB.
			Table("inventories").
			Select("inventories.*, qty_available AS qty_display, uom AS uom_display, barcode AS ean_display").
			Where("location = ? AND qty_available > 0", req.Location).Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var totalAllocated float64 = 0
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
			ID int `json:"id"`
		} `json:"list_inventory"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Input : ", input)

	movementID := uuid.NewString()

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

		// if inventory.QtyAvailable < input.QtyTransfer {

		// }

		var newInventory models.Inventory
		newInventory.OwnerCode = inventory.OwnerCode
		newInventory.DivisionCode = inventory.DivisionCode
		newInventory.Uom = inventory.Uom
		newInventory.InboundID = inventory.InboundID
		newInventory.InboundDetailId = inventory.InboundDetailId
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
		newInventory.RecDate = inventory.RecDate
		newInventory.ExpDate = inventory.ExpDate
		newInventory.ProdDate = inventory.ProdDate
		newInventory.LotNumber = inventory.LotNumber
		newInventory.InventoryNumber = inventory.InventoryNumber
		newInventory.CreatedAt = time.Now()
		newInventory.CreatedBy = int(ctx.Locals("userID").(float64))

		if err := tx.Create(&newInventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Record destination inventory movement
		destMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        newInventory.ID,
			RefType:            "TRANSFER",
			RefID:              inventory.ID,
			ItemID:             newInventory.ItemId,
			ItemCode:           newInventory.ItemCode,
			QtyOnhandChange:    newInventory.QtyAvailable,
			QtyAvailableChange: newInventory.QtyAvailable,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			FromWhsCode:        inventory.WhsCode,
			ToWhsCode:          newInventory.WhsCode,
			FromLocation:       input.FromLocation,
			ToLocation:         input.ToLocation,
			OldQaStatus:        inventory.QaStatus,
			NewQaStatus:        newInventory.QaStatus,
			Reason:             "TRANSFER USING SCANNER",
			CreatedBy:          int(ctx.Locals("userID").(float64)),
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&destMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
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

		// Record source inventory movement
		sourceMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        oldInventory.ID,
			RefType:            "TRANSFER",
			RefID:              newInventory.ID,
			ItemID:             oldInventory.ItemId,
			ItemCode:           oldInventory.ItemCode,
			QtyOnhandChange:    -inventory.QtyAvailable,
			QtyAvailableChange: -inventory.QtyAvailable,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			FromWhsCode:        oldInventory.WhsCode,
			ToWhsCode:          newInventory.WhsCode,
			FromLocation:       input.FromLocation,
			ToLocation:         input.ToLocation,
			OldQaStatus:        oldInventory.QaStatus,
			NewQaStatus:        newInventory.QaStatus,
			Reason:             "TRANSFER USING SCANNER",
			CreatedBy:          int(ctx.Locals("userID").(float64)),
			CreatedAt:          time.Now(),
		}

		if err := tx.Create(&sourceMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record source movement",
			})
		}

	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Confirm putaway successfully"})
}

func (c *MobileInventoryController) ConfirmTransferByInventoryID(ctx *fiber.Ctx) error {
	movementID := uuid.NewString()
	var input struct {
		FromLocation string  `json:"from_location"`
		ToLocation   string  `json:"to_location"`
		InventoryID  int     `json:"inventory_id"`
		QtyTransfer  float64 `json:"qty_transfer"`
		EanTransfer  string  `json:"ean_transfer"`
		UomTransfer  string  `json:"uom_transfer"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Input : ", input)
	// return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Confirm transfer successfully"})

	if input.FromLocation == "" || input.ToLocation == "" || input.InventoryID == 0 || input.QtyTransfer == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location, To Location, Inventory ID and Qty Transfer are required"})
	}

	if input.FromLocation == input.ToLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location and To Location cannot be the same"})
	}

	inventoryID := input.InventoryID

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

	var uomConversion models.UomConversion
	if err := tx.Where("ean = ?", input.EanTransfer).First(&uomConversion).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	uomRepo := repositories.NewUomRepository(tx)
	qtyTransferConverted, errqtc := uomRepo.ConversionQty(uomConversion.ItemCode, input.QtyTransfer, input.UomTransfer)
	if errqtc != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errqtc.Error()})
	}

	input.QtyTransfer = qtyTransferConverted.QtyConverted

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

	if input.QtyTransfer > inventory.QtyAvailable {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Qty Transfer is greater than available quantity"})
	}

	var newInventory models.Inventory
	newInventory.OwnerCode = inventory.OwnerCode
	newInventory.DivisionCode = inventory.DivisionCode
	newInventory.Uom = inventory.Uom
	newInventory.InboundID = inventory.InboundID
	newInventory.InboundDetailId = inventory.InboundDetailId
	newInventory.ItemId = inventory.ItemId
	newInventory.ItemCode = inventory.ItemCode
	newInventory.Barcode = inventory.Barcode
	newInventory.WhsCode = inventory.WhsCode
	newInventory.Pallet = input.ToLocation
	newInventory.Location = input.ToLocation
	newInventory.QaStatus = inventory.QaStatus
	newInventory.QtyOrigin = float64(input.QtyTransfer)
	newInventory.QtyOnhand = float64(input.QtyTransfer)
	newInventory.QtyAvailable = float64(input.QtyTransfer)
	newInventory.Trans = fmt.Sprintf("transfer from inventory_id : %d", inventory.ID)
	newInventory.IsTransfer = true
	newInventory.TransferFrom = inventory.ID
	newInventory.RecDate = inventory.RecDate
	newInventory.ExpDate = inventory.ExpDate
	newInventory.ProdDate = inventory.ProdDate
	newInventory.LotNumber = inventory.LotNumber
	newInventory.InventoryNumber = inventory.InventoryNumber
	newInventory.CreatedAt = time.Now()
	newInventory.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := tx.Create(&newInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Record destination inventory movement
	destMovement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        newInventory.ID,
		RefType:            "TRANSFER",
		RefID:              inventory.ID,
		ItemID:             newInventory.ItemId,
		ItemCode:           newInventory.ItemCode,
		QtyOnhandChange:    newInventory.QtyOnhand,
		QtyAvailableChange: newInventory.QtyAvailable,
		QtyAllocatedChange: 0,
		QtySuspendChange:   0,
		QtyShippedChange:   0,
		FromWhsCode:        inventory.WhsCode,
		ToWhsCode:          newInventory.WhsCode,
		FromLocation:       input.FromLocation,
		ToLocation:         input.ToLocation,
		OldQaStatus:        inventory.QaStatus,
		NewQaStatus:        newInventory.QaStatus,
		Reason:             "TRANSFER USING SCANNER",
		CreatedBy:          int(ctx.Locals("userID").(float64)),
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(&destMovement).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to record destination movement",
		})
	}

	var oldInventory models.Inventory
	if err := tx.Where("id = ?", inventoryID).First(&oldInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	oldInventory.QtyOrigin = oldInventory.QtyOrigin - float64(input.QtyTransfer)
	oldInventory.QtyOnhand = oldInventory.QtyOnhand - float64(input.QtyTransfer)
	oldInventory.QtyAvailable = oldInventory.QtyAvailable - float64(input.QtyTransfer)
	oldInventory.UpdatedAt = time.Now()
	oldInventory.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := tx.Select("qty_origin", "qty_onhand", "qty_available", "updated_at", "updated_by").Updates(&oldInventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Record source inventory movement
	sourceMovement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        oldInventory.ID,
		RefType:            "TRANSFER",
		RefID:              newInventory.ID,
		ItemID:             oldInventory.ItemId,
		ItemCode:           oldInventory.ItemCode,
		QtyOnhandChange:    -input.QtyTransfer,
		QtyAvailableChange: -input.QtyTransfer,
		QtyAllocatedChange: 0,
		QtySuspendChange:   0,
		QtyShippedChange:   0,
		FromWhsCode:        oldInventory.WhsCode,
		ToWhsCode:          newInventory.WhsCode,
		FromLocation:       input.FromLocation,
		ToLocation:         input.ToLocation,
		OldQaStatus:        oldInventory.QaStatus,
		NewQaStatus:        newInventory.QaStatus,
		Reason:             "TRANSFER USING SCANNER",
		CreatedBy:          int(ctx.Locals("userID").(float64)),
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(&sourceMovement).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to record source movement",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Transfer successful"})
}

type LocationRequest struct {
	WhsCode     string `json:"whs_code" validate:"required"`
	NewLocation string `json:"new_location" validate:"required"`
}

// CREATE
func (lc *MobileInventoryController) CreateLocation(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var newLocation LocationRequest
	if err := ctx.BodyParser(&newLocation); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// validate location length must 8
	if len(newLocation.NewLocation) != 8 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid location length, must be 8 characters"})
	}

	// check lokasi sudah ada
	var existingLocation models.Location
	if err := lc.DB.Where("location_code = ?", newLocation.NewLocation).First(&existingLocation).Error; err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location already exists"})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	warehouse := models.Warehouse{}
	if err := lc.DB.Where("code = ?", newLocation.WhsCode).First(&warehouse).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Warehouse not found"})
	}

	row := newLocation.NewLocation[0:2]   // "YM"
	bay := newLocation.NewLocation[2:4]   // "49"
	level := newLocation.NewLocation[4:6] // "B1"
	bin := newLocation.NewLocation[6:8]   // "02"

	var location models.Location
	location.WhsCode = warehouse.Code
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

type RegisterProductRequest struct {
	OwnerCode string `json:"owner_code"`
	SKU       string `json:"sku"`
	UnitModel string `json:"unit_model"`
	Ean       string `json:"ean"`
	Uom       string `json:"uom"`
}

func (c *MobileInventoryController) CreateRegisterProduct(ctx *fiber.Ctx) error {
	var req RegisterProductRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validasi input
	if req.OwnerCode == "" || req.SKU == "" || req.UnitModel == "" || req.Ean == "" || req.Uom == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "All fields are required",
		})
	}

	// Normalize input
	req.OwnerCode = strings.ToUpper(strings.TrimSpace(req.OwnerCode))
	req.SKU = strings.ToUpper(strings.TrimSpace(req.SKU))
	req.UnitModel = strings.ToUpper(strings.TrimSpace(req.UnitModel))
	req.Ean = strings.ToUpper(strings.TrimSpace(req.Ean))
	req.Uom = strings.ToUpper(strings.TrimSpace(req.Uom))

	// Validasi owner exists
	var ownerExists models.Owner
	if err := c.DB.Where("code = ?", req.OwnerCode).First(&ownerExists).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Owner code not found",
		})
	}

	// Validasi UOM exists
	var uomExists models.Uom
	if err := c.DB.Where("code = ?", req.Uom).First(&uomExists).Error; err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "UOM not found",
		})
	}

	// Cek apakah kombinasi sudah ada
	var existingProduct models.ProductRegister
	err := c.DB.Where("owner_code = ? AND sku = ? AND unit_model = ? AND ean = ? AND uom = ?",
		req.OwnerCode, req.SKU, req.UnitModel, req.Ean, req.Uom).
		First(&existingProduct).Error

	if err == nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Product with this combination already exists",
		})
	}

	// Get user ID from context (sesuaikan dengan auth middleware Anda)
	userID := int(ctx.Locals("userID").(float64))

	// Create new product
	newProduct := models.ProductRegister{
		OwnerCode: req.OwnerCode,
		SKU:       req.SKU,
		UnitModel: req.UnitModel,
		Ean:       req.Ean,
		Uom:       req.Uom,
		CreatedBy: userID,
		CreatedAt: time.Now(),
	}

	if err := c.DB.Create(&newProduct).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to register product",
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Product registered successfully",
		"data":    newProduct,
	})
}
