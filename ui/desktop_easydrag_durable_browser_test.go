package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These synthetic journeys never edit config.yaml or enable the production Runner.
func TestDesktopEasyDragDurableBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-durable-fixture.js")
	p := s.page
	first := p.MustEval(`()=>edDurableFixture.seed('Gespeicherte Wartezeit',false)`).Map()
	flowID, runID := first["flow_id"].Str(), first["run_id"].Str()
	p.MustEval(`x=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:x.flow,runId:x.run})`, map[string]string{"flow": flowID, "run": runID})
	s.wait(`()=>edFixture.editor()?.run?.id==='` + runID + `' && !!document.querySelector('[data-ed-durable-run]')`)
	s.wait(`()=>document.querySelector('[data-ed-durable-run]').textContent.includes(edFixture.editor().t('easydrag.ui.durable_reason_disabled'))`)
	s.wait(`()=>{const p=document.querySelector('.ed-run-results'),title=edFixture.editor().t('easydrag.ui.durable_title');return p.open && p.textContent.split(title).length===2;}`)
	s.shot("durable-blocked-standard-1440")
	for _, width := range []int{390, 320} {
		p.MustSetViewport(width, 844, 1, true)
		p.MustEval(`()=>fixtureTheme('fruity-dark')`)
		s.wait(`()=>{const p=document.querySelector('.ed-run-results');const r=p.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth+1 && p.scrollWidth<=p.clientWidth+1;}`)
		s.wait(`()=>{const b=document.querySelector('.ed-runview-banner'),title=b.querySelector('[data-ed-runview-text]').getBoundingClientRect(),buttons=[...b.querySelectorAll('button')].filter(x=>!x.hidden),rects=buttons.map(x=>x.getBoundingClientRect());return b.scrollWidth<=b.clientWidth+1 && buttons.every((x,i)=>{const r=rects[i],s=x.querySelector('span:not(.ed-icon)'),text=s?.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth+1 && r.top>=title.bottom-1 && x.scrollWidth<=x.clientWidth+1 && (!text || text.left>=r.left && text.right<=r.right+1) && rects.every((q,j)=>i===j || r.right<=q.left+1 || q.right<=r.left+1 || r.bottom<=q.top+1 || q.bottom<=r.top+1);});}`)
		if width == 390 {
			s.shot("durable-blocked-fruity-390")
		} else {
			s.shot("durable-blocked-fruity-320")
		}
	}
	// Accepted means pending, not cancelled, and never a second Stop without user action.
	p.MustElement(`[data-ed-cmd="stop-viewed"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	p.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(`()=>edFixture.editor().run.durableStopping===true && edFixture.editor().run.status==='waiting'`)
	s.shot("durable-cancel-pending-320")
	p.MustEval(`id=>edDurableFixture.completeCancel(id)`, runID)
	p.MustElement(`[data-ed-durable-check]`).MustClick()
	s.wait(`()=>edFixture.editor().run.status==='cancelled' && !edFixture.editor().run.durableStopping`)
	p.MustEval(`()=>{const posts=edDurableFixture.state.requests.filter(r=>r.method==='POST');if(posts.length!==1 || !posts[0].path.endsWith('/durable/cancel') || posts[0].body.generation!==2)throw Error('Unexpected activation or mutation replay');}`)

	// A terminal legacy run can still have an incompatible checkpoint to clean up.
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	legacy := p.MustEval(`()=>edDurableFixture.seed('Alter Zwischenstand',true)`).Map()
	p.MustEval(`x=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:x.flow,runId:x.run})`, map[string]string{"flow": legacy["flow_id"].Str(), "run": legacy["run_id"].Str()})
	s.wait(`()=>!!document.querySelector('[data-ed-durable-run]') && edFixture.editor().run.status==='error' && !document.querySelector('[data-ed-cmd="stop-viewed"]').hidden`)
	s.shot("durable-legacy-checkpoint")
	p.MustElement(`[data-ed-cmd="stop-viewed"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="stop"]')`)
	p.MustElement(`.ed-modal [data-ed-action="stop"]`).MustClick()
	s.wait(`()=>edFixture.editor().run.durableStopping`)
	p.MustEval(`id=>edDurableFixture.completeCancel(id)`, legacy["run_id"].Str())
	p.MustElement(`[data-ed-durable-check]`).MustClick()
	s.wait(`()=>edFixture.editor().run.status==='error' && document.querySelector('[data-ed-cmd="stop-viewed"]').hidden`)
	s.noRawKeys("durable lifecycle")
	s.failOnPageErrors("durable lifecycle")
}

func TestConfigEasyDragDurableSwitchBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := configRefreshFixtureOrigin(t, "de", false)
	p := newSmokeBrowser(t).MustPage()
	defer p.MustClose()
	p.MustEvalOnNewDocument(`(()=>{const old=window.fetch.bind(window);window.fetch=async(input,init)=>{const u=new URL(String(input),location.href),result=await old(input,init);if(u.pathname==='/api/config' && (!init||!init.method||init.method==='GET') && result.ok){const body=await result.json();body.flows=Object.assign({},body.flows,{durable_enabled:false});body._flows_durable_status={configured:false,effective:false,recovery_available:false,reason:'service_unavailable',waiting_count:null,claimed_count:null,blocked_count:null};return new Response(JSON.stringify(body),{status:200,headers:{'Content-Type':'application/json'}});}return result;};})()`)
	p.MustSetViewport(1440, 900, 1, false)
	p.MustNavigate(origin + "/config#flows").MustWaitLoad()
	waitForJSBool(t, p, `()=>!!document.querySelector('[data-path="flows.durable_enabled"]') && !!document.querySelector('[data-flows-durable-status]')`)
	waitForJSBool(t, p, `()=>!document.querySelector('[data-path="flows.durable_enabled"]').classList.contains('on') && !configData.flows.durable_enabled && document.querySelector('[data-flows-durable-status]').textContent.includes(t('config.status.unknown'))`)
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "easydrag-c11-browser")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	capture := &easyDragSmoke{t: t, page: p, dir: dir}
	capture.shot("durable-config-off-desktop")
	p.MustSetViewport(390, 844, 1, true)
	capture.shot("durable-config-off-390")
}

func TestDesktopEasyDragDurableReconnectBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-durable-fixture.js", "/testdata/easydrag-durable-ready-fixture.js")
	p := s.page
	flowID := p.MustEval(`()=>edDurableFixture.seed('Fortsetzung nach Verbindungsabbruch',false,true).flow_id`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, flowID)
	s.wait(`()=>edFixture.editor()?.flow.id==='` + flowID + `' && edFixture.editor().runtime?.value?.effective`)
	p.MustElement(`[data-ed-cmd="run-now"]`).MustClick()
	s.wait(`()=>edFixture.editor()?.run?.record?.durable && edFixture.editor().run.status==='waiting'`)
	runID := p.MustEval(`()=>edFixture.editor().run.id`).Str()
	p.MustEval(`id=>edDurableFixture.beginWait(id)`, runID)
	s.wait(`()=>edFixture.editor().run.durableStatus?.state==='waiting'`)
	s.wait(`()=>!document.querySelector('.ed-run-results').open`)
	s.shot("durable-waiting-standard-1440")
	p.MustEval(`id=>{edDurableFixture.forgetBacklog(id);edDurableFixture.disconnect(id);}`, runID)
	s.wait(`()=>edFixture.state.streams.filter(s=>!s.closed && s.url.includes('` + runID + `') && s.url.includes('after=1')).length===1 && edFixture.editor().run.durableStatus?.state==='waiting'`)
	p.MustEval(`id=>{if(edFixture.editor().run.id!==id || edDurableFixture.state.requests.filter(r=>r.method==='POST' && r.path.endsWith('/run')).length!==1)throw Error('Run identity changed or admission replayed');}`, runID)
	s.shot("durable-reconnected-same-run")
	p.MustElement(`.ed-run-results summary`).MustClick()
	s.wait(`()=>document.querySelector('.ed-run-results').open`)
	p.MustSetViewport(390, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-light')`)
	p.MustEval(`id=>edDurableFixture.wake(id)`, runID)
	// The five-second metadata poll needs a complete cadence after the streamed wake.
	p.Timeout(10 * time.Second).MustWait(`()=>edFixture.editor().run.status==='running' && edFixture.editor().run.durableStatus?.state==='running'`)
	s.wait(`()=>document.querySelector('.ed-run-results').open`)
	p.MustElement(`.ed-run-results summary`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-run-results').open`)
	s.shot("durable-resumed-fruity-light-390")
	p.MustEval(`id=>edDurableFixture.finish(id)`, runID)
	s.wait(`()=>edFixture.editor().run.status==='success' && edFixture.editor().run.record.durable && !edFixture.editor().run.durableStatus`)
	s.shot("durable-completed-fruity-light-390")
	s.noRawKeys("durable reconnect")
	s.failOnPageErrors("durable reconnect")
}
