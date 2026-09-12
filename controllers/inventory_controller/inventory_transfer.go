package inventory_controller

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===================================================================
// GET ALL INVENTORY (raw, for other purposes)
// ===================================================================

func (c *InventoryController) GetAllInventoryAvailable(ctx *fiber.Ctx) error {
	var inventories []models.Inventory

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

// ===================================================================
// GET GROUPED INVENTORY (for transfer UI)
// ===================================================================

type GroupedInventory struct {
	ItemCode     string             `json:"item_code"`
	ItemName     string             `json:"item_name"`
	WhsCode      string             `json:"whs_code"`
	Location     string             `json:"location"`
	DivisionCode string             `json:"division_code"`
	QaStatus     string             `json:"qa_status"`
	OwnerCode    string             `json:"owner_code"`
	RecDate      string             `json:"rec_date"`
	ProdDate     string             `json:"prod_date"`
	ExpDate      string             `json:"exp_date"`
	LotNumber    string             `json:"lot_number"`
	Pallet       string             `json:"pallet"`
	CartonNumber string             `json:"carton_number"`
	Uom          string             `json:"uom"`
	QtyAvailable float64            `json:"qty_available"`
	Records      []models.Inventory `json:"records"`
}

func (c *InventoryController) GetGroupedInventory(ctx *fiber.Ctx) error {
	var inventories []models.Inventory

	if err := c.DB.
		Preload("Product").
		Where("qty_available > ?", 0).
		Order("item_code ASC, whs_code ASC, location ASC, id ASC").
		Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch inventories",
		})
	}

	groupMap := make(map[string]*GroupedInventory)
	groupOrder := []string{}

	for _, inv := range inventories {
		key := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
			inv.ItemCode,
			inv.WhsCode,
			inv.Location,
			inv.DivisionCode,
			inv.QaStatus,
			inv.OwnerCode,
			inv.RecDate,
			inv.ProdDate,
			inv.ExpDate,
			inv.LotNumber,
			inv.Pallet,
			inv.CartonNumber,
		)

		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &GroupedInventory{
				ItemCode:     inv.ItemCode,
				ItemName:     inv.Product.ItemName,
				WhsCode:      inv.WhsCode,
				Location:     inv.Location,
				DivisionCode: inv.DivisionCode,
				QaStatus:     inv.QaStatus,
				OwnerCode:    inv.OwnerCode,
				RecDate:      inv.RecDate,
				ProdDate:     inv.ProdDate,
				ExpDate:      inv.ExpDate,
				LotNumber:    inv.LotNumber,
				Pallet:       inv.Pallet,
				CartonNumber: inv.CartonNumber,
				Uom:          inv.Uom,
				QtyAvailable: 0,
				Records:      []models.Inventory{},
			}
			groupOrder = append(groupOrder, key)
		}

		groupMap[key].QtyAvailable += inv.QtyAvailable
		groupMap[key].Records = append(groupMap[key].Records, inv)
	}

	result := make([]*GroupedInventory, 0, len(groupOrder))
	for _, key := range groupOrder {
		result = append(result, groupMap[key])
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"inventories": result,
			"total":       len(result),
		},
	})
}

// ptrFloat64 returns a pointer to a float64 value — used for the nullable
// QtyOnhandBefore/After and QtyAvailableBefore/After snapshot fields on
// InventoryMovement.
func ptrFloat64(v float64) *float64 {
	return &v
}

type TransferInventoryInput struct {
	ItemCode string `json:"item_code" validate:"required"`
	// OwnerCode is fixed source→destination (no cross-owner transfer allowed),
	// so a single field suffices — unlike DivisionCode which has separate
	// From/To variants because division CAN change during a transfer.
	OwnerCode        string  `json:"owner_code" validate:"required"`
	FromWhsCode      string  `json:"from_whs_code" validate:"required"`
	ToWhsCode        string  `json:"to_whs_code" validate:"required"`
	FromLocation     string  `json:"from_location" validate:"required"`
	ToLocation       string  `json:"to_location" validate:"required"`
	OldQaStatus      string  `json:"old_qa_status"`
	NewQaStatus      string  `json:"new_qa_status"`
	RecDate          string  `json:"rec_date"`
	ProdDate         string  `json:"prod_date"`
	ExpDate          string  `json:"exp_date"`
	FromLotNumber    string  `json:"from_lot_number"`
	LotNumber        string  `json:"lot_number"`
	Pallet           string  `json:"pallet"`
	QtyToTransfer    float64 `json:"qty_to_transfer" validate:"required,gt=0"`
	Reason           string  `json:"reason"`
	FromDivisionCode string  `json:"from_division_code" validate:"required"`
	DivisionCode     string  `json:"division_code" validate:"required"`
	CartonNumber     string  `json:"carton_number"`
}

func (c *InventoryController) TransferInventory(ctx *fiber.Ctx) error {
	var input TransferInventoryInput
	movementID := uuid.NewString()

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate input
	if input.ItemCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Item code is required",
		})
	}
	// ★ NEW
	if input.OwnerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Owner code is required",
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

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	inventoryRepo := repositories.NewInventoryRepository(tx)

	// ★ CHANGED: owner_code added to the WHERE clause, same COALESCE-free
	// pattern as division_code/qa_status (owner_code is never null in
	// practice, unlike lot_number/carton_number). Without this, two
	// different owners sharing the same item/whs/location/division/qa_status
	// combination could get their stock swept together.
	var sourceRecords []models.Inventory
	if err := tx.Debug().
		Where("item_code = ? AND owner_code = ? AND whs_code = ? AND location = ? AND division_code = ? AND qa_status = ? AND COALESCE(lot_number, '') = COALESCE(?, '') AND COALESCE(carton_number, '') = COALESCE(?, '') AND qty_available > 0",
			input.ItemCode,
			input.OwnerCode,
			input.FromWhsCode,
			input.FromLocation,
			input.FromDivisionCode,
			input.OldQaStatus,
			input.FromLotNumber,
			input.CartonNumber,
		).
		Order("id ASC").
		Find(&sourceRecords).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch source inventories",
		})
	}

	if len(sourceRecords) == 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "No source inventory found",
		})
	}

	// ★ NEW: defense-in-depth guard. Redundant with the WHERE clause above
	// under normal operation, but protects against future query changes
	// silently reintroducing cross-owner leakage.
	for _, r := range sourceRecords {
		if r.OwnerCode != input.OwnerCode {
			tx.Rollback()
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Owner code mismatch on source record %d. Cross-owner transfer is not allowed.", r.ID),
			})
		}
	}

	// Validate total qty
	var totalAvailable float64
	for _, r := range sourceRecords {
		totalAvailable += r.QtyAvailable
	}
	if totalAvailable < input.QtyToTransfer {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Insufficient quantity. Available: %.2f, Requested: %.2f", totalAvailable, input.QtyToTransfer),
		})
	}

	newQaStatus := input.NewQaStatus
	if newQaStatus == "" {
		newQaStatus = input.OldQaStatus
	}

	remaining := input.QtyToTransfer

	for _, sourceInventory := range sourceRecords {
		if remaining <= 0 {
			break
		}

		take := sourceInventory.QtyAvailable
		if take > remaining {
			take = remaining
		}
		remaining -= take

		isSplit := sourceInventory.QtyAvailable > take

		fromLotNumber := sourceInventory.LotNumber
		toLotNumber := input.LotNumber
		if toLotNumber == "" {
			toLotNumber = fromLotNumber
		}

		qtyOnhandBeforeSrc := sourceInventory.QtyOnhand
		qtyAvailableBeforeSrc := sourceInventory.QtyAvailable
		qtyOnhandAfterSrc := qtyOnhandBeforeSrc - take
		qtyAvailableAfterSrc := qtyAvailableBeforeSrc - take

		if err := tx.Model(&sourceInventory).Updates(map[string]interface{}{
			"qty_origin":    sourceInventory.QtyOrigin - take,
			"qty_onhand":    qtyOnhandAfterSrc,
			"qty_available": qtyAvailableAfterSrc,
			"updated_by":    userID,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to update source inventory",
			})
		}

		// ★ CHANGED: owner_code added to destQuery — prevents merging into an
		// existing destination bucket that belongs to a different owner but
		// otherwise matches every other field.
		var destInventory models.Inventory
		destQuery := tx.Where(
			"whs_code = ? AND location = ? AND item_code = ? AND owner_code = ? AND barcode = ? AND COALESCE(carton_number, '') = COALESCE(?, '') AND qa_status = ? AND lot_number = ?",
			input.ToWhsCode,
			input.ToLocation,
			sourceInventory.ItemCode,
			input.OwnerCode,
			sourceInventory.Barcode,
			sourceInventory.CartonNumber,
			newQaStatus,
			toLotNumber,
		)
		if input.RecDate != "" {
			destQuery = destQuery.Where("rec_date = ?", input.RecDate)
		}
		if input.ProdDate != "" {
			destQuery = destQuery.Where("prod_date = ?", input.ProdDate)
		}
		if input.ExpDate != "" {
			destQuery = destQuery.Where("exp_date = ?", input.ExpDate)
		}
		if input.DivisionCode != "" {
			destQuery = destQuery.Where("division_code = ?", input.DivisionCode)
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

		var toPallet string
		if !isNewDestination {
			toPallet = destInventory.Pallet
		} else if isSplit {
			generatedPallet, err := inventoryRepo.GeneratePalletID()
			if err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   "Failed to generate new pallet ID",
				})
			}
			toPallet = generatedPallet
		} else {
			toPallet = sourceInventory.Pallet
		}
		fromPallet := sourceInventory.Pallet

		var destInventoryID uint
		var qtyOnhandBeforeDest, qtyAvailableBeforeDest float64
		var qtyOnhandAfterDest, qtyAvailableAfterDest float64

		if isNewDestination {
			qtyOnhandBeforeDest = 0
			qtyAvailableBeforeDest = 0
			qtyOnhandAfterDest = take
			qtyAvailableAfterDest = take

			newInventory := models.Inventory{
				// ★ CHANGED: was sourceInventory.OwnerCode (implicit). Now
				// explicit from input.OwnerCode — equivalent in value since
				// the guard above already enforces they match, but explicit
				// makes the invariant visible at the call site.
				OwnerCode:       input.OwnerCode,
				WhsCode:         input.ToWhsCode,
				InboundID:       sourceInventory.InboundID,
				InboundDetailId: sourceInventory.InboundDetailId,
				DivisionCode:    input.DivisionCode,
				RecDate:         input.RecDate,
				ProdDate:        input.ProdDate,
				ExpDate:         input.ExpDate,
				LotNumber:       toLotNumber,
				Pallet:          toPallet,
				Location:        input.ToLocation,
				ItemId:          sourceInventory.ItemId,
				ItemCode:        sourceInventory.ItemCode,
				Barcode:         sourceInventory.Barcode,
				CaseNumber:      sourceInventory.CaseNumber,
				CartonNumber:    sourceInventory.CartonNumber,
				SerialNumber:    sourceInventory.SerialNumber,
				QaStatus:        newQaStatus,
				Uom:             sourceInventory.Uom,
				QtyOrigin:       take,
				QtyOnhand:       take,
				QtyAvailable:    take,
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
		} else {
			qtyOnhandBeforeDest = destInventory.QtyOnhand
			qtyAvailableBeforeDest = destInventory.QtyAvailable
			qtyOnhandAfterDest = qtyOnhandBeforeDest + take
			qtyAvailableAfterDest = qtyAvailableBeforeDest + take
			destInventory.QtyOrigin += take
			destInventory.QtyOnhand = qtyOnhandAfterDest
			destInventory.QtyAvailable = qtyAvailableAfterDest
			destInventory.UpdatedBy = userID

			if err := tx.Save(&destInventory).Error; err != nil {
				tx.Rollback()
				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   "Failed to update destination inventory",
				})
			}
			destInventoryID = destInventory.ID
		}

		sourceMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        sourceInventory.ID,
			RefType:            "TRANSFER",
			RefID:              destInventoryID,
			ItemID:             sourceInventory.ItemId,
			ItemCode:           sourceInventory.ItemCode,
			QtyOnhandChange:    -take,
			QtyAvailableChange: -take,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			QtyOnhandBefore:    ptrFloat64(qtyOnhandBeforeSrc),
			QtyOnhandAfter:     ptrFloat64(qtyOnhandAfterSrc),
			QtyAvailableBefore: ptrFloat64(qtyAvailableBeforeSrc),
			QtyAvailableAfter:  ptrFloat64(qtyAvailableAfterSrc),
			// ★ NEW
			OwnerCode:     input.OwnerCode,
			FromWhsCode:   input.FromWhsCode,
			ToWhsCode:     input.ToWhsCode,
			FromLocation:  input.FromLocation,
			ToLocation:    input.ToLocation,
			OldQaStatus:   sourceInventory.QaStatus,
			NewQaStatus:   newQaStatus,
			FromDivision:  input.FromDivisionCode,
			ToDivision:    input.DivisionCode,
			FromPallet:    fromPallet,
			ToPallet:      toPallet,
			FromLotNumber: fromLotNumber,
			ToLotNumber:   toLotNumber,
			Reason:        input.Reason,
			CreatedBy:     userID,
			CreatedAt:     time.Now(),
		}

		if err := tx.Create(&sourceMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record source movement " + err.Error(),
			})
		}

		destMovement := models.InventoryMovement{
			MovementID:         movementID,
			InventoryID:        destInventoryID,
			RefType:            "TRANSFER",
			RefID:              sourceInventory.ID,
			ItemID:             sourceInventory.ItemId,
			ItemCode:           sourceInventory.ItemCode,
			QtyOnhandChange:    take,
			QtyAvailableChange: take,
			QtyAllocatedChange: 0,
			QtySuspendChange:   0,
			QtyShippedChange:   0,
			QtyOnhandBefore:    ptrFloat64(qtyOnhandBeforeDest),
			QtyOnhandAfter:     ptrFloat64(qtyOnhandAfterDest),
			QtyAvailableBefore: ptrFloat64(qtyAvailableBeforeDest),
			QtyAvailableAfter:  ptrFloat64(qtyAvailableAfterDest),
			// ★ NEW
			OwnerCode:     input.OwnerCode,
			FromWhsCode:   input.FromWhsCode,
			ToWhsCode:     input.ToWhsCode,
			FromLocation:  input.FromLocation,
			ToLocation:    input.ToLocation,
			FromDivision:  input.FromDivisionCode,
			ToDivision:    input.DivisionCode,
			OldQaStatus:   sourceInventory.QaStatus,
			NewQaStatus:   newQaStatus,
			FromPallet:    fromPallet,
			ToPallet:      toPallet,
			FromLotNumber: fromLotNumber,
			ToLotNumber:   toLotNumber,
			Reason:        input.Reason,
			CreatedBy:     userID,
			CreatedAt:     time.Now(),
		}
		if err := tx.Create(&destMovement).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to record destination movement",
			})
		}
	}

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
			"quantity_transferred": input.QtyToTransfer,
			"from":                 fmt.Sprintf("%s | %s", input.FromWhsCode, input.FromLocation),
			"to":                   fmt.Sprintf("%s | %s", input.ToWhsCode, input.ToLocation),
		},
	})
}
