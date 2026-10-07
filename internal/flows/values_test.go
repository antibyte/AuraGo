package flows

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestStringify(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	cyclicMap := map[string]any{}
	cyclicMap["self"] = cyclicMap
	cyclicList := make([]any, 1)
	cyclicList[0] = cyclicList

	cases := []struct {
		name string
		in   any
		want string
	}{
		{"untyped nil", nil, ""},
		{"string", "text", "text"},
		{"bool", true, "true"},
		{"whole float", 3.0, "3"},
		{"fractional float", 2.5, "2.5"},
		{"float32", float32(1.5), "1.5"},
		{"int", 7, "7"},
		{"int64", int64(8), "8"},
		{"json.Number", json.Number("9.5"), "9.5"},
		{"list", []any{"a", 1.0}, `["a",1]`},
		{"object without HTML escaping", map[string]any{"a": "<b>"}, `{"a":"<b>"}`},
		{"time in UTC", time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), "2026-10-03T07:00:00Z"},
		{"time keeps its zone", time.Date(2026, 10, 3, 9, 0, 0, 0, cest), "2026-10-03T09:00:00+02:00"},
		{"typed nil list", []any(nil), "[]"},
		{"typed nil object", map[string]any(nil), "{}"},
		{"empty list", []any{}, "[]"},
		{"empty object", map[string]any{}, "{}"},
		{"NaN inside object", map[string]any{"x": math.NaN()}, "<unserializable>"},
		{"Inf inside list", []any{math.Inf(1)}, "<unserializable>"},
		{"cyclic object", cyclicMap, "<unserializable>"},
		{"cyclic list", cyclicList, "<unserializable>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Stringify(tc.in); got != tc.want {
				t.Errorf("Stringify(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToNumber(t *testing.T) {
	ok := []struct {
		in   any
		want float64
	}{
		{3.5, 3.5},
		{float32(1), 1},
		{4, 4},
		{int64(5), 5},
		{json.Number("6"), 6},
		{" 7.25 ", 7.25},
		{"-3", -3},
		{"1e3", 1000},
		{"0", 0},
		{-2.5, -2.5},
		{json.Number("-1e2"), -100},
	}
	for _, tc := range ok {
		got, isNum := toNumber(tc.in)
		if !isNum {
			t.Errorf("toNumber(%#v) failed", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("toNumber(%#v) = %v, want %v", tc.in, got, tc.want)
		}
	}

	bad := []any{
		"", "   ", "abc", "NaN", "Inf", "+Inf", "-Inf", "1e999", "-1e999",
		nil, true, []any{}, map[string]any{},
		math.NaN(), math.Inf(1), math.Inf(-1), float32(math.NaN()),
		json.Number("x"), json.Number(""), json.Number("1e999"),
	}
	for _, in := range bad {
		if got, isNum := toNumber(in); isNum {
			t.Errorf("toNumber(%#v) = %v, want failure", in, got)
		}
	}
}

func TestToTime(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	utc := func(y int, mo time.Month, d, h, mi, s, ns int) time.Time {
		return time.Date(y, mo, d, h, mi, s, ns, time.UTC)
	}
	instant2026 := utc(2026, 10, 3, 7, 0, 0, 0)

	cases := []struct {
		name string
		in   any
		loc  *time.Location
		want time.Time
		ok   bool
	}{
		// Strings with and without a zone.
		{"RFC3339 UTC", "2026-10-03T07:00:00Z", cest, instant2026, true},
		{"RFC3339 offset", "2026-10-03T09:00:00+02:00", time.UTC, instant2026, true},
		{"RFC3339 fractional seconds", "2026-10-03T09:00:00.5+02:00", time.UTC, utc(2026, 10, 3, 7, 0, 0, 500_000_000), true},
		{"RFC3339Nano fractional seconds", "2026-10-03T07:00:00.123456789Z", cest, utc(2026, 10, 3, 7, 0, 0, 123456789), true},
		{"space layout in loc", "2026-10-03 09:00:00", cest, instant2026, true},
		{"space layout without seconds", "2026-10-03 09:00", cest, instant2026, true},
		{"T layout without zone", "2026-10-03T09:00:00", cest, instant2026, true},
		{"T layout without zone or seconds", "2026-10-03T09:00", cest, instant2026, true},
		{"date only is midnight in loc", "2026-10-03", cest, utc(2026, 10, 2, 22, 0, 0, 0), true},
		{"zone-less fractional seconds", "2026-10-03 09:00:00.250", cest, utc(2026, 10, 3, 7, 0, 0, 250_000_000), true},
		{"surrounding whitespace", " \t2026-10-03 09:00:00\n ", cest, instant2026, true},
		{"free text", "tomorrow", time.UTC, time.Time{}, false},
		{"empty string", "", time.UTC, time.Time{}, false},
		{"numeric string is not a timestamp", "1700000000", time.UTC, time.Time{}, false},
		{"impossible date", "2026-02-30", time.UTC, time.Time{}, false},

		// Unix timestamps.
		{"unix zero", 0.0, time.UTC, utc(1970, 1, 1, 0, 0, 0, 0), true},
		{"unix seconds", 1700000000.0, cest, utc(2023, 11, 14, 22, 13, 20, 0), true},
		{"unix seconds int", 1700000000, time.UTC, utc(2023, 11, 14, 22, 13, 20, 0), true},
		{"unix seconds keep fraction", 1700000000.75, time.UTC, utc(2023, 11, 14, 22, 13, 20, 750_000_000), true},
		{"unix seconds json.Number", json.Number("1700000000"), time.UTC, utc(2023, 11, 14, 22, 13, 20, 0), true},
		{"unix seconds json.Number fraction", json.Number("1700000000.75"), time.UTC, utc(2023, 11, 14, 22, 13, 20, 750_000_000), true},
		{"unix millis", 1.7e12, time.UTC, utc(2023, 11, 14, 22, 13, 20, 0), true},
		{"unix millis int64", int64(1700000000123), time.UTC, utc(2023, 11, 14, 22, 13, 20, 123_000_000), true},
		{"negative seconds", -86400.0, time.UTC, utc(1969, 12, 31, 0, 0, 0, 0), true},
		{"negative seconds keep fraction", -1.5, time.UTC, utc(1969, 12, 31, 23, 59, 58, 500_000_000), true},
		{"negative millis", -1.7e12, time.UTC, utc(1916, 2, 18, 1, 46, 40, 0), true},

		// The seconds/milliseconds boundary sits at 1e12: that value still counts as
		// seconds (year 33658, out of range), the next integer already as milliseconds.
		{"boundary 1e12 is seconds and out of range", 1e12, time.UTC, time.Time{}, false},
		{"boundary just below 1e12", 999999999999.0, time.UTC, time.Time{}, false},
		{"boundary just above 1e12 is millis", 1e12 + 1, time.UTC, utc(2001, 9, 9, 1, 46, 40, 1_000_000), true},
		{"boundary -1e12 is seconds and out of range", -1e12, time.UTC, time.Time{}, false},
		{"boundary just below -1e12 is millis", -1e12 - 1, time.UTC, utc(1938, 4, 24, 22, 13, 19, 999_000_000), true},

		// Year range 0..9999.
		{"last valid second", 253402300799.0, time.UTC, utc(9999, 12, 31, 23, 59, 59, 0), true},
		{"first second of year 10000", 253402300800.0, time.UTC, time.Time{}, false},
		{"first valid second", -62167219200.0, time.UTC, utc(0, 1, 1, 0, 0, 0, 0), true},
		{"last second before year 0", -62167219201.0, time.UTC, time.Time{}, false},
		{"last valid millisecond", 253402300799999.0, time.UTC, utc(9999, 12, 31, 23, 59, 59, 999_000_000), true},
		{"first millisecond of year 10000", 253402300800000.0, time.UTC, time.Time{}, false},
		{"huge positive", 1e300, time.UTC, time.Time{}, false},
		{"huge negative", -1e300, time.UTC, time.Time{}, false},
		{"huge int64", int64(math.MaxInt64), time.UTC, time.Time{}, false},
		{"huge json.Number", json.Number("1e300"), time.UTC, time.Time{}, false},
		{"year is judged in loc", 253402300799.0, cest, time.Time{}, false},

		// Other shapes.
		{"time passthrough", utc(2026, 10, 3, 7, 0, 0, 0), cest, instant2026, true},
		{"NaN", math.NaN(), time.UTC, time.Time{}, false},
		{"nil", nil, time.UTC, time.Time{}, false},
		{"bool", true, time.UTC, time.Time{}, false},
		{"list", []any{}, time.UTC, time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toTime(tc.in, tc.loc)
			if ok != tc.ok {
				t.Fatalf("toTime(%#v) ok = %v, want %v (got %v)", tc.in, ok, tc.ok, got)
			}
			if !ok {
				if !got.IsZero() {
					t.Fatalf("toTime(%#v) failed but returned %v, want the zero time", tc.in, got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("toTime(%#v) = %v, want %v", tc.in, got.UTC(), tc.want)
			}
		})
	}

	t.Run("numeric results are expressed in loc", func(t *testing.T) {
		got, ok := toTime(1700000000.0, cest)
		if !ok || got.Location() != cest {
			t.Fatalf("location = %v, want CEST (ok=%v)", got.Location(), ok)
		}
	})

	t.Run("time passthrough keeps its own zone", func(t *testing.T) {
		in := time.Date(2026, 10, 3, 9, 0, 0, 0, cest)
		got, ok := toTime(in, time.UTC)
		if !ok || got.Location() != cest {
			t.Fatalf("location = %v, want CEST (ok=%v)", got.Location(), ok)
		}
	})

	t.Run("nil location means time.Local", func(t *testing.T) {
		want, err := time.ParseInLocation("2006-01-02 15:04:05", "2026-10-03 09:00:00", time.Local)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := toTime("2026-10-03 09:00:00", nil)
		if !ok || !got.Equal(want) {
			t.Fatalf("toTime with nil location = %v %v, want %v", got, ok, want)
		}
	})

	t.Run("Europe/Berlin", func(t *testing.T) {
		berlin, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			t.Skip("tzdata missing")
		}
		got, ok := toTime("2026-10-03 09:00", berlin)
		if !ok || !got.Equal(instant2026) {
			t.Fatalf("summer time = %v %v", got.UTC(), ok)
		}
		got, ok = toTime("2026-01-15 09:00", berlin)
		if !ok || !got.Equal(utc(2026, 1, 15, 8, 0, 0, 0)) {
			t.Fatalf("winter time = %v %v", got.UTC(), ok)
		}
		// 02:30 does not exist on 2026-03-29 (clocks jump from 02:00 to 03:00): the
		// local time is normalised forward to 03:30 CEST, which is 01:30 UTC.
		got, ok = toTime("2026-03-29 02:30:00", berlin)
		if !ok || !got.Equal(utc(2026, 3, 29, 1, 30, 0, 0)) {
			t.Fatalf("DST gap = %v %v, want 2026-03-29T01:30:00Z", got.UTC(), ok)
		}
	})
}

func TestToTimeStrict(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	in := time.Date(2026, 10, 3, 9, 0, 0, 0, cest)

	cases := []struct {
		name string
		in   any
		want time.Time
		ok   bool
	}{
		{"RFC3339 offset", "2026-10-03T09:00:00+02:00", in, true},
		{"RFC3339 UTC", "2026-10-03T07:00:00Z", in, true},
		{"RFC3339 fractional seconds", "2026-10-03T07:00:00.25Z", in.Add(250 * time.Millisecond), true},
		{"surrounding whitespace", " 2026-10-03T07:00:00Z ", in, true},
		{"time passthrough", in, in, true},
		{"space layout", "2026-10-03 09:00:00", time.Time{}, false},
		{"zone-less T layout", "2026-10-03T09:00:00", time.Time{}, false},
		{"date only", "2026-10-03", time.Time{}, false},
		{"unix number", 1700000000.0, time.Time{}, false},
		{"free text", "tomorrow", time.Time{}, false},
		{"nil", nil, time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toTimeStrict(tc.in)
			if ok != tc.ok {
				t.Fatalf("toTimeStrict(%#v) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if ok && !got.Equal(tc.want) {
				t.Fatalf("toTimeStrict(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
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

func TestMarshalCompact(t *testing.T) {
	// Compact, sorted keys, no HTML escaping, no trailing newline.
	in := map[string]any{"b": []any{1.0, nil, "x"}, "a": "<b>&amp;</b>"}
	got, err := marshalCompact(in)
	if err != nil {
		t.Fatalf("marshalCompact: unexpected error %v", err)
	}
	if want := `{"a":"<b>&amp;</b>","b":[1,null,"x"]}`; got != want {
		t.Errorf("marshalCompact = %s, want %s", got, want)
	}
	if got != strings.TrimRight(got, "\n") {
		t.Errorf("marshalCompact kept a trailing newline: %q", got)
	}
	if display := compactJSON(in); display != got {
		t.Errorf("compactJSON = %s, want the marshalCompact text %s", display, got)
	}

	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	for name, bad := range map[string]any{
		"NaN":    map[string]any{"x": math.NaN()},
		"Inf":    []any{math.Inf(-1)},
		"cyclic": cyclic,
	} {
		out, err := marshalCompact(bad)
		if err == nil {
			t.Errorf("marshalCompact(%s) = %q, want an error", name, out)
		}
		if out != "" {
			t.Errorf("marshalCompact(%s) returned %q alongside the error, want an empty string", name, out)
		}
		if display := compactJSON(bad); display != "<unserializable>" {
			t.Errorf("compactJSON(%s) = %q, want the placeholder", name, display)
		}
	}
}

func TestEnvLoc(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	var nilEnv *Env
	if got := nilEnv.loc(); got != time.Local {
		t.Errorf("nil Env location = %v, want time.Local", got)
	}
	if got := (&Env{}).loc(); got != time.Local {
		t.Errorf("empty Env location = %v, want time.Local", got)
	}
	if got := (&Env{Location: cest}).loc(); got != cest {
		t.Errorf("Env location = %v, want CEST", got)
	}
}
