package ui

import (
	"strings"
	"testing"
)

// The material table is the only ECalc drawing file that may hold colour literals: it is a DOM-free classic script, it is
// published as window.ECalcMaterials and the loader runs it before the drawing builders.
func TestECalcMaterialsHoldTheOnlyDrawingColours(t *testing.T) {
	t.Parallel()
	source, ok := ecalcScripts(t)["ecalc-materials.js"]
	if !ok {
		t.Fatalf("%s/ecalc-materials.js is missing", ecalcDir)
	}
	if !strings.Contains(source, "window.ECalcMaterials = {") {
		t.Error("ecalc-materials.js must publish window.ECalcMaterials = {")
	}
	for _, banned := range []string{"eval(", "new Function", "document.", "innerHTML"} {
		if strings.Contains(source, banned) {
			t.Errorf("ecalc-materials.js must not contain %q", banned)
		}
	}
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        ecalc: {")
	if start < 0 {
		t.Fatal("module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	materials := strings.Index(entry, "ecalc-materials.js")
	draw := strings.Index(entry, "ecalc-code-draw.js")
	if materials < 0 || draw < 0 || materials > draw {
		t.Errorf("the ecalc loader entry must load ecalc-materials.js before ecalc-code-draw.js (%d, %d)", materials, draw)
	}
}
