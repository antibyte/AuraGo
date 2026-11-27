package tools

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"aurago/internal/ecalc"
)

// ecalcLevelFixture is a calculator whose only inputs are two choices (dof 0), next to a normal calculator.
const ecalcLevelFixture = `- id: t_levels
  kind: calculator
  category: basics
  name: NAMES
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    nmh: {dim: voltage, label: NM_H, name: {en: "High margin", de: "Hoher Rand"}, input: false}
  choices:
    drv:
      name: NAMES
      default: a
      options:
        a: {name: NAMES, consts: {voh: 4.9}}
        b: {name: NAMES, consts: {voh: 2.4}}
    rcv:
      name: NAMES
      default: a
      options:
        a: {name: NAMES, consts: {vih: 2.0}}
        b: {name: NAMES, consts: {vih: 3.5}}
  dof: 0
  rules:
    nmh: ["voh - vih"]
  checks:
    - {when: "nmh < 0", warn: {en: "No high margin.", de: "Kein hoher Rand."}}
  formula: ["nmh = voh - vih"]
- id: t_div
  kind: calculator
  category: basics
  name: NAMES
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    a: {dim: voltage, label: A, name: {en: "A", de: "A"}, default: "10 V"}
    b: {dim: resistance, label: B, name: {en: "B", de: "B"}, default: "100"}
    c: {dim: current, label: C, name: {en: "C", de: "C"}}
  dof: 2
  rules:
    c: ["a/b"]
    a: ["c*b"]
    b: ["a/c"]
  formula: ["c = a / b"]
`

func ecalcLevelCatalog(t *testing.T) *ecalc.Catalog {
	t.Helper()
	parts := make([]string, len(ecalc.Locales))
	for i, l := range ecalc.Locales {
		parts[i] = l + `: "T"`
	}
	fsys := fstest.MapFS{}
	for _, name := range []string{"units.yaml", "categories.yaml"} {
		data, err := os.ReadFile("../ecalc/catalog/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	fsys["calculators/t.yaml"] = &fstest.MapFile{Data: []byte(strings.ReplaceAll(ecalcLevelFixture, "NAMES", "{"+strings.Join(parts, ", ")+"}"))}
	cat, err := ecalc.Load(fsys)
	if err != nil {
		t.Fatalf("fixture catalog: %v", err)
	}
	return cat
}

func TestECalcCalculatesAChoicesOnlyCalculatorWithoutInputs(t *testing.T) {
	cat := ecalcLevelCatalog(t)
	exec := executeECalc(nil, cat, ECalcRequest{Operation: "calculate", ID: "t_levels", Choices: ecalcPairs("drv", "b", "rcv", "b")})
	out := ecalcDecode(t, exec)
	if out["status"] != "ok" || !strings.Contains(exec.Output, `"No high margin."`) {
		t.Fatalf("output = %s", exec.Output)
	}
	exec = executeECalc(nil, cat, ECalcRequest{Operation: "calculate", ID: "t_levels"})
	if out := ecalcDecode(t, exec); out["status"] != "ok" || strings.Contains(exec.Output, "No high margin") {
		t.Fatalf("defaults: output = %s", exec.Output)
	}
	// Choices are still validated.
	exec = executeECalc(nil, cat, ECalcRequest{Operation: "calculate", ID: "t_levels", Choices: ecalcPairs("drv", "zzz")})
	if out := ecalcDecode(t, exec); out["status"] != "error" || out["code"] != "unknown_option" {
		t.Fatalf("bad option: output = %s", exec.Output)
	}
	// A normal calculator without inputs still is under-determined.
	exec = executeECalc(nil, cat, ECalcRequest{Operation: "calculate", ID: "t_div"})
	if out := ecalcDecode(t, exec); out["status"] != "error" || out["code"] != "underdetermined" {
		t.Fatalf("t_div without inputs: output = %s", exec.Output)
	}
}

func TestECalcOpensAChoicesOnlyCalculatorWithoutInputs(t *testing.T) {
	cat := ecalcLevelCatalog(t)
	exec := executeECalc(ecalcDesktopConfig(), cat, ECalcRequest{Operation: "open", ID: "t_levels", Choices: ecalcPairs("drv", "b")})
	if exec.Event == nil || ecalcDecode(t, exec)["status"] != "ok" {
		t.Fatalf("output = %s", exec.Output)
	}
	exec = executeECalc(ecalcDesktopConfig(), cat, ECalcRequest{Operation: "open", ID: "t_levels", Choices: ecalcPairs("drv", "zzz")})
	if exec.Event != nil || ecalcDecode(t, exec)["status"] != "error" {
		t.Fatalf("bad option: output = %s", exec.Output)
	}
}
