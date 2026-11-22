package services

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"fiber-app/models"

	"gorm.io/gorm"
)

type PrintService struct {
	DB *gorm.DB
}

func NewPrintService(db *gorm.DB) *PrintService {
	return &PrintService{DB: db}
}

// ============================================================
// REQUEST & RESPONSE STRUCTS
// ============================================================

type CreatePrintLogRequest struct {
	DocumentType string `json:"document_type"`
	DocumentNo   string `json:"document_no"`
	Module       string `json:"module"`
	PrintMethod  string `json:"print_method"`
	PrintFormat  string `json:"print_format"`
	CopyCount    int    `json:"copy_count"`
	PrinterName  string `json:"printer_name"`
	Workstation  string `json:"workstation_name"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

type CreatePrintLogParams struct {
	Request   CreatePrintLogRequest
	UserID    uint
	IPAddress string
}

type PrintLogFilter struct {
	Page         int
	PageSize     int
	Search       string
	Module       string
	DocumentType string
	PrintAction  string
	Status       string
	DateFrom     string
	DateTo       string
}

type PrintLogSummary struct {
	Total     int64 `json:"total"`
	Original  int64 `json:"original"`
	Reprint   int64 `json:"reprint"`
	Requested int64 `json:"requested"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

type PrintLogPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type PrintLogListResult struct {
	Data       []models.PrintLog  `json:"data"`
	Summary    PrintLogSummary    `json:"summary"`
	Pagination PrintLogPagination `json:"pagination"`
}

// ============================================================
// CREATE PRINT LOG
// ============================================================

func (s *PrintService) CreateLog(
	params CreatePrintLogParams,
) (*models.PrintLog, error) {

	req := params.Request

	req.DocumentType = strings.ToUpper(strings.TrimSpace(req.DocumentType))
	req.DocumentNo = strings.TrimSpace(req.DocumentNo)
	req.Module = strings.ToUpper(strings.TrimSpace(req.Module))
	req.PrintMethod = strings.ToUpper(strings.TrimSpace(req.PrintMethod))
	req.PrintFormat = strings.ToUpper(strings.TrimSpace(req.PrintFormat))
	req.PrinterName = strings.TrimSpace(req.PrinterName)
	req.Workstation = strings.TrimSpace(req.Workstation)

	if params.UserID == 0 {
		return nil, errors.New("Authenticated user is required")
	}

	// if req.DocumentType == "" || req.DocumentNo == "" || req.Module == "" {
	// 	return nil, errors.New("Document type, document number, and module are required")
	// }

	if req.PrintMethod == "" {
		req.PrintMethod = "BROWSER"
	}

	if req.PrintFormat == "" {
		req.PrintFormat = "PDF"
	}

	if req.CopyCount <= 0 {
		req.CopyCount = 1
	}

	if req.CopyCount > 500 {
		return nil, errors.New("Copy count cannot exceed 500")
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))

	if status == "" {
		status = "REQUESTED"
	}

	if status != "REQUESTED" &&
		status != "COMPLETED" &&
		status != "FAILED" {
		return nil, errors.New("Invalid print status")
	}

	// Browser print dialogs cannot confirm physical print completion.
	if req.PrintMethod == "BROWSER" && status != "REQUESTED" {
		return nil, errors.New(
			"Browser print status must remain REQUESTED",
		)
	}

	now := time.Now()

	log := models.PrintLog{
		DocumentType: req.DocumentType,
		DocumentNo:   req.DocumentNo,
		Module:       req.Module,

		PrintMethod: req.PrintMethod,
		PrintFormat: req.PrintFormat,

		CopyCount:   req.CopyCount,
		PrinterName: req.PrinterName,

		Status:       status,
		Reason:       strings.TrimSpace(req.Reason),
		ErrorMessage: strings.TrimSpace(req.ErrorMessage),

		RequestedBy: params.UserID,
		RequestedAt: now,

		IPAddress:   params.IPAddress,
		Workstation: req.Workstation,
	}

	if status == "COMPLETED" {
		log.CompletedAt = &now
	}

	// ========================================================
	// DETERMINE ORIGINAL OR REPRINT
	// ========================================================

	err := s.DB.Transaction(func(tx *gorm.DB) error {

		var previousCount int64

		// SQL Server locking:
		// UPDLOCK + HOLDLOCK prevents concurrent requests from
		// incorrectly classifying the same document as ORIGINAL.
		//
		// Keep the composite index on:
		// document_type + document_no

		err := tx.Raw(`
			SELECT COUNT(*)
			FROM print_logs WITH (UPDLOCK, HOLDLOCK)
			WHERE document_type = ?
			  AND document_no = ?
		`,
			req.DocumentType,
			req.DocumentNo,
		).Scan(&previousCount).Error

		if err != nil {
			return err
		}

		if previousCount == 0 {
			log.PrintAction = "ORIGINAL"
		} else {
			log.PrintAction = "REPRINT"
		}

		return tx.Create(&log).Error

	}, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})

	if err != nil {
		return nil, err
	}

	// ========================================================
	// GET REQUESTED USER NAME
	// ========================================================

	err = s.DB.Table("users").
		Select("name").
		Where("id = ?", log.RequestedBy).
		Scan(&log.RequestedByName).Error

	if err != nil {
		return nil, err
	}

	return &log, nil
}

// ============================================================
// GET PRINT LOGS
// ============================================================

func (s *PrintService) GetLogs(
	filter PrintLogFilter,
) (*PrintLogListResult, error) {

	if filter.Page < 1 {
		filter.Page = 1
	}

	if filter.PageSize < 1 {
		filter.PageSize = 20
	}

	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	// LEFT JOIN ensures historical print logs remain visible
	// even if the corresponding user record is unavailable.

	query := s.DB.
		Table("print_logs").
		Joins(`
			LEFT JOIN users
				ON users.id = print_logs.requested_by
		`)

	// ========================================================
	// SEARCH
	// ========================================================

	if search := strings.TrimSpace(filter.Search); search != "" {

		like := "%" + search + "%"

		query = query.Where(`
			(
				print_logs.document_no LIKE ?
				OR print_logs.document_type LIKE ?
				OR CAST(print_logs.requested_by AS varchar(30)) LIKE ?
				OR users.name LIKE ?
				OR print_logs.printer_name LIKE ?
				OR print_logs.workstation LIKE ?
			)
		`,
			like,
			like,
			like,
			like,
			like,
			like,
		)
	}

	// ========================================================
	// FILTERS
	// ========================================================

	if filter.Module != "" {
		query = query.Where(
			"print_logs.module = ?",
			strings.ToUpper(strings.TrimSpace(filter.Module)),
		)
	}

	if filter.DocumentType != "" {
		query = query.Where(
			"print_logs.document_type = ?",
			strings.ToUpper(strings.TrimSpace(filter.DocumentType)),
		)
	}

	if filter.PrintAction != "" {
		query = query.Where(
			"print_logs.print_action = ?",
			strings.ToUpper(strings.TrimSpace(filter.PrintAction)),
		)
	}

	if filter.Status != "" {
		query = query.Where(
			"print_logs.status = ?",
			strings.ToUpper(strings.TrimSpace(filter.Status)),
		)
	}

	if filter.DateFrom != "" {
		query = query.Where(
			"print_logs.requested_at >= ?",
			filter.DateFrom+" 00:00:00",
		)
	}

	if filter.DateTo != "" {
		query = query.Where(
			"print_logs.requested_at < DATEADD(day, 1, ?)",
			filter.DateTo,
		)
	}

	// ========================================================
	// TOTAL RECORDS
	// ========================================================

	var total int64

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// ========================================================
	// PAGINATION DATA
	// ========================================================

	var rows []models.PrintLog

	offset := (filter.Page - 1) * filter.PageSize

	err := query.
		Select(`
			print_logs.*,
			users.name AS requested_by_name
		`).
		Order("print_logs.requested_at DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	// ========================================================
	// SUMMARY
	// ========================================================

	summaryQuery := s.DB.Model(&models.PrintLog{})

	if filter.DateFrom != "" {
		summaryQuery = summaryQuery.Where(
			"requested_at >= ?",
			filter.DateFrom+" 00:00:00",
		)
	}

	if filter.DateTo != "" {
		summaryQuery = summaryQuery.Where(
			"requested_at < DATEADD(day, 1, ?)",
			filter.DateTo,
		)
	}

	var summary PrintLogSummary

	err = summaryQuery.Select(`
		COUNT(*) AS total,

		COALESCE(
			SUM(CASE WHEN print_action = 'ORIGINAL' THEN 1 ELSE 0 END),
			0
		) AS original,

		COALESCE(
			SUM(CASE WHEN print_action = 'REPRINT' THEN 1 ELSE 0 END),
			0
		) AS reprint,

		COALESCE(
			SUM(CASE WHEN status = 'REQUESTED' THEN 1 ELSE 0 END),
			0
		) AS requested,

		COALESCE(
			SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END),
			0
		) AS completed,

		COALESCE(
			SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END),
			0
		) AS failed
	`).Scan(&summary).Error

	if err != nil {
		return nil, err
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(filter.PageSize) - 1) /
				int64(filter.PageSize),
		)
	}

	return &PrintLogListResult{
		Data:    rows,
		Summary: summary,
		Pagination: PrintLogPagination{
			Page:       filter.Page,
			PageSize:   filter.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// ============================================================
// GET PRINT LOG BY ID
// ============================================================

func (s *PrintService) GetLogByID(
	id uint,
) (*models.PrintLog, error) {

	var item models.PrintLog

	err := s.DB.
		Table("print_logs").
		Select(`
			print_logs.*,
			users.name AS requested_by_name
		`).
		Joins(`
			LEFT JOIN users
				ON users.id = print_logs.requested_by
		`).
		Where("print_logs.id = ?", id).
		Scan(&item).Error

	if err != nil {
		return nil, err
	}

	if item.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &item, nil
}
