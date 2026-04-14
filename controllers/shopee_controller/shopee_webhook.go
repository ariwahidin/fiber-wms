package shopee_controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fiber-app/models"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ShopeeController struct {
	DB *gorm.DB
}

func NewShopeeController(db *gorm.DB) *ShopeeController {
	return &ShopeeController{DB: db}
}

// Push codes dari Shopee
const (
	PushCodeOrderStatus          = 3
	PushCodeOrderTrackingNo      = 4
	PushCodeBannedItem           = 6
	PushCodeReservedStockChange  = 8
	PushCodeViolationItem        = 16
	PushCodeAutoCorrectItem      = 17
	PushCodeItemPriceUpdate      = 22
	PushCodeScheduledPublishFail = 27
)

// ── Structs ──────────────────────────────────────────────────────────────────

type ShopeePushPayload struct {
	Code      int             `json:"code"`
	Timestamp int64           `json:"timestamp"`
	ShopID    int64           `json:"shop_id"`
	Data      json.RawMessage `json:"data"`
}

// Order status push (code 3)
type OrderStatusData struct {
	OrderSN           string `json:"ordersn"`
	Status            string `json:"status"`
	CompletedScenario string `json:"completed_scenario"` // tambah ini
	UpdateTime        int64  `json:"update_time"`
}

// type OrderStatusData struct {
// 	OrderSN       string `json:"ordersn"`
// 	Status        string `json:"status"`
// 	UpdateTime    int64  `json:"update_time"`
// 	BuyerUserID   int64  `json:"buyer_user_id"`
// 	BuyerUsername string `json:"buyer_username"`
// }

// Order tracking push (code 4)
type OrderTrackingData struct {
	OrderSN    string `json:"ordersn"`
	TrackingNo string `json:"tracking_no"`
}

// Reserved stock change push (code 8)
type ReservedStockData struct {
	ItemID        int64 `json:"item_id"`
	ModelID       int64 `json:"model_id"`
	CurrentStock  int   `json:"current_stock"`
	ReservedStock int   `json:"reserved_stock"`
}

// ── Signature Verification ───────────────────────────────────────────────────

func verifyShopeeSignature(partnerKey, pushURL string, timestamp int64, body []byte) (string, bool) {
	// Shopee signature: HMAC-SHA256( partner_key + "|" + push_url + "|" + timestamp + "|" + body )
	baseStr := partnerKey + "|" + pushURL + "|" + fmt.Sprintf("%d", timestamp) + "|" + string(body)
	mac := hmac.New(sha256.New, []byte(partnerKey))
	mac.Write([]byte(baseStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return expectedSig, true
}

// ── Main Webhook Handler ─────────────────────────────────────────────────────

func (s *ShopeeController) WebhookHandler(ctx *fiber.Ctx) error {
	// partnerKey := config.ShopeePartnerKey // ambil dari config/env
	// pushURL := config.ShopeePushURL       // e.g. "https://wms-api.logspeedy.com/webhook/shopee"

	cfg := s.loadConfig()

	partnerKey := cfg.PartnerKey
	pushURL := cfg.PushURL

	body := ctx.Body()

	// 1. Verifikasi signature dari header
	receivedSig := ctx.Get("Authorization")
	baseStr := partnerKey + "|" + pushURL + "|" + string(body)
	mac := hmac.New(sha256.New, []byte(partnerKey))
	mac.Write([]byte(baseStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expectedSig {
		log.Printf("[Shopee Webhook] Invalid signature. received=%s expected=%s", receivedSig, expectedSig)
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid signature",
		})
	}

	// 2. Parse payload
	var payload ShopeePushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Shopee Webhook] Failed to parse payload: %v", err)
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid payload",
		})
	}

	log.Printf("[Shopee Webhook] Received code=%d shop_id=%d", payload.Code, payload.ShopID)

	// 3. Route ke handler berdasarkan push code
	switch payload.Code {
	case PushCodeOrderStatus:
		go s.handleOrderStatus(payload)
	case PushCodeOrderTrackingNo:
		go s.handleOrderTracking(payload)
	case PushCodeReservedStockChange:
		go s.handleReservedStock(payload)
	case PushCodeBannedItem:
		go s.handleBannedItem(payload)
	case PushCodeViolationItem:
		go s.handleViolationItem(payload)
	case PushCodeItemPriceUpdate:
		go s.handleItemPriceUpdate(payload)
	default:
		log.Printf("[Shopee Webhook] Unhandled push code: %d", payload.Code)
	}

	// 4. Wajib balas 200 secepat mungkin ke Shopee
	return ctx.SendStatus(fiber.StatusOK)
}

// ── Event Handlers ───────────────────────────────────────────────────────────

func (s *ShopeeController) handleOrderStatus(payload ShopeePushPayload) {
	var data OrderStatusData
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("[Shopee] handleOrderStatus parse error: %v", err)
		return
	}

	log.Printf("[Shopee] Order status update: ordersn=%s status=%s", data.OrderSN, data.Status)

	// Simpan ke DB / trigger WMS logic
	event := models.ShopeeWebhookLog{
		ShopID:    payload.ShopID,
		PushCode:  payload.Code,
		OrderSN:   data.OrderSN,
		Status:    data.Status,
		RawData:   string(payload.Data),
		CreatedAt: time.Now(),
	}
	if err := s.DB.Create(&event).Error; err != nil {
		log.Printf("[Shopee] Failed to save webhook log: %v", err)
	}

	// TODO: trigger WMS inbound / update order status di WMS
}

func (s *ShopeeController) handleOrderTracking(payload ShopeePushPayload) {
	var data OrderTrackingData
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("[Shopee] handleOrderTracking parse error: %v", err)
		return
	}

	log.Printf("[Shopee] Tracking update: ordersn=%s tracking=%s", data.OrderSN, data.TrackingNo)

	// TODO: update tracking number di WMS
}

func (s *ShopeeController) handleReservedStock(payload ShopeePushPayload) {
	var data ReservedStockData
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("[Shopee] handleReservedStock parse error: %v", err)
		return
	}

	log.Printf("[Shopee] Reserved stock change: item_id=%d reserved=%d current=%d",
		data.ItemID, data.ReservedStock, data.CurrentStock)

	// TODO: sync stock ke WMS
}

func (s *ShopeeController) handleBannedItem(payload ShopeePushPayload) {
	log.Printf("[Shopee] Banned item event: shop_id=%d data=%s", payload.ShopID, string(payload.Data))
	// TODO: flag item di WMS
}

func (s *ShopeeController) handleViolationItem(payload ShopeePushPayload) {
	log.Printf("[Shopee] Violation item event: shop_id=%d data=%s", payload.ShopID, string(payload.Data))
	// TODO: flag item di WMS
}

func (s *ShopeeController) handleItemPriceUpdate(payload ShopeePushPayload) {
	log.Printf("[Shopee] Item price update: shop_id=%d data=%s", payload.ShopID, string(payload.Data))
	// TODO: sync price ke WMS
}

type ShopeeConfig struct {
	PartnerID    int64
	PartnerKey   string
	ShopID       int64
	AccessToken  string
	RefreshToken string
	BaseURL      string
	PushURL      string
}

func (c *ShopeeController) loadConfig() ShopeeConfig {
	var cfg models.ShopeeConfig

	err := c.DB.
		Where("is_active = ?", true).
		Order("id desc").
		Take(&cfg).Error

	if err == nil && cfg.AccessToken != "" {
		return ShopeeConfig{
			PartnerID:    cfg.PartnerID,
			PartnerKey:   cfg.PartnerKey,
			ShopID:       cfg.ShopID,
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
			BaseURL:      cfg.BaseURL,
		}
	}

	// Fallback ke .env
	return loadShopeeConfig()
}

func loadShopeeConfig() ShopeeConfig {
	partnerID, _ := strconv.ParseInt(os.Getenv("SHOPEE_PARTNER_ID"), 10, 64)
	shopID, _ := strconv.ParseInt(os.Getenv("SHOPEE_SHOP_ID"), 10, 64)

	return ShopeeConfig{
		PartnerID:    partnerID,
		PartnerKey:   os.Getenv("SHOPEE_PARTNER_KEY"),
		ShopID:       shopID,
		AccessToken:  os.Getenv("SHOPEE_ACCESS_TOKEN"),
		RefreshToken: os.Getenv("SHOPEE_REFRESH_TOKEN"),
		BaseURL:      os.Getenv("SHOPEE_BASE_URL"),
	}
}
