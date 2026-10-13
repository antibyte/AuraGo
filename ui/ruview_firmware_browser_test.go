package ui

import (
	"encoding/json"
	"testing"
	"time"

	"aurago/internal/ruview"
)

func TestRuViewSerializedFirmwareCatalogBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	catalog := ruview.Artifacts()
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#ruview").Timeout(30 * time.Second)
	defer page.MustClose()
	profiles := page.MustEval(`async encoded => {
		const {validateProfile} = await import('/js/ruview/protocol.mjs');
		return JSON.parse(encoded).firmware.map(profile => validateProfile(profile).id);
	}`, string(encoded)).Arr()
	if len(profiles) == 0 || len(profiles) != len(catalog.Firmware) {
		t.Fatalf("validated %d profiles, want %d", len(profiles), len(catalog.Firmware))
	}
}
