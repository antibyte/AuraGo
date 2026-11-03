package ecalc

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// Generated tables (generate: in tables/*.yaml): the columns and rows come from formulas and from the engine's own
// data, so they cannot disagree with them. The item in the YAML carries every other field (name, doc, notes, source,
// group, tags). The columns written here are validated like the columns of a transcribed table, and the rows are
// already in SI units.

// genResult is the output of a generator: the columns as the YAML would list them, the default key column and the rows.
type genResult struct {
	cols []rawColumn
	key  string
	rows [][]Cell
}

var tableGenerators = map[string]func() genResult{
	"awg":          genAWG,
	"ipc2221":      genIPC2221,
	"eseries_e24":  func() genResult { return genESeries([]string{"E3", "E6", "E12", "E24"}) },
	"eseries_e192": func() genResult { return genESeries([]string{"E48", "E96", "E192"}) },
}

func generatorIDs() []string {
	ids := make([]string, 0, len(tableGenerators))
	for id := range tableGenerators {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func en2(en, de string) Text { return Text{"en": en, "de": de} }

func numCell(x float64) Cell { return Cell{Num: x} }

func textCell(s string) Cell { return Cell{Num: math.NaN(), Text: Text{"en": s}} }

// Constants of the formulas. The values are SI.
const (
	awgBaseDiameter     = 0.000127  // m, AWG 36 = 0.005 in
	copperResistivity20 = 1e-6 / 58 // Ohm*m, annealed copper at 20 degC: 1/58 Ohm*mm2/m (IEC 60028)
	milMetres           = 25.4e-6   // m, one mil
	copperOunce         = 35e-6     // m, the thickness of 1 oz/ft2 copper (the unit oz)
)

// awgDiameter is the ASTM B258 diameter of gauge n: 0.127 mm * 92^((36 - n) / 39); 4/0 is n = -3, 3/0 is -2, 2/0 is -1
// and 1/0 is 0.
func awgDiameter(n int) float64 { return awgBaseDiameter * math.Pow(92, float64(36-n)/39) }

// awgSize is the name of gauge n: "4/0" for -3 ... "1/0" for 0, then "1" ... "40".
func awgSize(n int) string {
	if n < 1 {
		return fmt.Sprintf("%d/0", 1-n)
	}
	return fmt.Sprint(n)
}

// genAWG is the AWG wire table, AWG 4/0 to 40: diameter, cross-section and the DC resistance of 1 km of annealed
// copper at 20 degC.
func genAWG() genResult {
	g := genResult{key: "size", cols: []rawColumn{
		{ID: "size", Name: en2("AWG size", "AWG-Größe"), Dim: textDim},
		{ID: "awg", Name: en2("Gauge number (4/0 = -3, 1/0 = 0)", "Nummer (4/0 = -3, 1/0 = 0)"), Dim: "number"},
		{ID: "diameter", Name: en2("Diameter", "Durchmesser"), Dim: "length", Units: []string{"", "in", "mil"}, Label: "d"},
		{ID: "area", Name: en2("Cross-section", "Querschnitt"), Dim: "area", Unit: "mm2", Units: []string{"mm2", "kcmil"}, Label: "A"},
		{ID: "r_km", Name: en2("DC resistance per km (copper, 20 °C)", "Gleichstromwiderstand je km (Kupfer, 20 °C)"), Dim: "resistance", Label: "R"},
	}}
	for n := -3; n <= 40; n++ {
		d := awgDiameter(n)
		area := math.Pi / 4 * d * d
		g.rows = append(g.rows, []Cell{textCell(awgSize(n)), numCell(float64(n)), numCell(d), numCell(area), numCell(copperResistivity20 * 1000 / area)})
	}
	return g
}

// ipcWidth is the trace width in metres that carries current amps at a temperature rise dT (kelvin) in a layer of the
// given weight (ounces of copper): IPC-2221 gives I = k * dT^0.44 * A^0.725 with A the cross-section in square mils,
// k = 0.048 for an outer and 0.024 for an inner layer.
func ipcWidth(amps, k, dT, ounces float64) float64 {
	areaMil2 := math.Pow(amps/(k*math.Pow(dT, 0.44)), 1/0.725)
	return areaMil2 * milMetres * milMetres / (ounces * copperOunce)
}

// genIPC2221 is the trace width grid of IPC-2221: for currents of 0.25 A to 10 A the width of an external and an
// internal trace in 1 oz copper at a rise of 10 K and 20 K and in 2 oz copper at 10 K.
func genIPC2221() genResult {
	type variant struct {
		id, en, de string
		k, dT, oz  float64
	}
	variants := []variant{
		{"ext_1oz_10", "External, 1 oz, rise 10 °C", "Außenlage, 1 oz, Erwärmung 10 °C", 0.048, 10, 1},
		{"ext_1oz_20", "External, 1 oz, rise 20 °C", "Außenlage, 1 oz, Erwärmung 20 °C", 0.048, 20, 1},
		{"ext_2oz_10", "External, 2 oz, rise 10 °C", "Außenlage, 2 oz, Erwärmung 10 °C", 0.048, 10, 2},
		{"int_1oz_10", "Internal, 1 oz, rise 10 °C", "Innenlage, 1 oz, Erwärmung 10 °C", 0.024, 10, 1},
		{"int_1oz_20", "Internal, 1 oz, rise 20 °C", "Innenlage, 1 oz, Erwärmung 20 °C", 0.024, 20, 1},
		{"int_2oz_10", "Internal, 2 oz, rise 10 °C", "Innenlage, 2 oz, Erwärmung 10 °C", 0.024, 10, 2},
	}
	g := genResult{key: "current", cols: []rawColumn{{ID: "current", Name: en2("Current", "Strom"), Dim: "current", Label: "I"}}}
	for _, v := range variants {
		g.cols = append(g.cols, rawColumn{ID: v.id, Name: en2(v.en, v.de), Dim: "length", Unit: "mm", Units: []string{"mm", "mil"}, Label: "w"})
	}
	for _, amps := range []float64{0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 5, 6, 8, 10} {
		row := []Cell{numCell(amps)}
		for _, v := range variants {
			row = append(row, numCell(ipcWidth(amps, v.k, v.dT, v.oz)))
		}
		g.rows = append(g.rows, row)
	}
	return g
}

// genESeries is a table of the IEC 60063 mantissas of the finest listed series with one text column per listed series
// that marks (with a dot) the values that belong to it.
func genESeries(series []string) genResult {
	finest := series[len(series)-1]
	mantissas := eseriesTables[finest]
	g := genResult{key: "value", cols: []rawColumn{{ID: "value", Name: en2("Value (one decade, 1 to 10)", "Wert (eine Dekade, 1 bis 10)"), Dim: "number"}}}
	for _, s := range series {
		g.cols = append(g.cols, rawColumn{ID: strings.ToLower(s), Name: en2(s, s), Dim: textDim})
	}
	for _, m := range mantissas {
		row := []Cell{numCell(m)}
		for _, s := range series {
			mark := ""
			if slices.Contains(eseriesTables[s], m) {
				mark = "●"
			}
			row = append(row, textCell(mark))
		}
		g.rows = append(g.rows, row)
	}
	return g
}
