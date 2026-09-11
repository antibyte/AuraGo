package ui

import (
	"encoding/json"
	"github.com/go-rod/rod/lib/proto"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalMusicConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(30 * time.Second)
	page.MustWaitLoad()
	page.MustSetViewport(390, 844, 1, false)
	defer func() {
		_, _ = page.Eval(`() => window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		_ = page.Close()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
  configData.music_generation={enabled:true,provider:'aurago-acestep-local',local:{backend:'auto',device:'auto',vram_reserve_gb:0,timeout_seconds:1800}};
  AuraConfigState.init(configData);
  const original=window.fetch;window.musicActions=[];
  window.musicRuntime={state:'ready',ready:true,release_ready:true,devices:[],profile:{device:{name:'<img src=x onerror=window.musicInjection=true>',free_gb:12},model:'turbo',lm_model:'',max_duration:120}};
  window.fetch=async(url,options={})=>{
   if(!String(url).startsWith('/api/music-generation/'))return original(url,options);
   musicActions.push({url,method:options.method||'GET',body:options.body});
   const response=(value,status=200)=>new Response(JSON.stringify(value),{status,headers:{'Content-Type':'application/json'}});
   if(options.method==='POST'&&window.musicActionError)return response({message:musicActionError},403);
   if(options.method==='POST'&&window.holdMusicAction)return new Promise(resolve=>window.finishMusicAction=value=>{window.musicRuntime=value;resolve(response(value,202));});
   if(options.method!=='POST'&&window.failMusicStatus)throw new Error('offline');
   if(options.method!=='POST'&&window.holdMusicStatus){window.holdMusicStatus=false;return new Promise(resolve=>window.finishMusicStatus=value=>resolve(response(value)));}
   return response(musicRuntime);
  };
 }`)
	page.MustEval(`async () => {await selectSection('music_generation');resetDirtySnapshot();}`)
	waitForJSBool(t, page, `() => document.querySelector('#music-local-status')?.textContent.includes('turbo')`)
	if !page.MustEval(`() => document.querySelector('[data-path="music_generation.local.vram_reserve_gb"]').value==='0' && !window.musicInjection && !document.querySelector('#music-local-status img')`).Bool() {
		t.Fatal("unsafe status or lost reserve zero")
	}
	page.MustEval(`async () => {isDirty=true;await musicLocalAction('recheck');}`)
	if page.MustEval(`() => musicActions.some(r=>r.method==='POST')`).Bool() {
		t.Fatal("unsaved setup ran")
	}
	page.MustEval(`async () => {isDirty=false;await musicLocalAction('recheck');}`)
	if !page.MustEval(`() => musicActions.some(r=>r.method==='POST' && JSON.parse(r.body).action==='recheck')`).Bool() {
		t.Fatal("recheck did not post")
	}
	page.MustEval(`async () => {await musicLocalRefresh();}`)
	if page.MustEval(`() => document.querySelector('[data-path="music_generation.local.vram_reserve_gb"]').value!=='0'`).Bool() {
		t.Fatal("poll overwrote settings")
	}
	// Hold both an older status response and the command response to verify
	// immediate feedback and prevent late polling from undoing that feedback.
	page.MustEval(`() => {window.holdMusicAction=true;window.holdMusicStatus=true;window.oldMusicPoll=musicLocalRefresh();window.postsBefore=musicActions.filter(r=>r.method==='POST').length;}`)
	page.MustElement(`[data-music-local-action="start"]`).MustClick()
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').textContent.includes('gestartet') && [...document.querySelectorAll('[data-music-local-action]')].every(b=>b.disabled)`).Bool() {
		t.Fatal("slow action has no immediate feedback or duplicate protection")
	}
	page.MustEval(`async () => {await musicLocalAction('start');await musicLocalAction('stop');finishMusicStatus({state:'stopped',ready:false});await oldMusicPoll;}`)
	if !page.MustEval(`() => musicActions.filter(r=>r.method==='POST').length===postsBefore+1 && document.querySelector('#music-local-feedback').textContent.includes('gestartet')`).Bool() {
		t.Fatal("duplicate submission or stale status overwrote action")
	}
	page.MustEval(`() => finishMusicAction({state:'probing',pending:true,ready:false,release_ready:true,devices:[]})`)
	waitForJSBool(t, page, `() => document.querySelector('#music-local-status').textContent.includes('Hardwareprüfung') && !document.querySelector('[data-music-local-action="stop"]').disabled`)
	if !page.MustEval(`() => document.querySelector('[data-music-local-action="start"]').disabled && !document.querySelector('#music-local-progress').hidden && !document.querySelector('#music-local-progress').hasAttribute('value')`).Bool() {
		t.Fatal("background work missing progress or duplicate protection")
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		page.MustElement("#music-local-actions").MustScrollIntoView()
		if err := os.WriteFile(filepath.Join(dir, "music-action-progress.png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustEval(`async () => {window.musicRuntime={state:'ready',pending:false,ready:true,release_ready:true,devices:[]};await musicLocalRefresh();}`)
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').classList.contains('is-success') && !document.querySelector('[data-music-local-action="start"]').disabled`).Bool() {
		t.Fatal("start completion missing")
	}
	page.MustElement(`[data-music-local-action="stop"]`).MustClick()
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').textContent.includes('gestoppt') && document.querySelector('[data-music-local-action="stop"]').disabled`).Bool() {
		t.Fatal("stop feedback missing")
	}
	page.MustEval(`() => finishMusicAction({state:'ready',pending:true,ready:true,release_ready:true,devices:[]})`)
	waitForJSBool(t, page, `() => document.querySelector('#music-local-status').textContent.includes('Aktion wird ausgeführt')`)
	page.MustEval(`async () => {window.musicRuntime={state:'stopped',pending:false,ready:false,release_ready:true,devices:[]};await musicLocalRefresh();}`)
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').textContent.includes('Modelle bleiben erhalten') && document.querySelector('#music-local-feedback').classList.contains('is-success')`).Bool() {
		t.Fatal("stop completion missing")
	}
	page.MustEval(`async () => {window.musicActionError='<img src=x onerror=window.musicInjection=true>';await musicLocalAction('recheck');await musicLocalRefresh();}`)
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').textContent.includes('Aktion fehlgeschlagen') && !document.querySelector('#music-local-feedback img') && !window.musicInjection`).Bool() {
		t.Fatal("poll erased action failure or injected markup")
	}
	page.MustEval(`() => {window.musicActionError='';}`)
	page.MustElement(`[data-music-local-action="recheck"]`).MustClick()
	page.MustEval(`() => finishMusicAction({state:'error',error_code:'acestep_release_not_published',pending:true,ready:false,release_ready:false,devices:[]})`)
	waitForJSBool(t, page, `() => document.querySelector('#music-local-status').textContent.includes('AuraGo aktualisieren')`)
	page.MustEval(`async () => {window.musicRuntime.pending=false;await musicLocalRefresh();}`)
	if !page.MustEval(`() => document.querySelector('#music-local-feedback').classList.contains('is-danger') && document.querySelector('#music-local-feedback').textContent.includes('AuraGo aktualisieren') && !document.querySelector('[data-music-local-action="recheck"]').disabled`).Bool() {
		t.Fatal("background failure was not reported")
	}
	page.MustEval(`async () => {window.failMusicStatus=true;await musicLocalRefresh();}`)
	if !page.MustEval(`() => document.querySelector('#music-local-status').textContent.includes('Status nicht erreichbar')`).Bool() {
		t.Fatal("lost status connection has no feedback")
	}
}

func TestLocalMusicNoisemakerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var locale map[string]string
	_ = json.Unmarshal(mustReadUIFile(t, "lang/desktop/en.json"), &locale)
	translations, _ := json.Marshal(locale)
	markup := `<!doctype html><meta charset="utf-8"><style>` + string(mustReadUIFile(t, "css/desktop-app-noisemaker.css")) + `body{margin:0;width:360px}#host{height:900px}input{max-width:100%}</style><div id="host"></div><script>window.SYSTEM_LANG='en';window.locale=` + string(translations) + `;</script><script>` + string(mustReadUIFile(t, "js/desktop/apps/noisemaker-library.js")) + `</script><script>` + string(mustReadUIFile(t, "js/desktop/apps/noisemaker.js")) + `</script><script>
 window.requests=[];window.caps={enabled:true,supports_controls:true,supports_lyrics:true,local:{ready:true,state:'ready',profile:{max_duration:120,lm_model:'',model:'turbo'}}};
 const ctx={t:k=>locale[k]||k,esc:v=>String(v??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),notify:()=>{},api:async(path,options={})=>{
 requests.push({path,body:options.body});if(path.endsWith('/state'))return caps;if(path.includes('/tracks'))return {items:[],total:0};if(path.endsWith('/generate'))return {title:'Piano',web_path:'',daily_used:1};return {};
 }};NoisemakerApp.render(document.getElementById('host'),'music-test',ctx);
 </script>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(markup))
	}))
	defer server.Close()
	browser := newSmokeBrowser(t)
	page := browser.MustPage(server.URL).Timeout(20 * time.Second)
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 360, Height: 900, DeviceScaleFactor: 1, Mobile: false})
	page.MustWaitLoad()
	defer page.Close()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-nm-field="seed"]')`)
	if !page.MustEval(`() => {const grid=document.querySelector('.nm-local-controls');return grid.scrollWidth<=grid.clientWidth+1;}`).Bool() {
		t.Fatal("controls overflow narrow view")
	}
	page.MustEval(`() => {for(const [field,value] of Object.entries({idea:'Piano',duration_seconds:'120',seed:'0',bpm:'100',vocal_language:'de'})){const el=document.querySelector('[data-nm-field="'+field+'"]');el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));}}`)
	if !page.MustElement(`[data-nm-create-btn]`).MustProperty("disabled").Bool() {
		t.Fatal("missing local lyrics accepted")
	}
	page.MustEval(`() => {const el=document.querySelector('[data-nm-field="instrumental"]');el.checked=true;el.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-nm-create-btn]`).MustClick()
	waitForJSBool(t, page, `() => requests.some(r=>r.path.endsWith('/generate'))`)
	if !page.MustEval(`() => {const p=JSON.parse(requests.find(r=>r.path.endsWith('/generate')).body);return p.seed===0&&p.bpm===100&&p.duration_seconds===120&&p.vocal_language==='de';}`).Bool() {
		t.Fatal("controls not forwarded")
	}
	if page.MustEval(`() => requests.some(r=>r.path.startsWith('/api/music-generation'))`).Bool() {
		t.Fatal("desktop used admin surface")
	}
	page.MustEval(`() => NoisemakerApp.dispose('music-test')`)
}

func TestLocalMusicLocaleCoverage(t *testing.T) {
	for _, language := range strings.Fields("cs da de el en es fr hi it ja nl no pl pt sv zh") {
		for _, file := range []string{"lang/config/music_generation/" + language + ".json", "lang/desktop/" + language + ".json"} {
			var values map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, file), &values); err != nil {
				t.Fatal(err)
			}
			prefix := "config.music_gen.state_"
			if strings.Contains(file, "/desktop/") {
				prefix = "desktop.noisemaker_local_"
			}
			for _, state := range strings.Fields("disabled probing downloading starting loading testing ready busy stopped error") {
				if values[prefix+state] == "" {
					t.Errorf("missing %s in %s", state, file)
				}
			}
			if !strings.Contains(file, "/desktop/") {
				for _, key := range strings.Fields("action_start action_stop action_recheck action_start_done action_stop_done action_recheck_done action_pending action_failed status_unavailable") {
					if values["config.music_gen."+key] == "" {
						t.Errorf("missing %s in %s", key, file)
					}
				}
			}
		}
	}
}
