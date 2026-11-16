package ui

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// openFirstFlow opens the standard fixture's one flow in the editor and waits for its canvas.
func (s *easyDragSmoke) openFirstFlow() {
	s.t.Helper()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
}

// startOwnTest starts a real test of the open flow, each step taking delay milliseconds, and waits until the header
// shows the run (the Stop). The flow has effects, so Test opens in Simulation: the dialog is set to a real test.
func (s *easyDragSmoke) startOwnTest(delay int) {
	s.t.Helper()
	s.page.MustEval(fmt.Sprintf(`()=>{edFixture.state.delay=%d;}`, delay))
	s.page.MustEval(`()=>window.EasyDrag.actions.run('flow.test',{host:edFixture.editor()})`)
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	s.page.MustEval(`()=>{const m=document.querySelector('.ed-modal [data-ed-test-mode]');if(m){m.value='real';m.dispatchEvent(new Event('change',{bubbles:true}));}}`)
	s.page.MustEval(`()=>document.querySelector('.ed-modal [data-ed-action="run"]').click()`)
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && !!edFixture.editor().run && !edFixture.editor().run.view`)
	s.waitStop()
}

// waitNoStop waits until no Stop can be seen: the run ended or was stopped.
func (s *easyDragSmoke) waitNoStop() {
	s.t.Helper()
	s.wait(`()=>{` + easyDragVisibleJS + `return !vis(document.querySelector('[data-ed-cmd="cancel"]'));}`)
}

// showWaitingRun opens a new live run that waits for a slot in the run view and waits for its Stop.
func (s *easyDragSmoke) showWaitingRun() string {
	s.t.Helper()
	id := s.page.MustEval(`()=>edFixture.addWaitingRun()`).Str()
	s.page.MustEval(`id=>Array.from(window.EasyDragApp._instances.values())[0].screen.showRun(id)`, id)
	s.wait(fmt.Sprintf(`()=>{const ed=edFixture.editor();return !document.querySelector('.ed-runview-banner').hidden && !!ed.runView && ed.runView.run.id==='%s'}`, id))
	s.settle()
	s.waitStop()
	return id
}

// leaveRunView ends the run view with its "Back to draft" button.
func (s *easyDragSmoke) leaveRunView() {
	s.t.Helper()
	s.page.MustEval(`()=>document.querySelector('[data-ed-cmd="exit-run-view"]').click()`)
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	s.settle()
}

// TestEasyDragOneStopBrowser: exactly one Stop is visible in the three situations that used to show two or three
// (the own test running, a running run open in the review, the runs drawer open while a run is going), and it is the
// header's, inside its slot.
func TestEasyDragOneStopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	s.wantStops(0, "idle")

	// 1. The own test is running.
	s.startOwnTest(2500)
	s.wantStops(1, "the own test is running")
	if !p.MustEval(`()=>!!document.querySelector('[data-ed-stop-slot] [data-ed-cmd="cancel"]')`).Bool() {
		t.Fatal("the Stop is not inside the Stop slot")
	}
	s.shot("stop-own-test")
	s.clickStop()
	s.waitNoStop()
	s.wantStops(0, "the own test was stopped")

	// 2. A running run is open in the review: the banner has no Stop, the header's one shows.
	s.showWaitingRun()
	s.wantStops(1, "a running run open in the review")
	if p.MustEval(`()=>!!document.querySelector('.ed-runview-banner [data-ed-cmd="stop-viewed"], .ed-runview-banner [data-ed-cmd="cancel"]')`).Bool() {
		t.Fatal("the run view's banner still has a Stop")
	}
	s.shot("stop-run-view")
	s.leaveRunView()
	s.wantStops(0, "back on the draft, nothing running")

	// 3. The runs drawer is open while a run is going: the rows of the running runs have no Stop of their own.
	p.MustEval(`()=>{edFixture.addWaitingRun();edFixture.addWaitingRun();}`)
	s.startOwnTest(2500)
	p.MustEval(`()=>window.EasyDrag.actions.run('view.runs',{host:edFixture.editor()})`)
	s.wait(`()=>document.querySelectorAll('.ed-drawer .ed-run-row').length>=3`)
	s.wait(`()=>!document.querySelector('[data-ed-run-stop], .ed-run-item')`)
	s.wantStops(1, "the runs drawer is open while the own test runs")
	s.shot("stop-drawer-open")
	s.clickStop()
	s.waitNoStop()
	p.MustEval(`()=>window.EasyDrag.actions.run('view.runs',{host:edFixture.editor()})`)
	s.wait(`()=>!document.querySelector('.ed-drawer')`)
	s.failOnPageErrors("one Stop")
}

// headerPositions returns the x position of every header control by name, after the fonts and two animation frames
// have settled. The Stop is listed only while it is visible; its slot always is.
func (s *easyDragSmoke) headerPositions() map[string]float64 {
	s.t.Helper()
	raw := s.page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
		` + easyDragVisibleJS + `
		const head=document.querySelector('.ed-editor > .ed-head')||document.querySelector('.ed-head');
		const out={};
		head.querySelectorAll('button, input, [data-ed-stop-slot]').forEach(el=>{
			const slot=el.hasAttribute('data-ed-stop-slot');
			if(!slot&&!vis(el))return;
			const name=slot?'stop-slot':el.dataset.edCmd||(el.hasAttribute('data-ed-pill')&&'pill')||(el.hasAttribute('data-ed-primary')&&'primary')||(el.dataset.edHistory&&'history-'+el.dataset.edHistory)||(el.hasAttribute('data-ed-name')&&'name')||el.className;
			const r=el.getBoundingClientRect();
			// The pill is right-aligned on a phone (its right edge stays) and left-aligned on a wide header; its width follows its text.
			out[name]=Math.round((name==='pill'&&innerWidth<480?r.right:r.left)*2)/2;
		});
		return JSON.stringify(out);
	}`).Str()
	var out map[string]float64
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		s.t.Fatalf("header positions: %v (%s)", err, raw)
	}
	return out
}

// sameHeader fails when a control of the header is at another x than idle. The Stop itself is not in the idle list: it
// must lie inside its slot, and the slot must not have moved.
func (s *easyDragSmoke) sameHeader(idle, now map[string]float64, when string) {
	s.t.Helper()
	for name, x := range idle {
		got, ok := now[name]
		if !ok {
			s.t.Errorf("%s: the header control %q is gone", when, name)
			continue
		}
		if got-x > 0.5 || x-got > 0.5 {
			s.t.Errorf("%s: the header control %q moved from x=%v to x=%v", when, name, x, got)
		}
	}
	for name := range now {
		if _, ok := idle[name]; !ok && name != "cancel" {
			s.t.Errorf("%s: the header control %q appeared", when, name)
		}
	}
}

// TestEasyDragStopPositionsBrowser: no header control moves when a run starts, while the Stop says "Stopping…", or
// when the run ends, at 390, 900 and 1440 px. The Stop sits inside its slot.
func TestEasyDragStopPositionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	for _, width := range []int{390, 900, 1440} {
		p.MustSetViewport(width, 900, 1, width < 480)
		s.settle()
		idle := s.headerPositions()
		if _, ok := idle["stop-slot"]; !ok {
			t.Fatalf("%d px: the header has no Stop slot: %v", width, idle)
		}
		s.startOwnTest(500)
		during := s.headerPositions()
		s.sameHeader(idle, during, fmt.Sprintf("%d px, during the run", width))
		if cx, ok := during["cancel"]; !ok || cx < idle["stop-slot"]-0.5 {
			t.Errorf("%d px: the Stop (x=%v) is not inside its slot (x=%v)", width, cx, idle["stop-slot"])
		}
		inside := p.MustEval(`()=>{const b=document.querySelector('[data-ed-cmd="cancel"]').getBoundingClientRect(),s=document.querySelector('[data-ed-stop-slot]').getBoundingClientRect();return b.left>=s.left-0.5&&b.right<=s.right+0.5;}`).Bool()
		if !inside {
			t.Errorf("%d px: the Stop is wider than its slot", width)
		}
		// The words the Stop shows while a stop is out are the longest it ever has: the slot holds them.
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,true,ed);}`)
		s.sameHeader(idle, s.headerPositions(), fmt.Sprintf("%d px, while stopping", width))
		inside = p.MustEval(`()=>{const b=document.querySelector('[data-ed-cmd="cancel"]').getBoundingClientRect(),s=document.querySelector('[data-ed-stop-slot]').getBoundingClientRect();return b.left>=s.left-0.5&&b.right<=s.right+0.5;}`).Bool()
		if !inside {
			t.Errorf("%d px: the Stop is wider than its slot while it says Stopping", width)
		}
		s.shot(fmt.Sprintf("stop-positions-%d", width))
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,false,ed);}`)
		s.waitNoStop()
		s.sameHeader(idle, s.headerPositions(), fmt.Sprintf("%d px, after the run", width))
	}
	s.failOnPageErrors("stop positions")
}

// TestEasyDragStopLabelBrowser: the Stop's words ("Stoppen", and "Wird gestoppt…" while a stop is out) are visible
// and not clipped at 320, 760 and 1440 px.
func TestEasyDragStopLabelBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	for _, width := range []int{320, 760, 1440} {
		p.MustSetViewport(width, 900, 1, width < 480)
		s.settle()
		s.startOwnTest(1500)
		for _, pending := range []bool{false, true} {
			key := "run_cancel"
			if pending {
				key = "run_stopping"
			}
			p.MustEval(`(pending)=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,pending,ed);}`, pending)
			raw := p.MustEval(`async(key)=>{
				await document.fonts.ready;
				await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
				const ed=edFixture.editor(),b=document.querySelector('[data-ed-cmd="cancel"]'),span=b.querySelector('span'),cs=getComputedStyle(span);
				const sr=span.getBoundingClientRect(),br=b.getBoundingClientRect();
				return JSON.stringify({text:span.textContent.trim(),want:ed.t('easydrag.ui.'+key),w:sr.width,h:sr.height,
					inside:sr.left>=br.left-0.5&&sr.right<=br.right+0.5&&sr.top>=br.top-0.5&&sr.bottom<=br.bottom+0.5,
					inViewport:br.left>=0&&br.right<=innerWidth+0.5,overflow:span.scrollWidth>span.clientWidth+1,display:cs.display,clip:cs.clip,position:cs.position});
			}`, key).Str()
			var got struct {
				Text, Want, Display, Clip, Position string
				W, H                                float64
				Inside, InViewport, Overflow        bool
			}
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("%d px: %v (%s)", width, err, raw)
			}
			if got.Text != got.Want || got.W < 8 || got.H < 8 || !got.Inside || !got.InViewport || got.Overflow || got.Display == "none" || got.Position == "absolute" || (got.Clip != "auto" && got.Clip != "") {
				t.Errorf("%d px, %s: the Stop's words are hidden or clipped: %s", width, key, raw)
			}
		}
		s.shot(fmt.Sprintf("stop-label-%d", width))
		p.MustEval(`()=>{const ed=edFixture.editor();window.EasyDrag.runStop.paint(document.querySelector('[data-ed-cmd="cancel"]'),ed.t,false,ed);}`)
		s.clickStop()
		s.waitNoStop()
	}
	s.failOnPageErrors("stop label")
}

// TestEasyDragStopClicksBrowser clicks the real Stop with desktop animations on: the own test stops at once, and a
// running run open in the review stops after the question, whose dialog closes with an animation.
func TestEasyDragStopClicksBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page
	// The fixture shell sets windows.animations to false; make the desktop animate so the closing class really appears.
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)
	s.openFirstFlow()

	s.startOwnTest(2500)
	s.clickStop()
	s.waitNoStop()
	s.wait(`()=>{const r=edFixture.editor().run;return !!r && r.status==='cancelled'}`)

	id := s.showWaitingRun()
	s.clickStop()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	p.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(fmt.Sprintf(`()=>edFixture.state.runs.get('%s').run.status==='cancelled' && !document.querySelector('.ed-modal-backdrop')`, id))
	s.waitNoStop()
	s.wantStops(0, "the waiting run was stopped")
	s.failOnPageErrors("stop clicks")
}

// TestEasyDragStopKeyboardPendingBrowser stops the own test from the keyboard while the cancel answer is slow: the pending Stop is
// aria-disabled (not disabled, which would take the focus away) and keeps the focus, and when the run ends the focus lands on the
// canvas instead of on the body.
func TestEasyDragStopKeyboardPendingBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	s.startOwnTest(2500)
	// A slow cancel: the request waits before the fixture answers it.
	p.MustEval(`()=>{const f=window.fetch;window.fetch=async(url,o)=>{if(String(url).endsWith('/cancel'))await new Promise(r=>setTimeout(r,900));return f(url,o)}}`)
	s.waitStop()
	p.MustEval(`()=>document.querySelector('[data-ed-cmd="cancel"]').focus()`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>{const b=document.querySelector('[data-ed-cmd="cancel"]');return b.getAttribute('aria-busy')==='true' && b.getAttribute('aria-disabled')==='true' && !b.disabled && document.activeElement===b}`)
	s.wait(`()=>{` + easyDragVisibleJS + `return !vis(document.querySelector('[data-ed-cmd="cancel"]'));}`)
	s.wait(`()=>{const r=edFixture.editor().run;return !!r && r.status==='cancelled'}`)
	s.wait(`()=>{const a=document.activeElement;return a===document.querySelector('.ed-canvas') || a===document.querySelector('[data-ed-cmd="exit-run-view"]')}`)
	s.failOnPageErrors("stop keyboard pending")
}

// headerRects returns the rectangle of the Stop slot, Test, Publish, Undo, Redo and "⋯" as "left,top,width,height" rounded to a
// half pixel (the top relative to the row of actions: the pill above may wrap with its text), after the fonts and two animation frames have settled.
func (s *easyDragSmoke) headerRects() []string {
	s.t.Helper()
	raw := s.page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
		const head=document.querySelector('.ed-head'),top=head.querySelector('.ed-head-actions').getBoundingClientRect().top;
		return JSON.stringify(['[data-ed-stop-slot]','[data-ed-cmd="test"]','[data-ed-cmd="publish"]','[data-ed-history="undo"]','[data-ed-history="redo"]','[data-ed-cmd="more"]'].map(sel=>{
			const el=head.querySelector(sel);if(!el)return sel+' missing';
			const r=el.getBoundingClientRect();return sel+' '+[r.left,r.top-top,r.width,r.height].map(v=>Math.round(v*2)/2).join(',');
		}));
	}`).Str()
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		s.t.Fatalf("header rects: %v (%s)", err, raw)
	}
	return out
}

// TestEasyDragStopPositionsPublishBrowser: a changed draft whose test succeeds. Publish is in its place before the test, while it
// runs (unavailable) and after it ended (filled): the Stop slot, Test, Publish, Undo, Redo and "⋯" keep their rectangles.
func TestEasyDragStopPositionsPublishBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openFirstFlow()
	p.MustEval(`()=>{edFixture.state.changes={steps:2,connections:1};const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed before the test')}`)
	s.wait(`()=>edFixture.editor().saver.state==='saved'`)
	s.wait(`()=>{const b=document.querySelector('[data-ed-cmd="publish"]');const t=document.querySelector('[data-ed-cmd="test"]');return !b.hidden && !b.hasAttribute('data-ed-primary') && t.hasAttribute('data-ed-primary') && /3/.test(b.textContent)}`)
	same := func(idle, now []string, when string) {
		t.Helper()
		for i := range idle {
			if idle[i] != now[i] {
				t.Errorf("%s: %q became %q", when, idle[i], now[i])
			}
		}
	}
	for _, width := range []int{390, 900, 1440} {
		p.MustSetViewport(width, 900, 1, width < 480)
		s.settle()
		idle := s.headerRects()
		s.startOwnTest(300)
		same(idle, s.headerRects(), fmt.Sprintf("%d px, while the test runs", width))
		if got := p.MustEval(`()=>document.querySelector('[data-ed-cmd="publish"]').getAttribute('aria-disabled')`).String(); got != "true" {
			t.Errorf("%d px: Publish is %q while a run goes, want aria-disabled=true", width, got)
		}
		s.waitNoStop()
		s.wait(`()=>{const r=edFixture.editor().run;return !!r && r.status==='success'}`)
		// The server says that this revision passed the test (the fixture's runs carry no draft revision of their own).
		s.markTested()
		s.wait(`()=>{const b=document.querySelector('[data-ed-cmd="publish"]');return !b.hidden && b.hasAttribute('data-ed-primary') && !document.querySelector('[data-ed-cmd="test"]').hasAttribute('data-ed-primary')}`)
		same(idle, s.headerRects(), fmt.Sprintf("%d px, after the test", width))
		s.shot(fmt.Sprintf("stop-positions-publish-%d", width))
		// The next width starts from an untested draft again: an edit takes the evidence away.
		p.MustEval(`()=>{edFixture.state.tested=false;const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed again '+Date.now())}`)
		s.wait(`()=>edFixture.editor().saver.state==='saved' && document.querySelector('[data-ed-cmd="test"]').hasAttribute('data-ed-primary')`)
	}
	s.failOnPageErrors("stop positions publish")
}
