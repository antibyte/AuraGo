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

// TestGameMakerFailureCardBrowser proves the failure card renders the open
// checks with their next action, offers the repair extension and the
// retry-with-another-model dialog, and sends the model opt-in explicitly.
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

	// The card names the concrete open check. The model-facing next step
	// stays inside the folded technical details, so read its text content.
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>!!document.querySelector('[data-gm-open-checks] [data-gm-open-check="startup"]')`).ByPromise()); err != nil {
		t.Fatalf("%v; DOM=%s", err, page.MustElement("body").MustText())
	}
	if got := page.MustElement(`[data-gm-open-check="startup"] strong`).MustText(); got != "startup" {
		t.Fatalf("open check id = %q", got)
	}
	if got := page.MustElement(`[data-gm-open-check="goal"] span`).MustText(); got != "game_maker.check_unavailable" {
		t.Fatalf("unavailable check label = %q", got)
	}
	if got := page.MustElement(`[data-gm-open-check="startup"] p`).MustText(); got != "game_maker.check_help_generic" {
		t.Fatalf("check help = %q", got)
	}
	details := page.MustEval(`()=>document.querySelector('[data-gm-outcome] details').textContent`).Str()
	for _, value := range []string{"visible canvas", "blank canvas", "goal reached", "no target"} {
		if !strings.Contains(details, value) {
			t.Fatalf("technical details lost %q: %s", value, details)
		}
	}
	if page.MustEval(`()=>document.querySelector('[data-gm-outcome] details').open`).Bool() {
		t.Fatal("technical details should start collapsed")
	}
	page.MustElement(`[data-gm-apply-check="startup"]`).MustClick()
	if got := page.MustEval(`()=>document.querySelector('[data-gm-change-form] textarea').value`).Str(); got != "game_maker.check_prompt_generic" {
		t.Fatalf("applied prompt = %q", got)
	}
	if requests := page.MustEval(`()=>window.jobRequests.length`).Int(); requests != 0 {
		t.Fatalf("apply check started %d jobs", requests)
	}
	next := page.MustEval(`()=>document.querySelector('[data-gm-model-instruction]').textContent`).Str()
	if next != "game_maker.model_instruction: Fix the reported build or runtime error at its source location before any gameplay change." {
		t.Fatalf("next action = %q", next)
	}
	if !page.MustHas("[data-gm-retry]") {
		t.Fatal("failure card lost the plain retry button")
	}
	if got := page.MustElement("[data-gm-extend-repairs]").MustAttribute("data-gm-extend-repairs"); got == nil || *got != "5" {
		t.Fatalf("repair extension cap = %v, want used+3", got)
	}

	// The retry dialog loads the passive limits, warns below the floor and
	// sends the explicit small-model opt-in. Open it before extending repairs:
	// an extension starts a job and removes this card.
	page.MustElement("[data-gm-retry-model]").MustClick()
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>{
		const warning=document.querySelector('[data-gm-model-warning]');
		const line=document.querySelector('[data-gm-model-limits]');
		const compact=document.querySelector('[data-gm-compact-mode]');
		return !!document.querySelector('[data-gm-retry-model-form]') && !!warning && !warning.hidden
			&& warning.textContent==='game_maker.model_limits_warning'
			&& !!line && line.textContent==='game_maker.model_limits_line'
			&& !!compact && !compact.hidden
			&& compact.textContent==='game_maker.compact_mode_title — game_maker.compact_mode_body';
	}`).ByPromise()); err != nil {
		t.Fatalf("%v; DOM=%s", err, page.MustElement("body").MustText())
	}
	if got := page.MustElement("[data-gm-model-state]").MustAttribute("data-below-minimum"); got == nil || *got != "true" {
		t.Fatalf("below-minimum marker = %v", got)
	}
	page.MustElement("[data-gm-retry-model-form] button[type=submit]").MustClick()
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>window.jobRequests.length>=1`).ByPromise()); err != nil {
		t.Fatalf("%v; requests=%s", err, page.MustEval(`()=>JSON.stringify(window.jobRequests)`).Str())
	}
	request := page.MustEval(`()=>JSON.stringify(window.jobRequests[0])`).Str()
	for _, marker := range []string{`"resume":true`, `"allow_small_model":true`, `"provider_id":"main"`, `"model":"small-model"`} {
		if !strings.Contains(request, marker) {
			t.Fatalf("retry request missing %s: %s", marker, request)
		}
	}
	if strings.Contains(request, `"max_repair_passes"`) {
		t.Fatalf("retry with another model must not reset the repair budget: %s", request)
	}

	// Extending repairs resumes the retained draft with the raised budget.
	// Reopening the project without the active job brings the failure card back.
	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`()=>{GameMakerStudioApp.instances.get('fixture').activeJob=null}`)
	page.MustElement(`[data-project-id="bo"]`).MustClick()
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(`()=>!!document.querySelector('[data-gm-extend-repairs]')`).ByPromise()); err != nil {
		t.Fatalf("%v; DOM=%s", err, page.MustElement("body").MustText())
	}
	page.MustElement("[data-gm-extend-repairs]").MustClick()
	if err := page.Timeout(10 * time.Second).Wait(rod.Eval(`()=>window.jobRequests.length>=2`).ByPromise()); err != nil {
		t.Fatalf("%v; requests=%s", err, page.MustEval(`()=>JSON.stringify(window.jobRequests)`).Str())
	}
	extension := page.MustEval(`()=>JSON.stringify(window.jobRequests[1])`).Str()
	for _, marker := range []string{`"resume":true`, `"max_repair_passes":5`, `"provider_id":"main"`, `"model":"small-model"`} {
		if !strings.Contains(extension, marker) {
			t.Fatalf("repair extension request missing %s: %s", marker, extension)
		}
	}
}
