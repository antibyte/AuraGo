package ui

import (
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopECalcBackendRegistration(t *testing.T) {
	t.Parallel()

	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "ecalc" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("ecalc is not a builtin app")
	}
	if app.Name != "ECalc" || app.Category != "dev" || app.Entry != "builtin://ecalc" || len(app.Requires) != 0 || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	logo := app.Metadata["logo_path"]
	if logo != "/img/desktop-icons/ecalc.svg" {
		t.Fatalf("logo_path = %q", logo)
	}
	svg, err := Content.ReadFile(strings.TrimPrefix(logo, "/"))
	if err != nil {
		t.Fatalf("the ECalc logo is not embedded: %v", err)
	}
	text := string(svg)
	if !strings.HasPrefix(text, "<svg ") || strings.Contains(text, "href") || strings.Contains(text, "<script") {
		t.Fatalf("the ECalc logo must be a self-contained SVG: %.120s", text)
	}
}
