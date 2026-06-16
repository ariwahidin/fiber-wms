package qr_parser

import (
	"regexp"
	"strconv"
	"strings"
)

func parseBracketKV(raw string, config Config) (ParsedQRData, error) {
	pattern := regexp.MustCompile(`\((\d+)\)([A-Z_]+)=([^(]*)`)
	matches := pattern.FindAllStringSubmatch(raw, -1)

	if len(matches) == 0 {
		return ParsedQRData{}, ErrNoMatch
	}

	m := make(map[string]string)
	for _, match := range matches {
		m[strings.TrimSpace(match[2])] = strings.TrimSpace(match[3])
	}

	return mapToResult(m, config), nil
}

func parseKeyValue(raw string, config Config) (ParsedQRData, error) {
	m := make(map[string]string)

	pairs := strings.Split(raw, config.Delimiter)
	for _, pair := range pairs {
		// support both = and :
		var kv []string
		if strings.Contains(pair, "=") {
			kv = strings.SplitN(pair, "=", 2)
		} else if strings.Contains(pair, ":") {
			kv = strings.SplitN(pair, ":", 2)
		} else {
			continue
		}
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	if len(m) == 0 {
		return ParsedQRData{}, ErrNoMatch
	}

	return mapToResult(m, config), nil
}

func parsePositional(raw string, config Config) (ParsedQRData, error) {
	cols := strings.Split(raw, config.Delimiter)
	m := make(map[string]string)

	fm := config.FieldMap
	posMap := map[string]string{
		fm.SKU:              "SKU",
		fm.EAN:              "EAN",
		fm.Serial:           "SERIAL",
		fm.CartonSerial:     "CARTON_SERIAL",
		fm.Batch:            "BATCH",
		fm.MfgDate:          "MFG_DATE",
		fm.QtyPerCarton:     "QTY_PER_CARTON",
		fm.InnerSerialStart: "INNER_SERIAL_START",
		fm.InnerSerialEnd:   "INNER_SERIAL_END",
		fm.Product:          "PRODUCT",
		fm.Brand:            "BRAND",
		fm.Model:            "MODEL",
	}

	// FieldMap untuk positional berisi index kolom sebagai string, misal "0", "1", "2"
	for idxStr, internalKey := range posMap {
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			continue
		}
		if idx >= 0 && idx < len(cols) {
			val := strings.TrimSpace(cols[idx])
			if val != "" {
				m[internalKey] = val
			}
		}
	}

	if len(m) == 0 {
		return ParsedQRData{}, ErrNoMatch
	}

	return mapToResult(m, config), nil
}
