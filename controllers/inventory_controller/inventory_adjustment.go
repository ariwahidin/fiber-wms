package inventory_controller

import (
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AdjustmentController struct {
	DB *gorm.DB
}

func NewAdjustmentController(DB *gorm.DB) *AdjustmentController {
	return &AdjustmentController{DB: DB}
}

// ─── REQUEST STRUCTS ──────────────────────────────────────────────────────────

type CreateAdjustmentRequest struct {
	InventoryID uint    `json:"inventory_id" validate:"required"`
	ReasonCode  string  `json:"reason_code" validate:"required"`
	QtyAdjust   float64 `json:"qty_adjust" validate:"required"` // positif = tambah, negatif = kurangi
	Notes       string  `json:"notes"`
}

type ApproveAdjustmentRequest struct {
	RejectNote string `json:"reject_note"` // hanya diisi saat reject
}

// ─── GET ALL ──────────────────────────────────────────────────────────────────

// GET /inventory/adjustments
func (c *AdjustmentController) GetAll(ctx *fiber.Ctx) error {
	repo := repositories.NewAdjustmentRepository(c.DB)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.Query("page_size", "20"))

	result, err := repo.GetAll(repositories.AdjustmentFilter{
		Status:    ctx.Query("status"),
		OwnerCode: ctx.Query("owner_code"),
		WhsCode:   ctx.Query("whs_code"),
		ItemCode:  ctx.Query("item_code"),
		DateFrom:  ctx.Query("date_from"),
		DateTo:    ctx.Query("date_to"),
		Search:    ctx.Query("search"),
		Page:      page,
		PageSize:  pageSize,
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch adjustments",
			"error":   err.Error(),
		})
	}

	// filter options
	var statuses []string
	c.DB.Model(&models.InventoryAdjustment{}).
		Distinct("status").Order("status ASC").Pluck("status", &statuses)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result.Data,
		"pagination": fiber.Map{
			"page":        result.Page,
			"page_size":   result.PageSize,
			"total":       result.Total,
			"total_pages": result.TotalPages,
		},
		"filters": fiber.Map{
			"statuses": statuses,
		},
	})
}

// ─── GET BY ID ────────────────────────────────────────────────────────────────

// GET /inventory/adjustments/:id
func (c *AdjustmentController) GetByID(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	repo := repositories.NewAdjustmentRepository(c.DB)
	adj, err := repo.GetByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Adjustment not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    adj,
	})
}

// ─── CREATE (REQUEST ADJUSTMENT) ─────────────────────────────────────────────

// POST /inventory/adjustments
func (c *AdjustmentController) Create(ctx *fiber.Ctx) error {
	var req CreateAdjustmentRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid request body: " + err.Error(),
		})
	}

	if req.InventoryID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "inventory_id is required",
		})
	}
	if req.ReasonCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "reason_code is required",
		})
	}
	if req.QtyAdjust == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "qty_adjust cannot be zero",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Ambil inventory yang dimaksud
	var inv models.Inventory
	if err := c.DB.Preload("Product").First(&inv, req.InventoryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false, "message": "Inventory not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	// Validasi: pengurangan tidak melebihi qty_available
	qtyAfter := inv.QtyOnhand + req.QtyAdjust
	if qtyAfter < 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf(
				"Insufficient stock. Onhand: %.2f, Adjust: %.2f, Result: %.2f",
				inv.QtyOnhand, req.QtyAdjust, qtyAfter,
			),
		})
	}

	adj := models.InventoryAdjustment{
		InventoryID: inv.ID,
		OwnerCode:   inv.OwnerCode,
		WhsCode:     inv.WhsCode,
		Location:    inv.Location,
		ItemID:      inv.ItemId,
		ItemCode:    inv.ItemCode,
		Barcode:     inv.Barcode,
		LotNumber:   inv.LotNumber,
		Uom:         inv.Uom,
		ReasonCode:  req.ReasonCode,
		Notes:       req.Notes,
		QtyBefore:   inv.QtyOnhand,
		QtyAdjust:   req.QtyAdjust,
		QtyAfter:    qtyAfter,
		Status:      models.AdjStatusPending, // langsung pending, bukan draft
		RequestedBy: userID,
		RequestedAt: time.Now(),
	}

	repo := repositories.NewAdjustmentRepository(c.DB)
	if err := repo.Create(&adj); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to create adjustment request",
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Adjustment request submitted successfully",
		"data":    adj,
	})
}

func floatPtr(f float64) *float64 { return &f }

// ─── APPROVE ─────────────────────────────────────────────────────────────────

// POST /inventory/adjustments/:id/approve
func (c *AdjustmentController) Approve(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	userID := int(ctx.Locals("userID").(float64))
	repo := repositories.NewAdjustmentRepository(c.DB)

	adj, err := repo.GetByID(uint(id))
	if err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false, "message": "Adjustment not found",
		})
	}

	if adj.Status != models.AdjStatusPending {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot approve adjustment with status: %s", adj.Status),
		})
	}

	// Tidak boleh approve request dari diri sendiri
	// if adj.RequestedBy == userID {
	// 	return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
	// 		"success": false, "message": "You cannot approve your own adjustment request",
	// 	})
	// }

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update status ke approved
	if err := repo.UpdateStatus(tx, adj.ID, models.AdjStatusApproved, userID, ""); err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	// ─── APPLY KE INVENTORY ───────────────────────────────────────────────
	// Ambil inventory terbaru (bisa berubah sejak request dibuat)
	var inv models.Inventory
	if err := tx.First(&inv, adj.InventoryID).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Inventory not found when applying adjustment",
		})
	}
	oldQtyOnhand := inv.QtyOnhand
	oldQtyAvailable := inv.QtyAvailable

	// Re-validasi qty setelah approve
	newQtyOnhand := inv.QtyOnhand + adj.QtyAdjust
	newQtyAvailable := inv.QtyAvailable + adj.QtyAdjust
	if newQtyOnhand < 0 {
		tx.Rollback()
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Insufficient stock at time of approval. Onhand: %.2f, Adjust: %.2f", inv.QtyOnhand, adj.QtyAdjust),
		})
	}

	if err := tx.Model(&inv).
		Select("qty_onhand", "qty_available", "updated_by", "updated_at").
		Updates(map[string]interface{}{
			"qty_onhand":    newQtyOnhand,
			"qty_available": newQtyAvailable,
			"updated_by":    userID,
			"updated_at":    time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": "Failed to update inventory: " + err.Error(),
		})
	}

	// ─── CATAT KE INVENTORY MOVEMENT ──────────────────────────────────────
	movementID := uuid.NewString()
	movement := models.InventoryMovement{
		MovementID:         movementID,
		InventoryID:        inv.ID,
		ItemID:             inv.ItemId,
		ItemCode:           inv.ItemCode,
		RefType:            "ADJUST",
		RefID:              adj.ID,
		QtyOnhandChange:    adj.QtyAdjust,
		QtyAvailableChange: adj.QtyAdjust,

		// Snapshot before/after — diisi karena ini transaksi baru
		QtyOnhandBefore:    floatPtr(oldQtyOnhand), // qty sebelum adjustment
		QtyOnhandAfter:     floatPtr(newQtyOnhand), // qty setelah adjustment
		QtyAvailableBefore: floatPtr(oldQtyAvailable),
		QtyAvailableAfter:  floatPtr(newQtyAvailable),

		FromWhsCode:  inv.WhsCode,
		ToWhsCode:    inv.WhsCode,
		FromLocation: inv.Location,
		ToLocation:   inv.Location,
		OldQaStatus:  inv.QaStatus,
		NewQaStatus:  inv.QaStatus,
		Reason:       fmt.Sprintf("[%s] %s", adj.ReasonCode, adj.Notes),
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
	}

	if err := tx.Create(&movement).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": "Failed to record movement: " + err.Error(),
		})
	}

	// Update adj: applied_at + link ke movement
	now := time.Now()
	movUint := movement.ID
	if err := tx.Model(&models.InventoryAdjustment{}).Where("id = ?", adj.ID).Updates(map[string]interface{}{
		"status":      models.AdjStatusApplied,
		"applied_at":  now,
		"movement_id": movUint,
	}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": "Failed to commit: " + err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Adjustment approved and applied. Qty changed: %.2f", adj.QtyAdjust),
		"data": fiber.Map{
			"adj_number":    adj.AdjNumber,
			"qty_before":    adj.QtyBefore,
			"qty_adjust":    adj.QtyAdjust,
			"qty_after_new": newQtyOnhand,
			"movement_id":   movementID,
		},
	})
}

// ─── REJECT ──────────────────────────────────────────────────────────────────

// POST /inventory/adjustments/:id/reject
func (c *AdjustmentController) Reject(ctx *fiber.Ctx) error {
	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid ID",
		})
	}

	var req ApproveAdjustmentRequest
	ctx.BodyParser(&req)

	if req.RejectNote == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "reject_note is required",
		})
	}

	userID := int(ctx.Locals("userID").(float64))
	repo := repositories.NewAdjustmentRepository(c.DB)

	adj, err := repo.GetByID(uint(id))
	if err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false, "message": "Adjustment not found",
		})
	}

	if adj.Status != models.AdjStatusPending {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot reject adjustment with status: %s", adj.Status),
		})
	}

	if err := repo.UpdateStatus(c.DB, adj.ID, models.AdjStatusRejected, userID, req.RejectNote); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Adjustment rejected",
	})
}

// ─── GET REASON CODES ────────────────────────────────────────────────────────

// GET /inventory/adjustments/reason-codes
func (c *AdjustmentController) GetReasonCodes(ctx *fiber.Ctx) error {
	repo := repositories.NewAdjustmentRepository(c.DB)
	codes, err := repo.GetReasonCodes()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "error": err.Error(),
		})
	}

	// Kalau tabel masih kosong, return default hardcoded
	if len(codes) == 0 {
		codes = defaultReasonCodes()
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    codes,
	})
}

func defaultReasonCodes() []models.AdjustmentReasonCode {
	return []models.AdjustmentReasonCode{
		{Code: "DMGD", Description: "Damaged goods", Direction: "out", RequireNote: true},
		{Code: "EXPD", Description: "Expired / past expiry date", Direction: "out", RequireNote: true},
		{Code: "OPNAME", Description: "Stock count result", Direction: "both", RequireNote: false},
		{Code: "RECV_ERR", Description: "Receiving discrepancy", Direction: "both", RequireNote: true},
		{Code: "SYS_ERR", Description: "System correction", Direction: "both", RequireNote: true},
		{Code: "SHRINK", Description: "Unknown loss / shrinkage", Direction: "out", RequireNote: true},
		{Code: "FOUND", Description: "Found unrecorded stock", Direction: "in", RequireNote: true},
		{Code: "PROD_LOSS", Description: "Production / repacking loss", Direction: "out", RequireNote: false},
	}
}
