package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGameMakerDiagnosticFramesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html>
<iframe id="game" sandbox="allow-scripts" srcdoc="<script>addEventListener('message',e=>parent.postMessage(e.data,'*'))</script>"></iframe>
<script src="/js/desktop/apps/game-maker-studio-preview.js"></script><script>
window.reports=[];window.state={frame:document.getElementById('game'),project:{id:'p'},previewProjectID:'p',channelID:'channel',
job:{status:'building'},previewGrant:{token:'parent-only',validation_id:'build',expires_at:new Date(Date.now()+60000).toISOString()},previewReported:new Set(),previewDiagnostics:[],
api:{reportPreview:async(id,payload)=>{reports.push(payload)}},addDiagnostic:()=>{}};
addEventListener('message',e=>GameMakerStudioPreview.handleMessage(state,e));
window.send=(message,channel='channel')=>state.frame.contentWindow.postMessage({source:'aurago-game',type:'runtime_error',channel,message,
frames:[{file:'dist/game.js',line:10,column:3,secret:'dropped'},{file:'private.txt',line:1,column:1},{file:'dist/game.js',line:-1,column:1},
{file:'dist/game.js',line:12,column:5},{file:'dist/game.js',line:1e99,column:1},{file:'dist/game.js',line:20,column:4}]},'*');
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(15 * time.Second).MustWaitLoad()
	defer page.Close()
	page.MustEval(`()=>send('first')`)
	page.MustWait(`()=>reports.length===1`)
	if !page.MustEval(`()=>JSON.stringify(reports[0].frames)===JSON.stringify([{file:'dist/game.js',line:10,column:3},{file:'dist/game.js',line:12,column:5}])&&reports[0].token==='parent-only'`).Bool() {
		t.Fatal("invalid frames or payload leakage", page.MustEval(`()=>reports`))
	}
	page.MustEval(`()=>{send('wrong channel','wrong');window.postMessage({source:'aurago-game',channel:'channel',type:'runtime_error',message:'wrong source'},'*');state.previewGrant.expires_at=new Date(0).toISOString();send('expired')}`)
	// Two animation frames let queued cross-frame messages complete.
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if page.MustEval(`()=>reports.length`).Int() != 1 {
		t.Fatal("stale/foreign report accepted")
	}
}
