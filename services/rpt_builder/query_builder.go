package rpt_builder_service

import (
	"fmt"
	"regexp"
	"strings"
)

var namedParamRegex = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
var stripStringsRegex = regexp.MustCompile(`'[^']*'`)

// extractNamedParams scan named params hanya dari luar string literals.
// Mengembalikan SETIAP occurrence sesuai urutan kemunculan di query
// (tidak di-dedupe), supaya param yang sama dipakai berkali-kali
// (misal di subquery) tetap ter-replace semua dan args-nya tetap matching.
func extractNamedParams(query string) []string {
	stripped := stripStringsRegex.ReplaceAllString(query, "''")
	matches := namedParamRegex.FindAllStringSubmatch(stripped, -1)

	var params []string
	for _, m := range matches {
		params = append(params, m[1])
	}
	return params
}

func BuildQuery(sqlQuery string, params map[string]string) (string, []interface{}, error) {
	if err := ValidateQuery(sqlQuery); err != nil {
		return "", nil, err
	}

	paramNames := extractNamedParams(sqlQuery)

	query := sqlQuery
	var args []interface{}

	for _, paramName := range paramNames {
		if err := validateParamKey(paramName); err != nil {
			return "", nil, err
		}

		val, ok := params[paramName]
		if !ok || val == "" {
			return "", nil, fmt.Errorf("missing required param: '%s'", paramName)
		}

		// strings.Replace dengan n=1 akan selalu makan occurrence
		// paling kiri yang belum diganti, jadi urutan args tetap
		// sesuai urutan posisi "?" di query.
		query = strings.Replace(query, ":"+paramName, "?", 1)
		args = append(args, val)
	}

	return query, args, nil
}
