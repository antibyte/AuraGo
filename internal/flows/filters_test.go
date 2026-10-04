package flows

import (
	"math"
	"reflect"
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
		{"truncate", "äöüß", []any{2.0}, "äö…"},
		{"upper", "abc", nil, "ABC"},
		{"lower", "ÄBC", nil, "äbc"},
		{"trim", "  a  ", nil, "a"},
		{"join", []any{"a", 1.0, true}, nil, "a, 1, true"},
		{"join", []any{"a", "b"}, []any{"|"}, "a|b"},
		{"join", "x", nil, "x"},
		{"split", "a,b", []any{","}, []any{"a", "b"}},
		{"first", []any{"a", "b"}, nil, "a"},
		{"first", []any{}, nil, nil},
		{"first", "abc", nil, "a"},
		{"last", []any{"a", "b"}, nil, "b"},
		{"last", "abc", nil, "c"},
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
		{"replace", "a-b-c", []any{"-", "+"}, "a+b+c"},
		{"strip_html", "<p>Hallo&nbsp;<b>Welt</b></p>", nil, "Hallo Welt"},
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

func TestDateFilterTimeZoneArgument(t *testing.T) {
	if _, err := time.LoadLocation("Europe/Berlin"); err != nil {
		t.Skip("tzdata missing")
	}
	got, err := applyFilter("date", "2026-10-03T07:05:09Z", []any{"HH:mm", "Europe/Berlin"}, &Env{Location: time.UTC})
	if err != nil || got != "09:05" {
		t.Fatalf("date with tz = %#v, %v", got, err)
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
		{"date", "not a date", []any{"YYYY"}},
		{"date", "2026-10-03", []any{"YYYY", "Mars/Olympus"}},
		{"round", "abc", nil},
		{"nope", "x", nil},
	}
	for _, tc := range cases {
		if _, err := applyFilter(tc.name, tc.in, tc.args, nil); err == nil {
			t.Errorf("%s(%#v, %v): expected error", tc.name, tc.in, tc.args)
		}
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
