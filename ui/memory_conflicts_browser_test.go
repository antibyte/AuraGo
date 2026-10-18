package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestMemoryConflictDismissBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><body><div id="memory-curator-list"></div></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage(srv.URL + "/fixture").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			result := page.MustEval(`async(theme)=>{
 document.documentElement.dataset.theme=theme;
 const source=await(await fetch('/js/dashboard/dashboard-widgets.js')).text();
 const locale=await(await fetch('/lang/dashboard/de.json')).json();
 window.t=k=>locale[k]||k;window.esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 window.renderCollapsibleList=(items,fn)=>items.map(fn).join('');
 (0,eval)(source);
 renderMemoryHealth({memory_conflicts:[{id:7,left_value:'<German>',right_value:'dark theme',reason:'review'}]});
 const row=document.querySelector('.memory-conflict-pair').parentElement;
 let button=row.querySelector('button'), status=row.querySelector('[role=status]');
 if(!button||button.textContent!=='Beide Fakten sind gültig'||row.querySelector('german'))throw Error('missing control or unescaped fact');
 let release;const calls=[];
 window.fetch=async(url,options)=>{calls.push({url,body:JSON.parse(options.body)});await new Promise(r=>release=r);return {ok:false}};
 button.click();if(!button.disabled||status.textContent!==locale['dashboard.memory_conflict_saving'])throw Error('pending feedback missing');
 release();for(let i=0;i<20&&button.disabled;i++)await new Promise(r=>setTimeout(r,10));
 if(button.disabled||status.textContent!==locale['dashboard.memory_conflict_failed'])throw Error('failure cannot retry');
 window.fetch=async(url,options)=>{calls.push({url,body:JSON.parse(options.body)});return {ok:true}};
 button.click();for(let i=0;i<20&&row.querySelector('button');i++)await new Promise(r=>setTimeout(r,10));
 if(row.querySelector('button')||status.textContent!==locale['dashboard.memory_conflict_dismissed'])throw Error('success feedback missing');
 if(calls.some(x=>x.url!='/api/memory/conflicts/resolve'||x.body.action!=='dismiss'||x.body.conflict_id!==7||x.body.winning_doc_id))throw Error('incorrect dismissal request');
 if(!row.textContent.includes('<German>')||!row.textContent.includes('dark theme'))throw Error('facts not retained');
 return true;
 }`, theme)
			if !result.Bool() {
				t.Fatal("conflict controls failed")
			}
		})
	}
}
