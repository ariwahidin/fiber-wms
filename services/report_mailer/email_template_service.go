package services

import (
	"fiber-app/models/report_mailer"
	"fmt"
	"strings"
	"time"
)

// ─── Placeholder Replacer ─────────────────────────────────────────────────────

func resolvePlaceholders(text string, report report_mailer.Report) string {
	now := time.Now()

	outputLabel := "1 Excel file (multi-sheet)"
	if report.OutputMode == report_mailer.OutputModeMultiFile {
		outputLabel = fmt.Sprintf("%d Excel files", len(report.Queries))
	}

	// Daftar sheet/file dalam format HTML list
	sheetListHTML := "<ul style='margin:4px 0;padding-left:18px;'>"
	for _, rq := range report.Queries {
		sheetListHTML += fmt.Sprintf("<li>%s</li>", rq.Name)
	}
	sheetListHTML += "</ul>"

	// Daftar sheet/file plain text (untuk subject)
	sheetListPlain := ""
	for i, rq := range report.Queries {
		if i > 0 {
			sheetListPlain += ", "
		}
		sheetListPlain += rq.Name
	}

	replacer := strings.NewReplacer(
		"{{report_name}}", report.Name,
		"{{description}}", report.Description,
		"{{generated_date}}", now.Format("02 January 2006, 15:04 WIB"),
		"{{generated_date_short}}", now.Format("02 Jan 2006"),
		"{{output_label}}", outputLabel,
		"{{sheet_list}}", sheetListHTML,
		"{{sheet_list_plain}}", sheetListPlain,
		"{{year}}", now.Format("2006"),
	)

	return replacer.Replace(text)
}

// ─── Default Templates ────────────────────────────────────────────────────────

func defaultSubject(report report_mailer.Report) string {
	return fmt.Sprintf("[WMS Report] %s - %s", report.Name, time.Now().Format("02 Jan 2006"))
}

func defaultHeader(report report_mailer.Report) string {
	return fmt.Sprintf(`<b>%s</b>`, report.Name)
}

func defaultBody(report report_mailer.Report) string {
	now := time.Now()
	outputLabel := "1 Excel file (multi-sheet)"
	if report.OutputMode == report_mailer.OutputModeMultiFile {
		outputLabel = fmt.Sprintf("%d Excel files", len(report.Queries))
	}

	queryList := ""
	for i, rq := range report.Queries {
		queryList += fmt.Sprintf(
			"<tr><td style='color:#6B7280;padding:2px 12px 2px 0'>%d.</td><td>%s</td></tr>",
			i+1, rq.Name,
		)
	}

	return fmt.Sprintf(`Please find the attached report generated automatically by the WMS system.

<table style="border-collapse:collapse;font-size:13px;margin-top:8px;">
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Report Name</td><td><b>%s</b></td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Output</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0">Generated</td><td>%s</td></tr>
  <tr><td style="color:#6b7280;padding:2px 12px 2px 0;vertical-align:top">Contents</td>
    <td><table>%s</table></td>
  </tr>
</table>`,
		report.Name,
		outputLabel,
		now.Format("02 January 2006, 15:04 WIB"),
		queryList,
	)
}

func defaultFooter() string {
	return `This email was sent automatically by WMS Report Mailer. Please do not reply to this email.`
}

// ─── Main Builder ─────────────────────────────────────────────────────────────

// buildEmailSubject resolve subject dengan placeholder atau pakai default
func buildEmailSubject(report report_mailer.Report) string {
	if report.EmailSubject == "" {
		return defaultSubject(report)
	}
	return resolvePlaceholders(report.EmailSubject, report)
}

// buildEmailBody render full HTML email dari template report
func buildEmailBody(report report_mailer.Report) string {
	// Resolve tiap section — pakai custom kalau ada, fallback ke default
	headerContent := defaultHeader(report)
	if report.EmailHeader != "" {
		headerContent = resolvePlaceholders(report.EmailHeader, report)
	}

	bodyContent := defaultBody(report)
	if report.EmailBody != "" {
		bodyContent = resolvePlaceholders(report.EmailBody, report)
	}

	footerContent := defaultFooter()
	if report.EmailFooter != "" {
		footerContent = resolvePlaceholders(report.EmailFooter, report)
	}

	headerColor := "#1E40AF"
	if report.HeaderColor != "" {
		headerColor = report.HeaderColor
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background:#F3F4F6;font-family:Arial,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#F3F4F6;padding:32px 0;">
    <tr>
      <td align="center">
        <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.08);">

          <!-- HEADER -->
          <tr>
            <td style="background:%s;padding:28px 36px;">
              <div style="color:#ffffff;font-size:20px;font-weight:bold;line-height:1.3;">
                %s
              </div>
            </td>
          </tr>

          <!-- BODY -->
          <tr>
            <td style="padding:32px 36px;color:#374151;font-size:14px;line-height:1.7;">
              %s
            </td>
          </tr>

          <!-- DIVIDER -->
          <tr>
            <td style="padding:0 36px;">
              <hr style="border:none;border-top:1px solid #E5E7EB;margin:0;">
            </td>
          </tr>

          <!-- FOOTER -->
          <tr>
            <td style="padding:20px 36px;color:#9CA3AF;font-size:12px;line-height:1.6;">
              %s
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		headerColor,
		headerContent,
		bodyContent,
		footerContent,
	)
}
