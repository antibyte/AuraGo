package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestTresorTranslations(t *testing.T) {
	var keys []string
	for _, locale := range []string{"en", "de", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			t.Fatal(err)
		}
		if locale == "en" {
			for key := range labels {
				if strings.HasPrefix(key, "tresor.") {
					keys = append(keys, key)
				}
			}
		}
		for _, key := range keys {
			if labels[key] == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		if labels["desktop.app_tresor"] == "" {
			t.Errorf("%s missing app name", locale)
		}
	}
}

func TestDesktopTresorBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var mu sync.Mutex
	var header map[string]any
	type row struct {
		ID       string `json:"id"`
		Meta     string `json:"meta"`
		Body     string `json:"body"`
		Revision int    `json:"revision"`
	}
	items := map[string]row{}
	var wireBodies []string
	var desktopReads int
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/tresor", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		switch r.Method {
		case "GET":
			json.NewEncoder(w).Encode(map[string]any{"initialized": header != nil, "header": header})
		case "POST":
			var h map[string]any
			json.NewDecoder(r.Body).Decode(&h)
			header = h
			header["revision"] = 1
			w.WriteHeader(204)
		case "PUT":
			var h map[string]any
			json.NewDecoder(r.Body).Decode(&h)
			header = h
			header["revision"] = 2
			w.WriteHeader(204)
		}
	})
	mux.HandleFunc("/api/desktop/tresor/items", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			list := []row{}
			for _, item := range items {
				item.Body = ""
				list = append(list, item)
			}
			json.NewEncoder(w).Encode(list)
			return
		}
		var item row
		json.NewDecoder(r.Body).Decode(&item)
		wireBodies = append(wireBodies, item.Meta+item.Body)
		item.Revision = 1
		items[item.ID] = item
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/desktop/tresor/items/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		id := strings.TrimPrefix(r.URL.Path, "/api/desktop/tresor/items/")
		item, ok := items[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(item)
			return
		}
		if r.Header.Get("If-Match") != fmt.Sprintf("\"%d\"", item.Revision) {
			w.WriteHeader(412)
			return
		}
		if r.Method == "DELETE" {
			delete(items, id)
			w.WriteHeader(204)
			return
		}
		var next row
		json.NewDecoder(r.Body).Decode(&next)
		wireBodies = append(wireBodies, next.Meta+next.Body)
		next.Revision = item.Revision + 1
		items[id] = next
		w.WriteHeader(204)
	})
	mux.HandleFunc("/api/desktop/download", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		desktopReads++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("desktop source bytes"))
	})
	mux.HandleFunc("/tresor-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-tresor.css"><style>html,body,#host{margin:0;width:100%;height:100%;}body{background:#0d151c}</style></head><body><div id="host"></div><script>window.errors=[];addEventListener('error',e=>errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));const nativeTimeout=window.setTimeout;window.setTimeout=(fn,ms,...args)=>{if(ms===300000){window.expireTresor=fn;return 1}return nativeTimeout(fn,ms,...args)};</script><script src="/js/desktop/apps/tresor.js"></script><script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/de.json')).json();TresorApp.render(document.getElementById('host'),'test',{t:key=>labels[key]||key,openFileDialog:async()=>({path:'Downloads/from-desktop.bin'})});})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/tresor-fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`async()=>await window.ready`)
	page.MustElement("[data-form=setup]")
	artifactDir := filepath.Join("..", "reports", "tresor")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot(filepath.Join(artifactDir, "setup.png"))
	result := page.MustEval(`async()=>{
	 const wait=async selector=>{for(let i=0;i<150;i++){if(document.querySelector(selector))return;await new Promise(r=>setTimeout(r,100))}throw Error('missing '+selector+'; status='+document.querySelector('[data-status]')?.textContent+'; errors='+JSON.stringify(errors))};
	 const until=async predicate=>{for(let i=0;i<150;i++){if(predicate())return;await new Promise(r=>setTimeout(r,100))}throw Error('condition timed out: '+document.querySelector('[data-status]')?.textContent)};
	 const submit=selector=>document.querySelector(selector).dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
	 await wait('[data-form=setup]');
	 document.querySelector('[name=password]').value='a strong vault password 123';
	 document.querySelector('[name=repeat]').value='a strong vault password 123';submit('[data-form=setup]');
	 await wait('[data-form=confirm]');
	 document.querySelector('[name=recovery]').value=document.querySelector('.tresor-recovery output').textContent;
	 submit('[data-form=confirm]');await wait('.tresor-workspace');
	 document.querySelector('[data-action=new-note]').click();await wait('[data-note]');
	 const title=document.querySelector('[data-title]');title.value='Secret Plan';title.dispatchEvent(new Event('input',{bubbles:true}));
	 const note=document.querySelector('[data-note]');note.value='Private content';note.dispatchEvent(new Event('input',{bubbles:true}));
	 const draftKey='aurago:tresor:draft:'+document.querySelector('.tresor-item.is-selected').dataset.id;
	 await until(()=>!!localStorage.getItem(draftKey));
	 const draft=localStorage.getItem(draftKey);
	 if(draft.includes('Private content')||draft.includes('Secret Plan')||!JSON.parse(draft).cipher)throw Error('draft was not encrypted');
	 await until(()=>document.querySelector('[data-status]')?.textContent==='Gespeichert'&&!document.querySelector('[data-action=desktop]').disabled);
	 if(localStorage.getItem(draftKey))throw Error('saved draft was not cleared');
	 const tooLarge=new File([new Uint8Array(50*1024*1024+1)],'too-large.bin');
	 const largeInput=document.querySelector('[data-file]'),largeTransfer=new DataTransfer();largeTransfer.items.add(tooLarge);largeInput.files=largeTransfer.files;largeInput.dispatchEvent(new Event('change',{bubbles:true}));
	 await until(()=>document.querySelector('[data-status]')?.textContent.includes('50 MiB')&&!document.querySelector('[data-action=desktop]').disabled);
	 const file=new File([new Uint8Array([1,2,3])],'device.bin',{type:'application/octet-stream'});
	 const input=document.querySelector('[data-file]'),transfer=new DataTransfer();transfer.items.add(file);input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
	 await until(()=>document.querySelectorAll('.tresor-item').length>=2&&!document.querySelector('[data-action=desktop]').disabled);
	 document.querySelector('[data-action=desktop]').click();
	 await until(()=>document.querySelectorAll('.tresor-item').length>=3&&!document.querySelector('[data-action=desktop]').disabled);
	 const titles=[...document.querySelectorAll('.tresor-item strong')].map(x=>x.textContent);
	 document.querySelector('[data-action=export]').click();await wait('.tresor-modal');
	 if(!document.querySelector('.tresor-modal').textContent.includes('außerhalb'))throw Error('export warning missing');
	 document.querySelector('[data-action=cancel]').click();await until(()=>!document.querySelector('.tresor-modal'));
	 const nativeClick=HTMLAnchorElement.prototype.click;
	 HTMLAnchorElement.prototype.click=function(){if(this.download){window.exportedFile=fetch(this.href).then(r=>r.text());return}return nativeClick.call(this)};
	 document.querySelector('[data-action=export]').click();await wait('.tresor-modal');document.querySelector('[data-action=confirm-export]').click();
	 await until(()=>!document.querySelector('.tresor-modal')&&window.exportedFile);
	 if(await window.exportedFile!=='desktop source bytes')throw Error('export decrypt failed');
	 HTMLAnchorElement.prototype.click=nativeClick;
	 document.querySelector('[data-search]').value='Secret';document.querySelector('[data-search]').dispatchEvent(new Event('input',{bubbles:true}));
	 const searchCount=document.querySelectorAll('.tresor-item').length;
	 return {titles,searchCount,errors};
	}`)
	if result.Get("searchCount").Int() != 1 || len(result.Get("errors").Arr()) > 0 {
		t.Fatalf("browser flow: %s", result.JSON("", ""))
	}
	if !strings.Contains(result.JSON("", ""), "Secret Plan") || !strings.Contains(result.JSON("", ""), "device.bin") || !strings.Contains(result.JSON("", ""), "from-desktop.bin") {
		t.Fatalf("imports: %s", result.JSON("", ""))
	}
	page.MustEval(`()=>{const input=document.querySelector('[data-search]');input.value='';input.dispatchEvent(new Event('input',{bubbles:true}))}`)
	checkTresorFailureRecovery(t, page)
	page.MustScreenshot(filepath.Join(artifactDir, "open-wide.png"))
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1366, 430} {
			page.MustSetViewport(width, 768, 1, false)
			page.MustEval(`theme=>document.body.dataset.theme=theme`, theme)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
				t.Fatalf("overflow at %s %d", theme, width)
			}
			if theme == "fruity" && width == 430 {
				page.MustScreenshot(filepath.Join(artifactDir, "open-narrow.png"))
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`()=>document.querySelector('#host').style.width='430px'`)
	if !page.MustEval(`()=>{const app=document.querySelector('.tresor-app');return app.scrollWidth<=app.clientWidth+1&&getComputedStyle(document.querySelector('.tresor-main')).display==='block'}`).Bool() {
		t.Fatal("narrow desktop window did not adapt inside a wide viewport")
	}
	page.MustEval(`()=>document.querySelector('#host').style.width=''`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	if !page.MustEval(`()=>getComputedStyle(document.querySelector('.tresor-workspace')).animationName==='none'`).Bool() {
		t.Fatal("reduced motion animation still active")
	}
	locked := page.MustEval(`async()=>{const wait=async selector=>{for(let i=0;i<100;i++){if(document.querySelector(selector))return;await new Promise(r=>setTimeout(r,100))}throw Error('missing '+selector)};const submit=selector=>document.querySelector(selector).dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));document.querySelector('[data-action=lock]').click();await wait('[data-form=unlock]');document.querySelector('[name=password]').value='a strong vault password 123';submit('[data-form=unlock]');await wait('.tresor-workspace');[...document.querySelectorAll('.tresor-item')].find(x=>x.textContent.includes('Secret Plan')).click();await wait('[data-note]');if(document.querySelector('[data-note]').value!=='Private content')throw Error('note decrypt failed');window.expireTresor();await wait('[data-form=unlock]');return !!document.querySelector('[data-form=unlock]')}`)
	if !locked.Bool() {
		t.Fatal("idle timeout did not lock vault")
	}
	page.MustEval(`()=>{const field=document.querySelector('[name=password]');field.value='a strong vault password 123';field.focus()}`)
	if err := page.Keyboard.Press(input.Enter); err != nil {
		t.Fatal(err)
	}
	if err := page.Keyboard.Release(input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!!document.querySelector('.tresor-workspace')`)
	stale := page.MustEval(`async()=>{
	 const nativeFetch=window.fetch;let release;window.pendingRead=false;
	 window.fetch=async(...args)=>{if(String(args[0]).includes('/tresor/items/')){window.pendingRead=true;await new Promise(r=>release=r)}return nativeFetch(...args)};
	 const until=async predicate=>{for(let i=0;i<100;i++){if(predicate())return;await new Promise(r=>setTimeout(r,50))}throw Error('stale read condition timed out')};
	 await until(()=>document.querySelectorAll('.tresor-item').length===3&&!document.querySelector('[data-action=lock]').disabled);
	 [...document.querySelectorAll('.tresor-item')].find(x=>x.textContent.includes('Secret Plan')).click();await until(()=>window.pendingRead);
	 window.expireTresor();release();window.fetch=nativeFetch;
	 await until(()=>document.querySelector('[data-form=unlock]')&&!document.querySelector('[data-form=unlock] button').disabled);
	 if(document.querySelector('[data-note]')||document.querySelector('#host').textContent.includes('Private content'))throw Error('stale read restored plaintext');
	 document.querySelector('[name=password]').value='a strong vault password 123';document.querySelector('[data-form=unlock]').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
	 TresorApp.dispose('test');await new Promise(r=>setTimeout(r,500));return document.querySelector('#host').childElementCount===0;
	}`)
	if !stale.Bool() {
		t.Fatal("pending work restored disposed vault UI")
	}
	mu.Lock()
	defer mu.Unlock()
	if desktopReads != 1 || len(items) != 3 {
		t.Fatalf("import counts: reads=%d records=%d", desktopReads, len(items))
	}
	for _, body := range wireBodies {
		if strings.Contains(body, "Private content") || strings.Contains(body, "Secret Plan") || strings.Contains(body, "device.bin") {
			t.Fatal("plaintext sent to server")
		}
	}
}
