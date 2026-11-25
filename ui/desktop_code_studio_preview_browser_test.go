package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const previewFixtureBase = "/code-studio-preview/fixture-token/"

// registerPreview serves the HTML preview routes like internal/server does: the token endpoint, the sandbox CSP,
// the real live.js (read from the server package), live.js injected right after <head> with the data-aurago-version
// attribute, 404 for missing files and the version long-poll that wakes when the fixture backend is written to.
func (b *codeStudioFixtureBackend) registerPreview(mux *http.ServeMux, writeJSON func(http.ResponseWriter, interface{})) {
	live, err := os.ReadFile("../internal/server/code_studio_preview_live.js")
	if err != nil {
		live = []byte("/* live.js missing: " + err.Error() + " */")
	}
	mux.HandleFunc("/api/code-studio/preview", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Path string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		b.mu.Lock()
		version := b.previewVersion
		b.mu.Unlock()
		writeJSON(w, map[string]interface{}{"status": "ok", "base": previewFixtureBase, "url": previewFixtureBase + strings.TrimPrefix(body.Path, "/workspace/"), "token": "fixture-token", "path": body.Path, "workspace": "local", "version": version, "expires_at": time.Now().Add(12 * time.Hour).UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc(previewFixtureBase, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads; frame-ancestors 'self'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		rel := strings.TrimPrefix(r.URL.Path, previewFixtureBase)
		switch rel {
		case "__aurago/live.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(live)
			return
		case "__aurago/version":
			since, _ := strconv.Atoi(r.URL.Query().Get("since"))
			deadline := time.Now().Add(20 * time.Second)
			for time.Now().Before(deadline) && r.Context().Err() == nil {
				b.mu.Lock()
				version := b.previewVersion
				b.mu.Unlock()
				if version != since {
					writeJSON(w, map[string]interface{}{"version": version, "paths": []string{}, "full": true})
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			writeJSON(w, map[string]interface{}{"version": since, "paths": []string{}, "full": false})
			return
		}
		if rel == "" || strings.HasSuffix(rel, "/") {
			rel += "index.html"
		}
		b.mu.Lock()
		content, ok := b.files["/workspace/"+rel]
		version := b.previewVersion
		b.mu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tag := `<script src="` + previewFixtureBase + `__aurago/live.js" data-aurago-version="` + strconv.Itoa(version) + `"></script>`
		if i := strings.Index(strings.ToLower(content), "<head>"); i >= 0 {
			i += len("<head>")
			content = content[:i] + tag + content[i:]
		} else {
			content += tag
		}
		_, _ = w.Write([]byte(content))
	})
}

// Browser smoke for the HTML preview pane: sandboxed iframe, console badge, live reload after a save and the
// "open in new tab" token URL.
func TestDesktopCodeStudioPreviewBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files["/workspace/index.html"] = "<!doctype html><html><head><title>Fixture</title></head><body><h1 id=\"hi\">Hallo Vorschau</h1><script>console.error('smoke boom')</script></body></html>"
	backend.mu.Unlock()
	browserErrors := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }
	scenario := func(name, js string, args ...interface{}) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Fatalf("%s: eval failed: %v; browser errors: %s", name, err, browserErrors())
		}
		if !result.Value.Get("ok").Bool() {
			t.Fatalf("%s: %s; browser errors: %s", name, result.Value.JSON("", ""), browserErrors())
		}
	}
	frameText := func() string {
		t.Helper()
		return page.MustElement("[data-html-preview] [data-hp-frame]").MustFrame().MustEval(`()=>{const h=document.getElementById('hi');return h?h.textContent:''}`).Str()
	}
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v; errors=%s", err, browserErrors())
	}
	// Record window.open instead of spawning tabs.
	page.MustEval(`()=>{window.__opened=[];window.open=(...args)=>{window.__opened.push(args);return null;};}`)

	scenario("open the html file, the eye button appears", `async()=>{
		await CodeStudioApp.openFile('/workspace/index.html');
		const eye=await csWait(()=>csRoot().querySelector('[data-breadcrumbs] [data-hp-eye]'),10000);
		return {ok:eye.getAttribute('aria-pressed')==='false'&&csRoot().dataset.preview==='hidden'};
	}`)

	scenario("Ctrl+Shift+V shows the sandboxed preview", `async()=>{
		const content=await csWait(()=>csRoot().querySelector('.cm-content'),5000);
		content.focus();
		csKey(content,{key:'V',code:'KeyV',ctrlKey:true,shiftKey:true});
		const pane=await csWait(()=>{const el=csRoot().querySelector('[data-html-preview]');return el&&!el.hidden&&el.querySelector('[data-hp-frame]')&&el;},8000);
		await csWait(()=>csState().htmlPreview.status==='ready',10000).catch(()=>{throw new Error('status='+csState().htmlPreview.status+' '+csState().htmlPreview.errorDetail+' src='+pane.querySelector('[data-hp-frame]').getAttribute('src'));});
		const frame=pane.querySelector('[data-hp-frame]');
		const sandbox=frame.getAttribute('sandbox');
		const head=await fetch(frame.src,{method:'HEAD'});
		const csp=head.headers.get('content-security-policy')||'';
		return {ok:csRoot().dataset.preview==='visible'&&pane.classList.contains('code-studio-htmlpreview')&&sandbox==='allow-scripts allow-forms allow-modals allow-popups allow-downloads'&&!/allow-same-origin/.test(sandbox)&&/^sandbox /.test(csp)&&!/allow-same-origin/.test(csp)&&frame.getAttribute('src').endsWith('/code-studio-preview/fixture-token/index.html')&&csRoot().querySelector('[data-hp-eye]').getAttribute('aria-pressed')==='true',sandbox,csp,src:frame.getAttribute('src')};
	}`)
	if got := frameText(); got != "Hallo Vorschau" {
		t.Fatalf("preview frame did not render the page, got %q; errors=%s", got, browserErrors())
	}

	scenario("console.error from the page reaches the badge", `async()=>{
		const pane=csRoot().querySelector('[data-html-preview]');
		const badge=pane.querySelector('[data-hp-console-count]');
		await csWait(()=>!badge.hidden&&badge.textContent==='1',8000);
		window.postMessage({auragoPreview:1,kind:'console',text:'forged'},'*');
		await new Promise(resolve=>setTimeout(resolve,300));
		const count=badge.textContent;
		pane.querySelector('[data-hp-action="console"]').click();
		const entries=[...pane.querySelectorAll('.cs-hp-console-entry pre')].map(el=>el.textContent);
		return {ok:count==='1'&&entries.length===1&&entries[0].includes('smoke boom'),count,entries};
	}`)

	scenario("saving reloads the preview", `async()=>{
		const tab=await csWait(()=>csState().openTabs.find(item=>item.path==='/workspace/index.html'&&item.view&&item.view.state),10000);
		const text=tab.view.state.doc.toString();
		const at=text.indexOf('Hallo Vorschau');
		tab.view.dispatch({changes:{from:at,to:at+'Hallo Vorschau'.length,insert:'Neu geladen'}});
		await csWait(()=>tab.modified&&!csRoot().querySelector('[data-hp-dirty]').hidden,4000);
		await CodeStudioApp.saveCurrentFile();
		await csWait(()=>!tab.modified&&csRoot().querySelector('[data-hp-dirty]').hidden,6000);
		return {ok:true};
	}`)
	deadline := time.Now().Add(15 * time.Second)
	for frameText() != "Neu geladen" {
		if time.Now().After(deadline) {
			t.Fatalf("preview did not live-reload after the save, still %q; errors=%s", frameText(), browserErrors())
		}
		time.Sleep(200 * time.Millisecond)
	}

	scenario("open in new tab uses the token URL", `async()=>{
		csRoot().querySelector('[data-html-preview] [data-hp-action="external"]').click();
		const calls=window.__opened;
		return {ok:calls.length===1&&calls[0][0]==='/code-studio-preview/fixture-token/index.html'&&String(calls[0][1])==='_blank'&&String(calls[0][2]).includes('noopener'),calls};
	}`)

	scenario("the eye button closes the preview", `async()=>{
		csRoot().querySelector('[data-hp-eye]').click();
		await csWait(()=>csRoot().dataset.preview==='hidden'&&csRoot().querySelector('[data-html-preview]').hidden,3000);
		return {ok:true};
	}`)

	scenario("no browser errors", `()=>({ok:fixtureErrors.length===0,errors:fixtureErrors})`)
}
