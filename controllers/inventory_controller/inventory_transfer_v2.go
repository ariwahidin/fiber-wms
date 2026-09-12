package inventory_controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fiber-app/models"
	"fiber-app/repositories"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// INTERNAL INVENTORY TRANSFER
// ============================================================================
// One endpoint supports both inventory types:
//   1. Serial inventory  -> serial_numbers is supplied.
//   2. Non-serial inventory -> qty_to_transfer is supplied.
//
// Product.HasSerial is intentionally NOT used.
// InventorySerial is the source of truth when serial rows exist.
// InboundBarcode / InboundSerial are never modified by this transfer.
// ============================================================================

type InternalTransferInventory struct {
	models.Inventory
	AvailableSerials int    `json:"available_serials"`
	TransferMode     string `json:"transfer_mode"` // serial | quantity
}

type InternalTransferInput struct {
	InventoryID   uint     `json:"inventory_id" validate:"required"`
	ToWhsCode     string   `json:"to_whs_code" validate:"required"`
	ToLocation    string   `json:"to_location" validate:"required"`
	DivisionCode  string   `json:"division_code" validate:"required"`
	NewQaStatus   string   `json:"new_qa_status"`
	LotNumber     string   `json:"lot_number"`
	Reason        string   `json:"reason"`
	QtyToTransfer float64  `json:"qty_to_transfer"`
	SerialNumbers []string `json:"serial_numbers"`
}

// GET /inventory/internal-transfer/inventories
//
// Returns both serial and non-serial inventory records that have available
// stock. For records that have InventorySerial rows, transfer_mode is serial.
func (c *InventoryController) GetInternalTransferInventories(ctx *fiber.Ctx) error {
	itemCode := strings.TrimSpace(ctx.Query("item_code"))
	ownerCode := strings.TrimSpace(ctx.Query("owner_code"))
	whsCode := strings.TrimSpace(ctx.Query("whs_code"))
	location := strings.TrimSpace(ctx.Query("location"))
	divisionCode := strings.TrimSpace(ctx.Query("division_code"))
	qaStatus := strings.TrimSpace(ctx.Query("qa_status"))
	pallet := strings.TrimSpace(ctx.Query("pallet"))
	search := strings.TrimSpace(ctx.Query("search"))

	query := c.DB.Model(&models.Inventory{}).
		Preload("Product").
		Where("inventories.qty_available > ?", 0).
		Where("inventories.deleted_at IS NULL")

	if itemCode != "" {
		query = query.Where("inventories.item_code = ?", itemCode)
	}
	if ownerCode != "" {
		query = query.Where("inventories.owner_code = ?", ownerCode)
	}
	if whsCode != "" {
		query = query.Where("inventories.whs_code = ?", whsCode)
	}
	if location != "" {
		query = query.Where("inventories.location = ?", location)
	}
	if divisionCode != "" {
		query = query.Where("inventories.division_code = ?", divisionCode)
	}
	if qaStatus != "" {
		query = query.Where("inventories.qa_status = ?", qaStatus)
	}
	if pallet != "" {
		query = query.Where("inventories.pallet = ?", pallet)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(`(
            inventories.item_code LIKE ? OR
            inventories.barcode LIKE ? OR
            inventories.pallet LIKE ? OR
            inventories.location LIKE ? OR
            inventories.carton_number LIKE ? OR
            inventories.case_number LIKE ?
        )`, like, like, like, like, like, like)
	}

	var inventories []models.Inventory
	if err := query.
		Order("inventories.item_code ASC, inventories.whs_code ASC, inventories.location ASC, inventories.id ASC").
		Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch transfer inventories: " + err.Error(),
		})
	}

	if len(inventories) == 0 {
		return ctx.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"inventories": []InternalTransferInventory{},
				"total":       0,
			},
		})
	}

	ids := make([]uint, 0, len(inventories))
	for _, inv := range inventories {
		ids = append(ids, inv.ID)
	}

	type serialStat struct {
		InventoryID uint `gorm:"column:inventory_id"`
		Total       int  `gorm:"column:total"`
		AllRows     int  `gorm:"column:all_rows"`
	}

	// Count available serials and all active serial rows separately.
	// AllRows is used to distinguish a true non-serial inventory from a
	// serial inventory whose available serial count has reached zero.
	var stats []serialStat
	if err := c.DB.Model(&models.InventorySerial{}).
		Select(`
            inventory_id,
            SUM(CASE WHEN qty_available > 0 THEN 1 ELSE 0 END) AS total,
            COUNT(*) AS all_rows
        `).
		Where("inventory_id IN ?", ids).
		Where("deleted_at IS NULL").
		Group("inventory_id").
		Scan(&stats).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to count inventory serials: " + err.Error(),
		})
	}

	statMap := make(map[uint]serialStat, len(stats))
	for _, stat := range stats {
		statMap[stat.InventoryID] = stat
	}

	result := make([]InternalTransferInventory, 0, len(inventories))
	for _, inv := range inventories {
		stat := statMap[inv.ID]
		mode := "quantity"
		if stat.AllRows > 0 {
			mode = "serial"
		}

		result = append(result, InternalTransferInventory{
			Inventory:        inv,
			AvailableSerials: stat.Total,
			TransferMode:     mode,
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"inventories": result,
			"total":       len(result),
		},
	})
}

// GET /inventory/internal-transfer/serials?inventory_id=123
func (c *InventoryController) GetInternalTransferSerials(ctx *fiber.Ctx) error {
	inventoryID := ctx.QueryInt("inventory_id", 0)
	if inventoryID <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "inventory_id is required",
		})
	}

	var inventory models.Inventory
	if err := c.DB.Where("id = ?", inventoryID).First(&inventory).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Inventory not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch inventory: " + err.Error(),
		})
	}

	var serials []models.InventorySerial
	if err := c.DB.
		Where("inventory_id = ?", inventoryID).
		Where("deleted_at IS NULL").
		Where("qty_available > 0").
		Order("serial_number ASC").
		Find(&serials).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch inventory serials: " + err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"inventory": inventory,
			"serials":   serials,
			"total":     len(serials),
		},
	})
}

// POST /inventory/internal-transfer
func (c *InventoryController) TransferInventoryInternal(ctx *fiber.Ctx) error {
	var input InternalTransferInput
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	input.ToWhsCode = strings.TrimSpace(input.ToWhsCode)
	input.ToLocation = strings.TrimSpace(input.ToLocation)
	input.DivisionCode = strings.TrimSpace(input.DivisionCode)
	input.NewQaStatus = strings.TrimSpace(input.NewQaStatus)
	input.LotNumber = strings.TrimSpace(input.LotNumber)
	input.Reason = strings.TrimSpace(input.Reason)

	if input.InventoryID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "inventory_id is required",
		})
	}
	if input.ToWhsCode == "" || input.ToLocation == "" || input.DivisionCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Destination warehouse, location and division are required",
		})
	}

	serials := normalizeInternalTransferSerials(input.SerialNumbers)

	userID, ok := internalTransferUserID(ctx)
	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to begin transaction: " + tx.Error.Error(),
		})
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var source models.Inventory
	if err := tx.Where("id = ? AND deleted_at IS NULL", input.InventoryID).First(&source).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Source inventory not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch source inventory: " + err.Error(),
		})
	}

	if source.QtyAvailable <= 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Source inventory has no available quantity",
		})
	}

	// Determine transfer mode from InventorySerial itself.
	// We do not use Product.HasSerial.
	var activeSerialCount int64
	if err := tx.Model(&models.InventorySerial{}).
		Where("inventory_id = ? AND deleted_at IS NULL", source.ID).
		Count(&activeSerialCount).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to determine inventory transfer mode: " + err.Error(),
		})
	}

	isSerialInventory := activeSerialCount > 0

	if isSerialInventory {
		if len(serials) == 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "This inventory contains serial numbers. Select at least one serial number.",
			})
		}

		if input.QtyToTransfer > 0 && input.QtyToTransfer != float64(len(serials)) {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "qty_to_transfer must equal the number of selected serial numbers",
			})
		}

		result, err := transferSerialInternal(tx, source, input, serials, userID)
		if err != nil {
			tx.Rollback()
			return internalTransferError(ctx, err)
		}

		return ctx.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Successfully transferred %d serial(s)", len(serials)),
			"data":    result,
		})
	}

	// Non-serial inventory must not receive serial numbers.
	if len(serials) > 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "This inventory has no InventorySerial rows. Transfer it by quantity instead.",
		})
	}

	if input.QtyToTransfer <= 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "qty_to_transfer must be greater than 0 for non-serial inventory",
		})
	}

	if input.QtyToTransfer > source.QtyAvailable {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Transfer quantity %.4f exceeds available quantity %.4f", input.QtyToTransfer, source.QtyAvailable),
		})
	}

	result, err := transferQuantityInternal(tx, source, input, input.QtyToTransfer, userID)
	if err != nil {
		tx.Rollback()
		return internalTransferError(ctx, err)
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Successfully transferred %.4f unit(s)", input.QtyToTransfer),
		"data":    result,
	})
}

func transferSerialInternal(tx *gorm.DB, source models.Inventory, input InternalTransferInput, serials []string, userID int) (map[string]interface{}, error) {
	qty := float64(len(serials))

	var rows []models.InventorySerial
	if err := tx.
		Where("inventory_id = ?", source.ID).
		Where("serial_number IN ?", serials).
		Where("deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to validate source serials: %w", err)
	}

	rowMap := make(map[string]models.InventorySerial, len(rows))
	for _, row := range rows {
		sn := strings.TrimSpace(row.SerialNumber)
		if _, exists := rowMap[sn]; exists {
			return nil, fmt.Errorf("duplicate serial in source inventory: %s", sn)
		}
		rowMap[sn] = row
	}

	for _, sn := range serials {
		row, exists := rowMap[sn]
		if !exists {
			return nil, fmt.Errorf("serial %s is not available in the selected inventory", sn)
		}
		if row.QtyAvailable != 1 {
			return nil, fmt.Errorf("serial %s must have QtyAvailable = 1", sn)
		}
		if row.QtyAllocated > 0 || row.QtyShipped > 0 {
			return nil, fmt.Errorf("serial %s is allocated or shipped", sn)
		}
	}

	newQa := input.NewQaStatus
	if newQa == "" {
		newQa = source.QaStatus
	}
	newLot := input.LotNumber
	if newLot == "" {
		newLot = source.LotNumber
	}

	if sameInternalTransferDestination(source, input, newQa, newLot) {
		return nil, errors.New("destination is the same as source")
	}

	destination, isNew, err := findInternalDestination(tx, source, input, newQa, newLot)
	if err != nil {
		return nil, fmt.Errorf("failed to find destination inventory: %w", err)
	}

	if !isNew {
		var duplicateCount int64
		if err := tx.Model(&models.InventorySerial{}).
			Where("inventory_id = ? AND serial_number IN ? AND deleted_at IS NULL", destination.ID, serials).
			Count(&duplicateCount).Error; err != nil {
			return nil, fmt.Errorf("failed to validate destination serials: %w", err)
		}
		if duplicateCount > 0 {
			return nil, errors.New("one or more selected serials already exist at destination")
		}
	}

	sourceOnhandBefore := source.QtyOnhand
	sourceAvailableBefore := source.QtyAvailable
	sourceOnhandAfter := sourceOnhandBefore - qty
	sourceAvailableAfter := sourceAvailableBefore - qty

	if sourceOnhandAfter < 0 || sourceAvailableAfter < 0 {
		return nil, errors.New("source inventory quantity would become negative")
	}

	if err := tx.Model(&source).Updates(map[string]interface{}{
		"qty_origin":    gorm.Expr("qty_origin - ?", qty),
		"qty_onhand":    sourceOnhandAfter,
		"qty_available": sourceAvailableAfter,
		"updated_by":    userID,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to update source inventory: %w", err)
	}

	destinationID, toPallet, destOnhandBefore, destAvailableBefore, destOnhandAfter, destAvailableAfter, err :=
		upsertInternalDestination(tx, source, destination, isNew, input, newQa, newLot, qty, userID)
	if err != nil {
		return nil, err
	}

	for _, sn := range serials {
		row := rowMap[sn]
		if err := tx.Model(&models.InventorySerial{}).
			Where("id = ? AND inventory_id = ? AND deleted_at IS NULL", row.ID, source.ID).
			Updates(map[string]interface{}{
				"inventory_id": destinationID,
				"updated_at":   time.Now().UTC(),
				"updated_by":   userID,
			}).Error; err != nil {
			return nil, fmt.Errorf("failed to move serial %s: %w", sn, err)
		}
	}

	movementID := uuid.NewString()
	if err := createInternalMovements(
		tx,
		movementID,
		source,
		destinationID,
		input,
		newQa,
		newLot,
		toPallet,
		-qty,
		qty,
		sourceOnhandBefore,
		sourceOnhandAfter,
		sourceAvailableBefore,
		sourceAvailableAfter,
		destOnhandBefore,
		destOnhandAfter,
		destAvailableBefore,
		destAvailableAfter,
		userID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transfer: %w", err)
	}

	return map[string]interface{}{
		"movement_id":              movementID,
		"source_inventory_id":      source.ID,
		"destination_inventory_id": destinationID,
		"serial_numbers":           serials,
		"quantity":                 qty,
	}, nil
}

func transferQuantityInternal(tx *gorm.DB, source models.Inventory, input InternalTransferInput, qty float64, userID int) (map[string]interface{}, error) {
	newQa := input.NewQaStatus
	if newQa == "" {
		newQa = source.QaStatus
	}
	newLot := input.LotNumber
	if newLot == "" {
		newLot = source.LotNumber
	}

	if sameInternalTransferDestination(source, input, newQa, newLot) {
		return nil, errors.New("destination is the same as source")
	}

	destination, isNew, err := findInternalDestination(tx, source, input, newQa, newLot)
	if err != nil {
		return nil, fmt.Errorf("failed to find destination inventory: %w", err)
	}

	sourceOnhandBefore := source.QtyOnhand
	sourceAvailableBefore := source.QtyAvailable
	sourceOnhandAfter := sourceOnhandBefore - qty
	sourceAvailableAfter := sourceAvailableBefore - qty

	if sourceOnhandAfter < 0 || sourceAvailableAfter < 0 {
		return nil, errors.New("source inventory quantity would become negative")
	}

	if err := tx.Model(&source).Updates(map[string]interface{}{
		"qty_origin":    gorm.Expr("qty_origin - ?", qty),
		"qty_onhand":    sourceOnhandAfter,
		"qty_available": sourceAvailableAfter,
		"updated_by":    userID,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to update source inventory: %w", err)
	}

	destinationID, toPallet, destOnhandBefore, destAvailableBefore, destOnhandAfter, destAvailableAfter, err :=
		upsertInternalDestination(tx, source, destination, isNew, input, newQa, newLot, qty, userID)
	if err != nil {
		return nil, err
	}

	movementID := uuid.NewString()
	if err := createInternalMovements(
		tx,
		movementID,
		source,
		destinationID,
		input,
		newQa,
		newLot,
		toPallet,
		-qty,
		qty,
		sourceOnhandBefore,
		sourceOnhandAfter,
		sourceAvailableBefore,
		sourceAvailableAfter,
		destOnhandBefore,
		destOnhandAfter,
		destAvailableBefore,
		destAvailableAfter,
		userID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transfer: %w", err)
	}

	return map[string]interface{}{
		"movement_id":              movementID,
		"source_inventory_id":      source.ID,
		"destination_inventory_id": destinationID,
		"quantity":                 qty,
	}, nil
}

func findInternalDestination(tx *gorm.DB, source models.Inventory, input InternalTransferInput, newQa, newLot string) (models.Inventory, bool, error) {
	var destination models.Inventory

	query := tx.Where(
		"whs_code = ? AND location = ? AND item_code = ? AND owner_code = ? AND barcode = ? AND qa_status = ? AND lot_number = ?",
		input.ToWhsCode,
		input.ToLocation,
		source.ItemCode,
		source.OwnerCode,
		source.Barcode,
		newQa,
		newLot,
	)

	if source.RecDate != "" {
		query = query.Where("rec_date = ?", source.RecDate)
	}
	if source.ProdDate != "" {
		query = query.Where("prod_date = ?", source.ProdDate)
	}
	if source.ExpDate != "" {
		query = query.Where("exp_date = ?", source.ExpDate)
	}
	query = query.Where("division_code = ?", input.DivisionCode)

	if source.CartonNumber != "" {
		query = query.Where("COALESCE(carton_number, '') = ?", source.CartonNumber)
	}
	if source.CaseNumber != "" {
		query = query.Where("COALESCE(case_number, '') = ?", source.CaseNumber)
	}

	err := query.First(&destination).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Inventory{}, true, nil
	}
	if err != nil {
		return models.Inventory{}, false, err
	}

	return destination, false, nil
}

func upsertInternalDestination(
	tx *gorm.DB,
	source models.Inventory,
	destination models.Inventory,
	isNew bool,
	input InternalTransferInput,
	newQa string,
	newLot string,
	qty float64,
	userID int,
) (uint, string, float64, float64, float64, float64, error) {
	repo := repositories.NewInventoryRepository(tx)

	toPallet := source.Pallet
	if isNew && source.QtyAvailable > qty {
		generated, err := repo.GeneratePalletID()
		if err != nil {
			return 0, "", 0, 0, 0, 0, fmt.Errorf("failed to generate destination pallet: %w", err)
		}
		toPallet = generated
	} else if !isNew {
		toPallet = destination.Pallet
	}

	if isNew {
		newInventory := models.Inventory{
			OwnerCode:       source.OwnerCode,
			WhsCode:         input.ToWhsCode,
			InboundID:       source.InboundID,
			InboundDetailId: source.InboundDetailId,
			DivisionCode:    input.DivisionCode,
			RecDate:         source.RecDate,
			ProdDate:        source.ProdDate,
			ExpDate:         source.ExpDate,
			LotNumber:       newLot,
			Pallet:          toPallet,
			Location:        input.ToLocation,
			ItemId:          source.ItemId,
			ItemCode:        source.ItemCode,
			Barcode:         source.Barcode,
			CaseNumber:      source.CaseNumber,
			CartonNumber:    source.CartonNumber,
			SerialNumber:    "",
			QaStatus:        newQa,
			Uom:             source.Uom,
			QtyOrigin:       qty,
			QtyOnhand:       qty,
			QtyAvailable:    qty,
			QtyAllocated:    0,
			QtySuspend:      0,
			QtyShipped:      0,
			Trans:           "TRANSFER",
			IsTransfer:      true,
			TransferFrom:    source.ID,
			CreatedBy:       userID,
			UpdatedBy:       userID,
		}

		if err := tx.Create(&newInventory).Error; err != nil {
			return 0, "", 0, 0, 0, 0, fmt.Errorf("failed to create destination inventory: %w", err)
		}

		return newInventory.ID, toPallet, 0, 0, qty, qty, nil
	}

	destOnhandBefore := destination.QtyOnhand
	destAvailableBefore := destination.QtyAvailable
	destOnhandAfter := destOnhandBefore + qty
	destAvailableAfter := destAvailableBefore + qty

	if err := tx.Model(&destination).Updates(map[string]interface{}{
		"qty_origin":    gorm.Expr("qty_origin + ?", qty),
		"qty_onhand":    destOnhandAfter,
		"qty_available": destAvailableAfter,
		"updated_by":    userID,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return 0, "", 0, 0, 0, 0, fmt.Errorf("failed to update destination inventory: %w", err)
	}

	return destination.ID, toPallet, destOnhandBefore, destAvailableBefore, destOnhandAfter, destAvailableAfter, nil
}

func createInternalMovements(
	tx *gorm.DB,
	movementID string,
	source models.Inventory,
	destinationID uint,
	input InternalTransferInput,
	newQa string,
	newLot string,
	toPallet string,
	sourceChange float64,
	destinationChange float64,
	sourceOnhandBefore float64,
	sourceOnhandAfter float64,
	sourceAvailableBefore float64,
	sourceAvailableAfter float64,
	destOnhandBefore float64,
	destOnhandAfter float64,
	destAvailableBefore float64,
	destAvailableAfter float64,
	userID int,
) error {
	sourceMovement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        source.ID,
		RefType:            "TRANSFER",
		RefID:              destinationID,
		ItemID:             source.ItemId,
		ItemCode:           source.ItemCode,
		QtyOnhandChange:    sourceChange,
		QtyAvailableChange: sourceChange,
		QtyAllocatedChange: 0,
		QtySuspendChange:   0,
		QtyShippedChange:   0,
		QtyOnhandBefore:    &sourceOnhandBefore,
		QtyOnhandAfter:     &sourceOnhandAfter,
		QtyAvailableBefore: &sourceAvailableBefore,
		QtyAvailableAfter:  &sourceAvailableAfter,
		OwnerCode:          source.OwnerCode,
		FromWhsCode:        source.WhsCode,
		ToWhsCode:          input.ToWhsCode,
		FromLocation:       source.Location,
		ToLocation:         input.ToLocation,
		OldQaStatus:        source.QaStatus,
		NewQaStatus:        newQa,
		FromDivision:       source.DivisionCode,
		ToDivision:         input.DivisionCode,
		FromPallet:         source.Pallet,
		ToPallet:           toPallet,
		FromLotNumber:      source.LotNumber,
		ToLotNumber:        newLot,
		Reason:             input.Reason,
		CreatedBy:          userID,
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(&sourceMovement).Error; err != nil {
		return fmt.Errorf("failed to create source movement: %w", err)
	}

	destMovement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        destinationID,
		RefType:            "TRANSFER",
		RefID:              source.ID,
		ItemID:             source.ItemId,
		ItemCode:           source.ItemCode,
		QtyOnhandChange:    destinationChange,
		QtyAvailableChange: destinationChange,
		QtyAllocatedChange: 0,
		QtySuspendChange:   0,
		QtyShippedChange:   0,
		QtyOnhandBefore:    &destOnhandBefore,
		QtyOnhandAfter:     &destOnhandAfter,
		QtyAvailableBefore: &destAvailableBefore,
		QtyAvailableAfter:  &destAvailableAfter,
		OwnerCode:          source.OwnerCode,
		FromWhsCode:        source.WhsCode,
		ToWhsCode:          input.ToWhsCode,
		FromLocation:       source.Location,
		ToLocation:         input.ToLocation,
		OldQaStatus:        source.QaStatus,
		NewQaStatus:        newQa,
		FromDivision:       source.DivisionCode,
		ToDivision:         input.DivisionCode,
		FromPallet:         source.Pallet,
		ToPallet:           toPallet,
		FromLotNumber:      source.LotNumber,
		ToLotNumber:        newLot,
		Reason:             input.Reason,
		CreatedBy:          userID,
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(&destMovement).Error; err != nil {
		return fmt.Errorf("failed to create destination movement: %w", err)
	}

	return nil
}

func sameInternalTransferDestination(source models.Inventory, input InternalTransferInput, newQa, newLot string) bool {
	return source.WhsCode == input.ToWhsCode &&
		source.Location == input.ToLocation &&
		source.DivisionCode == input.DivisionCode &&
		source.QaStatus == newQa &&
		source.LotNumber == newLot
}

func normalizeInternalTransferSerials(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func internalTransferUserID(ctx *fiber.Ctx) (int, bool) {
	value := ctx.Locals("userID")

	switch v := value.(type) {
	case int:
		return v, v > 0
	case int32:
		return int(v), v > 0
	case int64:
		return int(v), v > 0
	case uint:
		return int(v), v > 0
	case uint32:
		return int(v), v > 0
	case uint64:
		return int(v), v > 0
	case float64:
		return int(v), v > 0
	case float32:
		return int(v), v > 0
	default:
		return 0, false
	}
}

func internalTransferError(ctx *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := err.Error()

	lower := strings.ToLower(message)
	if strings.Contains(lower, "not found") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "available") ||
		strings.Contains(lower, "destination is the same") ||
		strings.Contains(lower, "qty") ||
		strings.Contains(lower, "allocated") ||
		strings.Contains(lower, "shipped") ||
		strings.Contains(lower, "serial") ||
		strings.Contains(lower, "negative") {
		status = fiber.StatusBadRequest
	}
	if strings.Contains(lower, "already exist") || strings.Contains(lower, "duplicate") {
		status = fiber.StatusConflict
	}

	return ctx.Status(status).JSON(fiber.Map{
		"success": false,
		"error":   message,
	})
}

// Keep this compile-time reference so accidental removal of the repository
// dependency is caught while this controller is edited.
var _ = repositories.NewInventoryRepository
