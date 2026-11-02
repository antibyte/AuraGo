package ui

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"aurago/internal/ecalc"
)

const (
	ecalcSymbolsPath = "js/desktop/apps/ecalc/ecalc-symbols.js"
	// ecalcSchematicsWrapper starts every schematics file; the JSON object of the file follows and `);` closes it.
	ecalcSchematicsWrapper = "window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, "
)

// ecalcSchematicCategories are the categories that own a schematics file, in the load order of the module loader
// (basics first, the rest alphabetical). The catalog has more categories (tables, codes, pinouts) without drawings.
var ecalcSchematicCategories = []string{"basics", "digital", "filters", "opamps", "passive", "power", "rf", "semiconductors", "timers", "wiring"}

func ecalcSchematicsPath(category string) string {
	return "js/desktop/apps/ecalc/ecalc-schematics-" + category + ".js"
}

// ecalcSchematic is the schema of one entry of window.ECalcSchematics; unknown fields fail the decode, so a typo in a
// key cannot silently drop part of a drawing.
type ecalcSchematic struct {
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Parts []struct {
		Sym  string  `json:"sym"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Rot  int     `json:"rot"`
		Flip bool    `json:"flip"`
		Text string  `json:"text"`
		W    float64 `json:"w"`
		H    float64 `json:"h"`
		Pins []struct {
			N string  `json:"n"`
			S string  `json:"s"`
			Y float64 `json:"y"`
		} `json:"pins"`
	} `json:"parts"`
	Wires     [][][2]float64 `json:"wires"`
	Dots      [][2]float64   `json:"dots"`
	Terminals [][2]float64   `json:"terminals"`
	Arrows    []struct {
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
		Dir int     `json:"dir"`
	} `json:"arrows"`
	Texts []struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Text   string  `json:"text"`
		Anchor string  `json:"anchor"`
	} `json:"texts"`
	Anchors []struct {
		Var  string  `json:"var"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Side string  `json:"side"`
	} `json:"anchors"`
}

// ecalcCatalogItem is the part of the catalog JSON of the server route (contract section 7) the cross-check reads.
type ecalcCatalogItem struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Category  string `json:"category"`
	Schematic string `json:"schematic"`
	Vars      []struct {
		ID string `json:"id"`
	} `json:"vars"`
}

// loadECalcSchematics reads the ten schematics files. Each is exactly
// `window.ECalcSchematics = Object.assign(window.ECalcSchematics || {}, {JSON});`; the schematics of all files are merged
// by id and a duplicate id fails. files maps every schematic id to the category of the file that holds it.
func loadECalcSchematics(t *testing.T) (specs map[string]ecalcSchematic, files map[string]string) {
	t.Helper()
	specs, files = map[string]ecalcSchematic{}, map[string]string{}
	for _, category := range ecalcSchematicCategories {
		path := ecalcSchematicsPath(category)
		text := strings.TrimSpace(readDesktopAssetText(t, path))
		if !strings.HasPrefix(text, ecalcSchematicsWrapper+"{") || !strings.HasSuffix(text, "});") {
			t.Fatalf("%s must be exactly `%s{JSON});`", path, ecalcSchematicsWrapper)
		}
		body := strings.TrimSuffix(strings.TrimPrefix(text, ecalcSchematicsWrapper), ");")
		decoder := json.NewDecoder(bytes.NewReader([]byte(body)))
		decoder.DisallowUnknownFields()
		var own map[string]ecalcSchematic
		if err := decoder.Decode(&own); err != nil {
			t.Fatalf("%s is not strict JSON of the schematic schema: %v", path, err)
		}
		for id, spec := range own {
			if first, dup := files[id]; dup {
				t.Errorf("schematic %q is defined in both ecalc-schematics-%s.js and ecalc-schematics-%s.js", id, first, category)
				continue
			}
			specs[id], files[id] = spec, category
		}
	}
	return specs, files
}

func loadECalcCatalogItems(t *testing.T) []ecalcCatalogItem {
	t.Helper()
	catalog, err := ecalc.Default()
	if err != nil {
		t.Fatalf("ecalc.Default: %v", err)
	}
	body, _, err := catalog.LocalizedJSON("en")
	if err != nil {
		t.Fatalf("LocalizedJSON: %v", err)
	}
	var parsed struct {
		Items []ecalcCatalogItem `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("catalog JSON: %v", err)
	}
	return parsed.Items
}

// Every schematic belongs to a calculator, every calculator that names a schematic has one, and every field anchor
// names a variable of its calculator; the drawing itself stays inside its grid and uses symbols the library has.
func TestECalcSchematicsMatchTheCatalog(t *testing.T) {
	t.Parallel()
	specs, files := loadECalcSchematics(t)
	symbols := readDesktopAssetText(t, ecalcSymbolsPath)
	users := map[string][]ecalcCatalogItem{}
	for _, item := range loadECalcCatalogItems(t) {
		if item.Kind == "calculator" && item.Schematic != "" {
			users[item.Schematic] = append(users[item.Schematic], item)
		}
	}
	for id := range users {
		if _, ok := specs[id]; !ok {
			t.Errorf("the catalog names the schematic %q but the ecalc-schematics-*.js files have no entry for it", id)
		}
	}
	for id, spec := range specs {
		owners := users[id]
		if len(owners) == 0 {
			t.Errorf("schematic %q belongs to no calculator of the catalog", id)
			continue
		}
		for _, owner := range owners {
			if owner.Category != files[id] {
				t.Errorf("schematic %q sits in ecalc-schematics-%s.js but its calculator %q is in the category %q", id, files[id], owner.ID, owner.Category)
			}
		}
		inside := func(what string, x, y float64) {
			if x < 0 || y < 0 || x > spec.W || y > spec.H {
				t.Errorf("schematic %q: %s at (%v, %v) lies outside the %vx%v grid", id, what, x, y, spec.W, spec.H)
			}
		}
		for _, part := range spec.Parts {
			if !strings.Contains(symbols, "        "+part.Sym+": {") {
				t.Errorf("schematic %q uses the unknown symbol %q", id, part.Sym)
			}
			inside("part "+part.Sym, part.X, part.Y)
		}
		for _, wire := range spec.Wires {
			if len(wire) < 2 {
				t.Errorf("schematic %q has a wire with fewer than two points", id)
			}
			for _, point := range wire {
				inside("wire point", point[0], point[1])
			}
		}
		seen := map[string]bool{}
		for _, anchor := range spec.Anchors {
			if seen[anchor.Var] {
				t.Errorf("schematic %q anchors %q twice", id, anchor.Var)
			}
			seen[anchor.Var] = true
			if anchor.Side != "left" && anchor.Side != "right" && anchor.Side != "top" && anchor.Side != "bottom" {
				t.Errorf("schematic %q anchor %q has the side %q", id, anchor.Var, anchor.Side)
			}
			inside("anchor "+anchor.Var, anchor.X, anchor.Y)
			for _, owner := range owners {
				found := false
				for _, v := range owner.Vars {
					found = found || v.ID == anchor.Var
				}
				if !found {
					t.Errorf("schematic %q anchors %q, which is no variable of the calculator %q", id, anchor.Var, owner.ID)
				}
			}
		}
	}
}

// The split is complete: exactly the ten category files exist (the old single file is gone), every one starts with the
// wrapper and ends with `});`, and the symbol library holds the two symbols that phase 2 adds, with the pin names the
// schematic authors rely on (switch: a, b; pot: a, b and the wiper w).
func TestECalcSchematicFilesAndNewSymbols(t *testing.T) {
	t.Parallel()
	entries, err := fs.ReadDir(Content, ecalcDir)
	if err != nil {
		t.Fatalf("read %s: %v", ecalcDir, err)
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ecalc-schematics") {
			found[entry.Name()] = true
		}
	}
	for _, category := range ecalcSchematicCategories {
		name := "ecalc-schematics-" + category + ".js"
		if !found[name] {
			t.Errorf("%s/%s is missing", ecalcDir, name)
		}
		delete(found, name)
		text := strings.TrimSpace(readDesktopAssetText(t, ecalcSchematicsPath(category)))
		if !strings.HasPrefix(text, ecalcSchematicsWrapper+"{") || !strings.HasSuffix(text, "});") || strings.Count(text, "window.ECalcSchematics") != 2 {
			t.Errorf("%s must be exactly `%s{JSON});`", name, ecalcSchematicsWrapper)
		}
	}
	for name := range found {
		t.Errorf("%s/%s is not one of the ten category files (the single ecalc-schematics.js is gone)", ecalcDir, name)
	}
	symbols := readDesktopAssetText(t, ecalcSymbolsPath)
	for _, marker := range []string{"        switch: {", "        pot: {", "pins: () => ({ a: [0, 0], b: [4, 0], w: [2, -2] }),"} {
		if !strings.Contains(symbols, marker) {
			t.Errorf("%s is missing %q", ecalcSymbolsPath, marker)
		}
	}
}
