package flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Env holds the values templates can read and the location used for dates.
type Env struct {
	Roots    map[string]any
	Location *time.Location
}

func (e *Env) loc() *time.Location {
	if e == nil || e.Location == nil {
		return time.Local
	}
	return e.Location
}

// Stringify renders a value as text: nil is empty, scalars use their natural
// form and lists/objects become compact JSON without HTML escaping.
func Stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return compactJSON(x)
	}
}

func compactJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprint(v)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func toNumber(v any) (float64, bool) {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case float32:
		f = float64(x)
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case json.Number:
		parsed, err := x.Float64()
		if err != nil {
			return 0, false
		}
		f = parsed
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		f = parsed
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
}

// toTime converts time values, date strings (several layouts, interpreted in loc
// when they carry no zone) and unix timestamps (seconds, or milliseconds above 1e12).
func toTime(v any, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		s := strings.TrimSpace(x)
		for _, layout := range timeLayouts {
			if t, err := time.ParseInLocation(layout, s, loc); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	if f, ok := toNumber(v); ok {
		if math.Abs(f) > 1e12 {
			return time.UnixMilli(int64(f)).In(loc), true
		}
		return time.Unix(int64(f), 0).In(loc), true
	}
	return time.Time{}, false
}

// toTimeStrict accepts only time values and RFC3339 strings. Conditions with
// type "auto" use it so ordinary text never compares as a date by accident.
func toTimeStrict(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		s := strings.TrimSpace(x)
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "yes", "1", "ja", "on":
			return true
		}
		return false
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	if f, ok := toNumber(v); ok {
		return f != 0
	}
	return false
}

func isEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}
