package services

import (
	"fmt"
	"regexp"
	"strings"
)

// keyword berbahaya yang diblokir
var dangerousKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE",
	"ALTER", "CREATE", "EXEC", "EXECUTE", "SP_",
	"XP_", "MERGE", "REPLACE", "GRANT", "REVOKE",
	"DENY", "BACKUP", "RESTORE", "BULK", "OPENROWSET",
	"OPENDATASOURCE", "SHUTDOWN",
}

// ValidateSelectOnly memastikan query hanya berisi SELECT.
// Return error kalau ditemukan keyword berbahaya atau bukan SELECT.
func ValidateSelectOnly(query string) error {
	// Bersihkan komentar SQL sebelum validasi
	// supaya tidak bisa disembunyikan di dalam komentar
	cleaned := removeComments(query)

	// Normalisasi whitespace
	normalized := strings.TrimSpace(cleaned)
	upper := strings.ToUpper(normalized)

	// Harus dimulai dengan SELECT atau WITH (untuk CTE)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("query harus diawali dengan SELECT atau WITH (CTE)")
	}

	// Cek keyword berbahaya menggunakan word boundary
	// supaya "SELECTDELETE" tidak false positive
	for _, keyword := range dangerousKeywords {
		// Pattern: keyword sebagai kata utuh (bukan bagian dari kata lain)
		pattern := `(?i)\b` + keyword + `\b`
		matched, _ := regexp.MatchString(pattern, cleaned)
		if matched {
			return fmt.Errorf("keyword '%s' tidak diizinkan dalam query report", keyword)
		}
	}

	// Blokir multiple statements (titik koma di tengah)
	// cth: "SELECT 1; DROP TABLE users"
	if containsStatementSeparator(cleaned) {
		return fmt.Errorf("multiple SQL statements tidak diizinkan")
	}

	return nil
}

// removeComments menghapus komentar SQL (-- dan /* */)
func removeComments(query string) string {
	// Hapus block comment /* ... */
	blockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	result := blockComment.ReplaceAllString(query, " ")

	// Hapus line comment -- ...
	lineComment := regexp.MustCompile(`--[^\n]*`)
	result = lineComment.ReplaceAllString(result, " ")

	return result
}

// containsStatementSeparator cek apakah ada ; di luar string literal
func containsStatementSeparator(query string) bool {
	inString := false
	stringChar := rune(0)

	for _, ch := range query {
		if inString {
			if ch == stringChar {
				inString = false
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = true
			stringChar = ch
			continue
		}
		if ch == ';' {
			return true
		}
	}
	return false
}
