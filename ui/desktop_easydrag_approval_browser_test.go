package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

func TestDesktopEasyDragApprovalBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-approval-fixture.js")
	p := s.page
	ids := p.MustEval(`()=>edApprovalFixture.seed()`).Map()
	gate1, run1 := ids["gate1"].Str(), ids["run1"].Str()
	gate2, run2 := ids["gate2"].Str(), ids["run2"].Str()
	gate3, run3 := ids["gate3"].Str(), ids["run3"].Str()

	// The two inbox pages repeat one gate. The UI deduplicates by gate ID and never writes while opening them.
	p.MustElement(`[data-ed-approval-inbox]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-modal--approval-inbox .ed-approval-card').length===1 && !!document.querySelector('[data-ed-approval-more]')`)
	p.MustElement(`[data-ed-approval-more]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-modal--approval-inbox .ed-approval-card').length===3 && !document.querySelector('[data-ed-approval-more]')`)
	p.MustEval(`()=>{const r=edApprovalFixture.state.requests;const gets=r.filter(x=>x.method==='GET'&&x.path==='/api/desktop/flows/approvals');if(gets.length!==2||r.some(x=>x.method==='POST'))throw Error('Opening/paging the inbox must remain GET-only');}`)
	s.shot("approval-inbox-standard-dark-1440")
	p.MustElement(`[data-ed-approval-open="0"]`).MustClick()
	s.wait(`()=>edFixture.editor()?.runView?.run.id==='` + run1 + `' && !!document.querySelector('.ed-modal--approval-review [data-ed-approval-json]')`)
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review'),summary=d?.querySelector('[data-ed-approval-summary]'),data=d?.querySelector('[data-ed-approval-json]');return summary?.textContent===edApprovalFixture.state.gates.get('` + gate1 + `').review.summary && data?.textContent===JSON.stringify(edApprovalFixture.state.gates.get('` + gate1 + `').review.review_data,null,2) && !d.querySelector('script,img')}`)
	p.MustEval(`()=>{const d=document.querySelector('.ed-modal--approval-review'),b=d.querySelector('.ed-modal-body');if(document.activeElement!==d.querySelector('[data-ed-action="refresh"]'))throw Error('Review did not focus its real status control');if(b.scrollHeight<=b.clientHeight+1)throw Error('Complete review is not scrollable');}`)
	p.MustEval(`id=>{const ed=edFixture.editor(),button=document.querySelector('.ed-runview-banner [data-ed-cmd="approval-review"]');if(window.EasyDrag.durable.info(ed.run)?.approval_gate_id!==id||!button||button.hidden)throw Error('RunStatus approval gate was not routed to RunView');}`, gate1)
	p.MustEval(`()=>{const r=edApprovalFixture.state.requests.filter(x=>x.path.includes('/approvals/'));if(r.some(x=>x.method==='POST'))throw Error('Opening the exact run submitted a decision');}`)
	p.MustElement(`.ed-modal--approval-review [data-ed-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--approval-review')`)
	p.MustElement(`.ed-runview-banner [data-ed-cmd="approval-review"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-modal--approval-review [data-ed-approval-summary]')?.textContent.startsWith('Freigabe 1:')`)

	for _, view := range []struct {
		name, theme string
		width       int
	}{
		{"fruity-light-390", "fruity-light", 390},
		{"fruity-dark-320", "fruity-dark", 320},
	} {
		p.MustSetViewport(view.width, 844, 1, true)
		p.MustEval(`theme=>fixtureTheme(theme)`, view.theme)
		s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review'),b=d?.querySelector('.ed-modal-body'),r=d?.getBoundingClientRect(),c=document.querySelector('.ed-app')?.getBoundingClientRect(),h=d?.querySelector('.ed-modal-head')?.getBoundingClientRect(),f=d?.querySelector('.ed-modal-foot')?.getBoundingClientRect();return !!d&&!!c&&!!h&&!!f&&r.left>=-1&&r.right<=innerWidth+1&&d.scrollWidth<=d.clientWidth+1&&b.scrollHeight>b.clientHeight+1&&b.scrollWidth<=b.clientWidth+1&&h.top>=c.top-1&&h.bottom<=c.bottom+1&&f.top>=c.top-1&&f.bottom<=c.bottom+1;}`)
		s.shot("approval-" + view.name)
	}

	// Keyboard confirmation sends one POST. A lost AbortError response becomes uncertain; status reads stay GET-only.
	p.MustElement(`[data-ed-approval-start="approve"]`).MustClick()
	s.wait(`()=>document.activeElement===document.querySelector('[data-ed-approval-confirm="approve"]')`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>document.querySelector('.ed-modal--approval-review')?.textContent.includes(edFixture.editor().t('easydrag.ui.approval_uncertain')) && !document.querySelector('[data-ed-approval-confirm]')`)
	p.MustEval(`id=>edApprovalFixture.failNextDetail(id)`, gate1)
	p.MustElement(`.ed-modal--approval-review [data-ed-action="refresh"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review');return d?.textContent.includes(edFixture.editor().t('easydrag.ui.approval_unknown_help'))&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_uncertain'))&&!d.textContent.includes('nicht gesendet')}`)
	p.MustEval(`()=>{const f=edApprovalFixture.state;const posts=f.requests.filter(x=>x.method==='POST'&&x.path.endsWith('/decision'));if(posts.length!==1||posts[0].body.gate_generation!==2||posts[0].body.content_digest!=='a'.repeat(64)||posts[0].body.decision!=='approve')throw Error('Decision request identity or count changed');}`)

	// The first successful read can still show pending. It must not restore decision controls; only a later GET observes commit.
	p.MustElement(`.ed-modal--approval-review [data-ed-action="refresh"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review');return !!d?.querySelector('.ed-approval-state--pending')&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_uncertain'))&&!d.querySelector('[data-ed-approval-start]')}`)
	// A changed digest under the same IDs is not the outcome of the submitted review.
	p.MustEval(`id=>edApprovalFixture.state.gates.get(id).item.content_digest='f'.repeat(64)`, gate1)
	p.MustElement(`.ed-modal--approval-review [data-ed-action="refresh"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review');return !!d?.querySelector('.ed-approval-state--unknown')&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_uncertain'))&&!d.querySelector('[data-ed-approval-start]')&&!d.querySelector('[data-ed-approval-confirm]')}`)
	p.MustEval(`id=>edApprovalFixture.state.gates.get(id).item.content_digest='a'.repeat(64)`, gate1)
	p.MustElement(`.ed-modal--approval-review [data-ed-action="refresh"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review');return !!d?.querySelector('.ed-approval-state--approved')&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_actor'))&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_review_not_retained'))}`)
	p.MustEval(`()=>{const posts=edApprovalFixture.state.requests.filter(x=>x.method==='POST'&&x.path.endsWith('/decision'));if(posts.length!==1)throw Error('A status check replayed the decision POST');}`)
	s.shot("approval-decided-320")

	// A late old-run response cannot overwrite the new run's exact review.
	p.MustElement(`.ed-modal--approval-review [data-ed-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--approval-review')`)
	p.MustEval(`id=>edApprovalFixture.delayNextDetail(id,250)`, gate1)
	p.MustEval(`id=>{const ed=edFixture.editor();window.EasyDrag.approvals.openRunReview(ed,id,{runId:ed.runView.run.id,flowId:ed.flow.id,revision:ed.runView.run.revision});}`, gate1)
	s.wait(`()=>edApprovalFixture.state.requests.some(x=>x.method==='GET'&&x.gateID==='` + gate1 + `')`)
	p.MustEval(`x=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:edFixture.seededID,runId:x.run,gateId:x.gate})`, map[string]string{"run": run2, "gate": gate2})
	s.wait(`()=>edFixture.editor()?.runView?.run.id==='` + run2 + `' && document.querySelector('.ed-modal--approval-review [data-ed-approval-summary]')?.textContent==='Freigabe 2: Sitzung erforderlich'`)
	s.wait(`()=>edApprovalFixture.state.delayedReads===1`)
	p.MustEval(`id=>{const d=document.querySelector('.ed-modal--approval-review');if(edFixture.editor().runView.run.id!==id||d.querySelector('[data-ed-approval-summary]').textContent!=='Freigabe 2: Sitzung erforderlich'||!d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_blocked_session_required'))||d.querySelector('[data-ed-approval-start]'))throw Error('Late review crossed run identity or enabled a blocked decision');}`, run2)
	p.MustSetViewport(390, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-light')`)
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review'),r=d.getBoundingClientRect();return r.left>=-1&&r.right<=innerWidth+1&&d.scrollWidth<=d.clientWidth+1;}`)
	s.shot("approval-session-required-fruity-light-390")
	p.MustEval(`x=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:edFixture.seededID,runId:x.run,gateId:x.gate})`, map[string]string{"run": run3, "gate": gate3})
	s.wait(`()=>edFixture.editor()?.runView?.run.id==='` + run3 + `' && document.querySelector('.ed-modal--approval-review [data-ed-approval-summary]')?.textContent==='Freigabe 3: Direkte Bestätigung'`)
	p.MustElement(`[data-ed-approval-start="reject"]`).MustClick()
	s.wait(`()=>document.activeElement===document.querySelector('[data-ed-approval-confirm="reject"]')`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review'),ed=edFixture.editor();return !!d?.querySelector('.ed-approval-state--rejected')&&d.textContent.includes(ed.t('easydrag.ui.approval_actor'))&&d.textContent.includes(ed.t('easydrag.ui.approval_decision_saved',{state:ed.t('easydrag.ui.approval_state_rejected')}))}`)
	p.MustEval(`()=>{const posts=edApprovalFixture.state.requests.filter(x=>x.method==='POST'&&x.path.endsWith('/decision'));if(posts.length!==2||posts[1].body.decision!=='reject'||Object.keys(posts[1].body).sort().join(',')!=='content_digest,decision,gate_generation')throw Error('Successful acknowledgement or request shape failed');}`)
	s.shot("approval-rejected-fruity-light-390")
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	// Reopen the terminal run from history; its durable status retains only the gate route, never review data.
	p.MustElement(`.ed-modal--approval-review [data-ed-action="close"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--approval-review')`)
	p.MustElement(`[data-ed-cmd="exit-run-view"]`).MustClick()
	s.wait(`()=>!edFixture.editor()?.runView`)
	p.MustElement(`[data-ed-cmd="runs"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-run="` + run3 + `"]')`)
	p.MustElement(`[data-ed-run="` + run3 + `"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor(),info=window.EasyDrag.durable.info(ed?.run),button=document.querySelector('.ed-runview-banner [data-ed-cmd="approval-review"]');return ed?.runView?.run.id==='` + run3 + `'&&info?.state==='error'&&info.approval_gate_id==='` + gate3 + `'&&button&&!button.hidden}`)
	p.MustElement(`[data-ed-drawer-close]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-drawer')`)
	p.MustElement(`.ed-runview-banner [data-ed-cmd="approval-review"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--approval-review');return !!d?.textContent.includes(edFixture.editor().t('easydrag.ui.approval_review_not_retained'))&&!d.querySelector('[data-ed-approval-json]')&&d.textContent.includes(edFixture.editor().t('easydrag.ui.approval_actor'))}`)
	p.MustEval(`id=>{const ed=edFixture.editor(),info=window.EasyDrag.durable.info(ed.run),d=document.querySelector('.ed-modal--approval-review');if(info.state!=='error'||info.approval_gate_id!==id||d.querySelector('[data-ed-approval-summary]')||d.querySelector('[data-ed-approval-json]')||d.textContent.includes(ed.t('easydrag.ui.approval_blocked_decision_complete')))throw Error('Historical terminal run lost gate metadata, retained review content or showed a decision-complete warning');}`, gate3)
	s.shot("approval-historical-terminal-error-standard-1440")
	s.noRawKeys("approval flow")
	s.failOnPageErrors("approval flow")
}
