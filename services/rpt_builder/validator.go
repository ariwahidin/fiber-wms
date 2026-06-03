package rpt_builder_service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var dangerousKeywords = []string{
	"DROP", "CREATE", "ALTER", "TRUNCATE", "RENAME",
	"INSERT", "UPDATE", "DELETE", "MERGE", "UPSERT",
	"EXEC", "EXECUTE", "SP_", "XP_", "OPENROWSET", "OPENDATASOURCE",
	"--", ";--", "/*", "*/",
	"SYSOBJECTS", "SYSCOLUMNS", "INFORMATION_SCHEMA", "SYS.",
	"WAITFOR", "DELAY", "SHUTDOWN",
}

var safeParamKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func ValidateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("query cannot be empty")
	}

	normalized := normalizeSQL(query)

	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return errors.New("query must start with SELECT or WITH (CTE)")
	}

	for _, kw := range dangerousKeywords {
		pattern := `\b` + regexp.QuoteMeta(kw) + `\b`
		matched, _ := regexp.MatchString(pattern, normalized)
		if matched {
			return fmt.Errorf("query contains forbidden keyword: %s", kw)
		}
	}

	return nil
}

func validateParamKey(key string) error {
	if !safeParamKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid param key '%s': only letters, numbers, underscore allowed", key)
	}
	return nil
}

func normalizeSQL(query string) string {
	upper := strings.ToUpper(query)
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(upper, " "))
}
