package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigMeshCoreBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	page = page.Timeout(30 * time.Second)
	defer func() {
		_, _ = page.Eval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		_ = page.Close()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
        const original = window.fetch;
        window.meshRequests = [];
        const settings = {...configData.meshcore, enabled:true, port:'COM3', channels:[{index:7,mode:'prefix',binding:'55'.repeat(32)}]};
        configData.meshcore = settings; AuraConfigState.init(configData);
        window.fetch = async (url, options={}) => {
            if (!String(url).startsWith('/api/meshcore/')) return original(url,options);
            meshRequests.push({url:String(url), body:options.body || ''});
            const state = {state:'binding_required',identity_key:'11'.repeat(32),name:'Companion',firmware:'fixture',hardware_verified:false,
                contacts:[{key:'22'.repeat(32),name:'<script>window.meshInjection=true</script>',type:1}],
                channels:[{index:0,name:'Public',binding:'33'.repeat(32)}]};
            let body = {};
            if (url.endsWith('/status')) body={status:state,config:settings,ble_supported:false};
            if (url.endsWith('/test')) body={status:state};
            if (url.endsWith('/devices')) body={ports:['COM3']};
            if (url.includes('/messages')) body={messages:[{id:'44'.repeat(32),direction:'incoming',kind:'channel',channel:0,received_at:1800000000,state:'quarantine',review:'suspicious',text:'<img src=x onerror="window.meshInjection=true">'}]};
            return new Response(JSON.stringify(body),{status:200,headers:{'Content-Type':'application/json'}});
        };
    }`)
	page.MustEval(`async () => { await selectSection('meshcore'); resetDirtySnapshot(); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('#meshcore-inbox pre') && !document.querySelector('[data-mesh-action="test"]').disabled`)
	if page.MustEval(`() => !!window.meshInjection || !!document.querySelector('#meshcore-inbox img, #meshcore-contacts script')`).Bool() {
		t.Fatal("external HTML executed")
	}
	page.MustElement(`[data-mesh-action="test"]`).MustClick()
	waitForJSBool(t, page, `() => meshRequests.some(r=>r.url.endsWith('/test')) && !document.querySelector('[data-mesh-action="confirm"]').disabled`)
	page.MustElement(`[data-mesh-action="confirm"]`).MustClick()
	page.MustEval(`() => {const rows=document.querySelectorAll('.meshcore-channel');if(rows.length!==2)throw Error('Missing orphaned rule');rows[1].querySelector('button:last-child').click();}`)
	page.MustElement(`#meshcore-channels button`).MustClick()
	page.MustEval(`() => {const select=document.querySelector('#meshcore-channels select');select.value='prefix';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	if !page.MustEval(`() => {const patch=AuraConfigState.buildPatch().meshcore;return patch.identity_key==='11'.repeat(32) && patch.channels[0].binding==='33'.repeat(32) && patch.channels[0].mode==='prefix' && document.querySelector('[data-mesh-action="test"]').disabled;}`).Bool() {
		t.Fatal("binding or saved-state lock lost")
	}
	for _, width := range []int{390, 768, 1024, 1440, 1920} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`(theme,density) => {document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;AuraPrecisionWorkspace.setDensity(density);}`, theme, density)
				page.MustEval(`() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
				if page.MustEval(`() => {const el=document.getElementById('content'),dock=document.querySelector('.save-bar');return el.scrollWidth>el.clientWidth+1 || el.getBoundingClientRect().bottom>dock.getBoundingClientRect().top+1}`).Bool() {
					t.Fatalf("overflow at %d", width)
				}
			}
		}
	}
	if page.MustEval(`() => meshRequests.some(r=>r.url.includes('/send') || r.body.includes('pin'))`).Bool() {
		t.Fatal("setup sent radio text or PIN")
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meshcore-config.png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
