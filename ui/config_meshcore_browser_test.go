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
	capture := func(name string) {
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
        const original = window.fetch;
        window.meshRequests = [];
        window.meshPorts = ['COM3', 'COM10', 'COM4', 'COM4'];
        const settings = {...configData.meshcore, enabled:true, port:'COM3', channels:[{index:7,mode:'prefix',binding:'55'.repeat(32)}]};
        configData.meshcore = settings; AuraConfigState.init(configData);
        window.fetch = async (url, options={}) => {
            if (!String(url).startsWith('/api/meshcore/')) return original(url,options);
            meshRequests.push({url:String(url), body:options.body || ''});
            const state = {state:'binding_required',identity_key:'11'.repeat(32),name:'Companion',firmware:'fixture',hardware_verified:false,
                contacts:[{key:'22'.repeat(32),name:'<script>window.meshInjection=true</script>',type:1}],
                channels:[{index:0,name:'Public',binding:'33'.repeat(32)}]};
            let body = {};
            if (url.endsWith('/status')) body={status:state,config:settings,ble_supported:true};
            if (url.endsWith('/test')) {
                await new Promise((resolve,reject)=>{window.meshCompleteTest=resolve;window.meshFailTest=reject;});
                body={status:state};
            }
            if (url.endsWith('/devices')) {
                if (window.meshPortsFail) throw Error('port enumeration unavailable');
                body={ports:window.meshPorts};
            }
            if (url.endsWith('/scan')) {
                if (window.meshScanFails) throw Error('discovery unavailable');
                await new Promise(resolve=>{window.meshCompleteScan=resolve;});
                body={devices:[{address:'aa:bb:cc:dd:ee:ff',name:'<img src=x onerror="window.meshInjection=true">'}, {address:'11:22:33:44:55:66'}, {address:'invalid'}]};
            }
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
	waitForJSBool(t, page, `() => !!window.meshCompleteTest && !document.querySelector('#meshcore-test-status').hidden && !!document.querySelector('#meshcore-test-status .spinner') && document.querySelector('#meshcore-test-status').getAttribute('aria-busy')==='true' && document.querySelector('[data-mesh-action="test"]').disabled`)
	page.MustEval(`() => meshCompleteTest()`)
	waitForJSBool(t, page, `() => meshRequests.some(r=>r.url.endsWith('/test')) && !document.querySelector('[data-mesh-action="confirm"]').disabled`)
	if !page.MustEval(`() => {const result=document.querySelector('#meshcore-test-status');return result.getAttribute('aria-busy')==='false' && !result.querySelector('.spinner') && result.textContent.includes(t('config.meshcore.success')) && result.textContent.includes(t('config.meshcore.confirm')) && !AuraConfigState.get('meshcore.identity_key');}`).Bool() {
		t.Fatal("test must report success and required identity confirmation without granting trust")
	}
	page.MustElement(`[data-mesh-action="refresh"]`).MustClick()
	waitForJSBool(t, page, `() => document.querySelector('[data-path="meshcore.port"]').getAttribute('aria-busy')==='false' && !document.querySelector('[data-mesh-action="test"]').disabled`)
	if !page.MustEval(`() => document.querySelector('#meshcore-test-status').textContent.includes(t('config.meshcore.success'))`).Bool() {
		t.Fatal("runtime refresh erased the connection test result")
	}
	page.MustEval(`() => {window.meshCompleteTest=null;window.meshFailTest=null;}`)
	page.MustElement(`[data-mesh-action="test"]`).MustClick()
	waitForJSBool(t, page, `() => !!window.meshFailTest`)
	page.MustEval(`() => meshFailTest(new DOMException('Timed out','AbortError'))`)
	waitForJSBool(t, page, `() => {const result=document.querySelector('#meshcore-test-status');return result.textContent===t('config.meshcore.failed') && result.getAttribute('aria-busy')==='false' && !result.querySelector('.spinner') && !document.querySelector('[data-mesh-action="test"]').disabled;}`)
	if !page.MustEval(`() => {const port=document.querySelector('select[data-path="meshcore.port"]');return port.value==='COM3' && JSON.stringify([...port.options].map(o=>o.value))==='["","COM3","COM4","COM10"]';}`).Bool() {
		t.Fatal("serial port dropdown did not list detected ports")
	}
	page.MustEval(`() => {const port=document.querySelector('[data-path="meshcore.port"]');port.value='COM4';port.dispatchEvent(new Event('change',{bubbles:true}));window.meshPorts=null;}`)
	if !page.MustEval(`() => AuraConfigState.buildPatch().meshcore.port==='COM4' && document.querySelector('[data-mesh-action="test"]').disabled`).Bool() {
		t.Fatal("port selection did not enter the config draft")
	}
	page.MustElement(`[data-mesh-action="refresh"]`).MustClick()
	waitForJSBool(t, page, `() => {const port=document.querySelector('[data-path="meshcore.port"]');return port.getAttribute('aria-busy')==='false' && port.value==='COM4' && port.selectedOptions[0].textContent.includes(t('config.meshcore.port_unavailable')) && port.options[0].textContent===t('config.meshcore.no_ports');}`)
	page.MustEval(`() => {window.meshPortsFail=true;}`)
	page.MustElement(`[data-mesh-action="refresh"]`).MustClick()
	waitForJSBool(t, page, `() => {const port=document.querySelector('[data-path="meshcore.port"]');return port.getAttribute('aria-busy')==='false' && port.options[0].textContent===t('config.meshcore.failed') && port.value==='COM4' && AuraConfigState.get('meshcore.port')==='COM4';}`)
	page.MustEval(`() => {window.meshPortsFail=false;window.meshPorts=['COM3','COM4'];const port=document.querySelector('[data-path="meshcore.port"]');port.value='';port.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement(`[data-mesh-action="refresh"]`).MustClick()
	waitForJSBool(t, page, `() => {const port=document.querySelector('[data-path="meshcore.port"]');return port.getAttribute('aria-busy')==='false' && port.options.length===3 && port.value==='' && AuraConfigState.get('meshcore.port')==='';}`)
	page.MustEval(`() => {const port=document.querySelector('[data-path="meshcore.port"]');port.value='COM3';port.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement(`.meshcore-pairing summary`).MustClick()
	page.MustElement(`[data-mesh-action="scan"]`).MustClick()
	waitForJSBool(t, page, `() => !!window.meshCompleteScan && !!document.querySelector('#meshcore-discovery .spinner') && document.querySelector('#meshcore-discovery').getAttribute('aria-busy')==='true' && document.querySelector('[data-mesh-action="scan"]').disabled`)
	capture("meshcore-searching.png")
	page.MustEval(`() => meshCompleteScan()`)
	waitForJSBool(t, page, `() => document.querySelectorAll('.meshcore-device-choice').length===2 && document.querySelector('#meshcore-discovery').getAttribute('aria-busy')==='false' && !document.querySelector('[data-mesh-action="select_device"]').disabled`)
	if page.MustEval(`() => !!window.meshInjection || !!document.querySelector('#meshcore-discovery img')`).Bool() {
		t.Fatal("device name executed HTML")
	}
	page.MustEval(`() => document.querySelector('.meshcore-pairing').scrollIntoView({block:'start'})`)
	capture("meshcore-devices.png")
	page.MustElement(`[data-mesh-action="select_device"]`).MustClick()
	if !page.MustEval(`() => {const draft=AuraConfigState.get('meshcore');return draft.address==='AA:BB:CC:DD:EE:FF' && draft.transport==='ble' && document.querySelector('[data-path="meshcore.address"]').value===draft.address && document.querySelector('[data-path="meshcore.transport"]').value==='ble' && document.querySelector('[data-mesh-action="select_device"]').getAttribute('aria-pressed')==='true' && document.querySelector('[data-mesh-action="pair"]').disabled && document.querySelector('[data-mesh-action="test"]').disabled && !draft.identity_key && !(draft.trusted_nodes||[]).length && !meshRequests.some(r=>r.url.endsWith('/pair'));}`).Bool() {
		t.Fatalf("selection did not update only the draft connection: %s", page.MustEval(`() => JSON.stringify({draft:AuraConfigState.get('meshcore'),address:document.querySelector('[data-path="meshcore.address"]').value,transport:document.querySelector('[data-path="meshcore.transport"]').value,pressed:document.querySelector('[data-mesh-action="select_device"]').getAttribute('aria-pressed'),pair:document.querySelector('[data-mesh-action="pair"]').disabled,test:document.querySelector('[data-mesh-action="test"]').disabled,requests:meshRequests})`).Str())
	}
	page.MustEval(`() => {AuraConfigState.init(configData); resetDirtySnapshot(); document.dispatchEvent(new Event('cfg:statechange')); window.meshScanFails=true;}`)
	page.MustElement(`[data-mesh-action="scan"]`).MustClick()
	waitForJSBool(t, page, `() => !document.querySelector('[data-mesh-action="scan"]').disabled && !document.querySelector('#meshcore-discovery .spinner') && document.querySelector('#meshcore-discovery').getAttribute('aria-busy')==='false' && document.querySelector('#meshcore-discovery').textContent===t('config.meshcore.failed')`)
	page.MustEval(`() => {window.meshScanFails=false;window.meshCompleteScan=null;}`)
	page.MustElement(`[data-mesh-action="scan"]`).MustClick()
	waitForJSBool(t, page, `() => !!window.meshCompleteScan`)
	page.MustEval(`() => meshCompleteScan()`)
	waitForJSBool(t, page, `() => document.querySelectorAll('.meshcore-device-choice').length===2 && !document.querySelector('[data-mesh-action="scan"]').disabled`)
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
				if !page.MustEval(`() => {
                    const rect=selector=>document.querySelector(selector).getBoundingClientRect();
                    const intro=rect('.meshcore-intro'), note=rect('.meshcore-intro .cfg-note'), status=rect('#meshcore-status');
                    const body=document.querySelector('.meshcore-pairing .pw-disclosure-body');
                    const pin=rect('#meshcore-pin'), actions=body.querySelector('.cfg-actions-row').getBoundingClientRect();
                    const help=body.querySelector('.cfg-note').getBoundingClientRect(), field=body.querySelector('.field-group').getBoundingClientRect();
                    return status.top-note.bottom>=15 && document.querySelector('.meshcore-intro').nextElementSibling.getBoundingClientRect().top-intro.bottom>=15 && field.top-help.bottom>=15 && actions.top-pin.bottom>=15;
                }`).Bool() {
					t.Fatalf("MeshCore spacing collapsed at %d / %s / %s", width, theme, density)
				}
			}
		}
	}
	if page.MustEval(`() => meshRequests.some(r=>r.url.includes('/send') || r.body.includes('pin'))`).Bool() {
		t.Fatal("setup sent radio text or PIN")
	}
	page.MustEval(`() => document.querySelector('.section-header').scrollIntoView({block:'start'})`)
	capture("meshcore-config.png")
}
