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
