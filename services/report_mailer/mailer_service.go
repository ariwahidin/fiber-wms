// package services

// import (
// 	"bytes"
// 	"crypto/tls"
// 	"encoding/base64"
// 	"fiber-app/models/report_mailer"
// 	"fmt"
// 	"mime/multipart"
// 	"mime/quotedprintable"
// 	"net/smtp"
// 	"net/textproto"
// 	"strings"
// 	"time"

// 	"gorm.io/gorm"
// )

// // ─── Main Entry Point ─────────────────────────────────────────────────────────

// // SendReport adalah fungsi utama: generate Excel lalu kirim email.
// // Dipanggil oleh scheduler maupun trigger manual.
// // func SendReport(db *gorm.DB, report report_mailer.Report) error {

// func SendReport(db *gorm.DB, queryDB *gorm.DB, report report_mailer.Report) error {
// 	// 1. Load email config + recipients kalau belum di-preload
// 	if report.EmailConfig.ID == 0 {
// 		if err := db.First(&report.EmailConfig, report.EmailConfigID).Error; err != nil {
// 			return fmt.Errorf("email config tidak ditemukan: %w", err)
// 		}
// 	}
// 	if len(report.Recipients) == 0 {
// 		if err := db.Where("report_id = ?", report.ID).Find(&report.Recipients).Error; err != nil {
// 			return fmt.Errorf("gagal load recipients: %w", err)
// 		}
// 	}

// 	if len(report.Recipients) == 0 {
// 		return fmt.Errorf("report tidak memiliki penerima email")
// 	}

// 	// 2. Generate Excel
// 	// excelBytes, err := GenerateReportExcel(db, report)
// 	excelBytes, err := GenerateReportExcel(queryDB, report)
// 	if err != nil {
// 		return fmt.Errorf("gagal generate Excel: %w", err)
// 	}

// 	fileName := ExcelFileName(report.Name)

// 	// 3. Pisahkan To dan CC
// 	var toList, ccList []string
// 	for _, r := range report.Recipients {
// 		if r.Type == report_mailer.RecipientTO {
// 			toList = append(toList, r.Email)
// 		} else {
// 			ccList = append(ccList, r.Email)
// 		}
// 	}

// 	if len(toList) == 0 {
// 		return fmt.Errorf("tidak ada penerima TO, minimal 1 penerima TO diperlukan")
// 	}

// 	// 4. Kirim email
// 	subject := fmt.Sprintf("[WMS Report] %s - %s", report.Name, time.Now().Format("02 Jan 2006"))
// 	body := buildEmailBody(report)

// 	return sendEmailWithAttachment(report.EmailConfig, toList, ccList, subject, body, fileName, excelBytes)
// }

// // ─── Email Body ───────────────────────────────────────────────────────────────

// func buildEmailBody(report report_mailer.Report) string {
// 	now := time.Now()
// 	return fmt.Sprintf(`Halo,

// Terlampir adalah laporan <b>%s</b> yang digenerate otomatis oleh sistem WMS.

// <table style="border-collapse:collapse;font-size:13px;margin-top:8px;">
//   <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Nama Report</td><td><b>%s</b></td></tr>
//   <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Deskripsi</td><td>%s</td></tr>
//   <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Generated</td><td>%s</td></tr>
// </table>

// <br>
// <span style="color:#9ca3af;font-size:12px">Email ini dikirim otomatis oleh WMS Report Mailer. Mohon tidak membalas email ini.</span>`,
// 		report.Name,
// 		report.Name,
// 		report.Description,
// 		now.Format("02 January 2006, 15:04 WIB"),
// 	)
// }

// // ─── Core Send Function ───────────────────────────────────────────────────────

// func sendEmailWithAttachment(
// 	config report_mailer.EmailConfig,
// 	toList []string,
// 	ccList []string,
// 	subject string,
// 	htmlBody string,
// 	attachmentName string,
// 	attachmentData []byte,
// ) error {
// 	// Build raw MIME message
// 	raw, err := buildMIMEMessage(config, toList, ccList, subject, htmlBody, attachmentName, attachmentData)
// 	if err != nil {
// 		return fmt.Errorf("gagal build MIME message: %w", err)
// 	}

// 	// Semua penerima (To + CC) untuk SMTP RCPT TO
// 	allRecipients := append(toList, ccList...)

// 	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
// 	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

// 	if config.UseTLS {
// 		return sendViaTLS(addr, config.Host, auth, config.FromEmail, allRecipients, raw)
// 	}

// 	return smtp.SendMail(addr, auth, config.FromEmail, allRecipients, raw)
// }

// // ─── MIME Builder ─────────────────────────────────────────────────────────────

// func buildMIMEMessage(
// 	config report_mailer.EmailConfig,
// 	toList []string,
// 	ccList []string,
// 	subject string,
// 	htmlBody string,
// 	attachmentName string,
// 	attachmentData []byte,
// ) ([]byte, error) {
// 	var buf bytes.Buffer
// 	writer := multipart.NewWriter(&buf)

// 	// ── Headers utama ──────────────────────────────────────────────────────────
// 	headers := map[string]string{
// 		"From":         fmt.Sprintf("%s <%s>", config.FromName, config.FromEmail),
// 		"To":           strings.Join(toList, ", "),
// 		"Subject":      subject,
// 		"MIME-Version": "1.0",
// 		"Content-Type": fmt.Sprintf("multipart/mixed; boundary=%s", writer.Boundary()),
// 		"Date":         time.Now().Format(time.RFC1123Z),
// 	}
// 	if len(ccList) > 0 {
// 		headers["Cc"] = strings.Join(ccList, ", ")
// 	}

// 	for k, v := range headers {
// 		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
// 	}
// 	fmt.Fprintf(&buf, "\r\n")

// 	// ── Part 1: HTML body ──────────────────────────────────────────────────────
// 	htmlHeader := textproto.MIMEHeader{}
// 	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
// 	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")

// 	htmlPart, err := writer.CreatePart(htmlHeader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	qpWriter := quotedprintable.NewWriter(htmlPart)
// 	if _, err := qpWriter.Write([]byte(htmlBody)); err != nil {
// 		return nil, err
// 	}
// 	qpWriter.Close()

// 	// ── Part 2: Excel attachment ───────────────────────────────────────────────
// 	attachHeader := textproto.MIMEHeader{}
// 	attachHeader.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
// 	attachHeader.Set("Content-Transfer-Encoding", "base64")
// 	attachHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachmentName))

// 	attachPart, err := writer.CreatePart(attachHeader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	encoded := base64.StdEncoding.EncodeToString(attachmentData)
// 	// Wrap base64 per 76 karakter (standar MIME)
// 	for i := 0; i < len(encoded); i += 76 {
// 		end := i + 76
// 		if end > len(encoded) {
// 			end = len(encoded)
// 		}
// 		fmt.Fprintf(attachPart, "%s\r\n", encoded[i:end])
// 	}

// 	writer.Close()

// 	return buf.Bytes(), nil
// }

// // ─── TLS Sender ───────────────────────────────────────────────────────────────

// func sendViaTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
// 	tlsConfig := &tls.Config{
// 		InsecureSkipVerify: false,
// 		ServerName:         host,
// 	}

// 	conn, err := tls.Dial("tcp", addr, tlsConfig)
// 	if err != nil {
// 		return fmt.Errorf("TLS dial error: %w", err)
// 	}
// 	defer conn.Close()

// 	client, err := smtp.NewClient(conn, host)
// 	if err != nil {
// 		return fmt.Errorf("SMTP client error: %w", err)
// 	}
// 	defer client.Close()

// 	if err = client.Auth(auth); err != nil {
// 		return fmt.Errorf("SMTP auth error: %w", err)
// 	}

// 	if err = client.Mail(from); err != nil {
// 		return fmt.Errorf("SMTP MAIL FROM error: %w", err)
// 	}

// 	for _, recipient := range to {
// 		if err = client.Rcpt(recipient); err != nil {
// 			return fmt.Errorf("SMTP RCPT TO (%s) error: %w", recipient, err)
// 		}
// 	}

// 	w, err := client.Data()
// 	if err != nil {
// 		return fmt.Errorf("SMTP DATA error: %w", err)
// 	}

// 	if _, err = w.Write(msg); err != nil {
// 		return fmt.Errorf("SMTP write error: %w", err)
// 	}

// 	return w.Close()
// }

// // ─── Send History Logger ──────────────────────────────────────────────────────

// // LogSendHistory menyimpan hasil pengiriman ke tabel send_history.
// // Dipanggil setelah SendReport selesai (sukses maupun gagal).
// func LogSendHistory(db *gorm.DB, reportID uint, triggeredBy string, sendErr error) {
// 	history := report_mailer.SendHistory{
// 		ReportID:    reportID,
// 		SentAt:      time.Now(),
// 		TriggeredBy: triggeredBy,
// 	}

// 	if sendErr != nil {
// 		history.Status = report_mailer.SendStatusFailed
// 		history.Message = sendErr.Error()
// 	} else {
// 		history.Status = report_mailer.SendStatusSuccess
// 		history.Message = "Email berhasil dikirim"
// 	}

// 	db.Create(&history)
// }

package services

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fiber-app/models/report_mailer"
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

func SendReport(db *gorm.DB, queryDB *gorm.DB, report report_mailer.Report) error {
	// Load relasi kalau belum ada
	if report.EmailConfig.ID == 0 {
		if err := db.First(&report.EmailConfig, report.EmailConfigID).Error; err != nil {
			return fmt.Errorf("email config tidak ditemukan: %w", err)
		}
	}
	if len(report.Recipients) == 0 {
		if err := db.Where("report_id = ?", report.ID).Find(&report.Recipients).Error; err != nil {
			return fmt.Errorf("gagal load recipients: %w", err)
		}
	}
	if len(report.Queries) == 0 {
		if err := db.Where("report_id = ?", report.ID).Order("sort_order asc").Find(&report.Queries).Error; err != nil {
			return fmt.Errorf("gagal load queries: %w", err)
		}
	}

	if len(report.Recipients) == 0 {
		return fmt.Errorf("report tidak memiliki penerima email")
	}
	if len(report.Queries) == 0 {
		return fmt.Errorf("report tidak memiliki query")
	}

	// Pisahkan To dan CC
	var toList, ccList []string
	for _, r := range report.Recipients {
		if r.Type == report_mailer.RecipientTO {
			toList = append(toList, r.Email)
		} else {
			ccList = append(ccList, r.Email)
		}
	}
	if len(toList) == 0 {
		return fmt.Errorf("tidak ada penerima TO")
	}

	subject := fmt.Sprintf("[WMS Report] %s - %s", report.Name, time.Now().Format("02 Jan 2006"))
	body := buildEmailBody(report)

	// Generate Excel sesuai output mode
	switch report.OutputMode {
	case report_mailer.OutputModeMultiFile:
		// Banyak file Excel dalam 1 email
		files, err := GenerateMultiFileExcel(queryDB, report)
		if err != nil {
			return fmt.Errorf("gagal generate Excel: %w", err)
		}
		attachments := make([]Attachment, len(files))
		for i, f := range files {
			attachments[i] = Attachment{Name: f.FileName, Data: f.Data}
		}
		return sendEmailWithAttachments(report.EmailConfig, toList, ccList, subject, body, attachments)

	default:
		// Single file multi sheet
		excelBytes, err := GenerateSingleFileExcel(queryDB, report)
		if err != nil {
			return fmt.Errorf("gagal generate Excel: %w", err)
		}
		attachments := []Attachment{
			{Name: ExcelFileName(report.Name), Data: excelBytes},
		}
		return sendEmailWithAttachments(report.EmailConfig, toList, ccList, subject, body, attachments)
	}
}

// ─── Attachment Type ──────────────────────────────────────────────────────────

type Attachment struct {
	Name string
	Data []byte
}

// ─── Email Body ───────────────────────────────────────────────────────────────

func buildEmailBody(report report_mailer.Report) string {
	now := time.Now()

	// Daftar query / sheet
	queryList := ""
	for i, rq := range report.Queries {
		queryList += fmt.Sprintf("<tr><td style='color:#6B7280;padding:2px 12px 2px 0'>%d.</td><td>%s</td></tr>", i+1, rq.Name)
	}

	outputLabel := "1 file Excel (multi-sheet)"
	if report.OutputMode == report_mailer.OutputModeMultiFile {
		outputLabel = fmt.Sprintf("%d file Excel terpisah", len(report.Queries))
	}

	return fmt.Sprintf(`Halo,

Terlampir adalah laporan <b>%s</b> yang digenerate otomatis oleh sistem WMS.

<table style="border-collapse:collapse;font-size:13px;margin-top:8px;">
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Nama Report</td><td><b>%s</b></td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Output</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Generated</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0;vertical-align:top">Sheet / File</td>
    <td><table>%s</table></td>
  </tr>
</table>

<br>
<span style="color:#9ca3af;font-size:12px">Email ini dikirim otomatis oleh WMS Report Mailer. Mohon tidak membalas email ini.</span>`,
		report.Name,
		report.Name,
		outputLabel,
		now.Format("02 January 2006, 15:04 WIB"),
		queryList,
	)
}

// ─── Core Send Function ───────────────────────────────────────────────────────

func sendEmailWithAttachments(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
	attachments []Attachment,
) error {
	raw, err := buildMIMEMessage(config, toList, ccList, subject, htmlBody, attachments)
	if err != nil {
		return fmt.Errorf("gagal build MIME message: %w", err)
	}

	allRecipients := append(toList, ccList...)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return sendViaTLS(addr, config.Host, auth, config.FromEmail, allRecipients, raw)
	}
	return smtp.SendMail(addr, auth, config.FromEmail, allRecipients, raw)
}

// ─── MIME Builder ─────────────────────────────────────────────────────────────

func buildMIMEMessage(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
	attachments []Attachment,
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

	// HTML body
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

	// Attachments (bisa lebih dari 1)
	for _, att := range attachments {
		if att.Name == "" || att.Data == nil {
			continue
		}
		attachHeader := textproto.MIMEHeader{}
		attachHeader.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		attachHeader.Set("Content-Transfer-Encoding", "base64")
		attachHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, att.Name))

		attachPart, err := writer.CreatePart(attachHeader)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(att.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			fmt.Fprintf(attachPart, "%s\r\n", encoded[i:end])
		}
	}

	writer.Close()
	return buf.Bytes(), nil
}

// ─── TLS Sender ───────────────────────────────────────────────────────────────

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
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO (%s) error: %w", recipient, err)
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

// ─── Send History Logger ──────────────────────────────────────────────────────

func LogSendHistory(db *gorm.DB, reportID uint, triggeredBy string, sendErr error) {
	history := report_mailer.SendHistory{
		ReportID:    reportID,
		SentAt:      time.Now(),
		TriggeredBy: triggeredBy,
	}
	if sendErr != nil {
		history.Status = report_mailer.SendStatusFailed
		history.Message = sendErr.Error()
	} else {
		history.Status = report_mailer.SendStatusSuccess
		history.Message = "Email berhasil dikirim"
	}
	db.Create(&history)
}

// TriggerNow menjalankan report sekarang (manual).
func TriggerNow(db *gorm.DB, queryDB *gorm.DB, reportID uint) error {
	var report report_mailer.Report
	if err := db.
		Preload("EmailConfig").
		Preload("Recipients").
		Preload("Queries", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order asc")
		}).
		First(&report, reportID).Error; err != nil {
		return fmt.Errorf("report tidak ditemukan: %w", err)
	}
	sendErr := SendReport(db, queryDB, report)
	LogSendHistory(db, reportID, "manual", sendErr)
	return sendErr
}
