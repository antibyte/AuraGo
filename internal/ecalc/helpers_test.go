package ecalc

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// readJSON loads a JSON file below the package directory (go test runs in it).
func readJSON(t *testing.T, rel string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
}

// vecNum converts a vector number: a JSON number, or one of the strings "nan", "inf" and "-inf".
func vecNum(t *testing.T, v any) float64 {
	t.Helper()
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "nan":
			return math.NaN()
		case "inf":
			return math.Inf(1)
		case "-inf":
			return math.Inf(-1)
		}
	}
	t.Fatalf("bad vector number %#v", v)
	return 0
}

// near reports whether got matches want: NaN equals NaN, infinities must be identical, a zero want allows an
// absolute error of 1e-12, everything else must agree within the relative error rel.
func near(got, want, rel float64) bool {
	switch {
	case math.IsNaN(want):
		return math.IsNaN(got)
	case math.IsInf(want, 0):
		return got == want
	case want == 0:
		return math.Abs(got) <= 1e-12
	}
	return math.Abs(got-want) <= rel*math.Abs(want)
}
