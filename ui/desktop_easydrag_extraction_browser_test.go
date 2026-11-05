package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragExtractionBrowser checks the native review, undo and uncertain reply flow.
func TestDesktopEasyDragExtractionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-extraction-fixture.js")
	p := s.page
	seed := func(name string) string {
		t.Helper()
		return p.MustEval(`name=>{const id=edFixture.addEmptyDraft(name),d=edFixture.flow(id).rec.draft;d.nodes=[{id:'n_aaaaaaaa',key:'start',type:'trigger.manual',type_version:1,params:{},settings:{},position:{x:0,y:0}},{id:'n_bbbbbbbb',key:'result',type:'logic.set',type_version:1,params:{keep_input:false,fields:[{name:'message',type:'text',value:'Hello'}]},settings:{},position:{x:360,y:0}}];d.edges=[{id:'e_aaaaaaaa',source:{node:'n_aaaaaaaa',port:'out'},target:{node:'n_bbbbbbbb',port:'in'}}];return id;}`, name).Str()
	}
	open := func(id string) {
		t.Helper()
		p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, id)
		s.wait(`()=>edFixture.editor()?.flow.id==='` + id + `' && document.querySelectorAll('.ed-node').length===2`)
	}
	start := func() {
		t.Helper()
		node := p.MustElement(`.ed-node[data-node-id="n_bbbbbbbb"] .ed-node-label`)
		node.MustClick()
		s.wait(`()=>edFixture.editor().selection.has('n_bbbbbbbb')`)
		node.MustHover()
		p.Mouse.MustClick(proto.InputMouseButtonRight)
		s.wait(`()=>[...document.querySelectorAll('.vd-context-menu button')].some(b=>b.textContent.includes(edFixture.editor().t('easydrag.ui.extraction_title')))`)
		p.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu button')].find(b=>b.textContent.includes(edFixture.editor().t('easydrag.ui.extraction_title'))).click()`)
		s.wait(`()=>!!document.querySelector('.ed-modal--extraction [data-ed-extract-name]')`)
	}
	preview := func(name string) {
		t.Helper()
		p.MustElement(`[data-ed-extract-name]`).MustSelectAllText().MustInput(name)
		p.MustElement(`.ed-modal--extraction [data-ed-action="preview"]`).MustClick()
		s.wait(`()=>{const r=document.querySelector('[data-ed-extract-review]');return r && !r.hidden && !!r.textContent.trim() && !document.querySelector('.ed-modal--extraction [data-ed-action="apply"]').hidden;}`)
	}
	first := seed("Auslagern")
	open(first)
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustSetViewport(1440, 900, 1, false)
	start()
	s.shot("extraction-before-review")
	preview("Mein Baustein")
	s.shot("extraction-review-desktop")
	p.MustSetViewport(390, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-dark')`)
	s.shot("extraction-review-fruity-390")
	p.MustSetViewport(320, 844, 1, true)
	s.wait(`()=>[...document.querySelectorAll('.ed-modal--extraction [data-ed-action]')].filter(b=>!b.hidden).every(b=>{const r=b.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth && r.top>=0 && r.bottom<=innerHeight;})`)
	s.shot("extraction-review-fruity-320")
	p.MustElement(`.ed-modal--extraction [data-ed-action="apply"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--extraction') && edFixture.editor().model.doc.nodes.some(n=>n.type==='component.call')`)
	s.wait(`()=>edFixture.editor().saver.revision===2 && edFixture.flow('` + first + `').rec.draft_revision===2`)
	p.MustEval(`()=>window.__c10SaveCount=edFixture.state.saves.length`)
	p.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,500))`)
	p.MustEval(`id=>{const calls=edExtractionFixture.state.requests.filter(r=>r.flow_id===id && r.action==='apply');if(calls.length!==1 || edFixture.state.saves.length!==window.__c10SaveCount)throw Error('Apply retried or performed a second PUT');}`, first)
	s.shot("extraction-after-apply")
	p.MustElement(`[data-ed-history="undo"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.some(n=>n.id==='n_bbbbbbbb') && !edFixture.editor().model.doc.nodes.some(n=>n.type==='component.call')`)
	p.MustEval(`()=>{if(edBlockFixture.state.blocks.size!==1)throw Error('Undo deleted block');}`)

	// A lost Apply response must be reconciled, never sent a second time.
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	second := seed("Unsichere Antwort")
	open(second)
	start()
	preview("Zweiter Baustein")
	p.MustEval(`()=>{edExtractionFixture.state.loseNextApply=true}`)
	p.MustElement(`.ed-modal--extraction [data-ed-action="apply"]`).MustClick()
	s.wait(`()=>{const e=document.querySelector('[data-ed-extraction-status]');return e && !e.hidden && !!document.querySelector('.ed-modal--extraction [data-ed-action="reconcile"]:not([hidden])');}`)
	s.shot("extraction-uncertain")
	p.MustSetViewport(320, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-dark')`)
	s.wait(`()=>[...document.querySelectorAll('.ed-modal--extraction [data-ed-action]')].filter(b=>!b.hidden).every(b=>{const r=b.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth && r.top>=0 && r.bottom<=innerHeight;})`)
	s.shot("extraction-uncertain-fruity-320")
	p.MustElement(`.ed-modal--extraction [data-ed-action="reconcile"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--extraction') && edFixture.editor().model.doc.nodes.some(n=>n.type==='component.call') && document.querySelector('[data-ed-extraction-status]').hidden`)
	p.MustEval(`id=>{const calls=edExtractionFixture.state.requests.filter(r=>r.flow_id===id);if(calls.filter(r=>r.action==='apply').length!==1 || calls.filter(r=>r.action==='reconcile').length!==1)throw Error('unexpected retry');}`, second)
	s.shot("extraction-reconciled")
	s.noRawKeys("extract selection")
	s.failOnPageErrors("extract selection")
}
