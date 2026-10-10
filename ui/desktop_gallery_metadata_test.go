package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGalleryMetadataTranslations(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/apps/gallery-metadata.js") + readDesktopAssetText(t, "js/desktop/apps/gallery-view.js")
	keys := map[string]bool{}
	for _, match := range regexp.MustCompile(`['"]((?:desktop\.)?gallery_[a-z_]+)['"]`).FindAllStringSubmatch(source, -1) {
		key := match[1]
		if !strings.HasPrefix(key, "desktop.") {
			key = "desktop." + key
		}
		keys[key] = true
	}
	for _, locale := range desktopGalleryLocales {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/desktop/"+locale+".json"), &values); err != nil {
			t.Fatal(err)
		}
		for key := range keys {
			if values[key] == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
	}
}

func galleryMetadataBrowserOrigin(t *testing.T) string {
	t.Helper()
	translations := mustReadUIFile(t, "lang/desktop/de.json")
	scripts := ""
	for _, name := range []string{"gallery-library", "gallery-metadata", "gallery-view", "gallery-menus", "gallery-lightbox", "gallery"} {
		scripts += `<script src="/js/desktop/apps/` + name + `.js"></script>`
	}
	markup := `<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-gallery.css"><style>body{margin:0}#host{width:1100px;height:760px;font:14px Arial,sans-serif}</style><body class="desktop-body" data-theme="standard"><div id="host"></div>` + scripts + `<script>
const locale=` + string(translations) + `;
const t=(k,p={})=>Object.entries(p).reduce((s,[key,v])=>s.replaceAll('{{'+key+'}}',String(v)),locale[k]||k);
const esc=v=>String(v??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');
window.files=Array.from({length:21},(_,i)=>({name:'photo-'+i+'.png',path:'Photos/photo-'+i+'.png',web_path:'/image.svg',media_kind:'image',size:1234,mod_time:'2026-10-10T10:00:00Z'}));
window.entries=Object.fromEntries(files.map((f,i)=>[f.path,{description:i?'A green forest':'Music cover: mountain',tags:i?['nature']:['noisemaker','cover'],prompt:'Original image intent',source:'generate_image',version:'v0'}]));
window.categories=[{name:'Musikcover',keywords:['cover']}];window.requests=[];window.confirmations=[];window.accept=false;window.hold=false;
window.api=async(url,opt={})=>{
const body=opt.body?JSON.parse(opt.body):null;requests.push({url,method:opt.method||'GET',body});
if(url.includes('/files?'))return {files:structuredClone(files),has_more:false};
if(url.endsWith('/categories')){if(body)categories=body.categories;return {categories:structuredClone(categories),version:'rules1'}}
if(url.endsWith('/metadata')){
if(!body)return {items:structuredClone(entries),empty:{version:'empty',tags:[]}};
if(hold)await new Promise((resolve,reject)=>{opt.signal.addEventListener('abort',()=>reject(new DOMException('Aborted','AbortError')),{once:true})});
const old=entries[body.path];entries[body.path]={...old,description:opt.method==='POST'?'AI mountain description':body.description,tags:opt.method==='POST'?[...old.tags,'mountain']:body.tags,version:old.version+'1'};return structuredClone(entries[body.path]);}
throw Error('Unexpected API '+url);
};
window.mount=(readonly=false)=>{GalleryApp.render(document.getElementById('host'),'gallery-test',{t,esc,api,readonly,mediaPreviewURL:()=>'/image.svg',confirmDialog:async(title,message)=>{confirmations.push({title,message});return accept},notify:()=>{}})};
window.setInput=(selector,value)=>{const e=document.querySelector(selector);e.value=value;e.dispatchEvent(new Event('input',{bubbles:true}))};
window.postCount=()=>requests.filter(r=>r.method==='POST').length;
mount();</script></html>`
	files := http.FileServer(http.Dir("."))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(markup))
			return
		}
		if r.URL.Path == "/image.svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="120"><rect width="120" height="120" fill="#416873"/><path d="M0 120L70 25L120 120" fill="#afbfa3"/></svg>`))
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestGalleryMetadataBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(galleryMetadataBrowserOrigin(t)).Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1400, 960, 1, false)
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-gallery-item]').length===21 && !document.querySelector('[data-gallery-analyze-all]').disabled`)
	page.MustEval(`()=>{const s=document.querySelector('[data-gallery-category]');s.value='Musikcover';s.dispatchEvent(new Event('change'))}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-gallery-item]').length===1`)
	page.MustEval(`()=>{const s=document.querySelector('[data-gallery-category]');s.value='';s.dispatchEvent(new Event('change'));setInput('[data-gallery-search]','mountain')}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-gallery-item]').length===1`)
	page.MustEval(`()=>{document.querySelector('[data-gallery-search-clear]').click();document.querySelector('[data-gallery-analyze-all]').click()}`)
	waitForJSBool(t, page, `()=>confirmations.length===1`)
	if !page.MustEval(`()=>postCount()===0 && confirmations[0].message.includes('21') && confirmations[0].message.includes('erhöhte Kosten')`).Bool() {
		t.Fatal("batch did not warn before any billed request")
	}
	page.MustEval(`()=>{accept=true;document.querySelector('[data-gallery-analyze-all]').click()}`)
	waitForJSBool(t, page, `()=>postCount()===21 && !document.querySelector('[data-gallery-analyze-all]').disabled`)
	page.MustEval(`()=>{document.querySelector('[data-gallery-info-toggle]').click();const card=document.querySelector('[data-gallery-item]');card.dispatchEvent(new MouseEvent('click',{bubbles:true,ctrlKey:true}))}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-gallery-description]')`)
	page.MustEval(`()=>{setInput('[data-gallery-description]','My edited mountain');setInput('[data-gallery-tags]','cover, mountains');document.querySelector('[data-gallery-metadata-save]').click()}`)
	waitForJSBool(t, page, `()=>Object.values(entries).some(m=>m.description==='My edited mountain')`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-gallery-analyze]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-gallery-analyze]').click()`)
	waitForJSBool(t, page, `()=>postCount()===22 && !document.querySelector('[data-gallery-analyze]').disabled`)
	page.MustEval(`()=>{const card=document.querySelector('[data-gallery-item]');card.click()}`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.vd-lightbox')`)
	page.MustEval(`()=>document.querySelector('[data-lb-info]').click()`)
	waitForJSBool(t, page, `()=>!!document.querySelector('.vd-lightbox-info-list')`)
	if !page.MustEval(`()=>document.querySelector('.vd-lightbox-info-list').textContent.includes('AI mountain description')`).Bool() {
		t.Fatal("lightbox missing image metadata")
	}
	page.MustEval(`()=>GalleryLightbox.close()`)
	waitForJSBool(t, page, `()=>!document.querySelector('.vd-lightbox')`)
	page.MustEval(`()=>document.querySelector('[data-gallery-categories]').click()`)
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-category-name]')`)
	page.MustEval(`()=>{setInput('[data-category-name]','Berge');setInput('[data-category-keywords]','mountain, mountains');document.querySelector('[data-category-save]').click()}`)
	waitForJSBool(t, page, `()=>!document.querySelector('[data-category-name]') && categories.length===2`)
	for _, theme := range []string{"standard", "fruity", "fruity-dark"} {
		for _, width := range []int{1100, 600, 360} {
			page.MustEval(`(theme,width)=>{document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'standard';document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';document.getElementById('host').style.width=width+'px'}`, theme, width)
			waitForJSBool(t, page, fmt.Sprintf(`()=>Math.abs(document.querySelector('[data-gallery-root]').getBoundingClientRect().width-%d)<2`, width))
			waitForJSBool(t, page, fmt.Sprintf(`()=>document.querySelector('[data-gallery-root]').classList.contains('is-tiny')===%t`, width < 460))
			page.MustEval(`async()=>{await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame)}`)
			if !page.MustEval(`()=>{const root=document.querySelector('[data-gallery-root]');return root.scrollWidth<=root.clientWidth+1}`).Bool() {
				t.Fatalf("overflow %s/%d: %s", theme, width, page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.vd-gallery *')].filter(e=>e.getBoundingClientRect().right>361 && e.getBoundingClientRect().width>0).map(e=>[e.className,e.getBoundingClientRect().left,e.getBoundingClientRect().width,e.scrollWidth]).slice(0,25))`).Str())
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("gallery-%s-%d.png", theme, width)))
			}
		}
	}
	page.MustEval(`()=>{hold=true;document.querySelector('[data-gallery-analyze-all]').click()}`)
	waitForJSBool(t, page, `()=>postCount()===23`)
	page.MustEval(`()=>document.querySelector('[data-gallery-analysis-stop]').click()`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-gallery-metadata-status]').textContent.includes('Gestoppt')`)
	if page.MustEval(`()=>postCount()`).Int() != 23 {
		t.Fatal("cancel queued more analyses")
	}
	page.MustEval(`()=>{hold=false;accept=false;window.allFiles=files;files=files.slice(0,20);mount()}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-gallery-item]').length===20 && !document.querySelector('[data-gallery-analyze-all]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-gallery-analyze-all]').click()`)
	waitForJSBool(t, page, `()=>confirmations.at(-1).message.includes('20')`)
	if page.MustEval(`()=>confirmations.at(-1).message.includes('erhöhte Kosten') || postCount()!==23`).Bool() {
		t.Fatal("20-image warning threshold or rejection failed")
	}
	page.MustEval(`()=>files=allFiles`)
	page.MustEval(`()=>{hold=false;mount(true)}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-gallery-item]').length===21`)
	if page.MustEval(`()=>!!document.querySelector('[data-gallery-analyze-all], [data-gallery-categories], [data-gallery-metadata-save]')`).Bool() {
		t.Fatal("readonly exposes write controls")
	}
}
