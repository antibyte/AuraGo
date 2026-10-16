package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLearningLocaleCoverage(t *testing.T) {
	var baseline map[string]string
	for _, lang := range []string{"en", "cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := Content.ReadFile("lang/learnings/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var words map[string]string
		if err := json.Unmarshal(data, &words); err != nil {
			t.Fatal(err)
		}
		if baseline == nil {
			baseline = words
		}
		if len(words) != len(baseline) {
			t.Fatal("locale key count", lang)
		}
		for key := range baseline {
			if words[key] == "" {
				t.Fatal("missing translation", lang, key)
			}
		}
	}
}

func TestDesktopLearningsBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	words, err := Content.ReadFile("lang/learnings/de.json")
	if err != nil {
		t.Fatal(err)
	}
	var dictionary map[string]string
	if err := json.Unmarshal(words, &dictionary); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async(words)=>{
  await csWait(()=>csRoot()&&csRoot().querySelector('[data-action="knowledge"]'),20000);
		window.knowledgeWords=words;window.knowledgeFixture={writable:true,enabled:true,writes:[],failWrite:false,failRead:false};
  const original=window.fetch.bind(window);
  window.fetch=async(input,init={})=>{
   const url=new URL(typeof input==='string'?input:input.url,location.href);
   const k=window.knowledgeFixture;
   const json=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
   if(url.pathname==='/api/i18n'&&url.searchParams.get('sections')==='learnings')return json({data:words});
   if(url.pathname.endsWith('/learnings')){
    const item={id:'learning_fixture',statement:'TypeScript beibehalten <img src=x onerror="window.injected=true">',rationale:'Bestehende Schnittstellen erhalten',status:'active',revision:1,scope:{project:'code-studio:fixture',resource:'main.go'},sources:[{reference:'user:current',session:'fixture-session',quote:'Behalte TypeScript bei.'}]};
    if(init.method==='POST'){k.writes.push(JSON.parse(init.body));return json({learning:item},k.failWrite?409:200);}
    if(k.failRead)return json({error:'fixture'},503);
    if(url.searchParams.has('id'))return json({learning:item,history:[{learning:item,reason:'Explizite Entscheidung',actor:'user'}]});
    return json({learnings:[item],writable:k.writable,enabled:k.enabled,metrics:{retrievals:4,discarded:2,context_tokens:90}});
   }
   if(url.pathname.endsWith('/code-graph'))return json({status:{complete:false,indexed:1,files:2,unresolved:1,updated_at:'2026-10-16T10:00:00Z',skipped:{'bad.ts':'syntax_error'}},symbols:[{id:'main.go:0:main',name:'main',file:'main.go',line:5,snippet:'func main() {'}],edges:[],truncated:false});
   return original(input,init);
  };
 }`, dictionary)
	check := func(label, js string) {
		t.Helper()
		if !page.MustEval(js).Bool() {
			t.Fatal(label, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	for _, theme := range []string{"standard", "fruity"} {
		if _, err := page.Eval(`async(theme)=>{codeStudioTest.state.bootstrap.settings['appearance.theme']=theme;codeStudioTest.applyDesktopSettings();csRoot().querySelector('[data-action="knowledge"]').click();await csWait(()=>document.querySelector('.learning-item'));await csWait(()=>document.querySelector('.learning-graph .learning-item'));}`, theme); err != nil {
			t.Fatalf("knowledge opening: %v; %s", err, page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,library:typeof AuraGoLearnings,dialog:document.querySelector('.learning-dialog')?.outerHTML,buttons:[...csRoot().querySelectorAll('button')].map(b=>b.dataset.action)})`).Str())
		}
		check("source text escaped", `()=>!window.injected&&!document.querySelector('.learning-view img')&&document.querySelector('.learning-item h4').textContent.includes('<img')`)
		check("dialog named and focused", `()=>{const d=document.querySelector('.learning-dialog');return d.open&&!!d.getAttribute('aria-label')&&!!d.querySelector('.learning-close').getAttribute('aria-label')&&d.contains(document.activeElement);}`)
		check("partial graph coverage visible", `()=>document.querySelector('.learning-graph').textContent.includes('syntax_error')&&document.querySelector('.learning-graph [role="status"]').textContent.includes('1/2')`)
		page.MustEval(`async()=>{const b=[...document.querySelectorAll('.learning-list button')].find(b=>b.textContent===knowledgeWords['learnings.history']);b.click();await csWait(()=>document.querySelector('.learning-details summary'));}`)
		check("history revision visible", `()=>document.querySelector('.learning-details').textContent.includes('#1')`)
		page.MustSetViewport(500, 800, 1, false)
		check("narrow dialog fits", `()=>document.querySelector('.learning-dialog').getBoundingClientRect().width<=500`)
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "learnings-"+theme+".png"), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
		page.MustSetViewport(1440, 920, 1, false)
		page.MustEval(`async()=>{document.querySelector('.learning-graph .learning-item button').click();await csWait(()=>!document.querySelector('.learning-dialog'));await csWait(()=>csState().openTabs.some(tab=>tab.path==='/workspace/main.go'));}`)
	}
	page.MustEval(`async()=>{csRoot().querySelector('[data-action="knowledge"]').click();await csWait(()=>document.querySelector('.learning-item'));[...document.querySelectorAll('.learning-list button')].find(b=>b.textContent===knowledgeWords['learnings.archive']).click();await csWait(()=>document.querySelector('.learning-review'));knowledgeFixture.failWrite=true;const f=document.querySelector('.learning-review');f.querySelector('textarea[placeholder]').value='Prüfung abgeschlossen';f.requestSubmit();await csWait(()=>knowledgeFixture.writes.length===1&&!f.querySelector('[type="submit"]').disabled);}`)
	check("uncertain writes not retried", `()=>knowledgeFixture.writes.length===1&&document.querySelector('.learning-status').textContent.length>0`)
	page.MustEval(`async()=>{knowledgeFixture.writable=false;window.dispatchEvent(new Event('aurago:desktop-policy'));await csWait(()=>!document.querySelector('.learning-review'));}`)
	check("revoked write controls removed", `()=>![...document.querySelectorAll('.learning-list button')].some(b=>b.textContent===knowledgeWords['learnings.archive'])`)
	page.MustEval(`async()=>{knowledgeFixture.failRead=true;document.querySelector('.learning-controls').requestSubmit();await csWait(()=>!document.querySelector('.learning-controls [type="submit"]').disabled);}`)
	check("read failure visible", `()=>document.querySelector('.learning-status').textContent.length>0`)
	check("no browser exceptions", `()=>fixtureErrors.length===0`)
}
