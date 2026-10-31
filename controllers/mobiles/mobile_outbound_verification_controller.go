package mobiles

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobileOutboundVerificationController struct {
	DB *gorm.DB
}

// ============================================================
// Response DTOs
// ============================================================

type OutboundVerificationSummaryDTO struct {
	OrderID       uint   `json:"order_id"`
	OrderNo       string `json:"order_no"`
	Status        string `json:"status"`
	TotalExpected int64  `json:"total_expected"`
	TotalScanned  int64  `json:"total_scanned"`
	TotalPending  int64  `json:"total_pending"`
	IsComplete    bool   `json:"is_complete"`
}

type OutboundVerificationItemDTO struct {
	OrderDetailID uint       `json:"order_detail_id"`
	ShipmentID    string     `json:"shipment_id"`
	OutboundID    string     `json:"outbound_id"`
	OutboundNo    string     `json:"outbound_no"`
	QtyKoli       int        `json:"qty_koli"`
	TotalItem     int        `json:"total_item"`
	TotalQty      int        `json:"total_qty"`
	DelivTo       string     `json:"deliv_to"`
	DelivToName   string     `json:"deliv_to_name"`
	IsScanned     bool       `json:"is_scanned"`
	ScannedBy     *string    `json:"scanned_by"`
	ScannedAt     *time.Time `json:"scanned_at"`
}

type OutboundVerificationDetailDTO struct {
	Summary OutboundVerificationSummaryDTO `json:"summary"`
	Items   []OutboundVerificationItemDTO  `json:"items"`
}

type OutboundVerificationHistoryDTO struct {
	ID            uint      `json:"ID"`
	OrderNo       string    `json:"order_no"`
	ShipmentID    string    `json:"shipment_id"`
	OrderDetailID uint      `json:"order_detail_id"`
	ScannedBy     string    `json:"scanned_by"`
	ScannedAt     time.Time `json:"scanned_at"`
	Status        string    `json:"status"`
	Remarks       string    `json:"remarks"`
}

type outboundVerificationScanWithUsername struct {
	models.OrderShipmentScan
	ScannedByUsername *string `gorm:"column:scanned_by_username"`
}

func NewMobileOutboundVerificationController(
	DB *gorm.DB,
) *MobileOutboundVerificationController {
	return &MobileOutboundVerificationController{
		DB: DB,
	}
}

// ============================================================
// Response Helpers
// ============================================================

func verificationError(
	ctx *fiber.Ctx,
	status int,
	message string,
) error {
	return ctx.Status(status).JSON(fiber.Map{
		"success": false,
		"message": message,
	})
}

func verificationSuccess(
	ctx *fiber.Ctx,
	message string,
	data interface{},
) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// ============================================================
// User Context
// ============================================================

// Adjust this helper to match the authentication middleware
// used in your existing application.
func getVerificationUserID(ctx *fiber.Ctx) (int, error) {

	value := ctx.Locals("user_id")

	if value == nil {
		value = ctx.Locals("userID")
	}

	switch v := value.(type) {

	case int:
		if v > 0 {
			return v, nil
		}

	case int64:
		if v > 0 {
			return int(v), nil
		}

	case float64:
		if v > 0 {
			return int(v), nil
		}

	case string:
		id, err := strconv.Atoi(v)

		if err == nil && id > 0 {
			return id, nil
		}
	}

	return 0, errors.New("authenticated user ID not found")
}

// ============================================================
// Helpers
// ============================================================

func (c *MobileOutboundVerificationController) getOrderHeader(
	tx *gorm.DB,
	orderNo string,
) (*models.OrderHeader, error) {

	var order models.OrderHeader

	err := tx.
		Where("order_no = ?", orderNo).
		First(&order).Error

	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (c *MobileOutboundVerificationController) getOrderDetails(
	tx *gorm.DB,
	orderID uint,
) ([]models.OrderDetail, error) {

	var details []models.OrderDetail

	err := tx.
		Where("order_id = ?", orderID).
		Where("shipment_id <> ''").
		Order("id ASC").
		Find(&details).Error

	return details, err
}

// Validate that each Shipment ID uniquely identifies
// exactly one OrderDetail within the selected SPK.
func validateUniqueShipmentIDs(
	details []models.OrderDetail,
) error {

	seen := make(map[string]uint)

	for _, detail := range details {

		shipmentID := strings.TrimSpace(detail.ShipmentID)

		if shipmentID == "" {
			return fmt.Errorf(
				"order detail ID %d has an empty Shipment ID",
				detail.ID,
			)
		}

		if existingID, exists := seen[shipmentID]; exists {

			return fmt.Errorf(
				"duplicate Shipment ID %s found in Order Details (%d and %d)",
				shipmentID,
				existingID,
				detail.ID,
			)
		}

		seen[shipmentID] = detail.ID
	}

	return nil
}

// ============================================================
// 1. GET VERIFICATION DETAILS
// GET /verification/:orderNo
// ============================================================

func (c *MobileOutboundVerificationController) GetVerification(
	ctx *fiber.Ctx,
) error {

	orderNo := strings.TrimSpace(ctx.Params("orderNo"))

	if orderNo == "" {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Order number is required",
		)
	}

	var response OutboundVerificationDetailDTO

	err := c.DB.Transaction(func(tx *gorm.DB) error {

		order, err := c.getOrderHeader(tx, orderNo)

		if err != nil {
			return err
		}

		details, err := c.getOrderDetails(tx, order.ID)

		if err != nil {
			return err
		}

		if err := validateUniqueShipmentIDs(details); err != nil {
			return err
		}

		var scans []outboundVerificationScanWithUsername

		if err := tx.
			Model(&models.OrderShipmentScan{}).
			Select("order_shipment_scans.*, users.username AS scanned_by_username").
			Joins("LEFT JOIN users ON users.id = order_shipment_scans.scanned_by").
			Where("order_shipment_scans.order_id = ?", order.ID).
			Find(&scans).Error; err != nil {
			return err
		}

		scanMap := make(map[uint]outboundVerificationScanWithUsername)

		for _, scan := range scans {
			scanMap[scan.OrderDetailID] = scan
		}

		items := make([]OutboundVerificationItemDTO, 0, len(details))

		for _, detail := range details {

			item := OutboundVerificationItemDTO{
				OrderDetailID: detail.ID,
				ShipmentID:    detail.ShipmentID,
				OutboundID:    fmt.Sprint(detail.OutboundID),
				OutboundNo:    detail.OutboundNo,

				QtyKoli:   detail.QtyKoli,
				TotalItem: detail.TotalItem,
				TotalQty:  detail.TotalQty,

				DelivTo:     detail.DelivTo,
				DelivToName: detail.DelivToName,

				IsScanned: false,
			}

			if scan, exists := scanMap[detail.ID]; exists {

				item.IsScanned = true
				item.ScannedBy = scan.ScannedByUsername
				item.ScannedAt = &scan.ScannedAt
			}

			items = append(items, item)
		}

		totalExpected := int64(len(details))
		totalScanned := int64(len(scans))
		totalPending := totalExpected - totalScanned

		response = OutboundVerificationDetailDTO{

			Summary: OutboundVerificationSummaryDTO{
				OrderID:       order.ID,
				OrderNo:       order.OrderNo,
				Status:        order.Status,
				TotalExpected: totalExpected,
				TotalScanned:  totalScanned,
				TotalPending:  totalPending,
				IsComplete:    totalExpected > 0 && totalPending == 0,
			},

			Items: items,
		}

		return nil

	})

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return verificationError(
				ctx,
				fiber.StatusNotFound,
				"SPK not found",
			)
		}

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			err.Error(),
		)
	}

	return verificationSuccess(
		ctx,
		"Verification details retrieved successfully",
		response,
	)
}

// ============================================================
// 2. SCAN SHIPMENT ID
// POST /verification/:orderNo/scan
// ============================================================

func (c *MobileOutboundVerificationController) ScanShipment(
	ctx *fiber.Ctx,
) error {

	orderNo := strings.TrimSpace(ctx.Params("orderNo"))

	if orderNo == "" {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Order number is required",
		)
	}

	var request ScanShipmentRequest

	if err := ctx.BodyParser(&request); err != nil {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Invalid request payload",
		)
	}

	shipmentID := strings.TrimSpace(request.ShipmentID)

	if shipmentID == "" {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Shipment ID is required",
		)
	}

	userID, err := getVerificationUserID(ctx)

	if err != nil {
		return verificationError(
			ctx,
			fiber.StatusUnauthorized,
			"Unable to identify authenticated user",
		)
	}

	now := time.Now()

	// Serializable transaction protects concurrent
	// scan and confirmation operations.
	tx := c.DB.Begin(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})

	if tx.Error != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Unable to start verification transaction",
		)
	}

	defer tx.Rollback()

	// --------------------------------------------------------
	// Validate SPK
	// --------------------------------------------------------

	order, err := c.getOrderHeader(tx, orderNo)

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return verificationError(
				ctx,
				fiber.StatusNotFound,
				"SPK not found",
			)
		}

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve SPK",
		)
	}

	if !strings.EqualFold(order.Status, "open") {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"SPK is not available for verification",
		)
	}

	// --------------------------------------------------------
	// Validate Shipment ID
	// --------------------------------------------------------

	var matchingDetails []models.OrderDetail

	if err := tx.
		Where("order_id = ?", order.ID).
		Where("shipment_id = ?", shipmentID).
		Find(&matchingDetails).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to validate Shipment ID",
		)
	}

	if len(matchingDetails) == 0 {
		return verificationError(
			ctx,
			fiber.StatusNotFound,
			"Shipment ID does not belong to this SPK",
		)
	}

	if len(matchingDetails) > 1 {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"Shipment ID is ambiguous within this SPK",
		)
	}

	detail := matchingDetails[0]

	// --------------------------------------------------------
	// Check Duplicate Scan
	// --------------------------------------------------------

	var existingScan models.OrderShipmentScan

	err = tx.
		Where("order_id = ?", order.ID).
		Where("order_detail_id = ?", detail.ID).
		First(&existingScan).Error

	if err == nil {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"Shipment ID has already been scanned",
		)
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to check scan history",
		)
	}

	// --------------------------------------------------------
	// Insert Scan Record
	// --------------------------------------------------------

	scan := models.OrderShipmentScan{

		OrderID:       order.ID,
		OrderNo:       order.OrderNo,
		OrderDetailID: detail.ID,

		ShipmentID: detail.ShipmentID,

		ScannedBy: userID,
		ScannedAt: now,

		Status:  "verified",
		Remarks: request.Remarks,
	}

	if err := tx.Create(&scan).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"Shipment scan could not be recorded. It may have already been scanned",
		)
	}

	var scannedByUsername string
	if err := tx.Table("users").
		Select("username").
		Where("id = ?", userID).
		Scan(&scannedByUsername).Error; err != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve scanner username",
		)
	}

	// --------------------------------------------------------
	// Calculate Progress
	// --------------------------------------------------------

	var totalExpected int64
	var totalScanned int64

	if err := tx.Model(&models.OrderDetail{}).
		Where("order_id = ?", order.ID).
		Where("shipment_id <> ''").
		Count(&totalExpected).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to calculate expected shipments",
		)
	}

	if err := tx.Model(&models.OrderShipmentScan{}).
		Where("order_id = ?", order.ID).
		Count(&totalScanned).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to calculate scanned shipments",
		)
	}

	if err := tx.Commit().Error; err != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to commit shipment scan",
		)
	}

	totalPending := totalExpected - totalScanned

	return verificationSuccess(
		ctx,
		"Shipment verified successfully",
		fiber.Map{

			"shipment_id": shipmentID,

			"summary": OutboundVerificationSummaryDTO{
				OrderID:       order.ID,
				OrderNo:       order.OrderNo,
				Status:        order.Status,
				TotalExpected: totalExpected,
				TotalScanned:  totalScanned,
				TotalPending:  totalPending,
				IsComplete:    totalExpected > 0 && totalPending == 0,
			},

			"scan": fiber.Map{
				"id":          scan.ID,
				"shipment_id": scan.ShipmentID,
				"scanned_by":  scannedByUsername,
				"scanned_at":  scan.ScannedAt,
			},
		},
	)
}

// ============================================================
// 3. GET SCAN HISTORY
// GET /verification/:orderNo/history
// ============================================================

func (c *MobileOutboundVerificationController) GetHistory(
	ctx *fiber.Ctx,
) error {

	orderNo := strings.TrimSpace(ctx.Params("orderNo"))

	if orderNo == "" {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Order number is required",
		)
	}

	order, err := c.getOrderHeader(c.DB, orderNo)

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return verificationError(
				ctx,
				fiber.StatusNotFound,
				"SPK not found",
			)
		}

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve SPK",
		)
	}

	var history []outboundVerificationScanWithUsername

	if err := c.DB.
		Model(&models.OrderShipmentScan{}).
		Select("order_shipment_scans.*, users.username AS scanned_by_username").
		Joins("LEFT JOIN users ON users.id = order_shipment_scans.scanned_by").
		Where("order_shipment_scans.order_id = ?", order.ID).
		Order("order_shipment_scans.created_at DESC").
		Find(&history).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve scan history",
		)
	}

	result := make(
		[]OutboundVerificationHistoryDTO,
		0,
		len(history),
	)

	for _, scan := range history {

		result = append(result, OutboundVerificationHistoryDTO{

			ID:            scan.ID,
			OrderNo:       scan.OrderNo,
			ShipmentID:    scan.ShipmentID,
			OrderDetailID: scan.OrderDetailID,

			ScannedBy: func() string {
				if scan.ScannedByUsername == nil {
					return "-"
				}
				return *scan.ScannedByUsername
			}(),
			ScannedAt: scan.ScannedAt,

			Status:  scan.Status,
			Remarks: scan.Remarks,
		})
	}

	return verificationSuccess(
		ctx,
		"Scan history retrieved successfully",
		result,
	)
}

// ============================================================
// 4. CONFIRM VERIFICATION
// POST /verification/:orderNo/confirm
// ============================================================

func (c *MobileOutboundVerificationController) ConfirmVerification(
	ctx *fiber.Ctx,
) error {

	orderNo := strings.TrimSpace(ctx.Params("orderNo"))

	if orderNo == "" {
		return verificationError(
			ctx,
			fiber.StatusBadRequest,
			"Order number is required",
		)
	}

	userID, err := getVerificationUserID(ctx)

	if err != nil {
		return verificationError(
			ctx,
			fiber.StatusUnauthorized,
			"Unable to identify authenticated user",
		)
	}

	tx := c.DB.Begin(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})

	if tx.Error != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Unable to start confirmation transaction",
		)
	}

	defer tx.Rollback()

	// --------------------------------------------------------
	// Validate SPK
	// --------------------------------------------------------

	order, err := c.getOrderHeader(tx, orderNo)

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return verificationError(
				ctx,
				fiber.StatusNotFound,
				"SPK not found",
			)
		}

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve SPK",
		)
	}

	if !strings.EqualFold(order.Status, "open") {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"SPK has already been confirmed or is not open",
		)
	}

	// --------------------------------------------------------
	// Get Expected Shipments
	// --------------------------------------------------------

	details, err := c.getOrderDetails(tx, order.ID)

	if err != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to retrieve order details",
		)
	}

	if len(details) == 0 {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"Cannot confirm SPK without shipment details",
		)
	}

	if err := validateUniqueShipmentIDs(details); err != nil {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			err.Error(),
		)
	}

	// --------------------------------------------------------
	// Validate All Shipments Scanned
	// --------------------------------------------------------

	expectedIDs := make([]uint, 0, len(details))

	for _, detail := range details {
		expectedIDs = append(expectedIDs, detail.ID)
	}

	var totalScanned int64

	if err := tx.Model(&models.OrderShipmentScan{}).
		Where("order_id = ?", order.ID).
		Where("order_detail_id IN ?", expectedIDs).
		Count(&totalScanned).Error; err != nil {

		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to validate shipment verification",
		)
	}

	totalExpected := int64(len(details))

	if totalScanned != totalExpected {

		return ctx.Status(
			fiber.StatusUnprocessableEntity,
		).JSON(fiber.Map{

			"success": false,
			"message": "Not all shipments have been verified",

			"data": fiber.Map{
				"total_expected": totalExpected,
				"total_scanned":  totalScanned,
				"total_pending":  totalExpected - totalScanned,
			},
		})
	}

	now := time.Now()

	// --------------------------------------------------------
	// Update Order Header
	// --------------------------------------------------------

	result := tx.Model(&models.OrderHeader{}).
		Where("id = ?", order.ID).
		Where("status = ?", "open").
		Updates(map[string]interface{}{

			"status":      "confirmed",
			"verified_by": userID,
			"verified_at": now,
			"updated_by":  userID,
			"updated_at":  now,
		})

	if result.Error != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to confirm SPK",
		)
	}

	if result.RowsAffected != 1 {
		return verificationError(
			ctx,
			fiber.StatusUnprocessableEntity,
			"SPK has already been confirmed by another user",
		)
	}

	if err := tx.Commit().Error; err != nil {
		return verificationError(
			ctx,
			fiber.StatusInternalServerError,
			"Failed to commit confirmation",
		)
	}

	// --------------------------------------------------------
	// Success
	// --------------------------------------------------------

	return verificationSuccess(
		ctx,
		"Outbound verification confirmed successfully",
		fiber.Map{

			"order_id": order.ID,
			"order_no": order.OrderNo,

			"status": "confirmed",

			"verified_by": userID,
			"verified_at": now,

			"total_expected": totalExpected,
			"total_scanned":  totalScanned,
			"total_pending":  0,
		},
	)
}

func (c *MobileOutboundVerificationController) DeleteShipment(ctx *fiber.Ctx) error {
	orderNo := ctx.Params("orderNo")

	detailID, err := strconv.ParseUint(ctx.Params("orderDetailId"), 10, 64)
	if err != nil || detailID == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid Order Detail ID",
		})
	}

	err = c.DB.WithContext(ctx.UserContext()).Transaction(func(tx *gorm.DB) error {
		var order models.OrderHeader

		if err := tx.
			Where("order_no = ?", orderNo).
			First(&order).Error; err != nil {
			return err
		}

		if !strings.EqualFold(order.Status, "open") {
			return errors.New("shipment can only be deleted when SPK status is open")
		}

		var detail models.OrderDetail

		if err := tx.
			Where("id = ? AND order_id = ?", detailID, order.ID).
			First(&detail).Error; err != nil {
			return err
		}

		// Only verified shipments may be deleted.
		var scan models.OrderShipmentScan
		if err := tx.
			Where("order_id = ? AND order_detail_id = ?", order.ID, detail.ID).
			First(&scan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("pending Shipment ID cannot be deleted; only verified shipments are allowed")
			}
			return err
		}

		// Permanently delete the associated verification scan.
		if err := tx.Unscoped().
			Where("order_id = ? AND order_detail_id = ?", order.ID, detail.ID).
			Delete(&models.OrderShipmentScan{}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Shipment ID : " + orderNo + " deleted successfully",
	})
}
