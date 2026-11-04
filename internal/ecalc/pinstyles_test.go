package ecalc

import (
	"math"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestContrastRatio(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#ffffff", 1},
		{"#777777", "#ffffff", 4.48}, // the well-known grey that just fails 4.5
		{"#767676", "#ffffff", 4.54}, // and the one that just passes
	} {
		if got := contrastRatio(c.a, c.b); math.Abs(got-c.want) > 0.01 {
			t.Errorf("contrastRatio(%s, %s) = %.3f, want %.2f", c.a, c.b, got, c.want)
		}
		if got := contrastRatio(c.b, c.a); math.Abs(got-contrastRatio(c.a, c.b)) > 1e-12 {
			t.Errorf("contrastRatio is not symmetric for %s, %s", c.a, c.b)
		}
	}
}

// The embedded pinstyles.yaml: six pin types in order, fifteen wires, every ink readable on its fill, 16 locales.
func TestPinStylesOfTheCatalog(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, pt := range cat.PinTypeStyles {
		ids = append(ids, pt.ID)
		if c := contrastRatio(pt.Ink, pt.Hex); c < 4.5 {
			t.Errorf("pin type %s: ink %s on %s has a contrast of %.2f, want at least 4.5", pt.ID, pt.Ink, pt.Hex, c)
		}
		for _, lang := range Locales {
			if pt.Name[lang] == "" {
				t.Errorf("pin type %s has no %s name", pt.ID, lang)
			}
		}
		if cat.PinType(pt.ID) != pt {
			t.Errorf("PinType(%s) does not return the style", pt.ID)
		}
	}
	if strings.Join(ids, " ") != "power ground signal data io nc" || strings.Join(ids, " ") != strings.Join(PinTypes, " ") {
		t.Errorf("pin types = %v, PinTypes = %v", ids, PinTypes)
	}
	var wires []string
	for _, w := range cat.Wires {
		wires = append(wires, w.ID)
		for _, lang := range Locales {
			if w.Name[lang] == "" {
				t.Errorf("wire %s has no %s name", w.ID, lang)
			}
		}
		if (w.Stripe != "") != strings.HasPrefix(w.ID, "white_") || cat.Wire(w.ID) != w {
			t.Errorf("wire %s: stripe %q", w.ID, w.Stripe)
		}
	}
	if len(wires) != 15 || strings.Join(wires, " ") != strings.Join(wireIDs, " ") {
		t.Errorf("wires = %v", wires)
	}
}

func TestLoadRejectsBrokenPinStyles(t *testing.T) {
	data, err := os.ReadFile("catalog/pinstyles.yaml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)
	cases := []struct{ name, old, new, want string }{
		{"not a colour", `hex: "#c62828"`, `hex: "red"`, `type power: hex must be a colour like #1565c0`},
		{"upper case colour", `hex: "#c62828"`, `hex: "#C62828"`, "hex must be a colour"},
		{"ink missing", `ink: "#000000"`, `ink: ""`, `type signal: ink must be a colour`},
		{"ink too weak", `hex: "#f9a825"`, `hex: "#7b5200"`, "type signal: ink #000000 on #7b5200 has a contrast of"},
		{"missing locale", `, zh: "电源"`, "", `type power: name: missing "zh" text`},
		{"missing wire locale", `, zh: "黑色"`, "", `wire black: name: missing "zh" text`},
		{"type renamed", "  nc:", "  nx:", "types must be power, ground, signal, data, io, nc in this order"},
		{"wire renamed", "  pink:", "  rosa:", "wires must be black, brown,"},
		{"stripe lost", `stripe: "#ef6c00"`, `stripe: ""`, "wire white_orange: a striped wire needs a stripe colour"},
		{"stripe not a colour", `stripe: "#ef6c00"`, `stripe: "orange"`, "wire white_orange: stripe must be a colour"},
		{"unknown field", `hex: "#212121"`, `hex: "#212121"` + "\n    colour: red", "colour"},
	}
	for _, c := range cases {
		fsys := treeFS(t, nil)
		fsys["pinstyles.yaml"] = &fstest.MapFile{Data: []byte(mutate(t, base, c.old, c.new))}
		_, err := Load(fsys)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}
