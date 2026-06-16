package qr_parser

import (
	"fmt"
	"strconv"
	"strings"
)

// mapToResult mengkonversi raw map[key]value → ParsedQRData
// menggunakan field_map dari config sebagai lookup
func mapToResult(m map[string]string, config Config) ParsedQRData {
	fm := config.FieldMap
	result := ParsedQRData{}

	result.SKU = m[fm.SKU]
	result.EAN = m[fm.EAN]
	result.Product = m[fm.Product]
	result.Brand = m[fm.Brand]
	result.Model = m[fm.Model]
	result.Serial = m[fm.Serial]
	result.CartonSerial = m[fm.CartonSerial]
	result.Batch = m[fm.Batch]

	// Mfg Date conversion
	if raw := m[fm.MfgDate]; raw != "" {
		result.MfgDate = convertMfgDate(raw, config.MfgDateFormat)
	}

	// Qty Per Carton
	if raw := m[fm.QtyPerCarton]; raw != "" {
		var qty int
		var err error
		if config.QtyStripUnit {
			// parseInt — strip satuan (50PCS → 50)
			qty, err = strconv.Atoi(strings.TrimRight(raw, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz "))
		} else {
			qty64, e := strconv.ParseFloat(raw, 64)
			qty = int(qty64)
			err = e
		}
		if err == nil {
			result.QtyPerCarton = &qty
		}
	}

	// Label type
	if result.Serial != "" {
		result.LabelType = "UNIT"
	} else if result.CartonSerial != "" {
		result.LabelType = "CARTON"
	} else {
		result.LabelType = "UNKNOWN"
	}

	// Inner serials
	start := m[fm.InnerSerialStart]
	end := m[fm.InnerSerialEnd]
	result.InnerSerialStart = start
	result.InnerSerialEnd = end

	if start != "" {
		serials, rangeErr := generateInnerSerials(start, end, result.QtyPerCarton)
		if rangeErr != "" {
			result.InnerSerialRangeError = rangeErr
		} else {
			result.InnerSerials = serials
		}
	}

	return result
}

func convertMfgDate(raw, format string) string {
	if len(raw) != 8 {
		return raw
	}
	switch format {
	case "YYYYMMDD":
		return fmt.Sprintf("%s-%s-%s", raw[0:4], raw[4:6], raw[6:8])
	case "DDMMYYYY":
		return fmt.Sprintf("%s-%s-%s", raw[6:8], raw[2:4], raw[0:4]) // → YYYY-MM-DD
	case "MMDDYYYY":
		return fmt.Sprintf("%s-%s-%s", raw[6:8], raw[0:2], raw[4:8]) // → YYYY-MM-DD
	default:
		return raw
	}
}

func generateInnerSerials(start, end string, qty *int) ([]string, string) {
	// extract prefix + number dari start
	// contoh: "SN0001" → prefix="SN", startNum=1, padLen=4
	prefixEnd := strings.IndexAny(start, "0123456789")
	if prefixEnd == -1 {
		return nil, "Cannot parse INNER_SERIAL_START: no numeric suffix found"
	}

	prefix := start[:prefixEnd]
	numStr := start[prefixEnd:]
	padLen := len(numStr)
	startNum, err := strconv.Atoi(numStr)
	if err != nil {
		return nil, "Cannot parse number in INNER_SERIAL_START"
	}

	var endNum int
	if end != "" {
		endNumStr := strings.TrimPrefix(end, prefix)
		endNum, err = strconv.Atoi(endNumStr)
		if err != nil {
			return nil, "Cannot parse number in INNER_SERIAL_END"
		}
	} else if qty != nil && *qty > 0 {
		endNum = startNum + *qty - 1
	} else {
		return nil, "Cannot determine end of serial range: no INNER_SERIAL_END or QTY_PER_CARTON"
	}

	if endNum < startNum {
		return nil, fmt.Sprintf("Invalid range: start=%d end=%d", startNum, endNum)
	}

	serials := make([]string, 0, endNum-startNum+1)
	for i := startNum; i <= endNum; i++ {
		serials = append(serials, fmt.Sprintf("%s%0*d", prefix, padLen, i))
	}

	// Validate last generated vs INNER_SERIAL_END
	if end != "" && serials[len(serials)-1] != end {
		return nil, fmt.Sprintf("Range tidak valid: last generated %q ≠ INNER_SERIAL_END %q", serials[len(serials)-1], end)
	}

	// Validate count vs qty
	if qty != nil && len(serials) != *qty {
		return nil, fmt.Sprintf("Jumlah serial %d ≠ QTY_PER_CARTON %d", len(serials), *qty)
	}

	return serials, ""
}
