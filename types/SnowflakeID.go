package types // atau "common", "models", terserah kamu

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
)

type SnowflakeID int64

func (s SnowflakeID) Value() (driver.Value, error) {
	return int64(s), nil
}

func (s *SnowflakeID) Scan(value interface{}) error {
	switch v := value.(type) {
	case int64:
		*s = SnowflakeID(v)
		return nil
	case []byte:
		i, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}
		*s = SnowflakeID(i)
		return nil
	default:
		return fmt.Errorf("cannot convert %v to SnowflakeID", value)
	}
}

// Marshal: int64 → string
func (s SnowflakeID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(s), 10))
}

// // Unmarshal: string → int64
// func (s *SnowflakeID) UnmarshalJSON(data []byte) error {
// 	var strID string
// 	if err := json.Unmarshal(data, &strID); err != nil {
// 		return fmt.Errorf("failed to unmarshal snowflake ID string: %w", err)
// 	}

// 	val, err := strconv.ParseInt(strID, 10, 64)
// 	if err != nil {
// 		return fmt.Errorf("invalid snowflake ID format: %w", err)
// 	}

// 	*s = SnowflakeID(val)
// 	return nil
// }

func (s *SnowflakeID) UnmarshalJSON(data []byte) error {
	// Coba sebagai string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		val, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid snowflake ID string: %w", err)
		}
		*s = SnowflakeID(val)
		return nil
	}

	// Kalau gagal, coba langsung sebagai number
	var num int64
	if err := json.Unmarshal(data, &num); err == nil {
		*s = SnowflakeID(num)
		return nil
	}

	return fmt.Errorf("invalid snowflake ID format")
}
