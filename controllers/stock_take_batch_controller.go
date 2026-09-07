package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StockTakeBatchController struct {
	DB *gorm.DB
}

func NewStockTakeBatchController(DB *gorm.DB) *StockTakeBatchController {
	return &StockTakeBatchController{
		DB: DB,
	}
}

func (c *StockTakeBatchController) generateBatchCodeTx(tx *gorm.DB) (string, error) {
	now := time.Now()

	datePrefix := "STB" + now.Format("20060102")

	var lastCode string

	err := tx.
		Model(&models.StockTakeBatch{}).
		Select("code").
		Where("code LIKE ?", datePrefix+"%").
		Order("code DESC").
		Limit(1).
		Pluck("code", &lastCode).Error

	if err != nil {
		return "", err
	}

	seq := 1

	if lastCode != "" {
		// Ambil 4 digit terakhir
		lastSeq := lastCode[len(lastCode)-4:]

		lastSeqInt, err := strconv.Atoi(lastSeq)
		if err != nil {
			return "", fmt.Errorf("invalid batch code sequence: %s", lastCode)
		}

		seq = lastSeqInt + 1
	}

	return fmt.Sprintf("%s%04d", datePrefix, seq), nil
}

func (c *StockTakeBatchController) CreateStockTakeBatch(ctx *fiber.Ctx) error {
	type Input struct {
		OwnerCode   string `json:"owner_code"`
		Description string `json:"description"`
	}

	var input Input

	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if input.OwnerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Owner code is required",
		})
	}

	var batch models.StockTakeBatch

	err := c.DB.Transaction(func(tx *gorm.DB) error {

		code, err := c.generateBatchCodeTx(tx)
		if err != nil {
			return err
		}

		batch = models.StockTakeBatch{
			Code:        code,
			OwnerCode:   input.OwnerCode,
			Description: input.Description,
			Status:      "open",
			CreatedBy:   int(ctx.Locals("userID").(float64)),
		}

		if err := tx.Create(&batch).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create stock take batch",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Stock take batch created successfully",
		"data":    batch,
	})
}

type StockTakeBatchListItem struct {
	models.StockTakeBatch

	TotalSessions      int64 `json:"total_sessions"`
	CompletedSessions  int64 `json:"completed_sessions"`
	InProgressSessions int64 `json:"in_progress_sessions"`
	OpenSessions       int64 `json:"open_sessions"`
	CancelledSessions  int64 `json:"cancelled_sessions"`

	TotalLocations   int64 `json:"total_locations"`
	CountedLocations int64 `json:"counted_locations"`

	TotalSystemQty  int64 `json:"total_system_qty"`
	TotalCountedQty int64 `json:"total_counted_qty"`
	TotalDifference int64 `json:"total_difference"`
}

func (c *StockTakeBatchController) GetAllStockTakeBatch(ctx *fiber.Ctx) error {
	page, err := strconv.Atoi(ctx.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(ctx.Query("page_size", "20"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	search := strings.TrimSpace(ctx.Query("search"))
	statuses := strings.TrimSpace(ctx.Query("statuses"))

	offset := (page - 1) * pageSize

	// =========================================================
	// BASE QUERY
	// =========================================================
	baseQuery := c.DB.
		Model(&models.StockTakeBatch{}).
		Where("stock_take_batches.deleted_at IS NULL")

	// =========================================================
	// DATE FILTER
	// =========================================================
	if startDate != "" {
		startParsed, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid start_date format. Use YYYY-MM-DD",
			})
		}

		baseQuery = baseQuery.Where(
			"stock_take_batches.created_at >= ?",
			startParsed,
		)
	}

	if endDate != "" {
		endParsed, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid end_date format. Use YYYY-MM-DD",
			})
		}

		// Exclusive upper bound
		endExclusive := endParsed.AddDate(0, 0, 1)

		baseQuery = baseQuery.Where(
			"stock_take_batches.created_at < ?",
			endExclusive,
		)
	}

	// =========================================================
	// SEARCH
	// =========================================================
	if search != "" {
		keyword := "%" + search + "%"

		baseQuery = baseQuery.Where(`
			(
				stock_take_batches.code LIKE ?
				OR stock_take_batches.owner_code LIKE ?
				OR stock_take_batches.description LIKE ?
			)
		`, keyword, keyword, keyword)
	}

	// =========================================================
	// STATUS FILTER
	// =========================================================
	if statuses != "" {
		statusList := strings.Split(statuses, ",")

		var cleanedStatuses []string

		for _, status := range statusList {
			status = strings.TrimSpace(status)

			if status != "" {
				cleanedStatuses = append(cleanedStatuses, status)
			}
		}

		if len(cleanedStatuses) > 0 {
			baseQuery = baseQuery.Where(
				"stock_take_batches.status IN ?",
				cleanedStatuses,
			)
		}
	}

	// =========================================================
	// TOTAL COUNT
	// =========================================================
	var total int64

	if err := baseQuery.Count(&total).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to count stock take batches",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// RESULT STRUCT
	// =========================================================
	type StockTakeBatchListItem struct {
		ID          uint       `json:"id"`
		Code        string     `json:"code"`
		OwnerCode   string     `json:"owner_code"`
		Description string     `json:"description"`
		Status      string     `json:"status"`
		CreatedBy   int        `json:"created_by"`
		UpdatedBy   int        `json:"updated_by"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
		StartedAt   *time.Time `json:"started_at"`
		ClosedAt    *time.Time `json:"closed_at"`
		ClosedBy    *int       `json:"closed_by"`
		CancelAt    *time.Time `json:"cancel_at"`
		CancelBy    *int       `json:"cancel_by"`

		TotalSessions      int64 `json:"total_sessions"`
		CompletedSessions  int64 `json:"completed_sessions"`
		InProgressSessions int64 `json:"in_progress_sessions"`
		OpenSessions       int64 `json:"open_sessions"`
		CancelledSessions  int64 `json:"cancelled_sessions"`

		TotalLocations   int64 `json:"total_locations"`
		CountedLocations int64 `json:"counted_locations"`

		TotalSystemQty  int64 `json:"total_system_qty"`
		TotalCountedQty int64 `json:"total_counted_qty"`
		TotalDifference int64 `json:"total_difference"`
	}

	var result []StockTakeBatchListItem

	// =========================================================
	// MAIN QUERY
	//
	// Tidak menggunakan GROUP BY pada batch header.
	// =========================================================
	query := baseQuery.
		Select(`
			stock_take_batches.id,
			stock_take_batches.code,
			stock_take_batches.owner_code,
			stock_take_batches.description,
			stock_take_batches.status,
			stock_take_batches.created_by,
			stock_take_batches.updated_by,
			stock_take_batches.created_at,
			stock_take_batches.updated_at,
			stock_take_batches.started_at,
			stock_take_batches.closed_at,
			stock_take_batches.closed_by,
			stock_take_batches.cancel_at,
			stock_take_batches.cancel_by,

			-- =================================================
			-- SESSION SUMMARY
			-- =================================================

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = stock_take_batches.id
				  AND st.deleted_at IS NULL
			) AS total_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = stock_take_batches.id
				  AND st.status = 'closed'
				  AND st.deleted_at IS NULL
			) AS completed_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = stock_take_batches.id
				  AND st.status = 'in_progress'
				  AND st.deleted_at IS NULL
			) AS in_progress_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = stock_take_batches.id
				  AND st.status = 'open'
				  AND st.deleted_at IS NULL
			) AS open_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = stock_take_batches.id
				  AND st.status = 'cancelled'
				  AND st.deleted_at IS NULL
			) AS cancelled_sessions,


			-- =================================================
			-- TOTAL PLANNED LOCATIONS
			--
			-- Source:
			-- stock_take_items
			-- =================================================

			(
				SELECT COUNT(DISTINCT sti.location)
				FROM stock_take_items sti
				INNER JOIN stock_takes st
					ON st.id = sti.stock_take_id
				WHERE st.batch_id = stock_take_batches.id
				  AND st.deleted_at IS NULL
				  AND sti.deleted_at IS NULL
				  AND NULLIF(LTRIM(RTRIM(sti.location)), '') IS NOT NULL
			) AS total_locations,


			-- =================================================
			-- COUNTED LOCATIONS
			--
			-- Source:
			-- stock_take_barcodes
			--
			-- Sebuah lokasi dianggap sudah counted apabila
			-- sudah memiliki minimal 1 scan.
			-- =================================================

			(
				SELECT COUNT(DISTINCT stb.location)
				FROM stock_take_barcodes stb
				INNER JOIN stock_takes st
					ON st.id = stb.stock_take_id
				WHERE st.batch_id = stock_take_batches.id
				  AND st.deleted_at IS NULL
				  AND stb.deleted_at IS NULL
				  AND NULLIF(LTRIM(RTRIM(stb.location)), '') IS NOT NULL
			) AS counted_locations,


			-- =================================================
			-- TOTAL SYSTEM QTY
			--
			-- Source:
			-- stock_take_items
			-- =================================================

			(
				SELECT COALESCE(SUM(sti.system_qty), 0)
				FROM stock_take_items sti
				INNER JOIN stock_takes st
					ON st.id = sti.stock_take_id
				WHERE st.batch_id = stock_take_batches.id
				  AND st.deleted_at IS NULL
				  AND sti.deleted_at IS NULL
			) AS total_system_qty,


			-- =================================================
			-- TOTAL COUNTED QTY
			--
			-- Source:
			-- stock_take_barcodes
			-- =================================================

			(
				SELECT COALESCE(SUM(stb.counted_qty), 0)
				FROM stock_take_barcodes stb
				INNER JOIN stock_takes st
					ON st.id = stb.stock_take_id
				WHERE st.batch_id = stock_take_batches.id
				  AND st.deleted_at IS NULL
				  AND stb.deleted_at IS NULL
			) AS total_counted_qty,


			-- =================================================
			-- TOTAL DIFFERENCE
			--
			-- IMPORTANT:
			--
			-- Difference hanya dihitung untuk LOCATION
			-- yang sudah dilakukan counting.
			--
			-- Jadi lokasi yang belum dihitung TIDAK dianggap
			-- sebagai shortage.
			--
			-- Formula:
			--
			-- counted_qty - system_qty
			-- =================================================

			(
				SELECT COALESCE(SUM(
					x.counted_qty - x.system_qty
				), 0)
				FROM (
					SELECT
						sti.location,

						COALESCE(SUM(sti.system_qty), 0)
							AS system_qty,

						COALESCE((
							SELECT SUM(stb.counted_qty)
							FROM stock_take_barcodes stb
							WHERE stb.stock_take_id = sti.stock_take_id
							  AND stb.location = sti.location
							  AND stb.deleted_at IS NULL
						), 0) AS counted_qty

					FROM stock_take_items sti

					INNER JOIN stock_takes st
						ON st.id = sti.stock_take_id

					WHERE st.batch_id = stock_take_batches.id
					  AND st.deleted_at IS NULL
					  AND sti.deleted_at IS NULL

					GROUP BY
						sti.stock_take_id,
						sti.location

				) x

				WHERE EXISTS (
					SELECT 1
					FROM stock_take_barcodes stb2
					INNER JOIN stock_takes st2
						ON st2.id = stb2.stock_take_id
					WHERE st2.batch_id = stock_take_batches.id
					  AND stb2.stock_take_id = (
							SELECT TOP 1 sti2.stock_take_id
							FROM stock_take_items sti2
							WHERE sti2.location = x.location
							  AND sti2.deleted_at IS NULL
						)
					  AND stb2.location = x.location
					  AND stb2.deleted_at IS NULL
				)
			) AS total_difference
		`).
		Order("stock_take_batches.id DESC").
		Offset(offset).
		Limit(pageSize)

	if err := query.Scan(&result).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock take batches",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// PAGINATION
	// =========================================================

	totalPages := int64(0)

	if total > 0 {
		totalPages = (total + int64(pageSize) - 1) / int64(pageSize)
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    result,
		"meta": fiber.Map{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

type StockTakeBatchSummary struct {
	TotalSessions      int64 `json:"total_sessions"`
	CompletedSessions  int64 `json:"completed_sessions"`
	InProgressSessions int64 `json:"in_progress_sessions"`
	OpenSessions       int64 `json:"open_sessions"`
	CancelledSessions  int64 `json:"cancelled_sessions"`

	TotalLocations   int64 `json:"total_locations"`
	CountedLocations int64 `json:"counted_locations"`

	TotalSystemQty  int64 `json:"total_system_qty"`
	TotalCountedQty int64 `json:"total_counted_qty"`
	TotalDifference int64 `json:"total_difference"`
}

type StockTakeBatchSession struct {
	ID     uint   `json:"id"`
	Code   string `json:"code"`
	Status string `json:"status"`

	CreatedBy int `json:"created_by"`

	StartedAt *time.Time `json:"started_at"`
	ClosedAt  *time.Time `json:"closed_at"`

	TotalLocations   int64 `json:"total_locations"`
	CountedLocations int64 `json:"counted_locations"`

	TotalSystemQty  int64 `json:"total_system_qty"`
	TotalCountedQty int64 `json:"total_counted_qty"`
	TotalDifference int64 `json:"total_difference"`
}

func (c *StockTakeBatchController) GetStockTakeBatchSessions(ctx *fiber.Ctx) error {
	code := ctx.Params("code")

	var batch models.StockTakeBatch

	if err := c.DB.
		Where("code = ?", code).
		First(&batch).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Stock take batch not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock take batch",
			"error":   err.Error(),
		})
	}

	var sessions []StockTakeBatchSession

	query := `
		SELECT
			st.id,
			st.code,
			st.status,
			st.created_by,
			st.started_at,
			st.closed_at,

			-- Planned locations
			ISNULL((
				SELECT COUNT(DISTINCT sti.location)
				FROM stock_take_items sti
				WHERE sti.stock_take_id = st.id
				  AND sti.deleted_at IS NULL
			), 0) AS total_locations,

			-- Locations yang sudah dilakukan counting
			ISNULL((
				SELECT COUNT(DISTINCT stb.location)
				FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = st.id
				  AND stb.deleted_at IS NULL
				  AND NULLIF(LTRIM(RTRIM(stb.location)), '') IS NOT NULL
			), 0) AS counted_locations,

			-- System quantity
			ISNULL((
				SELECT SUM(sti.system_qty)
				FROM stock_take_items sti
				WHERE sti.stock_take_id = st.id
				  AND sti.deleted_at IS NULL
			), 0) AS total_system_qty,

			-- Actual counted quantity
			ISNULL((
				SELECT SUM(stb.counted_qty)
				FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = st.id
				  AND stb.deleted_at IS NULL
			), 0) AS total_counted_qty,

			-- Difference = counted - system
			ISNULL((
				SELECT SUM(stb.counted_qty)
				FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = st.id
				  AND stb.deleted_at IS NULL
			), 0)
			-
			ISNULL((
				SELECT SUM(sti.system_qty)
				FROM stock_take_items sti
				WHERE sti.stock_take_id = st.id
				  AND sti.deleted_at IS NULL
			), 0) AS total_difference

		FROM stock_takes st

		WHERE st.batch_id = ?
		  AND st.deleted_at IS NULL

		ORDER BY st.id ASC
	`

	if err := c.DB.Raw(query, batch.ID).Scan(&sessions).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock take sessions",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    sessions,
	})
}

func (c *StockTakeBatchController) GetStockTakeBatchDetail(ctx *fiber.Ctx) error {
	code := strings.TrimSpace(ctx.Params("code"))

	if code == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Batch code is required",
		})
	}

	// =========================================================
	// GET BATCH HEADER
	// =========================================================
	var batch models.StockTakeBatch

	if err := c.DB.
		Where("code = ?", code).
		Where("deleted_at IS NULL").
		First(&batch).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Stock take batch not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock take batch",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// SUMMARY
	// =========================================================
	type BatchSummary struct {
		TotalSessions      int64 `json:"total_sessions"`
		CompletedSessions  int64 `json:"completed_sessions"`
		InProgressSessions int64 `json:"in_progress_sessions"`
		OpenSessions       int64 `json:"open_sessions"`
		CancelledSessions  int64 `json:"cancelled_sessions"`

		TotalLocations   int64 `json:"total_locations"`
		CountedLocations int64 `json:"counted_locations"`

		TotalSystemQty  int64 `json:"total_system_qty"`
		TotalCountedQty int64 `json:"total_counted_qty"`
		TotalDifference int64 `json:"total_difference"`
	}

	var summary BatchSummary

	// =========================================================
	// SUMMARY QUERY
	//
	// stock_take_items:
	//   - planned/system data
	//
	// stock_take_barcodes:
	//   - actual/counting data
	// =========================================================
	summaryQuery := `
		SELECT

			-- =================================================
			-- SESSION SUMMARY
			-- =================================================

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = ?
				  AND st.deleted_at IS NULL
			) AS total_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = ?
				  AND st.status = 'closed'
				  AND st.deleted_at IS NULL
			) AS completed_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = ?
				  AND st.status = 'in_progress'
				  AND st.deleted_at IS NULL
			) AS in_progress_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = ?
				  AND st.status = 'open'
				  AND st.deleted_at IS NULL
			) AS open_sessions,

			(
				SELECT COUNT(*)
				FROM stock_takes st
				WHERE st.batch_id = ?
				  AND st.status = 'cancelled'
				  AND st.deleted_at IS NULL
			) AS cancelled_sessions,


			-- =================================================
			-- TOTAL PLANNED LOCATIONS
			-- From stock_take_items
			-- =================================================

			(
				SELECT COUNT(DISTINCT sti.location)
				FROM stock_take_items sti
				INNER JOIN stock_takes st
					ON st.id = sti.stock_take_id
				WHERE st.batch_id = ?
				  AND st.deleted_at IS NULL
				  AND sti.deleted_at IS NULL
				  AND NULLIF(LTRIM(RTRIM(sti.location)), '') IS NOT NULL
			) AS total_locations,


			-- =================================================
			-- COUNTED LOCATIONS
			-- From stock_take_barcodes
			-- =================================================

			(
				SELECT COUNT(DISTINCT stb.location)
				FROM stock_take_barcodes stb
				INNER JOIN stock_takes st
					ON st.id = stb.stock_take_id
				WHERE st.batch_id = ?
				  AND st.deleted_at IS NULL
				  AND stb.deleted_at IS NULL
				  AND NULLIF(LTRIM(RTRIM(stb.location)), '') IS NOT NULL
			) AS counted_locations,


			-- =================================================
			-- TOTAL SYSTEM QTY
			-- From stock_take_items
			-- =================================================

			(
				SELECT COALESCE(SUM(sti.system_qty), 0)
				FROM stock_take_items sti
				INNER JOIN stock_takes st
					ON st.id = sti.stock_take_id
				WHERE st.batch_id = ?
				  AND st.deleted_at IS NULL
				  AND sti.deleted_at IS NULL
			) AS total_system_qty,


			-- =================================================
			-- TOTAL COUNTED QTY
			-- From stock_take_barcodes
			-- =================================================

			(
				SELECT COALESCE(SUM(stb.counted_qty), 0)
				FROM stock_take_barcodes stb
				INNER JOIN stock_takes st
					ON st.id = stb.stock_take_id
				WHERE st.batch_id = ?
				  AND st.deleted_at IS NULL
				  AND stb.deleted_at IS NULL
			) AS total_counted_qty,


			-- =================================================
			-- TOTAL DIFFERENCE
			--
			-- Actual Count - System Qty
			-- =================================================

			(
				(
					SELECT COALESCE(SUM(stb.counted_qty), 0)
					FROM stock_take_barcodes stb
					INNER JOIN stock_takes st
						ON st.id = stb.stock_take_id
					WHERE st.batch_id = ?
					  AND st.deleted_at IS NULL
					  AND stb.deleted_at IS NULL
				)
				-
				(
					SELECT COALESCE(SUM(sti.system_qty), 0)
					FROM stock_take_items sti
					INNER JOIN stock_takes st
						ON st.id = sti.stock_take_id
					WHERE st.batch_id = ?
					  AND st.deleted_at IS NULL
					  AND sti.deleted_at IS NULL
				)
			) AS total_difference
	`

	args := []interface{}{
		// Sessions
		batch.ID,
		batch.ID,
		batch.ID,
		batch.ID,
		batch.ID,

		// Locations
		batch.ID,
		batch.ID,

		// Qty
		batch.ID,
		batch.ID,

		// Difference
		batch.ID,
		batch.ID,
	}

	if err := c.DB.Raw(summaryQuery, args...).Scan(&summary).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get stock take batch summary",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// RESPONSE
	// =========================================================

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"batch": fiber.Map{
				"ID":          batch.ID,
				"code":        batch.Code,
				"owner_code":  batch.OwnerCode,
				"description": batch.Description,
				"status":      batch.Status,

				"created_by": batch.CreatedBy,
				"updated_by": batch.UpdatedBy,

				"created_at": batch.CreatedAt,
				"updated_at": batch.UpdatedAt,

				"started_at": batch.StartedAt,
				"closed_at":  batch.ClosedAt,
				"closed_by":  batch.ClosedBy,

				"cancel_at": batch.CancelAt,
				"cancel_by": batch.CancelBy,

				"total_sessions":       summary.TotalSessions,
				"completed_sessions":   summary.CompletedSessions,
				"in_progress_sessions": summary.InProgressSessions,
				"open_sessions":        summary.OpenSessions,
				"cancelled_sessions":   summary.CancelledSessions,

				"total_locations":   summary.TotalLocations,
				"counted_locations": summary.CountedLocations,

				"total_system_qty":  summary.TotalSystemQty,
				"total_counted_qty": summary.TotalCountedQty,
				"total_difference":  summary.TotalDifference,
			},
		},
	})
}

func (c *StockTakeBatchController) CompleteStockTakeBatch(ctx *fiber.Ctx) error {
	code := strings.TrimSpace(ctx.Params("code"))

	if code == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Batch code is required",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	var batch models.StockTakeBatch

	err := c.DB.Transaction(func(tx *gorm.DB) error {

		// =====================================================
		// GET BATCH + LOCK
		// =====================================================

		if err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
			}).
			Where("code = ?", code).
			Where("deleted_at IS NULL").
			First(&batch).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(
					fiber.StatusNotFound,
					"Stock take batch not found",
				)
			}

			return fmt.Errorf("failed to get stock take batch: %w", err)
		}

		// =====================================================
		// VALIDATE BATCH STATUS
		// =====================================================

		if batch.Status == "completed" {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Stock take batch is already completed",
			)
		}

		if batch.Status == "cancelled" {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Cannot complete a cancelled stock take batch",
			)
		}

		// =====================================================
		// GET SESSION SUMMARY
		// =====================================================

		type SessionStatusCount struct {
			Total      int64 `json:"total"`
			Open       int64 `json:"open"`
			InProgress int64 `json:"in_progress"`
			Closed     int64 `json:"closed"`
			Cancelled  int64 `json:"cancelled"`
		}

		var sessionSummary SessionStatusCount

		if err := tx.
			Model(&models.StockTake{}).
			Select(`
				COUNT(*) AS total,

				SUM(
					CASE
						WHEN status = 'open' THEN 1
						ELSE 0
					END
				) AS open,

				SUM(
					CASE
						WHEN status = 'in_progress' THEN 1
						ELSE 0
					END
				) AS in_progress,

				SUM(
					CASE
						WHEN status = 'closed' THEN 1
						ELSE 0
					END
				) AS closed,

				SUM(
					CASE
						WHEN status = 'cancelled' THEN 1
						ELSE 0
					END
				) AS cancelled
			`).
			Where("batch_id = ?", batch.ID).
			Where("deleted_at IS NULL").
			Scan(&sessionSummary).Error; err != nil {

			return fmt.Errorf(
				"failed to get session summary: %w",
				err,
			)
		}

		// =====================================================
		// MUST HAVE AT LEAST ONE SESSION
		// =====================================================

		if sessionSummary.Total == 0 {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Cannot complete batch without any stock take session",
			)
		}

		// =====================================================
		// ALL SESSION MUST BE CLOSED
		// =====================================================

		if sessionSummary.Open > 0 ||
			sessionSummary.InProgress > 0 ||
			sessionSummary.Cancelled > 0 {

			return fiber.NewError(
				fiber.StatusBadRequest,
				fmt.Sprintf(
					"Cannot complete batch. %d session(s) are still open, in progress, or cancelled",
					sessionSummary.Open+
						sessionSummary.InProgress+
						sessionSummary.Cancelled,
				),
			)
		}

		// =====================================================
		// SAFETY CHECK
		// =====================================================

		if sessionSummary.Closed != sessionSummary.Total {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Cannot complete batch. Not all sessions are closed",
			)
		}

		// =====================================================
		// COMPLETE BATCH
		// =====================================================

		now := time.Now()

		if err := tx.
			Model(&batch).
			Updates(map[string]interface{}{
				"status":     "completed",
				"closed_at":  now,
				"closed_by":  userID,
				"updated_at": now,
				"updated_by": userID,
			}).Error; err != nil {

			return fmt.Errorf(
				"failed to complete stock take batch: %w",
				err,
			)
		}

		return nil
	})

	// =========================================================
	// HANDLE ERROR
	// =========================================================

	if err != nil {

		if fiberErr, ok := err.(*fiber.Error); ok {
			return ctx.Status(fiberErr.Code).JSON(fiber.Map{
				"success": false,
				"message": fiberErr.Message,
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to complete stock take batch",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// RESPONSE
	// =========================================================

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Stock take batch completed successfully",
		"data": fiber.Map{
			"code":   batch.Code,
			"status": "completed",
		},
	})
}

func (c *StockTakeBatchController) CancelStockTakeBatch(ctx *fiber.Ctx) error {
	code := strings.TrimSpace(ctx.Params("code"))

	if code == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Batch code is required",
		})
	}

	userID := int(ctx.Locals("userID").(float64))

	err := c.DB.Transaction(func(tx *gorm.DB) error {

		// =====================================================
		// GET BATCH
		// =====================================================

		var batch models.StockTakeBatch

		if err := tx.
			Where("code = ?", code).
			Where("deleted_at IS NULL").
			First(&batch).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(
					fiber.StatusNotFound,
					"Stock take batch not found",
				)
			}

			return fmt.Errorf(
				"failed to get stock take batch: %w",
				err,
			)
		}

		// =====================================================
		// VALIDATE BATCH STATUS
		// =====================================================

		if batch.Status == "completed" {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Cannot cancel a completed stock take batch",
			)
		}

		if batch.Status == "cancelled" {
			return fiber.NewError(
				fiber.StatusBadRequest,
				"Stock take batch is already cancelled",
			)
		}

		// =====================================================
		// CANCEL ACTIVE SESSIONS
		//
		// Closed session tetap closed karena merupakan
		// historical result.
		//
		// Yang di-cancel:
		// - open
		// - in_progress
		// =====================================================

		now := time.Now()

		if err := tx.
			Model(&models.StockTake{}).
			Where("batch_id = ?", batch.ID).
			Where("deleted_at IS NULL").
			Where("status IN ?", []string{
				"open",
				"in_progress",
			}).
			Updates(map[string]interface{}{
				"status":     "cancelled",
				"cancel_at":  now,
				"cancel_by":  userID,
				"updated_at": now,
				"updated_by": userID,
			}).Error; err != nil {

			return fmt.Errorf(
				"failed to cancel stock take sessions: %w",
				err,
			)
		}

		// =====================================================
		// CANCEL BATCH
		// =====================================================

		if err := tx.
			Model(&batch).
			Updates(map[string]interface{}{
				"status":     "cancelled",
				"cancel_at":  now,
				"cancel_by":  userID,
				"updated_at": now,
				"updated_by": userID,
			}).Error; err != nil {

			return fmt.Errorf(
				"failed to cancel stock take batch: %w",
				err,
			)
		}

		return nil
	})

	// =========================================================
	// HANDLE ERROR
	// =========================================================

	if err != nil {

		if fiberErr, ok := err.(*fiber.Error); ok {
			return ctx.Status(fiberErr.Code).JSON(fiber.Map{
				"success": false,
				"message": fiberErr.Message,
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to cancel stock take batch",
			"error":   err.Error(),
		})
	}

	// =========================================================
	// RESPONSE
	// =========================================================

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Stock take batch cancelled successfully",
	})
}

func (c *StockTakeBatchController) GetStockTakeBatchLocations(ctx *fiber.Ctx) error {
	batchCode := strings.TrimSpace(ctx.Params("code"))
	search := strings.TrimSpace(ctx.Query("search"))

	if batchCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Batch code is required",
		})
	}

	if search == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Search location is required",
		})
	}

	type LocationLookup struct {
		Location       string `json:"location"`
		SessionCode    string `json:"session_code"`
		SessionStatus  string `json:"session_status"`
		TotalSystemQty int64  `json:"total_system_qty"`
		CountedQty     int64  `json:"counted_qty"`
		Counted        bool   `json:"counted"`
	}

	query := `
		SELECT
			sti.location,
			st.code AS session_code,
			st.status AS session_status,
			SUM(sti.system_qty) AS total_system_qty,
			ISNULL((
				SELECT SUM(stb.counted_qty)
				FROM stock_take_barcodes stb
				WHERE stb.stock_take_id = st.id
				  AND UPPER(LTRIM(RTRIM(stb.location))) =
				      UPPER(LTRIM(RTRIM(sti.location)))
				  AND stb.deleted_at IS NULL
			), 0) AS counted_qty,
			CASE WHEN EXISTS (
				SELECT 1
				FROM stock_take_barcodes stb2
				WHERE stb2.stock_take_id = st.id
				  AND UPPER(LTRIM(RTRIM(stb2.location))) =
				      UPPER(LTRIM(RTRIM(sti.location)))
				  AND stb2.deleted_at IS NULL
			) THEN CAST(1 AS bit) ELSE CAST(0 AS bit) END AS counted
		FROM stock_take_items sti
		INNER JOIN stock_takes st
			ON st.id = sti.stock_take_id
		INNER JOIN stock_take_batches stbch
			ON stbch.id = st.batch_id
		WHERE stbch.code = ?
		  AND stbch.deleted_at IS NULL
		  AND st.deleted_at IS NULL
		  AND sti.deleted_at IS NULL
		  AND UPPER(LTRIM(RTRIM(sti.location))) LIKE UPPER(?)
		GROUP BY
			sti.location,
			st.id,
			st.code,
			st.status
		ORDER BY sti.location ASC, st.id ASC
	`

	likeSearch := "%" + search + "%"

	var rows []LocationLookup
	if err := c.DB.Raw(query, batchCode, likeSearch).Scan(&rows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to find stock take locations",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    rows,
	})
}
