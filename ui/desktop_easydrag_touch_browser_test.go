package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// easyDragTouchViewports are the two sizes every touch gesture runs at: a small tablet and a phone.
var easyDragTouchViewports = []struct {
	name string
	w, h int
}{
	{"1024x768", 1024, 768},
	{"390x844", 390, 844},
}

// touchPoint is a point in page (client) coordinates, in CSS pixels.
type touchPoint struct{ x, y float64 }

// startEasyDragTouch opens the seeded flow of the main editor fixture in a w×h touch device: a mobile
// viewport with 5 touch points and a coarse pointer. The flow is opened by a tap.
func startEasyDragTouch(t *testing.T, w, h int) *easyDragSmoke {
	t.Helper()
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Width: w, Height: h})
	page := s.page
	points := 5
	if err := (proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h, DeviceScaleFactor: 1, Mobile: true}).Call(page); err != nil {
		t.Fatalf("device metrics %dx%d: %v", w, h, err)
	}
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &points}).Call(page); err != nil {
		t.Fatalf("touch emulation: %v", err)
	}
	// The shell and the editor choose their layout when the window opens: reopen it as a phone does.
	page.MustEval(`()=>fixtureCloseAll()`)
	s.wait(`()=>!document.querySelector('.ed-app')`)
	page.MustEval(`async()=>{await fixtureOpen('easydrag')}`)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	if !page.MustEval(`()=>matchMedia('(pointer: coarse)').matches && navigator.maxTouchPoints===5`).Bool() {
		t.Fatalf("touch emulation is on, but the page reports no coarse pointer with 5 touch points")
	}
	// The start page is a scroller: bring the card into view first (a phone lists templates below).
	s.touchScrollTo(`.ed-flow-card h3`)
	s.touchTapElement(`.ed-flow-card h3`)
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.touchStable()
	return s
}

// touchStable waits until the canvas box and the view have not changed for 400 ms: the editor eases the
// view into place and resizes the canvas after the window opens, and a finger aimed at an empty point
// must not meet a card that moved in.
func (s *easyDragSmoke) touchStable() {
	s.t.Helper()
	s.settle()
	prev := ""
	same := 0
	for i := 0; i < 60 && same < 4; i++ {
		cur := s.page.MustEval(`()=>{const r=document.querySelector('.ed-canvas').getBoundingClientRect(),v=edFixture.editor().view;return [r.left,r.top,r.width,r.height,v.x,v.y,v.zoom].map(n=>Math.round(n*10)/10).join()}`).Str()
		if cur == prev {
			same++
		} else {
			same = 0
		}
		prev = cur
		time.Sleep(100 * time.Millisecond)
	}
	if same < 4 {
		s.t.Fatalf("the canvas did not come to rest: %s", prev)
	}
}

// touchSend dispatches one Input.dispatchTouchEvent. The list is never nil: touchEnd carries an
// empty array, and a nil slice would be sent as null, which Chrome rejects.
func (s *easyDragSmoke) touchSend(kind proto.InputDispatchTouchEventType, points ...touchPoint) {
	s.t.Helper()
	list := make([]*proto.InputTouchPoint, 0, len(points))
	for _, p := range points {
		list = append(list, &proto.InputTouchPoint{X: p.x, Y: p.y})
	}
	if err := (proto.InputDispatchTouchEvent{Type: kind, TouchPoints: list}).Call(s.page); err != nil {
		s.t.Fatalf("touch %s at %v: %v", kind, points, err)
	}
}

// touchTap puts a finger down at (x, y) and lifts it 50 ms later.
func (s *easyDragSmoke) touchTap(x, y float64) {
	s.t.Helper()
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, touchPoint{x, y})
	time.Sleep(50 * time.Millisecond)
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchLongPress holds a finger at (x, y) for ms milliseconds.
func (s *easyDragSmoke) touchLongPress(x, y float64, ms int) {
	s.t.Helper()
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, touchPoint{x, y})
	time.Sleep(time.Duration(ms) * time.Millisecond)
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchDrag moves one finger from one point to another in the given number of equal moves, 16 ms apart.
func (s *easyDragSmoke) touchDrag(from, to touchPoint, steps int) {
	s.t.Helper()
	if steps < 1 {
		steps = 1
	}
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, from)
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		s.touchSend(proto.InputDispatchTouchEventTypeTouchMove, touchPoint{from.x + (to.x-from.x)*f, from.y + (to.y-from.y)*f})
		time.Sleep(16 * time.Millisecond)
	}
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchCentre returns the centre of the part of the first element matching selector that lies in the
// viewport. It does not scroll: a canvas card scrolled into view would move the overflow:hidden canvas.
func (s *easyDragSmoke) touchCentre(selector string) touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`sel=>{
		const el=document.querySelector(sel);
		if(!el)throw Error('no element matches '+sel);
		const r=el.getBoundingClientRect();
		const l=Math.max(r.left,0),rt=Math.min(r.right,innerWidth),tp=Math.max(r.top,0),b=Math.min(r.bottom,innerHeight);
		if(rt-l<2||b-tp<2)throw Error(sel+' is outside the viewport: '+JSON.stringify(r));
		return [(l+rt)/2,(tp+b)/2];
	}`, selector)
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// touchScrollTo scrolls an element of a panel (a detail column, a list) into the middle of its
// scroller. Never use it on canvas cards.
func (s *easyDragSmoke) touchScrollTo(selector string) {
	s.t.Helper()
	s.page.MustEval(`sel=>{const el=document.querySelector(sel);if(!el)throw Error('no element matches '+sel);el.scrollIntoView({block:'center',inline:'nearest'});}`, selector)
}

// touchAssertHit fails when the element at p is not selector or one of its descendants: a finger would
// press something else.
func (s *easyDragSmoke) touchAssertHit(selector string, p touchPoint) {
	s.t.Helper()
	got := s.page.MustEval(`(sel,x,y)=>{
		const e=document.elementFromPoint(x,y),t=document.querySelector(sel);
		if(e&&t&&t.contains(e))return '';
		return e?(e.tagName+'.'+String(e.getAttribute('class')||'')):'nothing';
	}`, selector, p.x, p.y).Str()
	if got != "" {
		s.t.Fatalf("a finger at (%.0f, %.0f) would press %s, not %s", p.x, p.y, got, selector)
	}
}

// touchTapElement taps the visible centre of the first element matching selector, after checking that
// the finger would hit it. It returns the point.
func (s *easyDragSmoke) touchTapElement(selector string) touchPoint {
	s.t.Helper()
	p := s.touchCentre(selector)
	s.touchAssertHit(selector, p)
	s.touchTap(p.x, p.y)
	return p
}

// touchEdgePoint returns a point on the wire's hit stroke that a finger can reach: the first of the
// middle fractions of the path at which the stroke, and not a port button or a card, is on top.
func (s *easyDragSmoke) touchEdgePoint(edgeID string) touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`id=>{
		const hit=document.querySelector('.ed-edge[data-edge-id="'+id+'"] .ed-wire-hit');
		if(!hit)throw Error('no wire '+id);
		const len=hit.getTotalLength(),m=hit.getScreenCTM();
		for(const f of [0.5,0.4,0.6,0.3,0.7]){
			const p=hit.getPointAtLength(len*f),s=new DOMPoint(p.x,p.y).matrixTransform(m);
			if(document.elementFromPoint(s.x,s.y)===hit)return [s.x,s.y,f];
		}
		throw Error('no point of wire '+id+' is on screen and on top');
	}`, edgeID)
	if f := v.Get("2").Num(); f != 0.5 {
		s.t.Logf("wire %s: the middle is covered or off screen; using the point at %.0f%% of its length", edgeID, f*100)
	}
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// touchEmptyPoint returns a point of the canvas with no card, wire, control or group on it. It keeps
// 80 px from the canvas edges, where the zoom bar, the minimap and the scroll corners sit, and tries
// the middle of the canvas first.
func (s *easyDragSmoke) touchEmptyPoint() touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`()=>{
		const c=document.querySelector('.ed-canvas').getBoundingClientRect();
		const inset=80,w=c.width-2*inset,h=c.height-2*inset;
		if(w<20||h<20)throw Error('the canvas is too small for an empty point: '+JSON.stringify(c));
		const rows=[0.5,0.7,0.3,0.9,0.1,1,0],cols=[0.5,0.7,0.3,0.9,0.1,1,0];
		for(const ry of rows)for(const rx of cols){
			const x=c.left+inset+w*rx,y=c.top+inset+h*ry;
			const e=document.elementFromPoint(x,y);
			if(e&&e.closest('.ed-canvas')&&!e.closest('.ed-node,.ed-edge,.ed-zoom,.ed-minimap,.ed-empty,.ed-wire-actions,.ed-port-tip,.ed-group-frame,.ed-quick'))return [x,y];
		}
		throw Error('no empty point on the canvas');
	}`)
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// TestEasyDragTouchHelper proves the helper itself at both viewports: the page is a touch device, a
// tap is a touch pointer, and a finger dragged over empty canvas pans the view.
func TestEasyDragTouchHelper(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			s.page.MustEval(`()=>{window.__touchTypes=[];document.addEventListener('pointerdown',e=>window.__touchTypes.push(e.pointerType),true);}`)
			from := s.touchEmptyPoint()
			x0 := s.page.MustEval(`()=>edFixture.editor().view.x`).Num()
			s.touchDrag(from, touchPoint{from.x + 60, from.y}, 6)
			s.wait(fmt.Sprintf(`()=>Math.abs(edFixture.editor().view.x-(%f)-60)<4`, x0))
			s.wait(`()=>window.__touchTypes.length===1 && window.__touchTypes[0]==='touch'`)
			s.failOnPageErrors("touch helper")
		})
	}
}

// TestEasyDragTouchInsertOnWire taps the wire between the trigger and the first step: the wire is
// selected, its actions stay open for the next tap, Insert opens the quick-add list, and the step
// chosen there lands between the two cards.
func TestEasyDragTouchInsertOnWire(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			p := s.touchEdgePoint("e_1")
			s.touchTap(p.x, p.y)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');return !!a && !a.hidden && edFixture.editor().selectedEdge==='e_1' && !!document.querySelector('.ed-edge.is-selected[data-edge-id="e_1"]')}`)
			// A finger has no hover to keep the actions open: they must outlive the 220 ms hide delay.
			time.Sleep(450 * time.Millisecond)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');const b=a && a.querySelector('[data-ed-wire="insert"]');if(!a||a.hidden||!b)return false;const r=b.getBoundingClientRect();return r.width>=24 && r.height>=24}`)
			s.shot("touch-wire-actions-" + vp.name)
			s.touchTapElement(`.ed-wire-actions [data-ed-wire="insert"]`)
			s.wait(`()=>!!document.querySelector('.ed-quick [data-ed-index="0"]') && document.querySelector('.ed-wire-actions').hidden`)
			s.touchTapElement(`.ed-quick [data-ed-index="0"]`)
			s.wait(`()=>{const m=edFixture.editor().model;if(m.doc.nodes.length!==5||m.doc.edges.length!==4)return false;const added=m.doc.nodes.find(n=>!['n_zeitplan','n_suche','n_ki','n_pdf'].includes(n.id));return !!added && m.incoming(added.id).map(e=>e.source.node).join()==='n_zeitplan' && m.incoming('n_suche').map(e=>e.source.node).join()===added.id && !document.querySelector('.ed-quick')}`)
			s.shot("touch-wire-inserted-" + vp.name)
			page.MustEval(`()=>edFixture.editor().model.undo()`)
			s.wait(`()=>edFixture.editor().model.doc.nodes.length===4 && edFixture.editor().model.doc.edges.length===3`)
			s.failOnPageErrors("touch insert on wire")
		})
	}
}

// TestEasyDragTouchDeleteWire taps a wire and then its Delete button: the wire is gone, its
// neighbours stay, the actions leave, and undo brings the wire back.
func TestEasyDragTouchDeleteWire(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			p := s.touchEdgePoint("e_1")
			s.touchTap(p.x, p.y)
			time.Sleep(450 * time.Millisecond)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');const b=a && a.querySelector('[data-ed-wire="delete"]');if(!a||a.hidden||!b)return false;const r=b.getBoundingClientRect();return r.width>=24 && r.height>=24 && edFixture.editor().selectedEdge==='e_1'}`)
			s.touchTapElement(`.ed-wire-actions [data-ed-wire="delete"]`)
			s.wait(`()=>{const m=edFixture.editor().model;return m.doc.edges.length===2 && !m.edge('e_1') && m.doc.nodes.length===4 && document.querySelector('.ed-wire-actions').hidden && !document.querySelector('.ed-edge[data-edge-id="e_1"]')}`)
			s.shot("touch-wire-deleted-" + vp.name)
			s.clearToasts()
			page.MustEval(`()=>edFixture.editor().model.undo()`)
			s.wait(`()=>edFixture.editor().model.doc.edges.length===3 && !!edFixture.editor().model.edge('e_1')`)
			s.failOnPageErrors("touch delete wire")
		})
	}
}
