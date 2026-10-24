package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func infraPanelsSource(kind, instance, key, name string) map[string]any {
	return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current", "observed": map[string]any{"name": name}}
}

func infraPanelsNode(id, kind, name, icon, ip string, placement, source map[string]any) map[string]any {
	node := infraCanvasFixtureNode(id, kind, name, icon, placement, "")
	if ip != "" {
		node["profile"].(map[string]any)["ip_address"] = ip
	}
	if source != nil {
		node["sources"] = []any{source}
	}
	return node
}

func infraPanelsAt(x, y float64) map[string]any {
	return map[string]any{"x": x, "y": y, "pinned": true}
}

func infraPanelsPage(t *testing.T, fixture *infraAppBrowserFixture) *rod.Page {
	t.Helper()
	page := infraBrowserPageWithFixture(t, fixture)
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>Object.keys(localStorage).filter(key=>key.startsWith('infra.')).forEach(key=>localStorage.removeItem(key))`)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-nodes] [data-node]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	return page
}

// infraPanelsCenter is the centre of an element once the scene stopped animating.
func infraPanelsCenter(page *rod.Page, selector string) (float64, float64) {
	page.MustWait(`()=>!document.querySelector('[data-scene]').classList.contains('is-animating')`)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, selector).Arr()
	return point[0].Num(), point[1].Num()
}

func infraPanelsWait(t *testing.T, fixture *infraAppBrowserFixture, what string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		fixture.mu.Lock()
		ok := ready()
		fixture.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func infraPanelsConfirm(page *rod.Page) {
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-choice="discard"]').click()`)
}

// infraPanelsMenu lists the open context menu as "id,id!" (a trailing ! marks a disabled item).
func infraPanelsMenu(page *rod.Page) string {
	page.MustWait(`()=>document.querySelectorAll('.vd-context-menu [data-context-action]').length>0`)
	return page.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu [data-context-action]')].map(button=>button.dataset.contextAction+(button.disabled?'!':'')).join(',')`).Str()
}

func infraPanelsCloseMenu(page *rod.Page) {
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('.vd-context-menu')`)
}

// TestDesktopInfraCleanupAndIgnoreBrowser covers engine host assignment, remove-and-ignore from a card
// context menu, the duplicate cleanup with its Ignored list and Restore.
func TestDesktopInfraCleanupAndIgnoreBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.failCandidateOnce = ""
	registry := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	fixture.nodes = []map[string]any{
		infraPanelsNode("core-switch", "device", "Core switch", "managed-switch", "192.168.6.2", infraPanelsAt(160, 200), registry("dev-switch", "Core switch")),
		infraPanelsNode("tv", "device", "Living room TV", "tv", "192.168.6.30", infraPanelsAt(160, 380), registry("dev-tv", "Living room TV")),
		infraPanelsNode("tv-uuid", "device", "Chromecast-3c1d2e4f5a6b7c8d", "tv", "192.168.6.30", nil, registry("dev-tv-uuid", "Chromecast-3c1d2e4f5a6b7c8d")),
		infraPanelsNode("old-nas", "device", "Old NAS", "nas", "192.168.6.21", infraPanelsAt(640, 380), registry("dev-old-nas", "Old NAS")),
		infraPanelsNode("host-1", "device", "Docker host", "tower-server", "192.168.6.10", infraPanelsAt(640, 200), nil),
		infraPanelsNode("ct-1", "container", "radarr", "container", "", nil, infraPanelsSource("docker", "engine-a", "c1", "radarr")),
		infraPanelsNode("ct-2", "container", "sonarr", "container", "", nil, infraPanelsSource("docker", "engine-a", "c2", "sonarr")),
	}
	fixture.candidates = nil
	page := infraPanelsPage(t, fixture)
	defer page.Close()

	page.MustWait(`()=>!!document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"]')&&!!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')`)
	if !page.MustEval(`()=>document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"]').textContent.includes('Living room TV')`).Bool() {
		t.Fatal("the duplicate suggestion must name the asset it keeps")
	}

	// Assign the Docker engine host.
	page.MustEval(`()=>document.querySelector('[data-cleanup="engine-host:docker:engine-a"] [data-cleanup-action]').click()`)
	page.MustElement(`[data-picker] [data-picker-input]`).MustInput("Docker")
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-node-option="host-1"]')`)
	page.MustEval(`()=>document.querySelector('[data-picker] [data-node-option="host-1"]').click()`)
	infraPanelsWait(t, fixture, "the engine host write", func() bool { return len(fixture.sourceHostPuts) == 1 })
	fixture.mu.Lock()
	hostCall := fixture.sourceHostPuts[0]
	fixture.mu.Unlock()
	if hostCall["instance"] != "engine-a" || hostCall["node_id"] != "host-1" || hostCall["version"] != float64(0) {
		t.Fatalf("engine host write = %#v", hostCall)
	}
	page.MustWait(`()=>!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')&&!!document.querySelector('[data-toggle-host-services="host-1"]')`)

	// Remove and ignore through the card context menu.
	nx, ny := infraPanelsCenter(page, `[data-node="old-nas"]`)
	page.Mouse.MustMoveTo(nx, ny)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	page.MustWait(`()=>!!document.querySelector('.vd-context-menu [data-context-action="remove-ignore"]')`)
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="remove-ignore"]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "remove and ignore", func() bool { return len(fixture.nodeDeletes) == 1 })
	fixture.mu.Lock()
	removal := fixture.nodeDeletes[0]
	fixture.mu.Unlock()
	if removal != "old-nas?version=1&ignore_sources=1" {
		t.Fatalf("remove and ignore sent %q", removal)
	}
	oldNas := infraAppSourceID("registry", "inventory", "dev-old-nas")
	page.MustWait(fmt.Sprintf(`()=>!document.querySelector('[data-node="old-nas"]')&&!!document.querySelector('[data-ignored-source="%s"]')`, oldNas))

	// Remove the duplicate from Cleanup: the opaque twin goes and its sources are ignored.
	page.MustEval(`()=>document.querySelector('[data-cleanup^="duplicate:ip:192.168.6.30:"] [data-cleanup-action]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "duplicate removal", func() bool { return len(fixture.nodeDeletes) == 2 })
	fixture.mu.Lock()
	duplicate := fixture.nodeDeletes[1]
	fixture.mu.Unlock()
	if duplicate != "tv-uuid?version=1&ignore_sources=1" {
		t.Fatalf("cleanup removed %q; want the opaque twin with its sources ignored", duplicate)
	}
	page.MustWait(`()=>!document.querySelector('[data-cleanup^="duplicate:"]')&&!!document.querySelector('[data-node="tv"]')&&document.querySelectorAll('[data-ignored-source]').length===2`)

	// Restore from Ignored.
	page.MustEval(fmt.Sprintf(`()=>document.querySelector('[data-restore-source="%s"]').click()`, oldNas))
	infraPanelsWait(t, fixture, "restore", func() bool { return len(fixture.ignoreCalls) == 1 })
	fixture.mu.Lock()
	restore := fixture.ignoreCalls[0]
	fixture.mu.Unlock()
	if restore["ignored"] != false {
		t.Fatalf("restore sent %#v", restore)
	}
	page.MustWait(fmt.Sprintf(`()=>!document.querySelector('[data-ignored-source="%s"]')`, oldNas))
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraInspectorAndListBrowser covers the tab keyboard pattern, the app link picker's MeshCore
// rule, the sticky save bar with Ctrl S, List sorting and bulk remove-and-ignore.
func TestDesktopInfraInspectorAndListBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	registry := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	fixture.nodes = []map[string]any{
		infraPanelsNode("tv", "device", "Living room TV", "tv", "192.168.6.30", infraPanelsAt(160, 200), registry("dev-tv", "Living room TV")),
		infraPanelsNode("relay", "device", "Garden relay", "mesh-node", "", infraPanelsAt(400, 200), nil),
		infraPanelsNode("alpha", "device", "Alpha box", "mini-pc", "192.168.6.9", infraPanelsAt(160, 400), registry("dev-alpha", "Alpha box")),
		infraPanelsNode("mid", "device", "Middle box", "mini-pc", "192.168.6.10", infraPanelsAt(400, 400), nil),
		infraPanelsNode("zeta", "device", "Zeta box", "mini-pc", "192.168.6.40", infraPanelsAt(640, 400), registry("dev-zeta", "Zeta box")),
	}
	fixture.targets = append(fixture.targets, map[string]any{"app": "meshcore", "target_id": "conv-ipt", "label": "Iptingen Room Server"})
	page := infraPanelsPage(t, fixture)
	defer page.Close()

	page.MustEval(`()=>document.querySelector('[data-node="tv"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector-tab="overview"]')?.getAttribute('aria-selected')==='true'`)
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="overview"]').focus()`)
	activeTab := func() string {
		return page.MustEval(`()=>{const el=document.activeElement,tab=el?.dataset?.inspectorTab||'';const panel=document.querySelector('[data-inspector-panel="'+tab+'"]');return tab+(panel&&!panel.hidden&&el.getAttribute('aria-selected')==='true'?'':'!');}`).Str()
	}
	for _, step := range []struct {
		key  input.Key
		want string
	}{{input.ArrowRight, "details"}, {input.End, "connections"}, {input.Home, "overview"}, {input.ArrowLeft, "connections"}} {
		page.Keyboard.MustType(step.key)
		if got := activeTab(); got != step.want {
			t.Fatalf("tab key moved to %q; want %q", got, step.want)
		}
	}
	page.MustEval(`()=>document.querySelector('[data-inspector-tab="details"]').click()`)

	// A TV is offered no MeshCore conversations until two characters are typed.
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]')`)
	if got := page.MustEval(`()=>JSON.stringify({mesh:document.querySelectorAll('[data-app-picker] [data-app-target][data-app="meshcore"]').length,note:document.querySelector('[data-app-picker] [data-picker-note]').textContent})`).Str(); !strings.Contains(got, `"mesh":0`) || !strings.Contains(got, "2") {
		t.Fatalf("a TV must not see MeshCore before typing: %s", got)
	}
	page.MustElement(`[data-app-picker] [data-picker-input]`).MustInput("Ip")
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="meshcore"][data-target-id="conv-ipt"]')`)
	page.Keyboard.MustType(input.Escape)

	// A mesh node sees them at once; the tab choice persisted for the kind.
	page.MustEval(`()=>document.querySelector('[data-node="relay"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Garden relay'&&document.querySelector('[data-inspector-tab="details"]').getAttribute('aria-selected')==='true'`)
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="meshcore"][data-target-id="conv-ipt"]')`)
	page.MustEval(`()=>document.querySelector('[data-app-picker] [data-app-target][data-target-id="conv-ipt"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-binding="meshcore:conv-ipt"]')&&!document.querySelector('[data-savebar]').hidden&&!!document.querySelector('[data-savebar] .infra-draft-banner')`)

	// Field markers and Ctrl S.
	page.MustEval(`()=>{const field=document.querySelector('[data-profile="vendor"]');field.closest('details').open=true;field.value='Heltec';field.dispatchEvent(new Event('input',{bubbles:true}));field.focus();}`)
	page.MustWait(`()=>document.querySelector('[data-field="profile.vendor"]')?.dataset.dirty==='true'&&!document.querySelector('[data-inspector-tab="details"] .infra-tab-dirty').hidden`)
	page.KeyActions().Press(input.ControlLeft).Type('s').MustDo()
	infraPanelsWait(t, fixture, "the Ctrl S save", func() bool { return len(fixture.nodePuts) == 1 })
	fixture.mu.Lock()
	saved := fixture.nodePuts[0]
	fixture.mu.Unlock()
	profile, _ := saved["profile"].(map[string]any)
	bindings, _ := profile["app_bindings"].([]any)
	if saved["id"] != "relay" || profile["vendor"] != "Heltec" || len(bindings) != 1 {
		t.Fatalf("Ctrl S saved %#v", saved)
	}
	page.MustWait(`()=>document.querySelector('[data-savebar]').hidden&&!document.querySelector('.infra-draft-banner')`)

	// List view sorting.
	page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length===5`)
	order := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('[data-table-body] [data-row]')].map(row=>row.dataset.row).join(',')`).Str()
	}
	if got := order(); got != "alpha,relay,tv,mid,zeta" {
		t.Fatalf("default List order = %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-sort="name"]').click()`)
	page.MustWait(`()=>document.querySelector('th[data-sort-col="name"]').getAttribute('aria-sort')==='descending'`)
	if got := order(); got != "zeta,mid,tv,relay,alpha" {
		t.Fatalf("descending name order = %s", got)
	}
	page.MustEval(`()=>document.querySelector('[data-sort="address"]').click()`)
	page.MustWait(`()=>document.querySelector('th[data-sort-col="address"]').getAttribute('aria-sort')==='ascending'&&!document.querySelector('th[data-sort-col="name"]').hasAttribute('aria-sort')`)
	if got := order(); got != "alpha,mid,tv,zeta,relay" {
		t.Fatalf("address order = %s; want numeric IPv4 with the address-less asset last", got)
	}

	// Bulk remove and ignore. The selection left over from the panel (Garden relay) is cleared first.
	page.MustEval(`()=>document.querySelector('[data-action="bulk-clear"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulkbar]').hidden`)
	page.MustEval(`()=>{document.querySelector('[data-row-check="alpha"]').click();document.querySelector('[data-row-check="zeta"]').click();}`)
	page.MustWait(`()=>!document.querySelector('[data-bulkbar]').hidden&&document.querySelector('[data-bulk-count]').textContent.includes('2')`)
	page.MustEval(`()=>document.querySelector('[data-action="bulk-ignore"]').click()`)
	infraPanelsConfirm(page)
	infraPanelsWait(t, fixture, "bulk remove and ignore", func() bool { return len(fixture.nodeDeletes) == 2 })
	fixture.mu.Lock()
	calls := strings.Join(fixture.nodeDeletes, " ")
	fixture.mu.Unlock()
	if !strings.Contains(calls, "alpha?version=1&ignore_sources=1") || !strings.Contains(calls, "zeta?version=1&ignore_sources=1") {
		t.Fatalf("bulk remove and ignore sent %s", calls)
	}
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length===3`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraMenusAndShortcutsBrowser covers the context menus (card, place, connection, empty map,
// keyboard), Remove from map, the filter chips, the legend, the shortcut sheet and the List bulk bar at a
// narrow width, where the inspector is an overlay.
func TestDesktopInfraMenusAndShortcutsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.failCandidateOnce = ""
	registry := func(key, name string) map[string]any { return infraPanelsSource("registry", "inventory", key, name) }
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 560, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraPanelsNode("dev-a", "device", "Edge router", "router", "192.168.6.1", infraPanelsAt(160, 200), registry("dev-router", "Edge router")),
		infraPanelsNode("dev-b", "device", "Core switch", "managed-switch", "192.168.6.2", infraPanelsAt(160, 420), nil),
		infraPanelsNode("dev-c", "device", "Cold NAS", "nas", "192.168.6.3", nil, nil),
		infraPanelsNode("dev-d", "vm", "Build VM", "vm", "192.168.6.4", infraPanelsAt(360, 420), nil),
	}
	fixture.edges = []map[string]any{
		{"id": "edge-manual", "version": 1, "from": "dev-a", "to": "dev-b", "kind": "ethernet"},
		{"id": "edge-provider", "version": 1, "from": "dev-b", "to": "dev-d", "kind": "depends_on", "source": "proxmox"},
	}
	fixture.candidates = nil
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	page.MustEval(`()=>{window.w=(key,vars={})=>Object.entries(vars).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',v),window.fixtureWords[key]);}`)

	// A card offers its actions; Remove from map unplaces it in one layout save and Undo puts it back.
	cx, cy := infraPanelsCenter(page, `[data-node="dev-a"]`)
	page.Mouse.MustMoveTo(cx, cy)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	if got := infraPanelsMenu(page); got != "open,move,connect,unplace,remove,remove-ignore" {
		t.Fatalf("card menu = %s", got)
	}
	// The menu selects the asset it belongs to (the card repaints on the next frame).
	page.MustWait(`()=>document.querySelector('[data-node="dev-a"]').getAttribute('aria-pressed')==='true'`)
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="unplace"]').click()`)
	infraPanelsWait(t, fixture, "the unplace save", func() bool {
		last := fixture.layoutCalls
		if len(last) == 0 {
			return false
		}
		nodes, _ := last[len(last)-1]["nodes"].([]map[string]any)
		return len(nodes) == 1 && nodes[0]["id"] == "dev-a" && nodes[0]["unplace"] == true
	})
	page.MustWait(`()=>!document.querySelector('[data-node="dev-a"]')&&!!document.querySelector('[data-library-item="dev-a"][data-section="to-place"]')`)
	page.MustEval(`()=>document.querySelector('.infra-history [data-action="undo"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')`)

	// A place has its own menu: rename opens the inspector in rename mode.
	hx, hy := infraPanelsCenter(page, `[data-region-header="room-1"]`)
	page.Mouse.MustMoveTo(hx, hy)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	if got := infraPanelsMenu(page); got != "open,rename,move,unplace,remove" {
		t.Fatalf("place menu = %s", got)
	}
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="rename"]').click()`)
	page.MustWait(`()=>!document.querySelector('.infra-head-name-input').hidden&&document.activeElement===document.querySelector('.infra-head-name-input')`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('.infra-head-name-input').hidden&&document.querySelector('[data-inspector-tab="details"] .infra-tab-dirty').hidden&&document.querySelector('[data-savebar]').hidden`)

	// Connections: a manual one can be edited or removed, a provider one is read-only.
	menuOnEdge := func(id string) string {
		page.MustEval(`id=>{const hit=document.querySelector('[data-edge="'+id+'"] .infra-edge-hit');const length=hit.getTotalLength(),point=hit.getPointAtLength(length/2),matrix=hit.getScreenCTM(),x=matrix.a*point.x+matrix.c*point.y+matrix.e,y=matrix.b*point.x+matrix.d*point.y+matrix.f;hit.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:x,clientY:y}));}`, id)
		return infraPanelsMenu(page)
	}
	if got := menuOnEdge("edge-manual"); got != "edit,remove" {
		t.Fatalf("manual connection menu = %s", got)
	}
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="edit"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-edge-inline-form]')&&document.activeElement===document.querySelector('.infra-edge-inline [name="kind"]')`)
	if got := menuOnEdge("edge-provider"); got != "edit!,remove!" {
		t.Fatalf("provider connection menu = %s; both items must be disabled", got)
	}
	if !page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="edit"]').title===w('desktop.infra_connection_readonly')`).Bool() {
		t.Fatal("a disabled connection item must say why")
	}
	infraPanelsCloseMenu(page)

	// The empty map offers creation and fit; Import opens the sheet.
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]')?.click()`)
	page.MustWait(`()=>document.querySelector('[data-inspector]').hidden`)
	sx, sy := infraPanelsCenter(page, `[data-stage]`)
	page.Mouse.MustMoveTo(sx+300, sy+300)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	if got := infraPanelsMenu(page); got != "add,add-place,import,fit" {
		t.Fatalf("map menu = %s", got)
	}
	page.MustEval(`()=>document.querySelector('.vd-context-menu [data-context-action="import"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-action="close-import"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)

	// The keyboard opens the same menu: Shift+F10 on a library row, once, with the unplaced row's items.
	page.MustEval(`()=>document.querySelector('[data-library-item="dev-c"] [data-library-select]').focus()`)
	page.KeyActions().Press(input.ShiftLeft).Type(input.F10).MustDo()
	if got := infraPanelsMenu(page); got != "open,move,connect,remove" {
		t.Fatalf("library row menu = %s", got)
	}
	if count := page.MustEval(`()=>document.querySelectorAll('.vd-context-menu').length`).Int(); count != 1 {
		t.Fatalf("Shift+F10 opened %d menus", count)
	}
	infraPanelsCloseMenu(page)

	// Text fields keep their keys: a question mark typed into the search is text, not the shortcut sheet.
	page.MustEval(`()=>document.querySelector('[data-search]').focus()`)
	page.Keyboard.MustType(input.Key('?'))
	if page.MustEval(`()=>document.querySelector('[data-dialog]').open||document.querySelector('[data-search]').value!=='?'`).Bool() {
		t.Fatal("a question mark typed into the search field must stay text")
	}
	page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));search.blur();}`)

	// The shortcut sheet opens with ? on the map, lists three groups and closes with Escape.
	page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
	page.Keyboard.MustType(input.Key('?'))
	page.MustWait(`()=>document.querySelector('[data-dialog]').open&&document.querySelectorAll('.infra-shortcuts section').length===3`)
	if !page.MustEval(`()=>document.querySelector('[data-dialog] h2').textContent===w('desktop.infra_shortcuts')&&document.querySelectorAll('.infra-shortcuts kbd').length>20`).Bool() {
		t.Fatal("the shortcut sheet needs its title and key caps")
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('[data-dialog]').open`)

	// The legend: More > Legend shows the status, power and connection keys and takes the focus.
	page.MustEval(`()=>document.querySelector('[data-action="more"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-menu="more"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-menu="more"] [data-action="legend"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-popover="legend"]').hidden&&document.activeElement===document.querySelector('[data-popover="legend"]')`)
	if got := page.MustEval(`()=>[document.querySelectorAll('.infra-legend-ring').length,document.querySelectorAll('.infra-legend-dot').length,document.querySelectorAll('.infra-legend-line').length].join()`).Str(); got != "5,3,5" {
		t.Fatalf("legend has %s status, power and connection keys; want 5,3,5", got)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-popover="legend"]').hidden&&document.activeElement===document.querySelector('[data-action="more"]')`)

	// Filter chips: one per active filter, removable, with Clear filters in the popover.
	page.MustEval(`()=>document.querySelector('[data-action="filters"]').click()`)
	page.MustEval(`()=>{const kind=document.querySelector('[data-kind-filter]');kind.value='device';kind.dispatchEvent(new Event('change',{bubbles:true}));const power=document.querySelector('[data-power-filter]');power.value='none';power.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===2&&!document.querySelector('[data-filter-chips]').hidden&&!document.querySelector('[data-action="clear-filters"]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-remove-filter="kind"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===1&&document.querySelector('[data-kind-filter]').value===''&&document.activeElement===document.querySelector('[data-remove-filter="power"]')`)
	page.MustEval(`()=>document.querySelector('[data-remove-filter="power"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-filter-chips]').hidden&&document.querySelector('[data-power-filter]').value===''&&document.activeElement===document.querySelector('[data-search]')`)
	page.MustEval(`()=>{const source=document.querySelector('[data-source-filter]');source.value='registry';source.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===1`)
	page.MustEval(`()=>document.querySelector('[data-action="clear-filters"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-remove-filter]').length===0&&document.querySelector('[data-source-filter]').value===''`)
	page.Keyboard.MustType(input.Escape)

	// At 1000 px the inspector is an overlay: checking rows in the List never covers the bulk bar.
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='1000px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth<1100`)
	page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length>=4`)
	page.MustEval(`()=>document.querySelector('[data-action="bulk-clear"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-bulkbar]').hidden`)
	page.MustEval(`()=>{document.querySelector('[data-row-check="dev-a"]').click();document.querySelector('[data-row-check="dev-b"]').click();}`)
	page.MustWait(`()=>!document.querySelector('[data-bulkbar]').hidden&&document.querySelector('[data-bulk-count]').textContent===w('desktop.infra_selected_count',{count:2})`)
	if got := page.MustEval(`()=>{const bar=document.querySelector('[data-bulkbar]'),r=bar.getBoundingClientRect(),hit=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return JSON.stringify({bar:!!hit&&bar.contains(hit),inspector:!document.querySelector('[data-inspector]').hidden,scrim:getComputedStyle(document.querySelector('[data-scrim]')).display});}`).Str(); got != `{"bar":true,"inspector":false,"scrim":"none"}` {
		t.Fatalf("the bulk bar is covered at 1000 px: %s", got)
	}
	// A click on an asset's name still opens its panel.
	page.MustEval(`()=>document.querySelector('[data-row="dev-b"] [data-select]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden&&document.querySelector('[data-inspector] h2')?.textContent==='Core switch'`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
