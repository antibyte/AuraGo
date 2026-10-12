package ui

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"aurago/internal/cydcreator"
)

// cydDesignerScripts is the designer's load order (same as DESKTOP_APP_ASSETS['cyd-creator']
// between cyd-creator-events.js and cyd-creator-code.js).
var cydDesignerScripts = []string{
	"cyd-creator-geometry.js", "cyd-creator-model.js", "cyd-creator-render.js", "cyd-creator-canvas.js",
	"cyd-creator-interact.js", "cyd-creator-palette.js", "cyd-creator-inspector-fields.js",
	"cyd-creator-inspector.js", "cyd-creator-screens.js", "cyd-creator-designer.js",
}

var updateCYDDesignerCaps = flag.Bool("update-cyd-designer-caps", false, "rewrite ui/testdata/cyd_creator_designer_caps.json from internal/cydcreator")

// cydDesignerCapsFromCatalog builds the designer's capability subset from the Go domain package,
// as GET /capabilities serves it (boards reduced to the fields the designer reads).
func cydDesignerCapsFromCatalog(t *testing.T) map[string]any {
	t.Helper()
	board, err := cydcreator.BoardByID("esp32-2432s028r")
	if err != nil {
		t.Fatal(err)
	}
	themes, err := cydcreator.Themes()
	if err != nil {
		t.Fatal(err)
	}
	caps := map[string]any{
		"boards": []any{map[string]any{
			"id": board.ID, "label": board.Label, "status": board.Status, "chip": board.Chip, "flash_mb": board.FlashMB, "psram": board.PSRAM,
			"display": map[string]any{"driver": board.Display.Driver, "width": board.Display.Width, "height": board.Display.Height, "native_orientation": board.Display.NativeOrientation},
		}},
		"fonts":     map[string]any{"families": cydcreator.FontFamilies, "sizes": cydcreator.FontSizes},
		"languages": cydcreator.Languages,
		"libraries": cydcreator.Libraries(),
		"symbols":   cydcreator.Symbols,
		"themes":    themes,
		"widgets":   cydcreator.WidgetCatalog(),
	}
	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// cydCompactJSON writes one top-level key per line and, for arrays of objects, one element per
// line, so the fixture stays readable and diffable.
func cydCompactJSON(t *testing.T, caps map[string]any) []byte {
	t.Helper()
	keys := make([]string, 0, len(caps))
	for k := range caps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, k := range keys {
		b.WriteString("  \"" + k + "\": ")
		list, ok := caps[k].([]any)
		if ok && len(list) > 0 {
			_, ok = list[0].(map[string]any)
		}
		if ok {
			b.WriteString("[\n")
			for j, item := range list {
				data, err := json.Marshal(item)
				if err != nil {
					t.Fatal(err)
				}
				b.WriteString("    " + string(data))
				if j < len(list)-1 {
					b.WriteString(",")
				}
				b.WriteString("\n")
			}
			b.WriteString("  ]")
		} else {
			data, err := json.Marshal(caps[k])
			if err != nil {
				t.Fatal(err)
			}
			b.Write(data)
		}
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}

// TestCYDCreatorDesignerCapsFixtureMatchesCatalog keeps the Node/browser test fixture equal to the
// Go catalog, themes, libraries and the verified board. Regenerate after a domain change with
// go test ./ui -run TestCYDCreatorDesignerCapsFixtureMatchesCatalog -args -update-cyd-designer-caps
func TestCYDCreatorDesignerCapsFixtureMatchesCatalog(t *testing.T) {
	want := cydDesignerCapsFromCatalog(t)
	path := filepath.Join("testdata", "cyd_creator_designer_caps.json")
	if *updateCYDDesignerCaps {
		if err := os.WriteFile(path, cydCompactJSON(t, want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from internal/cydcreator; regenerate with: go test ./ui -run TestCYDCreatorDesignerCapsFixtureMatchesCatalog -args -update-cyd-designer-caps", path)
	}
}
