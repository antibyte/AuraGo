package ui

import (
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragDecisionBrowser covers the decision node's rendered forms and stable
// dynamic ports in the actual Desktop shell, including draft persistence and flow interchange.
func TestDesktopEasyDragDecisionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	page := s.page
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	flowID := page.MustEval(`()=>{const id=edFixture.addEmptyDraft('Decision browser check');window.__flowID=id;const inst=Array.from(EasyDragApp._instances.values())[0];EasyDragApp.open(inst.windowId,{flowId:id});return id;}`).Str()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed && ed.flow.id===` + fmt.Sprintf("%q", flowID) + ` && !!document.querySelector('.ed-canvas')}`)
	page.MustEval(`()=>{const ed=edFixture.editor();const trigger=ed.model.addNode('trigger.manual',{x:80,y:180});const decision=ed.model.addNode('ai.decision',{x:390,y:180},{mode:'route',state:'{{trigger.data}}',questions:[{id:'intent',type:'choice',instructions:'Choose the best option.',options:[{id:'accept',label:'Accept'},{id:'reject',label:'Reject'}]}],images:[],confidence_threshold:0.70});ed.model.connect(trigger,'out',decision,'in');const branch=ed.model.addNode('logic.if',{x:760,y:180},{condition:{match:'all',rules:[]}});ed.model.connect(decision,'choice_accept',branch,'in');window.__decisionIDs={trigger,decision,branch};ed.bus.emit('open-detail',{nodeId:decision});}`)
	s.wait(`()=>document.querySelectorAll('.ed-node').length===3 && !!document.querySelector('.ed-detail .ed-field[data-param="questions"] .ed-decision-card')`)
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision);return ed.model.outputs(n).join(',')==='choice_accept,choice_reject,uncertain' && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept')}`)
	s.wait(`()=>{const id=window.__decisionIDs.decision;return [...document.querySelectorAll('.ed-node[data-node-id="'+id+'"] [data-ed-port][data-ed-side="out"]')].map(p=>p.dataset.edPort).join(',')==='choice_accept,choice_reject,uncertain'}`)
	s.noRawKeys("decision route form")
	// Score values are only valid for score questions; converting back to choice must retain option
	// identity/labels and the connected output while removing the now-hidden score values.
	page.MustEval(`()=>{const sel=document.querySelector('.ed-detail [data-ed-decision-type]');sel.value='score';sel.dispatchEvent(new Event('change',{bubbles:true}));document.querySelectorAll('.ed-detail [data-ed-decision-score]').forEach((field,index)=>{field.value=String(index+1);field.dispatchEvent(new Event('change',{bubbles:true}));});}`)
	s.wait(`()=>{const n=edFixture.editor().model.node(window.__decisionIDs.decision),q=n.params.questions[0];return q.type==='score' && q.options[0].value===1 && q.options[1].value===2}`)
	page.MustEval(`()=>{const sel=document.querySelector('.ed-detail [data-ed-decision-type]');sel.value='choice';sel.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision),q=n.params.questions[0];return q.type==='choice' && q.options.map(o=>o.id+':'+o.label).join(',')==='accept:Accept,reject:Reject' && q.options.every(o=>!Object.prototype.hasOwnProperty.call(o,'value')) && ed.model.outputs(n).join(',')==='choice_accept,choice_reject,uncertain' && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept') && document.querySelectorAll('.ed-detail [data-ed-decision-score]').length===0}`)
	page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)

	// The fixture returns a structured per-question answer, so the actual detail output renderer
	// exercises nested probabilities, metadata, and the selected route result.
	page.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success' && ed.run.steps.has(window.__decisionIDs.decision)}`)
	s.clearToasts()
	page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('open-detail',{nodeId:window.__decisionIDs.decision});}`)
	s.wait(`()=>!!document.querySelector('.ed-detail') && !!document.querySelector('.ed-detail [data-ed-view="json"]')`)
	page.MustElement(`.ed-detail [data-ed-view="json"]`).MustClick()
	s.wait(`()=>{const out=document.querySelector('.ed-detail .ed-output-json');return !!out && out.textContent.includes('probabilities') && out.textContent.includes('accept') && out.textContent.includes('0.84') && out.textContent.includes('fixture-decisions')}`)
	s.shot("decision-route-standard-wide")
	s.noRawKeys("decision result")

	// Every theme/viewport combination renders both the route and evaluate forms without horizontal
	// overflow. A phone-sized viewport uses the same real editor and detail components.
	for _, theme := range []string{"standard", "fruity-light"} {
		for _, width := range []int{1440, 390} {
			if width < 600 {
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page); err != nil {
					t.Fatal(err)
				}
			}
			page.MustSetViewport(width, 844, 1, width < 600)
			page.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.wait(`()=>document.querySelector('.ed-detail') && document.querySelector('.ed-app').getBoundingClientRect().width>0`)
			s.wait(`()=>{const win=document.querySelector('.vd-window'),box=win.getBoundingClientRect(),detail=document.querySelector('.ed-detail').getBoundingClientRect(),title=document.querySelector('.ed-detail-title').getBoundingClientRect(),status=document.querySelector('.ed-detail-status'),statusBox=status.getBoundingClientRect(),test=document.querySelector('.ed-detail [data-ed-detail-test]').getBoundingClientRect(),close=document.querySelector('.ed-detail [data-ed-detail-close]').getBoundingClientRect(),form=document.querySelector('.ed-detail .ed-field[data-param="questions"]').getBoundingClientRect(),mobile=innerWidth<600,portal=document.querySelector('.ed-detail-backdrop.ed-detail-portal');return (mobile?win.classList.contains('vd-mobile-wide-window')&&box.width>innerWidth+300&&!!portal:!portal) && detail.left>=-1 && detail.right<=innerWidth+1 && detail.width<=innerWidth+1 && close.left>=-1 && close.right<=innerWidth+1 && test.left>=-1 && test.right<=innerWidth+1 && form.left>=-1 && form.right<=innerWidth+1 && (!mobile || !status.textContent.trim() || statusBox.top>=title.bottom-1)}`)
			s.shot(fmt.Sprintf("decision-route-%s-%d", theme, width))
		}
	}
	_ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`()=>fixtureTheme('standard')`)
	page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)

	// Reordering keeps an edge by choice ID. Removing that choice keeps the now-invalid edge so the
	// server validation error stays visible until the user explicitly deletes the connection.
	page.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__decisionIDs.decision})`)
	s.wait(`()=>!!document.querySelector('.ed-detail .ed-field[data-param="questions"]')`)
	page.MustElement(`.ed-detail [data-ed-decision-option-down="0"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision);return n.params.questions[0].options.map(o=>o.id).join(',')==='reject,accept' && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept')}`)
	page.MustElement(`.ed-detail [data-ed-decision-option-remove="1"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor(),id=window.__decisionIDs.decision,n=ed.model.node(id);return ed.model.outputs(n).join(',')==='choice_reject,uncertain' && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept') && !document.querySelector('.ed-node[data-node-id="'+id+'"] [data-ed-port="choice_accept"]') && ed.issues.some(i=>i.code==='EDGE_PORT_INVALID') && !!document.querySelector('.ed-foot-item.has-errors')}`)
	page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
	page.MustElement(`[data-ed-cmd="issues"]`).MustClick()
	s.wait(`()=>[...document.querySelectorAll('.ed-issues-popover .ed-issue-text')].some(issue=>issue.textContent.includes(edFixture.editor().t('easydrag.ui.issue_edge_port_invalid')))`)
	page.Keyboard.MustType(input.Escape)
	page.MustEval(`()=>{const ed=edFixture.editor(),edge=ed.model.outgoing(window.__decisionIDs.decision)[0];document.querySelector('.ed-edge[data-edge-id="'+edge.id+'"]').dispatchEvent(new MouseEvent('click',{bubbles:true}));}`)
	page.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	page.Keyboard.MustType(input.Delete)
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision);return !ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept') && !ed.issues.some(i=>i.code==='EDGE_PORT_INVALID')}`)
	page.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__decisionIDs.decision})`)
	s.wait(`()=>!!document.querySelector('.ed-detail .ed-field[data-param="questions"]')`)
	page.MustElement(`.ed-detail [data-ed-decision-add-option]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-detail [data-ed-decision-option-id]').length===2`)
	page.MustEval(`()=>{const id=window.__decisionIDs.decision;const field=document.querySelectorAll('.ed-detail [data-ed-decision-option-id]')[1];field.value='accept';field.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>edFixture.editor().model.outputs(edFixture.editor().model.node(window.__decisionIDs.decision)).includes('choice_accept')`)
	page.MustEval(`()=>edFixture.editor().model.connect(window.__decisionIDs.decision,'choice_accept',window.__decisionIDs.branch,'in')`)
	page.MustEval(`()=>{const sel=document.querySelector('.ed-detail [data-ed-decision-type]');sel.value='noul';sel.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision);return n.params.questions[0].type==='noul' && ed.model.outputs(n).join(',')==='choice_no,choice_yes,uncertain' && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_accept') && ed.issues.some(i=>i.code==='EDGE_PORT_INVALID')}`)
	page.MustEval(`()=>{const ed=edFixture.editor();ed.model.disconnect(ed.model.outgoing(window.__decisionIDs.decision).map(e=>e.id));ed.model.connect(window.__decisionIDs.decision,'choice_yes',window.__decisionIDs.branch,'in')}`)
	page.MustElement(`.ed-detail .ed-field[data-param="mode"] [data-value="evaluate"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision),fields=ed.model.fieldsOf(n).map(f=>f.name);return n.params.mode==='evaluate' && ed.model.outputs(n).join(',')==='out,uncertain' && fields.includes('answers') && !fields.includes('decision') && !fields.includes('confidence') && ed.model.outgoing(n.id).some(e=>e.source.port==='choice_yes') && ed.issues.some(i=>i.code==='EDGE_PORT_INVALID')}`)
	page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
	page.MustEval(`()=>{const ed=edFixture.editor(),edge=ed.model.outgoing(window.__decisionIDs.decision)[0];document.querySelector('.ed-edge[data-edge-id="'+edge.id+'"]').dispatchEvent(new MouseEvent('click',{bubbles:true}));}`)
	page.Keyboard.MustType(input.Delete)
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__decisionIDs.decision);return !ed.model.outgoing(n.id).length && !ed.issues.some(i=>i.code==='EDGE_PORT_INVALID')}`)
	page.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__decisionIDs.decision})`)
	s.wait(`()=>!!document.querySelector('.ed-detail .ed-field[data-param="questions"]')`)
	page.MustElement(`.ed-detail [data-ed-decision-add-question]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-detail .ed-decision-card').length===2`)
	page.MustEval(`()=>{const id=document.querySelectorAll('.ed-detail [data-ed-decision-question-id]')[1];id.value='priority';id.dispatchEvent(new Event('change',{bubbles:true}));const sel=document.querySelectorAll('.ed-detail [data-ed-decision-type]')[1];sel.value='score';sel.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>document.querySelectorAll('.ed-detail [data-ed-decision-score]').length===2`)
	page.MustEval(`()=>{const value=document.querySelector('.ed-detail [data-ed-decision-score]');value.value='2.5';value.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('.ed-detail [data-ed-decision-add-image]').click();}`)
	s.wait(`()=>document.querySelectorAll('.ed-detail .ed-decision-image').length===1`)
	page.MustElement(`.ed-detail .ed-decision-image .ed-tpl-view`).MustClick()
	page.MustElement(`.ed-detail .ed-decision-image textarea`).MustInput("media:123")
	page.MustElement(`.ed-detail [data-ed-pane="settings"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.node(window.__decisionIDs.decision).params.images[0]==='media:123'`)
	s.wait(`()=>!!document.querySelector('.ed-detail [data-pane="settings"] [data-value="error_port"]')`)
	if got := page.MustEval(`()=>[...document.querySelectorAll('.ed-detail [data-pane="settings"] [data-value]')].map(b=>b.dataset.value).join(',')`).Str(); got != "stop,error_port" {
		t.Fatalf("decision error policy choices = %q, want stop,error_port", got)
	}
	page.MustElement(`.ed-detail [data-pane="settings"] [data-value="error_port"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.outputs(edFixture.editor().model.node(window.__decisionIDs.decision)).join(',')==='out,uncertain,error'`)
	page.MustEval(`()=>{edFixture.state.decisionResult={answers:{intent:{type:'noul',decision:'yes',probabilities:{no:0.05,yes:0.95},confidence:0.95,noul:0.95},priority:{type:'score',decision:'high',probabilities:{low:0.1,high:0.9},confidence:0.9,score:8.9}},model:'fixture-decisions-multi',model_revision:'fixture-r2',calibration:'fixture',runtime:{deployment:'fixture',backend:'scripted',device:'cpu'},usage:{questions:2,candidates:4,images:1}};}`)
	page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-detail-backdrop')`)
	page.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success' && ed.run.steps.has(window.__decisionIDs.decision)}`)
	s.clearToasts()
	page.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__decisionIDs.decision})`)
	s.wait(`()=>!!document.querySelector('.ed-detail [data-ed-view="json"]')`)
	page.MustElement(`.ed-detail [data-ed-view="json"]`).MustClick()
	s.wait(`()=>{const out=document.querySelector('.ed-detail .ed-output-json');return !!out && out.textContent.includes('priority') && out.textContent.includes('score') && out.textContent.includes('8.9') && out.textContent.includes('fixture-decisions-multi')}`)
	s.shot("decision-evaluate-result-standard-wide")
	s.noRawKeys("decision evaluate form")
	for _, theme := range []string{"standard", "fruity-light"} {
		for _, width := range []int{1440, 390} {
			if width < 600 {
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page); err != nil {
					t.Fatal(err)
				}
			}
			page.MustSetViewport(width, 844, 1, width < 600)
			page.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.wait(`()=>{const win=document.querySelector('.vd-window'),box=win.getBoundingClientRect(),detail=document.querySelector('.ed-detail').getBoundingClientRect(),title=document.querySelector('.ed-detail-title').getBoundingClientRect(),status=document.querySelector('.ed-detail-status'),statusBox=status.getBoundingClientRect(),test=document.querySelector('.ed-detail [data-ed-detail-test]').getBoundingClientRect(),close=document.querySelector('.ed-detail [data-ed-detail-close]').getBoundingClientRect(),form=document.querySelector('.ed-detail .ed-field[data-param="questions"]').getBoundingClientRect(),mobile=innerWidth<600,portal=document.querySelector('.ed-detail-backdrop.ed-detail-portal');return (mobile?win.classList.contains('vd-mobile-wide-window')&&box.width>innerWidth+300&&!!portal:!portal) && detail.left>=-1 && detail.right<=innerWidth+1 && detail.width<=innerWidth+1 && close.left>=-1 && close.right<=innerWidth+1 && test.left>=-1 && test.right<=innerWidth+1 && form.left>=-1 && form.right<=innerWidth+1 && (!mobile || !status.textContent.trim() || statusBox.top>=title.bottom-1)}`)
			s.shot(fmt.Sprintf("decision-evaluate-%s-%d", theme, width))
		}
	}
	_ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`()=>fixtureTheme('standard')`)

	// Save, export through the editor's real More menu, close and reopen, then import the captured
	// export through the actual file-input/change path on the start page.
	s.wait(`()=>{const e=edFixture.flow(window.__flowID);return !!e}`)
	page.MustElement(`.ed-detail [data-ed-pane="params"]`).MustClick()
	page.Keyboard.MustType(input.Escape)
	s.wait(`()=>!document.querySelector('.ed-detail')`)
	s.wait(`()=>{const d=edFixture.flow(window.__flowID).rec.draft;const n=d.nodes.find(n=>n.id===window.__decisionIDs.decision);return n && n.params.mode==='evaluate' && n.params.questions.length===2 && n.params.images[0]==='media:123' && n.settings.on_error==='error_port'}`)
	page.MustElement(`[data-ed-cmd="more"]`).MustClick()
	page.MustEval(`()=>{const label=edFixture.editor().t('easydrag.ui.home_export');const button=[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(item=>item.textContent.includes(label));if(!button)throw new Error('Export action is missing');button.click();}`)
	// Native anchor downloads do not pass through the fixture's fetch mock, so request the same
	// export URL through its API mock after exercising the editor's export action.
	page.MustEval(`async()=>{const ed=edFixture.editor(),response=await fetch(ed.api.exportUrl(ed.flow.id));if(!response.ok)throw new Error('Flow export failed');window.__exportedDecisionFlow=await response.json();}`)
	s.wait(`()=>edFixture.state.exports.some(doc=>doc.name==='Decision browser check' && doc.nodes.some(n=>n.type==='ai.decision'))`)
	page.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-home')`)
	s.wait(`()=>{const e=edFixture.flow(window.__flowID);return e && e.rec.draft.nodes.some(n=>n.type==='ai.decision')}`)
	s.wait(`()=>!!document.querySelector('.ed-flow-card[data-ed-flow="'+window.__flowID+'"]')`)
	page.MustEval(`()=>{const card=document.querySelector('.ed-flow-card[data-ed-flow="'+window.__flowID+'"]');if(!card)throw new Error('saved Decision flow is missing');card.querySelector('h3').click();}`)
	s.wait(`()=>{const ed=edFixture.editor();return !!ed && ed.flow.id===window.__flowID && ed.model.node(window.__decisionIDs.decision).params.questions.length===2}`)
	s.wait(`()=>{const d=edFixture.flow(window.__flowID).rec.draft,n=d.nodes.find(n=>n.type==='ai.decision');return n.params.images[0]==='media:123' && n.settings.on_error==='error_port'}`)
	page.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-home [data-ed-import-file]')`)
	page.MustEval(`()=>{const doc=edFixture.state.exports[edFixture.state.exports.length-1];const input=document.querySelector('.ed-home [data-ed-import-file]');const transfer=new DataTransfer();transfer.items.add(new File([JSON.stringify(doc)],'decision.easydrag.json',{type:'application/json'}));input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>edFixture.state.flows.size===3`)
	s.wait(`()=>{const imported=[...edFixture.state.flows.entries()].find(([id,e])=>id!==window.__flowID && id!==edFixture.seededID && e.rec.draft.nodes.some(n=>n.type==='ai.decision'));if(!imported)return false;const n=imported[1].rec.draft.nodes.find(n=>n.type==='ai.decision');return n.params.mode==='evaluate' && n.params.questions.length===2 && n.params.images[0]==='media:123'}`)
	s.failOnPageErrors("decision import/export")
}
