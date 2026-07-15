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

func (c *MobileInventoryController) GetItemsByLocationAndBarcode(ctx *fiber.Ctx) error {

	type request struct {
		Location string `json:"location"`
		Barcode  string `json:"barcode"`
		Sku      string `json:"sku"`
		Pallet   string `json:"pallet"`
	}

	type resultInventory struct {
		ID              int64   `json:"ID"`
		InboundID       int64   `json:"inbound_id"`
		InboundDetailID int64   `json:"inbound_detail_id"`
		Barcode         string  `json:"barcode"`
		DivisionCode    string  `json:"division_code"`
		CartonNumber    string  `json:"carton_number"`
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
		ItemCode        string  `json:"item_code"`
		ItemName        string  `json:"item_name"` // ← tambah
	}

	var req request
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validasi: harus ada salah satu — location atau pallet
	if req.Location == "" && req.Pallet == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location or pallet is required"})
	}

	var inventories []resultInventory
	uomRepo := repositories.NewUomRepository(c.DB)

	// ── Branch: Pallet mode ───────────────────────────────────────────────────
	if req.Pallet != "" {
		// Pindahkan semua item by pallet, join ke items untuk item_name
		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				qty_available AS qty_display,
				products.uom AS uom_display,
				inventories.barcode AS ean_display,
				COALESCE(products.item_name, '') AS item_name`).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("inventories.pallet = ? AND inventories.qty_available > 0", req.Pallet).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// ── Branch: Barcode (EAN) mode ────────────────────────────────────────────
	} else if req.Barcode != "" && req.Location != "" && req.Sku == "" {
		uomConvByBarcode, err := uomRepo.GetUomConversionByEan(req.Barcode)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				qty_available / ? AS qty_display,
				? AS uom_display,
				? AS ean_display,
				COALESCE(products.item_name, '') AS item_name`,
				uomConvByBarcode.Rate, uomConvByBarcode.Uom, req.Barcode).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("inventories.location = ? AND inventories.barcode = ? AND inventories.qty_available > 0",
				req.Location, uomConvByBarcode.BaseEan).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// ── Branch: SKU mode ──────────────────────────────────────────────────────
	} else if req.Sku != "" && req.Location != "" && req.Barcode != "" {
		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				qty_available AS qty_display,
				inventories.uom AS uom_display,
				products.barcode AS ean_display,
				COALESCE(products.item_name, '') AS item_name`).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("inventories.location = ? AND inventories.item_code = ? AND inventories.qty_available > 0",
				req.Location, req.Sku).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// ── Branch: Location only — tampilkan semua ───────────────────────────────
	} else {
		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				qty_available AS qty_display,
				products.uom AS uom_display,
				inventories.barcode AS ean_display,
				COALESCE(products.item_name, '') AS item_name`).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("inventories.location = ? AND inventories.qty_available > 0", req.Location).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var totalAllocated float64 = 0
	for _, inv := range inventories {
		totalAllocated += inv.QtyAllocated
	}

	fmt.Println("Total Allocated : ", totalAllocated)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": inventories})
}

func (c *MobileInventoryController) ConfirmTransferByLocationAndBarcode(ctx *fiber.Ctx) error {

	var input struct {
		FromLocation  string `json:"from_location"`
		FromPallet    string `json:"from_pallet"` // sudah ada
		ToLocation    string `json:"to_location"`
		ToPallet      string `json:"to_pallet"` // ← BARU
		ListInventory []struct {
			ID       int    `json:"id"`
			Location string `json:"location"`
			Pallet   string `json:"pallet"`
		} `json:"list_inventory"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	fmt.Println("Input : ", input)

	movementID := uuid.NewString()

	if input.ToLocation == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "To Location is required"})
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

	// ── Pre-fetch: hitung total item per pallet di DB ─────────────────────────
	// Kumpulkan distinct pallet dari input
	palletSet := map[string]bool{}
	for _, inv := range input.ListInventory {
		if inv.Pallet != "" {
			palletSet[inv.Pallet] = true
		}
	}
	palletIDs := make([]string, 0, len(palletSet))
	for p := range palletSet {
		palletIDs = append(palletIDs, p)
	}

	// Hitung total qty_available > 0 per pallet di DB (semua item, bukan hanya yang di-transfer)
	type palletCount struct {
		Pallet string
		Total  int
	}
	var palletCounts []palletCount
	if err := tx.Model(&models.Inventory{}).
		Select("pallet, COUNT(*) as total").
		Where("pallet IN ? AND qty_available > 0", palletIDs).
		Group("pallet").
		Scan(&palletCounts).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Map: pallet → total item di DB
	dbPalletTotal := map[string]int{}
	for _, pc := range palletCounts {
		dbPalletTotal[pc.Pallet] = pc.Total
	}

	// Hitung berapa item per pallet yang ada di input (yang akan di-transfer)
	inputPalletCount := map[string]int{}
	for _, inv := range input.ListInventory {
		inputPalletCount[inv.Pallet]++
	}

	// Cache pallet baru per pallet lama (supaya semua item dari pallet yang sama
	// dapat pallet baru yang sama, bukan generate baru tiap item)
	inventoryRepo := repositories.NewInventoryRepository(tx)
	newPalletCache := map[string]string{}

	resolveNewPallet := func(oldPallet string) (string, error) {
		// Jika user memilih pallet tujuan → pakai langsung
		if input.ToPallet != "" {
			return input.ToPallet, nil
		}
		// behavior existing
		isSplit := inputPalletCount[oldPallet] < dbPalletTotal[oldPallet]
		if !isSplit {
			return oldPallet, nil
		}
		if cached, ok := newPalletCache[oldPallet]; ok {
			return cached, nil
		}
		newPallet, err := inventoryRepo.GeneratePalletID()
		if err != nil {
			return "", err
		}
		newPalletCache[oldPallet] = newPallet
		return newPallet, nil
	}

	// resolveNewPallet := func(oldPallet string) (string, error) {
	// 	// Cek apakah pallet ini perlu di-split
	// 	isSplit := inputPalletCount[oldPallet] < dbPalletTotal[oldPallet]
	// 	if !isSplit {
	// 		// Semua item pallet ini ikut transfer → pakai pallet lama
	// 		return oldPallet, nil
	// 	}
	// 	// Perlu split → cek cache dulu
	// 	if cached, ok := newPalletCache[oldPallet]; ok {
	// 		return cached, nil
	// 	}
	// 	// Generate pallet baru
	// 	newPallet, err := inventoryRepo.GeneratePalletID()
	// 	if err != nil {
	// 		return "", err
	// 	}
	// 	newPalletCache[oldPallet] = newPallet
	// 	return newPallet, nil
	// }
	// ─────────────────────────────────────────────────────────────────────────

	for _, inv := range input.ListInventory {

		var inventory models.Inventory
		if err := tx.Where("id = ? AND location = ? AND qty_available > 0", inv.ID, inv.Location).First(&inventory).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
		}

		var inventoryPolicy models.InventoryPolicy
		if err := tx.Where("owner_code = ?", inventory.OwnerCode).First(&inventoryPolicy).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch inventory policy: " + err.Error()})
		}
		if inventoryPolicy.PickingExcludeLocationsUnderCycleCount {

			// --- Check cycle count ---
			locationRepo := repositories.NewLocationRepository(tx)
			underCount, err := locationRepo.IsLocationUnderCycleCount(inventory.WhsCode, input.ToLocation)
			if err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to check cycle count status: " + err.Error(),
				})
			}
			if underCount {
				tx.Rollback()
				return ctx.Status(fiber.StatusLocked).JSON(fiber.Map{
					"error": "Location " + input.ToLocation + " is currently under cycle count, transfer is not allowed",
				})
			}

		}

		if inventory.Location == input.ToLocation {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location : " + inventory.Location + " and To Location : " + input.ToLocation + " cannot be the same"})
		}

		if inventory.QtyAvailable <= 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
		}

		// Resolve pallet untuk inventory ini (split atau tetap)
		assignedPallet, err := resolveNewPallet(inv.Pallet)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
		newInventory.Pallet = assignedPallet // ← pakai hasil resolve
		newInventory.Location = input.ToLocation
		newInventory.QaStatus = inventory.QaStatus
		newInventory.QtyOrigin = inventory.QtyAvailable
		newInventory.QtyOnhand = inventory.QtyAvailable
		newInventory.QtyAvailable = inventory.QtyAvailable
		newInventory.Trans = "TRANSFER"
		newInventory.IsTransfer = true
		newInventory.TransferFrom = inventory.ID
		newInventory.RecDate = inventory.RecDate
		newInventory.ExpDate = inventory.ExpDate
		newInventory.ProdDate = inventory.ProdDate
		newInventory.LotNumber = inventory.LotNumber
		newInventory.InventoryNumber = inventory.InventoryNumber
		newInventory.CartonNumber = inventory.CartonNumber
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
			FromLocation:       inventory.Location,
			ToLocation:         input.ToLocation,
			OldQaStatus:        inventory.QaStatus,
			NewQaStatus:        newInventory.QaStatus,
			FromDivision:       inventory.DivisionCode,
			ToDivision:         newInventory.DivisionCode,
			FromPallet:         inv.Pallet,     // ← BARU
			ToPallet:           assignedPallet, // ← BARU
			Reason:             "TRANSFER USING SCANNER",

			CreatedBy: int(ctx.Locals("userID").(float64)),
			CreatedAt: time.Now(),
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
			FromLocation:       inventory.Location,
			ToLocation:         input.ToLocation,
			OldQaStatus:        oldInventory.QaStatus,
			NewQaStatus:        newInventory.QaStatus,
			FromDivision:       inventory.DivisionCode,
			ToDivision:         newInventory.DivisionCode,
			FromPallet:         inv.Pallet,     // ← BARU
			ToPallet:           assignedPallet, // ← BARU
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

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Confirm transfer successfully"})
}

func (c *MobileInventoryController) ConfirmTransferByInventoryID(ctx *fiber.Ctx) error {
	movementID := uuid.NewString()

	var input struct {
		FromPallet   string  `json:"from_pallet"`
		FromLocation string  `json:"from_location"`
		ToLocation   string  `json:"to_location"`
		ToPallet     string  `json:"to_pallet"` // ← BARU
		InventoryID  int     `json:"inventory_id"`
		QtyTransfer  float64 `json:"qty_transfer"`
		EanTransfer  string  `json:"ean_transfer"`
		UomTransfer  string  `json:"uom_transfer"`
	}

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi existing tetap, to_pallet opsional jadi tidak masuk validasi required
	if input.FromPallet == "" || input.FromLocation == "" || input.ToLocation == "" || input.InventoryID == 0 || input.QtyTransfer == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Pallet, From Location, To Location, Inventory ID and Qty Transfer are required"})
	}

	if input.FromLocation == input.ToLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Location and To Location cannot be the same"})
	}

	inventoryID := input.InventoryID

	if input.FromPallet == "" || input.FromLocation == "" || input.ToLocation == "" || inventoryID == 0 || input.QtyTransfer == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "From Pallet, From Location, To Location, Inventory ID and Qty Transfer are required"})
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

	invetoryRepo := repositories.NewInventoryRepository(tx)
	isSplit := false

	var uomConversion models.UomConversion
	if err := tx.Where("ean = ?", input.EanTransfer).First(&uomConversion).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var product models.Product
	if err := tx.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; err != nil {
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

	var inventoryPolicy models.InventoryPolicy
	if err := tx.Where("owner_code = ?", product.OwnerCode).First(&inventoryPolicy).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch inventory policy: " + err.Error()})
	}
	if inventoryPolicy.PickingExcludeLocationsUnderCycleCount {

		// --- Check cycle count ---
		locationRepo := repositories.NewLocationRepository(tx)
		underCount, err := locationRepo.IsLocationUnderCycleCount(toLocation.WhsCode, toLocation.LocationCode)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check cycle count status: " + err.Error(),
			})
		}
		if underCount {
			tx.Rollback()
			return ctx.Status(fiber.StatusLocked).JSON(fiber.Map{
				"error": "Location " + toLocation.LocationCode + " is currently under cycle count, transfer is not allowed",
			})
		}

	}

	var inventory models.Inventory
	if err := tx.Where("id = ? AND location = ? AND qty_available > 0", inventoryID, input.FromLocation).First(&inventory).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

	if inventory.QtyAvailable > input.QtyTransfer {
		isSplit = true
	}

	if inventory.Location != input.FromLocation {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Inventory not found or not available"})
	}

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
	newInventory.Pallet = inventory.Pallet
	newInventory.Location = input.ToLocation
	newInventory.QaStatus = inventory.QaStatus
	newInventory.QtyOrigin = float64(input.QtyTransfer)
	newInventory.QtyOnhand = float64(input.QtyTransfer)
	newInventory.QtyAvailable = float64(input.QtyTransfer)
	newInventory.Trans = "TRANSFER"
	newInventory.IsTransfer = true
	newInventory.TransferFrom = inventory.ID
	newInventory.RecDate = inventory.RecDate
	newInventory.ExpDate = inventory.ExpDate
	newInventory.ProdDate = inventory.ProdDate
	newInventory.LotNumber = inventory.LotNumber
	newInventory.InventoryNumber = inventory.InventoryNumber
	newInventory.CreatedAt = time.Now()
	newInventory.CreatedBy = int(ctx.Locals("userID").(float64))

	// if isSplit {
	// 	newPallet, err := invetoryRepo.GeneratePalletID()
	// 	if err != nil {
	// 		tx.Rollback()
	// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// 	}

	// 	newInventory.Pallet = newPallet
	// }

	// Ganti logic isSplit untuk pallet assignment
	if input.ToPallet != "" {
		// User memilih pallet tujuan → pakai langsung, tidak generate baru
		newInventory.Pallet = input.ToPallet
	} else if isSplit {
		newPallet, err := invetoryRepo.GeneratePalletID()
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		newInventory.Pallet = newPallet
	}
	// jika tidak split dan tidak ada to_pallet → pakai pallet lama (behavior existing)

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
		FromDivision:       inventory.DivisionCode,
		ToDivision:         newInventory.DivisionCode,
		FromPallet:         inventory.Pallet,    // ← BARU
		ToPallet:           newInventory.Pallet, // ← BARU
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
		FromDivision:       inventory.DivisionCode,
		ToDivision:         newInventory.DivisionCode,
		FromPallet:         inventory.Pallet,    // ← BARU
		ToPallet:           newInventory.Pallet, // ← BARU
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

func (c *MobileInventoryController) GetPalletsByLocation(ctx *fiber.Ctx) error {
	location := strings.TrimSpace(ctx.Query("location"))
	if location == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "location is required",
		})
	}

	type PalletSummary struct {
		PalletID  string  `json:"pallet_id"`
		TotalQty  float64 `json:"total_qty"`
		TotalSkus int     `json:"total_skus"`
		Uom       string  `json:"uom"`
	}

	var results []PalletSummary

	err := c.DB.
		Model(&models.Inventory{}).
		Select(`
            pallet                        AS pallet_id,
            SUM(qty_available)            AS total_qty,
            COUNT(DISTINCT item_code)     AS total_skus,
            MAX(uom)                      AS uom
        `).
		Where("location = ? AND qty_available > 0 AND pallet != ''", location).
		Group("pallet").
		Order("pallet ASC").
		Scan(&results).Error

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
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

func (c *MobileInventoryController) GetInventoryByItem(ctx *fiber.Ctx) error {

	type request struct {
		Barcode string `json:"barcode"` // bisa EAN atau item_code
	}

	type resultInventory struct {
		ID           int64   `json:"ID"`
		Barcode      string  `json:"barcode"`
		DivisionCode string  `json:"division_code"`
		SerialNumber string  `json:"serial_number"`
		Pallet       string  `json:"pallet"`
		Location     string  `json:"location"`
		QaStatus     string  `json:"qa_status"`
		WhsCode      string  `json:"whs_code"`
		QtyAvailable float64 `json:"qty_available"`
		RecDate      string  `json:"rec_date"`
		LotNumber    string  `json:"lot_number"`
		ProdDate     string  `json:"prod_date"`
		ExpDate      string  `json:"exp_date"`
		QtyDisplay   float64 `json:"qty_display"`
		UomDisplay   string  `json:"uom_display"`
		EanDisplay   string  `json:"ean_display"`
		OwnerCode    string  `json:"owner_code"`
		ItemCode     string  `json:"item_code"`
		ItemName     string  `json:"item_name"`
	}

	var req request
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if req.Barcode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Barcode or item code is required",
		})
	}

	var inventories []resultInventory
	uomRepo := repositories.NewUomRepository(c.DB)

	// ── Coba lookup sebagai EAN dulu via uom_conversion ───────────────────────
	uomConv, err := uomRepo.GetUomConversionByEan(req.Barcode)

	if err == nil && uomConv.BaseEan != "" {
		// ── EAN mode: barcode ditemukan di uom_conversion ─────────────────────
		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				inventories.qty_available / ? AS qty_display,
				? AS uom_display,
				? AS ean_display,
				COALESCE(products.item_name, '') AS item_name`,
				uomConv.Rate, uomConv.Uom, req.Barcode).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("inventories.barcode = ? AND inventories.qty_available > 0", uomConv.BaseEan).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	} else {
		// ── Item code / base EAN mode: fallback langsung ke inventories ───────
		if err := c.DB.
			Table("inventories").
			Select(`inventories.*,
				inventories.qty_available AS qty_display,
				products.uom AS uom_display,
				inventories.barcode AS ean_display,
				COALESCE(products.item_name, '') AS item_name`).
			Joins("LEFT JOIN products ON products.item_code = inventories.item_code").
			Where("(inventories.barcode = ? OR inventories.item_code = ?) AND inventories.qty_available > 0",
				req.Barcode, req.Barcode).
			Find(&inventories).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	if len(inventories) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Item not found",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    inventories,
	})
}

type RegisterProductRequest struct {
	OwnerCode   string `json:"owner_code"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	UnitModel   string `json:"unit_model"`
	Ean         string `json:"ean"`
	Uom         string `json:"uom"`
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
	req.Description = strings.TrimSpace(req.Description)
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
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Product with this combination already exists",
		})
	}

	// Get user ID from context (sesuaikan dengan auth middleware Anda)
	userID := int(ctx.Locals("userID").(float64))

	// Create new product
	newProduct := models.ProductRegister{
		OwnerCode:   req.OwnerCode,
		SKU:         req.SKU,
		UnitModel:   req.UnitModel,
		Description: req.Description,
		Ean:         req.Ean,
		Uom:         req.Uom,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
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

// GetAllProducts - Endpoint untuk mendapatkan semua produk
func (c *MobileInventoryController) GetAllProducts(ctx *fiber.Ctx) error {
	var products []models.ProductRegister

	err := c.DB.
		Table("product_registers").
		Select(`
			product_registers.*,
			users.name AS created_by_name
		`).
		Joins("LEFT JOIN users ON users.id = product_registers.created_by").
		Order("product_registers.created_at DESC").
		Find(&products).Error

	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch products",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Products fetched successfully",
		"data":    products,
	})
}

// GetProductByID - Endpoint untuk mendapatkan produk berdasarkan ID
func (c *MobileInventoryController) GetProductByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var product models.ProductRegister
	if err := c.DB.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Product not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch product",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product fetched successfully",
		"data":    product,
	})
}

// UpdateProduct - Endpoint untuk update produk
func (c *MobileInventoryController) UpdateProduct(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

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
	req.Description = strings.ToUpper(strings.TrimSpace(req.Description))
	req.Ean = strings.ToUpper(strings.TrimSpace(req.Ean))
	req.Uom = strings.ToUpper(strings.TrimSpace(req.Uom))

	// Cek apakah produk ada
	var product models.ProductRegister
	if err := c.DB.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Product not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch product",
		})
	}

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

	// Cek apakah kombinasi sudah ada di produk lain (bukan produk yang sedang diupdate)
	var existingProduct models.ProductRegister
	err := c.DB.Where("owner_code = ? AND sku = ? AND unit_model = ? AND ean = ? AND uom = ? AND id != ?",
		req.OwnerCode, req.SKU, req.UnitModel, req.Ean, req.Uom, id).
		First(&existingProduct).Error

	if err == nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Product with this combination already exists",
		})
	}

	// Get user ID from context
	userID := int(ctx.Locals("userID").(float64))

	// Update product
	product.OwnerCode = req.OwnerCode
	product.SKU = req.SKU
	product.UnitModel = req.UnitModel
	product.Description = req.Description
	product.Ean = req.Ean
	product.Uom = req.Uom
	product.UpdatedBy = userID
	product.UpdatedAt = time.Now()

	if err := c.DB.Save(&product).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update product",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product updated successfully",
		"data":    product,
	})
}

// DeleteProduct - Endpoint untuk hapus produk
func (c *MobileInventoryController) DeleteProduct(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	// Cek apakah produk ada
	var product models.ProductRegister
	if err := c.DB.First(&product, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Product not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch product",
		})
	}

	// Hapus produk
	if err := c.DB.Delete(&product).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete product",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Product deleted successfully",
		"data":    product,
	})
}

func (c *MobileInventoryController) GetTransferHistory(ctx *fiber.Ctx) error {

	type request struct {
		DateFrom string `json:"date_from"` // format: YYYY-MM-DD
		DateTo   string `json:"date_to"`   // format: YYYY-MM-DD
	}

	type TransferRecord struct {
		ItemCode     string  `json:"item_code"`
		ItemName     string  `json:"item_name"`
		FromDivision string  `json:"from_division"`
		ToDivision   string  `json:"to_division"`
		FromLocation string  `json:"from_location"`
		ToLocation   string  `json:"to_location"`
		OldQaStatus  string  `json:"old_qa_status"`
		NewQaStatus  string  `json:"new_qa_status"`
		Qty          float64 `json:"qty"`
		Username     string  `json:"username"`
		CreatedBy    int64   `json:"created_by"`
		CreatedAt    string  `json:"created_at"`
	}

	var req request
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if req.DateFrom == "" || req.DateTo == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "date_from and date_to are required",
		})
	}

	var results []TransferRecord

	query := `
		SELECT
			a.item_code,
			COALESCE(b.item_name, '') AS item_name,
			COALESCE(a.from_division, '') AS from_division,
			COALESCE(a.to_division, '') AS to_division,
			COALESCE(a.from_location, '') AS from_location,
			COALESCE(a.to_location, '') AS to_location,
			COALESCE(a.old_qa_status, '') AS old_qa_status,
			COALESCE(a.new_qa_status, '') AS new_qa_status,
			SUM(a.qty_available_change) AS qty,
			COALESCE(c.username, '') AS username,
			a.created_by,
			CAST(a.created_at AS DATE) AS created_at
		FROM inventory_movements a
		LEFT JOIN products b ON a.item_code = b.item_code
		LEFT JOIN users c ON a.created_by = c.id
		WHERE a.ref_type = 'TRANSFER'
			AND a.qty_onhand_change > 0
			AND CAST(a.created_at AS DATE) BETWEEN ? AND ?
		GROUP BY
			a.item_code,
			b.item_name,
			a.from_division,
			a.to_division,
			a.from_location,
			a.to_location,
			a.old_qa_status,
			a.new_qa_status,
			c.username,
			a.created_by,
			CAST(a.created_at AS DATE)
		ORDER BY
			CAST(a.created_at AS DATE) DESC
	`

	if err := c.DB.Raw(query, req.DateFrom, req.DateTo).Scan(&results).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if len(results) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "No transfer records found for the selected date range",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}
