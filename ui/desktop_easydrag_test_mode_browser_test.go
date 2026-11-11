package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragTestModeBrowser: a flow that sends a Telegram message opens Test in
// Simulation, and the mode control is named "Testart" (the fixture speaks German). In a real
// test, Enter on the select, on a checkbox of the dialog and on the run button starts nothing
// and confirms nothing; Space on the run button still starts the real test.
func TestDesktopEasyDragTestModeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
	p.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!edFixture.editor() && !!document.querySelector('[data-ed-cmd="test"]')`)
	p.MustEval(`()=>{const ed=edFixture.editor();ed.model.addNode('notify.telegram',{x:1280,y:120},{message:'Hallo'},{node:'n_pdf',port:'out'});}`)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.some(n=>n.type==='notify.telegram')}`)
	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>{const m=document.querySelector('.ed-modal [data-ed-test-mode]');return !!m && m.value==='simulation' && !document.querySelector('.ed-modal .ed-simulation-fields').hidden}`)
	p.MustEval(`()=>{if(document.activeElement && document.activeElement.matches('[data-ed-action="run"]'))throw Error('the run button has the default focus')}`)

	// The accessible name, read from the browser's accessibility tree.
	if err := (proto.AccessibilityEnable{}).Call(p); err != nil {
		t.Fatal(err)
	}
	desc, err := p.MustElement(`.ed-modal [data-ed-test-mode]`).Describe(0, false)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := proto.AccessibilityGetPartialAXTree{BackendNodeID: desc.BackendNodeID}.Call(p)
	if err != nil {
		t.Fatal(err)
	}
	name := ""
	for _, n := range tree.Nodes {
		if n.BackendDOMNodeID == desc.BackendNodeID && n.Name != nil {
			name = n.Name.Value.Str()
		}
	}
	if name != "Testart" {
		t.Fatalf("test mode name = %q, want Testart", name)
	}

	// Real mode: Enter starts nothing and confirms nothing.
	p.MustEval(`()=>{window.__edRequests=edFixture.state.requests.length;const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	for _, selector := range []string{`.ed-modal [data-ed-test-mode]`, `.ed-modal [data-ed-test-remember]`, `.ed-modal [data-ed-action="run"]`} {
		p.MustElement(selector).MustFocus()
		p.Keyboard.MustType(input.Enter)
	}
	time.Sleep(500 * time.Millisecond)
	p.MustEval(`()=>{const posts=edFixture.state.requests.slice(window.__edRequests).filter(r=>/^POST \S+\/test$/.test(r));const c=edFixture.editor().effectsConfirmed;if(posts.length || (c instanceof Set && c.size) || !document.querySelector('.ed-modal [data-ed-action="run"]'))throw Error('Enter started a real test: '+posts.join());}`)

	// Space on the run button still starts the real test.
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustFocus()
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>edFixture.state.requests.slice(window.__edRequests).some(r=>/^POST \S+\/test$/.test(r))`)
	s.noRawKeys("test mode")
	s.failOnPageErrors("test mode")
}

// TestDesktopEasyDragGoalCounterBrowser: the AI goal has no maxlength, a counter linked by
// aria-describedby counts code points, and above 4,000 it turns to the warn ink and Generate
// is disabled.
func TestDesktopEasyDragGoalCounterBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
	p.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!edFixture.editor()`)
	p.MustEval(`()=>{window.__edAuthoring=EasyDrag.authoring.create(edFixture.editor(),null);window.__edAuthoring.open();}`)
	s.wait(`()=>!!document.querySelector('[data-ed-authoring-goal]') && !!document.querySelector('[data-ed-goal-counter]')`)
	p.MustEval(`()=>{const a=document.querySelector('[data-ed-authoring-goal]'),c=document.querySelector('[data-ed-goal-counter]');if(a.hasAttribute('maxlength'))throw Error('maxlength is still set');if(!c.id || a.getAttribute('aria-describedby')!==c.id)throw Error('the counter is not linked');a.value=String.fromCodePoint(0x1F600).repeat(4001);a.dispatchEvent(new Event('input',{bubbles:true}));}`)
	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]'),nf=new Intl.NumberFormat(document.documentElement.lang||undefined);return c.classList.contains('is-over') && g.disabled && c.textContent.includes(nf.format(4001)) && c.textContent.includes(nf.format(4000));}`)
	p.MustEval(`()=>{const a=document.querySelector('[data-ed-authoring-goal]');a.value=String.fromCodePoint(0x1F600).repeat(4000);a.dispatchEvent(new Event('input',{bubbles:true}));}`)
	s.wait(`()=>{const c=document.querySelector('[data-ed-goal-counter]'),g=document.querySelector('.ed-modal [data-ed-action="generate"]');return !c.classList.contains('is-over') && !g.disabled;}`)
	s.noRawKeys("goal counter")
	s.failOnPageErrors("goal counter")
}
