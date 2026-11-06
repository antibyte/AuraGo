package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopCalculatorUXBrowser walks the polished calculator behaviour: the live
// preview, CE against the current entry, the sign toggle, the clickable history, the
// copy affordance, the base-aware programmer keys and the error state.
func TestDesktopCalculatorUXBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/calculator-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/calculator-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await AuraDesktopModules.loadAppAssets('calculator');}`)
	page.MustEval(`async()=>{fixtureTheme('fruity-dark');await fixtureOpen('calculator');}`)

	click := func(selector string) {
		t.Helper()
		button := page.MustElement(selector)
		point := button.MustEval(`function(){const r=this.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`).Arr()
		if !button.MustEval(`function(){const r=this.getBoundingClientRect();return this.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}`).Bool() {
			t.Fatalf("calculator control is covered: %s", selector)
		}
		page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
	}
	pressKey := func(key string) { click(fmt.Sprintf(`[data-key="%s"]`, key)) }
	text := func(selector string) string { return page.MustElement(selector).MustText() }

	// The result previews the expression while it is typed, before '=' is pressed.
	pressKey("7")
	pressKey("+")
	pressKey("8")
	if got := text("[data-result]"); got != "15" {
		t.Fatalf("calculator must preview the result while typing, got %q", got)
	}

	// CE clears the entry that is being typed, C clears everything.
	pressKey("C")
	pressKey("1")
	pressKey("2")
	pressKey(".")
	pressKey("CE")
	if got := text("[data-expression]"); got != "0" {
		t.Fatalf("CE must clear the current entry, got %q", got)
	}
	pressKey("3")
	pressKey("4")
	pressKey("C")
	if got := text("[data-expression]"); got != "0" {
		t.Fatalf("C must clear the whole expression, got %q", got)
	}

	// The sign toggle negates the number that is being typed.
	pressKey("5")
	pressKey("negate")
	if got := text("[data-expression]"); got != "-5" {
		t.Fatalf("negate must toggle the sign of the current entry, got %q", got)
	}
	pressKey("negate")
	if got := text("[data-expression]"); got != "5" {
		t.Fatalf("negate must toggle the sign back, got %q", got)
	}
	pressKey("C")
	pressKey("1")
	pressKey("+")
	pressKey("2")
	pressKey("negate")
	if got := text("[data-result]"); got != "-1" {
		t.Fatalf("negate must apply to the current term, got %q", got)
	}

	// A finished calculation lands in the history and can be reused from there.
	pressKey("C")
	pressKey("6")
	pressKey("*")
	pressKey("7")
	pressKey("=")
	if got := text("[data-result]"); got != "42" {
		t.Fatalf("calculator must evaluate 6*7, got %q", got)
	}
	if entries := page.MustElements(".vd-calc-history-entry"); len(entries) != 1 {
		t.Fatalf("history must hold the finished calculation, got %d entries", len(entries))
	}
	click(".vd-calc-history-entry")
	if got := text("[data-expression]"); got != "42" {
		t.Fatalf("a history entry must restore its result, got %q", got)
	}

	// Switching the mode keeps the value instead of dropping it.
	click(`[data-mode="scientific"]`)
	if got := text("[data-expression]"); got != "42" {
		t.Fatalf("switching the mode must keep the value, got %q", got)
	}

	// The copy affordance is present, labelled and reacts to a click.
	if label := page.MustElement("[data-copy]").MustAttribute("aria-label"); label == nil || strings.TrimSpace(*label) == "" {
		t.Fatal("the copy button needs an accessible name")
	}
	click("[data-copy]")
	state := ""
	for i := 0; i < 40; i++ {
		state = page.MustEval(`()=>{const b=document.querySelector('[data-copy]');return b.classList.contains('copied')||b.classList.contains('failed')?'ok':'none';}`).Str()
		if state == "ok" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if state != "ok" {
		t.Fatalf("the copy button must report its outcome, got %q", state)
	}

	// The programmer mode disables the digits that the current base cannot hold.
	click(`[data-mode="programmer"]`)
	if !page.MustEval(`()=>document.querySelector('[data-key="HEX_C"]').disabled`).Bool() {
		t.Fatal("hex digits must be disabled in decimal mode")
	}
	if page.MustEval(`()=>document.querySelector('[data-key="8"]').disabled`).Bool() {
		t.Fatal("decimal digits must stay enabled in decimal mode")
	}
	click(`[data-base="16"]`)
	if got := text("[data-expression]"); got != "2A" {
		t.Fatalf("switching to hexadecimal must carry the value, got %q", got)
	}
	if page.MustEval(`()=>document.querySelector('[data-key="HEX_C"]').disabled`).Bool() {
		t.Fatal("hex digits must be enabled in hexadecimal mode")
	}
	if page.MustEval(`()=>document.querySelector('[data-key="9"]').disabled`).Bool() {
		t.Fatal("decimal digits must stay enabled in hexadecimal mode")
	}
	click(`[data-base="8"]`)
	if got := text("[data-expression]"); got != "52" {
		t.Fatalf("switching to octal must preserve the value, got %q", got)
	}
	if !page.MustEval(`()=>document.querySelector('[data-key="8"]').disabled`).Bool() {
		t.Fatal("digits above the octal base must be disabled")
	}
	click(`[data-base="2"]`)
	if got := text("[data-expression]"); got != "101010" {
		t.Fatalf("switching to binary must preserve the value, got %q", got)
	}
	if !page.MustEval(`()=>document.querySelector('[data-key="2"]').disabled`).Bool() {
		t.Fatal("digits above the binary base must be disabled")
	}
	pressKey("C")
	pressKey("1")
	pressKey("0")
	pressKey("CE")
	if got := text("[data-expression]"); got != "0" {
		t.Fatalf("binary CE must clear the current entry, got %q", got)
	}
	click(`[data-base="16"]`)
	pressKey("A")
	pressKey("B")
	pressKey("CE")
	if got := text("[data-expression]"); got != "0" {
		t.Fatalf("hexadecimal CE must clear the whole current entry, got %q", got)
	}
	pressKey("C")
	pressKey("A")
	pressKey("AND")
	pressKey("backspace") // Remove the operator's trailing space to exercise its token boundary.
	pressKey("CE")
	if got := strings.TrimSpace(text("[data-expression]")); got != "A" {
		t.Fatalf("programmer CE must clear the trailing operator, got %q", got)
	}

	// An invalid expression reports the translated error and keeps the expression.
	click(`[data-mode="standard"]`)
	pressKey("C")
	pressKey("*")
	pressKey("=")
	if !page.MustEval(`()=>document.querySelector('[data-result]').classList.contains('error')`).Bool() {
		t.Fatal("an invalid expression must switch the display into the error state")
	}
	if got := text("[data-result]"); got == "0" || strings.Contains(got, "Invalid expression") {
		t.Fatalf("the error state must show the translated message, got %q", got)
	}
	if got := text("[data-expression]"); got != "*" {
		t.Fatalf("an invalid expression must stay visible, got %q", got)
	}

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
