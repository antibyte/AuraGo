package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// Keep the inventory total distinct from search results and make reset actions usable by keyboard.
func TestDesktopInfraWorkspacePolishBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1500, 970)
	defer page.Close()
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='live'`)
	page.MustEval(`()=>document.fonts.ready`)
	if !page.MustEval(`()=>document.querySelector('[data-inventory-count]').textContent===fixtureWords['desktop.infra_asset_count'].replace('{count}',25)`).Bool() {
		t.Fatal("the status bar must count the complete inventory")
	}
	page.MustElement(`[data-node="fixture-node-0003"]`).MustClick()
	page.MustWait(`()=>!document.querySelector('[data-selection-count]').hidden`)
	if !page.MustEval(`()=>document.querySelector('[data-selection-count]').textContent===fixtureWords['desktop.infra_selected_count'].replace('{count}',1)`).Bool() {
		t.Fatal("the status bar must follow selection changes")
	}
	page.MustElement(`[data-action="close-inspector"]`).MustClick()
	page.MustElement(`[data-search]`).MustInput("Cold-store")
	page.MustWait(`()=>!document.querySelector('.infra-search-clear').hidden`)
	page.MustEval(`()=>{const el=document.querySelector('[data-kind-filter]');el.value='device';el.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelector('.infra-status-reset').getAttribute('aria-label')===fixtureWords['desktop.infra_clear_search_filters']`)
	page.MustElement(`.infra-search-clear`).MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('[data-search]')&&document.querySelector('[data-search]').value===''&&document.querySelector('.infra-search-clear').hidden`)
	if !page.MustEval(`()=>document.querySelector('[data-kind-filter]').value==='device'&&!document.querySelector('.infra-status-reset').hidden&&document.querySelector('[data-inventory-count]').textContent===fixtureWords['desktop.infra_asset_count'].replace('{count}',25)`).Bool() {
		t.Fatal("clearing the query must keep the type filter and the inventory total")
	}
	page.MustElement(`.infra-status-reset`).MustClick()
	infraPolishSettle(page)
	if got := page.MustEval(`()=>JSON.stringify({resetHidden:document.querySelector('.infra-status-reset').hidden,focusOnMap:document.querySelector('[data-stage]').contains(document.activeElement),active:document.activeElement.tagName})`).Str(); !page.MustEval(`()=>document.querySelector('.infra-status-reset').hidden&&document.querySelector('[data-stage]').contains(document.activeElement)`).Bool() {
		t.Fatalf("reset must clear filters and return focus to the map: %s", got)
	}
	if !page.MustEval(`()=>document.querySelector('[data-kind-filter]').value===''`).Bool() {
		t.Fatal("the footer reset must also clear the filter form")
	}

	// Review actual themes with the inspector open, then the small-window search and status controls.
	page.MustElement(`[data-node="fixture-node-0003"]`).MustClick()
	for _, theme := range infraReviewThemes[:3] {
		applyInfraTheme(page, theme)
		infraPolishSettle(page)
		if err := os.WriteFile(infraReviewArtifactPath(t, "polish-"+theme.name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustElement(`[data-action="close-inspector"]`).MustClick()
	infraPolishResize(page, 360, 700)
	page.MustElement(`[data-action="open-search"]`).MustClick()
	page.MustElement(`[data-search]`).MustInput("Cold-store")
	page.MustWait(`()=>!document.querySelector('.infra-search-clear').hidden`)
	page.MustElement(`.infra-search-clear`).MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('[data-search]')&&document.querySelector('.infra-app').dataset.searchOpen==='true'`)
	page.Keyboard.MustType(input.Escape)
	if !page.MustEval(`()=>{const footer=document.querySelector('.infra-statusbar'),live=footer.querySelector('[data-live]'),refresh=footer.querySelector('[data-action="refresh"]'),box=footer.getBoundingClientRect(),r=refresh.getBoundingClientRect();return footer.scrollWidth<=footer.clientWidth+1&&live.offsetWidth>0&&live.querySelector('.infra-sync-label').offsetWidth>0&&!refresh.disabled&&r.top>=box.top&&r.bottom<=box.bottom&&!live.hasAttribute('aria-live')&&!live.hasAttribute('role');}`).Bool() {
		t.Fatal("a 360 px window must keep the status text and refresh control visible without poll announcements")
	}
	if err := os.WriteFile(infraReviewArtifactPath(t, "polish-narrow.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// A failed refresh keeps last-known totals and can be retried directly from the status bar.
func TestDesktopInfraWorkspaceRefreshBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	flaky := &infraFlakySnapshot{base: fixture}
	page := infraBrowserPageWithFixture(t, flaky)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>document.querySelector('[data-live]')?.dataset.sync==='live'`)
	infraPolishResize(page, 1100, 800)
	flaky.fail.Store(true)
	page.MustElement(`.infra-statusbar [data-action="refresh"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='error'&&!document.querySelector('.infra-status-refresh').disabled`)
	if !page.MustEval(`()=>document.querySelector('[data-inventory-count]').textContent===fixtureWords['desktop.infra_asset_count'].replace('{count}',25)`).Bool() {
		t.Fatal("failed refreshes must preserve the last known inventory total")
	}
	if !page.MustEval(`()=>{const tools=document.querySelector('[data-tools]').getBoundingClientRect(),toast=document.querySelector('[data-toast]').getBoundingClientRect(),footer=document.querySelector('.infra-statusbar').getBoundingClientRect();return toast.bottom<=tools.top&&toast.bottom<=footer.top;}`).Bool() {
		t.Fatal("a refresh failure must leave both the canvas tools and the status bar reachable")
	}
	flaky.fail.Store(false)
	page.MustElement(`.infra-status-refresh`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='live'&&!document.querySelector('.infra-status-refresh').disabled`)
	if !page.MustEval(`()=>!document.querySelector('[data-live]').classList.contains('is-error')`).Bool() {
		t.Fatal("successful refresh must clear the failure state")
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

func TestDesktopInfraWorkspacePolishLocalesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 360, 700)
	defer page.Close()
	for _, code := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var all map[string]any
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+code+".json")), &all); err != nil {
			t.Fatal(err)
		}
		words := map[string]any{}
		for key, value := range all {
			if strings.HasPrefix(key, "desktop.infra") {
				words[key] = value
			}
		}
		page.MustEval(`(words)=>{Object.assign(fixtureWords,words);const el=document.querySelector('[data-kind-filter]');el.value=el.value?'':'device';el.dispatchEvent(new Event('change',{bubbles:true}));}`, words)
		page.MustEval(`()=>document.fonts.ready`)
		if !page.MustEval(`()=>{const bar=document.querySelector('.infra-statusbar'),live=bar.querySelector('.infra-sync-label');return bar.scrollWidth<=bar.clientWidth+1&&live.offsetWidth>0&&live.scrollWidth<=live.clientWidth+1;}`).Bool() {
			t.Fatalf("%s: the footer must keep its sync label readable at 360 px", code)
		}
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
