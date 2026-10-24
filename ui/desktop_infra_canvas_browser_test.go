package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func infraCanvasFixtureNode(id, kind, name, icon string, placement map[string]any, parent string) map[string]any {
	node := map[string]any{"id": id, "version": 1, "kind": kind, "name": name, "icon": icon,
		"profile":    map[string]any{"tags": []any{}, "interfaces": []any{}, "links": []any{}, "app_bindings": []any{}, "custom": map[string]any{}},
		"placements": map[string]any{}, "sources": []any{}, "available_actions": []any{}}
	if placement != nil {
		node["placements"] = map[string]any{"atlas": placement}
	}
	if parent != "" {
		node["parent_id"] = parent
	}
	return node
}

// TestDesktopInfraCanvasGesturesBrowser covers the canvas wiring: the map owns
// the window, a library row dropped into a region saves parent and position in
// one call, a card dragged out clears its parent, undo/redo replay the move,
// connecting by handle drag creates an edge, a layout conflict is rebased
// once, and keyboard move mode saves one move.
func TestDesktopInfraCanvasGesturesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 400, "y": 120, "w": 480, "h": 320, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-in-room", "device", "Media box", "mini-pc", map[string]any{"x": 640, "y": 300, "pinned": true}, "room-1"),
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-b", "device", "Core switch", "managed-switch", map[string]any{"x": 160, "y": 420, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-unplaced", "device", "Cold NAS", "nas", nil, ""),
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')&&!!document.querySelector('[data-library-item="dev-unplaced"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	// Positions are read in screen pixels: let animated pans, window resizes and the inspector opening settle first.
	settleScene := func() {
		page.MustWait(`()=>!document.querySelector('[data-scene]').classList.contains('is-animating')`)
		page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	}
	settleScene()

	center := func(selector string) (float64, float64) {
		settleScene()
		point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, selector).Arr()
		return point[0].Num(), point[1].Num()
	}
	drag := func(fromX, fromY, toX, toY float64) {
		page.Mouse.MustMoveTo(fromX, fromY).MustDown(proto.InputMouseButtonLeft)
		page.Mouse.MustMoveTo((fromX+toX)/2, (fromY+toY)/2)
		page.Mouse.MustMoveTo(toX, toY)
		page.Mouse.MustUp(proto.InputMouseButtonLeft)
	}
	layoutCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.layoutCalls...)
	}
	waitLayoutCalls := func(want int) []map[string]any {
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
	itemFor := func(call map[string]any, id string) map[string]any {
		for _, item := range call["nodes"].([]map[string]any) {
			if item["id"] == id {
				return item
			}
		}
		return nil
	}
	snapshotRequests := func() int {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		count := 0
		for _, path := range fixture.requests {
			if path == "/api/desktop/infra/snapshot" {
				count++
			}
		}
		return count
	}

	// The map owns the window.
	if !page.MustEval(`()=>{const app=document.querySelector('.infra-app').getBoundingClientRect(),canvas=document.querySelector('[data-canvas]').getBoundingClientRect();return canvas.height>=app.height*0.75;}`).Bool() {
		t.Fatal("the canvas must own at least 75 % of the window height")
	}

	// A library row dropped into a region saves parent and position in one call.
	start := len(layoutCalls())
	rx, ry := center(`[data-library-item="dev-unplaced"] .infra-library-row-name`)
	gx, gy := center(`[data-region="room-1"]`)
	drag(rx, ry, gx, gy+40)
	calls := waitLayoutCalls(start + 1)[start:]
	if len(calls) != 1 {
		t.Fatalf("placing into a region made %d layout calls; want one", len(calls))
	}
	if item := itemFor(calls[0], "dev-unplaced"); item == nil || item["parent_id"] != "room-1" {
		t.Fatalf("the drop did not save the region as parent: %#v", calls[0])
	}
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-unplaced"]')&&!document.querySelector('[data-place="dev-unplaced"]')`)

	// A card dragged out of every region clears its parent.
	start = len(layoutCalls())
	cx, cy := center(`[data-node="dev-in-room"]`)
	drag(cx, cy, cx+320, cy)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "" {
		t.Fatalf("dragging out of the region did not clear the parent: %#v", calls[0])
	}

	// Undo restores position and parent; redo replays the move.
	start = len(layoutCalls())
	page.MustEval(`()=>document.querySelector('[data-action="undo"]').click()`)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "room-1" || item["x"] != float64(640) {
		t.Fatalf("undo did not restore the saved position and parent: %#v", calls[0])
	}
	page.MustEval(`()=>document.querySelector('[data-action="redo"]').click()`)
	calls = waitLayoutCalls(start + 2)[start+1:]
	if item := itemFor(calls[0], "dev-in-room"); item == nil || item["parent_id"] != "" {
		t.Fatalf("redo did not replay the move: %#v", calls[0])
	}

	// Dragging a connection handle onto another card offers the connection popover.
	ax, ay := center(`[data-node="dev-a"]`)
	page.Mouse.MustMoveTo(ax, ay)
	hx, hy := center(`[data-node="dev-a"] [data-connect-handle="s"]`)
	bx, by := center(`[data-node="dev-b"]`)
	drag(hx, hy, bx, by)
	page.MustWait(`()=>!!document.querySelector('[data-connect-popover]')`)
	if kind := page.MustEval(`()=>document.querySelector('[data-connect-popover] [data-connect-kind]').value`).Str(); kind != "ethernet" {
		t.Fatalf("router to switch should default to ethernet, got %q", kind)
	}
	page.MustEval(`()=>document.querySelector('[data-connect-popover] [data-action="create-connection"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-connect-popover]')&&!!document.querySelector('[data-edges] [data-edge="edge-1"]')`)
	fixture.mu.Lock()
	edges := append([]map[string]any(nil), fixture.edges...)
	fixture.mu.Unlock()
	if len(edges) != 1 || edges[0]["from"] != "dev-a" || edges[0]["to"] != "dev-b" || edges[0]["kind"] != "ethernet" {
		t.Fatalf("connect gesture saved %#v", edges)
	}

	// A layout conflict reloads once and resends onto the fresh version.
	fixture.mu.Lock()
	fixture.fail409Once = true
	fixture.mu.Unlock()
	snapshotsBefore := snapshotRequests()
	start = len(layoutCalls())
	ax, ay = center(`[data-node="dev-a"]`)
	drag(ax, ay, ax, ay-96)
	calls = waitLayoutCalls(start + 2)[start:]
	if len(calls) != 2 {
		t.Fatalf("a conflict produced %d layout calls; want the original and one rebased resend", len(calls))
	}
	first, second := itemFor(calls[0], "dev-a"), itemFor(calls[1], "dev-a")
	if first == nil || second == nil || second["version"].(float64) != first["version"].(float64)+5 || second["y"] != first["y"] {
		t.Fatalf("the resend must reapply the same intent onto the fresh version: %#v", calls)
	}
	if snapshotRequests() <= snapshotsBefore {
		t.Fatal("the conflict did not reload the snapshot before resending")
	}
	page.MustWait(`()=>document.querySelector('[data-layout-chip]').hidden`)

	// Keyboard move mode: M, two steps right, Enter saves one move.
	page.MustEval(`()=>{const card=document.querySelector('[data-node="dev-b"]');card.click();card.focus();}`)
	start = len(layoutCalls())
	page.Keyboard.MustType(input.KeyM, input.ArrowRight, input.ArrowRight, input.Enter)
	calls = waitLayoutCalls(start + 1)[start:]
	if item := itemFor(calls[0], "dev-b"); len(calls) != 1 || item == nil || item["x"] != float64(208) || item["y"] != float64(420) {
		t.Fatalf("keyboard move mode saved %s", fmt.Sprint(calls))
	}

	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
