package ui

import (
	"strings"
	"testing"
)

func TestDesktopEasyDragBoundaryRegressionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-durable-fixture.js", "/testdata/easydrag-approval-fixture.js")
	p := s.page
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>!!edFixture.editor()?.model.doc.nodes.length`)

	wanted := strings.Repeat("a", 79) + "😀"
	p.MustEval(`x=>{const ed=edFixture.editor(),id=ed.model.doc.nodes[0].id;ed.bus.emit('open-detail',{nodeId:id});const input=document.querySelector('.ed-detail-label');if(!input||input.maxLength!==160)throw Error('The 80-code-point label input must allow 160 UTF-16 units');input.value=x.label;input.dispatchEvent(new Event('input',{bubbles:true}));if(ed.model.node(id).label!==x.expected)throw Error('Editing cut an astral label in half');const ids=ed.model.paste({easydrag:1,nodes:[{id:'clip',key:'clip',type:'trigger.manual',type_version:1,label:x.label,position:{x:0,y:0},params:{},settings:{}}],edges:[]},{x:900,y:0});if(ids.length!==1||ed.model.node(ids[0]).label!==x.expected)throw Error('Import sanitization cut an astral label in half');}`, map[string]string{"label": wanted + "z", "expected": wanted})

	p.MustEval(`()=>{edFixture.state.delay=2500;edFixture.state.malformedSnapshotOnce=true;edFixture.state.malformedSSEAfter=1;edFixture.state.malformedSSESent=false}`)
	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>edFixture.editor()?.run?.status==='success'`)
	s.wait(`()=>edFixture.state.streams.some(stream=>/[?&]after=1(?:&|$)/.test(stream.url))`)
	p.MustEval(`()=>{const posts=edFixture.state.requests.filter(request=>request.startsWith('POST ')&&request.endsWith('/test'));if(posts.length!==1)throw Error('Malformed stream frames replayed a run POST');}`)

	durable := p.MustEval(`()=>edDurableFixture.seed('Laufzeit nicht verfügbar',false,false,'runtime_unavailable')`).Map()
	p.MustEval(`x=>{const id=Array.from(EasyDragApp._instances.values())[0].windowId;EasyDragApp.open(id,{flowId:x.flow,runId:x.run});}`, map[string]string{"flow": durable["flow_id"].Str(), "run": durable["run_id"].Str()})
	s.wait(`()=>{const ed=edFixture.editor(),status=document.querySelector('[data-ed-durable-run]');return ed&&ed.runView?.run.id==='` + durable["run_id"].Str() + `'&&status?.textContent.includes(ed.t('easydrag.ui.durable_reason_runtime_unavailable'))}`)

	approval := p.MustEval(`()=>edApprovalFixture.seed(true)`).Map()
	p.MustEval(`x=>{const id=Array.from(EasyDragApp._instances.values())[0].windowId;EasyDragApp.open(id,{flowId:edFixture.seededID,runId:x.run});}`, map[string]string{"run": approval["run1"].Str()})
	s.wait(`()=>edFixture.editor()?.runView?.run.id==='` + approval["run1"].Str() + `'`)
	p.MustElement(`.ed-runview-banner [data-ed-cmd="approval-review"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-modal--approval-review')?.textContent.includes(edFixture.editor().t('easydrag.ui.approval_blocked_runtime_unavailable'))`)
	p.MustEval(`()=>{const dialog=document.querySelector('.ed-modal--approval-review');if(dialog.querySelector('[data-ed-approval-start]')||edApprovalFixture.state.requests.some(request=>request.method==='POST'))throw Error('Unavailable runtime exposed or submitted an approval decision');}`)
	s.noRawKeys("EasyDrag boundary regressions")
	s.failOnPageErrors("EasyDrag boundary regressions")
}
