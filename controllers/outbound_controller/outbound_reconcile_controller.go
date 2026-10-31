package outbound_controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"fiber-app/models"
	"fiber-app/repositories"
)

// GetExistingAllocation returns the total active allocation
// for a specific outbound detail.
func GetExistingAllocation(
	tx *gorm.DB,
	outboundDetailID uint,
) (float64, error) {

	var allocatedQty float64

	err := tx.Model(&models.OutboundPicking{}).
		Where(
			"outbound_detail_id = ? AND deleted_at IS NULL",
			outboundDetailID,
		).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&allocatedQty).Error

	return allocatedQty, err
}

// HasOutboundBarcode checks whether an outbound detail
// already has barcode records.
func HasOutboundBarcode(
	tx *gorm.DB,
	outboundDetailID uint,
) (bool, error) {

	var count int64

	err := tx.Model(&models.OutboundBarcode{}).
		Where(
			"outbound_detail_id = ? AND COALESCE(status, '') <> ?",
			int(outboundDetailID),
			"cancelled",
		).
		Count(&count).Error

	return count > 0, err
}

// ReleaseExcessAllocation releases excess allocation
// back to the original inventory.
func ReleaseExcessAllocation(
	tx *gorm.DB,
	outboundDetail models.OutboundDetail,
	excessQty float64,
	userID int,
) error {

	if excessQty <= 0 {
		return nil
	}

	// Safety: do not release allocation if scan records exist.
	hasScan, err := HasOutboundBarcode(tx, outboundDetail.ID)
	if err != nil {
		return err
	}

	if hasScan {
		return fmt.Errorf(
			"cannot reduce allocation for item %s because barcode records already exist",
			outboundDetail.ItemCode,
		)
	}

	var pickings []models.OutboundPicking

	if err := tx.
		Where(
			"outbound_detail_id = ? AND deleted_at IS NULL",
			outboundDetail.ID,
		).
		Order("created_at DESC, id DESC").
		Find(&pickings).Error; err != nil {
		return err
	}

	for _, picking := range pickings {

		if excessQty <= 0.000001 {
			break
		}

		if picking.Quantity <= 0 {
			continue
		}

		releaseQty := picking.Quantity

		if releaseQty > excessQty {
			releaseQty = excessQty
		}

		// Restore the original inventory.
		result := tx.Model(&models.Inventory{}).
			Where(
				"id = ? AND deleted_at IS NULL AND qty_allocated >= ?",
				picking.InventoryID,
				releaseQty,
			).
			Updates(map[string]interface{}{
				"qty_available": gorm.Expr(
					"qty_available + ?", releaseQty,
				),
				"qty_allocated": gorm.Expr(
					"qty_allocated - ?", releaseQty,
				),
				"updated_by": userID,
				"updated_at": time.Now(),
			})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf(
				"failed to release inventory ID %d: allocated quantity insufficient",
				picking.InventoryID,
			)
		}

		remainingPickingQty := picking.Quantity - releaseQty

		if remainingPickingQty <= 0.000001 {

			// Hard delete the picking record.
			if err := tx.Unscoped().Delete(&picking).Error; err != nil {
				return err
			}
		} else {

			// Maintain the original UOM conversion ratio.
			var remainingQtyDisplay float64

			if picking.Quantity > 0 {
				remainingQtyDisplay = picking.QtyDisplay *
					(remainingPickingQty / picking.Quantity)
			}

			if err := tx.Model(&picking).
				Updates(map[string]interface{}{
					"quantity":    remainingPickingQty,
					"qty_display": remainingQtyDisplay,
					"updated_by":  userID,
					"updated_at":  time.Now(),
				}).Error; err != nil {
				return err
			}
		}

		excessQty -= releaseQty
	}

	if excessQty > 0.000001 {
		return fmt.Errorf(
			"unable to release all excess allocation for item %s, remaining %.4f",
			outboundDetail.ItemCode,
			excessQty,
		)
	}

	return nil
}

// AllocateOutboundDetails allocates only the remaining quantity.
// Existing allocations are preserved.
func AllocateOutboundDetails(
	tx *gorm.DB,
	outboundHeader models.OutboundHeader,
	outboundDetails []models.OutboundDetail,
	inventoryPolicy models.InventoryPolicy,
	userID int,
) error {

	uomRepo := repositories.NewUomRepository(tx)
	locationRepo := repositories.NewLocationRepository(tx)

	for _, outboundDetail := range outboundDetails {

		// ============================================================
		// 1. UOM Conversion
		// ============================================================

		uomConversion, err := uomRepo.ConversionQty(
			outboundDetail.ItemCode,
			outboundDetail.Quantity,
			outboundDetail.Uom,
		)

		if err != nil {
			return fmt.Errorf(
				"UOM conversion error for %s: %w",
				outboundDetail.ItemCode,
				err,
			)
		}

		qtyReq := uomConversion.QtyConverted

		// ============================================================
		// 2. Get Existing Allocation
		// ============================================================

		allocatedQty, err := GetExistingAllocation(
			tx,
			outboundDetail.ID,
		)

		if err != nil {
			return err
		}

		// ============================================================
		// 3. Calculate Remaining Quantity
		// ============================================================

		remainingQty := qtyReq - allocatedQty

		if remainingQty <= 0.000001 {
			continue
		}

		qtyReq = remainingQty

		// ============================================================
		// 4. Build Inventory Query
		// ============================================================

		queryInventory := tx.
			Table("inventories as i").
			Joins(`
				LEFT JOIN locations as l
				ON i.whs_code = l.whs_code
				AND i.location = l.location_code
			`).
			Joins(`
				LEFT JOIN (
					SELECT
						location,
						whs_code,
						item_id,
						MIN(rec_date) AS min_rec_date,
						MIN(qty_available) AS min_qty,
						MIN(exp_date) AS min_exp_date
					FROM inventories
					WHERE qty_available > 0
					AND deleted_at IS NULL
					GROUP BY location, whs_code, item_id
				) AS loc_rank
				ON loc_rank.location = i.location
				AND loc_rank.whs_code = i.whs_code
				AND loc_rank.item_id = i.item_id
			`).
			Where(`
				i.item_id = ?
				AND i.whs_code = ?
				AND i.qty_available > 0
				AND i.deleted_at IS NULL
				AND i.uom = ?
				AND i.owner_code = ?
				AND i.qa_status = ?
				AND i.division_code = ?
				AND (
					l.id IS NULL
					OR (l.is_active = 1 AND l.is_pickable = 1)
				)
			`,
				outboundDetail.ItemID,
				outboundDetail.WhsCode,
				uomConversion.ToUom,
				outboundHeader.OwnerCode,
				outboundDetail.QaStatus,
				outboundDetail.DivisionCode,
			)

		// ============================================================
		// 5. Inventory Policy Filters
		// ============================================================

		if inventoryPolicy.RequireLotNumber {
			if outboundDetail.LotNumber == "" {
				return fmt.Errorf(
					"lot number is required for item %s",
					outboundDetail.ItemCode,
				)
			}

			queryInventory = queryInventory.Where(
				"i.lot_number = ?",
				outboundDetail.LotNumber,
			)
		}

		if inventoryPolicy.AllocationLotByOrder &&
			outboundDetail.LotNumber != "" {

			queryInventory = queryInventory.Where(
				"i.lot_number = ?",
				outboundDetail.LotNumber,
			)
		}

		if inventoryPolicy.AllocationLocationByOrder &&
			outboundDetail.Location != "" {

			queryInventory = queryInventory.Where(
				"i.location = ?",
				outboundDetail.Location,
			)
		}

		if inventoryPolicy.AllocationCaseByOrder &&
			inventoryPolicy.UseCaseNumber &&
			outboundDetail.CaseNumber != "" {

			queryInventory = queryInventory.Where(
				"i.case_number = ?",
				outboundDetail.CaseNumber,
			)
		}

		if inventoryPolicy.AllocationCartonByOrder &&
			inventoryPolicy.UseCartonNumber &&
			outboundDetail.CartonNumber != "" {

			queryInventory = queryInventory.Where(
				"i.carton_number = ?",
				outboundDetail.CartonNumber,
			)
		}

		if inventoryPolicy.AllocationSerialByOrder &&
			inventoryPolicy.UseSerialNumber &&
			outboundDetail.SerialNumber != "" {

			queryInventory = queryInventory.Where(
				"i.serial_number = ?",
				outboundDetail.SerialNumber,
			)
		}

		// ============================================================
		// 6. FIFO / FEFO Ordering
		// ============================================================

		if inventoryPolicy.UseFEFO {

			queryInventory = queryInventory.Order(`
				loc_rank.min_exp_date ASC,
				loc_rank.min_qty ASC,
				i.location ASC,
				i.exp_date ASC,
				i.lot_number ASC,
				i.rec_date ASC,
				i.qty_available ASC,
				i.pallet ASC
			`)

		} else {

			queryInventory = queryInventory.Order(`
				loc_rank.min_rec_date ASC,
				loc_rank.min_qty ASC,
				i.location ASC,
				i.qty_available ASC,
				i.rec_date ASC
			`)
		}

		if inventoryPolicy.PickingExcludeLocationsUnderCycleCount {
			queryInventory = locationRepo.ExcludeLocationsUnderCycleCount(
				queryInventory,
				"i",
			)
		}

		// ============================================================
		// 7. Fetch Inventory
		// ============================================================

		var inventories []models.Inventory

		if err := queryInventory.
			Select("i.*").
			Find(&inventories).Error; err != nil {
			return err
		}

		if len(inventories) == 0 {
			return fmt.Errorf(
				"insufficient stock available for item %s",
				outboundDetail.ItemCode,
			)
		}

		// ============================================================
		// 8. Product Lookup
		// ============================================================

		var product models.Product

		if err := tx.
			Where("id = ?", outboundDetail.ItemID).
			First(&product).Error; err != nil {
			return fmt.Errorf(
				"product not found for item %s",
				outboundDetail.ItemCode,
			)
		}

		// ============================================================
		// 9. Allocate Inventory
		// ============================================================

		for _, inventory := range inventories {

			if qtyReq <= 0.000001 {
				break
			}

			qtyPick := inventory.QtyAvailable

			if qtyPick > qtyReq {
				qtyPick = qtyReq
			}

			// Insert new picking allocation.
			pickingSheet := models.OutboundPicking{
				InventoryID:      int(inventory.ID),
				OutboundId:       outboundDetail.OutboundID,
				OutboundNo:       outboundDetail.OutboundNo,
				OutboundDetailId: int(outboundDetail.ID),

				OwnerCode:    inventory.OwnerCode,
				WhsCode:      inventory.WhsCode,
				DivisionCode: inventory.DivisionCode,

				ItemID:   inventory.ItemId,
				ItemCode: inventory.ItemCode,
				Barcode:  product.Barcode,

				Pallet:   inventory.Pallet,
				Location: inventory.Location,
				Quantity: qtyPick,

				Uom:      inventory.Uom,
				QaStatus: inventory.QaStatus,

				RecDate:      inventory.RecDate,
				ProdDate:     inventory.ProdDate,
				ExpDate:      inventory.ExpDate,
				LotNumber:    inventory.LotNumber,
				CartonNumber: inventory.CartonNumber,
				CaseNumber:   inventory.CaseNumber,
				SerialNumber: inventory.SerialNumber,

				UomDisplay: outboundDetail.Uom,
				QtyDisplay: qtyPick / uomConversion.Rate,
				EanDisplay: uomConversion.Ean,

				CreatedBy: userID,
			}

			if err := tx.Create(&pickingSheet).Error; err != nil {
				return err
			}

			// Update original inventory.
			result := tx.Model(&models.Inventory{}).
				Where(
					"id = ? AND qty_available >= ?",
					inventory.ID,
					qtyPick,
				).
				Updates(map[string]interface{}{
					"qty_available": gorm.Expr(
						"qty_available - ?", qtyPick,
					),
					"qty_allocated": gorm.Expr(
						"qty_allocated + ?", qtyPick,
					),
					"updated_by": userID,
					"updated_at": time.Now(),
				})

			if result.Error != nil {
				return result.Error
			}

			if result.RowsAffected == 0 {
				return fmt.Errorf(
					"inventory %d is no longer available",
					inventory.ID,
				)
			}

			qtyReq -= qtyPick
		}

		// ============================================================
		// 10. Validate Remaining Quantity
		// ============================================================

		if qtyReq > 0.000001 {
			return fmt.Errorf(
				"insufficient stock for item %s, shortage %.4f",
				outboundDetail.ItemCode,
				qtyReq,
			)
		}
	}

	return nil
}

type ReconcileOutboundRequest struct {
	Items   []ReconcileOutboundItem `json:"items"`
	Remarks string                  `json:"remarks"`
}

type ReconcileOutboundItem struct {
	Action           string                `json:"action"`
	OutboundDetailID uint                  `json:"outbound_detail_id"`
	Quantity         float64               `json:"quantity"`
	Detail           models.OutboundDetail `json:"detail"`
}

func (r *OutboundController) ReconcileOutbound(
	ctx *fiber.Ctx,
) error {

	id, err := ctx.ParamsInt("id")
	if err != nil || id <= 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "Invalid outbound ID"},
		)
	}

	var payload ReconcileOutboundRequest

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "Invalid payload"},
		)
	}

	if len(payload.Items) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{"error": "No changes provided"},
		)
	}

	userIDValue, ok := ctx.Locals("userID").(float64)
	if !ok {
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			fiber.Map{"error": "Invalid user ID"},
		)
	}

	userID := int(userIDValue)

	tx := r.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{"error": tx.Error.Error()},
		)
	}

	defer func() {
		if recover() != nil {
			tx.Rollback()
		}
	}()

	rollbackError := func(status int, message string) error {
		tx.Rollback()
		return ctx.Status(status).JSON(
			fiber.Map{"success": false, "message": message},
		)
	}

	// ============================================================
	// 1. Get Outbound Header
	// ============================================================

	var outboundHeader models.OutboundHeader

	if err := tx.First(
		&outboundHeader,
		"id = ?",
		id,
	).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rollbackError(
				fiber.StatusNotFound,
				"Outbound not found",
			)
		}

		return rollbackError(
			fiber.StatusInternalServerError,
			err.Error(),
		)
	}

	// Only picking and packing outbound can be reconciled.
	if outboundHeader.Status != "picking" &&
		outboundHeader.Status != "packing" {

		return rollbackError(
			fiber.StatusBadRequest,
			"Only picking or packing outbound can be edited",
		)
	}

	// ============================================================
	// 2. Get Inventory Policy
	// ============================================================

	var inventoryPolicy models.InventoryPolicy

	if err := tx.First(
		&inventoryPolicy,
		"owner_code = ?",
		outboundHeader.OwnerCode,
	).Error; err != nil {

		return rollbackError(
			fiber.StatusBadRequest,
			"Inventory policy not found",
		)
	}

	// ============================================================
	// 3. Process Changes
	// ============================================================

	for _, item := range payload.Items {

		switch item.Action {

		// --------------------------------------------------------
		// ADD ITEM
		// --------------------------------------------------------

		case "add":

			if item.Detail.ItemID <= 0 ||
				item.Detail.Quantity <= 0 ||
				item.Detail.Uom == "" {

				return rollbackError(
					fiber.StatusBadRequest,
					"Invalid new item data",
				)
			}

			var product models.Product

			if err := tx.First(
				&product,
				"id = ?",
				item.Detail.ItemID,
			).Error; err != nil {

				return rollbackError(
					fiber.StatusNotFound,
					"Product not found",
				)
			}

			detail := item.Detail

			detail.ID = 0
			detail.OutboundID = outboundHeader.ID
			detail.OutboundNo = outboundHeader.OutboundNo
			detail.CustomerCode = outboundHeader.CustomerCode
			detail.OwnerCode = outboundHeader.OwnerCode
			detail.WhsCode = outboundHeader.WhsCode

			detail.ItemCode = product.ItemCode
			detail.Barcode = product.Barcode

			if detail.DivisionCode == "" {
				detail.DivisionCode = "REGULAR"
			}

			if detail.QaStatus == "" {
				detail.QaStatus = "A"
			}

			detail.Status = "draft"
			detail.CreatedBy = userID
			detail.CreatedAt = time.Now()

			if err := tx.Create(&detail).Error; err != nil {
				return rollbackError(
					fiber.StatusInternalServerError,
					err.Error(),
				)
			}

		// --------------------------------------------------------
		// UPDATE QUANTITY
		// --------------------------------------------------------

		case "update":

			if item.OutboundDetailID == 0 ||
				item.Quantity <= 0 {

				return rollbackError(
					fiber.StatusBadRequest,
					"Invalid update data",
				)
			}

			var detail models.OutboundDetail

			if err := tx.Where(
				"outbound_id = ? AND id = ?",
				outboundHeader.ID,
				item.OutboundDetailID,
			).First(&detail).Error; err != nil {

				return rollbackError(
					fiber.StatusNotFound,
					"Outbound detail not found",
				)
			}

			if item.Quantity < detail.Quantity {

				hasScan, err := HasOutboundBarcode(
					tx,
					detail.ID,
				)

				if err != nil {
					return rollbackError(
						fiber.StatusInternalServerError,
						err.Error(),
					)
				}

				if hasScan {
					return rollbackError(
						fiber.StatusBadRequest,
						fmt.Sprintf(
							"Cannot reduce quantity for %s because barcode records already exist",
							detail.ItemCode,
						),
					)
				}
			}

			if err := tx.Model(&detail).Updates(
				map[string]interface{}{
					"quantity":   item.Quantity,
					"updated_by": userID,
					"updated_at": time.Now(),
				},
			).Error; err != nil {

				return rollbackError(
					fiber.StatusInternalServerError,
					err.Error(),
				)
			}

		// --------------------------------------------------------
		// REMOVE ITEM
		// --------------------------------------------------------

		case "remove":

			if item.OutboundDetailID == 0 {
				return rollbackError(
					fiber.StatusBadRequest,
					"Outbound detail ID is required",
				)
			}

			var detail models.OutboundDetail

			if err := tx.Where(
				"outbound_id = ? AND id = ?",
				outboundHeader.ID,
				item.OutboundDetailID,
			).First(&detail).Error; err != nil {

				return rollbackError(
					fiber.StatusNotFound,
					"Outbound detail not found",
				)
			}

			hasScan, err := HasOutboundBarcode(
				tx,
				detail.ID,
			)

			if err != nil {
				return rollbackError(
					fiber.StatusInternalServerError,
					err.Error(),
				)
			}

			if hasScan {
				return rollbackError(
					fiber.StatusBadRequest,
					fmt.Sprintf(
						"Cannot remove item %s because barcode records already exist",
						detail.ItemCode,
					),
				)
			}

			allocatedQty, err := GetExistingAllocation(
				tx,
				detail.ID,
			)

			if err != nil {
				return rollbackError(
					fiber.StatusInternalServerError,
					err.Error(),
				)
			}

			if allocatedQty > 0 {
				if err := ReleaseExcessAllocation(
					tx,
					detail,
					allocatedQty,
					userID,
				); err != nil {

					return rollbackError(
						fiber.StatusInternalServerError,
						err.Error(),
					)
				}
			}

			if err := tx.Delete(&detail).Error; err != nil {
				return rollbackError(
					fiber.StatusInternalServerError,
					err.Error(),
				)
			}

		default:

			return rollbackError(
				fiber.StatusBadRequest,
				"Unsupported action: "+item.Action,
			)
		}
	}

	// ============================================================
	// 4. Reload Outbound Details
	// ============================================================

	var outboundDetails []models.OutboundDetail

	if err := tx.Where(
		"outbound_id = ?",
		outboundHeader.ID,
	).Find(&outboundDetails).Error; err != nil {

		return rollbackError(
			fiber.StatusInternalServerError,
			err.Error(),
		)
	}

	// ============================================================
	// 5. Reconcile Existing Allocation
	// ============================================================

	uomRepo := repositories.NewUomRepository(tx)

	for _, detail := range outboundDetails {

		uomConversion, err := uomRepo.ConversionQty(
			detail.ItemCode,
			detail.Quantity,
			detail.Uom,
		)

		if err != nil {
			return rollbackError(
				fiber.StatusInternalServerError,
				"UOM conversion error: "+err.Error(),
			)
		}

		allocatedQty, err := GetExistingAllocation(
			tx,
			detail.ID,
		)

		if err != nil {
			return rollbackError(
				fiber.StatusInternalServerError,
				err.Error(),
			)
		}

		excessQty := allocatedQty - uomConversion.QtyConverted

		if excessQty > 0.000001 {

			if err := ReleaseExcessAllocation(
				tx,
				detail,
				excessQty,
				userID,
			); err != nil {

				return rollbackError(
					fiber.StatusBadRequest,
					err.Error(),
				)
			}
		}
	}

	// ============================================================
	// 6. Allocate Remaining Quantity
	// ============================================================

	if err := AllocateOutboundDetails(
		tx,
		outboundHeader,
		outboundDetails,
		inventoryPolicy,
		userID,
	); err != nil {

		return rollbackError(
			fiber.StatusBadRequest,
			err.Error(),
		)
	}

	// ============================================================
	// 7. Update Header Audit
	// ============================================================

	headerUpdates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now(),
	}
	if payload.Remarks != "" {
		headerUpdates["remarks"] = payload.Remarks
	}

	if err := tx.Model(&models.OutboundHeader{}).
		Where("id = ?", outboundHeader.ID).
		Updates(headerUpdates).Error; err != nil {

		return rollbackError(
			fiber.StatusInternalServerError,
			err.Error(),
		)
	}

	// ============================================================
	// 8. Commit
	// ============================================================

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(
			fiber.StatusInternalServerError,
		).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit reconciliation",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(
		fiber.Map{
			"success": true,
			"message": "Outbound reconciliation completed successfully",
		},
	)
}
