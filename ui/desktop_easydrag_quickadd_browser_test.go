package ui

import "testing"

// TestEasyDragQuickAddSuggestionsBrowser is the E9 acceptance of the quick-add (W1c Part D): from a
// trigger's "+" the header names the trigger, the suggestions come first, "Show all" opens the full
// palette scoped to that trigger, and the next step chosen there lands after it. It runs with
// motion on and uses real pointer events: the "+" button reveals itself on hover, and the
// popover animates in and out.
func TestEasyDragQuickAddSuggestionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()

	// The server sends the curated list as "suggest" on each step type (internal/flows, TestEveryStepWithOutputsGetsSuggestions);
	// the fixture catalog has none, so the test puts one on the editor's copy of the schedule trigger. The fifth entry is a
	// step that is not set up; a sixth shows that the list stops at five.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.catalog.types.get('trigger.schedule').suggest=['ai.prompt','web.search','notify.telegram','smart_home.service','http.request','logic.if'];window.__ids=ed.model.doc.nodes.map(n=>n.id);}`)

	// A real pointer: hover the trigger's card (the "+" scales up from 0.6 on hover), then click it.
	p.MustElement(`.ed-node[data-node-id="n_zeitplan"]`).MustHover()
	p.MustElement(`.ed-node[data-node-id="n_zeitplan"] [data-ed-port-add]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-quick.is-open') && document.activeElement && document.activeElement.classList.contains('ed-quick-search')`)
	s.settle()

	// The header reads "<Steps after>: <the trigger's label>" and is the dialog's accessible name.
	if got := p.MustEval(`()=>{const q=document.querySelector('.ed-quick');const head=q.querySelector('.ed-quick-head');const want=window.t('easydrag.ui.quick_head_after',{name:'Werktags 7 Uhr'});return head.textContent===want && q.getAttribute('aria-label')===want && want.includes('Werktags 7 Uhr') && !want.includes('{')?'':'header "'+head.textContent+'", name "'+q.getAttribute('aria-label')+'", want "'+want+'"';}`).Str(); got != "" {
		t.Fatalf("the quick-add header is wrong: %s", got)
	}

	// The suggestions come first: five entries, the curated order, the one that is not set up greyed, then the rest.
	if got := p.MustEval(`()=>{const q=document.querySelector('.ed-quick');const items=Array.from(q.querySelectorAll('[data-ed-index]'));const marked=items.filter(i=>i.hasAttribute('data-ed-suggested')).map(i=>i.dataset.edType);const first=items.slice(0,5).map(i=>i.dataset.edType);const group=q.querySelector('.ed-quick-list > [data-ed-quick-suggested]');const grey=items.filter(i=>i.classList.contains('is-unavailable')&&i.hasAttribute('data-ed-suggested')).map(i=>i.dataset.edType);const rest=items.slice(5).map(i=>i.dataset.edType);return JSON.stringify({marked,first,grey,group:group&&group.textContent===window.t('easydrag.ui.quick_suggested'),groupFirst:q.querySelector('.ed-quick-list').firstElementChild===group,restHasSixth:rest.includes('logic.if'),restRepeats:rest.some(x=>marked.includes(x))});}`).Str(); got != `{"marked":["ai.prompt","web.search","notify.telegram","smart_home.service","http.request"],"first":["ai.prompt","web.search","notify.telegram","smart_home.service","http.request"],"grey":["smart_home.service"],"group":true,"groupFirst":true,"restHasSixth":true,"restRepeats":false}` {
		t.Fatalf("the suggestions are wrong: %s", got)
	}
	// The popover is whole inside the editor.
	if !p.MustEval(`()=>{const q=document.querySelector('.ed-quick').getBoundingClientRect();const e=document.querySelector('.ed-editor').getBoundingClientRect();return q.left>=e.left-0.5&&q.right<=e.right+0.5&&q.top>=e.top-0.5&&q.bottom<=e.bottom+0.5;}`).Bool() {
		t.Fatal("the quick-add reaches outside the editor")
	}
	s.shot("quickadd-header-suggestions")

	// "Show all" closes the popover and opens the full palette with its search focused and the scope banner.
	p.MustElement(`.ed-quick [data-ed-quick-all]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-quick') && !!document.querySelector('[data-ed-palette-scope]:not([hidden])') && document.activeElement===document.querySelector('.ed-palette-search')`)
	if got := p.MustEval(`()=>{const b=document.querySelector('[data-ed-palette-scope]');const want=window.t('easydrag.ui.palette_scope',{name:'Werktags 7 Uhr'});return b.querySelector('.ed-palette-scope-text').textContent===want && want.includes('Werktags 7 Uhr') && !!b.querySelector('[data-ed-palette-scope-cancel]')?'':'banner "'+b.textContent+'", want "'+want+'"';}`).Str(); got != "" {
		t.Fatalf("the palette scope banner is wrong: %s", got)
	}
	s.shot("quickadd-show-all-scope")

	// The next click on a palette step places it after the trigger, exactly where the quick-add would have.
	p.MustElement(".ed-palette-search").MustInput("Telegram")
	s.wait(`()=>document.querySelectorAll('.ed-palette-item[data-ed-type="notify.telegram"]').length===1`)
	p.MustElement(`.ed-palette-item[data-ed-type="notify.telegram"]`).MustClick()
	s.wait(`()=>{const m=edFixture.editor().model;const added=m.doc.nodes.find(n=>!window.__ids.includes(n.id));return m.doc.nodes.length===5 && !!added && added.type==='notify.telegram' && m.incoming(added.id).map(e=>e.source.node).join()==='n_zeitplan' && document.querySelector('[data-ed-palette-scope]').hidden}`)
	s.settle()
	s.failOnPageErrors("quick-add suggestions")
}
