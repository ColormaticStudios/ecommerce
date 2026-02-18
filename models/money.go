package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money stores currency amounts in minor units (cents) to avoid float precision errors.
type Money int64

func MoneyFromFloat(value float64) Money {
	return Money(math.Round(value * 100))
}

func (m Money) Float64() float64 {
	return float64(m) / 100
}

func (m Money) Mul(quantity int) Money {
	return m * Money(quantity)
}

func (m Money) String() string {
	return fmt.Sprintf("%.2f", m.Float64())
}

func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(m.String()), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var floatValue float64
	if err := json.Unmarshal(data, &floatValue); err != nil {
		return err
	}
	*m = MoneyFromFloat(floatValue)
	return nil
}

func (m Money) Value() (driver.Value, error) {
	return m.String(), nil
}

func (m *Money) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*m = 0
		return nil
	case int64:
		*m = Money(v)
		return nil
	case float64:
		*m = MoneyFromFloat(v)
		return nil
	case []byte:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err != nil {
			return err
		}
		*m = MoneyFromFloat(parsed)
		return nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return err
		}
		*m = MoneyFromFloat(parsed)
		return nil
	default:
		return fmt.Errorf("unsupported money scan type %T", value)
	}
}
