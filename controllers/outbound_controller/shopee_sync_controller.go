package outbound_controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fiber-app/controllers/helpers"
	"fiber-app/models"
	"fiber-app/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ============================================================
// CONFIG
// ============================================================

type ShopeeConfig struct {
	PartnerID    int64
	PartnerKey   string
	ShopID       int64
	AccessToken  string
	RefreshToken string
	BaseURL      string
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

func (cfg ShopeeConfig) sign(path string, timestamp int64) string {
	base := fmt.Sprintf("%d%s%d%s%d", cfg.PartnerID, path, timestamp, cfg.AccessToken, cfg.ShopID)
	mac := hmac.New(sha256.New, []byte(cfg.PartnerKey))
	mac.Write([]byte(base))
	return hex.EncodeToString(mac.Sum(nil))
}

func (cfg ShopeeConfig) signPublic(path string, timestamp int64) string {
	base := fmt.Sprintf("%d%s%d", cfg.PartnerID, path, timestamp)
	mac := hmac.New(sha256.New, []byte(cfg.PartnerKey))
	mac.Write([]byte(base))
	return hex.EncodeToString(mac.Sum(nil))
}

func (cfg ShopeeConfig) buildURL(path string, timestamp int64, sign string, extra string) string {
	return fmt.Sprintf("%s%s?partner_id=%d&timestamp=%d&sign=%s&shop_id=%d&access_token=%s%s",
		cfg.BaseURL, path, cfg.PartnerID, timestamp, sign, cfg.ShopID, cfg.AccessToken, extra)
}

// ============================================================
// SHOPEE API TYPES
// ============================================================

type ShopeeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireIn     int64  `json:"expire_in"`
	Error        string `json:"error"`
	Message      string `json:"message"`
}

type ShopeeOrderListResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		More       bool   `json:"more"`
		NextCursor string `json:"next_cursor"`
		OrderList  []struct {
			OrderSN string `json:"order_sn"`
		} `json:"order_list"`
	} `json:"response"`
}

type ShopeeOrderDetailResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		OrderList []ShopeeOrderDetail `json:"order_list"`
	} `json:"response"`
}

type ShopeeOrderDetail struct {
	OrderSN          string  `json:"order_sn"`
	OrderStatus      string  `json:"order_status"`
	TotalAmount      float64 `json:"total_amount"`
	PaymentMethod    string  `json:"payment_method"`
	ShippingCarrier  string  `json:"shipping_carrier"`
	Note             string  `json:"note"`
	PayTime          int64   `json:"pay_time"`
	BuyerUsername    string  `json:"buyer_username"`
	RecipientAddress struct {
		Name        string `json:"name"`
		Phone       string `json:"phone"`
		District    string `json:"district"`
		City        string `json:"city"`
		State       string `json:"state"`
		FullAddress string `json:"full_address"`
	} `json:"recipient_address"`
	ItemList []struct {
		ItemID         int64   `json:"item_id"`
		ItemName       string  `json:"item_name"`
		ItemSKU        string  `json:"item_sku"`
		ModelSKU       string  `json:"model_sku"`
		ModelQty       int     `json:"model_quantity_purchased"`
		ModelOrigPrice float64 `json:"model_original_price"`
		ModelDiscPrice float64 `json:"model_discounted_price"`
	} `json:"item_list"`
}

// ============================================================
// SYNC RESULT
// ============================================================

type ShopeeSyncResult struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message"`
	TotalOrders   int      `json:"total_orders"`
	Synced        int      `json:"synced"`
	Skipped       int      `json:"skipped"`
	Failed        int      `json:"failed"`
	OutboundNos   []string `json:"outbound_nos"`
	SkippedOrders []string `json:"skipped_orders"`
	Errors        []string `json:"errors"`
}

// ============================================================
// CONTROLLER
// ============================================================

type ShopeeSyncController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
}

func NewShopeeSyncController(db *gorm.DB, queryDB *gorm.DB) *ShopeeSyncController {
	return &ShopeeSyncController{DB: db, QueryDB: queryDB}
}

// ============================================================
// REFRESH TOKEN
// ============================================================

func (c *ShopeeSyncController) RefreshToken() error {
	// cfg := loadShopeeConfig()
	cfg := c.loadConfig()

	path := "/api/v2/auth/access_token/get"
	timestamp := time.Now().Unix()
	sign := cfg.signPublic(path, timestamp)

	url := fmt.Sprintf("%s%s?partner_id=%d&timestamp=%d&sign=%s",
		cfg.BaseURL, path, cfg.PartnerID, timestamp, sign)

	payload := map[string]interface{}{
		"refresh_token": cfg.RefreshToken,
		"shop_id":       cfg.ShopID,
		"partner_id":    cfg.PartnerID,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result ShopeeTokenResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("shopee error: %s - %s", result.Error, result.Message)
	}

	// Untuk persistent: update file .env atau tabel DB

	// Simpan ke DB
	if err := c.saveToken(result.AccessToken, result.RefreshToken); err != nil {
		return fmt.Errorf("gagal simpan token ke DB: %w", err)
	}

	// Update .env di runtime (in-memory via os.Setenv)

	os.Setenv("SHOPEE_ACCESS_TOKEN", result.AccessToken)
	os.Setenv("SHOPEE_REFRESH_TOKEN", result.RefreshToken)

	fmt.Printf("[Shopee] Token refreshed. New access_token: %s..., expire_in: %d detik\n",
		result.AccessToken[:8], result.ExpireIn)

	return nil
}

// GET /api/shopee/refresh-token (manual trigger)
func (c *ShopeeSyncController) HandleRefreshToken(ctx *fiber.Ctx) error {
	if err := c.RefreshToken(); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to refresh token: " + err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{
		"success":      true,
		"message":      "Token refreshed successfully",
		"access_token": os.Getenv("SHOPEE_ACCESS_TOKEN"),
	})
}

// ============================================================
// FETCH ORDER LIST
// ============================================================

func (c *ShopeeSyncController) fetchOrderList(cfg ShopeeConfig) ([]string, error) {
	fmt.Printf("[Shopee] Fetching order list with access_token: %s...\n", cfg.AccessToken[:8])
	path := "/api/v2/order/get_order_list"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)

	timeTo := timestamp
	timeFrom := timestamp - (15 * 24 * 60 * 60)

	extra := fmt.Sprintf("&time_range_field=create_time&time_from=%d&time_to=%d&page_size=50&order_status=READY_TO_SHIP",
		timeFrom, timeTo)

	url := cfg.buildURL(path, timestamp, sign, extra)
	fmt.Printf("[Shopee] Calling API URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ShopeeOrderListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Jika token expired, refresh dan retry sekali
	if result.Error == "invalid_acceess_token" || result.Error == "invalid_access_token" {
		fmt.Println("[Shopee] Access token expired, refreshing...")
		fmt.Printf("[Shopee] Old access_token: %s...\n", cfg.AccessToken[:8])
		if err := c.RefreshToken(); err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
		// Retry dengan token baru
		// cfg = loadShopeeConfig()
		cfg = c.loadConfig()
		return c.fetchOrderList(cfg)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.Message)
	}

	var orderSNs []string
	for _, o := range result.Response.OrderList {
		orderSNs = append(orderSNs, o.OrderSN)
	}

	return orderSNs, nil
}

// ============================================================
// FETCH ORDER DETAILS
// ============================================================

func (c *ShopeeSyncController) fetchOrderDetails(cfg ShopeeConfig, orderSNs []string) ([]ShopeeOrderDetail, error) {
	path := "/api/v2/order/get_order_detail"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)

	extra := fmt.Sprintf("&order_sn_list=%s&response_optional_fields=item_list,recipient_address,total_amount,payment_method,shipping_carrier,note,pay_time,buyer_username",
		strings.Join(orderSNs, ","))

	url := cfg.buildURL(path, timestamp, sign, extra)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ShopeeOrderDetailResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.Message)
	}

	return result.Response.OrderList, nil
}

// ============================================================
// CORE SYNC LOGIC (dipakai oleh manual trigger & cron)
// ============================================================

func (c *ShopeeSyncController) RunSync(userID int) ShopeeSyncResult {
	result := ShopeeSyncResult{
		OutboundNos:   []string{},
		SkippedOrders: []string{},
		Errors:        []string{},
	}

	fmt.Println("[Shopee] Running sync...")

	// cfg := loadShopeeConfig()
	cfg := c.loadConfig()

	// 1. Fetch order list
	orderSNs, err := c.fetchOrderList(cfg)
	if err != nil {
		result.Message = "Failed to fetch order list: " + err.Error()
		return result
	}

	result.TotalOrders = len(orderSNs)

	if len(orderSNs) == 0 {
		result.Success = true
		result.Message = "No new orders from Shopee"
		return result
	}

	// 2. Fetch order details
	// cfg = loadShopeeConfig()
	cfg = c.loadConfig()
	orders, err := c.fetchOrderDetails(cfg, orderSNs)
	if err != nil {
		result.Message = "Failed to fetch order details: " + err.Error()
		return result
	}

	// 3. Process each order
	for _, order := range orders {
		outboundNo, skipped, errMsg := c.processOrder(order, userID)
		if errMsg != "" {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] %s", order.OrderSN, errMsg))
			continue
		}
		if skipped {
			result.Skipped++
			result.SkippedOrders = append(result.SkippedOrders, order.OrderSN)
			continue
		}
		result.Synced++
		result.OutboundNos = append(result.OutboundNos, outboundNo)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Sync completed. Synced: %d, Skipped: %d, Failed: %d",
		result.Synced, result.Skipped, result.Failed)

	return result
}

// ============================================================
// MANUAL TRIGGER — POST /api/shopee/sync
// ============================================================

func (c *ShopeeSyncController) SyncShopeeOrders(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))
	result := c.RunSync(userID)

	if !result.Success && result.TotalOrders == 0 {
		return ctx.Status(fiber.StatusInternalServerError).JSON(result)
	}

	return ctx.Status(fiber.StatusOK).JSON(result)
}

// ============================================================
// PROCESS SINGLE ORDER
// ============================================================

func (c *ShopeeSyncController) processOrder(order ShopeeOrderDetail, userID int) (outboundNo string, skipped bool, errMsg string) {
	// Deduplication
	var existing models.OutboundHeader
	err := c.DB.Where("shipment_id = ?", order.OrderSN).First(&existing).Error
	if err == nil {
		return "", true, ""
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, "DB error cek duplikat: " + err.Error()
	}

	// Config dari env
	ownerCode := os.Getenv("SHOPEE_WMS_OWNER_CODE")
	whsCode := os.Getenv("SHOPEE_WMS_WHS_CODE")
	customerCode := os.Getenv("SHOPEE_WMS_CUSTOMER_CODE")
	if ownerCode == "" {
		ownerCode = "YUWELL"
	}
	if whsCode == "" {
		whsCode = "WH-B"
	}
	if customerCode == "" {
		customerCode = "SHOPEE"
	}

	if len(order.ItemList) == 0 {
		return "", false, "order has no items"
	}

	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	repo := repositories.NewOutboundRepository(tx)

	generatedNo, err := repo.GenerateOutboundNumber()
	if err != nil {
		tx.Rollback()
		return "", false, "gagal generate outbound no: " + err.Error()
	}

	outboundDate := time.Now().Format("2006-01-02")
	if order.PayTime > 0 {
		outboundDate = time.Unix(order.PayTime, 0).Format("2006-01-02")
	}

	header := models.OutboundHeader{
		OutboundNo:   generatedNo,
		OutboundDate: outboundDate,
		CustomerCode: customerCode,
		ShipmentID:   order.OrderSN,
		WhsCode:      whsCode,
		OwnerCode:    ownerCode,
		Remarks:      fmt.Sprintf("Shopee Order: %s | Buyer: %s | %s", order.OrderSN, order.BuyerUsername, order.PaymentMethod),
		Status:       "open",
		RawStatus:    "DRAFT",
		OrderType:    "B2C - Marketplace",
		DraftTime:    time.Now(),
		Source:       "SHOPEE",
		Integration:  true,
		CustAddress:  order.RecipientAddress.FullAddress,
		CustCity:     order.RecipientAddress.City,
		DelivTo:      customerCode,
		DelivAddress: order.RecipientAddress.FullAddress,
		DelivCity:    order.RecipientAddress.City,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}

	if err := tx.Create(&header).Error; err != nil {
		tx.Rollback()
		return "", false, "gagal insert header: " + err.Error()
	}

	for _, item := range order.ItemList {
		sku := item.ModelSKU
		if sku == "" {
			sku = item.ItemSKU
		}
		if sku == "" {
			tx.Rollback()
			return "", false, fmt.Sprintf("item '%s' tidak punya SKU", item.ItemName)
		}

		var product models.Product
		if err := tx.Where("item_code = ?", sku).First(&product).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", false, fmt.Sprintf("SKU '%s' tidak ditemukan di WMS", sku)
			}
			return "", false, "DB error cari product: " + err.Error()
		}

		var uomConversion models.UomConversion
		if err := tx.Where("item_code = ? AND factor = 1", product.ItemCode).First(&uomConversion).Error; err != nil {
			if err2 := tx.Where("item_code = ?", product.ItemCode).First(&uomConversion).Error; err2 != nil {
				tx.Rollback()
				return "", false, fmt.Sprintf("UOM tidak ditemukan untuk SKU '%s'", sku)
			}
		}

		detail := models.OutboundDetail{
			OutboundNo:   generatedNo,
			OutboundID:   header.ID,
			ItemCode:     product.ItemCode,
			ItemID:       int(product.ID),
			Barcode:      uomConversion.Ean,
			CustomerCode: customerCode,
			Uom:          uomConversion.FromUom,
			Quantity:     float64(item.ModelQty),
			WhsCode:      whsCode,
			DivisionCode: "E-COMMERCE",
			OwnerCode:    ownerCode,
			QaStatus:     "A",
			SNCheck:      "N",
			Remarks:      item.ItemName,
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}

		if err := tx.Create(&detail).Error; err != nil {
			tx.Rollback()
			return "", false, "gagal insert detail: " + err.Error()
		}
	}

	helpers.InsertTransactionHistory(tx, generatedNo, "open", "OUTBOUND",
		fmt.Sprintf("Auto-sync Shopee - Order SN: %s", order.OrderSN), userID)

	if err := tx.Commit().Error; err != nil {
		return "", false, "gagal commit: " + err.Error()
	}

	return generatedNo, false, ""
}

// ============================================================
// LOGISTICS TYPES
// ============================================================

type ShopeeShippingParamResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		Pickup *struct {
			AddressList []struct {
				AddressID      int64  `json:"address_id"`
				Address        string `json:"address"`
				City           string `json:"city"`
				State          string `json:"state"`
				AddressType    []int  `json:"address_type"`
				PickupTimeList []struct {
					Date         string `json:"date"`
					TimeText     string `json:"time_text"`
					PickupTimeID string `json:"pickup_time_id"`
				} `json:"pickup_time_list"`
			} `json:"address_list"`
		} `json:"pickup"`
		Dropoff *struct {
			BranchList []struct {
				BranchID   int64  `json:"branch_id"`
				BranchName string `json:"branch_name"`
				Address    string `json:"address"`
				City       string `json:"city"`
			} `json:"branch_list"`
		} `json:"dropoff"`
		NonIntegrated *struct{} `json:"non_integrated"`
	} `json:"response"`
}

type ShopeeInitShipmentRequest struct {
	OrderSN string `json:"order_sn"`
	// Pickup
	AddressID    int64  `json:"address_id,omitempty"`
	PickupTimeID string `json:"pickup_time_id,omitempty"`
	// Dropoff
	BranchID int64 `json:"branch_id,omitempty"`
	// Method: "pickup" | "dropoff" | "non_integrated"
	Method string `json:"method"`
}

type ShopeeInitShipmentResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ============================================================
// GET SHIPPING PARAMETER
// GET /api/shopee/shipping-param/:order_sn
// ============================================================

func (c *ShopeeSyncController) GetShippingParameter(ctx *fiber.Ctx) error {
	orderSN := ctx.Params("order_sn")
	if orderSN == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "order_sn is required",
		})
	}

	// cfg := loadShopeeConfig()
	cfg := c.loadConfig()
	path := "/api/v2/logistics/get_shipping_parameter"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)

	url := cfg.buildURL(path, timestamp, sign, fmt.Sprintf("&order_sn=%s", orderSN))

	resp, err := http.Get(url)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to call Shopee API: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ShopeeShippingParamResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse response: " + err.Error(),
		})
	}

	if result.Error != "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": result.Error + ": " + result.Message,
		})
	}

	// Tentukan method yang tersedia
	method := ""
	if result.Response.Pickup != nil {
		method = "pickup"
	} else if result.Response.Dropoff != nil {
		method = "dropoff"
	} else {
		method = "non_integrated"
	}

	return ctx.JSON(fiber.Map{
		"success":  true,
		"method":   method,
		"response": result.Response,
	})
}

// ============================================================
// INIT SHIPMENT
// POST /api/shopee/init-shipment
// ============================================================

func (c *ShopeeSyncController) InitShipment(ctx *fiber.Ctx) error {
	var payload ShopeeInitShipmentRequest
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload: " + err.Error(),
		})
	}

	if payload.OrderSN == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "order_sn is required",
		})
	}

	// cfg := loadShopeeConfig()
	cfg := c.loadConfig()
	path := "/api/v2/logistics/init_shipment"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)
	url := cfg.buildURL(path, timestamp, sign, "")

	// Build request body sesuai method
	requestBody := map[string]interface{}{
		"order_sn": payload.OrderSN,
	}

	switch payload.Method {
	case "pickup":
		requestBody["pickup"] = map[string]interface{}{
			"address_id":     payload.AddressID,
			"pickup_time_id": payload.PickupTimeID,
		}
	case "dropoff":
		requestBody["dropoff"] = map[string]interface{}{
			"branch_id": payload.BranchID,
		}
	case "non_integrated":
		requestBody["non_integrated"] = map[string]interface{}{}
	default:
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "method harus pickup, dropoff, atau non_integrated",
		})
	}

	bodyBytes, _ := json.Marshal(requestBody)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to call Shopee API: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result ShopeeInitShipmentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse response: " + err.Error(),
		})
	}

	if result.Error != "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": result.Error + ": " + result.Message,
		})
	}

	// Setelah init shipment berhasil, ambil tracking number
	trackingPath := "/api/v2/logistics/get_tracking_number"
	trackingTimestamp := time.Now().Unix()
	trackingSign := cfg.sign(trackingPath, trackingTimestamp)
	trackingURL := cfg.buildURL(trackingPath, trackingTimestamp, trackingSign,
		fmt.Sprintf("&order_sn=%s", payload.OrderSN))

	trackingResp, err := http.Get(trackingURL)
	if err == nil {
		defer trackingResp.Body.Close()
		trackingBody, _ := io.ReadAll(trackingResp.Body)

		var trackingResult ShopeeTrackingResponse
		if err := json.Unmarshal(trackingBody, &trackingResult); err == nil {
			if trackingResult.Error == "" && trackingResult.Response.TrackingNumber != "" {
				// Simpan AWB ke outbound header
				c.DB.Table("outbound_headers").
					Where("shipment_id = ?", payload.OrderSN).
					Update("awb_no", trackingResult.Response.TrackingNumber)
			}
		}
	}

	return ctx.JSON(fiber.Map{
		"success":  true,
		"message":  "Arrange shipment berhasil",
		"order_sn": payload.OrderSN,
	})
}

// ============================================================
// TEST API — GET /api/shopee/test-api
// Return raw JSON response dari Shopee order list
// ============================================================

func (c *ShopeeSyncController) TestAPI(ctx *fiber.Ctx) error {
	cfg := c.loadConfig()

	path := "/api/v2/order/get_order_list"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)

	timeTo := timestamp
	timeFrom := timestamp - (15 * 24 * 60 * 60)

	extra := fmt.Sprintf("&time_range_field=create_time&time_from=%d&time_to=%d&page_size=50&order_status=READY_TO_SHIP",
		timeFrom, timeTo)

	url := cfg.buildURL(path, timestamp, sign, extra)

	resp, err := http.Get(url)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse ke map agar bisa dikirim sebagai JSON proper
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse Shopee response",
			"raw":     string(body),
		})
	}

	// return ctx.JSON(fiber.Map{
	// 	"success":  true,
	// 	"endpoint": path,
	// 	"data":     raw,
	// })

	return ctx.JSON(fiber.Map{
		"success":  true,
		"endpoint": path,
		"url":      url,
		"data":     raw,
	})
}
