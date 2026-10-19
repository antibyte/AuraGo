package ui

import (
	"testing"
	"time"
)

func TestGo2RTCMediaConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(40 * time.Second)
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	page.MustWait(`()=>!!window.AuraConfigState&&!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async()=>{
        const original=window.fetch;
        window.fetch=async(url,options)=>String(url).startsWith('/api/go2rtc/')?new Response(JSON.stringify(String(url).endsWith('hardware/test')?{verified:false}:{api_usable:true}),{status:String(url).endsWith('hardware/test')?502:200,headers:{'Content-Type':'application/json'}}):original(url,options);
        configData.go2rtc.streams=[{id:'door',name:'Door',enabled:true,source:'••••••••',source_configured:true,sub_source_configured:true,preview_source:'auto',detection_source:'auto',snapshot_mode:'auto',codec_profile:'original',transport:'inherit',playback_mode:'auto',preload:false,talk_enabled:false}];
        AuraConfigState.init(configData);await selectSection('go2rtc');
    }`)
	page.MustWait(`()=>!!document.querySelector('[data-path="go2rtc.streams.0.codec_profile"]')`)
	if !page.MustEval(`()=>{AuraConfigState.syncFromDOM();return !AuraConfigState.isDirty()}`).Bool() {
		t.Fatal("opening camera configuration changed neutral defaults")
	}
	page.MustEval(`()=>{
        document.querySelector('.go2rtc-stream-card details').open=true;
        const field=document.querySelector('[data-path="go2rtc.streams.0.name"]');field.value='Draft';field.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	if !page.MustEval(`()=>AuraConfigState.isDirty()`).Bool() {
		t.Fatal("camera edit was not tracked")
	}
	page.MustEval(`()=>{const field=document.querySelector('[data-path="go2rtc.streams.0.name"]');field.value='Door';field.dispatchEvent(new Event('input',{bubbles:true}))}`)
	if !page.MustEval(`()=>!AuraConfigState.isDirty()`).Bool() {
		t.Fatal("undoing the camera edit left media defaults dirty")
	}
	page.MustEval(`()=>{const field=document.querySelector('[data-path="go2rtc.streams.0.codec_profile"]');field.value='h264_opus';field.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-path="go2rtc.streams.0.clear_sub_source"]').click()}`)
	if !page.MustEval(`()=>{const rows=AuraConfigState.buildPatch().go2rtc.streams;return Array.isArray(rows)&&rows.length===1&&rows[0].id==='door'&&rows[0].codec_profile==='h264_opus'&&rows[0].clear_sub_source===true&&rows[0].source==='••••••••'&&rows[0].sub_source===''}`).Bool() {
		t.Fatal("media controls lost source preservation or complete-list semantics")
	}
	page.MustEval(`async()=>{configData.go2rtc.hardware={backend:'vaapi',render_node:'/dev/dri/renderD129',render_group:'993',device_id:''};AuraConfigState.init(configData);await selectSection('go2rtc');AuraConfigState.syncFromDOM();await go2rtcHardwareTest()}`)
	if !page.MustEval(`()=>document.querySelector('#go2rtc-hardware-result').classList.contains('is-danger')&&document.querySelector('#go2rtc-hardware-result').textContent===t('config.go2rtc.hardware_failed')`).Bool() {
		t.Fatal("failed saved GPU probe was not visibly reported")
	}
}

func TestGo2RTCLegacyMediaConfigStaysCleanBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, populated := range []bool{false, true} {
		page := browser.MustPage(configRefreshFixtureOrigin(t, "de", populated) + "/config#overview").Timeout(40 * time.Second)
		page.MustWait(`()=>!!window.AuraConfigState&&!!document.querySelector('.pw-overview-card')`)
		page.MustEval(`async()=>{
            const original=window.fetch;window.fetch=async(url,opt)=>String(url).startsWith('/api/go2rtc/')?new Response('{"api_usable":true}',{headers:{'Content-Type':'application/json'}}):original(url,opt);
            configData.go2rtc.hardware={backend:'',render_node:'',render_group:'',device_id:''};
            configData.go2rtc.streams=[{id:'door',name:'Door',enabled:true,source:'••••••••',source_configured:true,sub_source:'',snapshot_source:'',preview_source:'',detection_source:'',snapshot_mode:'',codec_profile:'',transport:'',playback_mode:'',preload:false,talk_enabled:false}];
            AuraConfigState.init(configData);await selectSection('go2rtc');AuraConfigState.syncFromDOM();
        }`)
		if page.MustEval(`()=>AuraConfigState.isDirty()`).Bool() {
			t.Fatal("opening legacy camera config dirtied " + page.MustEval(`()=>JSON.stringify(AuraConfigState.dirtyPaths())`).Str())
		}
		page.MustEval(`()=>{const name=document.querySelector('[data-path="go2rtc.streams.0.name"]');name.value='Changed';name.dispatchEvent(new Event('input',{bubbles:true}));renderGo2RTCSection(null)}`)
		if !page.MustEval(`()=>AuraConfigState.isDirty()&&document.querySelector('[data-path="go2rtc.streams.0.name"]').value==='Changed'`).Bool() {
			t.Fatal("normalization accepted or discarded a real draft edit")
		}
		page.MustEval(`()=>{const name=document.querySelector('[data-path="go2rtc.streams.0.name"]');name.value='Door';name.dispatchEvent(new Event('input',{bubbles:true}));go2rtcAddStream();go2rtcRemoveStream(1);AuraConfigState.syncFromDOM()}`)
		if page.MustEval(`()=>AuraConfigState.isDirty()`).Bool() {
			t.Fatal("undoing an edit or camera addition left legacy defaults dirty")
		}
		page.MustEval(`()=>{const profile=document.querySelector('[data-path="go2rtc.streams.0.codec_profile"]');profile.value='h264_opus';profile.dispatchEvent(new Event('change',{bubbles:true}));renderGo2RTCSection(null)}`)
		if !page.MustEval(`()=>{const rows=AuraConfigState.buildPatch().go2rtc.streams;return rows.length===1&&rows[0].codec_profile==='h264_opus'&&rows[0].source==='••••••••'}`).Bool() {
			t.Fatal("legacy migration lost a real profile edit or its preserved source")
		}
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}
}
