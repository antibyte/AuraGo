package ecalc

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// IEC 60063 preferred-number tables: one decade of mantissas in [1, 10) per series. E3 to E24 are the
// hand-picked two-digit values (2.7, 3.0, 3.3, 3.6, 3.9, 4.3, 4.7 and 8.2 differ from the rounded geometric
// progression). E48 and E96 are 10^(k/N) rounded to three digits; E192 is the same except for 9.20 at index 185,
// where the standard lists 9.20 although 10^(185/192) rounds to 9.19.

var e3 = []float64{
	1.0, 2.2, 4.7,
}

var e6 = []float64{
	1.0, 1.5, 2.2, 3.3, 4.7, 6.8,
}

var e12 = []float64{
	1.0, 1.2, 1.5, 1.8, 2.2, 2.7, 3.3, 3.9, 4.7, 5.6, 6.8, 8.2,
}

var e24 = []float64{
	1.0, 1.1, 1.2, 1.3, 1.5, 1.6, 1.8, 2.0, 2.2, 2.4, 2.7, 3.0,
	3.3, 3.6, 3.9, 4.3, 4.7, 5.1, 5.6, 6.2, 6.8, 7.5, 8.2, 9.1,
}

var e48 = []float64{
	1.00, 1.05, 1.10, 1.15, 1.21, 1.27, 1.33, 1.40, 1.47, 1.54, 1.62, 1.69,
	1.78, 1.87, 1.96, 2.05, 2.15, 2.26, 2.37, 2.49, 2.61, 2.74, 2.87, 3.01,
	3.16, 3.32, 3.48, 3.65, 3.83, 4.02, 4.22, 4.42, 4.64, 4.87, 5.11, 5.36,
	5.62, 5.90, 6.19, 6.49, 6.81, 7.15, 7.50, 7.87, 8.25, 8.66, 9.09, 9.53,
}

var e96 = []float64{
	1.00, 1.02, 1.05, 1.07, 1.10, 1.13, 1.15, 1.18, 1.21, 1.24, 1.27, 1.30,
	1.33, 1.37, 1.40, 1.43, 1.47, 1.50, 1.54, 1.58, 1.62, 1.65, 1.69, 1.74,
	1.78, 1.82, 1.87, 1.91, 1.96, 2.00, 2.05, 2.10, 2.15, 2.21, 2.26, 2.32,
	2.37, 2.43, 2.49, 2.55, 2.61, 2.67, 2.74, 2.80, 2.87, 2.94, 3.01, 3.09,
	3.16, 3.24, 3.32, 3.40, 3.48, 3.57, 3.65, 3.74, 3.83, 3.92, 4.02, 4.12,
	4.22, 4.32, 4.42, 4.53, 4.64, 4.75, 4.87, 4.99, 5.11, 5.23, 5.36, 5.49,
	5.62, 5.76, 5.90, 6.04, 6.19, 6.34, 6.49, 6.65, 6.81, 6.98, 7.15, 7.32,
	7.50, 7.68, 7.87, 8.06, 8.25, 8.45, 8.66, 8.87, 9.09, 9.31, 9.53, 9.76,
}

var e192 = []float64{
	1.00, 1.01, 1.02, 1.04, 1.05, 1.06, 1.07, 1.09, 1.10, 1.11, 1.13, 1.14,
	1.15, 1.17, 1.18, 1.20, 1.21, 1.23, 1.24, 1.26, 1.27, 1.29, 1.30, 1.32,
	1.33, 1.35, 1.37, 1.38, 1.40, 1.42, 1.43, 1.45, 1.47, 1.49, 1.50, 1.52,
	1.54, 1.56, 1.58, 1.60, 1.62, 1.64, 1.65, 1.67, 1.69, 1.72, 1.74, 1.76,
	1.78, 1.80, 1.82, 1.84, 1.87, 1.89, 1.91, 1.93, 1.96, 1.98, 2.00, 2.03,
	2.05, 2.08, 2.10, 2.13, 2.15, 2.18, 2.21, 2.23, 2.26, 2.29, 2.32, 2.34,
	2.37, 2.40, 2.43, 2.46, 2.49, 2.52, 2.55, 2.58, 2.61, 2.64, 2.67, 2.71,
	2.74, 2.77, 2.80, 2.84, 2.87, 2.91, 2.94, 2.98, 3.01, 3.05, 3.09, 3.12,
	3.16, 3.20, 3.24, 3.28, 3.32, 3.36, 3.40, 3.44, 3.48, 3.52, 3.57, 3.61,
	3.65, 3.70, 3.74, 3.79, 3.83, 3.88, 3.92, 3.97, 4.02, 4.07, 4.12, 4.17,
	4.22, 4.27, 4.32, 4.37, 4.42, 4.48, 4.53, 4.59, 4.64, 4.70, 4.75, 4.81,
	4.87, 4.93, 4.99, 5.05, 5.11, 5.17, 5.23, 5.30, 5.36, 5.42, 5.49, 5.56,
	5.62, 5.69, 5.76, 5.83, 5.90, 5.97, 6.04, 6.12, 6.19, 6.26, 6.34, 6.42,
	6.49, 6.57, 6.65, 6.73, 6.81, 6.90, 6.98, 7.06, 7.15, 7.23, 7.32, 7.41,
	7.50, 7.59, 7.68, 7.77, 7.87, 7.96, 8.06, 8.16, 8.25, 8.35, 8.45, 8.56,
	8.66, 8.76, 8.87, 8.98, 9.09, 9.20, 9.31, 9.42, 9.53, 9.65, 9.76, 9.88,
}

var eseriesTables = map[string][]float64{
	"E3": e3, "E6": e6, "E12": e12, "E24": e24, "E48": e48, "E96": e96, "E192": e192,
}

// ESeriesIDs lists the supported series from coarse to fine.
var ESeriesIDs = []string{"E3", "E6", "E12", "E24", "E48", "E96", "E192"}

// ESeriesValues returns a copy of the mantissas (one decade, 1 <= m < 10) of a series.
func ESeriesValues(series string) ([]float64, bool) {
	tbl, ok := eseriesTables[series]
	if !ok {
		return nil, false
	}
	return append([]float64(nil), tbl...), true
}

// Nearest returns the standard value of the series closest to v (smallest relative deviation, a tie goes to the
// lower value) and its deviation (value-v)/v in percent. v must be finite and greater than zero.
func Nearest(series string, v float64) (value, devPct float64, err error) {
	tbl, ok := eseriesTables[series]
	if !ok {
		return 0, 0, fmt.Errorf("unknown E-series %q", series)
	}
	if !(v > 0) || math.IsInf(v, 0) {
		return 0, 0, fmt.Errorf("E-series lookup needs a finite value greater than zero, got %v", v)
	}
	d := math.Pow10(int(math.Floor(log10(v))))
	best, bestDev := 0.0, math.Inf(1)
	try := func(c float64) {
		if dev := math.Abs(c-v) / v; dev < bestDev {
			best, bestDev = c, dev
		}
	}
	for _, m := range tbl {
		try(m * d)
	}
	try(10 * d)
	best = roundSig(best, 12)
	return best, (best - v) / v * 100, nil
}

func bigPow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// roundSig rounds x to n significant digits and returns the double nearest to that decimal. Exact decimal ties are
// rounded away from zero, which is what JavaScript's Number(x.toPrecision(n)) does (Go's strconv would round them
// to even: 12345 to 4 digits is 12350 here, 12340 with strconv). It works on the exact binary value of x, so
// 999.95, which lies a hair above the tie, rounds up to 1000 in both languages.
func roundSig(x float64, n int) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	if n < 1 {
		n = 1
	}
	ax := math.Abs(x)
	exact := new(big.Rat).SetFloat64(ax)
	lo := new(big.Rat).SetInt(bigPow10(n - 1))
	hi := new(big.Rat).SetInt(bigPow10(n))
	e := int(math.Floor(log10(ax)))
	for {
		p := e - n + 1 // q = ax / 10^p must lie in [10^(n-1), 10^n)
		q := new(big.Rat)
		if p >= 0 {
			q.Quo(exact, new(big.Rat).SetInt(bigPow10(p)))
		} else {
			q.Mul(exact, new(big.Rat).SetInt(bigPow10(-p)))
		}
		if q.Cmp(lo) < 0 {
			e--
			continue
		}
		if q.Cmp(hi) >= 0 {
			e++
			continue
		}
		num := new(big.Int).Lsh(q.Num(), 1)
		num.Add(num, q.Denom())
		m := num.Quo(num, new(big.Int).Lsh(q.Denom(), 1)) // floor(q + 1/2)
		r, _ := strconv.ParseFloat(m.String()+"e"+strconv.Itoa(p), 64)
		if x < 0 {
			r = -r
		}
		return r
	}
}
