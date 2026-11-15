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

type CreatePrintLogRequest struct {
	DocumentType string `json:"document_type"`
	DocumentNo   string `json:"document_no"`
	Module       string `json:"module"`

	PrintMethod string `json:"print_method"`
	PrintFormat string `json:"print_format"`

	CopyCount   int    `json:"copy_count"`
	PrinterName string `json:"printer_name"`
	Reason      string `json:"reason"`
	Workstation string `json:"workstation"`
}

type CreatePrintLogParams struct {
	Request   CreatePrintLogRequest
	UserID    uint
	IPAddress string
}

func NewPrintService(db *gorm.DB) *PrintService {
	return &PrintService{
		DB: db,
	}
}

func (s *PrintService) CreateLog(
	params CreatePrintLogParams,
) (*models.PrintLog, error) {

	if s.DB == nil {
		return nil, errors.New("database connection is not initialized")
	}

	req := params.Request

	// Normalize input
	req.DocumentType = strings.ToUpper(strings.TrimSpace(req.DocumentType))
	req.DocumentNo = strings.TrimSpace(req.DocumentNo)
	req.Module = strings.ToUpper(strings.TrimSpace(req.Module))
	req.PrintMethod = strings.ToUpper(strings.TrimSpace(req.PrintMethod))
	req.PrintFormat = strings.ToUpper(strings.TrimSpace(req.PrintFormat))
	req.PrinterName = strings.TrimSpace(req.PrinterName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Workstation = strings.TrimSpace(req.Workstation)

	// Validate required fields
	if req.DocumentType == "" {
		return nil, errors.New("document_type is required")
	}

	if req.DocumentNo == "" {
		return nil, errors.New("document_no is required")
	}

	if req.Module == "" {
		return nil, errors.New("module is required")
	}

	if params.UserID == 0 {
		return nil, errors.New("invalid user")
	}

	if req.PrintMethod != "BROWSER" &&
		req.PrintMethod != "AGENT" {
		return nil, errors.New("invalid print_method")
	}

	if req.CopyCount <= 0 {
		req.CopyCount = 1
	}

	if req.CopyCount > 100 {
		return nil, errors.New("copy_count cannot exceed 100")
	}

	if req.PrintFormat == "" {
		req.PrintFormat = "PDF"
	}

	if req.PrintMethod == "AGENT" && req.PrinterName == "" {
		return nil, errors.New("printer_name is required for agent printing")
	}

	var printLog models.PrintLog

	err := s.DB.Transaction(
		func(tx *gorm.DB) error {

			var count int64

			/*
				SQL Server locking:

				UPDLOCK:
				Requests update locks instead of shared locks.

				HOLDLOCK:
				Keep locks until transaction completes.
				Equivalent to SERIALIZABLE behavior for this table reference.

				Requires a suitable index on:
				document_type, document_no
			*/

			query := tx.Model(&models.PrintLog{}).
				Clauses(
				// Intentionally no GORM clause.Locking here.
				).
				Where(
					"document_type = ? AND document_no = ?",
					req.DocumentType,
					req.DocumentNo,
				)

			if err := query.
				Count(&count).Error; err != nil {
				return err
			}

			action := "ORIGINAL"

			if count > 0 {
				action = "REPRINT"

				// if req.Reason == "" {
				// 	return errors.New("reprint reason is required")
				// }
			}

			printLog = models.PrintLog{
				DocumentType: req.DocumentType,
				DocumentNo:   req.DocumentNo,
				Module:       req.Module,

				PrintAction: action,
				PrintMethod: req.PrintMethod,
				PrintFormat: req.PrintFormat,

				CopyCount:   req.CopyCount,
				PrinterName: req.PrinterName,

				Status: "REQUESTED",
				Reason: req.Reason,

				RequestedBy: params.UserID,
				RequestedAt: time.Now(),

				IPAddress:   params.IPAddress,
				Workstation: req.Workstation,
			}

			if err := tx.Create(&printLog).Error; err != nil {
				return err
			}

			return nil
		},
		&sql.TxOptions{
			Isolation: sql.LevelSerializable,
		},
	)

	if err != nil {
		return nil, err
	}

	return &printLog, nil
}
