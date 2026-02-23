package integration_service

import (
	"encoding/json"
	"fiber-app/models/integration"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ─── Push Entry Point (dari API eksternal) ────────────────────────────────────

// DispatchInboundPush dipanggil ketika eksternal POST ke endpoint WMS.
// data = body JSON yang sudah di-parse dari request
func DispatchInboundPush(db *gorm.DB, eventKey string, data map[string]interface{}, userID int) ProcessResult {
	var integrations []integration.Integration
	if err := db.
		Preload("Connection").
		Preload("Recipients").
		Where("event_key = ? AND is_active = ? AND direction = ? AND channel_type = ?",
			eventKey, true, integration.DirectionInbound, integration.ChannelAPI).
		Find(&integrations).Error; err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal query integrasi", Detail: err.Error()}},
		}
	}

	if len(integrations) == 0 {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Tidak ada integrasi aktif untuk event: " + eventKey}},
		}
	}

	var overall ProcessResult
	for _, intg := range integrations {
		// Konversi data ke []ParsedRow
		rows := []ParsedRow{ParsedRow(data)}

		// Kalau ada field "items" atau "orders" berbentuk array, expand
		if items, ok := data["items"]; ok {
			if arr, ok := items.([]interface{}); ok {
				rows = expandArrayField(data, arr)
			}
		} else if orders, ok := data["orders"]; ok {
			if arr, ok := orders.([]interface{}); ok {
				rows = expandArrayField(data, arr)
			}
		}

		result := ProcessInboundRows(db, intg, rows, userID)
		mergeResult(&overall, result)

		// Log history
		payloadBytes, _ := json.Marshal(data)
		logInboundHistory(db, intg, eventKey, string(payloadBytes), result, "push")

		// Notif email
		sendNotification(db, intg, data, errorFromResult(result))
	}

	return overall
}

// ─── Pull Manual / Scheduler ─────────────────────────────────────────────────

// DispatchInboundPull dipanggil oleh scheduler atau manual trigger dari UI
func DispatchInboundPull(db *gorm.DB, integrationID uint, userID int) ProcessResult {
	var intg integration.Integration
	if err := db.Preload("Connection").Preload("Recipients").First(&intg, integrationID).Error; err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Integrasi tidak ditemukan"}},
		}
	}

	if !intg.IsActive {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Integrasi tidak aktif"}},
		}
	}

	result := PullAndProcess(db, intg, userID)

	// Log history
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"triggered_by": "pull",
		"integration":  intg.Name,
		"outbound_nos": result.OutboundNos,
	})
	logInboundHistory(db, intg, intg.EventKey, string(payloadBytes), result, "pull")

	// Update last_run_at
	now := time.Now()
	db.Model(&integration.Integration{}).Where("id = ?", integrationID).Update("last_run_at", now)

	// Notif email
	sendNotification(db, intg, map[string]interface{}{
		"integration": intg.Name,
		"success":     result.SuccessCount,
		"failed":      result.FailedCount,
	}, errorFromResult(result))

	return result
}

// ─── Retrigger dari History ───────────────────────────────────────────────────

// RetriggerInbound mengulang proses inbound dari payload yang tersimpan di history
func RetriggerInbound(db *gorm.DB, intg integration.Integration, historyPayload string, userID int) ProcessResult {
	// Parse payload dari history
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(historyPayload), &data); err != nil {
		return ProcessResult{
			FailedCount: 1,
			Errors:      []ProcessError{{Row: 0, Message: "Gagal parse payload history", Detail: err.Error()}},
		}
	}

	rows := []ParsedRow{ParsedRow(data)}
	if items, ok := data["items"]; ok {
		if arr, ok := items.([]interface{}); ok {
			rows = expandArrayField(data, arr)
		}
	}

	result := ProcessInboundRows(db, intg, rows, userID)

	// Log history baru
	payloadBytes, _ := json.Marshal(data)
	logInboundHistory(db, intg, intg.EventKey, string(payloadBytes), result, "retrigger")

	sendNotification(db, intg, data, errorFromResult(result))

	return result
}

// ─── Log History ──────────────────────────────────────────────────────────────

func logInboundHistory(
	db *gorm.DB,
	intg integration.Integration,
	eventKey string,
	payload string,
	result ProcessResult,
	triggeredBy string,
) {
	status := integration.StatusSuccess
	message := fmt.Sprintf("Berhasil buat %d outbound: %v", result.SuccessCount, result.OutboundNos)

	if result.FailedCount > 0 {
		status = integration.StatusFailed
		errMsgs := []string{}
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, e.Message+": "+e.Detail)
		}
		message = fmt.Sprintf("Gagal: %d, Sukses: %d. Error: %v", result.FailedCount, result.SuccessCount, errMsgs)
	}

	h := integration.IntegrationHistory{
		IntegrationID:  intg.ID,
		EventKey:       eventKey,
		ChannelType:    intg.ChannelType,
		Status:         status,
		Message:        message,
		PayloadSummary: payload,
		TriggeredBy:    triggeredBy,
		SentAt:         time.Now(),
	}
	db.Create(&h)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// expandArrayField: gabungkan field header dengan tiap item di array
func expandArrayField(header map[string]interface{}, items []interface{}) []ParsedRow {
	var rows []ParsedRow
	for _, item := range items {
		row := make(ParsedRow)
		// Copy header fields dulu
		for k, v := range header {
			if k != "items" && k != "orders" {
				row[k] = v
			}
		}
		// Overlay dengan item fields
		if m, ok := item.(map[string]interface{}); ok {
			for k, v := range m {
				row[k] = v
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func errorFromResult(result ProcessResult) error {
	if result.FailedCount > 0 && len(result.Errors) > 0 {
		return fmt.Errorf("%s", result.Errors[0].Message)
	}
	return nil
}
