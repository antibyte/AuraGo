package ui

import (
	"fmt"
	"testing"
)

// TestDesktopEasyDragBatchesBrowser exercises the batch editor and pure simulation controls.
func TestDesktopEasyDragBatchesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-batches-fixture.js")
	p := s.page
	t.Cleanup(func() {
		if t.Failed() {
			if value, err := p.Eval(`()=>JSON.stringify({doc:edFixture.editor()?.model.doc,requests:edBatchFixture.requests,errors:window.fixtureErrors})`); err == nil {
				t.Logf("batch fixture state: %s", value.Value.Str())
			}
		}
	})
	selectValue := func(selector, value string) {
		t.Helper()
		p.MustEval(`(selector,value)=>{const e=document.querySelector(selector);if(!e)throw Error('Missing '+selector);e.value=value;e.dispatchEvent(new Event('change',{bubbles:true}));}`, selector, value)
	}
	closeDetail := func() {
		t.Helper()
		// The outgoing inspector remains in the DOM for its close transition.
		p.MustElement(`.ed-detail-backdrop:not(.is-closing) [data-ed-detail-close]`).MustClick()
		s.wait(`()=>!document.querySelector('.ed-detail')`)
	}
	// Setup supplies a published pure block; reference choice and item mapping are exercised below.
	blockID := p.MustEval(`async()=>{
		const api='/api/desktop/flow-blocks', call=async(path,method,body)=>{const r=await fetch(path,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw Error(await r.text());return r.json();};
		const rec=(await call(api,'POST',{name:'Greeting'})).block;
		for(const n of rec.draft.nodes){if(n.type==='block.input')n.params.fields=[{name:'name',type:'string',required:true,nullable:false},{name:'suffix',type:'string',required:true,nullable:false}];if(n.type==='block.output'){n.params.fields=[{name:'greeting',type:'string',required:true,nullable:false}];n.params.values={greeting:'{{block_input.data.name}} {{block_input.data.suffix}}'};}}
		await call(api+'/'+rec.id,'PUT',{doc:rec.draft,base_revision:1});await call(api+'/'+rec.id+'/publish','POST',{base_revision:2});return rec.id;
	}`).Str()
	parentID := p.MustEval(`()=>{
		const id=edFixture.addEmptyDraft('Batch parent'),doc=edFixture.flow(id).rec.draft;
		doc.nodes=[{id:'n_batchstart',type:'trigger.manual',type_version:1,key:'manual',label:'',params:{},settings:{},position:{x:0,y:0}},{id:'n_batchstep1',type:'component.each',type_version:1,key:'batch',label:'',params:{items:'{{trigger.data.items}}',item_field:'',inputs:{},max_items:10,concurrency:1,on_item_error:'collect'},settings:{retry:{count:0}},position:{x:360,y:0}}];
		doc.edges=[{id:'e_batchlink1',source:{node:'n_batchstart',port:'out'},target:{node:'n_batchstep1',port:'in'}}];return id;
	}`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, parentID)
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().model.doc.nodes.some(n=>n.type==='component.each')`)
	p.MustElement(`.ed-node[data-node-id="n_batchstep1"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>document.querySelector('[data-ed-call-block]')?.options.length>1`)
	p.MustElement(`.ed-field[data-param="items"] .ed-tpl-view`).MustClick()
	p.MustElement(`.ed-field[data-param="items"] textarea`).MustSelectAllText().MustInput(`{{trigger.data.items}}`)
	selectValue(`[data-ed-call-block]`, blockID)
	s.wait(`()=>document.querySelector('[data-ed-call-version]')?.options.length>1`)
	selectValue(`[data-ed-call-version]`, "1")
	p.MustElement(`[data-ed-call-apply]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-action="apply"]')`)
	p.MustElement(`[data-ed-action="apply"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-batch-item-field]')`)
	selectValue(`[data-ed-batch-item-field]`, "name")
	s.wait(`()=>!!document.querySelector('.ed-field[data-param="inputs.suffix"]') && !document.querySelector('.ed-field[data-param="inputs.name"]')`)
	selectValue(`.ed-field[data-param="inputs.suffix"] [data-ed-block-presence]`, "value")
	p.MustElement(`.ed-field[data-param="inputs.suffix"] .ed-tpl-view`).MustClick()
	p.MustElement(`.ed-field[data-param="inputs.suffix"] textarea`).MustInput("hello")
	for _, theme := range []string{"standard", "fruity-light"} {
		for _, width := range []int{390, 1440} {
			p.MustSetViewport(width, 900, 1, width < 760)
			p.MustEval(`theme=>fixtureTheme(theme)`, theme)
			p.MustEval(`()=>{const column=document.querySelector('.ed-detail-col[data-col="params"]');if(column)column.scrollTop=0;}`)
			s.shot(fmt.Sprintf("batch-editor-%s-%d", theme, width))
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	closeDetail()
	simulate := func(data string) {
		t.Helper()
		p.MustEval(`()=>window.__batchRequestCount=edBatchFixture.requests.length`)
		p.MustElement(`[data-ed-cmd="test"]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-test-mode]')`)
		selectValue(`[data-ed-test-mode]`, "simulation")
		p.MustEval(`()=>{const area=document.querySelector('[data-ed-test-data]'),details=area.closest('details');if(details && !details.open)details.querySelector('summary').click();}`)
		p.MustElement(`[data-ed-test-data]`).MustSelectAllText().MustInput(data)
		p.MustEval(`data=>{if(document.querySelector('[data-ed-test-data]').value!==data)throw Error('Sample input not applied');}`, data)
		p.MustElement(`[data-ed-action="run"]`).MustClick()
		s.wait(`()=>edBatchFixture.requests.length>window.__batchRequestCount && !document.querySelector('.ed-modal')`)
	}
	simulate(`{"items":["Ada",5,"Lin"]}`)
	s.wait(`()=>{const r=edFixture.editor().run;if(!r || r.mode!=='simulation')return false;const step=r.steps.get('n_batchstep1');return step && step.output && step.output.count===3 && step.output.succeeded===2 && step.output.failed===1;}`)
	s.wait(`()=>!!document.querySelector('[data-ed-batch-summary]')`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:'n_batchstep1',column:'output'})`)
	s.wait(`()=>!!document.querySelector('[data-ed-batch-results]')`)
	s.shot("batch-collect-result")
	p.MustEval(`()=>{const item=edFixture.editor().run.steps.get('n_batchstep1').output.items;if(item[0].output.greeting!=='Ada hello'||item[1].status!=='error'||item[2].output.greeting!=='Lin hello')throw Error('Batch item order or output');}`)
	closeDetail()
	p.MustElement(`.ed-node[data-node-id="n_batchstep1"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>!!document.querySelector('.ed-field[data-param="on_item_error"] select')`)
	selectValue(`.ed-field[data-param="on_item_error"] select`, "stop")
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.id==='n_batchstep1').params.on_item_error==='stop'`)
	closeDetail()
	simulate(`{"items":["Ada",5,"Lin"]}`)
	s.wait(`()=>{const r=edFixture.editor().run,step=r && r.steps.get('n_batchstep1');return step && step.status==='error' && step.error_code==='FLOW_BATCH_ITEM_FAILED' && !step.output;}`)
	s.shot("batch-stop-result")
	p.MustElement(`.ed-node[data-node-id="n_batchstep1"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>!!document.querySelector('.ed-field[data-param="on_item_error"] select')`)
	selectValue(`.ed-field[data-param="on_item_error"] select`, "collect")
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.id==='n_batchstep1').params.on_item_error==='collect'`)
	closeDetail()
	simulate(`{"items":[]}`)
	s.wait(`()=>{const r=edFixture.editor().run,step=r && r.steps.get('n_batchstep1');return step && step.output && step.output.count===0 && step.output.items.length===0;}`)

	// Publishing a newer child leaves the old reference intact; upgrading requires review and is undoable.
	p.MustEval(`async id=>{const base='/api/desktop/flow-blocks/'+id,old=await (await fetch(base)).json(),doc=old.block.draft;doc.nodes.find(n=>n.type==='block.output').params.values.greeting='new {{block_input.data.name}} {{block_input.data.suffix}}';for(const [path,method,body] of [[base,'PUT',{doc,base_revision:old.block.draft_revision}],[base+'/publish','POST',{base_revision:old.block.draft_revision+1}]]){const r=await fetch(path,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw Error(await r.text());}}`, blockID)
	simulate(`{"items":["Ada"]}`)
	s.wait(`()=>{const r=edFixture.editor().run,step=r && r.steps.get('n_batchstep1');return step && step.output && step.output.items[0]?.output.greeting==='Ada hello';}`)
	p.MustElement(`.ed-node[data-node-id="n_batchstep1"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>[...document.querySelector('[data-ed-call-version]').options].some(o=>o.value==='2')`)
	selectValue(`[data-ed-call-version]`, "2")
	p.MustElement(`[data-ed-call-apply]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-action="apply"]')`)
	p.MustElement(`[data-ed-action="apply"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.id==='n_batchstep1').params.revision===2`)
	closeDetail()
	p.MustElement(`[data-ed-history="undo"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.id==='n_batchstep1').params.revision===1`)

	// An immutable stored-run fixture retains an explicit truncation marker and read-only controls.
	p.MustEval(`()=>{const ed=edFixture.editor(),id='run_batchfixture',now=new Date().toISOString(),doc=JSON.parse(JSON.stringify(ed.model.doc));const run={id,flow_id:ed.flow.id,revision:ed.saver.revision,mode:'test',trigger_node:'n_batchstart',trigger_type:'test',status:'success',started_at:now,finished_at:now};const steps=[{node_id:'n_batchstep1',node_key:'batch',status:'success',output:{items:[{index:0,status:'success',output:{greeting:'Ada hello'}}],count:2,succeeded:2,failed:0},output_truncated:true}];edFixture.state.runs.set(id,{run,doc,steps,events:[],listeners:new Set(),done:true});EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:ed.flow.id,runId:id});}`)
	s.wait(`()=>!!edFixture.editor().runView`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:'n_batchstep1',column:'output'})`)
	s.wait(`()=>{const d=document.querySelector('.ed-detail');return d && d.textContent.includes(edFixture.editor().t('easydrag.ui.batch_result_incomplete'));}`)
	s.shot("batch-run-truncated")
	closeDetail()
	p.MustElement(`.ed-node[data-node-id="n_batchstep1"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>{const e=document.querySelector('[data-ed-batch-item-field]');return e && e.disabled;}`)
	s.noRawKeys("batch journey")
	s.failOnPageErrors("batch journey")
}
