package ui

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestContainersProtectedBadgeFitsNarrowCardsBrowserSmoke renders a protected
// card at phone widths with the longest badge translations and checks that
// the badge never squeezes the name away or overflows the header (K12 review).
// It loads the stylesheets of containers.html in the page's own order.
func TestContainersProtectedBadgeFitsNarrowCardsBrowserSmoke(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}
	browser := newSmokeBrowser(t)
	css := ""
	for _, name := range []string{
		"shared-variables.css", "shared-utilities.css", "shared-components.css", "css/tokens.css",
		"css/containers.css", "css/enhancements.css", "css/precision-workspace.css", "css/precision-pages.css",
	} {
		css += normalizeAssetText(mustReadUIFile(t, name)) + "\n"
	}
	page := browser.MustPage("about:blank")
	defer page.MustClose()
	for _, lang := range []string{"el", "hi", "de", "en"} {
		data, err := os.ReadFile(filepath.Join("lang", "containers", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{288, 320, 400} {
			page.MustSetDocumentContent(`<!doctype html><html><head><style>` + css + `</style></head>
				<body class="pw-page pw-operational-page" data-workspace-page="containers" data-density="comfortable">
				<div class="ct-card" style="width:` + strconv.Itoa(width) + `px">
					<div class="ct-card-header">
						<div class="ct-card-status running"></div>
						<div class="ct-card-name" id="name">aurago-store-commandcode</div>
						<span class="ct-card-protected" id="badge">` + html.EscapeString(values["containers.protected_badge"]) + `</span>
						<span class="ct-card-id" id="id">0123456789ab</span>
					</div>
				</div></body></html>`)
			state := page.MustEval(`() => {
				const r = id => document.getElementById(id).getBoundingClientRect();
				const overlap = (a, b) => a.left < b.right - 0.5 && b.left < a.right - 0.5 && a.top < b.bottom - 0.5 && b.top < a.bottom - 0.5;
				const header = document.querySelector('.ct-card-header');
				return {
					name: r('name').width,
					badge: r('badge').width,
					nameBadge: overlap(r('name'), r('badge')),
					badgeID: overlap(r('badge'), r('id')),
					overflow: header.scrollWidth - header.clientWidth,
				};
			}`).Map()
			if state["name"].Num() < 100 {
				t.Fatalf("%s at %d px: name width %.1f, want at least 100 (%v)", lang, width, state["name"].Num(), state)
			}
			if state["nameBadge"].Bool() || state["badgeID"].Bool() || state["overflow"].Num() > 1 {
				t.Fatalf("%s at %d px: header layout %v", lang, width, state)
			}
			t.Logf("%s at %d px: name %.1f px, badge %.1f px", lang, width, state["name"].Num(), state["badge"].Num())
		}
	}
}
