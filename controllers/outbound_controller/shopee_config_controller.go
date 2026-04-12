package outbound_controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ============================================================
// LOAD CONFIG — DB first, fallback ke .env
// ============================================================

func (c *ShopeeSyncController) loadConfig() ShopeeConfig {
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

// func (c *ShopeeSyncController) loadConfig() ShopeeConfig {
// 	var cfg models.ShopeeConfig
// 	err := c.DB.Where("is_active = ?", true).Order("id desc").First(&cfg).Error
// 	if err == nil && cfg.AccessToken != "" {
// 		return ShopeeConfig{
// 			PartnerID:    cfg.PartnerID,
// 			PartnerKey:   cfg.PartnerKey,
// 			ShopID:       cfg.ShopID,
// 			AccessToken:  cfg.AccessToken,
// 			RefreshToken: cfg.RefreshToken,
// 			BaseURL:      cfg.BaseURL,
// 		}
// 	}
// 	// Fallback ke .env
// 	return loadShopeeConfig()
// }

// ============================================================
// SAVE TOKEN KE DB
// ============================================================

func (c *ShopeeSyncController) saveToken(accessToken, refreshToken string) error {
	return c.DB.Model(&models.ShopeeConfig{}).
		Where("is_active = ?", true).
		Updates(map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"updated_at":    time.Now(),
		}).Error
}

// ============================================================
// REFRESH TOKEN — baca dari DB, simpan ke DB
// ============================================================

func (c *ShopeeSyncController) RefreshTokenFromDB() error {
	cfg := c.loadConfig()

	// Lihat config aktif
	fmt.Println("config aktif ", cfg)
	fmt.Printf("Attempting to refresh token for ShopID: %d, PartnerID: %d\n", cfg.ShopID, cfg.PartnerID)

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

	// Simpan ke DB
	if err := c.saveToken(result.AccessToken, result.RefreshToken); err != nil {
		return fmt.Errorf("gagal simpan token ke DB: %w", err)
	}

	// Update runtime env juga (untuk proses yang masih pakai env)
	os.Setenv("SHOPEE_ACCESS_TOKEN", result.AccessToken)
	os.Setenv("SHOPEE_REFRESH_TOKEN", result.RefreshToken)

	fmt.Printf("[Shopee] Token refreshed & saved to DB. Expire: %d detik\n", result.ExpireIn)
	return nil
}

// ============================================================
// GET CONFIG — GET /api/shopee/config
// ============================================================

func (c *ShopeeSyncController) GetConfig(ctx *fiber.Ctx) error {
	var cfg models.ShopeeConfig
	// if err := c.DB.Where("is_active = ?", true).Order("id desc").First(&cfg).Error; err != nil {
	if err := c.DB.
		Where("is_active = ?", true).
		Order("id desc").
		Take(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.JSON(fiber.Map{
				"success": true,
				"data":    nil,
				"message": "Belum ada konfigurasi Shopee",
			})
		}
		return ctx.Status(500).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Sembunyikan secret key sebagian
	maskedKey := ""
	if len(cfg.PartnerKey) > 8 {
		maskedKey = cfg.PartnerKey[:8] + "****"
	}
	maskedAccess := ""
	if len(cfg.AccessToken) > 6 {
		maskedAccess = cfg.AccessToken[:6] + "****"
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":           cfg.ID,
			"partner_id":   cfg.PartnerID,
			"partner_key":  maskedKey,
			"shop_id":      cfg.ShopID,
			"access_token": maskedAccess,
			"base_url":     cfg.BaseURL,
			"is_active":    cfg.IsActive,
			"environment":  cfg.Environment,
			"updated_at":   cfg.UpdatedAt,
			"has_token":    cfg.AccessToken != "",
			"has_refresh":  cfg.RefreshToken != "",
		},
	})
}

// ============================================================
// SAVE CONFIG — POST /api/shopee/config
// Dipakai pertama kali setup atau update credential
// ============================================================

type ShopeeConfigPayload struct {
	PartnerID   int64  `json:"partner_id"`
	PartnerKey  string `json:"partner_key"`
	ShopID      int64  `json:"shop_id"`
	BaseURL     string `json:"base_url"`
	Environment string `json:"environment"`
}

func (c *ShopeeSyncController) SaveConfig(ctx *fiber.Ctx) error {
	var payload ShopeeConfigPayload
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid payload"})
	}

	userID := int(ctx.Locals("userID").(float64))

	// Nonaktifkan config lama
	c.DB.Model(&models.ShopeeConfig{}).
		Where("is_active = ?", true).
		Update("is_active", false)

	// Insert config baru
	cfg := models.ShopeeConfig{
		PartnerID:   payload.PartnerID,
		PartnerKey:  payload.PartnerKey,
		ShopID:      payload.ShopID,
		BaseURL:     payload.BaseURL,
		Environment: payload.Environment,
		IsActive:    true,
		CreatedBy:   userID,
		UpdatedBy:   userID,
	}

	if err := c.DB.Create(&cfg).Error; err != nil {
		return ctx.Status(500).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Konfigurasi berhasil disimpan",
		"data":    fiber.Map{"id": cfg.ID},
	})
}

// ============================================================
// OAUTH CALLBACK — GET /api/shopee/callback
// Menerima code + shop_id dari Shopee setelah authorize
// ============================================================

func (c *ShopeeSyncController) OAuthCallback(ctx *fiber.Ctx) error {
	code := ctx.Query("code")
	shopIDStr := ctx.Query("shop_id")

	if code == "" || shopIDStr == "" {
		return ctx.Status(400).SendString("Missing code or shop_id")
	}

	cfg := c.loadConfig()

	path := "/api/v2/auth/token/get"
	timestamp := time.Now().Unix()

	// Sign untuk token/get (public — tidak pakai access token)
	baseStr := fmt.Sprintf("%d%s%d", cfg.PartnerID, path, timestamp)
	mac := hmac.New(sha256.New, []byte(cfg.PartnerKey))
	mac.Write([]byte(baseStr))
	sign := hex.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("%s%s?partner_id=%d&timestamp=%d&sign=%s",
		cfg.BaseURL, path, cfg.PartnerID, timestamp, sign)

	// Parse shop_id dari query
	var shopID int64
	fmt.Sscanf(shopIDStr, "%d", &shopID)

	payload := map[string]interface{}{
		"code":       code,
		"shop_id":    shopID,
		"partner_id": cfg.PartnerID,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return ctx.Status(500).SendString("Failed to call Shopee API: " + err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result ShopeeTokenResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return ctx.Status(500).SendString("Failed to parse response")
	}

	if result.Error != "" {
		return ctx.Status(400).SendString("Shopee error: " + result.Error + " - " + result.Message)
	}

	// Simpan token + shop_id ke DB
	err = c.DB.Model(&models.ShopeeConfig{}).
		Where("is_active = ?", true).
		Updates(map[string]interface{}{
			"shop_id":       shopID,
			"access_token":  result.AccessToken,
			"refresh_token": result.RefreshToken,
			"updated_at":    time.Now(),
		}).Error

	if err != nil {
		return ctx.Status(500).SendString("Failed to save token to DB: " + err.Error())
	}

	// Update runtime env
	os.Setenv("SHOPEE_ACCESS_TOKEN", result.AccessToken)
	os.Setenv("SHOPEE_REFRESH_TOKEN", result.RefreshToken)

	fmt.Printf("[Shopee OAuth] Token saved. ShopID: %d, Expire: %d detik\n", shopID, result.ExpireIn)

	// Redirect ke halaman settings WMS
	// return ctx.Redirect("/wms/settings/shopee?status=success")
	return ctx.Redirect(os.Getenv("APP_FRONTEND_URL") + "/notification/shopee?status=success")
}

// ============================================================
// GENERATE AUTH URL — GET /api/shopee/auth-url
// Frontend pakai ini untuk redirect seller ke halaman Shopee authorize
// ============================================================

func (c *ShopeeSyncController) GenerateAuthURL(ctx *fiber.Ctx) error {
	cfg := c.loadConfigForAuth()

	path := "/api/v2/shop/auth_partner"
	timestamp := time.Now().Unix()

	baseStr := fmt.Sprintf("%d%s%d", cfg.PartnerID, path, timestamp)
	mac := hmac.New(sha256.New, []byte(cfg.PartnerKey))
	mac.Write([]byte(baseStr))
	sign := hex.EncodeToString(mac.Sum(nil))

	// Redirect URL = backend callback endpoint
	redirectURL := os.Getenv("SHOPEE_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:9000/api/shopee/callback"
	}

	authURL := fmt.Sprintf("%s%s?partner_id=%d&timestamp=%d&sign=%s&redirect=%s",
		cfg.BaseURL, path, cfg.PartnerID, timestamp, sign, redirectURL)

	return ctx.JSON(fiber.Map{
		"success":  true,
		"auth_url": authURL,
	})
}

func (c *ShopeeSyncController) loadConfigForAuth() ShopeeConfig {
	var cfg models.ShopeeConfig

	err := c.DB.
		Where("is_active = ?", true).
		Order("id desc").
		Take(&cfg).Error

	if err == nil {
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

// ============================================================
// MANUAL REFRESH TOKEN — POST /api/shopee/refresh-token
// ============================================================

func (c *ShopeeSyncController) HandleRefreshTokenDB(ctx *fiber.Ctx) error {
	if err := c.RefreshTokenFromDB(); err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to refresh token: " + err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Token berhasil di-refresh dan disimpan ke database",
	})
}
