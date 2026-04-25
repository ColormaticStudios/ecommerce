package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	encoded := make([]string, 0, len(a))
	for _, value := range a {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
		encoded = append(encoded, `"`+escaped+`"`)
	}
	return "{" + strings.Join(encoded, ",") + "}", nil
}

func (a *StringArray) Scan(value any) error {
	if value == nil {
		*a = nil
		return nil
	}
	switch typed := value.(type) {
	case []byte:
		return a.scanString(string(typed))
	case string:
		return a.scanString(typed)
	case []string:
		*a = append((*a)[:0], typed...)
		return nil
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type *StringArray", value)
	}
}

func (a *StringArray) scanString(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "{}" {
		*a = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var decoded []string
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return err
		}
		*a = decoded
		return nil
	}
	decoded, err := parsePostgresTextArray(trimmed)
	if err != nil {
		return err
	}
	*a = decoded
	return nil
}

func parsePostgresTextArray(value string) ([]string, error) {
	if !strings.HasPrefix(value, "{") || !strings.HasSuffix(value, "}") {
		return nil, fmt.Errorf("invalid postgres text array")
	}
	body := value[1 : len(value)-1]
	if body == "" {
		return nil, nil
	}
	out := []string{}
	var current strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range body {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			out = append(out, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if escaped || inQuotes {
		return nil, fmt.Errorf("invalid postgres text array")
	}
	out = append(out, current.String())
	return out, nil
}
