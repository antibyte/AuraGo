package ui

import (
	"fmt"
	"testing"
)

// TestDesktopEasyDragChildHistoryBrowser uses the real editor with a bounded local run fixture.
// Collector, storage and pin verification are covered by the server/runtime integration tests.
func TestDesktopEasyDragChildHistoryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-batches-fixture.js", "/testdata/easydrag-child-history-fixture.js")
	p := s.page
	sample := p.MustEval(`()=>edChildHistoryFixture.seedSample()`).Map()
	flowID, runID := sample["flow_id"].Str(), sample["run_id"].Str()
	p.MustEval(`x=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:x.flow,runId:x.run})`, map[string]string{"flow": flowID, "run": runID})
	s.wait(`()=>edFixture.editor()?.run?.id==='` + runID + `' && !!edFixture.editor().run.steps.get('n_bbbbbbbb')`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:'n_bbbbbbbb',column:'output'})`)
	s.wait(`()=>!!document.querySelector('[data-ed-child-open]')`)
	p.MustElement(`[data-ed-child-open]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_1"]')`)
	p.MustElement(`[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_1"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] [data-ed-child-record="1"]') && document.querySelector('[data-ed-child-history]').textContent.includes('Outer block')`)
	p.MustElement(`[data-ed-child-history] [data-ed-child-record="1"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-child-history]')?.textContent.includes('Hello Ada')`)
	for _, theme := range []string{"standard", "fruity-light"} {
		for _, width := range []int{1440, 390, 320} {
			p.MustSetViewport(width, 900, 1, width < 760)
			p.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.wait(`()=>{const x=document.querySelector('[data-ed-child-history]');if(!x)return false;const r=x.getBoundingClientRect();return r.left>=-1 && r.right<=innerWidth+1 && x.scrollWidth<=x.clientWidth+1;}`)
			if width < 760 {
				// The selected value and navigation must fit without scrolling the test.
				s.wait(`()=>{const visible=e=>{if(!e)return false;const r=e.getBoundingClientRect();if(r.width<=0||r.height<=0||r.top<0||r.left<0||r.bottom>innerHeight||r.right>innerWidth)return false;for(let p=e.parentElement;p;p=p.parentElement){const s=getComputedStyle(p),b=p.getBoundingClientRect();if(/auto|scroll|hidden|clip/.test(s.overflowY)&&(r.top<b.top-1||r.bottom>b.bottom+1))return false;if(/auto|scroll|hidden|clip/.test(s.overflowX)&&(r.left<b.left-1||r.right>b.right+1))return false;}return true;};return visible(document.querySelector('[data-ed-child-value]')) && visible(document.querySelector('[data-ed-child-back]')) && visible(document.querySelector('.ed-modal--children [data-ed-action="close"]'));}`)
			}
			s.shot(fmt.Sprintf("child-history-record-%s-%d", theme, width))
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustElement(`[data-ed-child-history] [data-ed-child-back]`).MustClick()
	p.MustElement(`[data-ed-child-history] [data-ed-child-calls]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_2"]')`)
	p.MustElement(`[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_2"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] [data-ed-child-record="1"]') && document.querySelector('[data-ed-child-history]').textContent.includes('Leaf block')`)
	p.MustElement(`[data-ed-child-history] [data-ed-child-record="1"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-child-history]')?.textContent.includes('Hello Ada')`)
	s.shot("child-history-nested-record")
	// A missing pinned document must not hide retained record values.
	p.MustEval(`id=>{const docs=edChildHistoryFixture.state.documents,key=id+'/12/i_2',doc=docs.get(key);delete doc.doc;doc.doc_missing='unavailable';}`, runID)
	p.MustElement(`[data-ed-child-history] [data-ed-child-reload]`).MustClick()
	s.wait(`()=>{const x=document.querySelector('[data-ed-child-history] [data-ed-child-document-missing]');return !!x && !x.hidden && document.querySelector('[data-ed-child-history]').textContent.includes('Hello Ada');}`)
	s.shot("child-history-document-missing")
	// Losing read access clears retained trace/document contents without a mutation.
	p.MustEval(`()=>{edChildHistoryFixture.state.denied=true;}`)
	p.MustElement(`[data-ed-child-history] [data-ed-child-reload]`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-child-history] .ed-child-row') && !document.querySelector('[data-ed-child-history]')?.textContent.includes('Hello Ada')`)
	p.MustEval(`()=>{const r=edChildHistoryFixture.state.requests;if(r.filter(x=>x.path.endsWith('/children')).length!==3 || r.some(x=>x.method!=='GET'))throw Error('Unexpected history request or mutation');}`)
	// The same viewer handles inline batch simulation without child GETs or stored runs.
	p.MustEval(`()=>{edChildHistoryFixture.state.denied=false;window.__childReads=edChildHistoryFixture.state.requests.length;}`)
	simulationID := p.MustEval(`()=>edChildHistoryFixture.seedSimulation()`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, simulationID)
	s.wait(`()=>edFixture.editor()?.flow.id==='` + simulationID + `' && !edFixture.editor().run`)
	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-test-mode]')`)
	p.MustEval(`()=>{const e=document.querySelector('[data-ed-test-mode]');e.value='simulation';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`[data-ed-action="run"]`).MustClick()
	s.wait(`()=>edFixture.editor()?.run?.mode==='simulation' && !!edFixture.editor().run.steps.get('n_bbbbbbbb')`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:'n_bbbbbbbb',column:'output'})`)
	s.wait(`()=>!!document.querySelector('[data-ed-child-open]')`)
	p.MustElement(`[data-ed-child-open]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_1"]')`)
	p.MustEval(`()=>{const row=document.querySelector('[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_1"]');if(!row.textContent.includes(edFixture.editor().t('easydrag.ui.child_history_item',{number:1})))throw Error('Batch item zero is not shown');}`)
	p.MustElement(`[data-ed-child-history] .ed-child-row[data-ed-child-frame="i_1"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-child-history] [data-ed-child-record="1"]') && document.querySelector('[data-ed-child-history]').textContent.includes('Outer block')`)
	p.MustElement(`[data-ed-child-history] [data-ed-child-record="1"]`).MustClick()
	s.wait(`()=>{const x=document.querySelector('[data-ed-child-history]'),t=edFixture.editor().t;return x.textContent.includes(t('easydrag.ui.child_history_incomplete')) && x.textContent.includes(t('easydrag.ui.child_history_truncated'));}`)
	p.MustEval(`()=>{if(edChildHistoryFixture.state.requests.length!==window.__childReads || edChildHistoryFixture.state.simulationRequests.length!==1)throw Error('Simulation fetched persistent child history');}`)
	s.shot("child-history-inline-batch-omitted")
	s.noRawKeys("child history")
	s.failOnPageErrors("child history")
}
