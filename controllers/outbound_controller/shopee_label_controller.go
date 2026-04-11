package outbound_controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ============================================================
// SHOPEE LABEL TYPES
// ============================================================

type ShopeeTrackingResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		TrackingNumber string `json:"tracking_number"`
		PlpNumber      string `json:"plp_number"`
	} `json:"response"`
}

type ShopeeShippingDocResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		Result []struct {
			OrderSN     string `json:"order_sn"`
			PackageNo   string `json:"package_no"`
			Status      string `json:"status"`
			FailError   string `json:"fail_error"`
			FailMessage string `json:"fail_message"`
		} `json:"result"`
		FileType string `json:"file_type"`
		URL      string `json:"url"`
	} `json:"response"`
}

// ============================================================
// GET TRACKING NUMBER
// GET /api/shopee/tracking/:order_sn
// ============================================================

func (c *ShopeeSyncController) GetTrackingNumber(ctx *fiber.Ctx) error {
	orderSN := ctx.Params("order_sn")
	if orderSN == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "order_sn is required",
		})
	}

	cfg := loadShopeeConfig()

	path := "/api/v2/logistics/get_tracking_number"
	timestamp := time.Now().Unix()
	sign := cfg.sign(path, timestamp)

	extra := fmt.Sprintf("&order_sn=%s", orderSN)
	url := cfg.buildURL(path, timestamp, sign, extra)

	resp, err := http.Get(url)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to call Shopee API: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ShopeeTrackingResponse
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

	// Update awb_no di outbound_header
	c.DB.Model(&struct{ AwbNo string }{}).
		Table("outbound_headers").
		Where("shipment_id = ?", orderSN).
		Update("awb_no", result.Response.TrackingNumber)

	return ctx.JSON(fiber.Map{
		"success":         true,
		"tracking_number": result.Response.TrackingNumber,
		"plp_number":      result.Response.PlpNumber,
		"order_sn":        orderSN,
	})
}

// ============================================================
// GET SHIPPING LABEL URL
// GET /api/shopee/label/:order_sn
// ============================================================

func (c *ShopeeSyncController) GetShippingLabel(ctx *fiber.Ctx) error {
	orderSN := ctx.Params("order_sn")
	if orderSN == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "order_sn is required",
		})
	}

	cfg := loadShopeeConfig()

	// Step 1: Create shipping document
	createPath := "/api/v2/logistics/create_shipping_document"
	createTimestamp := time.Now().Unix()
	createSign := cfg.sign(createPath, createTimestamp)
	createURL := cfg.buildURL(createPath, createTimestamp, createSign, "")

	createPayload := map[string]interface{}{
		"order_list": []map[string]interface{}{
			{"order_sn": orderSN},
		},
	}

	createBody, _ := json.Marshal(createPayload)
	createResp, err := http.Post(createURL, "application/json",
		bytes.NewBuffer(createBody))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create shipping document: " + err.Error(),
		})
	}
	defer createResp.Body.Close()

	// Tunggu sebentar agar dokumen ter-generate
	time.Sleep(2 * time.Second)

	// Step 2: Download shipping document
	downloadPath := "/api/v2/logistics/download_shipping_document"
	downloadTimestamp := time.Now().Unix()
	downloadSign := cfg.sign(downloadPath, downloadTimestamp)
	downloadURL := cfg.buildURL(downloadPath, downloadTimestamp, downloadSign, "")

	downloadPayload := map[string]interface{}{
		"order_list": []map[string]interface{}{
			{"order_sn": orderSN},
		},
		"shipping_document_type": "NORMAL_AIR_WAYBILL",
	}

	downloadBody, _ := json.Marshal(downloadPayload)
	downloadResp, err := http.Post(downloadURL, "application/json",
		bytes.NewBuffer(downloadBody))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to download shipping document: " + err.Error(),
		})
	}
	defer downloadResp.Body.Close()

	// Check content type — bisa jadi PDF binary atau JSON
	contentType := downloadResp.Header.Get("Content-Type")

	if contentType == "application/pdf" || contentType == "application/octet-stream" {
		// Return PDF langsung ke browser
		pdfBytes, _ := io.ReadAll(downloadResp.Body)
		ctx.Set("Content-Type", "application/pdf")
		ctx.Set("Content-Disposition", fmt.Sprintf("inline; filename=label_%s.pdf", orderSN))
		return ctx.Send(pdfBytes)
	}

	// Return JSON response
	respBody, _ := io.ReadAll(downloadResp.Body)
	var docResult ShopeeShippingDocResponse
	if err := json.Unmarshal(respBody, &docResult); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse document response: " + err.Error(),
		})
	}

	if docResult.Error != "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": docResult.Error + ": " + docResult.Message,
		})
	}

	// Kalau ada URL, redirect ke URL tersebut
	if docResult.Response.URL != "" {
		return ctx.Redirect(docResult.Response.URL)
	}

	return ctx.JSON(fiber.Map{
		"success":  true,
		"order_sn": orderSN,
		"response": docResult.Response,
	})
}
