package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/ui"
	"github.com/go-rod/rod/lib/input"
)

// Controlled acknowledgements exercise actual CodeMirror and notebook state,
// without a provider, Docker, timers or a network mutation.
func TestKaggleNotebookDraftRacesBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	translations, err := ui.Content.ReadFile("lang/kaggle/de.json")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/editor-review", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-kaggle.css"><body data-theme="standard"><div class="kg-app" id="host"></div><script>window.I18N=%s;</script><script src="/js/desktop/apps/kaggle-notebook.js"></script><script>
const clone=value=>JSON.parse(JSON.stringify(value));
window.review={executions:0, saves:0, revision:'r0', listeners:{}, doc:{nbformat:4,nbformat_minor:5,metadata:{},cells:[{id:'first',cell_type:'code',metadata:{},source:"print('old')",execution_count:null,outputs:[]}]}};
const R=window.review;
R.session={id:'session',project_id:'project',file:'review.ipynb',status:'idle'};
window.EventSource=class {addEventListener(kind,fn){R.listeners[kind]=fn;} close(){}};
R.emit=value=>R.listeners.cell({data:JSON.stringify({session_id:'session',cell_id:'first',execution_id:'exec',revision:R.executionRevision,...value})});
R.releaseSave=()=>{R.doc=R.pending.snapshot;R.revision='r'+R.saves;R.pending.resolve({revision:R.revision});R.pending=null;};
R.api=async (path,options={})=>{
 if(options.method==='PUT'){R.saves++;return new Promise(resolve=>R.pending={resolve,snapshot:clone(options.body)});}
 if(path==='/runtime'){
  if(options.body.operation==='sessions')return [clone(R.session)];
  if(options.body.operation==='execute'){R.executions++;R.executionRevision=options.body.revision;return {execution_id:'exec'};}
 }
 return {content:clone(R.doc),revision:R.revision};
};
R.open=async id=>{R.editor=await KaggleNotebook(document.getElementById('host'),{api:R.api,t:key=>I18N['kaggle.'+key],project:'project',file:'review.ipynb',windowId:id,selectFiles:async()=>[],remote:async()=>{}});R.ready=true;};
R.open('first');
</script></html>`, translations)
	})
	mux.Handle("/", http.FileServer(http.FS(ui.Content.FS)))
	origin := httptest.NewServer(mux)
	defer origin.Close()
	for _, scenario := range []string{"output_during_save", "edit_during_run_save", "restored_draft_retired"} {
		t.Run(scenario, func(t *testing.T) {
			page := browser.MustPage().Timeout(30 * time.Second)
			defer page.Close()
			page.MustNavigate(origin.URL + "/editor-review").MustWaitLoad()
			page.MustWait(`()=>window.review?.ready`)
			page.MustEval(`()=>{localStorage.clear();}`)
			defer page.MustEval(`()=>{review.editor.dispose();localStorage.clear();}`)
			edit := func(code string) {
				page.MustElement(".cm-content").MustKeyActions().Press(input.ControlLeft).Type(input.KeyA).Release(input.ControlLeft).MustDo()
				page.MustElement(".cm-content").MustInput(code)
			}
			switch scenario {
			case "output_during_save":
				page.MustElement(`[data-action="run_cell"]`).MustClick()
				page.MustWait(`()=>review.executions===1`)
				page.MustEval(`()=>review.emit({kind:'output',output_type:'stream',content:{name:'stdout',text:'before\n'}})`)
				page.MustElement(`[data-action="save"]`).MustClick()
				page.MustWait(`()=>!!review.pending`)
				page.MustEval(`()=>{review.emit({kind:'output',output_type:'stream',content:{name:'stdout',text:'after\n'}});review.releaseSave();}`)
				page.MustWait(`()=>!review.pending`)
				if !page.MustEval(`()=>Object.keys(localStorage).some(key=>key.startsWith('aurago.kaggle.draft.')&&localStorage[key].includes('after'))`).Bool() {
					t.Fatal("save acknowledgement discarded output received after its snapshot")
				}
			case "edit_during_run_save":
				edit("print('first edit')")
				page.MustElement(`[data-action="run_cell"]`).MustClick()
				page.MustWait(`()=>!!review.pending`)
				edit("print('second edit')")
				page.MustEval(`()=>review.releaseSave()`)
				page.MustWait(`()=>!review.pending`)
				if page.MustEval(`()=>review.executions`).Int() != 0 {
					t.Fatal("run executed an older saved source after the cell changed")
				}
			case "restored_draft_retired":
				edit("print('restored')")
				page.MustEval(`async()=>{review.editor.dispose();review.ready=false;await review.open('second');}`)
				page.MustElement(`[data-action="save"]`).MustClick()
				page.MustWait(`()=>!!review.pending`)
				page.MustEval(`()=>review.releaseSave()`)
				page.MustWait(`()=>document.querySelector('.kg-state').textContent===I18N['kaggle.saved']`)
				page.MustEval(`async()=>{review.editor.dispose();review.ready=false;await review.open('third');}`)
				if !page.MustEval(`()=>document.querySelector('.kg-state').textContent===I18N['kaggle.kernel_ready']&&Object.keys(localStorage).every(key=>!key.startsWith('aurago.kaggle.draft.'))`).Bool() {
					t.Fatal("a successfully saved recovery draft reappeared with an obsolete revision")
				}
			}
			page.MustEval(`()=>{review.editor.dispose();localStorage.clear();}`)
		})
	}
}
