package ecalc

import (
	"fmt"
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

// combineVector is a Combine case: the entries Combine(Series, Target, Limit) must return, in this order. The same
// shape is used by the combine section of core.json and by the combos of a calculator with finder: combine.
type combineVector struct {
	Series string  `json:"series"`
	Target float64 `json:"target"`
	Limit  int     `json:"limit"`
	Ref    string  `json:"ref"`
	Want   []Combo `json:"want"`
}

// checkCombine runs one combineVector: the same entries in the same order, r1, r2 and value at 1e-12 and the
// deviation at 1e-9 relative error.
func checkCombine(t *testing.T, v combineVector) {
	t.Helper()
	name := fmt.Sprintf("Combine(%s, %v, %d)", v.Series, v.Target, v.Limit)
	if v.Ref == "" {
		t.Errorf("%s: the vector has no ref", name)
	}
	got, err := Combine(v.Series, v.Target, v.Limit)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	if len(got) != len(v.Want) {
		t.Errorf("%s returned %d entries, want %d: %+v", name, len(got), len(v.Want), got)
		return
	}
	for i, want := range v.Want {
		g := got[i]
		if g.Kind != want.Kind || !near(g.R1, want.R1, 1e-12) || !near(g.R2, want.R2, 1e-12) || !near(g.Value, want.Value, 1e-12) || !near(g.DevPct, want.DevPct, 1e-9) {
			t.Errorf("%s entry %d = %+v, want %+v", name, i, g, want)
		}
	}
}

func TestCombineVectors(t *testing.T) {
	var f struct {
		Combine []combineVector `json:"combine"`
		Count   []struct {
			Series string  `json:"series"`
			Target float64 `json:"target"`
			Limit  int     `json:"limit"`
			Want   int     `json:"want"`
		} `json:"combine_count"`
		Errors []struct {
			Series string `json:"series"`
			Target any    `json:"target"`
		} `json:"combine_error"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Combine) < 6 || len(f.Count) == 0 || len(f.Errors) == 0 {
		t.Fatalf("core.json has %d combine, %d combine_count and %d combine_error vectors, want at least 6, 1 and 1", len(f.Combine), len(f.Count), len(f.Errors))
	}
	for _, v := range f.Combine {
		checkCombine(t, v)
	}
	for _, v := range f.Count {
		if got, err := Combine(v.Series, v.Target, v.Limit); err != nil || len(got) != v.Want {
			t.Errorf("Combine(%s, %v, %d) returned %d entries (%v), want %d", v.Series, v.Target, v.Limit, len(got), err, v.Want)
		}
	}
	for _, v := range f.Errors {
		if got, err := Combine(v.Series, vecNum(t, v.Target), 5); err == nil {
			t.Errorf("Combine(%q, %v) = %+v, want an error", v.Series, v.Target, got)
		}
	}
}

// Properties that hold for every series and target, independent of the vectors: the single entry is the nearest value,
// series pairs have r1 >= r2 within [target/1000, target], parallel pairs r1 <= r2 within [target, 1000*target], every
// value is a three-digit value of the series, the entries of a group are ordered and no pair appears twice.
func TestCombineInvariants(t *testing.T) {
	for _, series := range ESeriesIDs {
		mantissas, _ := ESeriesValues(series)
		isStandard := func(r float64) bool {
			m := r / math.Pow10(int(math.Floor(math.Log10(r))))
			for _, x := range mantissas {
				if math.Abs(m-x) < 1e-9 {
					return true
				}
			}
			return false
		}
		for _, target := range []float64{1, 4.7, 5000, 123456, 0.0033} {
			got, err := Combine(series, target, 7)
			if err != nil {
				t.Fatalf("Combine(%s, %v): %v", series, target, err)
			}
			nearest, dev, _ := Nearest(series, target)
			if got[0] != (Combo{Kind: "single", R1: nearest, Value: nearest, DevPct: dev}) {
				t.Errorf("%s %v: first entry %+v, want the single nearest value %v", series, target, got[0], nearest)
			}
			seen := map[[3]float64]bool{}
			var lastKind string
			var lastDev, lastR1, lastR2 float64
			counts := map[string]int{}
			for _, c := range got[1:] {
				counts[c.Kind]++
				if !isStandard(c.R1) || roundSig(c.R1, 3) != c.R1 || !isStandard(c.R2) || roundSig(c.R2, 3) != c.R2 {
					t.Errorf("%s %v: %+v uses a value that is not a three-digit %s value", series, target, c, series)
				}
				key := [3]float64{c.R1, c.R2, float64(len(c.Kind))}
				if seen[key] {
					t.Errorf("%s %v: the pair %v, %v appears twice", series, target, c.R1, c.R2)
				}
				seen[key] = true
				switch c.Kind {
				case "series":
					if c.R1 < c.R2 || c.R1 > target || c.R2 < target/1000 || c.R1+c.R2 != c.Value {
						t.Errorf("%s %v: bad series entry %+v", series, target, c)
					}
				case "parallel":
					if c.R1 > c.R2 || c.R1 < target || c.R2 > 1000*target || c.R1*c.R2/(c.R1+c.R2) != c.Value {
						t.Errorf("%s %v: bad parallel entry %+v", series, target, c)
					}
				default:
					t.Errorf("%s %v: unexpected kind %q", series, target, c.Kind)
				}
				if !near(c.DevPct, (c.Value-target)/target*100, 1e-12) {
					t.Errorf("%s %v: %+v has the deviation %v", series, target, c, (c.Value-target)/target*100)
				}
				if c.Kind == lastKind {
					q, lastQ := math.Round(math.Abs(c.DevPct)*1e9), math.Round(math.Abs(lastDev)*1e9)
					if q < lastQ || (q == lastQ && (c.R1 < lastR1 || (c.R1 == lastR1 && c.R2 <= lastR2))) {
						t.Errorf("%s %v: %+v is out of order after |dev| %v r1 %v r2 %v", series, target, c, lastDev, lastR1, lastR2)
					}
				}
				lastKind, lastDev, lastR1, lastR2 = c.Kind, c.DevPct, c.R1, c.R2
			}
			if counts["series"] != 7 || counts["parallel"] != 7 {
				t.Errorf("%s %v: %d series and %d parallel entries, want 7 each", series, target, counts["series"], counts["parallel"])
			}
		}
	}
	// A larger limit only appends: the entries of a smaller limit are the first ones of each group.
	small, _ := Combine("E96", 5000, 5)
	big, _ := Combine("E96", 5000, 40)
	if len(small) != 11 || len(big) != 81 || !reflect.DeepEqual(small[:6], big[:6]) || !reflect.DeepEqual(small[6:], big[41:46]) {
		t.Errorf("limit 5 is not the head of limit 40: %d and %d entries", len(small), len(big))
	}
	// A limit below one leaves the single entry.
	if got, _ := Combine("E3", 100, 0); len(got) != 1 || got[0].Kind != "single" {
		t.Errorf("limit 0 gave %+v, want only the single entry", got)
	}
}

// BenchmarkCombine measures the finest series, the slowest case (about 170 000 pairs per group): go test -bench Combine.
func BenchmarkCombine(b *testing.B) {
	for _, series := range []string{"E24", "E96", "E192"} {
		b.Run(series, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Combine(series, 5000, 5); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
