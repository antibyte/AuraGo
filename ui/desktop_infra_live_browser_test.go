package ui

import (
	"os"
	"testing"
)

// TestDesktopInfraLiveIndicatorBrowser covers the reachability indicator of the atlas: the live dot rides on
// the asset icon of the map card and of the List row, the inspector names the state, and an asset without a
// signal shows nothing at all.
func TestDesktopInfraLiveIndicatorBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(8)
	// The first asset of the fixture rides in the card of its host, so the reachability
	// of the assets that carry a card is checked.
	for index, node := range fixture.nodes {
		switch index {
		case 2:
			node["live"] = "live"
		case 3:
			node["live"] = "offline"
		case 4:
			node["live"] = "unknown"
		}
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1600, 1000, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="fixture-node-0002"]')`)

	// The map card carries the live dot on the icon; an asset without a signal carries none.
	page.MustWait(`()=>!!document.querySelector('[data-node="fixture-node-0002"] .infra-live-dot[data-live="live"]')`)
	if !page.MustEval(`()=>{
		const live=document.querySelector('[data-node="fixture-node-0002"] .infra-live-dot[data-live="live"]');
		const offline=document.querySelector('[data-node="fixture-node-0003"] .infra-live-dot[data-live="offline"]');
		const unknown=document.querySelector('[data-node="fixture-node-0004"] .infra-live-dot[data-live="unknown"]');
		const quiet=document.querySelector('[data-node="fixture-node-0005"] .infra-live-dot');
		return !!live&&!!offline&&!!unknown&&!quiet&&live.getAttribute('aria-hidden')==='true';
	}`).Bool() {
		t.Fatal("the map card must show the live dot of every asset that has a signal and none for an asset without")
	}

	// The List shows the same dot next to the name and names the state for a screen reader.
	page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-row="fixture-node-0002"] .infra-live-dot[data-live="live"]')`)
	if !page.MustEval(`()=>{
		const row=document.querySelector('[data-row="fixture-node-0002"]');
		const dot=row.querySelector('.infra-live-dot[data-live="live"]');
		const hidden=row.querySelector('.infra-sr-only');
		return !!dot&&!!hidden&&hidden.textContent.includes(window.fixtureWords['desktop.infra_live_live']);
	}`).Bool() {
		t.Fatal("the list row must show the live dot and name the state for a screen reader")
	}

	// The inspector names the state in a pill next to the source status.
	page.MustEval(`()=>document.querySelector('[data-row="fixture-node-0003"] [data-select]').click()`)
	page.MustWait(`()=>!!document.querySelector('.infra-head-pills .infra-live-pill[data-live="offline"]')`)
	if !page.MustEval(`()=>document.querySelector('.infra-head-pills .infra-live-pill[data-live="offline"]').textContent.trim()===window.fixtureWords['desktop.infra_live_offline']`).Bool() {
		t.Fatal("the inspector must name the reachability of the asset")
	}
	if err := os.WriteFile(infraReviewArtifactPath(t, "live-list-indicator.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-node="fixture-node-0002"] .infra-live-dot[data-live="live"]')`)
	if err := os.WriteFile(infraReviewArtifactPath(t, "live-atlas-indicator.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
}
