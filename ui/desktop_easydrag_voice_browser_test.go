package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// W1b B.6 (voice guide rule 6): the editor does not toast its own visible results. Publishing and the
// active switch speak through the editor's live region instead, and "JSON kopieren" confirms on its button.
// The helpers below run inside TestDesktopEasyDragBrowser; the home Run now check is its own small test.

const easyDragToastCount = `()=>document.querySelectorAll('.vd-toast').length`

// recordAnnouncements starts writing every text of the editor's live region to window.__edSpoken.
func (s *easyDragSmoke) recordAnnouncements() {
	s.t.Helper()
	s.page.MustEval(`()=>{window.__edSpoken=[];const live=document.querySelector('[data-ed-live]');new MutationObserver(()=>{if(live.textContent)window.__edSpoken.push(live.textContent)}).observe(live,{childList:true,characterData:true,subtree:true})}`)
}

// publishAndActiveAreQuiet checks, after the journey's complete publish, that it was announced and not
// toasted, and that switching the flow off and on again does the same.
func (s *easyDragSmoke) publishAndActiveAreQuiet() {
	s.t.Helper()
	page := s.page
	s.wait(`()=>{const ed=edFixture.editor();const text=ed.t('easydrag.ui.publish_done',{name:ed.model.doc.name});return window.__edSpoken.includes(text) && ![...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(text))}`)
	for i := 0; i < 2; i++ {
		page.MustEval(`()=>{window.__edSpoken.length=0}`)
		page.MustElement(`[data-ed-cmd="active"]`).MustClick()
		s.wait(`()=>{const ed=edFixture.editor();const text=ed.t(ed.flowEnabled?'easydrag.ui.active_on':'easydrag.ui.active_off');return window.__edSpoken.includes(text) && ![...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(text))}`)
	}
}

// copyJSONConfirms opens the Telegram step after the test run, copies its output and checks that the
// button says "Kopiert" for a moment, that no notification appears, and that the label comes back.
func (s *easyDragSmoke) copyJSONConfirms() {
	s.t.Helper()
	page := s.page
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('open-detail',{nodeId:ed.model.doc.nodes.find(n=>n.type==='notify.telegram').id})}`)
	// In this window the step view uses its narrow tabs and opens on the parameters: the copy button sits in the output tab.
	s.wait(`()=>!!document.querySelector('.ed-detail [data-ed-narrow="output"]')`)
	page.MustElement(`.ed-detail [data-ed-narrow="output"]`).MustClick()
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');return !!b && !b.hidden && b.getClientRects().length>0}`)
	toasts := page.MustEval(easyDragToastCount).Int()
	page.MustEval(`()=>document.querySelector('[data-ed-copy-json]').click()`)
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');const done=edFixture.editor().t('easydrag.ui.copy_json_done');return done!=='easydrag.ui.copy_json_done' && b.hasAttribute('data-ed-flash') && b.textContent.trim()===done && b.getAttribute('aria-label')===done}`)
	if got := page.MustEval(easyDragToastCount).Int(); got > toasts {
		s.t.Fatalf("copying the JSON opened a desktop notification (%d toasts before, %d after)", toasts, got)
	}
	s.wait(`()=>{const b=document.querySelector('[data-ed-copy-json]');return !b.hasAttribute('data-ed-flash') && b.getAttribute('aria-label')===edFixture.editor().t('easydrag.ui.copy_json')}`)
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
}

// TestDesktopEasyDragHomeRunNowBrowser: Run now on a home card sends no notification; the card's last-run
// line says "Läuft" with the running disc, and the start is announced.
func TestDesktopEasyDragHomeRunNowBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustEval(`()=>{window.__edSpoken=[];const live=document.querySelector('[data-ed-home-live]');new MutationObserver(()=>{if(live.textContent)window.__edSpoken.push(live.textContent)}).observe(live,{childList:true,characterData:true,subtree:true})}`)
	p.MustElement(`.ed-flow-card [data-ed-card-menu]`).MustClick()
	s.wait(`()=>[...document.querySelectorAll('.vd-context-menu .vd-context-item')].some(b=>b.textContent.includes(t('easydrag.ui.run_now')) && !b.disabled)`)
	p.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(b=>b.textContent.includes(t('easydrag.ui.run_now'))).click()`)
	s.wait(`()=>{const run=document.querySelector('.ed-flow-card .ed-flow-run [data-status="running"]');return !!run && run.textContent.includes(t('easydrag.status.running')) && window.__edSpoken.includes(t('easydrag.ui.run_started')) && !document.querySelector('.vd-toast')}`)
	s.noRawKeys("home run now")
	s.failOnPageErrors("home run now")
}
