package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopInfraArrangeAndGroupBrowser covers the Map's automatic arrangement from the empty-map
// context menu (one layout save without overlapping cards) and grouping the selection with Ctrl+G
// (a coloured group is created, both assets are parented to it in one layout save).
func TestDesktopInfraArrangeAndGroupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	pos := func(x, y int) map[string]any { return map[string]any{"x": x, "y": y, "pinned": true} }
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", pos(300, 200), ""),
		infraCanvasFixtureNode("dev-b", "device", "Core switch", "managed-switch", pos(310, 210), ""),
		infraCanvasFixtureNode("dev-c", "device", "Media box", "mini-pc", pos(320, 220), ""),
	}
	fixture.edges = []map[string]any{
		{"id": "e1", "version": 1, "from": "dev-a", "to": "dev-b", "kind": "ethernet"},
		{"id": "e2", "version": 1, "from": "dev-b", "to": "dev-c", "kind": "ethernet"},
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1600, 1000, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')&&!!document.querySelector('[data-node="dev-c"]')`)

	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	waitLayoutCalls := func(want int) []map[string]any {
		deadline := time.Now().Add(8 * time.Second)
		for {
			if calls := layoutCalls(); len(calls) >= want {
				return calls
			}
			if time.Now().After(deadline) {
				t.Fatalf("waited for %d layout saves, got %d", want, len(layoutCalls()))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Empty map menu -> Arrange automatically -> By connections.
	page.MustEval(`()=>{
		const stage=document.querySelector('[data-stage]'),r=stage.getBoundingClientRect();
		stage.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.right-90,clientY:r.top+140}));
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-context-submenu')`)
	page.MustEval(`()=>{
		// The browser runs in German: the third item of the "Arrange automatically" submenu list is found by its position.
		const item=document.querySelector('.vd-context-submenu-popover [data-context-action]');
		if(!item||item.textContent.trim()!=='Nach Verbindungen') throw new Error('first arrange item is '+item?.textContent);
		item.click();
	}`)
	calls := waitLayoutCalls(1)
	nodes := calls[0]["nodes"].([]map[string]any)
	if len(nodes) != 3 {
		t.Fatalf("arranging saved %d cards; want 3: %#v", len(nodes), nodes)
	}
	for i := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			dx := infraFixtureNumber(nodes[i]["x"]) - infraFixtureNumber(nodes[j]["x"])
			dy := infraFixtureNumber(nodes[i]["y"]) - infraFixtureNumber(nodes[j]["y"])
			if (dx > -216 && dx < 216) && (dy > -64 && dy < 64) {
				t.Fatalf("arranged cards overlap: %#v", nodes)
			}
		}
	}

	// Ctrl+G groups the selection in a coloured group.
	// 2.15: Arrange names its scope: the whole map with nothing selected, the selection when several cards of one place are.
	arrangeLabel := `()=>document.querySelector('[data-action="arrange"]').textContent`
	if got, want := page.MustEval(arrangeLabel).Str(), page.MustEval(`()=>window.fixtureWords['desktop.infra_arrange_map']`).Str(); got != want {
		t.Fatalf("with nothing selected Arrange reads %q; want %q", got, want)
	}
	page.MustEval(`()=>document.querySelector('[data-node="dev-a"]').click()`)
	page.KeyActions().Press(input.ControlLeft).Type(input.KeyA).Release(input.ControlLeft).MustDo()
	page.MustWait(`()=>document.querySelector('[data-action="arrange"]').textContent===window.fixtureWords['desktop.infra_arrange_selection']`)
	page.KeyActions().Press(input.ControlLeft).Type(input.KeyG).Release(input.ControlLeft).MustDo()
	calls = waitLayoutCalls(2)
	fixture.mu.Lock()
	var group map[string]any
	for _, node := range fixture.nodes {
		if node["kind"] == "group" {
			group = node
		}
	}
	fixture.mu.Unlock()
	if group == nil {
		t.Fatal("Ctrl+G did not create a group node")
	}
	if color, _ := group["profile"].(map[string]any)["color"].(string); color == "" || color == "slate" {
		t.Fatalf("the new group has no palette colour: %#v", group["profile"])
	}
	parented := 0
	for _, item := range calls[1]["nodes"].([]map[string]any) {
		if item["parent_id"] == group["id"] && item["id"] != group["id"] {
			parented++
		}
	}
	if parented != 3 {
		t.Fatalf("the grouping layout save parented %d assets to the group; want 3: %#v", parented, calls[1]["nodes"])
	}
	page.MustWait(`()=>!!document.querySelector('.infra-region[data-color]')`)
}
