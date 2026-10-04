package flows

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// The helpers in this file work on JSON-decoded value shapes: nil, bool, float64,
// json.Number, int/int64 (values produced in Go code), string, []any and
// map[string]any. Other Go types are not interpreted.

// Env holds the values templates can read and the location used for dates.
type Env struct {
	Roots    map[string]any
	Location *time.Location
}

// loc returns the location used for zone-less dates: the Env's own location, or
// the server time zone (time.Local) when the Env or its Location is nil.
func (e *Env) loc() *time.Location {
	if e == nil || e.Location == nil {
		return time.Local
	}
	return e.Location
}

// Stringify renders a value as text: nil is empty, scalars use their natural
// form and lists/objects become compact JSON without HTML escaping. Typed nil
// lists and objects render as the empty containers "[]" and "{}"; values JSON
// cannot represent (NaN, Inf, cycles) render as "<unserializable>".
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
	case []any:
		if x == nil {
			return "[]"
		}
		return compactJSON(x)
	case map[string]any:
		if x == nil {
			return "{}"
		}
		return compactJSON(x)
	default:
		return compactJSON(x)
	}
}

// marshalCompact encodes v as compact JSON without HTML escaping or a trailing
// newline and reports values that cannot be encoded (NaN/Inf numbers, cyclic
// structures, unsupported types) as an error. Callers that must not hide such
// values, like the json filter, use it directly; compactJSON wraps it for
// display.
func marshalCompact(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// compactJSON is marshalCompact for display: values that cannot be encoded
// yield the fixed text "<unserializable>"; it deliberately does not fall back to
// fmt.Sprint, which recurses forever on cyclic maps.
func compactJSON(v any) string {
	s, err := marshalCompact(v)
	if err != nil {
		return "<unserializable>"
	}
	return s
}

// toNumber converts the numeric shapes of JSON-decoded data (float64,
// json.Number, int, int64, float32) and numeric strings (surrounding blanks
// allowed) to float64. It rejects NaN, infinities, out-of-range strings such as
// "1e999", and every other type including bool, nil, lists and objects.
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
// when they carry no zone; nil loc means time.Local) and unix timestamps
// (seconds with fractions kept, or milliseconds when the magnitude exceeds 1e12).
// Numeric strings are not timestamps. Zone-less local times that fall into a DST
// gap are normalised forward (02:30 in a 02:00->03:00 jump becomes 03:30), and
// timestamps whose year lies outside 0..9999 fail instead of producing times
// that cannot be formatted as RFC 3339.
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
	f, ok := toNumber(v)
	if !ok {
		return time.Time{}, false
	}
	var t time.Time
	if math.Abs(f) > 1e12 {
		// Milliseconds. The magnitude guard keeps the int64 conversion defined
		// for absurd values; the year check below does the precise filtering.
		if math.Abs(f) > 1e16 {
			return time.Time{}, false
		}
		t = time.UnixMilli(int64(f))
	} else {
		sec, frac := math.Modf(f)
		t = time.Unix(int64(sec), int64(math.Round(frac*1e9)))
	}
	t = t.In(loc)
	if y := t.Year(); y < 0 || y > 9999 {
		return time.Time{}, false
	}
	return t, true
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

// truthy decides whether a value counts as true in conditions. Bools are taken
// as they are; strings are true only for the words true, yes, 1, ja and on (case
// insensitive, surrounding blanks ignored), so other strings, including "2", are
// false; numbers are true when non-zero; lists and objects when non-empty. nil
// and every other type are false.
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

// isEmptyValue reports whether a value carries no content: nil, blank strings
// and empty lists or objects. Zero, false and a list holding only nil are not
// empty. It expects JSON-decoded shapes (string, []any, map[string]any).
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
