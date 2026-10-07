package flows

import (
	"strings"
	"testing"
	"time"
)

func TestConditionRows(t *testing.T) {
	cases := []struct {
		row  ConditionRow
		want bool
	}{
		{ConditionRow{Left: 5.0, Op: "gt", Right: "3"}, true},
		{ConditionRow{Left: "10", Op: "gt", Right: "9"}, true},
		{ConditionRow{Left: "b", Op: "gt", Right: "a"}, true},
		{ConditionRow{Left: "Hallo Welt", Op: "contains", Right: "WELT"}, true},
		{ConditionRow{Left: "Hallo", Op: "not_contains", Right: "x"}, true},
		{ConditionRow{Left: []any{"a", "B"}, Op: "contains", Right: "b"}, true},
		{ConditionRow{Left: map[string]any{"k": 1.0}, Op: "contains", Right: "k"}, true},
		{ConditionRow{Left: "Hallo", Op: "starts_with", Right: "ha"}, true},
		{ConditionRow{Left: "Hallo", Op: "ends_with", Right: "LO"}, true},
		{ConditionRow{Left: "abc123", Op: "matches", Right: `^[a-z]+\d+$`}, true},
		{ConditionRow{Left: "Abc", Op: "eq", Right: "abc"}, false},
		{ConditionRow{Left: 1.0, Op: "eq", Right: "1"}, true},
		{ConditionRow{Left: true, Op: "eq", Right: "true"}, true},
		{ConditionRow{Left: "x", Op: "ne", Right: "y"}, true},
		{ConditionRow{Left: "", Op: "empty"}, true},
		{ConditionRow{Left: []any{}, Op: "empty"}, true},
		{ConditionRow{Left: "x", Op: "not_empty"}, true},
		{ConditionRow{Left: "ja", Op: "is_true"}, true},
		{ConditionRow{Left: 0.0, Op: "is_false"}, true},
		{ConditionRow{Left: "2026-10-03T07:00:00Z", Op: "before", Right: "2026-10-04"}, true},
		{ConditionRow{Left: "2026-10-03T07:00:00Z", Op: "after", Right: "2026-10-03T06:00:00Z"}, true},
		{ConditionRow{Left: "2026-10-03T07:00:00Z", Op: "lt", Right: "2026-10-03T08:00:00+00:00"}, true},
		{ConditionRow{Left: "5", Op: "lte", Right: 5.0, Type: "number"}, true},
		{ConditionRow{Left: "10", Op: "lt", Right: "9", Type: "text"}, true},
		{ConditionRow{Left: 3.0, Op: "gte", Right: 3.0}, true},
		{ConditionRow{Left: 1.7e9, Op: "gt", Right: 1.6e9, Type: "date"}, true},
		{ConditionRow{Left: "2026-10-03", Op: "lt", Right: "2026-10-04", Type: "date"}, true},
		{ConditionRow{Left: "yes", Op: "eq", Right: true, Type: "bool"}, true},
		{ConditionRow{Left: "no", Op: "lt", Right: true, Type: "bool"}, true},
		{ConditionRow{Left: true, Op: "gt", Right: false}, true},
		// contains on an object checks an exact, case-sensitive key.
		{ConditionRow{Left: map[string]any{"K": 1.0}, Op: "contains", Right: "k"}, false},
		{ConditionRow{Left: "not an address", Op: "matches", Right: `^[\w.+-]+@[\w-]+\.[\w.]+$`}, false},
		{ConditionRow{Left: "jo.doe+x@mail-host.example.org", Op: "matches", Right: `^[\w.+-]+@[\w-]+\.[\w.]+$`}, true},
	}
	for _, tc := range cases {
		got, err := tc.row.Evaluate(time.UTC)
		if err != nil {
			t.Errorf("%+v: unexpected error %v", tc.row, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%+v = %v, want %v", tc.row, got, tc.want)
		}
	}
}

func TestConditionRowErrors(t *testing.T) {
	for _, row := range []ConditionRow{
		{Left: "x", Op: "before", Right: "2026-10-04"},
		{Left: "x", Op: "matches", Right: "("},
		{Left: "x", Op: "matches", Right: strings.Repeat("a", 201)},
		{Left: "abc", Op: "gt", Right: 1.0, Type: "number"},
		{Left: "x", Op: "foo", Right: "y"},
		{Left: "x", Op: "eq", Right: "y", Type: "weird"},
		{Left: "x", Op: "eq", Right: "2026-10-04", Type: "date"},
	} {
		if _, err := row.Evaluate(time.UTC); err == nil {
			t.Errorf("%+v: expected error", row)
		}
	}
}

func TestConditionMatchesProgramCap(t *testing.T) {
	// 176 bytes, well under maxPatternLength, but about 25,000 instructions: matching
	// it against a large input would run for seconds and cannot be cancelled.
	wide := strings.Repeat("a{1000}", 25) + "b"
	if len(wide) > maxPatternLength {
		t.Fatalf("test pattern is %d bytes, want at most %d", len(wide), maxPatternLength)
	}
	input := strings.Repeat("a", 64*1024)
	for _, pattern := range []string{wide, strings.Repeat(`\pL{1000}`, 20) + "b"} {
		start := time.Now()
		_, err := ConditionRow{Left: input, Op: "matches", Right: pattern}.Evaluate(time.UTC)
		if err == nil || !strings.Contains(err.Error(), "too complex") {
			t.Errorf("pattern of %d bytes: error = %v, want a too-complex error", len(pattern), err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("pattern of %d bytes took %v to reject", len(pattern), elapsed)
		}
	}
	// An ordinary pattern stays far below the cap and still matches.
	email := `^[\w.+-]+@[\w-]+\.[\w.]+$`
	got, err := ConditionRow{Left: "a.b+c@host-1.example.org", Op: "matches", Right: email}.Evaluate(time.UTC)
	if err != nil || !got {
		t.Errorf("email pattern = %v, %v; want true", got, err)
	}
	_, err = ConditionRow{Left: "x", Op: "matches", Right: strings.Repeat("a", 201)}.Evaluate(time.UTC)
	if err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("long pattern error = %v, want it to count bytes", err)
	}
}

func TestDecodeConditionGroupErrorHidesGoTypes(t *testing.T) {
	_, err := DecodeConditionGroup("text")
	if err == nil || strings.Contains(err.Error(), "Go") || strings.Contains(err.Error(), "flows.") {
		t.Errorf("decode error leaks Go types: %v", err)
	}
}

func TestConditionErrorsAreBounded(t *testing.T) {
	long := strings.Repeat("ü", 150)
	// An unbalanced pattern of 150 runes: the error may name the problem and a short
	// fragment, never the whole pattern.
	_, err := ConditionRow{Left: "x", Op: "matches", Right: "(" + long}.Evaluate(time.UTC)
	if err == nil || strings.Contains(err.Error(), long) || len(err.Error()) > 200 {
		t.Errorf("matches error not bounded: %v", err)
	}
	_, err = ConditionRow{Left: "x", Op: long, Right: "y"}.Evaluate(time.UTC)
	if err == nil || strings.Contains(err.Error(), long) {
		t.Errorf("operator error not bounded: %v", err)
	}
	_, err = ConditionRow{Left: "x", Op: "eq", Right: "y", Type: long}.Evaluate(time.UTC)
	if err == nil || strings.Contains(err.Error(), long) {
		t.Errorf("type error not bounded: %v", err)
	}
	for _, bad := range []map[string]any{
		{"match": long},
		{"rows": []any{map[string]any{"op": long}}},
		{"rows": []any{map[string]any{"op": "eq", "type": long}}},
	} {
		if _, err := DecodeConditionGroup(bad); err == nil || strings.Contains(err.Error(), long) {
			t.Errorf("decode error not bounded: %v", err)
		}
	}
}

func TestDecodeConditionGroupTypedNil(t *testing.T) {
	// A typed nil marshals to null; it must not turn into an empty group that is true.
	for _, v := range []any{map[string]any(nil), []any(nil), (*ConditionGroup)(nil)} {
		if _, err := DecodeConditionGroup(v); err == nil {
			t.Errorf("DecodeConditionGroup(%#v) succeeded, want error", v)
		}
	}
}

func TestDecodeConditionGroupDoesNotAlias(t *testing.T) {
	left := map[string]any{"k": []any{"a"}}
	src := map[string]any{"rows": []any{map[string]any{"left": left, "op": "not_empty"}}}
	g, err := DecodeConditionGroup(src)
	if err != nil {
		t.Fatal(err)
	}
	g.Rows[0].Left.(map[string]any)["k"] = "changed"
	if _, ok := left["k"].([]any); !ok {
		t.Fatalf("decoded group aliases its input: %v", left)
	}
}

func TestConditionRowDoesNotMutateInputs(t *testing.T) {
	list := []any{"a", "B"}
	obj := map[string]any{"k": []any{1.0}}
	for _, row := range []ConditionRow{
		{Left: list, Op: "contains", Right: "b"},
		{Left: obj, Op: "contains", Right: "k"},
		{Left: list, Op: "matches", Right: "a"},
		{Left: obj, Op: "eq", Right: list},
		{Left: list, Op: "empty"},
	} {
		if _, err := row.Evaluate(time.UTC); err != nil {
			t.Fatal(err)
		}
	}
	if len(list) != 2 || list[0] != "a" || list[1] != "B" || len(obj) != 1 {
		t.Fatalf("inputs changed: %v %v", list, obj)
	}
}

func TestConditionGroup(t *testing.T) {
	yes := ConditionRow{Left: "a", Op: "eq", Right: "a"}
	no := ConditionRow{Left: "a", Op: "eq", Right: "b"}
	cases := []struct {
		g    ConditionGroup
		want bool
	}{
		{ConditionGroup{Match: "all", Rows: []ConditionRow{yes, yes}}, true},
		{ConditionGroup{Match: "all", Rows: []ConditionRow{yes, no}}, false},
		{ConditionGroup{Rows: []ConditionRow{yes, no}}, false},
		{ConditionGroup{Match: "any", Rows: []ConditionRow{no, yes}}, true},
		{ConditionGroup{Match: "any", Rows: []ConditionRow{no, no}}, false},
		{ConditionGroup{Match: "all"}, true},
		{ConditionGroup{Match: "any"}, false},
	}
	for i, tc := range cases {
		got, err := tc.g.Evaluate(time.UTC)
		if err != nil || got != tc.want {
			t.Errorf("case %d = %v, %v; want %v", i, got, err, tc.want)
		}
	}
}

func TestConditionGroupErrorAfterTrueRow(t *testing.T) {
	// Built directly, not via Decode: an "all" group stops at the first false row,
	// but a true row followed by an invalid one must surface the error.
	g := ConditionGroup{Match: "all", Rows: []ConditionRow{
		{Left: "a", Op: "eq", Right: "a"},
		{Left: "a", Op: "foo", Right: "a"},
	}}
	if got, err := g.Evaluate(time.UTC); err == nil || got {
		t.Errorf("Evaluate = %v, %v; want false and an error", got, err)
	}
}

func TestDecodeConditionGroup(t *testing.T) {
	g, err := DecodeConditionGroup(map[string]any{
		"match": "any",
		"rows":  []any{map[string]any{"left": 5.0, "op": "gt", "right": "3"}},
	})
	if err != nil || g.Match != "any" || len(g.Rows) != 1 || g.Rows[0].Left != 5.0 {
		t.Fatalf("decode = %+v, %v", g, err)
	}
	for _, bad := range []any{
		nil,
		"text",
		map[string]any{"match": "some"},
		map[string]any{"rows": []any{map[string]any{"op": "nope"}}},
		map[string]any{"rows": []any{map[string]any{"op": "eq", "type": "weird"}}},
	} {
		if _, err := DecodeConditionGroup(bad); err == nil {
			t.Errorf("DecodeConditionGroup(%#v) succeeded, want error", bad)
		}
	}
	if !ValidOperator("matches") || ValidOperator("nope") {
		t.Fatal("ValidOperator mismatch")
	}
	if len(ConditionOperators()) != 17 {
		t.Fatalf("ConditionOperators() = %v", ConditionOperators())
	}
}
