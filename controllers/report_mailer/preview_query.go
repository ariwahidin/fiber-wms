package report_mailer

import (
	rm_services "fiber-app/services/report_mailer"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// previewQueryInput digunakan untuk endpoint preview query
var previewQueryInput struct {
	Query string `json:"query" validate:"required"`
}

// POST /api/v1/report-mailer/reports/preview-query
func (c *ReportController) PreviewQuery(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&previewQueryInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := rm_services.ValidateSelectOnly(previewQueryInput.Query); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Query tidak aman: " + err.Error(),
		})
	}

	if previewQueryInput.Query == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Query tidak boleh kosong"})
	}

	// Wrap query sebagai subquery, ambil TOP 5 saja
	// Pakai raw db untuk SQL Server compatibility
	// wrappedQuery := "SELECT TOP 5 * FROM (" + previewQueryInput.Query + ") AS _preview"

	// Untuk CTE (WITH), tidak bisa dibungkus sebagai subquery
	// Inject TOP 5 langsung ke SELECT terakhir
	upper := strings.ToUpper(strings.TrimSpace(previewQueryInput.Query))
	var finalQuery string

	if strings.HasPrefix(upper, "WITH") {
		// Inject TOP 5 ke SELECT terakhir di dalam CTE
		finalQuery = injectTopN(previewQueryInput.Query, 5)
	} else {
		finalQuery = "SELECT TOP 5 * FROM (" + previewQueryInput.Query + ") AS _preview"
	}

	rows, err := c.DB.Raw(finalQuery).Rows()
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Query error: " + err.Error(),
		})
	}
	defer rows.Close()

	// Ambil nama kolom
	columns, err := rows.Columns()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Kumpulkan data sebagai slice of map
	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte ke string
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result = append(result, row)
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"columns": columns,
			"rows":    result,
			"count":   len(result),
		},
	})
}

// injectTopN menyisipkan TOP N ke SELECT terakhir dalam query
func injectTopN(query string, n int) string {
	// Cari posisi SELECT terakhir (yang bukan di dalam CTE definition)
	upper := strings.ToUpper(query)
	lastSelect := strings.LastIndex(upper, "SELECT")
	if lastSelect == -1 {
		return query
	}
	// Sisipkan TOP N setelah SELECT terakhir
	return query[:lastSelect+6] + fmt.Sprintf(" TOP %d", n) + query[lastSelect+6:]
}
