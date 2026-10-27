package mobiles

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func (c *MobileOutboundController) startCCTVRecording(outboundNo string) {
	go func() {
		client := &http.Client{
			Timeout: 3 * time.Second,
		}

		payload := map[string]string{
			"outbound_no": outboundNo,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[CCTV] Failed to create payload for %s: %v", outboundNo, err)
			return
		}

		req, err := http.NewRequest(
			http.MethodPost,
			"http://127.0.0.1:8095/recording/start",
			bytes.NewBuffer(body),
		)
		if err != nil {
			log.Printf("[CCTV] Failed to create request for %s: %v", outboundNo, err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[CCTV] Recording start failed for %s: %v", outboundNo, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf(
				"[CCTV] Recording start returned HTTP %d for %s",
				resp.StatusCode,
				outboundNo,
			)
			return
		}

		log.Printf("[CCTV] Recording started for outbound %s", outboundNo)
	}()
}

func (c *MobileOutboundController) stopCCTVRecording(outboundNo string) {
	go func() {
		client := &http.Client{
			Timeout: 3 * time.Second,
		}

		payload := map[string]string{
			"outbound_no": outboundNo,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[CCTV] Stop payload failed for %s: %v", outboundNo, err)
			return
		}

		req, err := http.NewRequest(
			http.MethodPost,
			"http://127.0.0.1:8095/recording/stop",
			bytes.NewBuffer(body),
		)
		if err != nil {
			log.Printf("[CCTV] Stop request failed for %s: %v", outboundNo, err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[CCTV] Stop recording failed for %s: %v", outboundNo, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("[CCTV] Stop recording returned HTTP %d for %s",
				resp.StatusCode, outboundNo)
			return
		}

		log.Printf("[CCTV] Recording stopped for outbound %s", outboundNo)
	}()
}
