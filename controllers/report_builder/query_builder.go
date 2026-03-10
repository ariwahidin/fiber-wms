package report_builder

import (
	"errors"
	"fiber-app/models/report_builder"
	"fmt"
	"regexp"
	"strings"
)

// ─── Dangerous keyword validator ─────────────────────────────────────────────

var dangerousKeywords = []string{
	// DDL
	"DROP", "CREATE", "ALTER", "TRUNCATE", "RENAME",
	// DML write
	"INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT",
	// Execution
	"EXEC", "EXECUTE", "SP_", "XP_", "OPENROWSET", "OPENDATASOURCE",
	// Comment / stacking
	"--", ";--", "/*", "*/",
	// System tables
	"SYSOBJECTS", "SYSCOLUMNS", "INFORMATION_SCHEMA", "SYS.",
	// Time-based attacks
	"WAITFOR", "DELAY", "SHUTDOWN",
}

// hanya izinkan karakter aman untuk field key: huruf, angka, underscore, titik
// contoh valid: "item_code", "a.owner_code", "rec_date"
var safeFieldKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// ValidateBaseQuery memastikan base_query hanya berisi SELECT/WITH (CTE).
// Dipanggil saat admin menyimpan report definition, bukan tiap request user.
func ValidateBaseQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("base query cannot be empty")
	}

	normalized := normalizeSQL(query)

	// Harus diawali SELECT atau WITH (CTE)
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return errors.New("base query must start with SELECT or WITH (CTE)")
	}

	// Word boundary: "created_by" tidak match "CREATE", "deleted_at" tidak match "DELETE"
	for _, kw := range dangerousKeywords {
		pattern := `\b` + regexp.QuoteMeta(kw) + `\b`
		matched, _ := regexp.MatchString(pattern, normalized)
		if matched {
			return fmt.Errorf("base query contains forbidden keyword: %s", kw)
		}
	}

	return nil
}

func validateFieldKey(key string) error {
	if !safeFieldKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid field key '%s': only letters, numbers, underscore, dot allowed", key)
	}
	return nil
}

func normalizeSQL(query string) string {
	upper := strings.ToUpper(query)
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(upper, " "))
}

// ─── BuildQuery ───────────────────────────────────────────────────────────────
//
// Keamanan:
//   - base_query divalidasi: harus SELECT/WITH, tidak boleh DDL/DML
//   - field_key dari runtime filters di-whitelist via regex
//   - semua nilai filter pakai parameterized args — tidak pernah di-concat ke SQL
//   - operator di-whitelist eksplisit, unknown operator = error
//
// Support:
//   - WHERE, CTE (WITH ... AS), subquery, GROUP BY, HAVING, ORDER BY
//   - BETWEEN (value: "2026-01-01,2026-03-31")
//   - IN     (value: "A,B,C")
//   - LIKE   (value: "keyword" -> otomatis %keyword%)
func BuildQuery(
	baseQuery string,
	layoutFilters []report_builder.RptLayoutFilter,
	runtimeFilters map[string]string,
) (query string, args []interface{}, err error) {

	if err := ValidateBaseQuery(baseQuery); err != nil {
		return "", nil, err
	}

	type filterEntry struct {
		fieldKey string
		operator string
		value    string
	}

	mergedFilters := make(map[string]filterEntry)

	// Load default dari layout filters (field_key dari DB — tetap divalidasi defense-in-depth)
	for _, lf := range layoutFilters {
		if lf.Field.FieldKey == "" {
			continue
		}
		if err := validateFieldKey(lf.Field.FieldKey); err != nil {
			return "", nil, err
		}
		mergedFilters[lf.Field.FieldKey] = filterEntry{
			fieldKey: lf.Field.FieldKey,
			operator: lf.Operator,
			value:    lf.FilterValue,
		}
	}

	// Override dengan runtime filters — validasi ketat field key
	for fieldKey, val := range runtimeFilters {
		if val == "" {
			continue
		}
		if err := validateFieldKey(fieldKey); err != nil {
			return "", nil, err
		}
		if existing, ok := mergedFilters[fieldKey]; ok {
			existing.value = val
			mergedFilters[fieldKey] = existing
		} else {
			mergedFilters[fieldKey] = filterEntry{
				fieldKey: fieldKey,
				operator: "EQ",
				value:    val,
			}
		}
	}

	// Build WHERE clause — semua value via args (parameterized, tidak pernah di-concat)
	var conditions []string

	for _, f := range mergedFilters {
		if f.value == "" {
			continue
		}

		col := f.fieldKey // sudah divalidasi via regex, aman di-interpolasi ke SQL

		switch strings.ToUpper(f.operator) {
		case "EQ":
			conditions = append(conditions, fmt.Sprintf("%s = ?", col))
			args = append(args, f.value)

		case "NEQ":
			conditions = append(conditions, fmt.Sprintf("%s != ?", col))
			args = append(args, f.value)

		case "LIKE":
			conditions = append(conditions, fmt.Sprintf("%s LIKE ?", col))
			args = append(args, "%"+f.value+"%")

		case "GTE":
			conditions = append(conditions, fmt.Sprintf("%s >= ?", col))
			args = append(args, f.value)

		case "LTE":
			conditions = append(conditions, fmt.Sprintf("%s <= ?", col))
			args = append(args, f.value)

		case "BETWEEN":
			parts := strings.SplitN(f.value, ",", 2)
			if len(parts) != 2 {
				return "", nil, fmt.Errorf("BETWEEN filter for '%s' must have format 'val1,val2'", col)
			}
			conditions = append(conditions, fmt.Sprintf("%s BETWEEN ? AND ?", col))
			args = append(args, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))

		case "IN":
			parts := strings.Split(f.value, ",")
			if len(parts) == 0 {
				continue
			}
			placeholders := make([]string, len(parts))
			for i, p := range parts {
				placeholders[i] = "?"
				args = append(args, strings.TrimSpace(p))
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")))

		default:
			return "", nil, fmt.Errorf("unsupported operator: '%s'", f.operator)
		}
	}

	// Wrap + inject WHERE
	//
	// Kenapa outer SELECT * FROM (...)?
	// Base query bisa punya GROUP BY, HAVING, ORDER BY — WHERE di-inject di outer
	// tanpa merusak struktur dalam.
	//
	// CTE case (SQL Server):
	// SQL Server tidak bisa bungkus CTE sebagai subquery langsung,
	// jadi kita buat CTE wrapper _outer yang bungkus seluruh base query.
	trimmed := strings.TrimSpace(baseQuery)
	normalized := normalizeSQL(trimmed)
	whereClause := strings.Join(conditions, " AND ")

	if strings.HasPrefix(normalized, "WITH") {
		// WITH _outer AS (
		//   <base_cte_query>
		// )
		// SELECT * FROM _outer WHERE ...
		if len(conditions) > 0 {
			query = fmt.Sprintf("WITH _outer AS (\n%s\n)\nSELECT * FROM _outer\nWHERE %s", trimmed, whereClause)
		} else {
			query = fmt.Sprintf("WITH _outer AS (\n%s\n)\nSELECT * FROM _outer", trimmed)
		}
	} else {
		// SELECT * FROM (
		//   <base_query>
		// ) AS _rpt WHERE ...
		if len(conditions) > 0 {
			query = fmt.Sprintf("SELECT * FROM (\n%s\n) AS _rpt\nWHERE %s", trimmed, whereClause)
		} else {
			query = fmt.Sprintf("SELECT * FROM (\n%s\n) AS _rpt", trimmed)
		}
	}

	return query, args, nil
}

// ─── BuildQueryForDocument ────────────────────────────────────────────────────
//
// Untuk DOCUMENT type (Picking List, SPK, dll).
// Named params (:outbound_id, :inbound_id, dll) di-detect berurutan sesuai
// kemunculan di SQL — diganti ? dan nilai masuk ke args (parameterized).
func BuildQueryForDocument(baseQuery string, params map[string]string) (string, []interface{}, error) {
	if err := ValidateBaseQuery(baseQuery); err != nil {
		return "", nil, err
	}

	namedParamRegex := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := namedParamRegex.FindAllStringSubmatch(baseQuery, -1)

	query := baseQuery
	var args []interface{}

	for _, match := range matches {
		fullMatch := match[0] // ":outbound_id"
		paramName := match[1] // "outbound_id"

		val, ok := params[paramName]
		if !ok || val == "" {
			return "", nil, fmt.Errorf("missing required param: '%s'", paramName)
		}

		// Ganti kemunculan pertama saja — urutan args konsisten dengan urutan di SQL
		query = strings.Replace(query, fullMatch, "?", 1)
		args = append(args, val)
	}

	return query, args, nil
}
