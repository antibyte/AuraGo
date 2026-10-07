package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// Exercise the real app modules; only the provider-facing API is replaced.
func noisemakerBrowserOrigin(t *testing.T) string {
	t.Helper()
	var locale map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/desktop/de.json"), &locale); err != nil {
		t.Fatal(err)
	}
	translations, _ := json.Marshal(locale)
	scripts := ""
	for _, name := range []string{"noisemaker-menus", "noisemaker-library", "noisemaker-player", "noisemaker-create", "noisemaker"} {
		scripts += `<script src="/js/desktop/apps/` + name + `.js"></script>`
	}
	markup := `<!doctype html><html lang="de"><meta charset="utf-8">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-noisemaker.css">
<style>body{margin:0}#host{width:1100px;height:760px;font:14px Arial,sans-serif}</style>
<body class="desktop-body" data-theme="standard"><div id="host"></div><script>
window.SYSTEM_LANG='de';window.locale=` + string(translations) + `;</script>` + scripts + `<script>
const esc=v=>String(v??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');
const t=(k,p={})=>Object.entries(p).reduce((s,[key,v])=>s.replaceAll('{{'+key+'}}',String(v)),locale[k]||k);
window.requests=[];window.notices=[];window.enhancements=[];
window.caps={enabled:true,provider_type:'minimax',supports_lyrics:true,llm_available:true,covers_enabled:true,cover_provider:'openai',daily_max:20,daily_used:0};
window.tracks=Array.from({length:6},(_,i)=>({id:i+1,title:'Test song '+(i+1),style:'Piano',prompt:'A calm song',web_path:'/audio.mp3',duration_ms:30000,provider:'fixture',created_at:'2026-10-07T12:00:00Z'}));
window.api=async(path,options={})=>{
 requests.push({path,body:options.body});
if(path.endsWith('/state')){if(window.failState)throw new Error('Network unavailable');return window.stateResponse || structuredClone(caps)}
 if(path.includes('/tracks'))return {items:structuredClone(tracks),total:tracks.length};
 if(path.endsWith('/enhance'))return new Promise(resolve=>enhancements.push(resolve));
 if(path.endsWith('/generate'))return {title:'New song',web_path:'/audio.mp3',daily_used:1};
 return {};
};
window.mount=(readonly=false)=>{NoisemakerApp.render(document.getElementById('host'),'audit',{t,esc,api,readonly,notify:n=>notices.push(n),confirmDialog:async()=>false})};
window.input=(field,value)=>{const e=document.querySelector('[data-nm-field="'+field+'"]');if(e.type==='checkbox')e.checked=value;else e.value=value;e.dispatchEvent(new Event('input',{bubbles:true}))};
window.submitShortcut=()=>document.querySelector('[data-nm-field="idea"]').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',ctrlKey:true,bubbles:true}));
window.generated=()=>requests.filter(r=>r.path.endsWith('/generate'));
mount();</script></html>`
	files := http.FileServer(http.Dir("."))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(markup))
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestDesktopNoisemakerAuditBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	origin := noisemakerBrowserOrigin(t)
	pageFor := func(t *testing.T) *rod.Page {
		page := browser.MustPage(origin).Timeout(45 * time.Second)
		page.MustSetViewport(1400, 960, 1, false)
		page.MustWaitLoad()
		waitForJSBool(t, page, `() => !!document.querySelector('.nm-card, .nm-row')`)
		t.Cleanup(func() { _ = page.Close() })
		return page
	}
	t.Run("responsive_list", func(t *testing.T) {
		page := pageFor(t)
		for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
			for _, width := range []int{1100, 600, 360} {
				page.MustEval(`(theme,width) => {
document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'standard';
document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';
document.getElementById('host').style.width=width+'px';
document.querySelector('[data-nm-pane-btn="library"]').click();
document.querySelector('[data-nm-view="list"]').click();
}`, theme, width)
				waitForJSBool(t, page, fmt.Sprintf(`() => document.querySelector('.noisemaker-app').classList.contains('is-compact') === %t`, width < 860))
				if !page.MustEval(`() => {
const grid=document.querySelector('.nm-grid');const rows=[...document.querySelectorAll('.nm-row')];
return grid.scrollWidth<=grid.clientWidth+1 && rows.length===6 && rows.every(row=>row.getBoundingClientRect().width>=grid.clientWidth-32 && row.querySelector('.nm-card-title').getBoundingClientRect().width>=90);
}`).Bool() {
					t.Fatalf("list overflow or clipped titles: %s/%d", theme, width)
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("noisemaker-%s-%d.png", theme, width)))
				}
			}
		}
		page.MustSetViewport(390, 844, 1, false)
		if !page.MustEval(`() => {const e=document.querySelector('.nm-grid');return e.scrollWidth<=e.clientWidth+1}`).Bool() {
			t.Fatal("viewport fallback breaks list")
		}
	})
	t.Run("enhancement_preserves_edits_and_newer_requests", func(t *testing.T) {
		page := pageFor(t)
		page.MustEval(`() => {input('idea','Original');document.querySelector('[data-nm-enhance="idea"]').click();input('idea','New user draft');enhancements[0]({text:'Stale result'})}`)
		waitForJSBool(t, page, `() => !document.querySelector('[data-nm-enhance="idea"]').disabled`)
		if got := page.MustEval(`() => document.querySelector('[data-nm-field="idea"]').value`).Str(); got != "New user draft" {
			t.Fatalf("enhancement replaced new input: %s", got)
		}
		page.MustEval(`() => {document.querySelector('[data-nm-enhance="idea"]').click();document.querySelector('[data-nm-enhance="random"]').click();enhancements[2]({text:'Newer suggestion'})}`)
		waitForJSBool(t, page, `() => document.querySelector('[data-nm-field="idea"]').value==='Newer suggestion'`)
		page.MustEval(`() => enhancements[1]({text:'Older suggestion'})`)
		waitForJSBool(t, page, `() => !document.querySelector('[data-nm-enhance="idea"]').disabled`)
		if page.MustEval(`() => document.querySelector('[data-nm-field="idea"]').value`).Str() != "Newer suggestion" {
			t.Fatal("older request replaced newer suggestion")
		}
	})
	t.Run("readonly_cover_and_local_validation", func(t *testing.T) {
		page := pageFor(t)
		page.MustEval(`() => mount(true)`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-create-btn]')`)
		if !page.MustEval(`() => {input('idea','Piano');submitShortcut();document.querySelectorAll('[data-nm-enhance]').forEach(e=>e.click());return document.querySelector('[data-nm-create-btn]').disabled && !generated().length && !enhancements.length}`).Bool() {
			t.Fatal("readonly create/enhance sent a request")
		}
		page.MustEval(`() => mount()`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-field="cover"]')`)
		if !page.MustEval(`() => document.querySelector('[data-nm-field="cover"]').getBoundingClientRect().width>0`).Bool() {
			t.Fatal("Simple mode conceals cover option")
		}
		page.MustEval(`() => {input('idea','Piano');input('cover',false);document.querySelector('[data-nm-create-btn]').click()}`)
		waitForJSBool(t, page, `() => generated().length===1`)
		if page.MustEval(`() => JSON.parse(generated()[0].body).cover`).Bool() {
			t.Fatal("disabled cover was requested")
		}
		page.MustEval(`() => {requests=[];caps={enabled:true,provider_type:'acestep',supports_controls:true,supports_lyrics:true,llm_available:false,local:{ready:true,state:'ready',profile:{max_duration:120,lm_model:'',model:'turbo'}}};mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-create-btn]')`)
		page.MustEval(`() => {input('idea','Piano');input('instrumental',false);document.querySelector('[data-nm-mode="custom"]').click()}`)
		if !page.MustEval(`() => {submitShortcut();return document.querySelector('[data-nm-create-btn]').disabled && !generated().length && !document.querySelector('[data-nm-enhance="lyrics"]') && document.querySelector('[data-nm-lyrics-wrap] summary').innerText.includes(locale['desktop.noisemaker_lyrics_required_label'])}`).Bool() {
			t.Fatal("local required lyrics not explained/enforced")
		}
		page.MustEval(`() => input('instrumental',true)`)
		if !page.MustEval(`() => !document.querySelector('[data-nm-field="lyrics"]').required && document.querySelector('[data-nm-lyrics-wrap] summary').innerText.includes(locale['desktop.noisemaker_optional'])`).Bool() {
			t.Fatal("lyrics requirement did not update after changing Instrumental")
		}
		for _, field := range []struct{ name, bad, good string }{
			{"duration_seconds", "999", "120"}, {"bpm", "301", "100"}, {"seed", "-1", "0"}, {"vocal_language", "invalid!", "de"},
		} {
			page.MustEval(`(field,value) => {document.querySelector('[data-nm-mode="custom"]').click();input(field,value);document.querySelector('[data-nm-mode="simple"]').click();submitShortcut()}`, field.name, field.bad)
			if !page.MustEval(`() => document.querySelector('[data-nm-create-btn]').disabled && !generated().length`).Bool() {
				t.Fatalf("Simple mode submitted invalid %s", field.name)
			}
			page.MustEval(`(field,value) => {document.querySelector('[data-nm-mode="custom"]').click();input(field,value)}`, field.name, field.good)
		}
		page.MustEval(`() => {document.querySelector('[data-nm-mode="simple"]').click();submitShortcut()}`)
		waitForJSBool(t, page, `() => generated().length===1`)
		if !page.MustEval(`() => {const p=JSON.parse(generated()[0].body);return p.duration_seconds===120 && p.seed===0 && p.bpm===100 && p.vocal_language==='de'}`).Bool() {
			t.Fatal("valid local controls were lost across mode change")
		}
		if !page.MustEval(`() => {
const form=NoisemakerCreate.create({t,esc,request:api});let sent;
form.setCaps(caps);form.setForm({idea:'Piano',seed:0,instrumental:true});form.on('generate',p=>sent=p);
form.element.querySelector('[data-nm-create-btn]').click();form.dispose();return sent && sent.seed===0;
}`).Bool() {
			t.Fatal("numeric zero seed was lost from a template")
		}
	})
	t.Run("state_failure_retry_and_disabled_setup", func(t *testing.T) {
		page := pageFor(t)
		page.MustEval(`() => {failState=true;mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[role="alert"]')`)
		if !page.MustEval(`() => document.querySelector('[role="alert"]').innerText.includes(locale['desktop.noisemaker_state_error_title']) && !document.querySelector('[data-nm-open-settings]')`).Bool() {
			t.Fatal("network error was presented as missing setup")
		}
		page.MustEval(`() => {failState=false;document.querySelector('[data-nm-recheck]').click()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('.nm-card, .nm-row')`)
		page.MustEval(`() => {stateResponse={status:'error'};mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[role="alert"]')`)
		page.MustEval(`() => {stateResponse=null;caps.supports_controls=true;caps.local={ready:true,state:'ready',profile:{max_duration:120,lm_model:'small'}};mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-create-btn]')`)
		page.MustEval(`() => {input('idea','Preserved draft');stateResponse={status:'error'};requests=[]}`)
		waitForJSBool(t, page, `() => requests.some(r=>r.path.endsWith('/state'))`)
		if !page.MustEval(`() => document.querySelector('[data-nm-field="idea"]').value==='Preserved draft' && !document.querySelector('[data-nm-open-settings]')`).Bool() {
			t.Fatal("failed background state refresh replaced the workbench")
		}
		page.MustEval(`() => {stateResponse=null;caps.enabled=false;mount()}`)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-open-settings]')`)
	})
}
