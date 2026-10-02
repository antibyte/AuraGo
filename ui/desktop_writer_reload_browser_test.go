package ui

import (
	"aurago/internal/office"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// An open request for a document Writer already shows (Files, the agent's
// open_in_app) must show the current file, not the copy loaded earlier.
func TestDesktopWriterReloadsChangedFileOnReopen(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	const docPath = "Documents/report.docx"
	encode := func(text string) []byte {
		data, err := office.EncodeDOCX(office.Document{Text: text})
		if err != nil {
			t.Fatalf("encode docx: %v", err)
		}
		return data
	}
	var mu sync.Mutex
	file, version := encode("First version of the report."), 1
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/office/document", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Query().Get("path") != docPath {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPut {
			if r.Header.Get("If-Match") != fmt.Sprintf("%q", fmt.Sprint(version)) {
				http.Error(w, `{"error":"Document changed; reload or save a copy"}`, http.StatusPreconditionFailed)
				return
			}
			file, _ = io.ReadAll(r.Body)
			version++
			w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprint(version)))
			fmt.Fprint(w, "{}")
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprint(version)))
		w.Write(file)
	})
	// Stands in for the agent writing the file while Writer has it open.
	mux.HandleFunc("/test/replace", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		file = encode(r.URL.Query().Get("text"))
		version++
	})
	mux.HandleFunc("/writer-reload-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<link rel="stylesheet" href="/js/vendor/writer/engine.css"><link rel="stylesheet" href="/css/desktop-app-writer.css">
<style>html,body{margin:0;height:100%}#host{height:100%}</style></head><body class="desktop-body"><div id="host"></div>
<script src="/js/vendor/purify.min.js"></script><script src="/js/vendor/marked.min.js"></script><script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-panels.js"></script><script src="/js/desktop/apps/writer.js"></script>
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();WriterApp.render(document.getElementById('host'),'test',{path:'`+docPath+`',t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>value,confirmDialog:async()=>false,setWindowBeforeClose:()=>{},setWindowMenus:()=>{}});})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/writer-reload-fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustWait(`()=>WriterApp.instances.get('test')?.editor && !document.querySelector('[data-loading]').offsetHeight`)

	result := page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),text=()=>app.editor.surface.session.bodyText();
        const settle=()=>new Promise(r=>setTimeout(r,150));
        const loaded=async()=>{for(let i=0;i<200;i++){if(app.editor && !document.querySelector('[data-loading]').offsetHeight)return;await settle();}throw Error('reload did not finish');};
        if(!text().includes('First version'))throw Error('initial text missing: '+text());
        if(typeof app.reloadIfChanged!=='function')throw Error('Writer has no reloadIfChanged');

        const unchanged=app.editor;
        await app.reloadIfChanged();
        if(app.editor!==unchanged)throw Error('an unchanged file was reloaded');

        await fetch('/test/replace?text='+encodeURIComponent('Second version written by the agent.'));
        await app.reloadIfChanged();await loaded();
        if(!text().includes('Second version'))throw Error('changed file was not reloaded: '+text());
        if(app.session.dirty)throw Error('a reloaded document must start clean');

        const edit=app.editor.exec({type:'paste',text:' My unsaved edit.'});if(!edit.ok)throw Error(JSON.stringify(edit));
        await fetch('/test/replace?text='+encodeURIComponent('Third version written elsewhere.'));
        const kept=app.editor;
        await app.reloadIfChanged();
        if(app.editor!==kept || !text().includes('My unsaved edit'))throw Error('unsaved edits were replaced: '+text());
        if(document.querySelector('[data-notice]').hidden)throw Error('no conflict notice for a file changed under unsaved edits');
        return document.querySelector('[data-notice-text]').textContent;
    }`).Str()
	t.Log("conflict notice:", result)
}
