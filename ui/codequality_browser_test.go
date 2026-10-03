package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCodeQualityBrowserBoundaries(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("requires browser smoke flag")
	}
	browser := newSmokeBrowser(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><input id="port" data-path="server.port" type="number" value="8080"><div id="output"></div></body></html>`))
	}))
	defer server.Close()
	page := browser.MustPage(server.URL)
	defer page.MustClose()
	load := func(path string) {
		t.Helper()
		if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, path))); err != nil {
			t.Fatal(err)
		}
	}
	load("js/config/state.js")
	if !page.MustEval(`() => {
  const s=window.AuraConfigState; s.init({server:{port:8080}}); s.bind(document);
  const input=document.getElementById('port'); input.value='9090'; input.dispatchEvent(new Event('input',{bubbles:true}));
  const sent=s.snapshot().draft; input.value='8080'; input.dispatchEvent(new Event('input',{bubbles:true}));
  s.commitSent(sent,sent);
  return s.get('server.port')===8080 && s.get('server.port',{saved:true})===9090 && s.isDirty() && input.value==='8080';
 }`).Bool() {
		t.Fatal("save erased edits made in flight")
	}
	load("js/shared/chat-core.js")
	load("js/desktop/chat-renderer.js")
	if !page.MustEval(`() => {
  let fallbacks=0; const r=window.DesktopChatRenderer;
  const append=r.appendRichBubble; r.appendRichBubble=()=>{fallbacks++};
  try {r.appendImageMessage(document.body,{path:'https://foreign.example/a.png'});r.appendVideoMessage(document.body,{path:'https://foreign.example/a.mp4'});} finally {r.appendRichBubble=append;}
  return fallbacks===2;
 }`).Bool() {
		t.Fatal("Desktop media rejection failed to reach a safe fallback")
	}
	if !page.MustEval(`() => {
  const core=window.AuraChatCore;
  document.getElementById('output').innerHTML=core.sanitizeRenderedHTML('<img src="https://external.invalid/pixel" onerror="window.pwned=1"><img src="/files/safe.png"><script>window.pwned=1</script>');
  return !window.pwned && document.querySelectorAll('#output img').length===1 && !!document.querySelector('#output a') && !core.isSafeMediaSource('/auth/logout','img') && !core.isSafeMediaSource('//external.invalid/pixel','img') && core.isSafeMediaSource('/api/go2rtc/proxy/api/frame.jpeg?src=camera','img');
 }`).Bool() {
		t.Fatal("unsafe automatic media load permitted")
	}
	shared := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	shared = strings.Split(shared, "// Auto-initialize on DOM ready")[0]
	if err := page.AddScriptTag("", shared); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => {window.results=[];showModal('one','first',true).then(v=>results.push(['one',v]));showModal('two','second',true).then(v=>results.push(['two',v]));}`)
	page.MustElement("#shared-modal-confirm").MustClick()
	page.MustWait(`() => window.results.length===1 && document.getElementById('shared-modal-title').textContent==='two'`)
	page.MustElement("#shared-modal-cancel").MustClick()
	page.MustWait(`() => window.results.length===2`)
	if !page.MustEval(`() => results[0][0]==='one' && results[0][1]===true && results[1][0]==='two' && results[1][1]===false`).Bool() {
		t.Fatal("overlapping modal promises share a result")
	}
	outgoing := strings.Split(normalizeAssetText(mustReadUIFile(t, "js/chat/main/network-submit.js")), "/* ── Form submit")[0]
	page.MustEval(`() => {
  window.currentSession='a';window.historyGeneration=0;window.getActiveSessionId=()=>currentSession;
  window.closeComposerPanel=window.closeMoodFeedbackRow=()=>{};window.pendingAttachments=[];window.pendingSpeechLabTurnToken='';window.conversation=[];
  window.userInput={value:'question',style:{},focus(){}};window.sendBtn={};window.stopBtn={};window.agentStatusText={};window.agentStatusDiv={};
  window.appended=[];window.appendMessage=(role,content)=>appended.push({role,content});window.chatSetHidden=()=>{};
  window._httpResponseRendered=false;window._fetchConnectionLost=false;window.resetSSEDedupSets=()=>{};
  window.fetch=()=>new Promise(resolve=>{window.completeRequest=()=>resolve({ok:true,json:async()=>({choices:[{message:{role:'assistant',content:'answer A'}}]})})});
 }`)
	if err := page.AddScriptTag("", outgoing); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`() => {window.request=handleOutgoingMessage('question');currentSession='b';historyGeneration++;conversation=[];completeRequest();}`)
	page.MustEval(`async () => {await request;}`)
	if page.MustEval(`() => conversation.length!==0 || appended.some(m=>m.content==='answer A')`).Bool() {
		t.Fatal("late answer contaminated another session")
	}
	main := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	start := strings.Index(main, "function highlightSidebarLabel(")
	end := strings.Index(main[start:], "function applySidebarSearch(") + start
	if err := page.AddScriptTag("", main[start:end]); err != nil {
		t.Fatal(err)
	}
	if !page.MustEval(`() => {const node=document.createElement('span');node.innerHTML=highlightSidebarLabel('A & <b> "é"',['amp','<b>','é']);return node.textContent==='A & <b> "é"' && !node.querySelector('b');}`).Bool() {
		t.Fatal("highlight corrupted text or introduced markup")
	}
	page.MustEval(`() => {
  window.state={bootstrap:{},filesPath:'Documents'}; window.fileResult=null;window.writePending=false;
  window.api=async (_,opts)=>{if(opts && opts.method==='PUT'){writePending=true;return new Promise(resolve=>window.completeWrite=resolve)}return {entries:[]}};
 }`)
	load("js/desktop/core/file-dialog-runtime.js")
	page.MustEval(`() => {window.AuraDesktopFileDialogs.save({filename:'safe.txt',content:'synthetic',confirmOverwrite:false}).then(value=>window.fileResult=value);}`)
	page.MustEval(`() => document.querySelector('.vd-file-dialog').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}))`)
	page.MustWait(`() => window.writePending`)
	page.MustEval(`() => document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
	if page.MustEval(`() => fileResult!==null || !document.querySelector('.vd-file-dialog')`).Bool() {
		t.Fatal("Escape acknowledged cancellation while PUT was still pending")
	}
	page.MustEval(`() => completeWrite({ok:true})`)
	page.MustWait(`() => window.fileResult!==null`)
	if !page.MustEval(`() => fileResult.canceled===false && !document.querySelector('.vd-file-dialog')`).Bool() {
		t.Fatal("completed file save lost its result")
	}
}
