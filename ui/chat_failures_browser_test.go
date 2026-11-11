package ui

import (
	"testing"
	"time"
)

func TestDesktopChatFailuresAndQuestionCancellationBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	p := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t)).Timeout(40 * time.Second)
	defer p.MustClose()
	p.MustSetDocumentContent(`<!doctype html><html><body><button id="trigger">Chat</button><div id="host"><div class="vd-chat"><div class="vd-chat-log"></div><input class="vd-chat-input"><button class="vd-chat-send"></button></div></div><div id="quick"></div></body></html>`)
	for _, file := range []string{"js/shared/shared-core.js", "js/shared/chat-stream-parser.js", "js/desktop/chat-renderer.js", "js/chat/chat-question.js"} {
		if err := p.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, file))); err != nil {
			t.Fatal(err)
		}
	}
	p.MustEval(`()=>{
		window.t=key=>key; window.PetRuntime={announceAgentResponse(){}};
		const r=window.DesktopChatRenderer;
		r.escapeHtml=text=>String(text);r.renderMarkdown=text=>String(text);
		r.appendRichBubble=(log,role,text)=>{const e=document.createElement('div');e.className='vd-chat-bubble '+role;e.textContent=text;log.appendChild(e);};
		r.appendAvatar=(log,role,bubble)=>log.appendChild(bubble);r.appendTimestamp=()=>null;
		r.processImages=()=>{};r.enhanceCodeBlocks=()=>{};
		window.mode='error';window.failRequest=false;window.calls=[];
		window.fetch=async(url,opts={})=>{
			if(!url.includes('/chat/stream')) {calls.push({url,body:opts.body});return new Response(JSON.stringify({status:failRequest?'not_found':url.includes('/stop')?'success':'ok',message:failRequest?'fixture refused':''}),{status:failRequest===true?409:200,headers:{'Content-Type':'application/json'}});}
			if(mode==='http')return new Response('fixture HTTP failure',{status:503});
			if(mode==='network')throw new TypeError('fixture disconnected');
			const events=[{event:'llm_stream_delta',content:'Partial answer'},{event:'error_recovery',detail:'fixture model failed'},mode==='error'?{event:'llm_stream_done',finish_reason:'error',error:'fixture model failed'}:{event:'final_response',detail:'Recovered answer'},{event:'done'}];
			return new Response(events.map(e=>'data: '+JSON.stringify(e)+'\n\n').join('')+'data: [DONE]\n\n');
		};
	}`)
	chat := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/agent-chat.js"))
	if err := p.AddScriptTag("", "(()=>{"+chat+"\nwindow.chatFixture={submit:submitDesktopChatMessage,question:showDesktopQuestionModal,stop:requestDesktopChatAbort,clear:clearDesktopChatHistory,runtime:useDesktopChatRuntime};})();"); err != nil {
		t.Fatal(err)
	}
	quick := normalizeAssetText(mustReadUIFile(t, "js/desktop/core/window-shell-runtime.js"))
	if err := p.AddScriptTag("", "(()=>{const state={};const t=window.t;const desktopReadonly=()=>false;const registerWidgetCleanup=()=>{};const esc=String;const iconMarkup=()=>'';"+quick+"\nwindow.quickFixture=renderQuickChatWidget;})();"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"error", "http", "network", "recover"} {
		result := p.MustEval(`async(mode)=>{
			window.mode=mode;const h=document.getElementById('host');h.querySelector('.vd-chat-log').innerHTML='';await chatFixture.submit(h,'hello');
			const text=h.textContent;
			if(h.querySelector('.vd-chat-status,.vd-streaming')||h.querySelector('.vd-chat-input').disabled)return 'busy state survived';
			if(mode==='error'&&(!text.includes('Partial answer')||!text.includes('fixture model failed')))return 'partial/error lost';
			if(mode==='recover'&&(!text.includes('Recovered answer')||text.includes('fixture model failed')))return 'recovery treated as terminal';
			if(mode==='http'&&!text.includes('fixture HTTP failure')||mode==='network'&&!text.includes('fixture disconnected'))return 'request failure hidden';
			quickFixture(document.getElementById('quick'));const input=document.querySelector('.vd-quickchat-input');input.value='hello';input.form.requestSubmit();
			await new Promise(resolve=>setTimeout(resolve,80));const q=document.querySelector('.vd-quickchat-response');
			if(mode==='error'&&(!q.textContent.includes('Partial answer')||!q.querySelector('[role=alert]')?.textContent.includes('fixture model failed')))return 'quickchat lost partial/error';
			if(mode==='recover'&&q.querySelector('[role=alert]'))return 'quickchat recovery became error';
			if(mode==='http'&&!q.textContent.includes('fixture HTTP failure')||mode==='network'&&!q.textContent.includes('fixture disconnected'))return 'quickchat failure hidden';return '';
		}`, mode).Str()
		if result != "" {
			t.Fatalf("%s: %s", mode, result)
		}
	}
	for _, desktop := range []bool{false, true} {
		p.MustEval(`desktop=>{document.getElementById('trigger').focus();window.failRequest=true;const payload={session_id:desktop?'virtual-desktop':'default',question:'Continue?',options:[{value:'yes',label:'Yes'}],allow_free_text:true};if(desktop)chatFixture.question(document.getElementById('host'),payload);else showQuestionModal(payload);}`, desktop)
		selector := ".question-modal-overlay"
		if desktop {
			selector = "[data-desktop-question-modal]"
		}
		p.MustElement(selector + " button").MustClick()
		waitForJSBool(t, p, `()=>!!document.querySelector('[data-question-error]')?.textContent`)
		p.MustElement(selector + " [data-question-cancel]").MustClick()
		waitForJSBool(t, p, `()=>document.querySelector('[data-question-cancel]')?.disabled===false`)
		if !p.MustEval(`()=>document.querySelector('[data-question-error]').textContent.includes('fixture refused')`).Bool() {
			t.Fatal("failed stop hid error")
		}
		p.MustEval(`selector=>{window.failRequest='notfound';document.querySelector(selector+' button').click();}`, selector)
		waitForJSBool(t, p, `()=>document.querySelector('[data-question-cancel]')?.disabled===false`)
		if !p.MustEval(`()=>!!document.querySelector('[data-question-error]')?.textContent`).Bool() {
			t.Fatal("HTTP 200 not_found closed the question")
		}
		p.MustEval(`()=>{window.failRequest=false;document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));}`)
		waitForJSBool(t, p, `()=>!document.querySelector('[data-question-cancel]')`)
		if !p.MustEval(`()=>document.activeElement.id==='trigger'&&calls.at(-1).url==='/api/admin/stop'`).Bool() {
			t.Fatal("Escape did not stop/restore focus")
		}
	}
	p.MustEval(`()=>{chatFixture.runtime({__desktopRuntime:{contentEl:()=>document.getElementById('host')}});chatFixture.question(document.getElementById('host'),{question:'Close window?',options:[]});AgentChatApp.dispose('fixture');}`)
	if !p.MustEval(`()=>!document.querySelector('[data-desktop-question-modal]')&&!document.getElementById('trigger').inert`).Bool() {
		t.Fatal("closing chat left a modal or inert background")
	}
	p.MustEval(`async()=>{window.failRequest=true;const h=document.getElementById('host');h._desktopChatAbort=()=>{throw new Error('must not abort after failed stop')};await chatFixture.stop(h);await chatFixture.clear(h,true);}`)
	if !p.MustEval(`()=>document.getElementById('host').textContent.includes('fixture refused')&&!!document.querySelector('.vd-chat-bubble')`).Bool() {
		t.Fatal("failed stop/clear hid existing content or error")
	}
	for _, state := range []string{"failed", "blocked", "cancelled"} {
		if !p.MustEval(`state=>{const text=DesktopChatRenderer.formatAgentActionStatus({event:'agent_action',payload:{state,tool_name:'fixture_tool'}});return text.includes('fixture_tool')&&!text.includes('recovery');}`, state).Bool() {
			t.Fatalf("incorrect %s tool state", state)
		}
		if !p.MustEval(`state=>{const owner=document.createElement('div');const action={event:'agent_action',payload:{state,tool_name:'fixture_tool'}};const end={event:'tool_end',detail:'fixture_tool'};if(!shouldShowAgentToolStatus(owner,action)||shouldShowAgentToolStatus(owner,end))return false;shouldShowAgentToolStatus(owner,{event:'tool_start',detail:'fixture_tool'});return shouldShowAgentToolStatus(owner,end);}`, state).Bool() {
			t.Fatalf("legacy end overwrote %s or poisoned the next run", state)
		}
	}
}
