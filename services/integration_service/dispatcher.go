package integration_service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fiber-app/models/integration"
	report_mailer "fiber-app/models/report_mailer"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ─── Main Entry Point ─────────────────────────────────────────────────────────

// Dispatch mencari semua integrasi aktif untuk eventKey lalu menjalankannya.
// Dipanggil dengan goroutine dari controller.
func Dispatch(db *gorm.DB, queryDB *gorm.DB, eventKey string, eventData map[string]interface{}) {
	// Tambah field standar
	eventData["year"] = fmt.Sprintf("%d", time.Now().Year())
	eventData["date"] = time.Now().Format("20060102")
	eventData["datetime"] = time.Now().Format("2006-01-02 15:04:05")

	// Cari semua integrasi aktif untuk event ini
	var integrations []integration.Integration
	if err := db.
		Preload("Connection").
		Preload("Recipients").
		Where("event_key = ? AND is_active = ? AND timing = ?", eventKey, true, integration.TimingRealtime).
		Find(&integrations).Error; err != nil {
		return
	}

	for _, intg := range integrations {
		err := runIntegration(db, queryDB, intg, eventData, "event")
		// logHistory(db, intg, eventKey, err, "event")
		logHistory(db, intg, eventKey, err, "event", eventData)
		sendNotification(db, intg, eventData, err)
	}
}

// ─── Run Single Integration ───────────────────────────────────────────────────

func runIntegration(
	db *gorm.DB,
	queryDB *gorm.DB,
	intg integration.Integration,
	eventData map[string]interface{},
	triggeredBy string,
) error {
	if intg.Connection == nil {
		return fmt.Errorf("connection belum dikonfigurasi")
	}

	// Untuk API — tidak perlu generate file
	if intg.ChannelType == integration.ChannelAPI {
		return sendViaAPI(*intg.Connection, eventData)
	}

	// Untuk channel lain — generate file dulu
	// Pilih DB yang sesuai: queryDB untuk source query, tidak perlu DB untuk source event
	activeDB := db
	if intg.SourceType == integration.SourceQuery {
		activeDB = queryDB
	}

	file, err := GenerateFile(activeDB, intg, eventData)
	if err != nil {
		return fmt.Errorf("gagal generate file: %w", err)
	}

	switch intg.ChannelType {
	case integration.ChannelSFTP:
		return sendViaSFTP(*intg.Connection, file)
	case integration.ChannelFTP:
		return sendViaFTP(*intg.Connection, file)
	case integration.ChannelFile:
		return sendViaFile(*intg.Connection, file)
	default:
		return fmt.Errorf("channel_type tidak dikenal: %s", intg.ChannelType)
	}
}

// ─── Scheduler Entry Point ────────────────────────────────────────────────────

// RunScheduled dijalankan oleh scheduler untuk integrasi bertipe scheduled.
func RunScheduled(db *gorm.DB, queryDB *gorm.DB, integrationID uint) {
	var intg integration.Integration
	if err := db.
		Preload("Connection").
		Preload("Recipients").
		First(&intg, integrationID).Error; err != nil {
		return
	}

	if !intg.IsActive {
		return
	}

	// Untuk scheduled, event data minimal
	eventData := map[string]interface{}{
		"year":     fmt.Sprintf("%d", time.Now().Year()),
		"date":     time.Now().Format("20060102"),
		"datetime": time.Now().Format("2006-01-02 15:04:05"),
	}

	err := runIntegration(db, queryDB, intg, eventData, "scheduler")
	// logHistory(db, intg, intg.EventKey, err, "scheduler")
	logHistory(db, intg, intg.EventKey, err, "scheduler", eventData)
	sendNotification(db, intg, eventData, err)

	// Update last_run_at
	now := time.Now()
	db.Model(&integration.Integration{}).
		Where("id = ?", integrationID).
		Update("last_run_at", now)
}

// ─── Email Notifikasi ─────────────────────────────────────────────────────────

func sendNotification(db *gorm.DB, intg integration.Integration, data map[string]interface{}, sendErr error) {
	// Cek apakah perlu kirim notif
	if sendErr == nil && !intg.NotifyOnSuccess {
		return
	}
	if sendErr != nil && !intg.NotifyOnFailure {
		return
	}
	if intg.EmailConfigID == nil || len(intg.Recipients) == 0 {
		return
	}

	// Load email config
	var emailConfig report_mailer.EmailConfig
	if err := db.First(&emailConfig, *intg.EmailConfigID).Error; err != nil {
		return
	}

	// Pisahkan To dan CC
	var toList, ccList []string
	for _, r := range intg.Recipients {
		if r.Type == integration.RecipientTO {
			toList = append(toList, r.Email)
		} else {
			ccList = append(ccList, r.Email)
		}
	}
	if len(toList) == 0 {
		return
	}

	// Subject
	subject := intg.NotifyEmailSubject
	if subject == "" {
		if sendErr == nil {
			subject = fmt.Sprintf("[WMS Integration] %s — Success", intg.Name)
		} else {
			subject = fmt.Sprintf("[WMS Integration] %s — Failed", intg.Name)
		}
	}

	// Body
	statusColor := "#065F46"
	statusLabel := "✓ Success"
	statusDetail := "Integration completed successfully."
	if sendErr != nil {
		statusColor = "#B91C1C"
		statusLabel = "✕ Failed"
		statusDetail = sendErr.Error()
	}

	bodyTemplate := intg.NotifyEmailBody
	if bodyTemplate == "" {
		bodyTemplate = fmt.Sprintf(`Dear Team,

<p>Integration <b>%s</b> has been executed with the following result:</p>

<div style="background:%s;color:#fff;padding:12px 20px;border-radius:8px;font-weight:bold;margin:16px 0;">
  %s
</div>

<table style="border-collapse:collapse;font-size:13px;margin-top:8px;">
  <tr><td style="color:#6b7280;padding:3px 16px 3px 0;">Integration</td><td><b>%s</b></td></tr>
  <tr><td style="color:#6b7280;padding:3px 16px 3px 0;">Event</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:3px 16px 3px 0;">Channel</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:3px 16px 3px 0;">Time</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:3px 16px 3px 0;vertical-align:top;">Detail</td>
    <td style="color:%s;">%s</td>
  </tr>
</table>`,
			intg.Name,
			statusColor, statusLabel,
			intg.Name,
			intg.EventKey,
			string(intg.ChannelType),
			time.Now().Format("02 January 2006, 15:04 WIB"),
			statusColor, statusDetail,
		)
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;padding:0;background:#F3F4F6;font-family:Arial,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" style="background:#F3F4F6;padding:32px 0;">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.08);">
<tr><td style="background:#1E3A5F;padding:24px 36px;">
  <div style="color:#fff;font-size:18px;font-weight:bold;">WMS Integration Hub</div>
  <div style="color:rgba(255,255,255,0.7);font-size:12px;margin-top:4px;">Automated Integration Notification</div>
</td></tr>
<tr><td style="padding:28px 36px;color:#374151;font-size:14px;line-height:1.7;">%s</td></tr>
<tr><td style="padding:0 36px;"><hr style="border:none;border-top:1px solid #E5E7EB;"></td></tr>
<tr><td style="padding:16px 36px;color:#9CA3AF;font-size:12px;">
  © %d Warehouse Management System. This is an automated message.
</td></tr>
</table></td></tr></table>
</body></html>`, bodyTemplate, time.Now().Year())

	// Kirim email
	go sendEmail(emailConfig, toList, ccList, subject, htmlBody)
}

// ─── Email Sender ─────────────────────────────────────────────────────────────

func sendEmail(config report_mailer.EmailConfig, toList, ccList []string, subject, htmlBody string) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail),
		"To":           strings.Join(toList, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/mixed; boundary=%s", writer.Boundary()),
		"Date":         time.Now().Format(time.RFC1123Z),
	}
	if len(ccList) > 0 {
		headers["Cc"] = strings.Join(ccList, ", ")
	}
	for k, v := range headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(&buf, "\r\n")

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, _ := writer.CreatePart(htmlHeader)
	qp := quotedprintable.NewWriter(htmlPart)
	qp.Write([]byte(htmlBody))
	qp.Close()
	writer.Close()
	_ = base64.StdEncoding

	raw := buf.Bytes()
	allRecipients := append(toList, ccList...)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		tlsConf := &tls.Config{InsecureSkipVerify: false, ServerName: config.Host}
		conn, err := tls.Dial("tcp", addr, tlsConf)
		if err != nil {
			return
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, config.Host)
		if err != nil {
			return
		}
		defer client.Close()
		client.Auth(auth)
		client.Mail(config.FromEmail)
		for _, r := range allRecipients {
			client.Rcpt(r)
		}
		w, _ := client.Data()
		w.Write(raw)
		w.Close()
	} else {
		smtp.SendMail(addr, auth, config.FromEmail, allRecipients, raw)
	}
}

// ─── History Logger ───────────────────────────────────────────────────────────

// func logHistory(db *gorm.DB, intg integration.Integration, eventKey string, sendErr error, triggeredBy string) {
func logHistory(db *gorm.DB, intg integration.Integration, eventKey string, sendErr error, triggeredBy string, eventData map[string]interface{}) {
	payloadBytes, _ := json.Marshal(eventData)
	h := integration.IntegrationHistory{
		IntegrationID:  intg.ID,
		EventKey:       eventKey,
		ChannelType:    intg.ChannelType,
		TriggeredBy:    triggeredBy,
		PayloadSummary: string(payloadBytes),
		SentAt:         time.Now(),
	}
	if sendErr != nil {
		h.Status = integration.StatusFailed
		h.Message = sendErr.Error()
	} else {
		h.Status = integration.StatusSuccess
		h.Message = "Integration completed successfully"
	}
	db.Create(&h)
}
