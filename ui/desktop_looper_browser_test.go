package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
)

// looperFixtureStream is a controllable status stream: the test pushes the
// state the server would report, every connected client receives each change.
type looperFixtureStream struct {
	mu      sync.Mutex
	payload map[string]any
	version int
	held    bool
	started chan struct{}
	once    sync.Once
}

func newLooperFixtureStream() *looperFixtureStream {
	return &looperFixtureStream{
		payload: map[string]any{"status": "idle", "running": false, "paused": false, "round": 0, "max_rounds": 10, "logs": []any{}},
		started: make(chan struct{}),
	}
}

func (s *looperFixtureStream) set(payload map[string]any) {
	s.mu.Lock()
	s.payload = payload
	s.version++
	s.mu.Unlock()
}

// hold delays the next event, to model a status stream that answers late.
func (s *looperFixtureStream) hold(on bool) {
	s.mu.Lock()
	s.held = on
	s.mu.Unlock()
}

func (s *looperFixtureStream) snapshot() (map[string]any, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.payload, s.version, s.held
}

func (s *looperFixtureStream) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", http.StatusInternalServerError)
		return
	}
	lastVersion := -1
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		payload, version, held := s.snapshot()
		if version != lastVersion && !held {
			lastVersion = version
			raw, err := json.Marshal(payload)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func looperLog(round int, step string, ms int, extra map[string]any) map[string]any {
	entry := map[string]any{"round": round, "step": step, "duration_ms": ms}
	for k, v := range extra {
		entry[k] = v
	}
	return entry
}

func TestDesktopLooperBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/looper-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow};})();`

	stream := newLooperFixtureStream()
	round1 := []any{
		looperLog(1, "work", 800, map[string]any{"response": "Wrote the first draft."}),
		looperLog(1, "evaluate", 400, map[string]any{"score": 62, "feedback": "Add a clearer ending."}),
	}
	round2 := append(append([]any{}, round1...),
		looperLog(2, "work", 1200, map[string]any{"response": "Rewrote the ending."}),
		looperLog(2, "evaluate", 500, map[string]any{"score": 74, "feedback": "Tighten the middle."}),
	)
	running := func(extra map[string]any) map[string]any {
		payload := map[string]any{
			"status": "running", "running": true, "paused": false, "round": 3, "max_rounds": 10,
			"current_step": "work", "best_score": 74, "score_history": []int{62, 74}, "target_score": 85,
			"has_finish": true, "preset_name": "Short story", "goal_excerpt": "Write Documents/Looper/story.md",
			"last_feedback": "Tighten the middle.", "input_tokens": 1200, "output_tokens": 800,
			"estimated_cost_usd": 0.02, "elapsed_ms": 95000, "run_id": 1, "logs_from": 0, "log_total": 4, "logs": round2,
		}
		for k, v := range extra {
			payload[k] = v
		}
		return payload
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/looper-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(desktop.BuiltinApps())
	})
	mux.HandleFunc("/api/desktop/looper/run", func(w http.ResponseWriter, r *http.Request) {
		stream.once.Do(func() { close(stream.started) })
		stream.set(running(nil))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/api/desktop/looper/status", stream.serve)

	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)
	page.MustNavigate(server.URL + "/fixture")
	page.MustWaitLoad()
	page.MustEval(`async()=>await fixtureReady`)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}

	page.MustEval(`()=>{
        const prev=window.fetch;
        const json=data=>new Response(JSON.stringify(data),{headers:{'Content-Type':'application/json'}});
        window.fetch=async(url,options={})=>{
            const path=String(url);
            if(path==='/api/desktop/looper/run' || path==='/api/desktop/looper/resume'){
                window.__lastLooperBody=options.body||'';
                window.__lastLooperPath=path;
                if(window.__looperRunError){
                    return new Response(JSON.stringify({error:'English text',code:window.__looperRunError}),{status:402,headers:{'Content-Type':'application/json'}});
                }
                // the fixture server lives on the same origin; use a real request so it can switch the stream
                await new Promise((resolve,reject)=>{
                    const xhr=new XMLHttpRequest();
                    xhr.open(options.method||'POST',path);
                    xhr.onload=()=>resolve();
                    xhr.onerror=()=>reject(new Error('looper run signal failed'));
                    xhr.send(options.body||null);
                });
                return json({ok:true});
            }
            if(path.startsWith('/api/desktop/looper/presets')) return json({presets:[
                {id:1,is_builtin:true,builtin_key:'short_story',name:'Short story',
                 goal:'Write Documents/Looper/story.md',work:'Improve the story',evaluate:'Score the prose',
                 finish:'',max_rounds:10,target_score:85,stall_rounds:3},
                {id:2,is_builtin:false,name:'Second loop',goal:'Another goal',work:'Another work',evaluate:'Another review',
                 finish:'',max_rounds:6,target_score:90,stall_rounds:3}
            ]});
            if(path.startsWith('/api/desktop/looper/active')){
                if(window.__activeMissing) return new Response(JSON.stringify({error:'no active run',code:'no_active_run'}),{status:404,headers:{'Content-Type':'application/json'}});
                return json({status:'ok',config:{goal:'Restored goal',work:'Restored work',evaluate:'Restored review',finish:'',max_rounds:7,target_score:90,stall_rounds:2,provider_id:'',model:'',preset_name:'Restored loop'}});
            }
            if(path.startsWith('/api/desktop/looper/runs/7')) return json({status:'ok',run:{
                id:7,preset_name:'Past loop',status:'completed',rounds:2,max_rounds:6,best_score:91,target_score:90,
                logs:[{round:1,step:'work',duration_ms:300,response:'draft'},{round:1,step:'evaluate',score:91,duration_ms:200,feedback:'good'}],
                config:{goal:'History goal',work:'History work',evaluate:'History review',finish:'History finish',max_rounds:6,target_score:90,stall_rounds:2,provider_id:'',model:'kept-model',preset_name:'Past loop'}}});
            if(path.startsWith('/api/desktop/looper/runs/8')) return json({status:'ok',run:{
                id:8,preset_name:'Ancient loop',status:'stopped',rounds:1,max_rounds:5,best_score:40,target_score:85,
                logs:[{round:1,step:'work',duration_ms:300,response:'draft'}]}});
            if(path.startsWith('/api/desktop/looper/runs')) return json({runs:[
                {id:7,preset_name:'Past loop',status:'completed',rounds:2,max_rounds:6,best_score:91,target_score:90,goal_excerpt:'History goal'},
                {id:8,preset_name:'Ancient loop',status:'stopped',rounds:1,max_rounds:5,best_score:40,target_score:85,goal_excerpt:'Old goal'}]});
            if(path.startsWith('/api/providers')) return json({providers:[]});
            return prev(url,options);
        };
    }`)

	page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('looper');await fixtureOpen('looper');}`)

	// The client side of the log-delta protocol: append by absolute index, restart
	// on a new run, and take the sent window when the server trimmed what we missed.
	merged := page.MustEval(`()=>{
        const m=window.LooperMonitor, cache=m.newLogCache();
        const first=m.mergeStatus(cache,{run_id:1,logs_from:0,logs:[{n:1},{n:2}]});
        const next=m.mergeStatus(cache,{run_id:1,logs_from:2,logs:[{n:3}]});
        const repeat=m.mergeStatus(cache,{run_id:1,logs_from:1,logs:[{n:'b'},{n:'c'},{n:'d'}]});
        const gap=m.mergeStatus(cache,{run_id:1,logs_from:50,logs:[{n:51}]});
        const fresh=m.mergeStatus(cache,{run_id:2,logs_from:0,logs:[{n:'x'}]});
        return JSON.stringify([first.logs.length,next.logs.length,next.logs_base,repeat.logs.length,gap.logs.length,gap.logs_base,fresh.logs.length,fresh.logs_base]);
    }`).Str()
	if merged != "[2,3,0,4,1,50,1,0]" {
		t.Fatalf("log delta merge changed behaviour: %s", merged)
	}
	page.MustEval(`()=>{
        const win=[...aurora.state.windows.values()].find(w=>w.appId==='looper');
        if(!win) throw new Error('looper window missing');
        win.element.classList.remove('maximized');
        Object.assign(win.element.style,{width:'1120px',height:'720px',left:'40px',top:'40px'});
    }`)
	page.MustWait(`()=>!!document.querySelector('.vd-looper')`)

	wide := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        const list=root.querySelector('.vd-looper-list');
        const editor=root.querySelector('.vd-looper-editor');
        const side=root.querySelector('.vd-looper-side');
        const tabs=root.querySelector('.vd-looper-tabs');
        const lr=list.getBoundingClientRect(),er=editor.getBoundingClientRect(),sr=side.getBoundingClientRect();
        return {
            compact:root.classList.contains('vd-looper--compact'),
            overflow:root.scrollWidth>root.clientWidth+1,
            columns:lr.left<er.left && er.left<sr.left && lr.width>80 && er.width>80 && sr.width>80,
            tabsHidden:getComputedStyle(tabs).display==='none',
            briefHidden:getComputedStyle(root.querySelector('.vd-looper-brief')).display==='none',
            emptyVisible:!!root.querySelector('.vd-looper-empty') && !root.querySelector('.vd-looper-empty').hidden,
            settingsAboveGoal:root.querySelector('.vd-looper-settings').getBoundingClientRect().top < root.querySelector('.vd-looper-card').getBoundingClientRect().top
        };
    }`).Map()
	if wide["compact"].Bool() {
		t.Fatal("wide looper window must use the three-column layout")
	}
	if wide["overflow"].Bool() {
		t.Fatal("wide looper window must not overflow horizontally")
	}
	if !wide["columns"].Bool() {
		t.Fatal("wide looper window must show list, editor and run columns")
	}
	if !wide["tabsHidden"].Bool() {
		t.Fatal("wide looper window must hide compact tabs")
	}
	if !wide["briefHidden"].Bool() {
		t.Fatal("the run summary card must stay hidden until a run has the stage")
	}
	if !wide["emptyVisible"].Bool() {
		t.Fatal("an idle looper must explain how a run works")
	}
	if !wide["settingsAboveGoal"].Bool() {
		t.Fatal("run settings must sit above the text fields, not below the fold")
	}
	saveLooperShot(t, page, "wide-setup.png")

	page.MustEval(`()=>{
        const win=[...aurora.state.windows.values()].find(w=>w.appId==='looper');
        Object.assign(win.element.style,{width:'700px',height:'720px'});
    }`)
	page.MustWait(`()=>document.querySelector('.vd-looper').classList.contains('vd-looper--compact')`)
	compact := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        const tabs=root.querySelector('.vd-looper-tabs');
        const list=root.querySelector('.vd-looper-list');
        const editor=root.querySelector('.vd-looper-editor');
        const resume=root.querySelector('.vd-looper-resume');
        return {
            compact:root.classList.contains('vd-looper--compact'),
            pane:root.dataset.pane||'',
            tabsVisible:getComputedStyle(tabs).display==='flex',
            overflow:root.scrollWidth>root.clientWidth+1,
            listH:list.getBoundingClientRect().height,
            editorH:editor.getBoundingClientRect().height,
            resumeHidden:resume.hidden || getComputedStyle(resume).display==='none'
        };
    }`).Map()
	if !compact["compact"].Bool() || !compact["tabsVisible"].Bool() {
		t.Fatal("narrow looper window must show compact setup/run/history tabs")
	}
	if compact["pane"].Str() != "setup" {
		t.Fatalf("compact setup pane missing, got %q", compact["pane"].Str())
	}
	if compact["listH"].Num() < 80 || compact["editorH"].Num() < 80 {
		t.Fatalf("compact setup must show list and editor, heights %.0f / %.0f", compact["listH"].Num(), compact["editorH"].Num())
	}
	if compact["overflow"].Bool() {
		t.Fatal("compact looper window must not overflow horizontally")
	}
	if !compact["resumeHidden"].Bool() {
		t.Fatal("resume must stay hidden while idle")
	}
	saveLooperShot(t, page, "compact-setup.png")

	page.MustEval(`()=>{
        const win=[...aurora.state.windows.values()].find(w=>w.appId==='looper');
        Object.assign(win.element.style,{width:'1120px',height:'720px'});
    }`)
	page.MustWait(`()=>!document.querySelector('.vd-looper').classList.contains('vd-looper--compact')`)
	page.MustWait(`()=>!!document.querySelector('.vd-looper-preset')`)

	// Unsaved edits are flagged, and an untouched preset is not.
	page.MustEval(`()=>document.querySelector('.vd-looper-preset').click()`)
	page.MustWait(`()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value.startsWith('Write Documents')`)
	if page.MustEval(`()=>!document.querySelector('.vd-looper-dirty').hidden`).Bool() {
		t.Fatal("a freshly loaded preset must not look edited")
	}
	page.MustEval(`()=>{
        const goal=document.querySelector('.vd-looper textarea[id^="looper-goal-"]');
        goal.value+=' with a twist';
        goal.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	if !page.MustEval(`()=>!document.querySelector('.vd-looper-dirty').hidden && document.querySelector('.vd-looper').classList.contains('is-dirty')`).Bool() {
		t.Fatal("editing a preset must show the unsaved-changes dot")
	}
	page.MustEval(`()=>{
        const goal=document.querySelector('.vd-looper textarea[id^="looper-goal-"]');
        goal.value=goal.value.replace(' with a twist','');
        goal.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	if page.MustEval(`()=>!document.querySelector('.vd-looper-dirty').hidden`).Bool() {
		t.Fatal("undoing the edit must clear the unsaved-changes dot")
	}

	page.MustElement(".vd-looper-start").MustClick()
	page.MustWait(`()=>document.querySelectorAll('.vd-looper-log:not(.vd-looper-log--pending)').length>=4`)
	focus := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        const visible=el=>!!el && getComputedStyle(el).display!=='none' && !el.hidden;
        const rounds=[...root.querySelectorAll('.vd-looper-round')];
        const active=root.querySelector('.vd-looper-step.is-active');
        return {
            pane:root.dataset.pane,
            focus:root.classList.contains('is-focus'),
            briefVisible:visible(root.querySelector('.vd-looper-brief')),
            briefName:root.querySelector('.vd-looper-brief-name').textContent,
            fieldsHidden:!visible(root.querySelector('.vd-looper-card')) && !visible(root.querySelector('.vd-looper-settings')),
            listHidden:getComputedStyle(root.querySelector('.vd-looper-list')).visibility==='hidden',
            sideWidth:root.querySelector('.vd-looper-side').getBoundingClientRect().width,
            score:root.querySelector('.vd-looper-score-num').textContent,
            roundLine:root.querySelector('.vd-looper-roundline').textContent,
            progress:root.querySelector('.vd-looper-progress').getAttribute('aria-valuenow'),
            activeStep:active?active.dataset.step:'',
            finishStep:visible(root.querySelector('.vd-looper-step[data-step="finish"]')),
            roundCount:rounds.length,
            firstCollapsed:rounds[0].classList.contains('is-collapsed'),
            lastCollapsed:rounds[rounds.length-1].classList.contains('is-collapsed'),
            chart:!!root.querySelector('.vd-looper-chart-target'),
            clock:root.querySelector('.vd-looper-clock').textContent,
            pending:!!root.querySelector('.vd-looper-log--pending'),
            startDisabled:root.querySelector('.vd-looper-start').disabled,
            stopEnabled:!root.querySelector('.vd-looper-stop').disabled
        };
    }`).Map()
	if focus["pane"].Str() != "run" {
		t.Fatalf("start must switch to the run pane, got %q", focus["pane"].Str())
	}
	if !focus["focus"].Bool() || !focus["briefVisible"].Bool() || !focus["fieldsHidden"].Bool() || !focus["listHidden"].Bool() {
		t.Fatalf("a running loop must collapse the editor into the summary card: %v", focus)
	}
	if focus["briefName"].Str() != "Short story" {
		t.Fatalf("summary card must name the running loop, got %q", focus["briefName"].Str())
	}
	if focus["sideWidth"].Num() < 560 {
		t.Fatalf("the run view must take the stage, width %.0f", focus["sideWidth"].Num())
	}
	if focus["score"].Str() != "74" || !strings.Contains(focus["roundLine"].Str(), "3") || focus["progress"].Str() == "0" {
		t.Fatalf("hero must show score, round and progress: %v", focus)
	}
	if focus["activeStep"].Str() != "work" || !focus["finishStep"].Bool() || !focus["pending"].Bool() {
		t.Fatalf("hero must show the active step and the pending row: %v", focus)
	}
	if focus["roundCount"].Int() != 2 || !focus["firstCollapsed"].Bool() || focus["lastCollapsed"].Bool() {
		t.Fatalf("finished rounds fold away, the latest stays open: %v", focus)
	}
	if !focus["chart"].Bool() || !strings.Contains(focus["clock"].Str(), "1:3") {
		t.Fatalf("hero needs the target-line chart and the live clock: %v", focus)
	}
	if focus["startDisabled"].Bool() == false || !focus["stopEnabled"].Bool() {
		t.Fatal("start must lock and stop must unlock while running")
	}
	saveLooperShot(t, page, "running-timeline.png")

	// New entries are appended in place: existing DOM nodes survive an update.
	page.MustEval(`()=>{
        window.__firstLog=document.querySelector('.vd-looper-log');
        window.__firstLog.dataset.survivor='yes';
    }`)
	round3 := append(append([]any{}, round2...),
		looperLog(3, "work", 900, map[string]any{"response": "Tightened the middle."}),
	)
	stream.set(running(map[string]any{
		"current_step": "evaluate", "logs_from": 4, "log_total": 5, "logs": round3[4:], "elapsed_ms": 99000,
	}))
	page.MustWait(`()=>document.querySelectorAll('.vd-looper-log:not(.vd-looper-log--pending)').length>=5`)
	if !page.MustEval(`()=>document.querySelector('.vd-looper-log').dataset.survivor==='yes' && document.querySelector('.vd-looper-step.is-active').dataset.step==='evaluate'`).Bool() {
		t.Fatal("a delta update must append in place instead of rebuilding the timeline")
	}

	// A pending pause is visible and cannot be requested twice.
	stream.set(running(map[string]any{
		"current_step": "evaluate", "logs_from": 5, "log_total": 5, "logs": []any{}, "pause_requested": true,
	}))
	page.MustWait(`()=>!document.querySelector('.vd-looper-pausenote').hidden`)
	if !page.MustEval(`()=>document.querySelector('.vd-looper-pause').disabled && document.querySelector('.vd-looper-pause-label').textContent.includes('Pause nach')`).Bool() {
		t.Fatal("a requested pause must show up on the pause button")
	}

	// Paused: resume is offered and the run can be ended.
	stream.set(map[string]any{
		"status": "paused", "running": false, "paused": true, "round": 3, "max_rounds": 10, "resume_from": 3,
		"current_step": "paused", "best_score": 74, "score_history": []int{62, 74}, "target_score": 85,
		"preset_name": "Short story", "run_id": 1, "logs_from": 5, "log_total": 5, "logs": []any{},
	})
	page.MustWait(`()=>!document.querySelector('.vd-looper-resume').hidden`)
	paused := page.MustEval(`()=>({
        startDisabled:document.querySelector('.vd-looper-start').disabled,
        stopEnabled:!document.querySelector('.vd-looper-stop').disabled,
        stopLabel:document.querySelector('.vd-looper-stop-label').textContent,
        editEnabled:!document.querySelector('.vd-looper-brief-edit').disabled
    })`).Map()
	if !paused["startDisabled"].Bool() || !paused["stopEnabled"].Bool() || !paused["editEnabled"].Bool() {
		t.Fatalf("a paused run must be endable and editable: %v", paused)
	}
	if paused["stopLabel"].Str() != "Lauf beenden" {
		t.Fatalf("stop on a paused run must read as ending the run, got %q", paused["stopLabel"].Str())
	}
	saveLooperShot(t, page, "paused.png")

	// A narrow window keeps the run readable: no sideways scrolling, hero in view.
	page.MustEval(`()=>{
        const win=[...aurora.state.windows.values()].find(w=>w.appId==='looper');
        Object.assign(win.element.style,{width:'600px',height:'720px'});
    }`)
	page.MustWait(`()=>document.querySelector('.vd-looper').classList.contains('vd-looper--compact')`)
	narrow := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        const hero=root.querySelector('.vd-looper-hero').getBoundingClientRect();
        const side=root.querySelector('.vd-looper-side').getBoundingClientRect();
        return {
            pane:root.dataset.pane,
            overflow:root.scrollWidth>root.clientWidth+1 || root.querySelector('.vd-looper-side-pane.is-active').scrollWidth>root.querySelector('.vd-looper-side-pane.is-active').clientWidth+1,
            heroInside:hero.width>0 && hero.left>=side.left-1 && hero.right<=side.right+1
        };
    }`).Map()
	if narrow["pane"].Str() != "run" || narrow["overflow"].Bool() || !narrow["heroInside"].Bool() {
		t.Fatalf("compact run view must fit without sideways scrolling: %v", narrow)
	}
	saveLooperShot(t, page, "compact-run.png")
	page.MustEval(`()=>{
        const win=[...aurora.state.windows.values()].find(w=>w.appId==='looper');
        Object.assign(win.element.style,{width:'1120px',height:'720px'});
    }`)
	page.MustWait(`()=>!document.querySelector('.vd-looper').classList.contains('vd-looper--compact')`)

	// Finished: the verdict explains why the run ended and the editor can return.
	stream.set(map[string]any{
		"status": "completed", "running": false, "paused": false, "round": 4, "max_rounds": 10,
		"current_step": "idle", "best_score": 88, "score_history": []int{62, 74, 81, 88}, "target_score": 85,
		"preset_name": "Short story", "last_feedback": "Ready to ship.", "elapsed_ms": 140000,
		"input_tokens": 3000, "output_tokens": 1500, "estimated_cost_usd": 0.05, "run_id": 1,
		"logs_from": 5, "log_total": 5, "logs": []any{},
	})
	page.MustWait(`()=>!document.querySelector('.vd-looper-verdict').hidden`)
	done := page.MustEval(`()=>({
        verdict:document.querySelector('.vd-looper-verdict').textContent,
        reached:document.querySelector('.vd-looper-gauge').classList.contains('is-reached'),
        startEnabled:!document.querySelector('.vd-looper-start').disabled,
        stopDisabled:document.querySelector('.vd-looper-stop').disabled
    })`).Map()
	if !strings.Contains(done["verdict"].Str(), "Ziel erreicht") || !done["reached"].Bool() {
		t.Fatalf("a finished run must say why it ended: %v", done)
	}
	if !done["startEnabled"].Bool() || !done["stopDisabled"].Bool() {
		t.Fatalf("a finished run can be started again: %v", done)
	}
	saveLooperShot(t, page, "completed.png")

	page.MustElement(".vd-looper-brief-edit").MustClick()
	page.MustWait(`()=>!document.querySelector('.vd-looper').classList.contains('is-focus')`)
	if !page.MustEval(`()=>getComputedStyle(document.querySelector('.vd-looper-card')).display!=='none'`).Bool() {
		t.Fatal("editing after a run must bring the fields back")
	}

	// A stored run can be loaded into the editor, or started again; a run saved
	// before settings were recorded says so instead.
	page.MustEval(`()=>document.querySelector('.vd-looper-side-tab[data-side="history"]').click()`)
	mustWaitDump(t, page, "step 1", `()=>document.querySelectorAll('.vd-looper-history-item').length===2`)
	page.MustEval(`()=>document.querySelector('.vd-looper-history-item[data-run-id="8"]').click()`)
	mustWaitDump(t, page, "step 2", `()=>!!document.querySelector('.vd-looper-history-back')`)
	if !page.MustEval(`()=>!document.querySelector('.vd-looper-history-load') && !document.querySelector('.vd-looper-history-rerun') && document.querySelector('.vd-looper-history-actions').textContent.includes('aufgezeichnet')`).Bool() {
		t.Fatal("a run without stored settings must explain why it cannot be reused")
	}
	page.MustEval(`()=>document.querySelector('.vd-looper-history-back').click()`)
	mustWaitDump(t, page, "step 3", `()=>document.querySelectorAll('.vd-looper-history-item').length===2`)
	page.MustEval(`()=>document.querySelector('.vd-looper-history-item[data-run-id="7"]').click()`)
	mustWaitDump(t, page, "step 4", `()=>!!document.querySelector('.vd-looper-history-load')`)
	page.MustEval(`()=>document.querySelector('.vd-looper-history-load').click()`)
	mustWaitDump(t, page, "step 5", `()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value==='History goal'`)
	loaded := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        const val=prefix=>root.querySelector('textarea[id^="'+prefix+'"],input[id^="'+prefix+'"],select[id^="'+prefix+'"]').value;
        return {
            name:val('looper-name-'), work:val('looper-work-'), evaluate:val('looper-evaluate-'), finish:val('looper-finish-'),
            rounds:val('looper-max-'), score:val('looper-score-'), model:val('looper-model-'),
            dirty:!root.querySelector('.vd-looper-dirty').hidden, focus:root.classList.contains('is-focus')
        };
    }`).Map()
	if loaded["name"].Str() != "Past loop" || loaded["work"].Str() != "History work" || loaded["evaluate"].Str() != "History review" ||
		loaded["finish"].Str() != "History finish" || loaded["rounds"].Str() != "6" || loaded["score"].Str() != "90" || loaded["model"].Str() != "kept-model" {
		t.Fatalf("history settings were not loaded into the editor: %v", loaded)
	}
	if loaded["dirty"].Bool() || loaded["focus"].Bool() {
		t.Fatalf("a loaded run is a clean starting point, not an unsaved edit: %v", loaded)
	}

	page.MustEval(`()=>{ window.__lastLooperBody=''; document.querySelector('.vd-looper-history-rerun').click(); }`)
	mustWaitDump(t, page, "step 6", `()=>document.querySelector('.vd-looper').classList.contains('is-focus') && window.__lastLooperBody.includes('History goal')`)
	if !page.MustEval(`()=>window.__lastLooperPath==='/api/desktop/looper/run' && JSON.parse(window.__lastLooperBody).target_score===90`).Bool() {
		t.Fatal("Run again must start a run with the stored settings")
	}

	// A run restored after a restart arrives paused with its settings only on the
	// server: the editor adopts them, and the reason is spelled out.
	stream.set(map[string]any{"status": "idle", "running": false, "paused": false, "round": 0, "max_rounds": 10, "run_id": 3, "logs": []any{}})
	mustWaitDump(t, page, "step 7", `()=>!document.querySelector('.vd-looper').classList.contains('is-active-run')`)
	page.MustElement(".vd-looper-brief-edit").MustClick()
	mustWaitDump(t, page, "step 8", `()=>!document.querySelector('.vd-looper').classList.contains('is-focus')`)
	page.MustEval(`()=>document.querySelector('.vd-looper-new').click()`)
	mustWaitDump(t, page, "step 9", `()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value===''`)
	interrupted := map[string]any{
		"status": "paused", "running": false, "paused": true, "round": 3, "max_rounds": 7, "resume_from": 3,
		"current_step": "paused", "best_score": 74, "score_history": []int{62, 74}, "target_score": 90,
		"preset_name": "Restored loop", "goal_excerpt": "Restored goal", "pause_reason": "interrupted",
		"run_id": 4, "logs_from": 0, "log_total": 0, "logs": []any{},
	}
	stream.set(interrupted)
	mustWaitDump(t, page, "step 10", `()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value==='Restored goal'`)
	mustWaitDump(t, page, "step 11", `()=>!document.querySelector('.vd-looper-pausenote').hidden`)
	restored := page.MustEval(`()=>({
        note:document.querySelector('.vd-looper-pausenote').textContent,
        resume:!document.querySelector('.vd-looper-resume').hidden,
        dirty:!document.querySelector('.vd-looper-dirty').hidden
    })`).Map()
	if !strings.Contains(restored["note"].Str(), "neu gestartet") || !strings.Contains(restored["note"].Str(), "Runde 4") || !restored["resume"].Bool() || restored["dirty"].Bool() {
		t.Fatalf("an interrupted run must explain itself and be resumable: %v", restored)
	}
	saveLooperShot(t, page, "interrupted.png")

	budgetPaused := map[string]any{}
	for k, v := range interrupted {
		budgetPaused[k] = v
	}
	budgetPaused["pause_reason"] = "budget"
	stream.set(budgetPaused)
	mustWaitDump(t, page, "step 12", `()=>document.querySelector('.vd-looper-pausenote').textContent.includes('Tagesbudget')`)

	// With no settings anywhere, Resume lets the server continue from its checkpoint.
	stream.set(map[string]any{"status": "idle", "running": false, "paused": false, "round": 0, "max_rounds": 10, "run_id": 5, "logs": []any{}})
	mustWaitDump(t, page, "step 13", `()=>!document.querySelector('.vd-looper').classList.contains('is-active-run')`)
	page.MustElement(".vd-looper-brief-edit").MustClick()
	page.MustEval(`()=>{ window.__activeMissing=true; document.querySelector('.vd-looper-new').click(); }`)
	mustWaitDump(t, page, "step 14", `()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value===''`)
	interrupted["run_id"] = 6
	stream.set(interrupted)
	mustWaitDump(t, page, "step 15", `()=>!document.querySelector('.vd-looper-resume').hidden`)
	page.MustEval(`()=>{ window.__lastLooperBody='unset'; document.querySelector('.vd-looper-resume').click(); }`)
	mustWaitDump(t, page, "step 16", `()=>window.__lastLooperBody!=='unset'`)
	if !page.MustEval(`()=>window.__lastLooperPath==='/api/desktop/looper/resume' && window.__lastLooperBody==='{}'`).Bool() {
		t.Fatal("resuming without any editor content must send an empty request")
	}

	// A coded refusal re-enables the controls instead of leaving the window stuck.
	page.MustEval(`()=>{ window.__looperRunError='budget_exceeded'; window.__lastLooperBody='unset'; document.querySelector('.vd-looper-resume').click(); }`)
	mustWaitDump(t, page, "step 17", `()=>window.__lastLooperBody!=='unset'`)
	if !page.MustEval(`()=>!document.querySelector('.vd-looper-resume').disabled`).Bool() {
		t.Fatal("a refused resume must leave the controls usable")
	}

	// A saved draft must not be written over a run the server is still about to
	// report: the run's own settings win once its status arrives.
	page.MustEval(`()=>{ [...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id)); window.__looperRunError=''; window.__activeMissing=false; }`)
	stream.hold(true)
	interrupted["run_id"] = 9
	stream.set(interrupted)
	page.MustEval(`()=>window.localStorage.setItem('aurago.looper.draft.v1', JSON.stringify({presetId:null, fields:{goal:'Draft goal', work:'Draft work', evaluate:'Draft review'}, at:Date.now()}))`)
	page.MustEval(`async()=>{await fixtureOpen('looper')}`)
	mustWaitDump(t, page, "step 18", `()=>!!document.querySelector('.vd-looper')`)
	time.Sleep(500 * time.Millisecond)
	stream.hold(false)
	mustWaitDump(t, page, "step 19", `()=>document.querySelector('.vd-looper').classList.contains('is-active-run')`)
	mustWaitDump(t, page, "step 20", `()=>document.querySelector('.vd-looper textarea[id^="looper-goal-"]').value==='Restored goal'`)
	if !page.MustEval(`()=>document.querySelector('.vd-looper-dirty').hidden && !document.querySelector('.vd-looper-resume').hidden`).Bool() {
		t.Fatal("a stale draft must not replace the settings of a restored run")
	}

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

func saveLooperShot(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "reports", "desktop-looper")
	if extra := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); extra != "" {
		dir = extra
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, name)
	page.MustScreenshot(dest)
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		t.Fatalf("looper screenshot missing or empty: %s (%v)", dest, err)
	}
	t.Logf("wrote %s (%d bytes)", dest, info.Size())
}

// mustWaitDump waits for a page condition and, on failure, reports the Looper
// markup instead of a bare timeout.
func mustWaitDump(t *testing.T, page *rod.Page, label, js string) {
	t.Helper()
	if err := rod.Try(func() { page.Timeout(10 * time.Second).MustWait(js) }); err != nil {
		dump := page.MustEval(`()=>document.querySelector('.vd-looper').outerHTML.replace(/\s+/g,' ').slice(0,2400)`).Str()
		t.Fatalf("%s: %v\n%s", label, err, dump)
	}
}
