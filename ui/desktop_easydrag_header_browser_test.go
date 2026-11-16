package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// openSeededEditor opens the seeded published flow and waits until its four steps are drawn and settled.
func (s *easyDragSmoke) openSeededEditor() {
	s.t.Helper()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.wait(`()=>!!document.querySelector('[data-ed-pill]') && document.querySelector('[data-ed-pill-text]').textContent.length>0`)
}

// TestEasyDragHeaderPopoverBrowser opens the pill's popover and closes it with the keyboard and the mouse: Enter opens it and moves the
// focus in, Tab walks its entries, Escape closes it and gives the focus back to the pill, a press outside closes it. The next run the
// fixture reports shows as the weekday and the time.
func TestEasyDragHeaderPopoverBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openSeededEditor()
	p.MustEval(`()=>{edFixture.state.nextRun={at:new Date(Date.now()+36*3600*1000).toISOString(),node:'n_zeitplan'}}`)

	p.MustEval(`()=>document.querySelector('[data-ed-pill]').focus()`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]') && document.activeElement===document.querySelector('[data-ed-pop="active"]') && document.querySelector('[data-ed-pill]').getAttribute('aria-expanded')==='true'`)
	// The popover read the flow when it opened: the fixture's next run is there, in the words of ED.format.
	s.wait(`()=>{const at=edFixture.state.nextRun.at;const v=document.querySelector('[data-ed-next-run]').textContent;return v.length>0 && v===window.EasyDrag.format.nextRun(at)}`)
	s.shot("header-popover")
	s.noRawKeys("header popover")
	p.Keyboard.MustType(input.Tab)
	s.wait(`()=>document.activeElement===document.querySelector('[data-ed-pop="run-now"]')`)
	p.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]') && document.activeElement===document.querySelector('[data-ed-pill]') && document.querySelector('[data-ed-pill]').getAttribute('aria-expanded')==='false'`)

	// Tab past the last entry ("Läufe") closes it and the focus goes back to the pill, not on to the end of the editor.
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	p.MustEval(`()=>{const all=[...document.querySelectorAll('[data-ed-pill-popover] button')];all[all.length-1].focus()}`)
	if name := p.MustEval(`()=>document.activeElement.getAttribute('data-ed-pop')`).String(); name != "runs" {
		t.Fatalf("the last popover entry is %q, want runs", name)
	}
	p.Keyboard.MustType(input.Tab)
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]') && document.activeElement===document.querySelector('[data-ed-pill]')`)

	// A press on the canvas closes it without moving the focus to the pill.
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	// A point near the top edge of the canvas: the centre may sit on a step, and rod waits for the clicked element to be uncovered.
	box := p.MustElement(`.ed-canvas`).MustShape().Box()
	p.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+12).MustClick(proto.InputMouseButtonLeft)
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]')`)
	s.failOnPageErrors("header popover")
}

// TestEasyDragHeaderPrimaryBrowser walks the primary button through its states with the fixture's facts: none for a clean live flow, Test for a
// changed and untested draft, Publish with its change count once the server says the draft is tested, and the dialog (never a direct publish).
func TestEasyDragHeaderPrimaryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openSeededEditor()
	filled := `[...document.querySelectorAll('.ed-head [data-ed-primary]')].map(b=>b.getAttribute('data-ed-cmd')).join()`
	s.wait(`()=>` + filled + `==='' && document.querySelector('[data-ed-cmd="publish"]').hidden && !document.querySelector('[data-ed-cmd="test"]').hidden`)
	s.shot("header-primary-none")

	// A changed draft the server has not seen pass a test: Test is the filled button.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed once')}`)
	s.wait(`()=>` + filled + `==='test' && document.querySelector('[data-ed-cmd="publish"]').hidden && edFixture.editor().saver.state==='saved'`)
	s.shot("header-primary-test")

	// The next save answers "tested" and the count of the changes: three, as the button says.
	p.MustEval(`()=>{edFixture.state.tested=true;edFixture.state.changes={steps:2,connections:1};const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed twice')}`)
	s.wait(`()=>` + filled + `==='publish' && !document.querySelector('[data-ed-cmd="publish"]').hidden && /3/.test(document.querySelector('[data-ed-cmd="publish"]').textContent)`)
	s.shot("header-primary-publish")
	s.noRawKeys("header primary")

	// The primary button opens the full dialog and publishes nothing by itself.
	published := p.MustEval(`()=>edFixture.flow().rec.published_draft_revision`).Int()
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	if got := p.MustEval(`()=>edFixture.flow().rec.published_draft_revision`).Int(); got != published {
		t.Fatalf("the primary button published the draft by itself: revision %d, want %d", got, published)
	}
	p.MustElement(`.ed-modal [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)

	// A real publish: the header hides Publish (a clean live flow) while the dialog is open, and the focus must still land on a visible
	// control of the editor, not on the body.
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	p.MustElement(`.ed-modal [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && document.querySelector('[data-ed-cmd="publish"]').hidden`)
	s.wait(`()=>{const a=document.activeElement,r=document.querySelector('.ed-editor');return !!a && a!==document.body && !!r && r.contains(a) && a.getClientRects().length>0 && !a.hidden}`)
	s.failOnPageErrors("header primary")
}

// TestEasyDragHeaderPopoverMotionBrowser clicks real popover entries with the desktop's animations on (the W1b lesson: a test with reduced
// motion cannot see an action that does nothing while a menu closes with animation): Active switches the flow off and on in place, Run now
// closes the popover and starts a run, Runs opens the drawer.
func TestEasyDragHeaderPopoverMotionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)
	s.openSeededEditor()

	p.MustElement(`[data-ed-pill]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="active"]`).MustClick()
	s.wait(`()=>edFixture.editor().flowEnabled===false && document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='paused' && document.querySelector('[data-ed-pop="active"]').getAttribute('aria-checked')==='false'`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="active"]`).MustClick()
	s.wait(`()=>edFixture.editor().flowEnabled===true && document.querySelector('[data-ed-pill]').getAttribute('data-ed-pill-kind')==='live'`)
	p.MustElement(`[data-ed-pill-popover] [data-ed-pop="run-now"]`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-pill-popover]') && !!edFixture.editor().run`)
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success'}`)
	s.pillEntry("runs")
	s.wait(`()=>!!document.querySelector('.ed-drawer') && !document.querySelector('[data-ed-pill-popover]')`)
	s.failOnPageErrors("header popover motion")
}
