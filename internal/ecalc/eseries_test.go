package ecalc

import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func TestESeriesVectors(t *testing.T) {
	var f struct {
		RoundSig []struct {
			X    float64 `json:"x"`
			N    int     `json:"n"`
			Want float64 `json:"want"`
		} `json:"round_sig"`
		Nearest []struct {
			Series string  `json:"series"`
			V      float64 `json:"v"`
			Want   float64 `json:"want"`
			Dev    float64 `json:"dev"`
		} `json:"eseries"`
		NearestError []struct {
			Series string `json:"series"`
			V      any    `json:"v"`
		} `json:"eseries_error"`
		Tables map[string][]float64 `json:"eseries_tables"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.RoundSig) == 0 || len(f.Nearest) == 0 {
		t.Fatal("core.json lacks the round_sig / eseries sections")
	}
	for _, v := range f.RoundSig {
		if got := roundSig(v.X, v.N); got != v.Want {
			t.Errorf("roundSig(%v, %d) = %v, want %v", v.X, v.N, got, v.Want)
		}
	}
	for _, v := range f.Nearest {
		got, dev, err := Nearest(v.Series, v.V)
		if err != nil {
			t.Errorf("Nearest(%s, %v): %v", v.Series, v.V, err)
			continue
		}
		if !near(got, v.Want, 1e-12) || !near(dev, v.Dev, 1e-9) {
			t.Errorf("Nearest(%s, %v) = %v (%v %%), want %v (%v %%)", v.Series, v.V, got, dev, v.Want, v.Dev)
		}
	}
	for _, v := range f.NearestError {
		if got, _, err := Nearest(v.Series, vecNum(t, v.V)); err == nil {
			t.Errorf("Nearest(%s, %v) = %v, want an error", v.Series, v.V, got)
		}
	}
	if len(f.Tables) != len(ESeriesIDs) {
		t.Errorf("core.json carries %d eseries_tables, want %d", len(f.Tables), len(ESeriesIDs))
	}
	for id, want := range f.Tables {
		if got, _ := ESeriesValues(id); !reflect.DeepEqual(got, want) {
			t.Errorf("eseries_tables[%s] differs from the Go table", id)
		}
	}
}

func TestESeriesTables(t *testing.T) {
	wantLen := map[string]int{"E3": 3, "E6": 6, "E12": 12, "E24": 24, "E48": 48, "E96": 96, "E192": 192}
	for _, id := range ESeriesIDs {
		vals, ok := ESeriesValues(id)
		if !ok || len(vals) != wantLen[id] {
			t.Fatalf("%s: ok=%v len=%d, want %d values", id, ok, len(vals), wantLen[id])
		}
		if !sort.Float64sAreSorted(vals) || vals[0] != 1 || vals[len(vals)-1] >= 10 {
			t.Errorf("%s is not an ascending decade starting at 1 below 10", id)
		}
	}
	subset := func(small, big string) {
		bigVals, _ := ESeriesValues(big)
		in := map[float64]bool{}
		for _, v := range bigVals {
			in[v] = true
		}
		smallVals, _ := ESeriesValues(small)
		for _, v := range smallVals {
			if !in[v] {
				t.Errorf("%s value %v is missing from %s", small, v, big)
			}
		}
	}
	subset("E3", "E6")
	subset("E6", "E12")
	subset("E12", "E24")
	subset("E48", "E96")
	subset("E96", "E192")

	// The hand-picked IEC 60063 values that differ from the rounded progression.
	e24, _ := ESeriesValues("E24")
	have := map[float64]bool{}
	for _, v := range e24 {
		have[v] = true
	}
	for _, v := range []float64{2.7, 3.0, 3.3, 3.6, 3.9, 4.3, 4.7, 8.2} {
		if !have[v] {
			t.Errorf("E24 lacks %v", v)
		}
	}

	// E48, E96 and E192 are 10^(k/N) rounded to three digits (E192 index 185 is 9.20 in the standard, 9.19 rounded).
	for _, n := range []int{48, 96, 192} {
		vals, _ := ESeriesValues("E" + strconv.Itoa(n))
		for k, v := range vals {
			want := roundSig(math.Pow(10, float64(k)/float64(n)), 3)
			if n == 192 && k == 185 {
				want = 9.20
			}
			if v != want {
				t.Errorf("E%d[%d] = %v, want %v", n, k, v, want)
			}
		}
	}
}
