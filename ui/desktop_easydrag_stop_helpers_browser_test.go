package ui

import (
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// easyDragVisibleJS defines vis(el) for page scripts: the control is not hidden, has a real size and none of its
// ancestors is display:none, visibility:hidden or fully transparent. The Stop slot keeps its width when idle, so
// "the button exists" and "the button can be seen" differ.
const easyDragVisibleJS = `const vis=el=>{if(!el||el.hidden)return false;const r=el.getBoundingClientRect();if(r.width<2||r.height<2)return false;for(let n=el;n&&n.nodeType===1;n=n.parentElement){const c=getComputedStyle(n);if(c.display==='none'||c.visibility==='hidden'||c.opacity==='0')return false;}return true;};`

// stopsVisible counts the controls of the open editor that a user reads as a Stop: the header's button, the old
// banner and row hooks, and any other button whose text or name is the Stop word or the Stopping word. A
// confirmation dialog's own "Stop" button does not count (it asks, it does not stop by itself).
func (s *easyDragSmoke) stopsVisible() int {
	s.t.Helper()
	return s.page.MustEval(`()=>{` + easyDragVisibleJS + `
		const ed=edFixture.editor(),words=[ed.t('easydrag.ui.run_cancel'),ed.t('easydrag.ui.run_stopping')];
		return [...ed.root.querySelectorAll('button')].filter(b=>!b.closest('.ed-modal-backdrop')&&vis(b)&&(['cancel','stop-viewed'].includes(b.dataset.edCmd)||b.hasAttribute('data-ed-run-stop')||words.includes(b.textContent.trim())||words.includes(b.getAttribute('aria-label')||''))).length;}`).Int()
}

// waitStop waits until the header Stop can be pressed: visible, enabled, not busy, and at the same place in two
// samples in a row (the header repaints on every run event; a click on a button that was just replaced fails with
// "element has no visible shape").
func (s *easyDragSmoke) waitStop() {
	s.t.Helper()
	s.wait(`()=>{` + easyDragVisibleJS + `
		const b=document.querySelector('[data-ed-cmd="cancel"]');
		if(!vis(b)||b.disabled||b.getAttribute('aria-disabled')==='true'||b.getAttribute('aria-busy')==='true'){window.__stopRect='';return false;}
		const r=b.getBoundingClientRect(),key=[r.left,r.top,r.width,r.height].map(Math.round).join(',');
		const same=window.__stopRect===key;window.__stopRect=key;return same;}`)
}

// clickStop clicks the real header Stop once it is ready, and tries again when the page repainted under the click.
func (s *easyDragSmoke) clickStop() {
	s.t.Helper()
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		s.waitStop()
		el, err := s.page.Element(`[data-ed-cmd="cancel"]`)
		if err == nil {
			if err = el.Click(proto.InputMouseButtonLeft, 1); err == nil {
				return
			}
		}
		last = err
		time.Sleep(60 * time.Millisecond)
	}
	s.t.Fatalf("the header Stop could not be clicked: %v", last)
}

// wantStops fails the test unless exactly n Stop controls are visible.
func (s *easyDragSmoke) wantStops(n int, when string) {
	s.t.Helper()
	if got := s.stopsVisible(); got != n {
		s.t.Fatalf("%s: %d Stop controls are visible, want %d", when, got, n)
	}
}
