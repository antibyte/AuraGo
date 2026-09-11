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

	var mu sync.Mutex
	started := false
	runCh := make(chan struct{}, 1)
	runningPayload := map[string]any{
		"status":             "running",
		"running":            true,
		"paused":             false,
		"round":              1,
		"max_rounds":         10,
		"current_step":       "evaluate",
		"best_score":         72,
		"score_history":      []int{72},
		"last_feedback":      "Add a clearer ending.",
		"input_tokens":       120,
		"output_tokens":      80,
		"estimated_cost_usd": 0.02,
		"logs": []map[string]any{
			{"round": 1, "step": "work", "duration_ms": 800, "response": "Wrote the first draft."},
			{"round": 1, "step": "evaluate", "score": 72, "duration_ms": 400, "feedback": "Add a clearer ending."},
		},
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
		mu.Lock()
		started = true
		mu.Unlock()
		select {
		case runCh <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/api/desktop/looper/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", http.StatusInternalServerError)
			return
		}
		writeEvent := func(payload any) {
			raw, err := json.Marshal(payload)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
		mu.Lock()
		already := started
		mu.Unlock()
		if already {
			writeEvent(runningPayload)
			<-r.Context().Done()
			return
		}
		writeEvent(map[string]any{"status": "idle", "running": false, "paused": false, "round": 0, "max_rounds": 10, "logs": []any{}})
		select {
		case <-runCh:
			writeEvent(runningPayload)
			<-r.Context().Done()
		case <-r.Context().Done():
		}
	})

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
        const nativePost=(path,options={})=>new Promise((resolve,reject)=>{
            const xhr=new XMLHttpRequest();
            xhr.open(options.method||'POST',path);
            xhr.onload=()=>resolve();
            xhr.onerror=()=>reject(new Error('looper run signal failed'));
            xhr.send(options.body||null);
        });
        window.fetch=async(url,options={})=>{
            const path=String(url);
            const json=data=>new Response(JSON.stringify(data),{headers:{'Content-Type':'application/json'}});
            if(path.startsWith('/api/desktop/looper/run') || path.startsWith('/api/desktop/looper/resume')){
                await nativePost(path,options);
                return json({ok:true});
            }
            if(path.startsWith('/api/desktop/looper/presets')) return json({presets:[{
                id:1,is_builtin:true,builtin_key:'short_story',name:'Short story',
                goal:'Write Documents/Looper/story.md',work:'Improve the story',evaluate:'Score the prose',
                finish:'',max_rounds:10,target_score:85,stall_rounds:3
            }]});
            if(path.startsWith('/api/desktop/looper/runs')) return json({runs:[]});
            if(path.startsWith('/api/providers')) return json({providers:[]});
            return prev(url,options);
        };
    }`)

	page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('looper');await fixtureOpen('looper');}`)
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
            tabsHidden:getComputedStyle(tabs).display==='none'
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
	page.MustEval(`()=>document.querySelector('.vd-looper-preset').click()`)
	page.MustElement(".vd-looper-start").MustClick()
	page.MustWait(`()=>document.querySelectorAll('.vd-looper-log').length>=2`)

	run := page.MustEval(`()=>{
        const root=document.querySelector('.vd-looper');
        return {
            pane:root.dataset.pane,
            logs:document.querySelectorAll('.vd-looper-log').length,
            status:document.querySelector('.vd-looper-status')?.textContent||''
        };
    }`).Map()
	if run["pane"].Str() != "run" {
		t.Fatalf("start must switch to the run pane, got %q", run["pane"].Str())
	}
	if run["logs"].Int() < 2 {
		t.Fatal("SSE fixture must render work and evaluate timeline entries")
	}
	saveLooperShot(t, page, "running-timeline.png")

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
