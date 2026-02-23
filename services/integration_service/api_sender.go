package integration_service

import (
	"bytes"
	"encoding/json"
	"fiber-app/models/integration"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func sendViaAPI(conn integration.IntegrationConnection, eventData map[string]interface{}) error {
	if conn.URL == "" {
		return fmt.Errorf("API URL tidak dikonfigurasi")
	}

	method := conn.Method
	if method == "" {
		method = "POST"
	}

	timeout := conn.TimeoutSec
	if timeout == 0 {
		timeout = 30
	}

	// Build request body
	var bodyBytes []byte
	var err error

	if conn.BodyTemplate != "" {
		// Resolve placeholder di body template
		body := conn.BodyTemplate
		for k, v := range eventData {
			body = strings.ReplaceAll(body, "{{"+k+"}}", fmt.Sprintf("%v", v))
		}
		bodyBytes = []byte(body)
	} else {
		// Default: kirim seluruh eventData sebagai JSON
		bodyBytes, err = json.Marshal(eventData)
		if err != nil {
			return fmt.Errorf("gagal marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, conn.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("gagal buat HTTP request: %w", err)
	}

	// Default headers
	req.Header.Set("Content-Type", "application/json")

	// Custom headers dari konfigurasi
	if conn.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(conn.Headers), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	// Auth
	switch conn.AuthType {
	case "basic":
		// AuthValue format: "username:password"
		parts := strings.SplitN(conn.AuthValue, ":", 2)
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		}
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+conn.AuthValue)
	case "api_key":
		req.Header.Set("X-API-Key", conn.AuthValue)
	}

	// Send
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal kirim HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Cek response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API response %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
