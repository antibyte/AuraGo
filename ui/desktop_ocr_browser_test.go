package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopOCRBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var locale map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/ocr/de.json"), &locale); err != nil {
		t.Fatal(err)
	}
	translations, _ := json.Marshal(locale)
	markup := `<!doctype html><html lang="de"><meta charset="utf-8">
<link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-ocr.css">
<style>body{margin:0}#host{width:1100px;height:760px;font:14px Arial,sans-serif}</style><body class="desktop-body" data-theme="standard"><div id="host"></div>
<script src="/js/vendor/purify.min.js"></script><script src="/js/vendor/marked.min.js"></script><script src="/js/desktop/apps/ocr.js"></script><script>
window.locale=` + string(translations) + `;
const esc=v=>String(v??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');
const t=k=>locale[k]||k;
window.requests=[];window.exports=[];window.saves=[];window.calls=0;window.fail=false;window.jobs=[];
const file=(id,state='running')=>({id,name:id+'.pdf',mime_type:'application/pdf',state,page_count:state==='completed'?2:0});
window.api=async(path,options={})=>{
 requests.push({path,method:options.method||'GET',body:options.body});
 if(fail)throw new Error('offline');
 if(path.endsWith('/status'))return {state:'ready',ready:true};
 if(path.endsWith('/uploads')){const f=options.body.get('file');return {id:f.name,name:f.name}};
 if(path.endsWith('/jobs')&&options.method==='POST'){const body=JSON.parse(options.body);jobs=[{id:'job',state:'running',files:body.uploads.concat(body.paths).map(id=>file(id))}];return structuredClone(jobs[0])};
 if(path.endsWith('/jobs')){calls++;return structuredClone(jobs)};
 if(path.endsWith('/cancel')){jobs[0].state='cancelled';jobs[0].finished_at='now';jobs[0].files.forEach(f=>f.state='cancelled');return {}};
 if(path.includes('/result'))return {...jobs[0].files.find(f=>f.id===new URL(path,location.href).searchParams.get('file_id')),pages:[{index:0,text:'Seite eins',markdown:'<script>window.injected=true</scr'+'ipt><img src=x onerror="window.injected=true"><a href="javascript:alert(1)">Link</a> **Sicher**'},{index:1,text:'Seite zwei',markdown:'Seite zwei'}]};
 return {};
};
window.mount=(readonly=false)=>OCRApp.render(document.getElementById('host'),'test',{t,esc,api,readonly,openFileDialog:async()=>({paths:['Documents/existing.pdf']}),saveFileDialog:async v=>saves.push(v),exportDesktopFile:v=>exports.push(v)});
window.drop=()=>{const dt=new DataTransfer();dt.items.add(new File(['%PDF-fixture'],'one.pdf',{type:'application/pdf'}));dt.items.add(new File(['%PDF-fixture'],'two.pdf',{type:'application/pdf'}));document.querySelector('.ocr-app').dispatchEvent(new DragEvent('drop',{bubbles:true,dataTransfer:dt}))};
window.finish=()=>{jobs[0].state='completed';jobs[0].finished_at='now';jobs[0].files.forEach(f=>{f.state='completed';f.page_count=2})};
// The real PDF module is loaded; its input is a bounded synthetic fixture.
import('/js/vendor/pdf.min.js').then(()=>{mount();window.ready=true});
</script></html>`
	files := http.FileServer(http.Dir("."))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, markup)
		case "/api/desktop/ocr/jobs/job/export":
			fmt.Fprint(w, "Seite eins\nIndra OCR by Datalab\nhttps://huggingface.co/datalab-to/indra-ocr/LICENSE")
		case "/api/desktop/ocr/jobs/job/source":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.1\n1 0 obj <</Type/Catalog/Pages 2 0 R>> endobj\n2 0 obj <</Type/Pages/Count 2/Kids[3 0 R 4 0 R]>> endobj\n3 0 obj <</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]>> endobj\n4 0 obj <</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]>> endobj\ntrailer <</Root 1 0 R>>\n%%EOF"))
		default:
			files.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL).MustWaitLoad()
	waitForJSBool(t, page, `()=>window.ready===true`)
	page.MustEval(`()=>drop()`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ocr-queued li').length===2`)
	page.MustElement("[data-action=pick]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.ocr-queued li').length===3`)
	page.MustElement("[data-action=start]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('.ocr-job-state')?.textContent===locale['ocr.state_running']`)
	page.MustEval(`()=>{OCRApp.dispose('test');document.getElementById('host').replaceChildren();finish();mount()}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.ocr-output pre')`)
	if page.MustEval(`()=>requests.some(r=>r.path.endsWith('/cancel'))`).Bool() {
		t.Fatal("app close cancelled server job")
	}
	waitForJSBool(t, page, `()=>!!document.querySelector('.ocr-preview canvas')`)
	page.MustElement("[data-action=next]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('.ocr-output').textContent==='Seite zwei'`)
	page.MustElement(".ocr-format").MustSelect("Markdown")
	waitForJSBool(t, page, `()=>document.querySelector('.ocr-output').textContent.trim()==='Seite zwei'`)
	page.MustElement("[data-action=prev]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('.ocr-output').textContent.includes('Sicher')`)
	if page.MustEval(`()=>!!window.injected||!!document.querySelector('.ocr-output script,.ocr-output img,.ocr-output a[href^="javascript:"]')`).Bool() {
		t.Fatal("unsafe Markdown")
	}
	page.MustElement("[data-action=export]").MustClick()
	page.MustElement("[data-action=zip]").MustClick()
	page.MustElement("[data-action=save]").MustClick()
	waitForJSBool(t, page, `()=>exports.length===2&&saves.length===1`)
	if !page.MustEval(`()=>exports[0].filename==='ocr-result.md'&&exports[1].filename.endsWith('.zip')&&saves[0].content.includes('Indra OCR by Datalab')`).Bool() {
		t.Fatal("export/save lost metadata")
	}
	for _, theme := range []string{"lollipop", "black_gloss"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme;document.getElementById('host').style.width='420px'}`, theme)
		if !page.MustEval(`()=>{const r=document.querySelector('.ocr-app'),p=document.querySelector('.ocr-panels');return r.scrollWidth<=r.clientWidth+1&&getComputedStyle(p).gridTemplateColumns.split(' ').length===1}`).Bool() {
			t.Fatal("narrow window overflow", theme)
		}
		if dir := os.Getenv("AURAGO_OCR_SCREENSHOT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, theme+".png"))
		}
	}
	page.MustEval(`()=>{jobs[0].state='running';delete jobs[0].finished_at;mount()}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-action=cancel]')?.disabled`)
	page.MustElement("[data-action=cancel]").MustClick()
	waitForJSBool(t, page, `()=>jobs[0].state==='cancelled'`)
	page.MustEval(`()=>mount(true)`)
	if !page.MustElement("[data-action=upload]").MustProperty("disabled").Bool() {
		t.Fatal("readonly upload allowed")
	}
	page.MustEval(`()=>{fail=true;mount()}`)
	waitForJSBool(t, page, `()=>document.querySelector('.ocr-live').textContent===locale['ocr.error']`)
	page.MustEval(`()=>OCRApp.dispose('test')`)
}

func TestDesktopOCRSettingsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	markup := `<!doctype html><html><body><div id="content"></div><script>
window.configData={ocr:{enabled:false,agent_access:true,automatic_chat:true,automatic_indexing:true}};window.dirty=false;window.actions=[];
window.t=k=>k;window.attachChangeListeners=()=>{};window.hasUnsavedConfigChanges=()=>dirty;
window.toggleBool=el=>{const key=el.dataset.path.split('.')[1];configData.ocr[key]=!configData.ocr[key];el.classList.toggle('on',configData.ocr[key]);el.setAttribute('aria-checked',String(configData.ocr[key]));dirty=true};
window.fetch=async(url,opts={})=>{actions.push({url,method:opts.method||'GET'});return {ok:true,json:async()=>({state:'ready'})}};
</script><script src="/cfg/ocr.js"></script><script>renderOCRSection({})</script></body></html>`
	files := http.FileServer(http.Dir("."))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, markup)
			return
		}
		files.ServeHTTP(w, r)
	}))
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL).MustWaitLoad()
	if !page.MustEval(`()=>document.querySelectorAll('[role=switch]').length===4&&[...document.querySelectorAll('.cfg-actions button')].every(b=>b.disabled)`).Bool() {
		t.Fatal("disabled defaults or controls wrong")
	}
	page.MustElement("[data-path='ocr.enabled']").MustClick()
	if !page.MustEval(`()=>dirty&&configData.ocr.enabled&&[...document.querySelectorAll('.cfg-actions button')].every(b=>b.disabled)`).Bool() {
		t.Fatal("unsaved setup allowed")
	}
	page.MustEval(`()=>{dirty=false;document.querySelector('.cfg-section').dispatchEvent(new Event('change'))}`)
	page.MustElement(".cfg-actions button").MustClick()
	waitForJSBool(t, page, `()=>actions.some(a=>a.url==='/api/ocr/setup'&&a.method==='POST')`)
	page.MustEval(`()=>window.dispatchEvent(new Event('cfg:section-leave'))`)
}

func TestOCRTranslations(t *testing.T) {
	var reference map[string]string
	for _, locale := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var keys map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/ocr/"+locale+".json"), &keys); err != nil {
			t.Fatal(err)
		}
		if reference == nil {
			reference = keys
		}
		if len(keys) != len(reference) {
			t.Fatal(locale, "key count mismatch")
		}
		for key := range reference {
			if keys[key] == "" {
				t.Fatal(locale, key)
			}
		}
	}
}
