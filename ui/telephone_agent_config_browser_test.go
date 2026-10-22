package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func newTelephoneConfigBrowserPage(t *testing.T, locale string) *rod.Page {
	t.Helper()
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, locale, false) + "/config#overview").Timeout(45 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
 const original=window.fetch.bind(window), clone=value=>JSON.parse(JSON.stringify(value));
 const voice={backend:'classic',agent_provider_id:'phone',classic:{asr_provider_id:'phone',asr_mode:'whisper',tts_provider:'google'},behavior:{purpose:'Saved purpose'},allowed_tools:['list_processes'],language:'auto'};
 const state=window.telephoneFixture={
  profile:{config:{inbound_route:'agent',auto_answer_delay_ms:1000,voice},blockers:[],inherited:{},daily_usage:{available:true,used:0,limit:10}},
  catalog:{providers:[{id:'phone',name:'Phone provider',ready:true},{id:'incomplete',name:'Incomplete provider',ready:false,reason:'missing_model'}],tts_providers:[{id:'google',name:'Google',ready:true}],realtime_profiles:[],asr_modes:['whisper'],tools:[{id:'list_processes',description:'List processes'}]},
  sip:{enabled:true,readonly:false,preset_id:'fritzbox',registrar:'192.0.2.10',domain:'192.0.2.10',username:'desk',auth_username:'desk',display_name:'Saved handset',bind_host:'127.0.0.1',bind_port:5060,transport:'udp',outbound_proxy:'',advertised_signaling_host:'',advertised_signaling_port:0,register_expires_seconds:600,prefer_srv:false,password_set:true,
   tls:{server_name:'',cert_file:'',key_file:'',insecure_skip_verify:false},media:{rtp_port_start:30000,rtp_port_end:30099,jitter_buffer_ms:60,rtp_idle_timeout_seconds:60,advertised_host:'',codecs:['pcma','pcmu']},browser_media:{enabled:true,bind_host:'',udp_port:30100,advertised_ip:''},
   inbound:{route:'agent',auto_answer_delay_ms:1000,ring_timeout_seconds:90,number_region:'',trusted_peer_cidrs:['192.0.2.10/32'],allowed_callers:['*'],denied_callers:[]},outbound:{allowed_domains:['192.0.2.10'],denied_domains:[],allowed_users:['101'],denied_users:[],allowed_e164_prefixes:[],denied_e164_prefixes:[]},permissions:{answer_inbound:true,originate_outbound:true,send_dtmf:true,agent_hangup:true},voice:clone(voice),history_retention_days:90},
  saves:[],sipSaves:[],tests:[],profileReads:0,catalogReads:0
 };
 const json=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
 window.fetch=async(input,options={})=>{
  const path=new URL(typeof input==='string'?input:input.url,location.origin).pathname;
  if(path==='/api/sip/agent'){
   if(options.method==='PUT'){
    const sent=JSON.parse(options.body);state.saves.push(clone(sent));
    if(state.holdSave)await new Promise(resolve=>state.releaseSave=resolve);
    if(state.failSave)return json({message:'Simulated save failure'},500);
    state.profile.config=sent;return json(state.profile);
   }
   state.profileReads++;
   if(state.failProfile)return json({message:'Simulated profile failure'},503);
   if(state.holdProfile)await new Promise(resolve=>state.releaseProfile=resolve);
   return json(state.profile);
  }
  if(path==='/api/sip/agent/catalog'){
   state.catalogReads++;
   return state.failCatalog?json({message:'Simulated catalog failure'},503):json(state.catalog);
  }
  if(path==='/api/sip/agent/test'){
   const body=JSON.parse(options.body);state.tests.push(body);
   if(state.holdTest)await new Promise(resolve=>state.releaseTest=resolve);
   if(state.blockTest)return json({status:'blocked',blockers:['asr_unavailable']},409);
   return json({status:'ok',mode:body.live?'live':'preflight'});
  }
  if(path==='/api/sip/config'){
   if(options.method==='PUT'){
    const sent=JSON.parse(options.body);state.sipSaves.push(clone(sent));
    if(state.holdSIPSave)await new Promise(resolve=>state.releaseSIPSave=resolve);
    if(state.failSIPSave)return json({message:'Simulated SIP save failure'},500);
    state.sip=sent;return json(state.sip);
   }
   return json(state.sip);
  }
  return original(input,options);
 };
}`)
	return page
}

func TestTelephoneAgentInitialCatalogFailureKeepsASRMode(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, mode := range []string{"whisper", "multimodal", "local", "speech_lab"} {
		t.Run(mode, func(t *testing.T) {
			page := newTelephoneConfigBrowserPage(t, "de")
			page.MustEval(`async mode=>{
 telephoneFixture.profile.config.voice.classic.asr_mode=mode;
 telephoneFixture.failCatalog=true;
 await selectSection('telephone_agent',{scrollBehavior:'auto'});
}`, mode)
			if !page.MustEval(`mode=>document.querySelector('[data-ta="voice.classic.asr_mode"]').value===mode&&taRead().voice.classic.asr_mode===mode&&!telephoneAgentHasUnsavedChanges()&&!isDirty`, mode).Bool() {
				t.Fatal("initial catalog failure changed the ASR mode or created an unsaved draft")
			}
			page.MustEval(`async()=>{
 const input=document.querySelector('[data-ta="voice.behavior.purpose"]');
 input.value='Purpose edited without catalog';input.dispatchEvent(new Event('input',{bubbles:true}));
 await taSave();
}`)
			if !page.MustEval(`mode=>telephoneFixture.saves.length===1&&telephoneFixture.saves[0].voice.classic.asr_mode===mode&&telephoneFixture.saves[0].voice.behavior.purpose==='Purpose edited without catalog'&&!telephoneAgentHasUnsavedChanges()`, mode).Bool() {
				t.Fatal("saving after a catalog failure lost the ASR mode or draft edit")
			}
			page.MustEval(`async()=>{
 telephoneFixture.failCatalog=false;
 telephoneFixture.catalog.asr_modes=['whisper','multimodal','local','speech_lab'];
 await taLoad();
}`)
			if !page.MustEval(`mode=>document.querySelector('[data-ta="voice.classic.asr_mode"]').value===mode&&document.querySelector('[data-ta="voice.classic.asr_mode"]').options.length===4&&!telephoneAgentHasUnsavedChanges()&&!document.querySelector('[data-ta-action="retry"]')`, mode).Bool() {
				t.Fatal("catalog recovery changed the ASR mode or kept the failure state")
			}
		})
	}
}

func TestTelephoneAgentConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, theme := range []string{"dark", "light"} {
		for _, width := range []int{1440, 390} {
			t.Run(fmt.Sprintf("%s-%d", theme, width), func(t *testing.T) {
				locale := "en"
				if width == 390 {
					locale = "de"
				}
				page := newTelephoneConfigBrowserPage(t, locale)
				page.MustSetViewport(width, 900, 1, width == 390)
				page.MustEval(`async theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;telephoneFixture.failProfile=true;await selectSection('telephone_agent',{scrollBehavior:'auto'});}`, theme)
				if !page.MustEval(`()=>!document.querySelector('[data-ta]')&&!!document.querySelector('[data-ta-action="retry"]')`).Bool() {
					t.Fatal("failed first profile load offered an editable default")
				}
				page.MustEval(`async()=>{telephoneFixture.failProfile=false;await taLoad();resetDirtySnapshot();}`)
				if !page.MustEval(`()=>!!document.querySelector('[data-ta-search][aria-label]')&&document.querySelector('[data-ta="voice.language"] option[value="auto"]').textContent!=='auto'&&!document.querySelector('[data-ta="voice.agent_provider_id"] option[value="incomplete"]').textContent.includes('missing_model')&&document.querySelector('[data-ta="voice.agent_provider_id"] option[value="incomplete"]').title===t('config.telephone_agent.provider_unavailable')`).Bool() {
					t.Fatal("search or language choices lack accessible labels")
				}

				page.MustEval(`()=>{const input=document.querySelector('[data-ta="voice.behavior.purpose"]');input.value='Sent purpose';input.dispatchEvent(new Event('input',{bubbles:true}));telephoneFixture.holdSave=true;document.querySelector('[data-ta-action="save"]').click();document.querySelector('[data-ta-action="save"]').click();window.joinedTelephoneSave=saveConfig();}`)
				waitForJSBool(t, page, `()=>!!telephoneFixture.releaseSave`)
				page.MustEval(`()=>{const input=document.querySelector('[data-ta="voice.behavior.purpose"]');input.value='Newer purpose';input.dispatchEvent(new Event('input',{bubbles:true}));const pipeline=document.querySelector('[data-ta="voice.backend"]');pipeline.value='gemini_live';pipeline.dispatchEvent(new Event('change',{bubbles:true}));}`)
				if !page.MustEval(`async()=>telephoneFixture.saves.length===1&&document.querySelector('[data-ta-action="save"]').disabled&&await navigateToConfigSection('overview')===false`).Bool() {
					t.Fatal("duplicate save, rerender, or navigation escaped the pending save")
				}
				page.MustEval(`async()=>{telephoneFixture.holdSave=false;telephoneFixture.releaseSave();await joinedTelephoneSave;}`)
				if !page.MustEval(`()=>document.querySelector('[data-ta="voice.behavior.purpose"]').value==='Newer purpose'&&telephoneAgentHasUnsavedChanges()&&isDirty&&document.getElementById('saveStatus').textContent===t('config.save_bar.unsaved_changes')&&telephoneFixture.saves[0].voice.behavior.purpose==='Sent purpose'&&!document.querySelector('[data-ta-action="save"]').disabled`).Bool() {
					t.Fatal("successful save discarded or marked newer edits as saved")
				}

				page.MustEval(`()=>{window.showUnsavedChangesDecision=async()=> 'save';telephoneFixture.holdSave=true;window.telephoneNavigation=navigateToConfigSection('overview');}`)
				waitForJSBool(t, page, `()=>telephoneFixture.saves.length===2`)
				page.MustEval(`async()=>{const input=document.querySelector('[data-ta="voice.behavior.purpose"]');input.value='Latest purpose';input.dispatchEvent(new Event('input',{bubbles:true}));telephoneFixture.holdSave=false;telephoneFixture.releaseSave();await telephoneNavigation;}`)
				if !page.MustEval(`()=>activeSection==='telephone_agent'&&telephoneAgentHasUnsavedChanges()&&document.querySelector('[data-ta="voice.behavior.purpose"]').value==='Latest purpose'`).Bool() {
					t.Fatal("save-and-continue left behind edits made during saving")
				}
				page.MustEval(`async()=>{telephoneFixture.failSave=true;await taSave();telephoneFixture.failSave=false;}`)
				if !page.MustEval(`()=>telephoneAgentHasUnsavedChanges()&&!document.querySelector('[data-ta-action="save"]').disabled`).Bool() {
					t.Fatal("failed save lost the draft or left controls disabled")
				}

				page.MustEval(`async()=>{telephoneAgentDiscardUnsaved();telephoneFixture.profile.config.voice.allowed_tools=['list_processes','retired_tool'];telephoneFixture.profile.config.voice.backend='classic';await renderTelephoneAgentSection();}`)
				if !page.MustEval(`()=>document.querySelector('[data-ta-tool][value="retired_tool"]')?.checked`).Bool() {
					t.Fatal("unavailable saved tool is not removable")
				}
				page.MustEval(`async()=>{const input=document.querySelector('[data-ta-tool][value="retired_tool"]');input.checked=false;input.dispatchEvent(new Event('change',{bubbles:true}));const search=document.querySelector('[data-ta-search]');search.value='retired';search.dispatchEvent(new Event('input',{bubbles:true}));await taSave();}`)
				if !page.MustEval(`()=>!telephoneFixture.saves.at(-1).voice.allowed_tools.includes('retired_tool')&&telephoneFixture.saves.at(-1).voice.allowed_tools.includes('list_processes')`).Bool() {
					t.Fatal("tool removal lost the remaining selection")
				}

				page.MustEval(`async()=>{telephoneFixture.blockTest=true;await taTest(false);telephoneFixture.blockTest=false;}`)
				if !page.MustEval(`()=>document.querySelector('[data-ta-blockers]').textContent.includes(t('config.telephone_agent.blocker.asr_unavailable'))&&!document.getElementById('telephone-agent-status').textContent.includes('HTTP 409')&&!document.querySelector('[data-ta-action="preflight"]').disabled`).Bool() {
					t.Fatal("structured blocker was not translated or controls stayed disabled")
				}

				page.MustEval(`async()=>{telephoneFixture.profile.config.voice.agent_provider_id='retired_provider';telephoneFixture.failCatalog=true;await taLoad();const input=document.querySelector('[data-ta="voice.behavior.purpose"]');input.value='Catalog draft';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
				if !page.MustEval(`()=>!!document.querySelector('[data-ta-action="retry"]')&&!document.querySelector('[data-ta-action="save"]').disabled&&document.querySelector('[data-ta="voice.agent_provider_id"]').value==='retired_provider'`).Bool() {
					t.Fatal("catalog failure discarded or disabled the loaded profile")
				}
				page.MustEval(`async()=>{telephoneFixture.failCatalog=false;await taLoad();telephoneFixture.profile.blockers=['tts_unavailable'];document.dispatchEvent(new CustomEvent('aurago:config-saved'));}`)
				waitForJSBool(t, page, `()=>document.querySelector('[data-ta-blockers]').textContent.includes(t('config.telephone_agent.blocker.tts_unavailable'))`)
				if !page.MustEval(`()=>document.querySelector('[data-ta="voice.behavior.purpose"]').value==='Catalog draft'&&telephoneAgentHasUnsavedChanges()&&!document.querySelector('[data-ta-action="retry"]')&&document.documentElement.scrollWidth<=innerWidth+1`).Bool() {
					t.Fatal("retry or status refresh lost the draft or broke the layout")
				}
				page.MustEval(`async()=>{telephoneFixture.failProfile=true;await taLoad();}`)
				if !page.MustEval(`()=>document.querySelector('[data-ta="voice.behavior.purpose"]').value==='Catalog draft'&&document.querySelector('[data-ta-action="save"]').disabled&&telephoneAgentHasUnsavedChanges()`).Bool() {
					t.Fatal("profile refresh failure discarded an existing draft or allowed a blind save")
				}
				page.MustEval(`async()=>{telephoneFixture.failProfile=false;await taLoad();}`)
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("telephone-agent-%s-%d.png", theme, width)), page.MustScreenshot(), 0o644); err != nil {
						t.Fatal(err)
					}
				}

				page.MustEval(`async()=>{telephoneAgentDiscardUnsaved();markDirty();telephoneFixture.profile.config.voice.agent_provider_id='phone';await taLoad();telephoneFixture.holdTest=true;document.querySelector('[data-ta-live-confirm]').checked=true;document.querySelector('[data-ta-action="live-test"]').click();document.querySelector('[data-ta-action="live-test"]').click();}`)
				waitForJSBool(t, page, `()=>!!telephoneFixture.releaseTest`)
				if !page.MustEval(`()=>telephoneFixture.tests.filter(test=>test.live).length===1&&document.querySelector('[data-ta-action="live-test"]').disabled`).Bool() {
					t.Fatal("double click started multiple live tests")
				}
				page.MustEval(`async()=>{await navigateToConfigSection('overview');telephoneFixture.holdTest=false;telephoneFixture.releaseTest();}`)
				waitForJSBool(t, page, `()=>telephoneAgentAction===''`)
				if !page.MustEval(`()=>activeSection==='overview'&&!!document.querySelector('.pw-overview-card')&&!document.querySelector('.telephone-agent-section')`).Bool() {
					t.Fatal("late test result repainted another section")
				}
			})
		}
	}
}

func TestSIPConfigDraftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newTelephoneConfigBrowserPage(t, "de")
	page.MustEval(`async()=>{await selectSection('sip',{scrollBehavior:'auto'});document.querySelector('.sip-advanced').open=true;resetDirtySnapshot();}`)
	if !page.MustEval(`()=>sipWizardInboundEnabled`).Bool() {
		t.Fatal("guided setup treated an existing agent route as disabled")
	}
	page.MustEval(`()=>{const input=document.querySelector('[data-sip="display_name"]');input.value='Sent handset';input.dispatchEvent(new Event('input',{bubbles:true}));telephoneFixture.holdSIPSave=true;window.firstSIPSave=sipSave();window.joinedSIPSave=sipSave();}`)
	waitForJSBool(t, page, `()=>!!telephoneFixture.releaseSIPSave`)
	if !page.MustEval(`async()=>telephoneFixture.sipSaves.length===1&&await navigateToConfigSection('overview')===false`).Bool() {
		t.Fatal("expert save duplicated or allowed navigation during a pending write")
	}
	page.MustEval(`async()=>{const input=document.querySelector('[data-sip="display_name"]');input.value='Newer handset';input.dispatchEvent(new Event('input',{bubbles:true}));telephoneFixture.holdSIPSave=false;telephoneFixture.releaseSIPSave();await Promise.all([firstSIPSave,joinedSIPSave]);}`)
	if !page.MustEval(`()=>document.querySelector('[data-sip="display_name"]').value==='Newer handset'&&sipHasUnsavedChanges()&&isDirty&&!document.querySelector('[data-sip-action="save"]').disabled&&telephoneFixture.sipSaves[0].display_name==='Sent handset'`).Bool() {
		t.Fatal("expert save discarded or marked newer edits as saved")
	}
	page.MustEval(`async()=>{telephoneFixture.failSIPSave=true;await sipSave();telephoneFixture.failSIPSave=false;}`)
	if !page.MustEval(`()=>sipHasUnsavedChanges()&&document.querySelector('[data-sip="display_name"]').value==='Newer handset'&&!document.querySelector('[data-sip-action="save"]').disabled`).Bool() {
		t.Fatal("failed expert save lost the draft or disabled saving")
	}
}
