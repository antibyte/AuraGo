package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGameMakerAgentCaptureBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/game-maker/projects/p/agent-captures/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="blue"/></svg>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL).Timeout(20 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustEval(`()=>{
 document.body.innerHTML='<main><div data-gm-visual hidden><span data-gm-visual-status></span></div></main>';
 window.fixture={container:document.querySelector('main'),project:{id:'p'},context:{t:()=> 'Agent-Ansicht'}};
 }`)
	page.MustEval(`source=>(0,eval)(source)`, string(mustReadUIFile(t, "js/desktop/apps/game-maker-studio-preview.js")))
	page.MustEval(`()=>{for(let i=0;i<12;i++)GameMakerStudioPreview.showAgentCapture(fixture,{id:String(i),build_id:'build',url:'/api/game-maker/projects/p/agent-captures/job/'+i});}`)
	page.MustWait(`()=>[...document.images].every(i=>i.complete&&i.naturalWidth>0)`)
	if !page.MustEval(`()=>document.images.length===3&&document.images[0].dataset.captureId==='9'&&document.images[2].alt==='Agent-Ansicht'`).Bool() {
		t.Fatal("latest agent captures missing")
	}
	page.MustEval(`()=>{GameMakerStudioPreview.cancelVisual(fixture);GameMakerStudioPreview.showAgentCapture(fixture,{id:'foreign',build_id:'other',url:'/api/game-maker/projects/other/agent-captures/job/x'});}`)
	if !page.MustEval(`()=>document.images.length===3&&document.images[2].dataset.captureId==='11'`).Bool() {
		t.Fatal("reload lost captures or foreign project image was admitted")
	}
	page.MustEval(`()=>{fixture.disposed=true;GameMakerStudioPreview.cancelVisual(fixture)}`)
	if page.MustEval(`()=>document.images.length`).Int() != 0 {
		t.Fatal("disposed view retained captures")
	}
}
