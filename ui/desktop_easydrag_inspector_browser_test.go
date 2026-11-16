package ui

import (
	"encoding/json"
	"testing"
)

// TestEasyDragInspectorSwapBrowser double-clicks a step, then another step while the inspector shows the first, then closes the
// inspector. The inspector is a side panel in the editor body's flex row: the panel it replaces and the panel that closes must leave
// that row at once. A panel that stays there while it is already invisible takes its width from the canvas for a moment, and an
// empty strip shows between the canvas and the new inspector (or where the closed one was).
func TestEasyDragInspectorSwapBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.openSeededEditor()

	// The probe runs after the editor's own handlers (document, bubble phase): it records how many inspector panels the body
	// holds and the empty width between the canvas and the panel that is open (or the body's right edge when none is).
	p.MustEval(`()=>{window.__edProbe=[];const measure=what=>{const body=document.querySelector('.ed-body').getBoundingClientRect(),canvas=document.querySelector('.ed-canvas').getBoundingClientRect(),panels=[...document.querySelectorAll('.ed-body > .ed-detail-backdrop')],live=panels.find(x=>!x.classList.contains('is-closing'));window.__edProbe.push({what,panels:panels.length,gap:Math.round((live?live.getBoundingClientRect().left:body.right)-canvas.right)})};document.addEventListener('dblclick',()=>measure('dblclick'));document.addEventListener('click',e=>{if(e.target.closest&&e.target.closest('[data-ed-detail-close]'))measure('close')})}`)

	first := p.MustEval(`()=>document.querySelector('.ed-node').dataset.nodeId`).Str()
	p.MustElement(`.ed-node[data-node-id="` + first + `"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>{const d=edFixture.editor().detail;return !!d && d.nodeId()==='` + first + `'}`)
	s.settle()

	// The second step: one that the narrower canvas still shows whole, so the double-click reaches it and not the inspector.
	second := p.MustEval(`id=>{const c=document.querySelector('.ed-canvas').getBoundingClientRect();const n=[...document.querySelectorAll('.ed-node')].find(n=>{const r=n.getBoundingClientRect();return n.dataset.nodeId!==id && r.left>=c.left && r.right<=c.right && r.top>=c.top && r.bottom<=c.bottom});return n?n.dataset.nodeId:''}`, first).Str()
	if second == "" {
		t.Fatal("no second step is fully visible beside the inspector")
	}
	p.MustElement(`.ed-node[data-node-id="` + second + `"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>{const d=edFixture.editor().detail;return !!d && d.nodeId()==='` + second + `'}`)
	p.MustElement(`.ed-detail-backdrop:not(.is-closing) [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!edFixture.editor().detail`)

	type sample struct {
		What   string `json:"what"`
		Panels int    `json:"panels"`
		Gap    int    `json:"gap"`
	}
	var got []sample
	if err := json.Unmarshal([]byte(p.MustEval(`()=>JSON.stringify(window.__edProbe)`).Str()), &got); err != nil {
		t.Fatal(err)
	}
	want := []sample{{"dblclick", 1, 0}, {"dblclick", 1, 0}, {"close", 0, 0}}
	if len(got) != len(want) {
		t.Fatalf("probe samples = %+v, want %+v", got, want)
	}
	for i, w := range want {
		g := got[i]
		if g.What != w.What || g.Panels != w.Panels || g.Gap < -1 || g.Gap > 1 {
			t.Fatalf("sample %d = %+v, want %+v (gap within 1 px): an inspector panel that is replaced or closed still holds its width in the row", i, g, w)
		}
	}
	s.failOnPageErrors("inspector swap")
}
