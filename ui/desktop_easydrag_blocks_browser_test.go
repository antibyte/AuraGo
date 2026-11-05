package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopEasyDragBlocksBrowser exercises the shared editor through real controls.
// The fixture retains immutable published documents independently of the editable draft.
func TestDesktopEasyDragBlocksBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js")
	p := s.page
	t.Cleanup(func() {
		if t.Failed() {
			if value, err := p.Eval(`()=>JSON.stringify({doc:edFixture.editor() && edFixture.editor().model.doc,errors:window.fixtureErrors})`); err == nil {
				t.Logf("block fixture editor state: %s", value.Value.Str())
			}
		}
	})
	selectValue := func(selector, value string) {
		t.Helper()
		p.MustEval(`(selector,value)=>{const select=document.querySelector(selector);select.focus();select.value=value;select.dispatchEvent(new Event('change',{bubbles:true}));}`, selector, value)
	}
	edit := func(selector, value string) {
		t.Helper()
		p.MustElement(selector).MustSelectAllText().MustInput(value)
		p.Keyboard.MustType(input.Tab)
	}
	closeDetail := func() {
		t.Helper()
		p.MustElement("[data-ed-detail-close]").MustClick()
		s.wait(`()=>!document.querySelector('.ed-detail')`)
	}
	publish := func(block bool) {
		t.Helper()
		p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-action="publish"]')`)
		if block {
			s.wait(`()=>!document.querySelector('[data-ed-activate]') && !!document.querySelector('.ed-block-contract')`)
		} else {
			p.MustEval(`()=>{const input=document.querySelector('[data-ed-activate]');if(input)input.checked=false;}`)
		}
		p.MustElement(`[data-ed-action="publish"]`).MustClick()
		s.wait(`()=>!document.querySelector('.ed-modal') && edFixture.editor().flow.live_revision>0`)
		s.clearToasts()
	}
	openFlow := func(id string) {
		t.Helper()
		p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, id)
		s.wait(`()=>{const ed=edFixture.editor();return ed && !ed.isBlock && ed.model.doc.nodes.some(n=>n.type==='component.call');}`)
	}
	openCall := func() {
		t.Helper()
		p.MustElement(`.ed-node[data-node-id="n_parentcall1"] .ed-node-label`).MustDoubleClick()
		s.wait(`()=>!!document.querySelector('[data-ed-call-block]')`)
	}
	moreAction := func(key string) {
		t.Helper()
		p.MustElement(`[data-ed-cmd="more"]`).MustClick()
		p.MustEval(`key=>{const label=edFixture.editor().t(key);[...document.querySelectorAll('.vd-context-menu button')].find(b=>b.textContent.includes(label)).click();}`, key)
	}
	simulateParent := func(expected string) {
		t.Helper()
		p.MustElement(`[data-ed-cmd="test"]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-test-mode]')`)
		selectValue(`[data-ed-test-mode]`, "simulation")
		p.MustElement(`[data-ed-action="run"]`).MustClick()
		s.wait(`()=>!document.querySelector('.ed-modal') && edFixture.editor().run && edFixture.editor().run.status==='success'`)
		p.MustEval(`value=>window.__blockExpected=value`, expected)
		s.wait(`()=>{const r=edFixture.editor().run;return [...r.steps.values()].some(step=>step.output && step.output.greeting===window.__blockExpected);}`)
	}

	s.wait(`()=>!!document.querySelector('.ed-block-library-link')`)
	t.Log("open block library")
	p.MustElement(".ed-block-library-link").MustClick()
	s.wait(`()=>!!document.querySelector('.ed-block-library')`)
	p.MustElement(`[data-ed-new]`).MustClick()
	p.MustElement(`[data-ed-block-name]`).MustInput("Begrüßung")
	p.MustElement(`[data-ed-action="create"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return ed && ed.isBlock && ed.model.doc.nodes.length===2;}`)
	blockID := p.MustEval(`()=>edFixture.editor().flow.id`).Str()
	t.Log("edit block contract", blockID)
	s.wait(`()=>edFixture.state.flows.size===1 && edBlockFixture.state.blocks.size===1 && !document.querySelector('[data-ed-cmd="active"]').getClientRects().length && !document.querySelector('[data-ed-cmd="runs"]').getClientRects().length`)

	// Native double click opens the localized input boundary; no raw JSON is required.
	p.MustElement(`.ed-node[data-node-id="n_blockinput01"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>!!document.querySelector('[data-ed-block-field-add]')`)
	p.MustElement(`[data-ed-block-field-add]`).MustClick()
	edit(`[data-ed-block-key="name"]`, "name")
	closeDetail()
	p.MustElement(`.ed-node[data-node-id="n_blockoutput1"] .ed-node-label`).MustDoubleClick()
	p.MustElement(`[data-ed-block-field-add]`).MustClick()
	edit(`[data-ed-block-key="name"]`, "greeting")
	s.wait(`()=>!!document.querySelector('.ed-field[data-param="values.greeting"]')`)
	selectValue(`.ed-field[data-param="values.greeting"] [data-ed-block-presence]`, "value")
	p.MustElement(`.ed-field[data-param="values.greeting"] .ed-tpl-view`).MustClick()
	p.MustElement(`.ed-field[data-param="values.greeting"] textarea`).MustInput("Hello ")
	p.MustElement(`[data-ed-narrow="input"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.type==='block.output').params.values.greeting==='Hello '`)
	s.wait(`()=>{const area=document.querySelector('.ed-field[data-param="values.greeting"] textarea');return area.value==='Hello ' && area.selectionStart===6 && area.selectionEnd===6;}`)
	selectValue(`[data-ed-mapping-target]`, "values.greeting")
	s.wait(`()=>!!document.querySelector('[data-ed-toggle="block_input.data"]')`)
	p.MustElement(`[data-ed-toggle="block_input.data"]`).MustClick()
	p.MustElement(`.ed-tree-row[data-ref="block_input.data.name"]`).MustClick()
	t.Log("mapped output", p.MustEval(`()=>JSON.stringify(edFixture.editor().model.doc.nodes.find(n=>n.type==='block.output').params.values)`).Str())
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.type==='block.output').params.values.greeting==='Hello {{block_input.data.name}}'`)
	closeDetail()
	s.shot("blocks-editor-contract")
	t.Log("simulate block draft")

	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-block-inputs]')`)
	selectValue(`.ed-field[data-param="name"] [data-ed-block-presence]`, "value")
	edit(`.ed-field[data-param="name"] input`, "Ada")
	p.MustElement(`[data-ed-action="simulate"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal') && !!document.querySelector('.ed-block-result') && document.querySelector('.ed-block-result').textContent.includes('Hello Ada') && !edFixture.editor().flow.live_revision`)
	s.wait(`()=>{const result=document.querySelector('.ed-run-results-body');return result.textContent.includes(edFixture.editor().t('easydrag.node.block_input.label')) && !result.textContent.includes('block.input') && result.textContent.includes(edFixture.editor().t('easydrag.ui.block_simulation_hint'));}`)
	s.shot("blocks-draft-simulation")
	publish(true)
	firstRevision := p.MustEval(`()=>edFixture.editor().flow.live_revision`).Int()
	if firstRevision != 1 {
		t.Fatalf("first immutable block revision = %d", firstRevision)
	}

	// Only graph scaffolding is fixture setup. Reference selection and named binding use UI.
	parents := p.MustEval(`()=>['Parent A','Parent B'].map(name=>{const id=edFixture.addEmptyDraft(name),doc=edFixture.flow(id).rec.draft;doc.nodes=[{id:'n_parentstart',type:'trigger.manual',type_version:1,key:'manual',label:'',params:{sample_data:{}},settings:{},position:{x:0,y:0}},{id:'n_parentcall1',type:'component.call',type_version:1,key:'greet',label:'',params:{},settings:{retry:{count:0}},position:{x:360,y:0}}];doc.edges=[{id:'e_parentlink',source:{node:'n_parentstart',port:'out'},target:{node:'n_parentcall1',port:'in'}}];return id;})`).Arr()
	for i, parent := range parents {
		openFlow(parent.Str())
		openCall()
		s.wait(`()=>document.querySelector('[data-ed-call-block]').options.length>1`)
		selectValue(`[data-ed-call-block]`, blockID)
		s.wait(`()=>document.querySelector('[data-ed-call-version]').options.length>1`)
		selectValue(`[data-ed-call-version]`, "1")
		p.MustElement(`[data-ed-call-apply]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-action="apply"]')`)
		p.MustElement(`[data-ed-action="apply"]`).MustClick()
		s.wait(`()=>!!document.querySelector('.ed-field[data-param="inputs.name"]')`)
		selectValue(`.ed-field[data-param="inputs.name"] [data-ed-block-presence]`, "value")
		p.MustElement(`.ed-field[data-param="inputs.name"] .ed-tpl-view`).MustClick()
		name := []string{"World", "Bob"}[i]
		edit(`.ed-field[data-param="inputs.name"] textarea`, name)
		if i == 0 {
			s.shot("blocks-parent-mapping-v1")
		}
		closeDetail()
		simulateParent("Hello " + name)
		publish(false)
	}

	// Updating the block leaves both published parents pinned to the earlier immutable version.
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{blockId:id})`, blockID)
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().isBlock`)
	p.MustElement(`.ed-node[data-node-id="n_blockoutput1"] .ed-node-label`).MustDoubleClick()
	p.MustElement(`.ed-field[data-param="values.greeting"] .ed-tpl-view`).MustClick()
	edit(`.ed-field[data-param="values.greeting"] textarea`, "Welcome {{block_input.data.name}}")
	closeDetail()
	publish(true)
	s.wait(`()=>edFixture.editor().flow.live_revision===2`)
	for i, parent := range parents {
		openFlow(parent.Str())
		s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.type==='component.call').params.revision===1`)
		simulateParent("Hello " + []string{"World", "Bob"}[i])
	}
	openFlow(parents[0].Str())
	openCall()
	s.wait(`()=>[...document.querySelector('[data-ed-call-version]').options].some(o=>o.value==='2')`)
	selectValue(`[data-ed-call-version]`, "2")
	p.MustElement(`[data-ed-call-apply]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-action="apply"]')`)
	s.shot("blocks-explicit-upgrade-review")
	p.MustElement(`[data-ed-action="apply"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.type==='component.call').params.revision===2`)
	closeDetail()
	simulateParent("Welcome World")
	p.MustEval(`id=>{const doc=edFixture.flow(id).rec.draft;if(doc.nodes.find(n=>n.type==='component.call').params.revision!==1)throw new Error('Other parent upgraded silently');}`, parents[1].Str())

	// Reopen the library through the public routing path, then verify deletion's real dependents.
	p.MustEval(`()=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{section:'blocks'})`)
	s.wait(`()=>!!document.querySelector('.ed-block-library')`)
	p.MustElement(`[data-ed-block-open]`).MustClick()
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().isBlock`)
	moreAction("easydrag.ui.block_delete")
	s.wait(`()=>document.querySelectorAll('.ed-block-dependents li').length===2 && document.querySelector('[data-ed-action="delete"]').disabled`)
	s.shot("blocks-delete-dependencies")
	p.MustElement(`[data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal')`)
	moreAction("easydrag.ui.versions_title")
	s.wait(`()=>document.querySelectorAll('[data-ed-block-version]').length===2`)
	p.MustElement(`[data-ed-block-version="1"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return ed && ed.isBlock && ed.readonly && ed.model.doc.nodes.find(n=>n.type==='block.output').params.values.greeting==='Hello {{block_input.data.name}}';}`)
	s.wait(`()=>document.querySelector('[data-ed-cmd="publish"]').disabled && document.querySelector('[data-ed-cmd="test"]').disabled`)
	moreAction("easydrag.ui.block_open_draft")
	s.wait(`()=>{const ed=edFixture.editor();return ed && ed.isBlock && !ed.readonly && ed.model.doc.nodes.find(n=>n.type==='block.output').params.values.greeting==='Welcome {{block_input.data.name}}';}`)

	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		p.MustEval(`theme=>fixtureTheme(theme)`, theme)
		p.MustSetViewport(390, 844, 1, true)
		p.MustElement(`[data-ed-cmd="test"]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-block-inputs]')`)
		s.wait(`()=>{const notes=document.querySelector('[data-ed-block-inputs] .ed-details');return notes && !notes.open && notes.textContent.includes(edFixture.editor().t('easydrag.ui.block_sample_limit'));}`)
		s.wait(`()=>{const d=document.querySelector('.ed-modal');return d.scrollWidth<=d.clientWidth+1 && [...d.querySelectorAll('button,select,input')].filter(e=>e.getClientRects().length).every(e=>e.getBoundingClientRect().right<=innerWidth+1);}`)
		s.shot("blocks-simulation-390-" + theme)
		if theme == "standard" {
			p.MustElement(`[data-ed-block-inputs] .ed-details summary`).MustClick()
			s.wait(`()=>document.querySelector('[data-ed-block-inputs] .ed-details').open`)
			s.shot("blocks-simulation-limits-390")
			p.MustElement(`[data-ed-block-inputs] .ed-details summary`).MustClick()
		}
		p.MustSetViewport(320, 760, 1, true)
		s.wait(`()=>{const d=document.querySelector('.ed-modal');return d.scrollWidth<=d.clientWidth+1 && d.getBoundingClientRect().right<=innerWidth+1;}`)
		s.wait(`()=>{const head=document.querySelector('.ed-head'),test=head.querySelector('[data-ed-cmd="test"]');return head.scrollWidth<=head.clientWidth+1 && test.getBoundingClientRect().right<=innerWidth+1;}`)
		p.MustElement(`[data-ed-action="cancel"]`).MustClick()
	}
	s.noRawKeys("blocks journey")
	s.failOnPageErrors("blocks journey")
}
