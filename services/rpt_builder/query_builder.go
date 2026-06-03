package rpt_builder_service

import (
	"fmt"
	"regexp"
	"strings"
)

var namedParamRegex = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)

// BuildQuery mengganti named params (:key) di SQL dengan ? dan mengumpulkan args.
// Urutan args mengikuti urutan kemunculan param di SQL.
func BuildQuery(sqlQuery string, params map[string]string) (string, []interface{}, error) {
	if err := ValidateQuery(sqlQuery); err != nil {
		return "", nil, err
	}

	matches := namedParamRegex.FindAllStringSubmatch(sqlQuery, -1)

	query := sqlQuery
	var args []interface{}

	for _, match := range matches {
		fullMatch := match[0]
		paramName := match[1]

		if err := validateParamKey(paramName); err != nil {
			return "", nil, err
		}

		val, ok := params[paramName]
		if !ok || val == "" {
			return "", nil, fmt.Errorf("missing required param: '%s'", paramName)
		}

		query = strings.Replace(query, fullMatch, "?", 1)
		args = append(args, val)
	}

	return query, args, nil
}
