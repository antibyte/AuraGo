package flows

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFilters(t *testing.T) {
	utc := &Env{Location: time.UTC}
	cases := []struct {
		name string
		in   any
		args []any
		want any
	}{
		{"default", nil, []any{"x"}, "x"},
		{"default", "", []any{"x"}, "x"},
		{"default", "a", []any{"x"}, "a"},
		{"default", 0.0, []any{5.0}, 0.0},
		{"truncate", "Hallo Welt", []any{5.0}, "Hallo…"},
		{"truncate", "Hi", []any{5.0}, "Hi"},
		{"truncate", "abc", []any{3.0}, "abc"},
		{"truncate", "abcd", []any{3.0}, "abc…"},
		{"truncate", "äöüß", []any{2.0}, "äö…"},
		{"upper", "abc", nil, "ABC"},
		{"lower", "ÄBC", nil, "äbc"},
		{"trim", "  a  ", nil, "a"},
		{"join", []any{"a", 1.0, true}, nil, "a, 1, true"},
		{"join", []any{"a", "b"}, []any{"|"}, "a|b"},
		{"join", "x", nil, "x"},
		{"split", "a,b", []any{","}, []any{"a", "b"}},
		{"split", "ab", []any{""}, []any{"a", "b"}},
		{"first", []any{"a", "b"}, nil, "a"},
		{"first", []any{}, nil, nil},
		{"first", "abc", nil, "a"},
		{"first", "", nil, ""},
		{"first", "�x", nil, "�"},
		{"last", []any{"a", "b"}, nil, "b"},
		{"last", "abc", nil, "c"},
		{"last", "", nil, ""},
		{"last", "x�", nil, "�"},
		{"count", []any{1.0, 2.0, 3.0}, nil, 3.0},
		{"count", map[string]any{"a": 1.0, "b": 2.0}, nil, 2.0},
		{"count", "äb", nil, 2.0},
		{"count", nil, nil, 0.0},
		{"pluck", []any{map[string]any{"t": "a"}, map[string]any{"t": "b"}, "x"}, []any{"t"}, []any{"a", "b", nil}},
		{"json", map[string]any{"a": 1.0}, nil, `{"a":1}`},
		{"json", "x<y", nil, `"x<y"`},
		{"date", "2026-10-03T07:05:09Z", []any{"DD.MM.YYYY HH:mm:ss"}, "03.10.2026 07:05:09"},
		{"date", "2026-10-03T07:05:09Z", []any{"YYYY-MM-DD um HH Uhr"}, "2026-10-03 um 07 Uhr"},
		{"date", 0.0, []any{"DD.MM.YYYY"}, "01.01.1970"},
		{"round", 3.14159, []any{2.0}, 3.14},
		{"round", "2.5", nil, 3.0},
		{"round", 2.5, nil, 3.0},
		{"round", -2.5, nil, -3.0},
		{"round", -3.14159, []any{2.0}, -3.14},
		{"round", 1.7e308, nil, 1.7e308},
		{"round", 1.7e308, []any{1.0}, 1.7e308},
		{"round", -1.7e308, []any{10.0}, -1.7e308},
		{"replace", "a-b-c", []any{"-", "+"}, "a+b+c"},
		{"replace", "abc", []any{"", "x"}, "abc"},
		{"strip_html", "<p>Hallo&nbsp;<b>Welt</b></p>", nil, "Hallo Welt"},
		{"strip_html", "<p>a</p><p>b</p>", nil, "a b"},
		{"strip_html", "line<br/>two<br>three", nil, "line two three"},
		{"strip_html", `<a href="x" title="t">link</a>`, nil, "link"},
		{"strip_html", "if x < 5 and y > 3 then", nil, "if x < 5 and y > 3 then"},
		{"strip_html", "Max <max@example.com>", nil, "Max <max@example.com>"},
		{"strip_html", "<script>alert(1)</script>hi", nil, "hi"},
		{"strip_html", "a<SCRIPT type=\"x\">\nvar x = 1 < 2;\n</Script >b", nil, "a b"},
		{"strip_html", "<style>p { color: red }</style>Text", nil, "Text"},
		{"strip_html", "a<!-- hidden <b>x</b> -->b", nil, "a b"},
		{"strip_html", "a<!--\nmulti\nline\n-->b", nil, "a b"},
		{"strip_html", "<?xml version=\"1.0\"?><!DOCTYPE html><html><head><title>T</title></head><body>B</body></html>", nil, "T B"},
		{"strip_html", "a<![CDATA[hidden]]>b", nil, "a b"},
		{"strip_html", "<!doctype html>Hi", nil, "Hi"},
		{"strip_html", "Tom &amp; Jerry &lt;3 &eacute;", nil, "Tom & Jerry <3 é"},
		{"strip_html", nil, nil, ""},
	}
	for _, tc := range cases {
		got, err := applyFilter(tc.name, tc.in, tc.args, utc)
		if err != nil {
			t.Errorf("%s(%#v, %v): unexpected error %v", tc.name, tc.in, tc.args, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s(%#v, %v) = %#v, want %#v", tc.name, tc.in, tc.args, got, tc.want)
		}
	}
}

func loadTestZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata missing: %v", err)
	}
	return loc
}

func TestDateFilterTimeZoneArgument(t *testing.T) {
	loadTestZone(t, "Europe/Berlin")
	utc := &Env{Location: time.UTC}
	cases := []struct {
		name string
		in   any
		args []any
		want string
	}{
		{"zoned input converts", "2026-10-03T07:05:09Z", []any{"HH:mm", "Europe/Berlin"}, "09:05"},
		{"zone-less input is read in the Env zone", "2026-10-03 07:05", []any{"HH:mm", "Europe/Berlin"}, "09:05"},
		{"date-only input is read in the Env zone", "2026-10-03", []any{"YYYY-MM-DD HH:mm", "Europe/Berlin"}, "2026-10-03 02:00"},
		{"unix timestamp converts", 1791011109.0, []any{"HH:mm", "Europe/Berlin"}, "09:05"},
		{"offset input converts", "2026-10-03T07:05:09+05:00", []any{"HH:mm", "Europe/Berlin"}, "04:05"},
		{"empty zone name counts as not given", "2026-10-03T07:05:09Z", []any{"HH:mm", ""}, "07:05"},
	}
	for _, tc := range cases {
		got, err := applyFilter("date", tc.in, tc.args, utc)
		if err != nil || got != tc.want {
			t.Errorf("%s: date(%#v, %v) = %#v, %v; want %q", tc.name, tc.in, tc.args, got, err, tc.want)
		}
	}
}

func TestDateFilterUsesEnvZoneWithoutArgument(t *testing.T) {
	berlin := loadTestZone(t, "Europe/Berlin")
	env := &Env{Location: berlin}
	cases := []struct {
		name string
		in   any
		args []any
		want string
	}{
		{"no zone argument shows the Env zone", "2026-10-03T07:05:09Z", []any{"HH:mm"}, "09:05"},
		{"empty zone argument shows the Env zone", "2026-10-03T07:05:09Z", []any{"HH:mm", ""}, "09:05"},
		{"zone-less input is read in the Env zone", "2026-10-03 07:05", []any{"HH:mm"}, "07:05"},
		{"zone-less input converts to the named zone", "2026-10-03 07:05", []any{"HH:mm", "UTC"}, "05:05"},
	}
	for _, tc := range cases {
		got, err := applyFilter("date", tc.in, tc.args, env)
		if err != nil || got != tc.want {
			t.Errorf("%s: date(%#v, %v) = %#v, %v; want %q", tc.name, tc.in, tc.args, got, err, tc.want)
		}
	}
}

func TestDateFilterDaylightSavingTime(t *testing.T) {
	berlin := loadTestZone(t, "Europe/Berlin")
	utc := &Env{Location: time.UTC}
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"last minute of CET", "2026-03-29T00:59:00Z", "2026-03-29 01:59"},
		{"first minute of CEST", "2026-03-29T01:00:00Z", "2026-03-29 03:00"},
		{"CEST ends, first 02:30", "2026-10-25T00:30:00Z", "2026-10-25 02:30"},
		{"CEST ends, second 02:30", "2026-10-25T01:30:00Z", "2026-10-25 02:30"},
		{"CET again", "2026-10-25T02:00:00Z", "2026-10-25 03:00"},
	}
	for _, tc := range cases {
		got, err := applyFilter("date", tc.in, []any{"YYYY-MM-DD HH:mm", "Europe/Berlin"}, utc)
		if err != nil || got != tc.want {
			t.Errorf("%s: date(%v) = %#v, %v; want %q", tc.name, tc.in, got, err, tc.want)
		}
	}

	// A zone-less time inside the spring-forward gap moves forward, like toTime documents.
	got, err := applyFilter("date", "2026-03-29 02:30", []any{"HH:mm"}, &Env{Location: berlin})
	if err != nil || got != "03:30" {
		t.Errorf("date of a time in the DST gap = %#v, %v; want 03:30", got, err)
	}
}

func TestDateFilterUTCAndNilEnv(t *testing.T) {
	// No tzdata needed: "UTC" is always known, and the zoned input does not depend on time.Local.
	got, err := applyFilter("date", "2026-10-03T07:05:09+02:00", []any{"HH:mm", "UTC"}, nil)
	if err != nil || got != "05:05" {
		t.Fatalf("date with nil Env = %#v, %v; want 05:05", got, err)
	}
}

func TestDateFilterCachesZoneLookups(t *testing.T) {
	const bad = "Mars/Olympus"
	for i := 0; i < 3; i++ {
		if _, err := applyFilter("date", "2026-10-03", []any{"YYYY", bad}, nil); err == nil {
			t.Fatal("expected an error for an unknown zone")
		}
	}
	cached, ok := zoneCache.Load(bad)
	if !ok {
		t.Fatal("failed lookup was not cached")
	}
	if res := cached.(zoneLookup); res.err == nil || res.loc != nil {
		t.Fatalf("cached entry = %+v, want the lookup error", res)
	}

	got, err := applyFilter("date", "2026-10-03T07:05:09Z", []any{"HH:mm", "UTC"}, nil)
	if err != nil || got != "07:05" {
		t.Fatalf("date with cached UTC = %#v, %v", got, err)
	}
	if _, ok := zoneCache.Load("UTC"); !ok {
		t.Fatal("successful lookup was not cached")
	}
}

func TestFilterErrors(t *testing.T) {
	cases := []struct {
		name string
		in   any
		args []any
	}{
		{"truncate", "x", []any{0.0}},
		{"truncate", "x", []any{"a"}},
		{"truncate", "x", nil},
		{"truncate", "x", []any{2.5}},
		{"truncate", "x", []any{1e10}},
		{"truncate", "x", []any{-1e10}},
		{"truncate", "x", []any{math.Inf(1)}},
		{"truncate", "x", []any{math.NaN()}},
		{"date", "not a date", []any{"YYYY"}},
		{"date", "2026-10-03", []any{"YYYY", "Mars/Olympus"}},
		{"date", "2026-10-03", []any{"YYYY", 1.0}},
		{"date", "2026-10-03", []any{1.0}},
		{"date", "2026-10-03", []any{"YYYY", "UTC", "extra"}},
		{"date", "2026-10-03", nil},
		{"round", "abc", nil},
		{"round", 1.0, []any{-1.0}},
		{"round", 1.0, []any{11.0}},
		{"round", 1.0, []any{1e10}},
		{"round", math.NaN(), nil},
		{"nope", "x", nil},
		{"upper", "x", []any{"a"}},
		{"default", "x", nil},
		{"default", "x", []any{"a", "b"}},
		{"join", []any{"a", "b"}, []any{1.0}},
		{"join", []any{"a", "b"}, []any{"a", "b"}},
		{"split", "a", []any{1.0}},
		{"split", "a", nil},
		{"pluck", []any{}, []any{1.0}},
		{"pluck", []any{}, nil},
		{"replace", "a", []any{"a"}},
		{"replace", "a", []any{1.0, "b"}},
		{"replace", "a", []any{"a", 1.0}},
	}
	for _, tc := range cases {
		if got, err := applyFilter(tc.name, tc.in, tc.args, nil); err == nil {
			t.Errorf("%s(%#v, %v) = %#v: expected error", tc.name, tc.in, tc.args, got)
		}
	}
}

func TestRoundFilterNeverReturnsNegativeZero(t *testing.T) {
	negZero := math.Copysign(0, -1)
	cases := []struct {
		in   any
		args []any
	}{
		{negZero, nil},
		{negZero, []any{2.0}},
		{-0.4, nil},
		{-0.004, []any{2.0}},
		{"-0.4", nil},
	}
	for _, tc := range cases {
		got, err := applyFilter("round", tc.in, tc.args, nil)
		if err != nil {
			t.Errorf("round(%#v, %v): unexpected error %v", tc.in, tc.args, err)
			continue
		}
		f, ok := got.(float64)
		if !ok || f != 0 || math.Signbit(f) {
			t.Errorf("round(%#v, %v) = %#v, want +0", tc.in, tc.args, got)
		}
	}
}

func TestRoundFilterOverflowKeepsInput(t *testing.T) {
	got, err := applyFilter("round", 1.7e308, []any{1.0}, nil)
	if err != nil {
		t.Fatalf("round(1.7e308, 1): unexpected error %v", err)
	}
	f, ok := got.(float64)
	if !ok || f != 1.7e308 || math.IsInf(f, 0) || math.IsNaN(f) {
		t.Fatalf("round(1.7e308, 1) = %#v, want the input unchanged", got)
	}
}

// The json filter must fail on values JSON cannot represent instead of emitting
// the "<unserializable>" placeholder that Stringify uses for display.
func TestJSONFilterRejectsUnserializableValues(t *testing.T) {
	got, err := applyFilter("json", map[string]any{"x": math.NaN()}, nil, nil)
	if err == nil {
		t.Fatalf("json(NaN) = %#v, want an error", got)
	}
	if got != nil {
		t.Errorf("json(NaN) returned %#v alongside the error, want nil", got)
	}
}

func TestReplaceFilterOutputBound(t *testing.T) {
	// Exactly at the limit is fine.
	atLimit := strings.Repeat("a", maxFilterOutputBytes/2)
	got, err := applyFilter("replace", atLimit, []any{"a", "bb"}, nil)
	if err != nil {
		t.Fatalf("replace at the limit: unexpected error %v", err)
	}
	if s, _ := got.(string); len(s) != maxFilterOutputBytes {
		t.Fatalf("replace at the limit produced %d bytes, want %d", len(s), maxFilterOutputBytes)
	}

	// One byte over is rejected before the result is built.
	over := strings.Repeat("a", maxFilterOutputBytes/2+1)
	if got, err := applyFilter("replace", over, []any{"a", "bb"}, nil); err == nil {
		t.Fatalf("replace over the limit returned %d bytes, want an error", len(got.(string)))
	}

	// The bound applies to the result, not to the input: shrinking a large text works.
	big := strings.Repeat("a", maxFilterOutputBytes+2)
	got, err = applyFilter("replace", big, []any{"aa", "a"}, nil)
	if err != nil {
		t.Fatalf("shrinking replace: unexpected error %v", err)
	}
	if s, _ := got.(string); len(s) != (maxFilterOutputBytes+2)/2 {
		t.Fatalf("shrinking replace produced %d bytes", len(s))
	}
}

func TestSplitFilterPartBound(t *testing.T) {
	atLimit := strings.Repeat(",", maxSplitParts-1)
	got, err := applyFilter("split", atLimit, []any{","}, nil)
	if err != nil {
		t.Fatalf("split at the limit: unexpected error %v", err)
	}
	if parts, _ := got.([]any); len(parts) != maxSplitParts {
		t.Fatalf("split at the limit produced %d parts, want %d", len(parts), maxSplitParts)
	}

	over := strings.Repeat(",", maxSplitParts)
	if got, err := applyFilter("split", over, []any{","}, nil); err == nil {
		t.Fatalf("split over the limit returned %d parts, want an error", len(got.([]any)))
	}

	// An empty separator splits into characters and is bounded the same way.
	got, err = applyFilter("split", strings.Repeat("a", maxSplitParts), []any{""}, nil)
	if err != nil {
		t.Fatalf("split into characters at the limit: unexpected error %v", err)
	}
	if parts, _ := got.([]any); len(parts) != maxSplitParts {
		t.Fatalf("split into characters produced %d parts, want %d", len(parts), maxSplitParts)
	}
	if _, err := applyFilter("split", strings.Repeat("a", maxSplitParts+1), []any{""}, nil); err == nil {
		t.Fatal("split into characters over the limit: expected error")
	}
}

func TestQuoteForErrorTruncates(t *testing.T) {
	exact := strings.Repeat("ä", maxErrorEchoRunes)
	if got, want := quoteForError(exact), `"`+exact+`"`; got != want {
		t.Errorf("quoteForError(40 runes) = %s, want %s", got, want)
	}
	over := exact + "x"
	if got, want := quoteForError(over), `"`+exact+`…"`; got != want {
		t.Errorf("quoteForError(41 runes) = %s, want %s", got, want)
	}
	if got := quoteForError(""); got != `""` {
		t.Errorf("quoteForError(empty) = %s", got)
	}
}

func TestJoinOutputIsBounded(t *testing.T) {
	ones := func(n int) []any {
		items := make([]any, n)
		for i := range items {
			items[i] = "x"
		}
		return items
	}

	// Separators alone can blow the limit: 200 items, 1 MiB separator.
	_, err := applyFilter("join", ones(200), []any{strings.Repeat("-", 1<<20)}, nil)
	if err == nil || !strings.Contains(err.Error(), "join result would exceed") {
		t.Fatalf("200 items x 1 MiB separator: got %v", err)
	}

	// So can the items; the limit counts the parts as join renders them.
	big := strings.Repeat("y", 1<<20)
	nine := []any{big, big, big, big, big, big, big, big, big}
	if _, err = applyFilter("join", nine, []any{","}, nil); err == nil {
		t.Fatal("9 MiB of items: expected an error")
	}
	if _, err = applyFilter("join", nine[:8], []any{""}, nil); err != nil {
		t.Fatalf("exactly 8 MiB of items must pass: %v", err)
	}
	if _, err = applyFilter("join", nine[:8], []any{","}, nil); err == nil {
		t.Fatal("8 MiB of items plus separators: expected an error")
	}

	// The limit is on the result size, to the byte: parts "a" and "b" plus the
	// separator make 2+len(sep).
	got, err := applyFilter("join", []any{"a", "b"}, []any{strings.Repeat("-", maxFilterOutputBytes-2)}, nil)
	if s, _ := got.(string); err != nil || len(s) != maxFilterOutputBytes {
		t.Fatalf("result of exactly %d bytes: len %d, err %v", maxFilterOutputBytes, len(s), err)
	}
	if _, err = applyFilter("join", []any{"a", "b"}, []any{strings.Repeat("-", maxFilterOutputBytes-1)}, nil); err == nil {
		t.Fatal("result one byte over the limit: expected an error")
	}

	// One item or none has no separators, so a huge separator is harmless.
	hugeSep := []any{strings.Repeat("-", maxFilterOutputBytes+1)}
	if got, err = applyFilter("join", []any{"a"}, hugeSep, nil); err != nil || got != "a" {
		t.Errorf("one item: got %#v, %v", got, err)
	}
	if got, err = applyFilter("join", []any{}, hugeSep, nil); err != nil || got != "" {
		t.Errorf("no items: got %#v, %v", got, err)
	}

	// Ordinary joins are unchanged.
	if got, err = applyFilter("join", []any{"a", 1.0, nil, true}, nil, nil); err != nil || got != "a, 1, , true" {
		t.Errorf("small join: got %#v, %v", got, err)
	}
}

func TestTruncateForError(t *testing.T) {
	exact := strings.Repeat("ä", maxErrorEchoRunes)
	if got := truncateForError(exact); got != exact {
		t.Errorf("truncateForError(40 runes) = %q, want it unchanged", got)
	}
	if got := truncateForError(exact + "x"); got != exact+"…" {
		t.Errorf("truncateForError(41 runes) = %q", got)
	}
	// It cuts only; quoting and escaping stay with quoteForError.
	if got := truncateForError("a\"b\n"); got != "a\"b\n" {
		t.Errorf("truncateForError must not quote, got %q", got)
	}
	if got := truncateForError(""); got != "" {
		t.Errorf("truncateForError(empty) = %q", got)
	}
}

func TestFilterErrorsDoNotEchoLongInput(t *testing.T) {
	long := strings.Repeat("ä", 500)
	cases := []struct {
		name string
		in   any
		args []any
	}{
		{"date", long, []any{"YYYY"}},
		{"round", long, nil},
		{"date", "2026-10-03", []any{"YYYY", long}},
		{long, "x", nil},
	}
	for _, tc := range cases {
		_, err := applyFilter(tc.name, tc.in, tc.args, nil)
		if err == nil {
			t.Errorf("%.10s...: expected error", tc.name)
			continue
		}
		msg := err.Error()
		if strings.Contains(msg, long) || len(msg) > 300 || !strings.Contains(msg, "…") {
			t.Errorf("error for a long input is not truncated: %q", msg)
		}
	}

	_, err := applyFilter("round", "abc", nil, nil)
	if err == nil || !strings.Contains(err.Error(), `"abc"`) {
		t.Errorf("short input should still be echoed in full, got %v", err)
	}
}

func TestFilterNamesSorted(t *testing.T) {
	names := FilterNames()
	if len(names) != 16 {
		t.Fatalf("FilterNames() has %d entries, want 16: %v", len(names), names)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("FilterNames not sorted: %v", names)
		}
	}
}
