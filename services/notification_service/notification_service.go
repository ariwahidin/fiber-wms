package notification_service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fiber-app/models/notification"
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

// SendNotification mencari semua notifikasi aktif untuk eventKey,
// lalu mengirim email ke masing-masing penerima.
// Dipanggil dengan goroutine dari controller supaya tidak memblok response.
func SendNotification(db *gorm.DB, eventKey string, data map[string]interface{}) {
	// Tambah {{year}} otomatis
	data["year"] = fmt.Sprintf("%d", time.Now().Year())

	// Cari semua notifikasi aktif untuk event ini
	var notifs []notification.EmailNotification
	if err := db.
		Preload("EmailConfig").
		Preload("Recipients").
		Where("event_key = ? AND is_active = ?", eventKey, true).
		Find(&notifs).Error; err != nil {
		logHistory(db, 0, eventKey, data, fmt.Errorf("gagal query notifikasi: %w", err))
		return
	}

	if len(notifs) == 0 {
		return // tidak ada notifikasi aktif untuk event ini
	}

	for _, notif := range notifs {
		sendErr := sendOne(db, notif, data)
		logHistory(db, notif.ID, eventKey, data, sendErr)
	}
}

// ─── Send One Notification ────────────────────────────────────────────────────

func sendOne(db *gorm.DB, notif notification.EmailNotification, data map[string]interface{}) error {
	if notif.EmailConfig.ID == 0 {
		return fmt.Errorf("email config tidak ditemukan")
	}
	if len(notif.Recipients) == 0 {
		return fmt.Errorf("tidak ada penerima email")
	}

	// Pisahkan To dan CC
	var toList, ccList []string
	for _, r := range notif.Recipients {
		if r.Type == notification.RecipientTO {
			toList = append(toList, r.Email)
		} else {
			ccList = append(ccList, r.Email)
		}
	}
	if len(toList) == 0 {
		return fmt.Errorf("tidak ada penerima TO")
	}

	subject := resolvePlaceholders(notif.EmailSubject, data)
	body := buildBody(notif, data)

	return sendEmail(notif.EmailConfig, toList, ccList, subject, body)
}

// ─── Template Engine ──────────────────────────────────────────────────────────

func resolvePlaceholders(text string, data map[string]interface{}) string {
	result := text
	for key, val := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		case time.Time:
			strVal = v.Format("02 January 2006, 15:04 WIB")
		case int, int64, uint, uint64, float64:
			strVal = fmt.Sprintf("%v", v)
		case nil:
			strVal = "-"
		default:
			strVal = fmt.Sprintf("%v", v)
		}
		result = strings.ReplaceAll(result, placeholder, strVal)
	}
	return result
}

func buildBody(notif notification.EmailNotification, data map[string]interface{}) string {
	headerColor := notif.HeaderColor
	if headerColor == "" {
		headerColor = "#1E40AF"
	}

	header := resolvePlaceholders(notif.EmailHeader, data)
	body := resolvePlaceholders(notif.EmailBody, data)
	footer := resolvePlaceholders(notif.EmailFooter, data)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#F3F4F6;font-family:Arial,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#F3F4F6;padding:32px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.08);">
        <tr>
          <td style="background:%s;padding:28px 36px;">
            <div style="color:#ffffff;font-size:20px;font-weight:bold;line-height:1.3;">%s</div>
          </td>
        </tr>
        <tr>
          <td style="padding:32px 36px;color:#374151;font-size:14px;line-height:1.7;">%s</td>
        </tr>
        <tr>
          <td style="padding:0 36px;"><hr style="border:none;border-top:1px solid #E5E7EB;margin:0;"></td>
        </tr>
        <tr>
          <td style="padding:20px 36px;color:#9CA3AF;font-size:12px;line-height:1.6;">%s</td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, headerColor, header, body, footer)
}

// ─── Email Sender ─────────────────────────────────────────────────────────────

func sendEmail(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
) error {
	raw, err := buildMIMEMessage(config, toList, ccList, subject, htmlBody)
	if err != nil {
		return fmt.Errorf("gagal build MIME: %w", err)
	}

	allRecipients := append(toList, ccList...)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return sendViaTLS(addr, config.Host, auth, config.FromEmail, allRecipients, raw)
	}
	return smtp.SendMail(addr, auth, config.FromEmail, allRecipients, raw)
}

func buildMIMEMessage(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
) ([]byte, error) {
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
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	qpWriter := quotedprintable.NewWriter(htmlPart)
	qpWriter.Write([]byte(htmlBody))
	qpWriter.Close()

	writer.Close()
	_ = base64.StdEncoding // imported for future attachment support
	return buf.Bytes(), nil
}

func sendViaTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{InsecureSkipVerify: false, ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial error: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client error: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth error: %w", err)
	}
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM error: %w", err)
	}
	for _, r := range to {
		if err = client.Rcpt(r); err != nil {
			return fmt.Errorf("SMTP RCPT TO (%s) error: %w", r, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA error: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write error: %w", err)
	}
	return w.Close()
}

// ─── History Logger ───────────────────────────────────────────────────────────

func logHistory(db *gorm.DB, notifID uint, eventKey string, data map[string]interface{}, sendErr error) {
	payloadBytes, _ := json.Marshal(data)

	history := notification.NotificationHistory{
		NotificationID: notifID,
		EventKey:       eventKey,
		Payload:        string(payloadBytes),
		SentAt:         time.Now(),
	}

	if sendErr != nil {
		history.Status = notification.NotifStatusFailed
		history.Message = sendErr.Error()
	} else {
		history.Status = notification.NotifStatusSuccess
		history.Message = "Notification sent successfully"
	}

	db.Create(&history)
}
