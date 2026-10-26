package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopInfraConnectionStatesBrowser covers amendments A29 and A30 (design QA batch D-fix-3):
//   - in Topology a power or dependency connection is drawn only while one of its ends is selected, hovered or focused
//     with the keyboard (the Map always draws it), and it takes no hit area while it is hidden;
//   - a selection emphasises the connections that touch it (accent stroke, last in the layer) and fades the others to
//     35 %, a hover only reveals;
//   - a connection with a dimmed (filtered out) end is dimmed like the cards.
//
// The fixture is the topology one (see infraTopologyFixture) plus a power connection from the laptop to the Proxmox
// host (e10); e9 is the dependency from the TV to the NAS.
func TestDesktopInfraConnectionStatesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := infraTopologyFixture()
	fixture.edges = append(fixture.edges, map[string]any{"id": "e10", "version": 1, "kind": "powered_by", "from": "laptop", "to": "pve"})
	page := infraPanelsPage(t, fixture)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraAccent=()=>{const probe=document.createElement('i');probe.style.color='var(--vd-accent)';document.querySelector('.infra-app').appendChild(probe);const c=getComputedStyle(probe).color;probe.remove();return c;};
		// off lists the hidden connections, linked the emphasised ones, faded and dimmed the others (each sorted by id)
		window.infraEdges=()=>{
			const out={off:[],linked:[],faded:[],dimmed:[]};
			for(const g of document.querySelectorAll('[data-edge-layer] .infra-edge'))for(const key of Object.keys(out))if(g.classList.contains('is-'+key))out[key].push(g.dataset.edge);
			for(const key of Object.keys(out))out[key].sort();
			return JSON.stringify(out);
		};
		window.infraOpacity=id=>Number(getComputedStyle(document.querySelector('[data-edge="'+id+'"]')).opacity);
		window.infraStroke=id=>getComputedStyle(document.querySelector('[data-edge="'+id+'"] .infra-edge-line')).stroke;
	}`)
	waitEdges := func(step, want string) {
		t.Helper()
		got := ""
		for deadline := time.Now().Add(6 * time.Second); time.Now().Before(deadline); time.Sleep(60 * time.Millisecond) {
			if got = page.MustEval(`()=>infraEdges()`).Str(); got == want {
				return
			}
		}
		t.Fatalf("%s: %s; want %s; active %s", step, got, want, page.MustEval(`()=>{const r=document.querySelector('[data-stage]').getBoundingClientRect(),el=document.elementFromPoint(r.left+30,r.top+30);return (document.activeElement.outerHTML||'').slice(0,100)+' | under pointer '+(el&&el.outerHTML||'').slice(0,100)+' | hover '+[...document.querySelectorAll(':hover')].map(e=>e.className||e.tagName).join(',');}`).Str())
	}
	// away moves the pointer to the empty top left corner of the map; escape clears the selection from the keyboard and lets go of
	// the card the inspector hands the focus back to (a focused card would keep its connections shown).
	away := func() {
		box := page.MustEval(`()=>{const r=document.querySelector('[data-stage]').getBoundingClientRect();return [r.left+30,r.top+30];}`).Arr()
		page.Mouse.MustMoveTo(box[0].Num(), box[1].Num())
	}
	escape := func() {
		page.MustEval(`()=>document.querySelector('[data-stage]').focus()`)
		page.Keyboard.MustType(input.Escape)
		page.MustWait(`()=>!document.querySelector('[data-node][aria-pressed="true"]')`)
		infraPolishSettle(page)
		page.MustEval(`()=>document.activeElement&&document.activeElement.blur()`)
	}
	const (
		quiet    = `{"off":["e10","e9"],"linked":[],"faded":[],"dimmed":[]}`
		nothing  = `{"off":[],"linked":[],"faded":[],"dimmed":[]}`
		nasPicks = `{"off":["e10"],"linked":["e5","e9"],"faded":["e1","e2","e3","e4","e6","e7","e8"],"dimmed":[]}`
	)

	// The Map draws every kind at all times.
	waitEdges("Map", nothing)

	page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-lanes] [data-lane]').length===4&&!!document.querySelector('[data-node="tv"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	infraPolishSettle(page)

	// A29: Topology hides the power and dependency connections until an end is selected or hovered; Ethernet, fiber and Wi-Fi stay.
	waitEdges("Topology without a selection", quiet)
	page.MustWait(`()=>infraOpacity('e9')===0&&infraOpacity('e10')===0&&infraOpacity('e1')===1`)
	if !page.MustEval(`()=>['e9','e10'].every(id=>getComputedStyle(document.querySelector('[data-edge="'+id+'"] .infra-edge-hit')).pointerEvents==='none')`).Bool() {
		t.Fatal("a hidden connection must not take the pointer")
	}

	// A30: selecting the NAS (an end of e5 and e9) shows its dependency, emphasises both and fades the rest; e10 stays hidden.
	page.MustEval(`()=>document.querySelector('[data-node="nas"]').click()`)
	waitEdges("NAS selected", nasPicks)
	page.MustWait(`()=>Math.abs(infraOpacity('e1')-0.35)<0.01&&infraOpacity('e5')===1&&infraOpacity('e9')===1`)
	if got := page.MustEval(`()=>{
		const accent=infraAccent(),layer=[...document.querySelector('[data-edge-layer]').children].map(g=>g.dataset.edge);
		return JSON.stringify({e5:infraStroke('e5')===accent,e9:infraStroke('e9')===accent,e1:infraStroke('e1')===accent,last:layer.slice(-2).sort()});
	}`).Str(); got != `{"e5":true,"e9":true,"e1":false,"last":["e5","e9"]}` {
		t.Fatalf("the connections of the selected card must use the accent stroke and sit last in the layer: %s", got)
	}

	// Hovering a card reveals its hidden connections, but only a selection emphasises: the laptop's power line shows, faded like the rest.
	x, y := infraPanelsCenter(page, `[data-node="laptop"]`)
	page.Mouse.MustMoveTo(x, y)
	waitEdges("laptop hovered while the NAS is selected", `{"off":[],"linked":["e5","e9"],"faded":["e1","e10","e2","e3","e4","e6","e7","e8"],"dimmed":[]}`)
	away()
	waitEdges("pointer left", nasPicks)
	escape()
	waitEdges("selection cleared", quiet)

	// Without a selection a hover reveals and emphasises nothing.
	x, y = infraPanelsCenter(page, `[data-node="laptop"]`)
	page.Mouse.MustMoveTo(x, y)
	waitEdges("laptop hovered", `{"off":["e9"],"linked":[],"faded":[],"dimmed":[]}`)
	away()
	waitEdges("pointer left again", quiet)

	// Keyboard focus counts like a hover (Escape was a key press, so script focus is keyboard focus).
	page.MustEval(`()=>document.querySelector('[data-node="pve"]').focus()`)
	waitEdges("pve focused", `{"off":["e9"],"linked":[],"faded":[],"dimmed":[]}`)
	page.MustEval(`()=>document.querySelector('[data-node="pve"]').blur()`)
	waitEdges("focus left", quiet)

	// A selected connection is emphasised like a selected card and the others fade. e5 passes through free space at its midpoint.
	hit := page.MustEval(`()=>{const hit=document.querySelector('[data-edge="e5"] .infra-edge-hit'),at=hit.getPointAtLength(hit.getTotalLength()/2),m=hit.getScreenCTM();return [m.a*at.x+m.c*at.y+m.e,m.b*at.x+m.d*at.y+m.f];}`).Arr()
	page.Mouse.MustMoveTo(hit[0].Num(), hit[1].Num()).MustClick(proto.InputMouseButtonLeft)
	waitEdges("connection selected", `{"off":["e10","e9"],"linked":["e5"],"faded":["e1","e2","e3","e4","e6","e7","e8"],"dimmed":[]}`)
	away()
	escape()
	waitEdges("connection deselected", quiet)

	// A30: a filter dims the connections that have a dimmed end, hidden ones included. "a" keeps the NAS, the access point, the laptop
	// and (through its guest "Lab VM") the Proxmox host; e7 and e10 join two of them.
	const filtered = `"dimmed":["e1","e2","e3","e4","e5","e6","e8","e9"]`
	page.MustEval(`()=>{const el=document.querySelector('[data-search]');el.value='a';el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	waitEdges("filtered", `{"off":["e10","e9"],"linked":[],"faded":[],`+filtered+`}`)
	page.MustWait(`()=>Math.abs(infraOpacity('e1')-0.28)<0.01&&infraOpacity('e7')===1`)
	// The dimmed state does not depend on the view: the Map dims the same connections and hides none.
	page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-lanes]').hidden`)
	waitEdges("filtered in the Map", `{"off":[],"linked":[],"faded":[],`+filtered+`}`)
	requireInfraBrowserClean(t, page)
}
