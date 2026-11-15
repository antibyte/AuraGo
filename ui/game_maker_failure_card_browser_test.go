package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Draft recovery accepts the selected model without retired admission gates.
func TestGameMakerFailureCardBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><div id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script>
<script src="/js/desktop/apps/game-maker-studio-api.js"></script>
<script src="/js/desktop/apps/game-maker-studio.js"></script>
<script>
const project={id:'bo',name:'bo',description:'Breakout',dimension:'2d',status:'failed',
 provider_id:'main',model:'small-model',current_revision:1};
const latest={id:'job',project_id:'bo',kind:'create',status:'failed',phase:'failed',
 error:'game validation failed: goal untouched',error_code:'validation',
 checks_passed:3,checks_failed:1,repair_passes:2,open_checks:[
  {id:'startup',status:'failed',expected:'visible canvas',observed:'blank canvas',
   next_action:'Fix the reported build or runtime error at its source location before any gameplay change.'},
  {id:'goal',status:'unavailable',repairable:true,expected:'goal reached',observed:'no target'}
 ]};
window.jobRequests=[];
GameMakerStudioApp.render(document.getElementById('app'),'fixture',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),t:key=>key,
 confirmDialog:async()=>true,
 api:async (path,opts)=>{
  if(path.endsWith('/jobs')) {window.jobRequests.push(JSON.parse(opts.body));return {id:'resumed',status:'queued',phase:'queued'};}
  if(path.includes('/model-limits')) return {context_window:8192,max_output_tokens:4096,known:true,
   minimum_context_window:16384,below_minimum:true,small_model:true,
   small_context_window:65536,small_output_tokens:16384};
  if(path.endsWith('/capabilities')) return {enabled:true,allow_create:true,allow_edit:true,allow_delete:true,
   skills_ready:true,phaser_version:'4.2.1',three_version:'0.186.1',
   providers:[{id:'main',name:'Main',model:'small-model',usage:'llm'},{id:'big',name:'Big',model:'big-model',usage:'llm'}],
   default_provider_id:'main',default_model:'small-model'};
  if(path.endsWith('/projects')) return {projects:[project]};
  return {project,messages:[],latest_job:latest};
 }
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

	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>!!document.querySelector('[data-gm-retry-model]')`)); err != nil {
		t.Fatal(err)
	}
	if page.MustHas("[data-gm-extend-repairs]") || page.MustHas("[data-gm-open-checks]") {
		t.Fatal("retired gates still displayed")
	}
	if !strings.Contains(page.MustElement("[data-gm-outcome]").MustText(), "game_maker.draft_retained") {
		t.Fatal("draft recovery missing")
	}
	page.MustElement("[data-gm-retry-model]").MustClick()
	page.Timeout(5 * time.Second).MustElement("[data-gm-retry-model-form]")
	if page.MustHas("[data-gm-model-warning]") || page.MustHas("[data-gm-compact-mode]") {
		t.Fatal("small model still gated")
	}
	page.MustElement("[data-gm-retry-model-form] button[type=submit]").MustClick()
	page.Timeout(5 * time.Second).MustWait(`()=>jobRequests.length===1`)
	request := page.MustEval(`()=>JSON.stringify(jobRequests[0])`).Str()
	if !strings.Contains(request, `"resume":true`) {
		t.Fatal("retry lost draft", request)
	}
	for _, removed := range []string{"max_repair_passes", "allow_small_model", "small_model"} {
		if strings.Contains(request, removed) {
			t.Fatal("retry carries retired gate", request)
		}
	}
	page.MustEval(`()=>GameMakerStudioApp.dispose('fixture')`)
}
