package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type NullableTime struct {
	time *time.Time
}

func (n *NullableTime) Scan(value any) error {
	if value == nil {
		n.time = nil
		return nil
	}

	switch typed := value.(type) {
	case time.Time:
		parsed := typed.UTC()
		n.time = &parsed
		return nil
	case []byte:
		return n.parse(string(typed))
	case string:
		return n.parse(typed)
	default:
		return fmt.Errorf("unsupported time value type %T", value)
	}
}

func (n NullableTime) Value() (driver.Value, error) {
	if n.time == nil {
		return nil, nil
	}
	return *n.time, nil
}

func (n NullableTime) Valid() bool {
	return n.time != nil
}

func (n *NullableTime) parse(raw string) error {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			utc := parsed.UTC()
			n.time = &utc
			return nil
		}
	}
	return fmt.Errorf("unsupported time format %q", raw)
}
