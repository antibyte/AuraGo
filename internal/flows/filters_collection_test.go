package flows

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCollectionFilterParityVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/collection_filters.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string `json:"name"`
		Filter string `json:"filter"`
		Input  any    `json:"input"`
		Args   []any  `json:"args"`
		Want   any    `json:"want"`
		Error  bool   `json:"error"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			before, err := json.Marshal(tc.Input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := applyFilter(tc.Filter, tc.Input, tc.Args, nil)
			if (err != nil) != tc.Error {
				t.Fatalf("error = %v, expected error %v", err, tc.Error)
			}
			if err == nil && !reflect.DeepEqual(got, tc.Want) {
				t.Fatalf("got %#v, want %#v", got, tc.Want)
			}
			after, err := json.Marshal(tc.Input)
			if err != nil || string(before) != string(after) {
				t.Fatalf("input mutated: %s / %s, %v", before, after, err)
			}
		})
	}
}

func TestCollectionFilterLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
	}{
		{"too many items", make([]any, collectionMaxItems+1)},
		{"too much text", []any{strings.Repeat("a", collectionMaxBytes+1)}},
		{"NaN", []any{math.NaN()}},
		{"infinity", []any{math.Inf(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := applyFilter("unique", tc.input, nil, nil); err == nil {
				t.Fatal("expected bounded JSON input rejection")
			}
		})
	}
	fields := make(map[string]any, collectionMaxFields+1)
	for i := 0; i <= collectionMaxFields; i++ {
		fields[strings.Repeat("x", i)] = nil
	}
	if _, err := applyFilter("pick", fields, []any{"x"}, nil); err == nil {
		t.Fatal("expected too many fields rejection")
	}
	if got, err := applyFilter("unique", make([]any, collectionMaxItems), nil, nil); err != nil || len(got.([]any)) != 1 {
		t.Fatalf("boundary: %#v, %v", got, err)
	}
}

func TestCollectionFilterTemplatePipeline(t *testing.T) {
	tpl, err := ParseTemplate(`{{rows | where("active", "eq", true) | sort("n", "desc") | sum("n")}}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.Evaluate(&Env{Roots: map[string]any{"rows": []any{
		map[string]any{"active": true, "n": 2.0},
		map[string]any{"active": false, "n": 10.0},
		map[string]any{"active": true, "n": 3.0},
	}}})
	if err != nil || got != 5.0 {
		t.Fatalf("pipeline = %#v, %v", got, err)
	}
}
