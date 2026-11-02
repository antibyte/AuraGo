package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/ecalc"
)

const (
	ecalcSchematicsPath = "js/desktop/apps/ecalc/ecalc-schematics.js"
	ecalcSymbolsPath    = "js/desktop/apps/ecalc/ecalc-symbols.js"
)

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
	Schematic string `json:"schematic"`
	Vars      []struct {
		ID string `json:"id"`
	} `json:"vars"`
}

func loadECalcSchematics(t *testing.T) map[string]ecalcSchematic {
	t.Helper()
	text := strings.TrimSpace(readDesktopAssetText(t, ecalcSchematicsPath))
	const prefix = "window.ECalcSchematics = {"
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, "};") {
		t.Fatalf("%s must be exactly `window.ECalcSchematics = {JSON};`", ecalcSchematicsPath)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(text, "window.ECalcSchematics = "), ";")
	decoder := json.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.DisallowUnknownFields()
	var specs map[string]ecalcSchematic
	if err := decoder.Decode(&specs); err != nil {
		t.Fatalf("%s is not strict JSON of the schematic schema: %v", ecalcSchematicsPath, err)
	}
	return specs
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
	specs := loadECalcSchematics(t)
	symbols := readDesktopAssetText(t, ecalcSymbolsPath)
	users := map[string][]ecalcCatalogItem{}
	for _, item := range loadECalcCatalogItems(t) {
		if item.Kind == "calculator" && item.Schematic != "" {
			users[item.Schematic] = append(users[item.Schematic], item)
		}
	}
	for id := range users {
		if _, ok := specs[id]; !ok {
			t.Errorf("the catalog names the schematic %q but %s has no entry for it", id, ecalcSchematicsPath)
		}
	}
	for id, spec := range specs {
		owners := users[id]
		if len(owners) == 0 {
			t.Errorf("schematic %q belongs to no calculator of the catalog", id)
			continue
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
