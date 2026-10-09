package ui

import (
	"testing"
	"time"
)

// A Permissions-Policy can disable Web Serial although navigator.serial exists.
// The flasher must then show its unsupported hint instead of creating a token
// and letting ESP Web Tools fail on requestPort().
func TestCYDFlasherDetectsPolicyBlockedWebSerialBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(60 * time.Second)
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
        const original=window.fetch; window.cydRequests=[];
        const json=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
        window.fetch=async(url,options={})=>{
            const path=String(url);
            if(!path.startsWith('/api/cyd/')&&path!=='/api/tokens')return original(url,options);
            window.cydRequests.push(path);
            if(path==='/api/cyd/firmware/status')return json({provision_offset:2031616,variants:[{id:'cyd',available:true,version:'0.2.1',parts:[]},{id:'cyd2usb',available:true,version:'0.2.1',parts:[]}]});
            if(path==='/api/cyd/status')return json({enabled:true,devices:[],device_url:'https://aurago.fixture.invalid'});
            return json({error:'unexpected request'},500);
        };
    }`)
	page.MustEval(`async()=>{await selectSection('cyd');}`)
	waitForJSBool(t, page, `()=>document.getElementById('cyd-flash-status')?.textContent===t('config.cyd.flash_ready').replace('{version}','0.2.1').replace('{variant}','cyd')`)
	if page.MustEval(`()=>document.getElementById('cyd-flash-btn').disabled`).Bool() {
		t.Fatal("flash button disabled although Web Serial is allowed")
	}

	page.MustEval(`()=>{
        const policy={allowsFeature:feature=>feature!=='serial'};
        for(const name of ['permissionsPolicy','featurePolicy'])Object.defineProperty(document,name,{configurable:true,value:policy});
        cydUpdateFlashStatus();
    }`)
	if !page.MustEval(`()=>document.getElementById('cyd-flash-status').textContent===t('config.cyd.flash_unsupported')&&document.getElementById('cyd-flash-btn').disabled`).Bool() {
		t.Fatalf("policy-blocked Web Serial was not reported: %s", page.MustEval(`()=>document.getElementById('cyd-flash-status').textContent`).Str())
	}
	result := page.MustEval(`async()=>{
        window.cydRequests.length=0;
        await cydFlashDisplay();
        return {
            requests:window.cydRequests.slice(),
            installer:!!document.querySelector('#cyd-ewt-host esp-web-install-button'),
            status:document.getElementById('cyd-flash-status').textContent,
            unsupported:t('config.cyd.flash_unsupported')
        };
    }`)
	for _, request := range result.Get("requests").Arr() {
		if path := request.Str(); path == "/api/tokens" || path == "/api/cyd/firmware/provision" {
			t.Fatalf("blocked flasher still requested %s: %s", path, result.JSON("", ""))
		}
	}
	if result.Get("installer").Bool() || result.Get("status").Str() != result.Get("unsupported").Str() {
		t.Fatalf("blocked flasher still started ESP Web Tools: %s", result.JSON("", ""))
	}
}
