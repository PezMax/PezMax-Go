package kadmin

import (
	"strconv"
	"strings"
	"time"
)

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}

func toDateTimeString(v interface{}) string {
	switch value := v.(type) {
	case time.Time:
		return value.Format("2006-01-02 15:04:05")
	case string:
		return formatDateTimeText(value)
	case []byte:
		return formatDateTimeText(string(value))
	default:
		return ""
	}
}

func formatDateTimeText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}

	value = strings.Replace(value, "T", " ", 1)
	value = strings.TrimSuffix(value, "Z")
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		value = value[:dot]
	}
	if len(value) >= len("2006-01-02 15:04:05") {
		return value[:len("2006-01-02 15:04:05")]
	}
	return value
}

func toInt64(v interface{}) int64 {
	switch value := v.(type) {
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		return int64(value)
	case uint8:
		return int64(value)
	case uint16:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		return int64(value)
	case []byte:
		i, _ := strconv.ParseInt(string(value), 10, 64)
		return i
	case string:
		i, _ := strconv.ParseInt(value, 10, 64)
		return i
	default:
		return 0
	}
}
