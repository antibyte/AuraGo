package homedash

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/widget_catalog.json from the Go catalog")

func TestWidgetCatalogGolden(t *testing.T) {
	got, err := Catalog().MarshalGolden()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "widget_catalog.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))) != string(got) {
		t.Fatal("widget catalog differs from testdata/widget_catalog.json (rerun with -update after an intended change)")
	}
	if h := catalogHash(Catalog()); h != Catalog().Hash || len(h) != 64 {
		t.Fatalf("hash %q", h)
	}
}

func TestWidgetCatalogInvariants(t *testing.T) {
	c := Catalog()
	variants, types := 0, map[string]bool{}
	page := map[string]bool{"main": true}
	for _, wt := range c.Widgets {
		if types[wt.Type] {
			t.Errorf("type %s twice", wt.Type)
		}
		types[wt.Type] = true
		ids := map[string]bool{}
		for _, v := range wt.Variants {
			variants++
			if ids[v.ID] {
				t.Errorf("%s/%s twice", wt.Type, v.ID)
			}
			ids[v.ID] = true
			if v.MinSize.W < 1 || v.MinSize.H < 1 || v.DefaultSize.W < v.MinSize.W || v.DefaultSize.H < v.MinSize.H ||
				v.DefaultSize.W > wt.MaxSize.W || v.DefaultSize.H > wt.MaxSize.H || wt.MaxSize.W > c.Grid.Columns.LG {
				t.Errorf("%s/%s sizes %+v %+v max %+v", wt.Type, v.ID, v.MinSize, v.DefaultSize, wt.MaxSize)
			}
			for fam, to := range v.FamilyAlias {
				if !slices.Contains(c.Families, fam) {
					t.Errorf("%s/%s alias family %s", wt.Type, v.ID, fam)
				}
				if _, ok := wt.Variant(to); !ok && to != "scale" {
					t.Errorf("%s/%s alias target %s", wt.Type, v.ID, to)
				}
			}
		}
		for _, b := range wt.Bindings {
			for _, k := range b.Kinds {
				if !slices.Contains(c.Kinds, k) {
					t.Errorf("%s.%s kind %s", wt.Type, b.Name, k)
				}
			}
		}
		for name, caps := range wt.Commands {
			if _, ok := wt.Binding(name); !ok {
				t.Errorf("%s commands for unknown binding %s", wt.Type, name)
			}
			for _, cp := range caps {
				if info, ok := LookupCapability(cp); !ok || !info.Command {
					t.Errorf("%s.%s command %s", wt.Type, name, cp)
				}
			}
		}
		v := &validator{cat: c}
		for _, o := range wt.Options {
			if o.Default != nil {
				v.option("/"+wt.Type+"/"+o.Key, o, o.Default, page)
			}
		}
		if len(v.r.Issues) > 0 {
			t.Errorf("%s option defaults invalid: %+v", wt.Type, v.r.Issues)
		}
	}
	if len(c.Widgets) != 24 || variants != 47 {
		t.Fatalf("%d types, %d variants; want 24 and 47", len(c.Widgets), variants)
	}
	if len(c.PaletteTokens) > 40 || c.Grid.Columns != (GridColumns{12, 8, 4}) {
		t.Fatal("palette tokens or grid columns")
	}
}
