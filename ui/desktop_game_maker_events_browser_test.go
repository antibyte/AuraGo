package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestGameMakerEventsReconnectBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var connections atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, connections.Load()) })
	mux.HandleFunc("/api/game-maker/projects/bo/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if connections.Add(1) == 1 {
			fmt.Fprint(w, "retry: 50\nid: 1\nevent: job_status\ndata: {\"id\":1,\"type\":\"job_status\",\"payload\":{\"status\":\"building\",\"job\":{\"id\":\"job\",\"status\":\"building\"}}}\n\n")
			fmt.Fprint(w, "id: 2\nevent: job_status\ndata: {\"id\":2,\"type\":\"job_status\",\"payload\":{\"status\":\"cancelled\",\"error\":\"Game creation exceeded its time limit.\"}}\n\n")
			w.(http.Flusher).Flush()
			return
		}
		// A healthy reconnect with no new job events reproduces the stale badge.
		fmt.Fprint(w, ": heartbeat\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><div id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script>
<script src="/js/desktop/apps/game-maker-studio-api.js"></script>
<script src="/js/desktop/apps/game-maker-studio.js"></script>
<script>
const project={id:'bo',name:'bo',description:'Breakout',dimension:'2d',status:'draft',current_revision:0};
GameMakerStudioApp.render(document.getElementById('app'),'fixture',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:key=>key,
 api:async path=>path.endsWith('/capabilities')?{enabled:true,allow_create:true,skills_ready:true,phaser_version:'4.2.1'}:path.endsWith('/projects')?{projects:[project]}:{project,messages:[]}
});
</script>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	page := browser.MustPage(server.URL + "/fixture").MustWaitLoad()
	defer page.Close()
	if err := page.Timeout(10 * time.Second).Wait(rod.Eval(`async()=>Number(await(await fetch('/connections')).text())>=2&&GameMakerStudioApp.instances.get('fixture')?.reconnecting===false&&!!document.querySelector('[data-gm-retry]')`).ByPromise()); err != nil {
		t.Fatalf("%v; connections=%d; state=%s; DOM=%s", err, connections.Load(), page.MustEval(`()=>JSON.stringify({reconnecting:GameMakerStudioApp.instances.get('fixture').reconnecting,ready:GameMakerStudioApp.instances.get('fixture').eventSource.readyState})`).Str(), page.MustElement("body").MustText())
	}
	if got := page.MustElement("[data-gm-status]").MustText(); got != "game_maker.status_cancelled" {
		t.Fatal(got)
	}
	if got := page.MustElement(".gm-result-card p").MustText(); got != "Game creation exceeded its time limit." {
		t.Fatal(got)
	}
	if page.MustHas("[data-gm-phases] .is-current") {
		t.Fatal("cancelled job still has an active phase")
	}
	page.MustEval(`()=>GameMakerStudioApp.dispose('fixture')`)
}
