// package inventory_controller

// import (
// 	"fiber-app/models"
// 	"fiber-app/repositories"
// 	"fmt"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/google/uuid"
// 	"gorm.io/gorm"
// )

// func (c *InventoryController) GetAllInventoryAvailable(ctx *fiber.Ctx) error {
// 	var inventories []models.Inventory

// 	// Query inventory dengan qty_available > 0
// 	if err := c.DB.
// 		Preload("Product").
// 		Where("qty_available > ?", 0).
// 		Order("item_code ASC, whs_code ASC, location ASC").
// 		Find(&inventories).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to fetch inventories",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"data": fiber.Map{
// 			"inventories": inventories,
// 			"total":       len(inventories),
// 		},
// 	})
// }

// //===================================================================
// // BEGIN INTERNAL TRANSFER
// // ==================================================================

// type TransferInventoryInput struct {
// 	InventoryID      uint    `json:"inventory_id" validate:"required"`
// 	FromWhsCode      string  `json:"from_whs_code" validate:"required"`
// 	ToWhsCode        string  `json:"to_whs_code" validate:"required"`
// 	FromLocation     string  `json:"from_location" validate:"required"`
// 	ToLocation       string  `json:"to_location" validate:"required"`
// 	OldQaStatus      string  `json:"old_qa_status"`
// 	NewQaStatus      string  `json:"new_qa_status"`
// 	RecDate          string  `json:"rec_date"`
// 	ProdDate         string  `json:"prod_date"`
// 	ExpDate          string  `json:"exp_date"`
// 	LotNumber        string  `json:"lot_number"`
// 	Pallet           string  `json:"pallet"`
// 	QtyToTransfer    float64 `json:"qty_to_transfer" validate:"required,gt=0"`
// 	Reason           string  `json:"reason"`
// 	FromDivisionCode string  `json:"from_division_code" validate:"required"`
// 	DivisionCode     string  `json:"division_code" validate:"required"`
// }

// func (c *InventoryController) TransferInventory(ctx *fiber.Ctx) error {
// 	var input TransferInventoryInput
// 	movementID := uuid.NewString()

// 	isSplit := false

// 	// Parse body
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Invalid request body",
// 		})
// 	}

// 	// Validate input
// 	if input.InventoryID == 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Inventory ID is required",
// 		})
// 	}

// 	if input.QtyToTransfer <= 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Quantity to transfer must be greater than 0",
// 		})
// 	}

// 	if input.FromWhsCode == "" || input.ToWhsCode == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To warehouse codes are required",
// 		})
// 	}

// 	if input.FromLocation == "" || input.ToLocation == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To locations are required",
// 		})
// 	}

// 	userID := int(ctx.Locals("userID").(float64))

// 	// Start transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	inventoryRepo := repositories.NewInventoryRepository(tx)

// 	// Get source inventory
// 	var sourceInventory models.Inventory
// 	if err := tx.Where("id = ?", input.InventoryID).First(&sourceInventory).Error; err != nil {
// 		tx.Rollback()
// 		if err == gorm.ErrRecordNotFound {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Source inventory not found",
// 			})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to fetch source inventory",
// 		})
// 	}

// 	// Validate warehouse and location match
// 	if sourceInventory.WhsCode != input.FromWhsCode {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   fmt.Sprintf("Source warehouse mismatch. Expected: %s, Got: %s", sourceInventory.WhsCode, input.FromWhsCode),
// 		})
// 	}

// 	if sourceInventory.Location != input.FromLocation {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   fmt.Sprintf("Source location mismatch. Expected: %s, Got: %s", sourceInventory.Location, input.FromLocation),
// 		})
// 	}

// 	// Validate sufficient quantity
// 	if sourceInventory.QtyAvailable < input.QtyToTransfer {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   fmt.Sprintf("Insufficient quantity. Available: %.2f, Requested: %.2f", sourceInventory.QtyAvailable, input.QtyToTransfer),
// 		})
// 	}

// 	// Use existing QA status if not provided
// 	newQaStatus := input.NewQaStatus
// 	if newQaStatus == "" {
// 		newQaStatus = sourceInventory.QaStatus
// 	}

// 	// Check if destination inventory exists with same attributes
// 	var destInventory models.Inventory
// 	destQuery := tx.Where("whs_code = ? AND location = ? AND item_code = ? AND barcode = ? AND qa_status = ? AND lot_number = ?",
// 		input.ToWhsCode,
// 		input.ToLocation,
// 		sourceInventory.ItemCode,
// 		sourceInventory.Barcode,
// 		newQaStatus,
// 		input.LotNumber,
// 	)

// 	// Add optional filters if provided
// 	if input.RecDate != "" {
// 		destQuery = destQuery.Where("rec_date = ?", input.RecDate)
// 	}
// 	if input.ProdDate != "" {
// 		destQuery = destQuery.Where("prod_date = ?", input.ProdDate)
// 	}
// 	if input.ExpDate != "" {
// 		destQuery = destQuery.Where("exp_date = ?", input.ExpDate)
// 	}
// 	if input.Pallet != "" {
// 		destQuery = destQuery.Where("pallet = ?", input.Pallet)
// 	}
// 	if input.DivisionCode != "" {
// 		destQuery = destQuery.Where("division_code = ?", input.DivisionCode)
// 	}

// 	err := destQuery.First(&destInventory).Error
// 	isNewDestination := err == gorm.ErrRecordNotFound

// 	if err != nil && err != gorm.ErrRecordNotFound {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to check destination inventory",
// 		})
// 	}

// 	if sourceInventory.QtyAvailable > input.QtyToTransfer {
// 		isSplit = true
// 	}

// 	// Update source inventory - deduct quantity
// 	sourceInventory.QtyOrigin -= input.QtyToTransfer
// 	sourceInventory.QtyOnhand -= input.QtyToTransfer
// 	sourceInventory.QtyAvailable -= input.QtyToTransfer
// 	sourceInventory.UpdatedBy = userID

// 	if err := tx.Save(&sourceInventory).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to update source inventory",
// 		})
// 	}

// 	// Record source inventory movement
// 	sourceMovement := models.InventoryMovement{
// 		MovementID:         movementID,
// 		InventoryID:        sourceInventory.ID,
// 		RefType:            "TRANSFER",
// 		RefID:              0,
// 		ItemID:             sourceInventory.ItemId,
// 		ItemCode:           sourceInventory.ItemCode,
// 		QtyOnhandChange:    -input.QtyToTransfer,
// 		QtyAvailableChange: -input.QtyToTransfer,
// 		QtyAllocatedChange: 0,
// 		QtySuspendChange:   0,
// 		QtyShippedChange:   0,
// 		FromWhsCode:        input.FromWhsCode,
// 		ToWhsCode:          input.ToWhsCode,
// 		FromLocation:       input.FromLocation,
// 		ToLocation:         input.ToLocation,
// 		OldQaStatus:        sourceInventory.QaStatus,
// 		NewQaStatus:        newQaStatus,
// 		FromDivision:       input.FromDivisionCode,
// 		ToDivision:         input.DivisionCode,
// 		Reason:             input.Reason,
// 		CreatedBy:          userID,
// 		CreatedAt:          time.Now(),
// 	}

// 	if err := tx.Create(&sourceMovement).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to record source movement",
// 		})
// 	}

// 	var destInventoryID uint

// 	if input.Pallet == sourceInventory.Pallet {
// 		input.Pallet = input.ToLocation
// 	}

// 	if isNewDestination {
// 		// Create new destination inventory
// 		newInventory := models.Inventory{
// 			OwnerCode:       sourceInventory.OwnerCode,
// 			WhsCode:         input.ToWhsCode,
// 			InboundID:       sourceInventory.InboundID,
// 			InboundDetailId: sourceInventory.InboundDetailId,
// 			DivisionCode:    input.DivisionCode,
// 			RecDate:         input.RecDate,
// 			ProdDate:        input.ProdDate,
// 			ExpDate:         input.ExpDate,
// 			LotNumber:       input.LotNumber,
// 			Pallet:          input.Pallet,
// 			Location:        input.ToLocation,
// 			ItemId:          sourceInventory.ItemId,
// 			ItemCode:        sourceInventory.ItemCode,
// 			Barcode:         sourceInventory.Barcode,
// 			QaStatus:        newQaStatus,
// 			Uom:             sourceInventory.Uom,
// 			QtyOrigin:       input.QtyToTransfer,
// 			QtyOnhand:       input.QtyToTransfer,
// 			QtyAvailable:    input.QtyToTransfer,
// 			QtyAllocated:    0,
// 			QtySuspend:      0,
// 			QtyShipped:      0,
// 			Trans:           "TRANSFER",
// 			IsTransfer:      true,
// 			TransferFrom:    sourceInventory.ID,
// 			CreatedBy:       userID,
// 			UpdatedBy:       userID,
// 		}

// 		fmt.Println("QTY BEFORE ", sourceInventory.QtyAvailable)
// 		fmt.Println("QTY TO TRANSFER ", input.QtyToTransfer)

// 		if isSplit {

// 			newPallet, err := inventoryRepo.GeneratePalletID()
// 			if err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to generate new pallet ID",
// 				})
// 			}

// 			newInventory.Pallet = newPallet
// 		} else {
// 			newInventory.Pallet = sourceInventory.Pallet
// 		}

// 		if err := tx.Create(&newInventory).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to create destination inventory",
// 			})
// 		}

// 		destInventoryID = newInventory.ID

// 		// Record destination inventory movement
// 		destMovement := models.InventoryMovement{
// 			MovementID:         movementID,
// 			InventoryID:        newInventory.ID,
// 			RefType:            "TRANSFER",
// 			RefID:              sourceInventory.ID,
// 			ItemID:             sourceInventory.ItemId,
// 			ItemCode:           sourceInventory.ItemCode,
// 			QtyOnhandChange:    input.QtyToTransfer,
// 			QtyAvailableChange: input.QtyToTransfer,
// 			QtyAllocatedChange: 0,
// 			QtySuspendChange:   0,
// 			QtyShippedChange:   0,
// 			FromWhsCode:        input.FromWhsCode,
// 			ToWhsCode:          input.ToWhsCode,
// 			FromLocation:       input.FromLocation,
// 			ToLocation:         input.ToLocation,
// 			FromDivision:       input.FromDivisionCode,
// 			ToDivision:         input.DivisionCode,
// 			OldQaStatus:        sourceInventory.QaStatus,
// 			NewQaStatus:        newQaStatus,
// 			Reason:             input.Reason,
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		}

// 		if err := tx.Create(&destMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to record destination movement",
// 			})
// 		}

// 	} else {
// 		// Update existing destination inventory
// 		destInventory.QtyOnhand += input.QtyToTransfer
// 		destInventory.QtyAvailable += input.QtyToTransfer
// 		destInventory.UpdatedBy = userID

// 		if err := tx.Save(&destInventory).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to update destination inventory",
// 			})
// 		}

// 		destInventoryID = destInventory.ID

// 		// Record destination inventory movement
// 		destMovement := models.InventoryMovement{
// 			MovementID:         movementID,
// 			InventoryID:        destInventory.ID,
// 			RefType:            "TRANSFER",
// 			RefID:              sourceInventory.ID,
// 			ItemID:             sourceInventory.ItemId,
// 			ItemCode:           sourceInventory.ItemCode,
// 			QtyOnhandChange:    input.QtyToTransfer,
// 			QtyAvailableChange: input.QtyToTransfer,
// 			QtyAllocatedChange: 0,
// 			QtySuspendChange:   0,
// 			QtyShippedChange:   0,
// 			FromWhsCode:        input.FromWhsCode,
// 			ToWhsCode:          input.ToWhsCode,
// 			FromLocation:       input.FromLocation,
// 			ToLocation:         input.ToLocation,
// 			OldQaStatus:        sourceInventory.QaStatus,
// 			ToDivision:         input.DivisionCode,
// 			FromDivision:       input.FromDivisionCode,
// 			NewQaStatus:        newQaStatus,
// 			Reason:             input.Reason,
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		}

// 		if err := tx.Create(&destMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to record destination movement",
// 			})
// 		}
// 	}

// 	// Update source movement with destination ref
// 	sourceMovement.RefID = destInventoryID
// 	if err := tx.Save(&sourceMovement).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to update source movement reference",
// 		})
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to commit transaction",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": fmt.Sprintf("Successfully transferred %.2f units from %s to %s", input.QtyToTransfer, input.FromLocation, input.ToLocation),
// 		"data": fiber.Map{
// 			"source_inventory_id":      sourceInventory.ID,
// 			"destination_inventory_id": destInventoryID,
// 			"quantity_transferred":     input.QtyToTransfer,
// 			"is_new_destination":       isNewDestination,
// 		},
// 	})
// }

// //===================================================================
// // END INTERNAL TRANSFER
// //===================================================================

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

// ===================================================================
// INTERNAL TRANSFER
// ===================================================================

// type TransferInventoryInput struct {
// 	ItemCode         string  `json:"item_code" validate:"required"`
// 	FromWhsCode      string  `json:"from_whs_code" validate:"required"`
// 	ToWhsCode        string  `json:"to_whs_code" validate:"required"`
// 	FromLocation     string  `json:"from_location" validate:"required"`
// 	ToLocation       string  `json:"to_location" validate:"required"`
// 	OldQaStatus      string  `json:"old_qa_status"`
// 	NewQaStatus      string  `json:"new_qa_status"`
// 	RecDate          string  `json:"rec_date"`
// 	ProdDate         string  `json:"prod_date"`
// 	ExpDate          string  `json:"exp_date"`
// 	LotNumber        string  `json:"lot_number"`
// 	Pallet           string  `json:"pallet"`
// 	QtyToTransfer    float64 `json:"qty_to_transfer" validate:"required,gt=0"`
// 	Reason           string  `json:"reason"`
// 	FromDivisionCode string  `json:"from_division_code" validate:"required"`
// 	DivisionCode     string  `json:"division_code" validate:"required"`
// 	CartonNumber     string  `json:"carton_number"`
// }

// func (c *InventoryController) TransferInventory(ctx *fiber.Ctx) error {
// 	var input TransferInventoryInput
// 	movementID := uuid.NewString()

// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Invalid request body",
// 		})
// 	}

// 	// Validate input
// 	if input.ItemCode == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Item code is required",
// 		})
// 	}
// 	if input.QtyToTransfer <= 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Quantity to transfer must be greater than 0",
// 		})
// 	}
// 	if input.FromWhsCode == "" || input.ToWhsCode == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To warehouse codes are required",
// 		})
// 	}
// 	if input.FromLocation == "" || input.ToLocation == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To locations are required",
// 		})
// 	}

// 	userID := int(ctx.Locals("userID").(float64))

// 	// Start transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	inventoryRepo := repositories.NewInventoryRepository(tx)

// 	// Fetch all source records FIFO (by id ASC)
// 	var sourceRecords []models.Inventory
// 	if err := tx.Debug().
// 		Where("item_code = ? AND whs_code = ? AND location = ? AND division_code = ? AND qa_status = ? AND COALESCE(carton_number, '') = COALESCE(?, '') AND qty_available > 0",
// 			input.ItemCode,
// 			input.FromWhsCode,
// 			input.FromLocation,
// 			input.FromDivisionCode,
// 			input.OldQaStatus,
// 			input.CartonNumber,
// 		).
// 		Order("id ASC").
// 		Find(&sourceRecords).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to fetch source inventories",
// 		})
// 	}

// 	if len(sourceRecords) == 0 {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "No source inventory found",
// 		})
// 	}

// 	// Validate total qty
// 	var totalAvailable float64
// 	for _, r := range sourceRecords {
// 		totalAvailable += r.QtyAvailable
// 	}
// 	if totalAvailable < input.QtyToTransfer {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   fmt.Sprintf("Insufficient quantity. Available: %.2f, Requested: %.2f", totalAvailable, input.QtyToTransfer),
// 		})
// 	}

// 	newQaStatus := input.NewQaStatus
// 	if newQaStatus == "" {
// 		newQaStatus = input.OldQaStatus
// 	}

// 	remaining := input.QtyToTransfer

// 	for _, sourceInventory := range sourceRecords {
// 		if remaining <= 0 {
// 			break
// 		}

// 		// How much to take from this record
// 		take := sourceInventory.QtyAvailable
// 		if take > remaining {
// 			take = remaining
// 		}
// 		remaining -= take

// 		isSplit := sourceInventory.QtyAvailable > take

// 		// Deduct source
// 		// sourceInventory.QtyOrigin -= take
// 		// sourceInventory.QtyOnhand -= take
// 		// sourceInventory.QtyAvailable -= take
// 		// sourceInventory.UpdatedBy = userID

// 		// if err := tx.Save(&sourceInventory).Error; err != nil {
// 		// 	tx.Rollback()
// 		// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 		// 		"success": false,
// 		// 		"error":   "Failed to update source inventory",
// 		// 	})
// 		// }

// 		if err := tx.Model(&sourceInventory).Updates(map[string]interface{}{
// 			"qty_origin":    sourceInventory.QtyOrigin - take,
// 			"qty_onhand":    sourceInventory.QtyOnhand - take,
// 			"qty_available": sourceInventory.QtyAvailable - take,
// 			"updated_by":    userID,
// 			"updated_at":    time.Now().UTC(),
// 		}).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to update source inventory",
// 			})
// 		}

// 		// Record source movement
// 		sourceMovement := models.InventoryMovement{
// 			MovementID:         movementID,
// 			InventoryID:        sourceInventory.ID,
// 			RefType:            "TRANSFER",
// 			RefID:              0,
// 			ItemID:             sourceInventory.ItemId,
// 			ItemCode:           sourceInventory.ItemCode,
// 			QtyOnhandChange:    -take,
// 			QtyAvailableChange: -take,
// 			QtyAllocatedChange: 0,
// 			QtySuspendChange:   0,
// 			QtyShippedChange:   0,
// 			FromWhsCode:        input.FromWhsCode,
// 			ToWhsCode:          input.ToWhsCode,
// 			FromLocation:       input.FromLocation,
// 			ToLocation:         input.ToLocation,
// 			OldQaStatus:        sourceInventory.QaStatus,
// 			NewQaStatus:        newQaStatus,
// 			FromDivision:       input.FromDivisionCode,
// 			ToDivision:         input.DivisionCode,
// 			Reason:             input.Reason,
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		}

// 		if err := tx.Create(&sourceMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to record source movement",
// 			})
// 		}

// 		// Check if destination inventory exists
// 		var destInventory models.Inventory
// 		destQuery := tx.Where(
// 			"whs_code = ? AND location = ? AND item_code = ? AND barcode = ? AND COALESCE(carton_number, '') = COALESCE(?, '') AND qa_status = ? AND lot_number = ?",
// 			input.ToWhsCode,
// 			input.ToLocation,
// 			sourceInventory.ItemCode,
// 			sourceInventory.Barcode,
// 			sourceInventory.CartonNumber,
// 			newQaStatus,
// 			input.LotNumber,
// 		)
// 		if input.RecDate != "" {
// 			destQuery = destQuery.Where("rec_date = ?", input.RecDate)
// 		}
// 		if input.ProdDate != "" {
// 			destQuery = destQuery.Where("prod_date = ?", input.ProdDate)
// 		}
// 		if input.ExpDate != "" {
// 			destQuery = destQuery.Where("exp_date = ?", input.ExpDate)
// 		}
// 		if input.DivisionCode != "" {
// 			destQuery = destQuery.Where("division_code = ?", input.DivisionCode)
// 		}

// 		err := destQuery.First(&destInventory).Error
// 		isNewDestination := err == gorm.ErrRecordNotFound

// 		if err != nil && err != gorm.ErrRecordNotFound {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to check destination inventory",
// 			})
// 		}

// 		var destInventoryID uint

// 		if isNewDestination {
// 			newPallet := sourceInventory.Pallet
// 			if isSplit {
// 				generatedPallet, err := inventoryRepo.GeneratePalletID()
// 				if err != nil {
// 					tx.Rollback()
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 						"success": false,
// 						"error":   "Failed to generate new pallet ID",
// 					})
// 				}
// 				newPallet = generatedPallet
// 			}

// 			newInventory := models.Inventory{
// 				OwnerCode:       sourceInventory.OwnerCode,
// 				WhsCode:         input.ToWhsCode,
// 				InboundID:       sourceInventory.InboundID,
// 				InboundDetailId: sourceInventory.InboundDetailId,
// 				DivisionCode:    input.DivisionCode,
// 				RecDate:         input.RecDate,
// 				ProdDate:        input.ProdDate,
// 				ExpDate:         input.ExpDate,
// 				LotNumber:       input.LotNumber,
// 				Pallet:          newPallet,
// 				Location:        input.ToLocation,
// 				ItemId:          sourceInventory.ItemId,
// 				ItemCode:        sourceInventory.ItemCode,
// 				Barcode:         sourceInventory.Barcode,
// 				CartonNumber:    sourceInventory.CartonNumber,
// 				QaStatus:        newQaStatus,
// 				Uom:             sourceInventory.Uom,
// 				QtyOrigin:       take,
// 				QtyOnhand:       take,
// 				QtyAvailable:    take,
// 				QtyAllocated:    0,
// 				QtySuspend:      0,
// 				QtyShipped:      0,
// 				Trans:           "TRANSFER",
// 				IsTransfer:      true,
// 				TransferFrom:    sourceInventory.ID,
// 				CreatedBy:       userID,
// 				UpdatedBy:       userID,
// 			}

// 			if err := tx.Create(&newInventory).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to create destination inventory",
// 				})
// 			}
// 			destInventoryID = newInventory.ID

// 			destMovement := models.InventoryMovement{
// 				MovementID:         movementID,
// 				InventoryID:        newInventory.ID,
// 				RefType:            "TRANSFER",
// 				RefID:              sourceInventory.ID,
// 				ItemID:             sourceInventory.ItemId,
// 				ItemCode:           sourceInventory.ItemCode,
// 				QtyOnhandChange:    take,
// 				QtyAvailableChange: take,
// 				QtyAllocatedChange: 0,
// 				QtySuspendChange:   0,
// 				QtyShippedChange:   0,
// 				FromWhsCode:        input.FromWhsCode,
// 				ToWhsCode:          input.ToWhsCode,
// 				FromLocation:       input.FromLocation,
// 				ToLocation:         input.ToLocation,
// 				FromDivision:       input.FromDivisionCode,
// 				ToDivision:         input.DivisionCode,
// 				OldQaStatus:        sourceInventory.QaStatus,
// 				NewQaStatus:        newQaStatus,
// 				Reason:             input.Reason,
// 				CreatedBy:          userID,
// 				CreatedAt:          time.Now(),
// 			}
// 			if err := tx.Create(&destMovement).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to record destination movement",
// 				})
// 			}

// 		} else {
// 			destInventory.QtyOrigin += take
// 			destInventory.QtyOnhand += take
// 			destInventory.QtyAvailable += take
// 			destInventory.UpdatedBy = userID

// 			if err := tx.Save(&destInventory).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to update destination inventory",
// 				})
// 			}
// 			destInventoryID = destInventory.ID

// 			destMovement := models.InventoryMovement{
// 				MovementID:         movementID,
// 				InventoryID:        destInventory.ID,
// 				RefType:            "TRANSFER",
// 				RefID:              sourceInventory.ID,
// 				ItemID:             sourceInventory.ItemId,
// 				ItemCode:           sourceInventory.ItemCode,
// 				QtyOnhandChange:    take,
// 				QtyAvailableChange: take,
// 				QtyAllocatedChange: 0,
// 				QtySuspendChange:   0,
// 				QtyShippedChange:   0,
// 				FromWhsCode:        input.FromWhsCode,
// 				ToWhsCode:          input.ToWhsCode,
// 				FromLocation:       input.FromLocation,
// 				ToLocation:         input.ToLocation,
// 				OldQaStatus:        sourceInventory.QaStatus,
// 				NewQaStatus:        newQaStatus,
// 				FromDivision:       input.FromDivisionCode,
// 				ToDivision:         input.DivisionCode,
// 				Reason:             input.Reason,
// 				CreatedBy:          userID,
// 				CreatedAt:          time.Now(),
// 			}
// 			if err := tx.Create(&destMovement).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to record destination movement",
// 				})
// 			}
// 		}

// 		// Update source movement ref
// 		sourceMovement.RefID = destInventoryID
// 		if err := tx.Save(&sourceMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to update source movement reference",
// 			})
// 		}
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to commit transaction",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": fmt.Sprintf("Successfully transferred %.2f units from %s to %s", input.QtyToTransfer, input.FromLocation, input.ToLocation),
// 		"data": fiber.Map{
// 			"quantity_transferred": input.QtyToTransfer,
// 			"from":                 fmt.Sprintf("%s | %s", input.FromWhsCode, input.FromLocation),
// 			"to":                   fmt.Sprintf("%s | %s", input.ToWhsCode, input.ToLocation),
// 		},
// 	})
// }

// ptrFloat64 returns a pointer to a float64 value — used for the nullable
// QtyOnhandBefore/After and QtyAvailableBefore/After snapshot fields on
// InventoryMovement.
func ptrFloat64(v float64) *float64 {
	return &v
}

// type TransferInventoryInput struct {
// 	ItemCode     string `json:"item_code" validate:"required"`
// 	FromWhsCode  string `json:"from_whs_code" validate:"required"`
// 	ToWhsCode    string `json:"to_whs_code" validate:"required"`
// 	FromLocation string `json:"from_location" validate:"required"`
// 	ToLocation   string `json:"to_location" validate:"required"`
// 	OldQaStatus  string `json:"old_qa_status"`
// 	NewQaStatus  string `json:"new_qa_status"`
// 	RecDate      string `json:"rec_date"`
// 	ProdDate     string `json:"prod_date"`
// 	ExpDate      string `json:"exp_date"`
// 	// FromLotNumber identifies the EXACT source lot the client selected in the
// 	// UI (GroupedInventory.lot_number in By Quantity, CartonGroup.lot_number in
// 	// By Carton). It's used as a WHERE filter when fetching FIFO source
// 	// records, so a transfer never silently pulls stock from a different lot
// 	// that happens to share the same item/whs/location/division/qa_status.
// 	FromLotNumber string `json:"from_lot_number"`
// 	// LotNumber is the DESTINATION/new lot number. Empty string = keep each
// 	// consumed source record's own lot number at the destination.
// 	LotNumber        string  `json:"lot_number"`
// 	Pallet           string  `json:"pallet"`
// 	QtyToTransfer    float64 `json:"qty_to_transfer" validate:"required,gt=0"`
// 	Reason           string  `json:"reason"`
// 	FromDivisionCode string  `json:"from_division_code" validate:"required"`
// 	DivisionCode     string  `json:"division_code" validate:"required"`
// 	CartonNumber     string  `json:"carton_number"`
// }

// func (c *InventoryController) TransferInventory(ctx *fiber.Ctx) error {
// 	var input TransferInventoryInput
// 	movementID := uuid.NewString()

// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Invalid request body",
// 		})
// 	}

// 	// Validate input
// 	if input.ItemCode == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Item code is required",
// 		})
// 	}
// 	if input.QtyToTransfer <= 0 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Quantity to transfer must be greater than 0",
// 		})
// 	}
// 	if input.FromWhsCode == "" || input.ToWhsCode == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To warehouse codes are required",
// 		})
// 	}
// 	if input.FromLocation == "" || input.ToLocation == "" {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "From and To locations are required",
// 		})
// 	}

// 	userID := int(ctx.Locals("userID").(float64))

// 	// Start transaction
// 	tx := c.DB.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	inventoryRepo := repositories.NewInventoryRepository(tx)

// 	// Fetch all source records FIFO (by id ASC).
// 	// ★ CHANGED: lot_number is now part of the WHERE clause (via
// 	// input.FromLotNumber), matched with the same COALESCE pattern already
// 	// used for carton_number. Without this, a transfer request that didn't
// 	// pin an exact carton could silently sweep stock from a different lot
// 	// that happens to share the same item/whs/location/division/qa_status —
// 	// this closes that gap.
// 	var sourceRecords []models.Inventory
// 	if err := tx.Debug().
// 		Where("item_code = ? AND whs_code = ? AND location = ? AND division_code = ? AND qa_status = ? AND COALESCE(lot_number, '') = COALESCE(?, '') AND COALESCE(carton_number, '') = COALESCE(?, '') AND qty_available > 0",
// 			input.ItemCode,
// 			input.FromWhsCode,
// 			input.FromLocation,
// 			input.FromDivisionCode,
// 			input.OldQaStatus,
// 			input.FromLotNumber,
// 			input.CartonNumber,
// 		).
// 		Order("id ASC").
// 		Find(&sourceRecords).Error; err != nil {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to fetch source inventories",
// 		})
// 	}

// 	if len(sourceRecords) == 0 {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "No source inventory found",
// 		})
// 	}

// 	// Validate total qty
// 	var totalAvailable float64
// 	for _, r := range sourceRecords {
// 		totalAvailable += r.QtyAvailable
// 	}
// 	if totalAvailable < input.QtyToTransfer {
// 		tx.Rollback()
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"success": false,
// 			"error":   fmt.Sprintf("Insufficient quantity. Available: %.2f, Requested: %.2f", totalAvailable, input.QtyToTransfer),
// 		})
// 	}

// 	newQaStatus := input.NewQaStatus
// 	if newQaStatus == "" {
// 		newQaStatus = input.OldQaStatus
// 	}

// 	// Effective destination lot number: use the requested new lot number,
// 	// falling back to "keep the source record's lot number" when blank.
// 	// Resolved per source record below (fromLotNumber differs per record
// 	// when FIFO consumes across multiple lots).

// 	remaining := input.QtyToTransfer

// 	for _, sourceInventory := range sourceRecords {
// 		if remaining <= 0 {
// 			break
// 		}

// 		// How much to take from this record
// 		take := sourceInventory.QtyAvailable
// 		if take > remaining {
// 			take = remaining
// 		}
// 		remaining -= take

// 		isSplit := sourceInventory.QtyAvailable > take

// 		// ── From/To lot number for this record ──
// 		fromLotNumber := sourceInventory.LotNumber
// 		toLotNumber := input.LotNumber
// 		if toLotNumber == "" {
// 			toLotNumber = fromLotNumber
// 		}

// 		// ── Snapshot source qty before mutating ──
// 		qtyOnhandBeforeSrc := sourceInventory.QtyOnhand
// 		qtyAvailableBeforeSrc := sourceInventory.QtyAvailable
// 		qtyOnhandAfterSrc := qtyOnhandBeforeSrc - take
// 		qtyAvailableAfterSrc := qtyAvailableBeforeSrc - take

// 		// Deduct source
// 		if err := tx.Model(&sourceInventory).Updates(map[string]interface{}{
// 			"qty_origin":    sourceInventory.QtyOrigin - take,
// 			"qty_onhand":    qtyOnhandAfterSrc,
// 			"qty_available": qtyAvailableAfterSrc,
// 			"updated_by":    userID,
// 			"updated_at":    time.Now().UTC(),
// 		}).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to update source inventory",
// 			})
// 		}

// 		// ── Resolve destination inventory (find matching bucket by toLotNumber, not raw input) ──
// 		var destInventory models.Inventory
// 		destQuery := tx.Where(
// 			"whs_code = ? AND location = ? AND item_code = ? AND barcode = ? AND COALESCE(carton_number, '') = COALESCE(?, '') AND qa_status = ? AND lot_number = ?",
// 			input.ToWhsCode,
// 			input.ToLocation,
// 			sourceInventory.ItemCode,
// 			sourceInventory.Barcode,
// 			sourceInventory.CartonNumber,
// 			newQaStatus,
// 			toLotNumber,
// 		)
// 		if input.RecDate != "" {
// 			destQuery = destQuery.Where("rec_date = ?", input.RecDate)
// 		}
// 		if input.ProdDate != "" {
// 			destQuery = destQuery.Where("prod_date = ?", input.ProdDate)
// 		}
// 		if input.ExpDate != "" {
// 			destQuery = destQuery.Where("exp_date = ?", input.ExpDate)
// 		}
// 		if input.DivisionCode != "" {
// 			destQuery = destQuery.Where("division_code = ?", input.DivisionCode)
// 		}

// 		err := destQuery.First(&destInventory).Error
// 		isNewDestination := err == gorm.ErrRecordNotFound

// 		if err != nil && err != gorm.ErrRecordNotFound {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to check destination inventory",
// 			})
// 		}

// 		// ── Resolve the pallet that will end up on the destination side ──
// 		// - Merging into an existing destination bucket -> keep that bucket's pallet.
// 		// - Creating a new destination bucket from a full (non-split) source record -> carry the source pallet.
// 		// - Creating a new destination bucket from a split source record -> a fresh pallet ID is generated,
// 		//   since the remainder stays behind on the original pallet.
// 		var toPallet string
// 		if !isNewDestination {
// 			toPallet = destInventory.Pallet
// 		} else if isSplit {
// 			generatedPallet, err := inventoryRepo.GeneratePalletID()
// 			if err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to generate new pallet ID",
// 				})
// 			}
// 			toPallet = generatedPallet
// 		} else {
// 			toPallet = sourceInventory.Pallet
// 		}
// 		fromPallet := sourceInventory.Pallet

// 		var destInventoryID uint
// 		var qtyOnhandBeforeDest, qtyAvailableBeforeDest float64
// 		var qtyOnhandAfterDest, qtyAvailableAfterDest float64

// 		if isNewDestination {
// 			qtyOnhandBeforeDest = 0
// 			qtyAvailableBeforeDest = 0
// 			qtyOnhandAfterDest = take
// 			qtyAvailableAfterDest = take

// 			newInventory := models.Inventory{
// 				OwnerCode:       sourceInventory.OwnerCode,
// 				WhsCode:         input.ToWhsCode,
// 				InboundID:       sourceInventory.InboundID,
// 				InboundDetailId: sourceInventory.InboundDetailId,
// 				DivisionCode:    input.DivisionCode,
// 				RecDate:         input.RecDate,
// 				ProdDate:        input.ProdDate,
// 				ExpDate:         input.ExpDate,
// 				LotNumber:       toLotNumber,
// 				Pallet:          toPallet,
// 				Location:        input.ToLocation,
// 				ItemId:          sourceInventory.ItemId,
// 				ItemCode:        sourceInventory.ItemCode,
// 				Barcode:         sourceInventory.Barcode,
// 				CartonNumber:    sourceInventory.CartonNumber,
// 				SerialNumber:    sourceInventory.SerialNumber,
// 				QaStatus:        newQaStatus,
// 				Uom:             sourceInventory.Uom,
// 				QtyOrigin:       take,
// 				QtyOnhand:       take,
// 				QtyAvailable:    take,
// 				QtyAllocated:    0,
// 				QtySuspend:      0,
// 				QtyShipped:      0,
// 				Trans:           "TRANSFER",
// 				IsTransfer:      true,
// 				TransferFrom:    sourceInventory.ID,
// 				CreatedBy:       userID,
// 				UpdatedBy:       userID,
// 			}

// 			if err := tx.Create(&newInventory).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to create destination inventory",
// 				})
// 			}
// 			destInventoryID = newInventory.ID
// 		} else {
// 			qtyOnhandBeforeDest = destInventory.QtyOnhand
// 			qtyAvailableBeforeDest = destInventory.QtyAvailable
// 			qtyOnhandAfterDest = qtyOnhandBeforeDest + take
// 			qtyAvailableAfterDest = qtyAvailableBeforeDest + take

// 			destInventory.QtyOrigin += take
// 			destInventory.QtyOnhand = qtyOnhandAfterDest
// 			destInventory.QtyAvailable = qtyAvailableAfterDest
// 			destInventory.UpdatedBy = userID

// 			if err := tx.Save(&destInventory).Error; err != nil {
// 				tx.Rollback()
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"success": false,
// 					"error":   "Failed to update destination inventory",
// 				})
// 			}
// 			destInventoryID = destInventory.ID
// 		}

// 		// ── Record source movement (now that destInventoryID/toPallet/toLotNumber are known) ──
// 		sourceMovement := models.InventoryMovement{
// 			MovementID:         movementID,
// 			InventoryID:        sourceInventory.ID,
// 			RefType:            "TRANSFER",
// 			RefID:              destInventoryID,
// 			ItemID:             sourceInventory.ItemId,
// 			ItemCode:           sourceInventory.ItemCode,
// 			QtyOnhandChange:    -take,
// 			QtyAvailableChange: -take,
// 			QtyAllocatedChange: 0,
// 			QtySuspendChange:   0,
// 			QtyShippedChange:   0,
// 			QtyOnhandBefore:    ptrFloat64(qtyOnhandBeforeSrc),
// 			QtyOnhandAfter:     ptrFloat64(qtyOnhandAfterSrc),
// 			QtyAvailableBefore: ptrFloat64(qtyAvailableBeforeSrc),
// 			QtyAvailableAfter:  ptrFloat64(qtyAvailableAfterSrc),
// 			FromWhsCode:        input.FromWhsCode,
// 			ToWhsCode:          input.ToWhsCode,
// 			FromLocation:       input.FromLocation,
// 			ToLocation:         input.ToLocation,
// 			OldQaStatus:        sourceInventory.QaStatus,
// 			NewQaStatus:        newQaStatus,
// 			FromDivision:       input.FromDivisionCode,
// 			ToDivision:         input.DivisionCode,
// 			FromPallet:         fromPallet,
// 			ToPallet:           toPallet,
// 			FromLotNumber:      fromLotNumber,
// 			ToLotNumber:        toLotNumber,
// 			Reason:             input.Reason,
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		}

// 		if err := tx.Create(&sourceMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to record source movement " + err.Error(),
// 			})
// 		}

// 		// ── Record destination movement ──
// 		destMovement := models.InventoryMovement{
// 			MovementID:         movementID,
// 			InventoryID:        destInventoryID,
// 			RefType:            "TRANSFER",
// 			RefID:              sourceInventory.ID,
// 			ItemID:             sourceInventory.ItemId,
// 			ItemCode:           sourceInventory.ItemCode,
// 			QtyOnhandChange:    take,
// 			QtyAvailableChange: take,
// 			QtyAllocatedChange: 0,
// 			QtySuspendChange:   0,
// 			QtyShippedChange:   0,
// 			QtyOnhandBefore:    ptrFloat64(qtyOnhandBeforeDest),
// 			QtyOnhandAfter:     ptrFloat64(qtyOnhandAfterDest),
// 			QtyAvailableBefore: ptrFloat64(qtyAvailableBeforeDest),
// 			QtyAvailableAfter:  ptrFloat64(qtyAvailableAfterDest),
// 			FromWhsCode:        input.FromWhsCode,
// 			ToWhsCode:          input.ToWhsCode,
// 			FromLocation:       input.FromLocation,
// 			ToLocation:         input.ToLocation,
// 			FromDivision:       input.FromDivisionCode,
// 			ToDivision:         input.DivisionCode,
// 			OldQaStatus:        sourceInventory.QaStatus,
// 			NewQaStatus:        newQaStatus,
// 			FromPallet:         fromPallet,
// 			ToPallet:           toPallet,
// 			FromLotNumber:      fromLotNumber,
// 			ToLotNumber:        toLotNumber,
// 			Reason:             input.Reason,
// 			CreatedBy:          userID,
// 			CreatedAt:          time.Now(),
// 		}
// 		if err := tx.Create(&destMovement).Error; err != nil {
// 			tx.Rollback()
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Failed to record destination movement",
// 			})
// 		}
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"success": false,
// 			"error":   "Failed to commit transaction",
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": fmt.Sprintf("Successfully transferred %.2f units from %s to %s", input.QtyToTransfer, input.FromLocation, input.ToLocation),
// 		"data": fiber.Map{
// 			"quantity_transferred": input.QtyToTransfer,
// 			"from":                 fmt.Sprintf("%s | %s", input.FromWhsCode, input.FromLocation),
// 			"to":                   fmt.Sprintf("%s | %s", input.ToWhsCode, input.ToLocation),
// 		},
// 	})
// }

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
