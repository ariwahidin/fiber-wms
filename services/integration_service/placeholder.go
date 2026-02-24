// services/integration_service/placeholder.go

package integration_service

import (
	"fmt"
	"strings"
	"time"
)

func resolvePlaceholders(template string, data map[string]interface{}) string {
	result := template

	// Resolve dari event data
	for k, v := range data {
		var strVal string
		switch val := v.(type) {
		case time.Time:
			strVal = val.Format("02 January 2006, 15:04 WIB")
		case nil:
			strVal = ""
		default:
			strVal = fmt.Sprintf("%v", val)
		}
		result = strings.ReplaceAll(result, "{{"+k+"}}", strVal)
	}

	// Resolve built-in placeholders
	now := time.Now()
	result = strings.ReplaceAll(result, "{{date}}", now.Format("02 January 2006"))
	result = strings.ReplaceAll(result, "{{datetime}}", now.Format("02 January 2006, 15:04 WIB"))
	result = strings.ReplaceAll(result, "{{year}}", now.Format("2006"))
	result = strings.ReplaceAll(result, "{{month}}", now.Format("January"))

	return result
}
