package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigSpeechLabVoiceContractBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(30 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
        const originalFetch = window.fetch;
        const json = (body, status=200) => new Response(JSON.stringify(body), {status, headers:{'Content-Type':'application/json'}});
        window.stackPuts = [];
        window.stackConfirm = false;
        window.stackCode = 'voice_not_active';
        window.showConfirm = async () => window.stackConfirm;
        window.stackToasts = [];
        window.showToast = (message) => stackToasts.push(message);
        window.fetch = (url, options={}) => {
            const path = String(url);
            if (!path.startsWith('/api/speech-lab/')) return originalFetch(url, options);
            if (path.endsWith('/status')) return Promise.resolve(json({enabled:true,ready:true,asr_id:'asr',tts_id:'fixed',voice:'M1',deployment:{managed:true,state:'ready'}}));
            if (path.endsWith('/capability')) return Promise.resolve(json({vendor:'intel',device_name:'Intel Arc B580',tier:'host-vulkan',accelerators:['cpu','vulkan'],host_agent_online:false,vram_total_gb:null,ram_total_gb:32}));
            if (path.endsWith('/catalog')) return Promise.resolve(json({backends:[
                {id:'asr',stage:'asr',stable:true,available:true},
                ...['fixed','request','restart','future','legacy'].map(mode=>({id:mode,stage:'tts',stable:true,available:true,default_voice:'M1',voices:['M1','F1'],...(mode==='legacy'?{}:{voice_mode:mode})}))
            ]}));
            if (path.includes('/suggestions')) return Promise.resolve(json({suggested_pairs:[
                {asr_id:'asr',tts_id:'request',score:42,reason:'Local <img src=x onerror=alert(1)>',vram_gb:2.5},
                {asr_id:'missing',tts_id:'request',score:1}
            ]}));
            if (path.endsWith('/stack')) {stackPuts.push(JSON.parse(options.body));return Promise.resolve(json({error:stackCode,message:'SECRET upstream diagnostic'},409));}
            return Promise.resolve(json({},404));
        };
    }`)
	page.MustEval(`async () => {await selectSection('speech_lab');resetDirtySnapshot();}`)
	if !page.MustEval(`() => {
        const panel=document.getElementById('speech-lab-capability').textContent;
        return ['Intel Arc B580','intel','host-vulkan','cpu','vulkan','32 GB',t('config.speech_lab.capability_host_agent')].every(value=>panel.includes(value)) && !panel.includes('null');
    }`).Bool() {
		t.Fatal("flat hardware capability did not render")
	}
	for _, mode := range []string{"fixed", "legacy", "request", "restart", "future"} {
		page.MustEval(`mode => {const select=document.getElementById('speech-lab-tts');select.value=mode;select.dispatchEvent(new Event('change',{bubbles:true}));}`, mode)
		if !page.MustEval(`mode => {
            const voice=document.getElementById('speech-lab-voice');
            const apply=document.querySelector('.speech-lab-stack-actions button');
            if(mode==='future') return voice.disabled && apply.disabled && document.getElementById('speech-lab-voice-help').textContent===t('config.speech_lab.voice_mode_unsupported');
            if(mode==='fixed'||mode==='legacy') return voice.disabled && voice.options.length===1 && voice.value==='M1' && !apply.disabled;
            return !voice.disabled && voice.options.length===2 && !apply.disabled && document.getElementById('speech-lab-voice-help').textContent===t('config.speech_lab.voice_mode_'+mode);
        }`, mode).Bool() {
			t.Fatalf("voice controls violate mode %s", mode)
		}
	}
	if !page.MustEval(`() => document.querySelector('[data-speech-lab-suggestion="1"]').disabled && !document.querySelector('#speech-lab-suggestions img') && document.querySelector('[data-speech-lab-suggestion="0"]').title.includes('42')`).Bool() {
		t.Fatal("recommendations must validate catalog entries and escape metadata")
	}
	page.MustSetViewport(390, 800, 1, true)
	page.MustElement(`[data-speech-lab-suggestion="0"]`).MustClick()
	if !page.MustEval(`() => document.getElementById('speech-lab-tts').value==='request' && document.getElementById('speech-lab-voice').value==='M1' && stackPuts.length===0`).Bool() {
		t.Fatal("suggestion did not prefill without mutation")
	}
	page.MustElement(`.speech-lab-stack-actions button`).MustClick()
	if !page.MustEval(`() => stackPuts.length===0`).Bool() {
		t.Fatal("cancelled confirmation mutated the stack")
	}
	page.MustEval(`() => {window.stackConfirm=true;}`)
	page.MustElement(`.speech-lab-stack-actions button`).MustClick()
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.error_voice') && !document.getElementById('speech-lab-refresh').disabled`)
	if !page.MustEval(`() => stackPuts.length===1 && stackPuts[0].voice==='M1' && !('llm_id' in stackPuts[0]) && !stackToasts.join(' ').includes('SECRET')`).Bool() {
		t.Fatal("stack conflict lost identity, leaked diagnostics, or duplicated the mutation")
	}
	page.MustEval(`() => {window.stackCode='module_not_provisioned';}`)
	page.MustElement(`[data-speech-lab-suggestion="0"]`).MustClick()
	page.MustElement(`.speech-lab-stack-actions button`).MustClick()
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.error_module')`)
	page.MustEval(`() => {window.stackCode='unknown_private_diagnostic';}`)
	page.MustElement(`.speech-lab-stack-actions button`).MustClick()
	waitForJSBool(t, page, `() => document.getElementById('speech-lab-action-status').textContent===t('config.speech_lab.apply_failed')`)
	if !page.MustEval(`() => !stackToasts.join(' ').includes('SECRET') && !stackToasts.join(' ').includes('unknown_private_diagnostic')`).Bool() {
		t.Fatal("unknown upstream error leaked into the UI")
	}
	if !page.MustEval(`async () => {
        try {await speechLabJSON(new Response(JSON.stringify({error:'voice_not_active',message:'SECRET'}),{status:409}));} catch(error) {
            return error.status===409 && error.code==='voice_not_active' && error.message===t('config.speech_lab.error_voice');
        }
        return false;
    }`).Bool() {
		t.Fatal("UI HTTP error lost the status or structured code")
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`() => document.getElementById('speech-lab-stack').scrollIntoView({block:'center',behavior:'instant'})`)
		if err := os.WriteFile(filepath.Join(dir, "speech-lab-stack-mobile.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSpeechLabDrawerRespectsAvailabilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(newPrecisionSmokeOrigin(t)).Timeout(20 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	page.MustSetDocumentContent(`<div id="integrations-drawer" class="integrations-drawer"><div id="integrations-list" class="integrations-list"></div></div>`)
	page.MustEval(`source => {const style=document.createElement('style');style.textContent=source;document.head.appendChild(style);}`, readSpeechLabUIFile(t, "css/integrations-drawer.css"))
	page.MustEval(`() => {
        window.I18N={'chat.speech_lab_auth_required':'AUTH_REQUIRED: Bitte aktiviere die AuraGo-Anmeldung, um das Sprachlabor zu öffnen.','chat.speech_lab_not_ready_settings':'CHECK_SETTINGS: Das Sprachlabor ist noch nicht bereit. Prüfe die Verbindung in den Einstellungen.','chat.speech_lab_settings':'Speech-Lab-Einstellungen'};
        window.drawerItem={id:'speech_lab',name:'Speech Lab',status:'offline',url:'',message_key:'chat.speech_lab_auth_required'};
        window.fetch=async()=>({ok:true,json:async()=>({webhosts:[window.drawerItem]})});
    }`)
	page.MustEval(`source => {const script=document.createElement('script');script.textContent=source;document.body.appendChild(script);}`, readSpeechLabUIFile(t, "js/chat/modules/integrations-drawer.js"))
	page.MustEval(`async () => {await IntegrationsDrawer.refresh();IntegrationsDrawer.open();}`)
	if !page.MustEval(`() => document.getElementById('integrations-list').textContent.includes('AUTH_REQUIRED') && !!document.querySelector('a[href="/config#speech_lab"]') && !document.querySelector('a[href="/speech-lab/"]')`).Bool() {
		t.Fatal("drawer replaced the unavailable browser URL or hid the auth hint")
	}
	page.MustEval(`async () => {window.drawerItem={id:'speech_lab',status:'starting',url:'/speech-lab/'};await IntegrationsDrawer.refresh();}`)
	if !page.MustEval(`() => document.getElementById('integrations-list').textContent.includes('CHECK_SETTINGS') && !!document.querySelector('a[href="/config#speech_lab"]') && !!document.querySelector('a[href="/speech-lab/"]')`).Bool() {
		t.Fatal("starting Speech Lab must offer its settings alongside the authorized Browser Lab")
	}
	for _, width := range []int{390, 1280} {
		page.MustSetViewport(width, 800, 1, false)
		if !page.MustEval(`() => {
            const item=document.querySelector('.integration-item'), actions=document.querySelector('.integration-actions');
            return item.scrollWidth<=item.clientWidth+1 && actions.scrollWidth<=actions.clientWidth+1 && getComputedStyle(document.querySelector('.integration-hint')).whiteSpace==='normal';
        }`).Bool() {
			t.Fatalf("drawer actions or status hint do not fit width %d", width)
		}
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		page.MustSetViewport(390, 800, 1, false)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "speech-lab-drawer-mobile.png"), page.MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustEval(`async () => {window.drawerItem={id:'speech_lab',status:'running',url:'/speech-lab/'};await IntegrationsDrawer.refresh();}`)
	if !page.MustEval(`() => !!document.querySelector('a[href="/speech-lab/"]') && !document.querySelector('a[href="/config#speech_lab"]')`).Bool() {
		t.Fatal("ready Speech Lab should retain its browser action")
	}
}
