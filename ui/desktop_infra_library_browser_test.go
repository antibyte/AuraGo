package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopInfraLibrarySectionsBrowser covers the library sections and the selection inspector against a
// fleet with duplicates, a stale source and ignored sources: an attention row opens the Sources tab, a duplicate
// is removed and its source ignored (confirmation, DELETE with ignore_sources), Restore brings a source back,
// and the selection inspector moves two assets into a place in one layout save.
func TestDesktopInfraLibrarySectionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	src := func(kind, instance, key, name, state string) map[string]any {
		return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": state, "observed": map[string]any{"name": name}}
	}
	add := func(id, kind, name, icon, ip string, placement map[string]any, sources ...map[string]any) {
		node := infraCanvasFixtureNode(id, kind, name, icon, placement, "")
		node["profile"] = map[string]any{"ip_address": ip, "tags": []any{}, "interfaces": []any{}, "links": []any{}, "app_bindings": []any{}, "custom": map[string]any{}}
		list := []any{}
		for _, source := range sources {
			list = append(list, source)
		}
		node["sources"] = list
		fixture.nodes = append(fixture.nodes, node)
	}
	add("dup-a", "device", "Backup NAS", "nas", "192.0.2.77", map[string]any{"x": 900, "y": 700, "pinned": true}, src("registry", "inv", "r1", "Backup NAS", "current"))
	add("dup-b", "device", "Backup NAS mDNS", "nas", "192.0.2.77", nil, src("registry", "inv", "r2", "Backup NAS mDNS", "current"))
	add("old-switch", "device", "Garage switch", "managed-switch", "192.0.2.5", map[string]any{"x": 900, "y": 900, "pinned": true}, src("proxmox", "pve", "n1", "Garage switch", "stale"))
	add("room-b", "room", "Room B", "room", "", map[string]any{"x": 2400, "y": 100, "w": 480, "h": 320, "pinned": true})
	fixture.ignoredRows = map[string]map[string]any{
		infraFixtureSourceKey(src("registry", "inv", "old-tv", "", "")):     src("registry", "inv", "old-tv", "Old living room TV", "current"),
		infraFixtureSourceKey(src("fritzbox", "fritz", "phone-77", "", "")): src("fritzbox", "fritz", "phone-77", "Spare phone", "current"),
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-cleanup^="duplicate:"]')&&!!document.querySelector('[data-library-item="old-switch"][data-section="attention"]')`)

	// The Add button owns the accent: cleanup actions are compact secondary buttons and Dismiss is a ghost icon.
	if !page.MustEval(`()=>{const library=document.querySelector('.infra-library'),accent=getComputedStyle(document.querySelector('[data-action="add"]')).backgroundColor,actions=[...library.querySelectorAll('[data-cleanup-action]')];return !library.querySelector('.infra-button-primary')&&actions.length>0&&actions.every(button=>getComputedStyle(button).backgroundColor!==accent)&&[...library.querySelectorAll('[data-cleanup-dismiss]')].every(button=>!!button.querySelector('svg')&&button.title!==''&&button.textContent.trim()==='');}`).Bool() {
		t.Fatal("the library must hold no accent-filled button; Dismiss is an icon button with a tooltip")
	}
	// An attention row opens the asset on the tab that explains the problem.
	page.MustEval(`()=>document.querySelector('[data-library-item="old-switch"][data-section="attention"] [data-library-select]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Garage switch'&&document.querySelector('[data-inspector-tab="sources"]').getAttribute('aria-selected')==='true'`)

	// Ignored sources are listed (collapsed) with Restore; the ledger does not count them.
	if !page.MustEval(`()=>{const list=document.querySelector('[data-section-list="ignored"]');return list.hidden&&list.querySelectorAll('[data-ignored-source]').length===2&&document.querySelector('[data-section-count="ignored"]').textContent==='2';}`).Bool() {
		t.Fatal("the Ignored section must start collapsed with its two sources")
	}

	// Removing the duplicate asks first, then deletes with ignore_sources and lists its source as ignored.
	page.MustEval(`()=>document.querySelector('[data-cleanup-action^="duplicate:"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-choice="discard"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-cleanup^="duplicate:"]')&&document.querySelector('[data-section-count="ignored"]').textContent==='3'`)
	fixture.mu.Lock()
	deletes := append([]string(nil), fixture.nodeDeletes...)
	fixture.mu.Unlock()
	if len(deletes) != 1 || deletes[0] != "dup-b?version=1&ignore_sources=1" {
		t.Fatalf("the duplicate was removed with %v; want the weaker asset with ignore_sources", deletes)
	}

	// Restore brings the source back: the row leaves Ignored and the server forgets the ignore.
	page.MustEval(`()=>document.querySelector('[data-section-toggle="ignored"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-section-list="ignored"]').hidden`)
	restore := fmt.Sprintf(`()=>document.querySelector('[data-restore-source="%s"]').click()`, infraAppSourceID("registry", "inv", "old-tv"))
	page.MustEval(restore)
	page.MustWait(`()=>document.querySelectorAll('[data-ignored-source]').length===2`)
	fixture.mu.Lock()
	_, stillIgnored := fixture.ignoredRows[infraFixtureSourceKey(src("registry", "inv", "old-tv", "", ""))]
	fixture.mu.Unlock()
	if stillIgnored {
		t.Fatal("Restore did not clear the ignore on the server")
	}

	// A selection of two assets moves into a place with one layout save.
	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	page.MustEval(`()=>{for(const id of ['fixture-node-0003','fixture-node-0004'])document.querySelector('[data-node="'+id+'"]').dispatchEvent(new MouseEvent('click',{bubbles:true,shiftKey:true}));}`)
	page.MustWait(`()=>!!document.querySelector('[data-action="move-selection"]')`)
	before := len(layoutCalls())
	page.MustEval(`()=>document.querySelector('[data-action="move-selection"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-place-option="room-b"]')`)
	if !page.MustEval(`()=>!document.querySelector('[data-picker] [data-place-option="fixture-node-0003"]')&&!!document.querySelector('[data-picker] [data-place-option=""]')`).Bool() {
		t.Fatal("the place picker offers places and No place, never assets")
	}
	page.MustEval(`()=>document.querySelector('[data-picker] [data-place-option="room-b"]').click()`)
	deadline := time.Now().Add(8 * time.Second)
	for len(layoutCalls()) <= before {
		if time.Now().After(deadline) {
			t.Fatal("moving the selection into a place saved nothing")
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(250 * time.Millisecond) // a stray second save would land here
	calls := layoutCalls()[before:]
	moved := 0
	for _, item := range calls[0]["nodes"].([]map[string]any) {
		if item["parent_id"] == "room-b" && (item["id"] == "fixture-node-0003" || item["id"] == "fixture-node-0004") {
			moved++
		}
	}
	if moved != 2 || len(calls) != 1 {
		t.Fatalf("moving the selection saved %v; want both assets into room-b in one call", calls)
	}
	if text := strings.TrimSpace(page.MustEval(`()=>document.querySelector('[data-inspector] h2')?.textContent||''`).Str()); text == "" {
		t.Fatal("the inspector should still show the selection summary")
	}

	// The header edits the name inline (Enter keeps it in the draft), the icon picker changes the draft icon,
	// the Details tab is marked, and Ctrl S saves both in one asset update.
	page.MustEval(`()=>document.querySelector('[data-node="old-switch"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Garage switch'`)
	page.MustEval(`()=>document.querySelector('[data-action="rename"]').click()`)
	page.MustWait(`()=>{const input=document.querySelector('.infra-head-name-input');return !input.hidden&&document.activeElement===input;}`)
	page.MustEval(`()=>{const input=document.querySelector('.infra-head-name-input');input.value='Garage switch 2';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Garage switch 2'&&document.querySelector('.infra-head-name-input').hidden`)
	page.MustEval(`()=>document.querySelector('[data-action="pick-icon"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-icon-picker] [data-pick-icon="router"]')&&document.activeElement===document.querySelector('[data-icon-search]')`)
	page.MustEval(`()=>document.querySelector('[data-icon-picker] [data-pick-icon="router"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-icon-picker]')&&!document.querySelector('.infra-tab-dirty').hidden&&document.querySelector('[data-node-field="icon"]').value==='router'`)
	page.KeyActions().Press(input.ControlLeft).Type(input.KeyS).Release(input.ControlLeft).MustDo()
	page.MustWait(`()=>document.querySelector('.infra-tab-dirty').hidden&&document.querySelector('[data-inspector] h2')?.textContent==='Garage switch 2'`)
	fixture.mu.Lock()
	saved := map[string]any{}
	for _, node := range fixture.nodes {
		if node["id"] == "old-switch" {
			saved = node
		}
	}
	fixture.mu.Unlock()
	if saved["name"] != "Garage switch 2" || saved["icon"] != "router" {
		t.Fatalf("Ctrl S saved name %v and icon %v", saved["name"], saved["icon"])
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
