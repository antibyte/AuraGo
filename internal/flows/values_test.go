package flows

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStringify(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"text", "text"},
		{true, "true"},
		{3.0, "3"},
		{2.5, "2.5"},
		{7, "7"},
		{int64(8), "8"},
		{json.Number("9.5"), "9.5"},
		{[]any{"a", 1.0}, `["a",1]`},
		{map[string]any{"a": "<b>"}, `{"a":"<b>"}`},
		{time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), "2026-10-03T07:00:00Z"},
	}
	for _, tc := range cases {
		if got := Stringify(tc.in); got != tc.want {
			t.Errorf("Stringify(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestToNumber(t *testing.T) {
	for _, in := range []any{3.5, float32(1), 4, int64(5), json.Number("6"), " 7.25 "} {
		if _, ok := toNumber(in); !ok {
			t.Errorf("toNumber(%#v) failed", in)
		}
	}
	for _, in := range []any{"", "abc", "NaN", "Inf", nil, true, []any{}} {
		if _, ok := toNumber(in); ok {
			t.Errorf("toNumber(%#v) succeeded, want failure", in)
		}
	}
}

func TestToTime(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	got, ok := toTime("2026-10-03T07:00:00Z", berlin)
	if !ok || !got.Equal(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("RFC3339 = %v %v", got, ok)
	}
	got, ok = toTime("2026-10-03 09:00:00", berlin)
	if !ok || !got.Equal(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("local layout = %v %v", got, ok)
	}
	got, ok = toTime("2026-10-03 09:00", berlin)
	if !ok || !got.Equal(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("local layout without seconds = %v %v", got, ok)
	}
	if got, ok = toTime(0.0, time.UTC); !ok || got.Year() != 1970 {
		t.Fatalf("unix seconds = %v %v", got, ok)
	}
	if got, ok = toTime(1.7e12, time.UTC); !ok || got.Year() != 2023 {
		t.Fatalf("unix millis = %v %v", got, ok)
	}
	if _, ok = toTime("tomorrow", time.UTC); ok {
		t.Fatal("free text must not parse")
	}
	if _, ok = toTimeStrict("2026-10-03 09:00:00"); ok {
		t.Fatal("toTimeStrict accepts only RFC3339")
	}
	if _, ok = toTimeStrict("2026-10-03T09:00:00+02:00"); !ok {
		t.Fatal("toTimeStrict must accept RFC3339")
	}
}

func TestTruthyAndEmpty(t *testing.T) {
	for _, v := range []any{true, "true", "YES", "1", "ja", "on", 2.0, []any{1.0}, map[string]any{"a": 1.0}} {
		if !truthy(v) {
			t.Errorf("truthy(%#v) = false", v)
		}
	}
	for _, v := range []any{false, "false", "no", "", 0.0, nil, []any{}, map[string]any{}} {
		if truthy(v) {
			t.Errorf("truthy(%#v) = true", v)
		}
	}
	for _, v := range []any{nil, "", "  ", []any{}, map[string]any{}} {
		if !isEmptyValue(v) {
			t.Errorf("isEmptyValue(%#v) = false", v)
		}
	}
	for _, v := range []any{"x", 0.0, false, []any{nil}} {
		if isEmptyValue(v) {
			t.Errorf("isEmptyValue(%#v) = true", v)
		}
	}
}
