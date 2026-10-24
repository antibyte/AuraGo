package ui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// infraGermanWord reads one desktop locale text; the browser fixture runs in German.
func infraGermanWord(t *testing.T, key string) string {
	t.Helper()
	var words map[string]any
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &words); err != nil {
		t.Fatal(err)
	}
	text, _ := words[key].(string)
	if text == "" {
		t.Fatalf("de.json lacks %s", key)
	}
	return text
}

func infraTopologyFixture() *infraAppBrowserFixture {
	fixture := newInfraAppBrowserFixture()
	device := func(id, name, icon string, x, y float64) map[string]any {
		return infraPanelsNode(id, "device", name, icon, "", infraPanelsAt(x, y), nil)
	}
	vm := infraPanelsNode("vm1", "vm", "Lab VM", "vm", "", nil, nil)
	vm["host_id"] = "pve"
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room", "room", "Living room", "room", map[string]any{"x": 1400, "y": 0, "w": 360, "h": 240, "pinned": true}, ""),
		device("inet", "Internet", "internet", 120, 120),
		device("ont", "Fiber ONT", "modem-ont", 360, 120),
		device("router", "Edge router", "router", 600, 120),
		device("switch", "Core switch", "managed-switch", 840, 120),
		device("ap", "Attic access point", "access-point", 1080, 120),
		device("nas", "Basement NAS", "nas", 120, 360),
		device("pve", "Proxmox host", "generic-device", 360, 360),
		device("laptop", "Admin laptop", "laptop", 600, 360),
		device("tv", "Living room TV", "tv", 840, 360),
		vm,
		infraPanelsNode("ct-orphan", "container", "watchtower", "container", "", nil, infraPanelsSource("docker", "engine-b", "w1", "watchtower")),
	}
	edge := func(id, kind, from, to string) map[string]any {
		return map[string]any{"id": id, "version": 1, "kind": kind, "from": from, "to": to}
	}
	fixture.edges = []map[string]any{
		edge("e1", "ethernet", "inet", "ont"), edge("e2", "fiber", "ont", "router"), edge("e3", "ethernet", "router", "switch"),
		edge("e4", "ethernet", "switch", "ap"), edge("e5", "ethernet", "switch", "nas"), edge("e6", "ethernet", "pve", "switch"),
		edge("e7", "wifi", "ap", "laptop"), edge("e8", "ethernet", "switch", "tv"), edge("e9", "depends_on", "tv", "nas"),
	}
	return fixture
}

// TestDesktopInfraTopologyBrowser covers spec section 6:
//   - four labelled lanes are drawn behind the cards;
//   - assets without a topology placement are laid out by connection order,
//     and nothing is written;
//   - guests stay nested and places and hostless containers stay out;
//   - dragging into another lane saves a pinned placement;
//   - Arrange keeps that lane while it re-lays everything else in one
//     undoable save.
//
// Cards are 216x64, so auto columns sit 240 px apart: two cards are centred at
// -120/120, three at -240/0/240, four at -360/-120/120/360. Inside a lane the cards are ordered by the barycentre
// of their neighbours in the other lanes (A20): the switch (its children sit below) comes before the router (hung on the ONT above).
func TestDesktopInfraTopologyBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraTopologyFixture()
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	waitCalls := func(want int) []map[string]any {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for {
			if calls := layoutCalls(); len(calls) >= want {
				time.Sleep(250 * time.Millisecond) // a stray extra save would land here
				return layoutCalls()
			}
			if time.Now().After(deadline) {
				t.Fatalf("waited for %d layout saves, got %d", want, len(layoutCalls()))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	item := func(call map[string]any, id string) map[string]any {
		nodes, _ := call["nodes"].([]map[string]any)
		for _, node := range nodes {
			if node["id"] == id {
				return node
			}
		}
		return nil
	}

	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)

	labels, _ := json.Marshal([][2]string{
		{"internet", infraGermanWord(t, "desktop.infra_lane_internet")},
		{"network", infraGermanWord(t, "desktop.infra_lane_network")},
		{"compute", infraGermanWord(t, "desktop.infra_lane_compute")},
		{"clients", infraGermanWord(t, "desktop.infra_lane_clients")},
	})
	if got := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('[data-lanes] [data-lane]')].map(lane=>[lane.dataset.lane,lane.querySelector('.infra-lane-label').textContent.trim()]))`).Str(); got != string(labels) {
		t.Fatalf("lanes = %s; want %s", got, labels)
	}
	if !page.MustEval(`()=>{const first=document.querySelector('[data-scene]').firstElementChild;return first.matches('[data-lanes]')&&first.getAttribute('aria-hidden')==='true';}`).Bool() {
		t.Fatal("lanes must be the first scene layer, behind the cards, and hidden from assistive technology")
	}
	// A lane label is counter-scaled like a card name: 12 px or more on screen at fit and at the smallest zoom.
	const labelPx = `()=>{const scene=document.querySelector('[data-scene]'),style=getComputedStyle(scene),k=parseFloat(style.getPropertyValue('--infra-lane-k'))||1,zoom=new DOMMatrix(style.transform).a,label=document.querySelector('.infra-lane-label'),box=label.getBoundingClientRect();return {px:parseFloat(getComputedStyle(label).fontSize)*k*zoom,height:box.height,zoom};}`
	for _, step := range []string{"fit", "smallest zoom"} {
		if step == "smallest zoom" {
			for i := 0; i < 6; i++ {
				page.MustEval(`()=>document.querySelector('[data-action="zoom-out"]').click()`)
			}
			infraPanelsCenter(page, `[data-lane="network"]`)
		}
		got := page.MustEval(labelPx)
		if px := got.Get("px").Num(); px < 12 {
			t.Fatalf("a lane label is %.1f px on screen at %s (zoom %.2f); want at least 12", px, step, got.Get("zoom").Num())
		}
	}
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	infraPanelsCenter(page, `[data-lane="network"]`)

	const wantAuto = `{"inet":"-120px,96px","ont":"120px,96px","router":"0px,480px","switch":"-240px,480px","ap":"240px,480px","nas":"-120px,864px","pve":"120px,864px","tv":"-120px,1248px","laptop":"120px,1248px"}`
	if got := page.MustEval(`()=>JSON.stringify(Object.fromEntries(['inet','ont','router','switch','ap','nas','pve','tv','laptop'].map(id=>{const card=document.querySelector('[data-node="'+id+'"]');return [id,card?card.style.left+','+card.style.top:'missing'];})))`).Str(); got != wantAuto {
		t.Fatalf("topology auto layout = %s; want %s", got, wantAuto)
	}
	if got := page.MustEval(`()=>{const lane=document.querySelector('[data-lane="network"]').getBoundingClientRect(),card=document.querySelector('[data-node="router"]').getBoundingClientRect();return JSON.stringify({guest:!!document.querySelector('[data-node="vm1"]'),orphan:!!document.querySelector('[data-node="ct-orphan"]'),place:!!document.querySelector('[data-node="room"]')||document.querySelectorAll('[data-region]').length>0,stack:!!document.querySelector('[data-toggle-host-services="pve"]'),toPlace:document.querySelectorAll('[data-section-list="to-place"] [data-library-item]').length,inLane:card.top>=lane.top&&card.bottom<=lane.bottom});}`).Str(); got != `{"guest":false,"orphan":false,"place":false,"stack":true,"toPlace":0,"inLane":true}` {
		t.Fatalf("topology must nest guests, hide places and hostless containers, and leave nothing to place: %s", got)
	}
	if n := len(layoutCalls()); n != 0 {
		t.Fatalf("opening Topology saved %d layouts; the auto layout must not write", n)
	}

	// Dragging a card into another lane saves a pinned topology placement.
	tx, ty := infraPanelsCenter(page, `[data-node="tv"]`)
	_, ly := infraPanelsCenter(page, `[data-lane="network"]`)
	page.Mouse.MustMoveTo(tx, ty).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo(tx, (ty+ly)/2)
	page.Mouse.MustMoveTo(tx, ly)
	page.MustWait(`()=>document.querySelector('[data-lane="network"]').classList.contains('is-drop-target')`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	calls := waitCalls(1)
	tv := item(calls[0], "tv")
	if calls[0]["view"] != "topology" || tv == nil || tv["pinned"] != true {
		t.Fatalf("dragging into another lane must save a pinned topology placement: %#v", calls[0])
	}
	droppedY, _ := tv["y"].(float64)
	if droppedY < 384 || droppedY >= 768 || int(droppedY)%24 != 0 {
		t.Fatalf("the card landed at y=%v; want a grid point inside the network lane (384-767)", droppedY)
	}
	page.MustWait(`()=>!document.querySelector('[data-lane].is-drop-target')`)

	// Arrange without a selection: one save, the dragged card keeps its lane.
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]')?.click()`)
	page.MustWait(`()=>!document.querySelector('[data-node][aria-pressed="true"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="more"]').click()`)
	page.MustWait(`()=>{const item=document.querySelector('[data-menu="more"] [data-action="arrange"]');return !!item&&!item.disabled&&item.offsetParent!==null;}`)
	page.MustEval(`()=>document.querySelector('[data-menu="more"] [data-action="arrange"]').click()`)
	calls = waitCalls(2)
	arranged := calls[1]
	if nodes, _ := arranged["nodes"].([]map[string]any); len(nodes) != 9 {
		t.Fatalf("Arrange without a selection must save all nine cards in one call: %#v", arranged)
	}
	for id, want := range map[string][3]any{
		"tv":     {float64(120), float64(480), true},
		"router": {float64(360), float64(480), false},
		"inet":   {float64(-120), float64(96), false},
		"laptop": {float64(0), float64(1248), false},
	} {
		got := item(arranged, id)
		if got == nil || got["x"] != want[0] || got["y"] != want[1] || got["pinned"] != want[2] {
			t.Fatalf("Arrange placed %s at %#v; want x=%v y=%v pinned=%v", id, got, want[0], want[1], want[2])
		}
	}

	// Undo hands the auto cards back to the automatic layout.
	page.MustEval(`()=>document.querySelector('[data-action="undo"]').click()`)
	calls = waitCalls(3)
	undone := calls[2]
	if got := item(undone, "router"); got == nil || got["unplace"] != true {
		t.Fatalf("undoing Arrange must unplace the cards Infra laid out before: %#v", undone)
	}
	if got := item(undone, "tv"); got == nil || got["y"] != droppedY || got["pinned"] != true {
		t.Fatalf("undoing Arrange must put the dragged card back: %#v", undone)
	}
	page.MustWait(`()=>{const card=document.querySelector('[data-node="router"]');return card.style.left==='0px'&&card.style.top==='480px';}`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
