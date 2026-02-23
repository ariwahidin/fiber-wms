package notification_service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	report_mailer "fiber-app/models/report_mailer"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// ─── Plain Email (tanpa attachment) ──────────────────────────────────────────

func sendEmailPlain(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
) error {
	raw, err := buildMIMEMessage(config, toList, ccList, subject, htmlBody, "", nil)
	if err != nil {
		return fmt.Errorf("gagal build MIME: %w", err)
	}
	return deliver(config, toList, ccList, raw)
}

// ─── Email dengan Excel Attachment ───────────────────────────────────────────

func sendEmailWithAttachment(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
	attachmentName string,
	attachmentData []byte,
) error {
	raw, err := buildMIMEMessage(config, toList, ccList, subject, htmlBody, attachmentName, attachmentData)
	if err != nil {
		return fmt.Errorf("gagal build MIME dengan attachment: %w", err)
	}
	return deliver(config, toList, ccList, raw)
}

// ─── MIME Builder ─────────────────────────────────────────────────────────────

func buildMIMEMessage(
	config report_mailer.EmailConfig,
	toList []string,
	ccList []string,
	subject string,
	htmlBody string,
	attachmentName string,
	attachmentData []byte,
) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Headers
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

	// HTML part
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	qp := quotedprintable.NewWriter(htmlPart)
	qp.Write([]byte(htmlBody))
	qp.Close()

	// Attachment part (kalau ada)
	if attachmentName != "" && len(attachmentData) > 0 {
		attachHeader := textproto.MIMEHeader{}
		attachHeader.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		attachHeader.Set("Content-Transfer-Encoding", "base64")
		attachHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachmentName))

		attachPart, err := writer.CreatePart(attachHeader)
		if err != nil {
			return nil, err
		}

		// Encode base64 dengan line wrap 76 chars
		encoded := base64.StdEncoding.EncodeToString(attachmentData)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			attachPart.Write([]byte(encoded[i:end] + "\r\n"))
		}
	}

	writer.Close()
	return buf.Bytes(), nil
}

// ─── SMTP Deliver ─────────────────────────────────────────────────────────────

func deliver(config report_mailer.EmailConfig, toList, ccList []string, raw []byte) error {
	allRecipients := append(toList, ccList...)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return deliverTLS(addr, config.Host, auth, config.FromEmail, allRecipients, raw)
	}
	return smtp.SendMail(addr, auth, config.FromEmail, allRecipients, raw)
}

func deliverTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
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
