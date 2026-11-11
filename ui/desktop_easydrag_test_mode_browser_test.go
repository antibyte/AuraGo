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
