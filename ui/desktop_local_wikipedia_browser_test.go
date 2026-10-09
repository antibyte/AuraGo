package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// localWikipediaFixtureCSP is the policy the real content route sends
// (localWikipediaContentCSP in internal/server), including connect-src 'none'.
const localWikipediaFixtureCSP = "sandbox allow-same-origin allow-popups allow-popups-to-escape-sandbox; default-src 'self'; script-src 'none'; connect-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; form-action 'none'; frame-ancestors 'self'"

// {{EXTERNAL}} is replaced by an origin that differs from the page's (localhost
// instead of 127.0.0.1) and still reaches the fixture server's sinks.
var localWikipediaFixturePages = map[string]string{
	"Hauptseite": `<!doctype html><html lang="de"><head><meta charset="utf-8"><title>Hauptseite</title></head><body><h1>Willkommen</h1><p>` +
		`<a id="to-berlin" href="Berlin" ping="/ping-sink" attributionsrc="/attr-sink">Berlin</a> · ` +
		`<a id="to-redirect" href="Berlin_(Stadt)">Berlin (Stadt)</a> · ` +
		`<a id="to-missing" href="Gibt_es_nicht">Fehlt</a> · ` +
		`<a id="to-broken" href="Kaputt">Kaputt</a> · ` +
		`<a id="to-outside" href="{{EXTERNAL}}/external-sink" ping="{{EXTERNAL}}/ping-sink" attributionsrc="">Extern</a> · ` +
		`<a id="to-mail" href="mailto:info@example.org">Mail</a> · ` +
		`<a id="to-script" href="javascript:window.parent.injected=true">Skript</a> · ` +
		`<a id="to-data" href="data:text/html,x">Daten</a> · ` +
		`<a id="to-api" href="/api/vault" ping="/ping-sink">Tresor</a> · ` +
		`<a id="to-relative-api" href="../vault">Relativ</a> · ` +
		`<a id="to-anchor">Ohne Ziel</a></p>` +
		`<map name="m"><area id="area-content" shape="rect" coords="0,0,5,5" href="Bern" ping="/ping-sink" alt="Bern"><area id="area-outside" shape="rect" coords="5,5,9,9" href="{{EXTERNAL}}/external-sink" attributionsrc="/attr-sink" alt="Extern"></map>` +
		`<script>window.parent.injected=true</script></body></html>`,
	"Berlin": `<!doctype html><html lang="de"><head><meta charset="utf-8"><title>Berlin</title></head><body><h1>Berlin</h1><p>Berlin ist die Hauptstadt Deutschlands. <a id="to-bern" href="Bern">Bern</a></p></body></html>`,
	"Bern":   `<!doctype html><html lang="de"><head><meta charset="utf-8"><title>Bern</title></head><body><h1>Bern</h1><p>Bern ist die Bundesstadt der Schweiz.</p></body></html>`,
}

// localWikipediaFixture is the fake Desktop API. Its status JSON has the shape
// of the real handler (internal/server/local_wikipedia_desktop_handlers.go).
type localWikipediaFixture struct {
	mu       sync.Mutex
	status   map[string]any
	failCode int
	failBody map[string]any
	log      []string
}

// localWikipediaFixtureStatus builds a status: readable means an edition is
// served (any state but error), loading is false, update_available is null and
// error_code is absent, as in the real JSON. Use localWikipediaFixtureWith to
// add the rest.
func localWikipediaFixtureStatus(state string, canManage, edition, fulltext bool, progress float64) map[string]any {
	status := map[string]any{
		"state": state, "progress": progress, "edition": nil, "fulltext": fulltext,
		"readable": edition && state != "error", "loading": false, "update_available": nil, "can_manage": canManage,
	}
	if edition {
		status["edition"] = map[string]any{"language": "de", "variant": "nopic", "date": "2026-10", "article_count": 3}
	}
	return status
}

// localWikipediaFixtureWith returns a copy of status with the key/value pairs set.
func localWikipediaFixtureWith(status map[string]any, pairs ...any) map[string]any {
	out := make(map[string]any, len(status)+len(pairs)/2)
	for key, value := range status {
		out[key] = value
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i].(string)] = pairs[i+1]
	}
	return out
}

func localWikipediaFixtureUpdate() map[string]any {
	return map[string]any{"date": "2026-11", "size": 19000000000}
}

func (f *localWikipediaFixture) setStatus(status map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
	f.failCode = 0
	f.failBody = nil
}

// setFailure makes /status answer with an HTTP error, like a disabled switch or a failing server.
func (f *localWikipediaFixture) setFailure(code int, body map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCode = code
	f.failBody = body
}

func (f *localWikipediaFixture) record(entry string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, entry)
}

func (f *localWikipediaFixture) requests(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, entry := range f.log {
		if strings.HasPrefix(entry, prefix) {
			out = append(out, strings.TrimPrefix(entry, prefix))
		}
	}
	return out
}

func (f *localWikipediaFixture) resetRequests() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = nil
}

func setLocalWikipediaFixtureContentHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", localWikipediaFixtureCSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func writeLocalWikipediaFixtureContentError(w http.ResponseWriter, status int, code string) {
	setLocalWikipediaFixtureContentHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="aurago-local-wikipedia-error" content="%s"><title>%s</title></head><body></body></html>`, code, code)
}

func (f *localWikipediaFixture) serveContent(w http.ResponseWriter, r *http.Request, path string) {
	f.record("content:" + path)
	switch path {
	case "Berlin_(Stadt)":
		setLocalWikipediaFixtureContentHeaders(w)
		w.Header().Set("Location", "/api/desktop/local-wikipedia/content/Berlin")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusFound)
		return
	case "Kaputt":
		writeLocalWikipediaFixtureContentError(w, http.StatusInternalServerError, "content_failed")
		return
	}
	page, ok := localWikipediaFixturePages[path]
	if !ok {
		writeLocalWikipediaFixtureContentError(w, http.StatusNotFound, "not_found")
		return
	}
	external := "http://" + strings.Replace(r.Host, "127.0.0.1", "localhost", 1)
	setLocalWikipediaFixtureContentHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, strings.ReplaceAll(page, "{{EXTERNAL}}", external))
}

func (f *localWikipediaFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	status, failCode, failBody := f.status, f.failCode, f.failBody
	f.mu.Unlock()
	route := strings.TrimPrefix(r.URL.Path, "/api/desktop/local-wikipedia/")
	if strings.HasPrefix(route, "content/") {
		f.serveContent(w, r, strings.TrimPrefix(route, "content/"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	switch route {
	case "status":
		f.record("status")
		if failCode != 0 {
			w.WriteHeader(failCode)
			_ = json.NewEncoder(w).Encode(failBody)
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	case "suggest":
		f.record("suggest:" + query)
		refs := []map[string]string{}
		for _, title := range []string{"Berlin", "Bern"} {
			if query != "" && strings.HasPrefix(strings.ToLower(title), strings.ToLower(query)) {
				refs = append(refs, map[string]string{"title": title, "path": title})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": refs})
	case "search":
		f.record("search:" + query)
		switch query {
		case "Nichts":
			_ = json.NewEncoder(w).Encode(map[string]any{"fulltext": true, "results": []map[string]string{}})
		case "Fehler":
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "The search failed", "code": "search_failed"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"fulltext": status["fulltext"], "results": []map[string]string{
				{"title": "Berlin", "path": "Berlin_(Stadt)", "snippet": "Berlin ist die Hauptstadt. <script>window.injected=true</script>"},
				{"title": "Bern", "path": "Bern", "snippet": "Bern ist die Bundesstadt."},
			}})
		}
	case "main":
		f.record("main")
		_ = json.NewEncoder(w).Encode(map[string]string{"title": "Hauptseite", "path": "Hauptseite"})
	case "random":
		f.record("random")
		_ = json.NewEncoder(w).Encode(map[string]string{"title": "Bern", "path": "Bern"})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Not found", "code": "not_found"})
	}
}

// localWikipediaFixtureHTML hosts the app module with the Desktop shell's theme
// tokens. ctx.api mirrors the shell's api() (parsed JSON bodies, err.body and
// err.status on failures, no-store); ctx.t resolves the German desktop labels.
const localWikipediaFixtureHTML = `<!doctype html><html lang="de"><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-local-wikipedia.css"><style>
html,body{margin:0;height:100%;font-family:system-ui}
body{--vd-text:#e4e8f0;--vd-accent:#e95420;--vd-coral:#e97c75;--vd-theme-app-bg:#11151c;--vd-theme-panel-bg:#191f28;--vd-theme-panel-bg-strong:rgba(24,30,39,.98);--vd-theme-chrome-bg:rgba(25,32,42,.94);--vd-theme-control-bg:rgba(255,255,255,.055);--vd-theme-control-hover:rgba(255,255,255,.1);--vd-theme-border:rgba(191,210,235,.14);--vd-theme-border-strong:rgba(191,210,235,.28);--vd-theme-muted:#aeb7c6;--vd-theme-accent-soft:rgba(233,84,32,.16);--vd-theme-accent-border:rgba(233,84,32,.38)}
body[data-theme=fruity]{--vd-text:#192334;--vd-accent:#0a84ff;--vd-theme-app-bg:#eff2f6;--vd-theme-panel-bg:#d8e0eb;--vd-theme-panel-bg-strong:rgba(225,232,242,.98);--vd-theme-chrome-bg:rgba(203,210,220,.94);--vd-theme-control-bg:#fafcff;--vd-theme-control-hover:#e2e9f3;--vd-theme-border:rgba(49,66,94,.24);--vd-theme-border-strong:rgba(49,66,94,.56);--vd-theme-muted:#43516a;--vd-theme-accent-soft:rgba(10,132,255,.2);--vd-theme-accent-border:rgba(10,132,255,.26)}
body[data-theme=fruity][data-fruity-mode=dark]{--vd-text:#f5f5f7;--vd-theme-app-bg:#121419;--vd-theme-panel-bg:#1b1e26;--vd-theme-panel-bg-strong:rgba(26,29,37,.98);--vd-theme-chrome-bg:rgba(30,34,43,.94);--vd-theme-control-bg:rgba(255,255,255,.06);--vd-theme-control-hover:rgba(255,255,255,.11);--vd-theme-border:rgba(200,211,233,.14);--vd-theme-border-strong:rgba(200,211,233,.28);--vd-theme-muted:#aeb7c8}
#host{height:100%}
</style></head><body class="desktop-body" data-theme="standard"><div id="host"></div>
<script>window.errors=[];window.calls=0;addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script>
<script src="/js/desktop/apps/local-wikipedia-views.js"></script>
<script src="/js/desktop/apps/local-wikipedia.js"></script>
<script>window.ready=(async()=>{
const labels=await(await fetch('/lang/desktop/de.json')).json();
window.ctx={
t:(k,p)=>{let s=labels[k]||k;for(const [a,b] of Object.entries(p||{}))s=s.replaceAll('{{'+a+'}}',String(b)).replaceAll('{'+a+'}',String(b));return s;},
api:async(path,opts)=>{calls++;const options=Object.assign({credentials:'same-origin',cache:'no-store'},opts||{});const resp=await fetch(path,options);const body=(resp.headers.get('content-type')||'').includes('application/json')?await resp.json():{};if(!resp.ok){const err=new Error(body.error||body.message||('HTTP '+resp.status));err.body=body;err.status=resp.status;throw err;}return body;}
};
window.remount=()=>{LocalWikipediaApp.dispose('test');LocalWikipediaApp.render(document.querySelector('#host'),'test',ctx);};
LocalWikipediaApp.render(document.querySelector('#host'),'test',ctx);
})();</script></body></html>`

func loadLocalWikipediaLabels(t *testing.T) func(key string, params ...string) string {
	t.Helper()
	var labels map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/desktop/de.json"), &labels); err != nil {
		t.Fatal(err)
	}
	return func(key string, params ...string) string {
		text := labels[key]
		if text == "" {
			t.Fatalf("de.json has no %s", key)
		}
		for i := 0; i+1 < len(params); i += 2 {
			text = strings.ReplaceAll(text, "{"+params[i]+"}", params[i+1])
		}
		return text
	}
}

// compactJSON marshals without HTML escaping so it can be compared with JSON.stringify output.
func compactJSON(t *testing.T, value any) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String())
}

// assertJSON compares the JSON text a page script returned with the expected
// Go value, ignoring key order and white space.
func assertJSON(t *testing.T, what, got string, want any) {
	t.Helper()
	canonical := func(raw []byte) string {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("%s: invalid JSON %q: %v", what, raw, err)
		}
		return compactJSON(t, value)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := canonical([]byte(got)), canonical(wantRaw); g != w {
		t.Fatalf("%s\n got: %s\nwant: %s", what, g, w)
	}
}

type localWikipediaBanner struct {
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Settings bool   `json:"settings"`
}

func TestDesktopLocalWikipediaBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	label := loadLocalWikipediaLabels(t)
	ready := localWikipediaFixtureStatus("ready", true, true, true, 1)
	fixture := &localWikipediaFixture{status: ready}
	var pingHits, attrHits, vaultHits, externalHits atomic.Int32
	var externalReferer atomic.Value
	externalReferer.Store("unset")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.Handle("/api/desktop/local-wikipedia/", fixture)
	mux.HandleFunc("/ping-sink", func(w http.ResponseWriter, r *http.Request) { pingHits.Add(1); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/attr-sink", func(w http.ResponseWriter, r *http.Request) { attrHits.Add(1); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/api/vault", func(w http.ResponseWriter, r *http.Request) { vaultHits.Add(1); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/external-sink", func(w http.ResponseWriter, r *http.Request) {
		externalHits.Add(1)
		externalReferer.Store(r.Referer())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>external</title><p>external</p>`)
	})
	mux.HandleFunc("/local-wikipedia-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, localWikipediaFixtureHTML)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/local-wikipedia-fixture").Timeout(240 * time.Second)
	defer page.Close()
	page.MustSetViewport(1280, 860, 1, false)
	page.MustWaitLoad()
	page.MustWait(`()=>!!window.LocalWikipediaApp&&!!window.remount`)

	// Everything goes through page scripts for clicks and focus: rod's MustClick waits for
	// element stability and has hung in this repository.
	screen := func() string { return page.MustEval(`()=>document.querySelector('#host').innerText`).Str() }
	waitFor := func(t *testing.T, what, js string, args ...any) {
		t.Helper()
		if err := page.Timeout(12 * time.Second).Wait(rod.Eval(js, args...)); err != nil {
			t.Fatalf("timed out waiting for %s: %v\nscreen: %q", what, err, screen())
		}
	}
	waitTitle := func(t *testing.T, title string) {
		t.Helper()
		waitFor(t, "article "+title, `(title)=>{const node=document.querySelector('.lw-title');return !!node&&node.textContent===title&&document.querySelector('.lw-frame-error').hidden}`, title)
	}
	click := func(selector string) {
		page.MustEval(`(selector)=>{const el=document.querySelector(selector);if(!el)throw new Error('missing '+selector);el.click()}`, selector)
	}
	focus := func(selector string) {
		page.MustEval(`(selector)=>{const el=document.querySelector(selector);if(!el)throw new Error('missing '+selector);el.focus()}`, selector)
	}
	frameClick := func(id string) {
		page.MustEval(`(id)=>{const el=document.querySelector('.lw-frame').contentDocument.getElementById(id);if(!el)throw new Error('missing '+id);el.click()}`, id)
	}
	mount := func(status map[string]any) {
		fixture.setStatus(status)
		fixture.resetRequests()
		page.MustEval(`()=>window.remount()`)
	}
	mountArticle := func(t *testing.T, status map[string]any) {
		t.Helper()
		mount(status)
		waitTitle(t, "Hauptseite")
	}
	tools := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('.lw-tool')].map(b=>b.dataset.action+':'+b.getAttribute('tabindex')+(b.disabled?':disabled':'')).join(',')`).Str()
	}
	banners := func() string {
		return page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.lw-banner')].map(b=>({kind:b.dataset.kind,text:b.querySelector('span').textContent,settings:!!b.querySelector('[data-action="settings"]')})))`).Str()
	}
	step := func(name string, fn func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v\nscreen: %q", r, screen())
				}
			}()
			fn(t)
		})
	}

	// State screens: the text follows the state and error code and the caller's rights.
	// The edition is not readable in any of them, so search and navigation stay disabled.
	step("state screens", func(t *testing.T) {
		notReadable := func(status map[string]any, pairs ...any) map[string]any {
			return localWikipediaFixtureWith(status, append([]any{"readable", false}, pairs...)...)
		}
		type stateCase struct {
			name, kind, title, text string
			status                  map[string]any
			failCode                int
			failBody                map[string]any
			settings, retry         bool
			progress                string
		}
		var cases []stateCase
		for _, admin := range []bool{true, false} {
			who, pick := "user", func(a, u string) string { return u }
			if admin {
				who, pick = "admin", func(a, u string) string { return a }
			}
			none := localWikipediaFixtureStatus("not_installed", admin, false, false, 0)
			cases = append(cases,
				stateCase{name: "not_installed_" + who, kind: "not_installed", status: none, settings: admin,
					title: label("desktop.local_wikipedia_not_installed_title"), text: label(pick("desktop.local_wikipedia_not_installed_admin", "desktop.local_wikipedia_not_installed_user"))},
				stateCase{name: "interrupted_" + who, kind: "interrupted", status: localWikipediaFixtureStatus("interrupted", admin, false, false, 0.3), settings: admin,
					title: label("desktop.local_wikipedia_interrupted_title"), text: label(pick("desktop.local_wikipedia_interrupted_admin", "desktop.local_wikipedia_interrupted_user"))},
				stateCase{name: "state_unreadable_" + who, kind: "state_unreadable", settings: admin,
					status: localWikipediaFixtureWith(localWikipediaFixtureStatus("error", admin, false, false, 0), "error_code", "state_unreadable"),
					title:  label("desktop.local_wikipedia_error_title"), text: label(pick("desktop.local_wikipedia_state_error_admin", "desktop.local_wikipedia_state_error_user"))},
				stateCase{name: "zim_unreadable_" + who, kind: "error", settings: admin,
					status: localWikipediaFixtureWith(localWikipediaFixtureStatus("error", admin, true, false, 0), "error_code", "zim_unreadable"),
					title:  label("desktop.local_wikipedia_error_title"), text: label(pick("desktop.local_wikipedia_error_admin", "desktop.local_wikipedia_error_user"))},
				stateCase{name: "install_failed_" + who, kind: "install_failed", settings: admin,
					status: localWikipediaFixtureWith(localWikipediaFixtureStatus("error", admin, false, false, 0), "error_code", "download_failed"),
					title:  label("desktop.local_wikipedia_install_failed_title"), text: label(pick("desktop.local_wikipedia_install_failed_admin", "desktop.local_wikipedia_install_failed_user"))},
				stateCase{name: "disabled_" + who, kind: "disabled", settings: admin,
					failCode: http.StatusServiceUnavailable, failBody: map[string]any{"error": "Local Wikipedia is switched off", "code": "disabled", "can_manage": admin},
					title: label("desktop.local_wikipedia_disabled_title"), text: label(pick("desktop.local_wikipedia_disabled_admin", "desktop.local_wikipedia_disabled_user"))},
			)
		}
		cases = append(cases,
			stateCase{name: "loading", kind: "loading",
				status: notReadable(localWikipediaFixtureStatus("not_installed", true, false, false, 0), "loading", true, "error_code", "busy"),
				title:  label("desktop.local_wikipedia_edition_loading_title"), text: label("desktop.local_wikipedia_edition_loading_text")},
			stateCase{name: "downloading", kind: "downloading", progress: "42/100",
				status: localWikipediaFixtureStatus("downloading", true, false, false, 0.42),
				title:  label("desktop.local_wikipedia_downloading_title"), text: label("desktop.local_wikipedia_downloading_text", "percent", "42")},
			stateCase{name: "verifying", kind: "verifying", progress: "indeterminate",
				status: localWikipediaFixtureStatus("verifying", true, false, false, 1),
				title:  label("desktop.local_wikipedia_verifying_title"), text: label("desktop.local_wikipedia_verifying_text")},
			stateCase{name: "failed", kind: "failed", retry: true, failCode: http.StatusInternalServerError, failBody: map[string]any{"error": "boom", "code": "internal"},
				title: label("desktop.local_wikipedia_failed_title"), text: label("desktop.local_wikipedia_failed_text")},
		)
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				if c.failCode != 0 {
					fixture.setFailure(c.failCode, c.failBody)
				} else {
					fixture.setStatus(c.status)
				}
				page.MustEval(`()=>window.remount()`)
				waitFor(t, "state card "+c.kind, `(kind)=>!document.querySelector('.lw-state').hidden&&!!document.querySelector('.lw-state-card[data-state="'+kind+'"]')`, c.kind)
				got := page.MustEval(`()=>{const card=document.querySelector('.lw-state-card');const bar=card.querySelector('progress');return JSON.stringify({
title:card.querySelector('h2').textContent,text:card.querySelector('p').textContent,settings:!!card.querySelector('[data-action="settings"]'),retry:!!card.querySelector('[data-action="retry"]'),
progress:bar?(bar.hasAttribute('value')?bar.getAttribute('value')+'/'+bar.getAttribute('max'):'indeterminate'):'none',
tools:[...document.querySelectorAll('.lw-tool')].every(b=>b.disabled&&b.getAttribute('tabindex')==='-1'),input:document.querySelector('.lw-input').disabled,submit:document.querySelector('.lw-submit').disabled,
banners:document.querySelectorAll('.lw-banner').length,footer:document.querySelector('.lw-footer').textContent,view:[...document.querySelectorAll('.lw-view')].filter(v=>!v.hidden).map(v=>v.dataset.view).join()})}`).Str()
				progress := c.progress
				if progress == "" {
					progress = "none"
				}
				assertJSON(t, "state "+c.kind, got, map[string]any{
					"title": c.title, "text": c.text, "settings": c.settings, "retry": c.retry, "progress": progress,
					"tools": true, "input": true, "submit": true, "banners": 0, "footer": "", "view": "state",
				})
			})
		}
	})

	step("settings button opens the config section in a new tab", func(t *testing.T) {
		mount(localWikipediaFixtureStatus("not_installed", true, false, false, 0))
		waitFor(t, "not installed", `()=>!!document.querySelector('.lw-state-card[data-state="not_installed"] [data-action="settings"]')`)
		page.MustEval(`()=>{window.opened=[];window.open=(...args)=>{opened.push(args);return null}}`)
		click(`.lw-state-card [data-action="settings"]`)
		if got := page.MustEval(`()=>JSON.stringify(opened)`).Str(); got != `[["/config#local_wikipedia","_blank","noopener"]]` {
			t.Fatalf("window.open calls = %s", got)
		}
	})

	step("loading turns into the main page without a reload", func(t *testing.T) {
		loading := localWikipediaFixtureWith(localWikipediaFixtureStatus("not_installed", true, false, false, 0), "readable", false, "loading", true, "error_code", "busy")
		mount(loading)
		waitFor(t, "loading card", `()=>!!document.querySelector('.lw-state-card[data-state="loading"]')`)
		before := page.MustEval(`()=>calls`).Int()
		waitFor(t, "fast status polling while loading", `(before)=>calls>=before+2`, before)
		if !page.MustEval(`()=>document.querySelector('.lw-input').disabled`).Bool() {
			t.Fatal("search must stay disabled while the edition loads")
		}
		fixture.setStatus(ready)
		waitTitle(t, "Hauptseite")
		got := page.MustEval(`()=>JSON.stringify({state:document.querySelector('.lw-state').hidden,article:!document.querySelector('.lw-article').hidden,input:document.querySelector('.lw-input').disabled,frame:document.querySelector('.lw-frame').title})`).Str()
		if want := `{"state":true,"article":true,"input":false,"frame":"` + label("desktop.local_wikipedia_article_frame", "title", "Hauptseite") + `"}`; got != want {
			t.Fatalf("after loading = %s, want %s", got, want)
		}
	})

	step("a failed status offers a retry that recovers", func(t *testing.T) {
		fixture.setFailure(http.StatusInternalServerError, map[string]any{"error": "boom", "code": "internal"})
		page.MustEval(`()=>window.remount()`)
		waitFor(t, "failed card", `()=>!!document.querySelector('.lw-state-card[data-state="failed"] [data-action="retry"]')`)
		before := page.MustEval(`()=>calls`).Int()
		click(`[data-action="retry"]`)
		waitFor(t, "second status request", `(before)=>calls>before&&!!document.querySelector('.lw-state-card[data-state="failed"]')`, before)
		fixture.setStatus(ready)
		click(`[data-action="retry"]`)
		waitTitle(t, "Hauptseite")
	})

	step("banners follow the status and the caller's rights", func(t *testing.T) {
		update := localWikipediaFixtureUpdate()
		november := label("desktop.local_wikipedia_update_available", "date", "November 2026")
		failed := label("desktop.local_wikipedia_update_failed")
		titleOnly := label("desktop.local_wikipedia_title_only")
		for _, c := range []struct {
			name   string
			status map[string]any
			want   []localWikipediaBanner
		}{
			{"admin update available", localWikipediaFixtureWith(ready, "update_available", update), []localWikipediaBanner{{"info", november, true}}},
			{"user update available", localWikipediaFixtureWith(ready, "can_manage", false, "update_available", update), []localWikipediaBanner{{"info", november, false}}},
			{"title-only edition", localWikipediaFixtureWith(ready, "fulltext", false, "error_code", "fulltext_unsupported", "update_available", update), []localWikipediaBanner{{"info", november, true}, {"hint", titleOnly, false}}},
			{"admin failed update", localWikipediaFixtureWith(ready, "error_code", "checksum_mismatch", "update_available", update), []localWikipediaBanner{{"warn", failed, true}, {"info", november, true}}},
			{"user failed update", localWikipediaFixtureWith(ready, "can_manage", false, "error_code", "checksum_mismatch", "update_available", update), []localWikipediaBanner{{"info", november, false}}},
			{"catalog unreachable is not an update failure", localWikipediaFixtureWith(ready, "error_code", "catalog_unreachable"), []localWikipediaBanner{}},
			{"admin interrupted update", localWikipediaFixtureStatus("interrupted", true, true, true, 0.3), []localWikipediaBanner{{"info", label("desktop.local_wikipedia_update_interrupted"), true}}},
			{"user interrupted update", localWikipediaFixtureStatus("interrupted", false, true, true, 0.3), []localWikipediaBanner{{"info", label("desktop.local_wikipedia_update_interrupted"), false}}},
			{"update download", localWikipediaFixtureStatus("downloading", true, true, true, 0.5), []localWikipediaBanner{{"info", label("desktop.local_wikipedia_update_downloading", "percent", "50"), false}}},
			{"update verification", localWikipediaFixtureStatus("verifying", true, true, true, 1), []localWikipediaBanner{{"info", label("desktop.local_wikipedia_update_downloading", "percent", "100"), false}}},
		} {
			// The old edition stays readable in every one of these states.
			mountArticle(t, c.status)
			assertJSON(t, c.name, banners(), c.want)
			if page.MustEval(`()=>document.querySelector('.lw-input').disabled`).Bool() {
				t.Fatalf("%s: search disabled although the edition is readable", c.name)
			}
		}
		// The settings button of a banner opens the config section.
		mountArticle(t, localWikipediaFixtureWith(ready, "update_available", update))
		page.MustEval(`()=>{window.opened=[];window.open=(...args)=>{opened.push(args);return null}}`)
		click(`.lw-banner [data-action="settings"]`)
		if got := page.MustEval(`()=>JSON.stringify(opened)`).Str(); got != `[["/config#local_wikipedia","_blank","noopener"]]` {
			t.Fatalf("window.open calls = %s", got)
		}
		footer := page.MustEval(`()=>document.querySelector('.lw-footer').textContent`).Str()
		if !strings.Contains(footer, label("desktop.local_wikipedia_edition")) || !strings.Contains(footer, "Deutsch · "+label("desktop.local_wikipedia_variant_nopic")+" · "+label("desktop.local_wikipedia_edition_date", "date", "Oktober 2026")) {
			t.Fatalf("footer = %q", footer)
		}
	})

	step("article links are rewritten and ZIM scripts never run", func(t *testing.T) {
		pingHits.Store(0)
		attrHits.Store(0)
		vaultHits.Store(0)
		mountArticle(t, ready)
		if sandbox := page.MustEval(`()=>document.querySelector('.lw-frame').getAttribute('sandbox')`).Str(); sandbox != "allow-same-origin allow-popups allow-popups-to-escape-sandbox" {
			t.Fatalf("frame sandbox = %q", sandbox)
		}
		if page.MustEval(`()=>window.injected===true`).Bool() {
			t.Fatal("article script executed")
		}
		got := page.MustEval(`()=>{const d=document.querySelector('.lw-frame').contentDocument;const attrs=id=>{const el=d.getElementById(id);return el?{href:el.getAttribute('href'),target:el.getAttribute('target'),rel:el.getAttribute('rel'),ping:el.hasAttribute('ping'),attributionsrc:el.hasAttribute('attributionsrc')}:'missing'};
return JSON.stringify({
internal:attrs('to-berlin'),redirect:attrs('to-redirect'),missing:attrs('to-missing'),outside:attrs('to-outside'),mail:attrs('to-mail'),script:attrs('to-script'),data:attrs('to-data'),api:attrs('to-api'),relativeApi:attrs('to-relative-api'),anchor:attrs('to-anchor'),
areaContent:attrs('area-content'),areaOutside:attrs('area-outside'),
pings:d.querySelectorAll('[ping]').length,attributions:d.querySelectorAll('[attributionsrc]').length})}`).Str()
		external := page.MustEval(`()=>document.querySelector('.lw-frame').contentDocument.getElementById('to-outside').getAttribute('href')`).Str()
		if !strings.HasPrefix(external, "http://localhost:") || !strings.HasSuffix(external, "/external-sink") {
			t.Fatalf("external link = %q", external)
		}
		// Content links keep their href and stay in the frame, http(s) links get a new tab without
		// opener or referrer, mailto stays, everything else loses its href; ping and attributionsrc
		// are gone from every link and area.
		link := func(href, target, rel any) map[string]any {
			return map[string]any{"href": href, "target": target, "rel": rel, "ping": false, "attributionsrc": false}
		}
		assertJSON(t, "link rewrite", got, map[string]any{
			"internal": link("Berlin", nil, nil), "redirect": link("Berlin_(Stadt)", nil, nil), "missing": link("Gibt_es_nicht", nil, nil),
			"outside": link(external, "_blank", "noopener noreferrer"), "mail": link("mailto:info@example.org", nil, nil),
			"script": link(nil, nil, nil), "data": link(nil, nil, nil), "api": link(nil, nil, nil), "relativeApi": link(nil, nil, nil), "anchor": link(nil, nil, nil),
			"areaContent": link("Bern", nil, nil), "areaOutside": link(external, "_blank", "noopener noreferrer"),
			"pings": 0, "attributions": 0,
		})
		// Following links: content links stay in the frame, nothing reaches the sinks.
		frameClick("to-api")
		frameClick("to-berlin")
		waitTitle(t, "Berlin")
		if pingHits.Load() != 0 || attrHits.Load() != 0 || vaultHits.Load() != 0 {
			t.Fatalf("link pings = %d, attribution requests = %d, API requests = %d", pingHits.Load(), attrHits.Load(), vaultHits.Load())
		}
	})

	step("external links open a new tab without opener or referrer", func(t *testing.T) {
		externalHits.Store(0)
		externalReferer.Store("unset")
		pingHits.Store(0)
		mountArticle(t, ready)
		pos := page.MustEval(`()=>{const frame=document.querySelector('.lw-frame');const f=frame.getBoundingClientRect();const l=frame.contentDocument.getElementById('to-outside').getBoundingClientRect();return {x:f.left+l.left+l.width/2,y:f.top+l.top+l.height/2}}`)
		page.Mouse.MustMoveTo(pos.Get("x").Num(), pos.Get("y").Num())
		page.Mouse.MustClick(proto.InputMouseButtonLeft)
		var popup *rod.Page
		deadline := time.Now().Add(12 * time.Second)
		for popup == nil && time.Now().Before(deadline) {
			pages, err := browser.Pages()
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range pages {
				if info, err := candidate.Info(); err == nil && strings.HasSuffix(info.URL, "/external-sink") {
					popup = candidate
				}
			}
			if popup == nil {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if popup == nil {
			t.Fatal("the external link did not open a new tab")
		}
		defer popup.Close()
		if !popup.Timeout(10 * time.Second).MustEval(`()=>window.opener===null`).Bool() {
			t.Fatal("the new tab can reach its opener")
		}
		if externalHits.Load() != 1 || externalReferer.Load().(string) != "" {
			t.Fatalf("external requests = %d, referer = %q", externalHits.Load(), externalReferer.Load())
		}
		if pingHits.Load() != 0 {
			t.Fatalf("link ping requests = %d", pingHits.Load())
		}
		// The reader itself stays on its article.
		if title := page.MustEval(`()=>document.querySelector('.lw-title').textContent`).Str(); title != "Hauptseite" {
			t.Fatalf("title after the external click = %q", title)
		}
	})

	step("missing and broken articles show an in-app message, Back returns", func(t *testing.T) {
		mountArticle(t, ready)
		frameClick("to-missing")
		waitFor(t, "missing article message", `(text)=>{const e=document.querySelector('.lw-frame-error');return !e.hidden&&e.textContent===text}`, label("desktop.local_wikipedia_article_missing"))
		// Back returns to the last article that loaded; Forward has nothing to go to.
		if got := tools(); got != "back:-1,forward:-1:disabled,main:0,random:-1" {
			t.Fatalf("tools after a missing article = %s", got)
		}
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
		frameClick("to-broken")
		waitFor(t, "failed article message", `(text)=>{const e=document.querySelector('.lw-frame-error');return !e.hidden&&e.textContent===text}`, label("desktop.local_wikipedia_article_failed"))
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
	})

	step("redirects land on the target article and leave one history entry", func(t *testing.T) {
		mountArticle(t, ready)
		frameClick("to-redirect")
		waitTitle(t, "Berlin")
		if path := page.MustEval(`()=>document.querySelector('.lw-frame').contentWindow.location.pathname`).Str(); path != "/api/desktop/local-wikipedia/content/Berlin" {
			t.Fatalf("frame path after the redirect = %q", path)
		}
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
		if !page.MustEval(`()=>document.querySelector('.lw-tool[data-action="back"]').disabled`).Bool() {
			t.Fatal("the redirect left an extra history entry behind")
		}
		click(`.lw-tool[data-action="forward"]`)
		waitTitle(t, "Berlin")
		if !page.MustEval(`()=>document.querySelector('.lw-tool[data-action="forward"]').disabled`).Bool() {
			t.Fatal("Forward must be disabled at the end of the history")
		}
		// In-article links extend the history; Back and Forward walk it.
		frameClick("to-bern")
		waitTitle(t, "Bern")
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Berlin")
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
		click(`.lw-tool[data-action="forward"]`)
		waitTitle(t, "Berlin")
		click(`.lw-tool[data-action="forward"]`)
		waitTitle(t, "Bern")
		// Random and main page buttons.
		click(`.lw-tool[data-action="main"]`)
		waitTitle(t, "Hauptseite")
		click(`.lw-tool[data-action="random"]`)
		waitTitle(t, "Bern")
	})

	step("suggestions are debounced and keyboard selectable", func(t *testing.T) {
		mountArticle(t, ready)
		focus(".lw-input")
		page.Keyboard.MustType(input.KeyB, input.KeyE, input.KeyR)
		waitFor(t, "two suggestions", `()=>document.querySelectorAll('.lw-option').length===2&&document.querySelector('.lw-input').getAttribute('aria-expanded')==='true'`)
		if got := fixture.requests("suggest:"); len(got) != 1 || got[0] != "ber" {
			t.Fatalf("suggest requests = %v, want one for the final text", got)
		}
		state := func() string {
			return page.MustEval(`()=>{const input=document.querySelector('.lw-input');const list=document.querySelector('.lw-suggest');const active=input.getAttribute('aria-activedescendant');return JSON.stringify({
options:[...list.querySelectorAll('[role="option"]')].map(o=>o.textContent),expanded:input.getAttribute('aria-expanded'),controls:input.getAttribute('aria-controls')===list.id&&list.getAttribute('role')==='listbox',hidden:list.hidden,
active:active===null?null:active.slice(list.id.length+1),selected:[...list.querySelectorAll('[aria-selected="true"]')].map(o=>o.dataset.index).join()})}`).Str()
		}
		wantOpen := func(active any, selected string) map[string]any {
			return map[string]any{"options": []string{"Berlin", "Bern"}, "expanded": "true", "controls": true, "hidden": false, "active": active, "selected": selected}
		}
		assertJSON(t, "open suggestions", state(), wantOpen(nil, ""))
		for _, move := range []struct {
			key    input.Key
			name   string
			active string
		}{
			{input.ArrowDown, "ArrowDown", "0"},
			{input.ArrowDown, "ArrowDown", "1"},
			{input.ArrowDown, "ArrowDown wraps to the first option", "0"},
			{input.ArrowUp, "ArrowUp wraps to the last option", "1"},
			{input.ArrowUp, "ArrowUp", "0"},
		} {
			page.Keyboard.MustType(move.key)
			assertJSON(t, "after "+move.name, state(), wantOpen(move.active, move.active))
		}
		// Escape closes the list but keeps the text; ArrowDown reopens it on the first option.
		page.Keyboard.MustType(input.Escape)
		assertJSON(t, "after Escape", page.MustEval(`()=>JSON.stringify({hidden:document.querySelector('.lw-suggest').hidden,expanded:document.querySelector('.lw-input').getAttribute('aria-expanded'),active:document.querySelector('.lw-input').hasAttribute('aria-activedescendant'),value:document.querySelector('.lw-input').value})`).Str(),
			map[string]any{"hidden": true, "expanded": "false", "active": false, "value": "ber"})
		page.Keyboard.MustType(input.ArrowDown)
		waitFor(t, "reopened suggestions", `()=>document.querySelectorAll('.lw-option').length===2&&document.querySelector('.lw-input').getAttribute('aria-activedescendant')?.endsWith('-0')`)
		page.Keyboard.MustType(input.ArrowDown, input.Enter)
		waitTitle(t, "Bern")
		assertJSON(t, "after choosing a suggestion", page.MustEval(`()=>JSON.stringify({hidden:document.querySelector('.lw-suggest').hidden,expanded:document.querySelector('.lw-input').getAttribute('aria-expanded'),value:document.querySelector('.lw-input').value,articleHidden:document.querySelector('.lw-article').hidden,frameFocused:document.activeElement===document.querySelector('.lw-frame')})`).Str(),
			map[string]any{"hidden": true, "expanded": "false", "value": "Bern", "articleHidden": false, "frameFocused": true})
	})

	step("search lists escaped results and opens the redirecting one", func(t *testing.T) {
		mountArticle(t, ready)
		box := page.MustElement(`.lw-input`)
		box.MustInput("Hauptstadt")
		page.Keyboard.MustType(input.Enter)
		waitFor(t, "two results", `()=>document.querySelectorAll('.lw-result').length===2`)
		assertJSON(t, "results", page.MustEval(`()=>JSON.stringify({heading:document.querySelector('.lw-results-title').textContent,titles:[...document.querySelectorAll('.lw-result strong')].map(n=>n.textContent),paths:[...document.querySelectorAll('.lw-result')].map(n=>n.dataset.path),
snippet:document.querySelector('.lw-result span').textContent,scripts:document.querySelectorAll('.lw-results script').length,title:document.querySelector('.lw-title').textContent,suggestHidden:document.querySelector('.lw-suggest').hidden})`).Str(),
			map[string]any{
				"heading": label("desktop.local_wikipedia_results_title", "query", "Hauptstadt"), "titles": []string{"Berlin", "Bern"}, "paths": []string{"Berlin_(Stadt)", "Bern"},
				"snippet": "Berlin ist die Hauptstadt. <script>window.injected=true</script>", "scripts": 0, "title": "", "suggestHidden": true,
			})
		if page.MustEval(`()=>window.injected===true`).Bool() {
			t.Fatal("a snippet executed")
		}
		if requests := fixture.requests("search:"); len(requests) != 1 || requests[0] != "Hauptstadt" {
			t.Fatalf("search requests = %v", requests)
		}
		// From the results, Back returns to the article that was open.
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
		if !page.MustEval(`()=>document.querySelector('.lw-results').hidden`).Bool() {
			t.Fatal("results stayed visible after Back")
		}
		// The redirecting result lands on its target.
		page.MustEval(`()=>document.querySelector('.lw-search').requestSubmit()`)
		waitFor(t, "two results again", `()=>document.querySelectorAll('.lw-result').length===2`)
		click(`.lw-result`)
		waitTitle(t, "Berlin")
		if path := page.MustEval(`()=>document.querySelector('.lw-frame').contentWindow.location.pathname`).Str(); path != "/api/desktop/local-wikipedia/content/Berlin" {
			t.Fatalf("frame path = %q", path)
		}
		// Empty and failing searches say so.
		box.MustSelectAllText().MustInput("Nichts")
		page.Keyboard.MustType(input.Enter)
		waitFor(t, "no results", `(text)=>document.querySelector('.lw-results .lw-muted')?.textContent===text`, label("desktop.local_wikipedia_no_results"))
		box.MustSelectAllText().MustInput("Fehler")
		page.Keyboard.MustType(input.Enter)
		waitFor(t, "search failure", `(text)=>document.querySelector('.lw-results .lw-error')?.textContent===text`, label("desktop.local_wikipedia_search_failed"))
	})

	step("the toolbar is one tab stop with arrow-key navigation", func(t *testing.T) {
		mountArticle(t, ready)
		assertJSON(t, "toolbar semantics", page.MustEval(`()=>{const nav=document.querySelector('.lw-nav');return JSON.stringify({role:nav.getAttribute('role'),label:nav.getAttribute('aria-label'),names:[...nav.querySelectorAll('.lw-tool')].map(b=>b.getAttribute('aria-label'))})}`).Str(),
			map[string]any{"role": "toolbar", "label": label("desktop.local_wikipedia_toolbar"), "names": []string{label("desktop.back"), label("desktop.forward"), label("desktop.local_wikipedia_main_page"), label("desktop.local_wikipedia_random")}})
		// Back and Forward are disabled on the first article, so the tab stop is the first enabled tool.
		if got := tools(); got != "back:-1:disabled,forward:-1:disabled,main:0,random:-1" {
			t.Fatalf("initial tools = %s", got)
		}
		focus(`.lw-tool[data-action="main"]`)
		for _, move := range []struct {
			key  input.Key
			want string
		}{
			{input.ArrowRight, "random"},
			{input.ArrowRight, "main"},
			{input.ArrowLeft, "random"},
			{input.Home, "main"},
			{input.End, "random"},
		} {
			page.Keyboard.MustType(move.key)
			active := page.MustEval(`()=>document.activeElement.dataset.action`).Str()
			if active != move.want {
				t.Fatalf("after %v the focus is on %q, want %q (%s)", move.key, active, move.want, tools())
			}
			if got := page.MustEval(`()=>[...document.querySelectorAll('.lw-tool')].filter(b=>b.getAttribute('tabindex')==='0').map(b=>b.dataset.action).join()`).Str(); got != move.want {
				t.Fatalf("tab stop after %v = %q, want %q", move.key, got, move.want)
			}
		}
		// A new article enables Back; activating it disables it again and focus moves to the new tab stop.
		click(`.lw-tool[data-action="random"]`)
		waitTitle(t, "Bern")
		if got := tools(); got != "back:-1,forward:-1:disabled,main:-1,random:0" {
			t.Fatalf("tools on the second article = %s", got)
		}
		focus(`.lw-tool[data-action="back"]`)
		if got := tools(); got != "back:0,forward:-1:disabled,main:-1,random:-1" {
			t.Fatalf("tools with the focus on Back = %s", got)
		}
		click(`.lw-tool[data-action="back"]`)
		waitTitle(t, "Hauptseite")
		assertJSON(t, "focus after Back disabled itself", page.MustEval(`()=>JSON.stringify({tools:[...document.querySelectorAll('.lw-tool')].map(b=>b.dataset.action+':'+b.getAttribute('tabindex')+(b.disabled?':disabled':'')).join(),focus:document.activeElement.dataset.action,focusDisabled:document.activeElement.disabled})`).Str(),
			map[string]any{"tools": "back:-1:disabled,forward:0,main:-1,random:-1", "focus": "forward", "focusDisabled": false})
	})

	step("keyboard focus is visible", func(t *testing.T) {
		mountArticle(t, ready)
		focus(".lw-input")
		page.Keyboard.MustType(input.Tab)
		if style := page.MustEval(`()=>{const a=document.activeElement;return a.classList.contains('lw-submit')?getComputedStyle(a).outlineStyle:'wrong:'+a.className}`).Str(); style != "solid" {
			t.Fatalf("submit focus ring = %q", style)
		}
	})

	// Standard and Fruity light/dark at desktop and phone widths.
	step("layout and themes", func(t *testing.T) {
		mountArticle(t, localWikipediaFixtureWith(ready, "fulltext", false, "update_available", localWikipediaFixtureUpdate()))
		dir := filepath.Join("..", "reports", "local-wikipedia")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		measure := `()=>{const box=s=>document.querySelector(s).getBoundingClientRect();const app=document.querySelector('.lw-app');const submit=document.querySelector('.lw-submit');return JSON.stringify({
overflowApp:app.scrollWidth>app.clientWidth+1,overflowDocument:document.documentElement.scrollWidth>document.documentElement.clientWidth+1,
stacked:box('.lw-search').top>=box('.lw-nav').bottom-1,searchFull:box('.lw-search').width>=box('.lw-toolbar').width-24,searchInside:box('.lw-search').right<=box('.lw-app').right+1,
submitWidth:Math.round(box('.lw-submit').width),labelHidden:getComputedStyle(submit.querySelector('span')).display==='none',toolWidth:Math.round(box('.lw-tool').width),inputHeight:Math.round(box('.lw-input').height),
frameHeight:box('.lw-frame').height>200,frameBackground:getComputedStyle(document.querySelector('.lw-frame')).backgroundColor,banners:document.querySelectorAll('.lw-banner').length})}`
		for _, combo := range []struct{ theme, mode string }{{"standard", "dark"}, {"fruity", "light"}, {"fruity", "dark"}} {
			for _, width := range []int{1280, 420} {
				page.MustSetViewport(width, 860, 1, false)
				page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode}`, combo.theme, combo.mode)
				name := fmt.Sprintf("%s-%s-%d", combo.theme, combo.mode, width)
				var got struct {
					OverflowApp      bool   `json:"overflowApp"`
					OverflowDocument bool   `json:"overflowDocument"`
					Stacked          bool   `json:"stacked"`
					SearchFull       bool   `json:"searchFull"`
					SearchInside     bool   `json:"searchInside"`
					LabelHidden      bool   `json:"labelHidden"`
					FrameHeight      bool   `json:"frameHeight"`
					SubmitWidth      int    `json:"submitWidth"`
					ToolWidth        int    `json:"toolWidth"`
					InputHeight      int    `json:"inputHeight"`
					FrameBackground  string `json:"frameBackground"`
					Banners          int    `json:"banners"`
				}
				raw := page.MustEval(measure).Str()
				if err := json.Unmarshal([]byte(raw), &got); err != nil {
					t.Fatal(err)
				}
				if got.OverflowApp || got.OverflowDocument || !got.SearchInside || !got.FrameHeight || got.FrameBackground != "rgb(255, 255, 255)" || got.Banners != 2 {
					t.Fatalf("layout %s: %s", name, raw)
				}
				if width == 420 {
					// Phone width: the search row moves below the navigation and spans the toolbar, the buttons are touch sized and the label is dropped.
					if !got.Stacked || !got.SearchFull || got.SubmitWidth != 40 || got.ToolWidth != 40 || got.InputHeight != 40 || !got.LabelHidden {
						t.Fatalf("narrow layout %s: %s", name, raw)
					}
				} else if got.Stacked || got.ToolWidth != 34 || got.SubmitWidth <= 40 || got.LabelHidden {
					// Wide: navigation and search share one row and the search button keeps its label.
					t.Fatalf("wide layout %s: %s", name, raw)
				}
				page.MustScreenshot(filepath.Join(dir, name+".png"))
			}
		}
		// The results view fits the phone width too.
		page.MustSetViewport(420, 860, 1, false)
		page.MustElement(`.lw-input`).MustInput("Hauptstadt")
		page.Keyboard.MustType(input.Enter)
		waitFor(t, "two results", `()=>document.querySelectorAll('.lw-result').length===2`)
		if page.MustEval(`()=>{const app=document.querySelector('.lw-app');return app.scrollWidth>app.clientWidth+1||document.querySelector('.lw-results').scrollWidth>document.querySelector('.lw-results').clientWidth+1}`).Bool() {
			t.Fatal("results overflow at phone width")
		}
		page.MustSetViewport(1280, 860, 1, false)
	})

	step("status polling follows an update, flags a lost connection and stops with dispose", func(t *testing.T) {
		downloading := localWikipediaFixtureStatus("downloading", true, true, true, 0.5)
		mountArticle(t, downloading)
		staleText := label("desktop.local_wikipedia_status_stale")
		waitFor(t, "update banner", `(text)=>[...document.querySelectorAll('.lw-banner')].some(b=>b.textContent.includes(text))`, label("desktop.local_wikipedia_update_downloading", "percent", "50"))
		before := page.MustEval(`()=>calls`).Int()
		waitFor(t, "status polling during the download", `(before)=>calls>before`, before)
		// One failed poll keeps the article and says the connection was lost.
		fixture.setFailure(http.StatusInternalServerError, map[string]any{"error": "boom", "code": "internal"})
		waitFor(t, "stale banner", `(text)=>[...document.querySelectorAll('.lw-banner[data-kind="warn"]')].some(b=>b.textContent.includes(text))`, staleText)
		got := page.MustEval(`()=>JSON.stringify({title:document.querySelector('.lw-title').textContent,article:!document.querySelector('.lw-article').hidden,state:document.querySelector('.lw-state').hidden,input:document.querySelector('.lw-input').disabled,
download:[...document.querySelectorAll('.lw-banner')].some(b=>b.textContent.includes('50 %'))})`).Str()
		if got != `{"title":"Hauptseite","article":true,"state":true,"input":false,"download":true}` {
			t.Fatalf("while the connection is lost: %s", got)
		}
		// The next successful poll clears the notice and shows the new progress.
		fixture.setStatus(localWikipediaFixtureStatus("downloading", true, true, true, 0.75))
		waitFor(t, "recovered banners", `(stale,progress)=>{const b=[...document.querySelectorAll('.lw-banner')];return !b.some(n=>n.textContent.includes(stale))&&b.some(n=>n.textContent.includes(progress))}`, staleText, label("desktop.local_wikipedia_update_downloading", "percent", "75"))
		// Polling stops with dispose.
		before = page.MustEval(`()=>calls`).Int()
		waitFor(t, "another poll", `(before)=>calls>before`, before)
		page.MustEval(`()=>LocalWikipediaApp.dispose('test')`)
		time.Sleep(200 * time.Millisecond)
		after := page.MustEval(`()=>calls`).Int()
		time.Sleep(3500 * time.Millisecond)
		if page.MustEval(`()=>calls`).Int() != after {
			t.Fatal("polling continued after dispose")
		}
		if src := page.MustEval(`()=>document.querySelector('.lw-frame').getAttribute('src')`).Str(); src != "about:blank" {
			t.Fatalf("frame src after dispose = %q", src)
		}
	})

	if errs := page.MustEval(`()=>errors`).Arr(); len(errs) > 0 {
		t.Fatalf("page errors: %v", errs)
	}
	if page.MustEval(`()=>window.injected===true`).Bool() {
		t.Fatal("ZIM content ran a script")
	}
}
