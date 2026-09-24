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
// CHANGE ITEM
// ============================================================================
//
// Change Item:
//     Source Item A -> Target Item B
//
// Physical warehouse/location/etc remain the same.
//
// SERIAL:
//     InventorySerial is the source of truth.
//     Selected serial rows are moved from source inventory to destination.
//
// NON-SERIAL:
//     Quantity is deducted from source inventory and added to destination.
//
// IMPORTANT:
//     Product.HasSerial is NOT used.
//     If active InventorySerial rows exist, inventory is treated as SERIAL.
// ============================================================================

type ChangeItemInventory struct {
	models.Inventory
	AvailableSerials int    `json:"available_serials"`
	ChangeMode       string `json:"change_mode"` // serial | quantity
}

type ChangeItemInput struct {
	InventoryID    uint     `json:"inventory_id" validate:"required"`
	TargetItemID   uint     `json:"target_item_id" validate:"required"`
	TargetItemCode string   `json:"target_item_code"`
	QtyToChange    float64  `json:"qty_to_change"`
	SerialNumbers  []string `json:"serial_numbers"`
	Reason         string   `json:"reason"`
}

// ============================================================================
// GET SOURCE INVENTORIES
// ============================================================================

// GET /inventory/change-item/inventories
func (c *InventoryController) GetChangeItemInventories(ctx *fiber.Ctx) error {
	itemCode := strings.TrimSpace(ctx.Query("item_code"))
	ownerCode := strings.TrimSpace(ctx.Query("owner_code"))
	whsCode := strings.TrimSpace(ctx.Query("whs_code"))
	location := strings.TrimSpace(ctx.Query("location"))
	divisionCode := strings.TrimSpace(ctx.Query("division_code"))
	qaStatus := strings.TrimSpace(ctx.Query("qa_status"))
	pallet := strings.TrimSpace(ctx.Query("pallet"))
	search := strings.TrimSpace(ctx.Query("search"))

	query := c.DB.
		Model(&models.Inventory{}).
		Preload("Product").
		Where("inventories.deleted_at IS NULL").
		Where("inventories.qty_available > ?", 0)

	// =========================
	// FILTER
	// =========================

	if itemCode != "" {
		query = query.Where(
			"inventories.item_code = ?",
			itemCode,
		)
	}

	if ownerCode != "" {
		query = query.Where(
			"inventories.owner_code = ?",
			ownerCode,
		)
	}

	if whsCode != "" {
		query = query.Where(
			"inventories.whs_code = ?",
			whsCode,
		)
	}

	if location != "" {
		query = query.Where(
			"inventories.location = ?",
			location,
		)
	}

	if divisionCode != "" {
		query = query.Where(
			"inventories.division_code = ?",
			divisionCode,
		)
	}

	if qaStatus != "" {
		query = query.Where(
			"inventories.qa_status = ?",
			qaStatus,
		)
	}

	if pallet != "" {
		query = query.Where(
			"inventories.pallet = ?",
			pallet,
		)
	}

	// =========================
	// SEARCH
	// =========================

	if search != "" {
		like := "%" + search + "%"

		query = query.Where(`
			(
				inventories.item_code LIKE ? OR
				inventories.barcode LIKE ? OR
				inventories.pallet LIKE ? OR
				inventories.location LIKE ? OR
				inventories.carton_number LIKE ? OR
				inventories.case_number LIKE ? OR
				EXISTS (
					SELECT 1
					FROM products
					WHERE products.item_code = inventories.item_code
					  AND products.unit_model LIKE ?
					  AND products.deleted_at IS NULL
				)
			)
		`,
			like,
			like,
			like,
			like,
			like,
			like,
			like,
		)
	}

	// =========================
	// FETCH INVENTORY
	// =========================

	var inventories []models.Inventory

	if err := query.
		Order(`
			inventories.item_code ASC,
			inventories.whs_code ASC,
			inventories.location ASC,
			inventories.id ASC
		`).
		Find(&inventories).Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch change item inventories: " + err.Error(),
		})
	}

	// =========================
	// EMPTY RESULT
	// =========================

	if len(inventories) == 0 {
		return ctx.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"inventories": []ChangeItemInventory{},
				"total":       0,
			},
		})
	}

	// =========================
	// GET INVENTORY IDS
	// =========================

	ids := make([]uint, 0, len(inventories))

	for _, inventory := range inventories {
		ids = append(ids, inventory.ID)
	}

	// =========================
	// DETERMINE INVENTORY MODE
	// =========================

	type serialStat struct {
		InventoryID uint `gorm:"column:inventory_id"`
		Total       int  `gorm:"column:total"`
		AllRows     int  `gorm:"column:all_rows"`
	}

	// SQL Server max 2100 parameter per query, jadi di-chunk
	const chunkSize = 1000

	statMap := make(map[uint]serialStat, len(ids))

	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}

		var stats []serialStat

		if err := c.DB.
			Model(&models.InventorySerial{}).
			Select(`
				inventory_id,
				SUM(
					CASE
						WHEN qty_available > 0 THEN 1
						ELSE 0
					END
				) AS total,
				COUNT(*) AS all_rows
			`).
			Where("inventory_id IN ?", ids[start:end]).
			Where("deleted_at IS NULL").
			Group("inventory_id").
			Scan(&stats).Error; err != nil {

			return ctx.Status(
				fiber.StatusInternalServerError,
			).JSON(fiber.Map{
				"success": false,
				"error": "Failed to determine inventory mode: " +
					err.Error(),
			})
		}

		// =========================
		// BUILD SERIAL STAT MAP
		// =========================

		for _, stat := range stats {
			statMap[stat.InventoryID] = stat
		}
	}

	// =========================
	// BUILD RESULT
	// =========================

	result := make(
		[]ChangeItemInventory,
		0,
		len(inventories),
	)

	for _, inventory := range inventories {

		stat := statMap[inventory.ID]

		mode := "quantity"

		// Jika inventory mempunyai
		// inventory_serial record,
		// maka mode = serial.
		if stat.AllRows > 0 {
			mode = "serial"
		}

		result = append(
			result,
			ChangeItemInventory{
				Inventory:        inventory,
				AvailableSerials: stat.Total,
				ChangeMode:       mode,
			},
		)
	}

	// =========================
	// RESPONSE
	// =========================

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"inventories": result,
			"total":       len(result),
		},
	})
}

// GET /inventory/change-item/inventories
// func (c *InventoryController) GetChangeItemInventories(ctx *fiber.Ctx) error {
// 	itemCode := strings.TrimSpace(ctx.Query("item_code"))
// 	ownerCode := strings.TrimSpace(ctx.Query("owner_code"))
// 	whsCode := strings.TrimSpace(ctx.Query("whs_code"))
// 	location := strings.TrimSpace(ctx.Query("location"))
// 	divisionCode := strings.TrimSpace(ctx.Query("division_code"))
// 	qaStatus := strings.TrimSpace(ctx.Query("qa_status"))
// 	pallet := strings.TrimSpace(ctx.Query("pallet"))
// 	search := strings.TrimSpace(ctx.Query("search"))

// 	query := c.DB.
// 		Model(&models.Inventory{}).
// 		Preload("Product").
// 		Where("inventories.deleted_at IS NULL").
// 		Where("inventories.qty_available > ?", 0)

// 	// =========================
// 	// FILTER
// 	// =========================

// 	if itemCode != "" {
// 		query = query.Where(
// 			"inventories.item_code = ?",
// 			itemCode,
// 		)
// 	}

// 	if ownerCode != "" {
// 		query = query.Where(
// 			"inventories.owner_code = ?",
// 			ownerCode,
// 		)
// 	}

// 	if whsCode != "" {
// 		query = query.Where(
// 			"inventories.whs_code = ?",
// 			whsCode,
// 		)
// 	}

// 	if location != "" {
// 		query = query.Where(
// 			"inventories.location = ?",
// 			location,
// 		)
// 	}

// 	if divisionCode != "" {
// 		query = query.Where(
// 			"inventories.division_code = ?",
// 			divisionCode,
// 		)
// 	}

// 	if qaStatus != "" {
// 		query = query.Where(
// 			"inventories.qa_status = ?",
// 			qaStatus,
// 		)
// 	}

// 	if pallet != "" {
// 		query = query.Where(
// 			"inventories.pallet = ?",
// 			pallet,
// 		)
// 	}

// 	// =========================
// 	// SEARCH
// 	// =========================

// 	if search != "" {
// 		like := "%" + search + "%"

// 		query = query.Where(`
// 			(
// 				inventories.item_code LIKE ? OR
// 				inventories.barcode LIKE ? OR
// 				inventories.pallet LIKE ? OR
// 				inventories.location LIKE ? OR
// 				inventories.carton_number LIKE ? OR
// 				inventories.case_number LIKE ? OR
// 				EXISTS (
// 					SELECT 1
// 					FROM products
// 					WHERE products.item_code = inventories.item_code
// 					  AND products.unit_model LIKE ?
// 					  AND products.deleted_at IS NULL
// 				)
// 			)
// 		`,
// 			like,
// 			like,
// 			like,
// 			like,
// 			like,
// 			like,
// 			like,
// 		)
// 	}

// 	// =========================
// 	// FETCH INVENTORY
// 	// =========================

// 	var inventories []models.Inventory

// 	if err := query.
// 		Order(`
// 			inventories.item_code ASC,
// 			inventories.whs_code ASC,
// 			inventories.location ASC,
// 			inventories.id ASC
// 		`).
// 		Find(&inventories).Error; err != nil {

// 		return ctx.Status(
// 			fiber.StatusInternalServerError,
// 		).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to fetch change item inventories: " + err.Error(),
// 		})
// 	}

// 	// =========================
// 	// EMPTY RESULT
// 	// =========================

// 	if len(inventories) == 0 {
// 		return ctx.JSON(fiber.Map{
// 			"success": true,
// 			"data": fiber.Map{
// 				"inventories": []ChangeItemInventory{},
// 				"total":       0,
// 			},
// 		})
// 	}

// 	// =========================
// 	// GET INVENTORY IDS
// 	// =========================

// 	ids := make([]uint, 0, len(inventories))

// 	for _, inventory := range inventories {
// 		ids = append(ids, inventory.ID)
// 	}

// 	// =========================
// 	// DETERMINE INVENTORY MODE
// 	// =========================

// 	type serialStat struct {
// 		InventoryID uint `gorm:"column:inventory_id"`
// 		Total       int  `gorm:"column:total"`
// 		AllRows     int  `gorm:"column:all_rows"`
// 	}

// 	var stats []serialStat

// 	if err := c.DB.
// 		Model(&models.InventorySerial{}).
// 		Select(`
// 			inventory_id,
// 			SUM(
// 				CASE
// 					WHEN qty_available > 0 THEN 1
// 					ELSE 0
// 				END
// 			) AS total,
// 			COUNT(*) AS all_rows
// 		`).
// 		Where("inventory_id IN ?", ids).
// 		Where("deleted_at IS NULL").
// 		Group("inventory_id").
// 		Scan(&stats).Error; err != nil {

// 		return ctx.Status(
// 			fiber.StatusInternalServerError,
// 		).JSON(fiber.Map{
// 			"success": false,
// 			"error": "Failed to determine inventory mode: " +
// 				err.Error(),
// 		})
// 	}

// 	// =========================
// 	// BUILD SERIAL STAT MAP
// 	// =========================

// 	statMap := make(map[uint]serialStat, len(stats))

// 	for _, stat := range stats {
// 		statMap[stat.InventoryID] = stat
// 	}

// 	// =========================
// 	// BUILD RESULT
// 	// =========================

// 	result := make(
// 		[]ChangeItemInventory,
// 		0,
// 		len(inventories),
// 	)

// 	for _, inventory := range inventories {

// 		stat := statMap[inventory.ID]

// 		mode := "quantity"

// 		// Jika inventory mempunyai
// 		// inventory_serial record,
// 		// maka mode = serial.
// 		if stat.AllRows > 0 {
// 			mode = "serial"
// 		}

// 		result = append(
// 			result,
// 			ChangeItemInventory{
// 				Inventory:        inventory,
// 				AvailableSerials: stat.Total,
// 				ChangeMode:       mode,
// 			},
// 		)
// 	}

// 	// =========================
// 	// RESPONSE
// 	// =========================

// 	return ctx.JSON(fiber.Map{
// 		"success": true,
// 		"data": fiber.Map{
// 			"inventories": result,
// 			"total":       len(result),
// 		},
// 	})
// }

// ============================================================================
// GET SERIALS
// ============================================================================

// GET /inventory/change-item/serials?inventory_id=123
func (c *InventoryController) GetChangeItemSerials(ctx *fiber.Ctx) error {
	inventoryID := ctx.QueryInt("inventory_id", 0)

	if inventoryID <= 0 {
		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "inventory_id is required",
		})
	}

	var inventory models.Inventory

	if err := c.DB.
		Preload("Product").
		Where(
			"id = ? AND deleted_at IS NULL",
			inventoryID,
		).
		First(&inventory).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(
				fiber.StatusNotFound,
			).JSON(fiber.Map{
				"success": false,
				"error":   "Inventory not found",
			})
		}

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch inventory: " + err.Error(),
		})
	}

	var serials []models.InventorySerial

	if err := c.DB.
		Where(
			"inventory_id = ?",
			inventoryID,
		).
		Where("deleted_at IS NULL").
		Where("qty_available > 0").
		Order("serial_number ASC").
		Find(&serials).Error; err != nil {

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch serials: " + err.Error(),
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

// ============================================================================
// POST CHANGE ITEM
// ============================================================================

// POST /inventory/change-item
func (c *InventoryController) ChangeItem(ctx *fiber.Ctx) error {
	var input ChangeItemInput

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	input.TargetItemCode = strings.TrimSpace(
		input.TargetItemCode,
	)

	input.Reason = strings.TrimSpace(input.Reason)

	input.SerialNumbers = normalizeChangeItemSerials(
		input.SerialNumbers,
	)

	if input.InventoryID == 0 {
		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "inventory_id is required",
		})
	}

	if input.TargetItemID == 0 {
		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "target_item_id is required",
		})
	}

	userID, ok := changeItemUserID(ctx)

	if !ok {
		return ctx.Status(
			fiber.StatusUnauthorized,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid user ID",
		})
	}

	tx := c.DB.Begin()

	if tx.Error != nil {
		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
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

	// ------------------------------------------------------------------------
	// SOURCE
	// ------------------------------------------------------------------------

	var source models.Inventory

	if err := tx.
		Preload("Product").
		Where(
			"id = ? AND deleted_at IS NULL",
			input.InventoryID,
		).
		First(&source).Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(
				fiber.StatusNotFound,
			).JSON(fiber.Map{
				"success": false,
				"error":   "Source inventory not found",
			})
		}

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch source inventory: " + err.Error(),
		})
	}

	if source.QtyAvailable <= 0 {
		tx.Rollback()

		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Source inventory has no available quantity",
		})
	}

	// ------------------------------------------------------------------------
	// TARGET PRODUCT
	// ------------------------------------------------------------------------

	var targetProduct models.Product

	targetQuery := tx.
		Where("id = ?", input.TargetItemID).
		Where("deleted_at IS NULL")

	if input.TargetItemCode != "" {
		targetQuery = targetQuery.Where(
			"item_code = ?",
			input.TargetItemCode,
		)
	}

	if err := targetQuery.
		First(&targetProduct).Error; err != nil {

		tx.Rollback()

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(
				fiber.StatusNotFound,
			).JSON(fiber.Map{
				"success": false,
				"error":   "Target item not found",
			})
		}

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch target item: " + err.Error(),
		})
	}

	if targetProduct.ID == source.ItemId ||
		targetProduct.ItemCode == source.ItemCode {

		tx.Rollback()

		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Target item must be different from source item",
		})
	}

	// ------------------------------------------------------------------------
	// DETERMINE MODE FROM InventorySerial
	// ------------------------------------------------------------------------

	var activeSerialCount int64

	if err := tx.
		Model(&models.InventorySerial{}).
		Where(
			"inventory_id = ? AND deleted_at IS NULL",
			source.ID,
		).
		Count(&activeSerialCount).Error; err != nil {

		tx.Rollback()

		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to determine inventory mode: " + err.Error(),
		})
	}

	isSerial := activeSerialCount > 0

	// ------------------------------------------------------------------------
	// SERIAL
	// ------------------------------------------------------------------------

	if isSerial {
		if len(input.SerialNumbers) == 0 {
			tx.Rollback()

			return ctx.Status(
				fiber.StatusBadRequest,
			).JSON(fiber.Map{
				"success": false,
				"error":   "This inventory contains serial numbers. Select at least one serial number",
			})
		}

		if input.QtyToChange > 0 &&
			input.QtyToChange != float64(len(input.SerialNumbers)) {

			tx.Rollback()

			return ctx.Status(
				fiber.StatusBadRequest,
			).JSON(fiber.Map{
				"success": false,
				"error":   "qty_to_change must equal the selected serial count",
			})
		}

		result, err := changeItemSerial(
			tx,
			source,
			targetProduct,
			input,
			userID,
		)

		if err != nil {
			tx.Rollback()
			return changeItemError(ctx, err)
		}

		return ctx.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf(
				"Successfully changed %d serial(s)",
				len(input.SerialNumbers),
			),
			"data": result,
		})
	}

	// ------------------------------------------------------------------------
	// NON SERIAL
	// ------------------------------------------------------------------------

	if len(input.SerialNumbers) > 0 {
		tx.Rollback()

		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "This inventory has no InventorySerial rows. Change item by quantity instead",
		})
	}

	if input.QtyToChange <= 0 {
		tx.Rollback()

		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error":   "qty_to_change must be greater than 0",
		})
	}

	if input.QtyToChange > source.QtyAvailable {
		tx.Rollback()

		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(fiber.Map{
			"success": false,
			"error": fmt.Sprintf(
				"Change quantity %.4f exceeds available quantity %.4f",
				input.QtyToChange,
				source.QtyAvailable,
			),
		})
	}

	result, err := changeItemQuantity(
		tx,
		source,
		targetProduct,
		input,
		userID,
	)

	if err != nil {
		tx.Rollback()
		return changeItemError(ctx, err)
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf(
			"Successfully changed %.4f unit(s)",
			input.QtyToChange,
		),
		"data": result,
	})
}

// ============================================================================
// SERIAL CHANGE
// ============================================================================

func changeItemSerial(
	tx *gorm.DB,
	source models.Inventory,
	targetProduct models.Product,
	input ChangeItemInput,
	userID int,
) (map[string]interface{}, error) {

	qty := float64(len(input.SerialNumbers))

	var rows []models.InventorySerial

	if err := tx.
		Where("inventory_id = ?", source.ID).
		Where("serial_number IN ?", input.SerialNumbers).
		Where("deleted_at IS NULL").
		Find(&rows).Error; err != nil {

		return nil, fmt.Errorf(
			"failed to fetch source serials: %w",
			err,
		)
	}

	rowMap := make(
		map[string]models.InventorySerial,
		len(rows),
	)

	for _, row := range rows {
		sn := strings.TrimSpace(row.SerialNumber)

		if _, exists := rowMap[sn]; exists {
			return nil, fmt.Errorf(
				"duplicate serial in source inventory: %s",
				sn,
			)
		}

		rowMap[sn] = row
	}

	for _, serial := range input.SerialNumbers {
		row, exists := rowMap[serial]

		if !exists {
			return nil, fmt.Errorf(
				"serial %s is not available in source inventory",
				serial,
			)
		}

		if row.QtyAvailable != 1 {
			return nil, fmt.Errorf(
				"serial %s must have QtyAvailable = 1",
				serial,
			)
		}

		if row.QtyAllocated > 0 {
			return nil, fmt.Errorf(
				"serial %s is allocated",
				serial,
			)
		}

		if row.QtyShipped > 0 {
			return nil, fmt.Errorf(
				"serial %s is already shipped",
				serial,
			)
		}
	}

	// ------------------------------------------------------------------------
	// Find / create destination
	// ------------------------------------------------------------------------

	destination, isNew, err := findChangeItemDestination(
		tx,
		source,
		targetProduct,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to find destination inventory: %w",
			err,
		)
	}

	// ------------------------------------------------------------------------
	// Prevent duplicate serials
	// ------------------------------------------------------------------------

	if !isNew {
		var duplicateCount int64

		if err := tx.
			Model(&models.InventorySerial{}).
			Where(
				"inventory_id = ? AND serial_number IN ? AND deleted_at IS NULL",
				destination.ID,
				input.SerialNumbers,
			).
			Count(&duplicateCount).Error; err != nil {

			return nil, fmt.Errorf(
				"failed to validate destination serials: %w",
				err,
			)
		}

		if duplicateCount > 0 {
			return nil, errors.New(
				"one or more selected serials already exist at destination inventory",
			)
		}
	}

	// ------------------------------------------------------------------------
	// Source quantities
	// ------------------------------------------------------------------------

	sourceOnhandBefore := source.QtyOnhand
	sourceAvailableBefore := source.QtyAvailable

	sourceOnhandAfter := sourceOnhandBefore - qty
	sourceAvailableAfter := sourceAvailableBefore - qty

	if sourceOnhandAfter < 0 ||
		sourceAvailableAfter < 0 {

		return nil, errors.New(
			"source inventory quantity would become negative",
		)
	}

	if err := tx.
		Model(&source).
		Updates(map[string]interface{}{
			"qty_origin": gorm.Expr(
				"qty_origin - ?",
				qty,
			),
			"qty_onhand":    sourceOnhandAfter,
			"qty_available": sourceAvailableAfter,
			"updated_by":    userID,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {

		return nil, fmt.Errorf(
			"failed to update source inventory: %w",
			err,
		)
	}

	// ------------------------------------------------------------------------
	// Destination
	// ------------------------------------------------------------------------

	destinationID,
		destOnhandBefore,
		destAvailableBefore,
		destOnhandAfter,
		destAvailableAfter,
		err := upsertChangeItemDestination(
		tx,
		source,
		targetProduct,
		destination,
		isNew,
		qty,
		userID,
	)

	if err != nil {
		return nil, err
	}

	// ------------------------------------------------------------------------
	// Move serial rows
	// ------------------------------------------------------------------------

	for _, serial := range input.SerialNumbers {
		row := rowMap[serial]

		if err := tx.
			Model(&models.InventorySerial{}).
			Where(
				"id = ? AND inventory_id = ? AND deleted_at IS NULL",
				row.ID,
				source.ID,
			).
			Updates(map[string]interface{}{
				"inventory_id": destinationID,
				"updated_by":   userID,
				"updated_at":   time.Now().UTC(),
			}).Error; err != nil {

			return nil, fmt.Errorf(
				"failed to move serial %s: %w",
				serial,
				err,
			)
		}
	}

	// ------------------------------------------------------------------------
	// Movement
	// ------------------------------------------------------------------------

	movementID := uuid.NewString()

	if err := createChangeItemMovements(
		tx,
		movementID,
		source,
		targetProduct,
		destinationID,
		input,
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
		return nil, fmt.Errorf(
			"failed to commit change item: %w",
			err,
		)
	}

	return map[string]interface{}{
		"movement_id":              movementID,
		"source_inventory_id":      source.ID,
		"destination_inventory_id": destinationID,
		"source_item_code":         source.ItemCode,
		"target_item_code":         targetProduct.ItemCode,
		"serial_numbers":           input.SerialNumbers,
		"quantity":                 qty,
	}, nil
}

// ============================================================================
// QUANTITY CHANGE
// ============================================================================

func changeItemQuantity(
	tx *gorm.DB,
	source models.Inventory,
	targetProduct models.Product,
	input ChangeItemInput,
	userID int,
) (map[string]interface{}, error) {

	qty := input.QtyToChange

	destination, isNew, err := findChangeItemDestination(
		tx,
		source,
		targetProduct,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to find destination inventory: %w",
			err,
		)
	}

	sourceOnhandBefore := source.QtyOnhand
	sourceAvailableBefore := source.QtyAvailable

	sourceOnhandAfter := sourceOnhandBefore - qty
	sourceAvailableAfter := sourceAvailableBefore - qty

	if sourceOnhandAfter < 0 ||
		sourceAvailableAfter < 0 {

		return nil, errors.New(
			"source inventory quantity would become negative",
		)
	}

	if err := tx.
		Model(&source).
		Updates(map[string]interface{}{
			"qty_origin": gorm.Expr(
				"qty_origin - ?",
				qty,
			),
			"qty_onhand":    sourceOnhandAfter,
			"qty_available": sourceAvailableAfter,
			"updated_by":    userID,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {

		return nil, fmt.Errorf(
			"failed to update source inventory: %w",
			err,
		)
	}

	destinationID,
		destOnhandBefore,
		destAvailableBefore,
		destOnhandAfter,
		destAvailableAfter,
		err := upsertChangeItemDestination(
		tx,
		source,
		targetProduct,
		destination,
		isNew,
		qty,
		userID,
	)

	if err != nil {
		return nil, err
	}

	movementID := uuid.NewString()

	if err := createChangeItemMovements(
		tx,
		movementID,
		source,
		targetProduct,
		destinationID,
		input,
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
		return nil, fmt.Errorf(
			"failed to commit change item: %w",
			err,
		)
	}

	return map[string]interface{}{
		"movement_id":              movementID,
		"source_inventory_id":      source.ID,
		"destination_inventory_id": destinationID,
		"source_item_code":         source.ItemCode,
		"target_item_code":         targetProduct.ItemCode,
		"quantity":                 qty,
	}, nil
}

// ============================================================================
// FIND DESTINATION
// ============================================================================
//
// Physical attributes remain the same.
// Only ItemId / ItemCode / Barcode / UOM change.
//

func findChangeItemDestination(
	tx *gorm.DB,
	source models.Inventory,
	targetProduct models.Product,
) (models.Inventory, bool, error) {

	var destination models.Inventory

	query := tx.
		Where("whs_code = ?", source.WhsCode).
		Where("location = ?", source.Location).
		Where("item_id = ?", targetProduct.ID).
		Where("item_code = ?", targetProduct.ItemCode).
		Where("owner_code = ?", source.OwnerCode).
		Where("qa_status = ?", source.QaStatus).
		Where("division_code = ?", source.DivisionCode).
		Where("deleted_at IS NULL")

	// Target product determines barcode/UOM.
	query = query.
		Where(
			"barcode = ?",
			targetProduct.Barcode,
		).
		Where(
			"uom = ?",
			targetProduct.Uom,
		)

	// Keep the same physical inventory attributes.
	query = query.
		Where(
			"COALESCE(lot_number, '') = ?",
			source.LotNumber,
		).
		Where(
			"COALESCE(case_number, '') = ?",
			source.CaseNumber,
		).
		Where(
			"COALESCE(carton_number, '') = ?",
			source.CartonNumber,
		).
		Where(
			"COALESCE(pallet, '') = ?",
			source.Pallet,
		)

	if source.RecDate != "" {
		query = query.Where(
			"rec_date = ?",
			source.RecDate,
		)
	}

	if source.ProdDate != "" {
		query = query.Where(
			"prod_date = ?",
			source.ProdDate,
		)
	}

	if source.ExpDate != "" {
		query = query.Where(
			"exp_date = ?",
			source.ExpDate,
		)
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

// ============================================================================
// UPSERT DESTINATION
// ============================================================================

func upsertChangeItemDestination(
	tx *gorm.DB,
	source models.Inventory,
	targetProduct models.Product,
	destination models.Inventory,
	isNew bool,
	qty float64,
	userID int,
) (
	uint,
	float64,
	float64,
	float64,
	float64,
	error,
) {

	if isNew {

		newInventory := models.Inventory{
			OwnerCode:       source.OwnerCode,
			WhsCode:         source.WhsCode,
			DivisionCode:    source.DivisionCode,
			InboundID:       source.InboundID,
			InboundDetailId: source.InboundDetailId,
			RecDate:         source.RecDate,
			ProdDate:        source.ProdDate,
			ExpDate:         source.ExpDate,
			LotNumber:       source.LotNumber,
			CaseNumber:      source.CaseNumber,
			CartonNumber:    source.CartonNumber,
			Pallet:          source.Pallet,
			Location:        source.Location,
			ItemId:          targetProduct.ID,
			ItemCode:        targetProduct.ItemCode,
			Barcode:         targetProduct.Barcode,
			SerialNumber:    "",
			QaStatus:        source.QaStatus,
			Uom:             targetProduct.Uom,
			QtyOrigin:       qty,
			QtyOnhand:       qty,
			QtyAvailable:    qty,
			QtyAllocated:    0,
			QtySuspend:      0,
			QtyShipped:      0,
			Trans:           "CHANGE_ITEM",
			IsTransfer:      true,
			TransferFrom:    source.ID,
			CreatedBy:       userID,
			UpdatedBy:       userID,
		}

		if err := tx.
			Create(&newInventory).Error; err != nil {

			return 0, 0, 0, 0, 0, fmt.Errorf(
				"failed to create destination inventory: %w",
				err,
			)
		}

		return newInventory.ID,
			0,
			0,
			qty,
			qty,
			nil
	}

	destOnhandBefore := destination.QtyOnhand
	destAvailableBefore := destination.QtyAvailable

	destOnhandAfter := destOnhandBefore + qty
	destAvailableAfter := destAvailableBefore + qty

	if err := tx.
		Model(&destination).
		Updates(map[string]interface{}{
			"qty_origin": gorm.Expr(
				"qty_origin + ?",
				qty,
			),
			"qty_onhand":    destOnhandAfter,
			"qty_available": destAvailableAfter,
			"updated_by":    userID,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {

		return 0, 0, 0, 0, 0, fmt.Errorf(
			"failed to update destination inventory: %w",
			err,
		)
	}

	return destination.ID,
		destOnhandBefore,
		destAvailableBefore,
		destOnhandAfter,
		destAvailableAfter,
		nil
}

// ============================================================================
// MOVEMENT
// ============================================================================

func createChangeItemMovements(
	tx *gorm.DB,
	movementID string,
	source models.Inventory,
	targetProduct models.Product,
	destinationID uint,
	input ChangeItemInput,

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

	// ------------------------------------------------------------------------
	// SOURCE
	// ------------------------------------------------------------------------

	sourceMovement := models.InventoryMovement{
		MovementID: movementID,

		InventoryID: source.ID,

		RefType: "CHANGE_ITEM",

		RefID: destinationID,

		ItemID: source.ItemId,

		ItemCode: source.ItemCode,

		QtyOnhandChange: sourceChange,

		QtyAvailableChange: sourceChange,

		QtyAllocatedChange: 0,

		QtySuspendChange: 0,

		QtyShippedChange: 0,

		QtyOnhandBefore: &sourceOnhandBefore,

		QtyOnhandAfter: &sourceOnhandAfter,

		QtyAvailableBefore: &sourceAvailableBefore,

		QtyAvailableAfter: &sourceAvailableAfter,

		OwnerCode: source.OwnerCode,

		FromWhsCode: source.WhsCode,

		ToWhsCode: source.WhsCode,

		FromLocation: source.Location,

		ToLocation: source.Location,

		OldQaStatus: source.QaStatus,

		NewQaStatus: source.QaStatus,

		FromDivision: source.DivisionCode,

		ToDivision: source.DivisionCode,

		FromPallet: source.Pallet,

		ToPallet: source.Pallet,

		FromLotNumber: source.LotNumber,

		ToLotNumber: source.LotNumber,

		Reason: input.Reason,

		CreatedBy: userID,

		CreatedAt: time.Now(),
	}

	if err := tx.
		Create(&sourceMovement).Error; err != nil {

		return fmt.Errorf(
			"failed to create source movement: %w",
			err,
		)
	}

	// ------------------------------------------------------------------------
	// DESTINATION
	// ------------------------------------------------------------------------

	destinationMovement := models.InventoryMovement{
		MovementID: movementID,

		InventoryID: destinationID,

		RefType: "CHANGE_ITEM",

		RefID: source.ID,

		ItemID: targetProduct.ID,

		ItemCode: targetProduct.ItemCode,

		QtyOnhandChange: destinationChange,

		QtyAvailableChange: destinationChange,

		QtyAllocatedChange: 0,

		QtySuspendChange: 0,

		QtyShippedChange: 0,

		QtyOnhandBefore: &destOnhandBefore,

		QtyOnhandAfter: &destOnhandAfter,

		QtyAvailableBefore: &destAvailableBefore,

		QtyAvailableAfter: &destAvailableAfter,

		OwnerCode: source.OwnerCode,

		FromWhsCode: source.WhsCode,

		ToWhsCode: source.WhsCode,

		FromLocation: source.Location,

		ToLocation: source.Location,

		OldQaStatus: source.QaStatus,

		NewQaStatus: source.QaStatus,

		FromDivision: source.DivisionCode,

		ToDivision: source.DivisionCode,

		FromPallet: source.Pallet,

		ToPallet: source.Pallet,

		FromLotNumber: source.LotNumber,

		ToLotNumber: source.LotNumber,

		Reason: input.Reason,

		CreatedBy: userID,

		CreatedAt: time.Now(),
	}

	if err := tx.
		Create(&destinationMovement).Error; err != nil {

		return fmt.Errorf(
			"failed to create destination movement: %w",
			err,
		)
	}

	return nil
}

// ============================================================================
// HELPERS
// ============================================================================

func normalizeChangeItemSerials(
	values []string,
) []string {

	seen := make(
		map[string]struct{},
		len(values),
	)

	result := make(
		[]string,
		0,
		len(values),
	)

	for _, value := range values {
		value = strings.TrimSpace(value)

		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}

		result = append(
			result,
			value,
		)
	}

	return result
}

func changeItemUserID(
	ctx *fiber.Ctx,
) (int, bool) {

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

func changeItemError(
	ctx *fiber.Ctx,
	err error,
) error {

	status := fiber.StatusInternalServerError

	message := err.Error()

	lower := strings.ToLower(message)

	if strings.Contains(lower, "not found") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "available") ||
		strings.Contains(lower, "quantity") ||
		strings.Contains(lower, "qty") ||
		strings.Contains(lower, "serial") ||
		strings.Contains(lower, "allocated") ||
		strings.Contains(lower, "shipped") ||
		strings.Contains(lower, "negative") ||
		strings.Contains(lower, "different") {

		status = fiber.StatusBadRequest
	}

	if strings.Contains(lower, "already exist") ||
		strings.Contains(lower, "duplicate") {

		status = fiber.StatusConflict
	}

	return ctx.
		Status(status).
		JSON(fiber.Map{
			"success": false,
			"error":   message,
		})
}

// Prevent accidental removal if repository package is still required
// elsewhere in this controller file.
var _ = repositories.NewInventoryRepository
