package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRustFSS3SettingsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#s3").Timeout(45 * time.Second)
	defer page.MustClose()
	waitForJSBool(t, page, `()=>typeof renderS3Section==='function'&&!!window.AuraConfigState`)
	page.MustEval(`async()=>{
        window.__s3RealFetch=window.fetch.bind(window);window.__s3Mutations=[];window.__s3Reads=0;window.__s3Tests=0;
        window.__s3Status={enabled:true,installed:false,provisioned:false,running:false,ready:false,agent_access:false,state:'not_installed',access_state:'disabled',bucket:'aurago',endpoint:'',data_volume:'aurago-rustfs-data',credential_volume:'aurago-rustfs-credentials',generation:'fixture-1'};
        window.__s3Operation=null;
        window.fetch=async(url,options={})=>{
            const path=String(url);const reply=(body,status=200)=>({ok:status<400,status,json:async()=>structuredClone(body)});
            if(path==='/api/rustfs/status'){window.__s3Reads++;return reply(window.__s3Status)}
            if(path.startsWith('/api/rustfs/operations/'))return window.__s3Operation?reply(window.__s3Operation):reply({code:'operation_not_found'},404);
            if(path==='/api/rustfs/preflight')return reply({supported:true,mutation_allowed:true,data_retained:false,placement:'container'});
            if(path==='/api/s3/test'){window.__s3Tests++;return reply({status:'ok'})}
            if(path.startsWith('/api/rustfs/')){
                const action=path.split('/').pop();window.__s3Mutations.push(action);
                window.__s3Operation={id:'fixture-'+action,action,state:'running',phase:action==='install'?'image_pull':'stopping'};
                if(action==='install')Object.assign(window.__s3Status,{installed:true,provisioned:true,running:true,ready:true,state:'ready',endpoint:'http://aurago-rustfs:9000',operation_id:window.__s3Operation.id});
                return reply({status:'accepted',operation:window.__s3Operation},202);
            }
            return window.__s3RealFetch(url,options);
        };
        configData.docker={enabled:true,readonly:false};
        configData.s3={enabled:true,provider:'external',readonly:true,endpoint:'https://external.fixture',bucket:'external-bucket',access_key:'stored-key',secret_key:'stored-secret'};
        configData.rustfs={enabled:true,bucket:'aurago',agent_access:false,permissions:{allow_read:false,allow_write:false,allow_delete:false,allowed_locations:[]}};
        AuraConfigState.init(configData);renderS3Section({label:'S3',desc:'Fixture'});
    }`)
	waitForJSBool(t, page, `()=>s3ConfigView?.status?.state==='not_installed'`)
	if !page.MustEval(`()=>document.querySelector('#s3-provider option[value="rustfs"]').disabled&&!document.querySelector('[data-path="rustfs.agent_access"]').classList.contains('on')&&!AuraConfigState.isDirty()`).Bool() {
		t.Fatal("unprovisioned provider was selectable or rendering granted access/changed the draft")
	}
	page.MustElement("#s3-rustfs-preflight").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#s3-rustfs-status').textContent.includes('erfüllt')`)
	if page.MustEval(`()=>window.__s3Mutations.length`).Int() != 0 {
		t.Fatal("preflight changed the runtime")
	}
	page.MustElement("#s3-rustfs-install").MustClick()
	waitForJSBool(t, page, `()=>window.__s3Mutations.length===1&&document.querySelector('#s3-rustfs-operation').textContent.includes('Image herunterladen')&&document.querySelector('#s3-rustfs-install').disabled&&!document.querySelector('#s3-rustfs-stop').disabled`)
	if !page.MustEval(`()=>document.querySelector('#s3-rustfs-install').disabled&&!document.querySelector('#s3-rustfs-stop').disabled`).Bool() {
		t.Fatal("active setup allowed duplicate installation or blocked containment stop")
	}
	page.MustEval(`async()=>{window.__s3Operation.state='succeeded';window.__s3Operation.phase='complete';window.__s3Status.operation_id='';await s3RefreshStatus(s3ConfigView);window.__s3Operation=null;await s3RefreshStatus(s3ConfigView)}`)
	if !page.MustEval(`()=>s3ConfigView.status.provisioned&&!s3ConfigView.operationID&&document.querySelector('#s3-rustfs-operation').textContent.includes('unbekannt')`).Bool() {
		t.Fatal("an expired receipt hid observed state or kept actions permanently blocked")
	}
	page.MustElement("#s3-provider").MustSelect("Verwaltetes RustFS")
	if !page.MustEval(`()=>AuraConfigState.get('s3.provider')==='rustfs'&&AuraConfigState.get('s3.endpoint')==='https://external.fixture'&&AuraConfigState.get('s3.secret_key')==='stored-secret'&&document.querySelector('#s3-rustfs-install').disabled`).Bool() {
		t.Fatal("provider draft lost external settings or allowed saved actions")
	}
	page.MustEval(`async()=>{Object.assign(configData,AuraConfigState.snapshot().draft);configData.s3.access_key='';configData.s3.secret_key='';AuraConfigState.init(configData);await new Promise(resolve=>setTimeout(resolve,0))}`)
	waitForJSBool(t, page, `()=>s3ConfigView?.status?.provisioned===true`)
	page.MustElement("#integration-s3-test-btn").MustClick()
	waitForJSBool(t, page, `()=>window.__s3Tests===1`)
	page.MustElement("#s3-location-add").MustClick()
	page.MustElement("#s3-location-prefixes-0").MustInput("agent/\n Agent%2F")
	if !page.MustEval(`()=>JSON.stringify(AuraConfigState.buildPatch().rustfs.permissions.allowed_locations)==='[{"bucket":"aurago","prefixes":["agent/"," Agent%2F"]}]'`).Bool() {
		t.Fatal("literal prefix rules were normalized or not saved as an atomic list")
	}
	page.MustElement(`[data-s3-field="whole"]`).MustClick()
	if !page.MustEval(`()=>JSON.stringify(AuraConfigState.get('rustfs.permissions.allowed_locations')[0].prefixes)==='[""]'`).Bool() {
		t.Fatal("whole-bucket access was not explicit")
	}
	page.MustEval(`async()=>{Object.assign(configData,AuraConfigState.snapshot().draft);AuraConfigState.init(configData);await new Promise(resolve=>setTimeout(resolve,0))}`)
	page.MustElement(`[data-s3-remove="0"]`).MustClick()
	if !page.MustEval(`()=>JSON.stringify(AuraConfigState.buildPatch().rustfs.permissions.allowed_locations)==='[]'`).Bool() {
		t.Fatal("removing the final rule did not preserve an explicit empty list")
	}
	page.MustEval(`async()=>{Object.assign(configData,AuraConfigState.snapshot().draft);AuraConfigState.init(configData);configData.docker.readonly=true;AuraConfigState.init(configData);await new Promise(resolve=>setTimeout(resolve,0))}`)
	waitForJSBool(t, page, `()=>s3ConfigView?.status?.provisioned===true`)
	if !page.MustEval(`()=>document.querySelector('#s3-rustfs-start').disabled&&document.querySelector('#s3-rustfs-recreate').disabled&&!document.querySelector('#s3-rustfs-preflight').disabled&&!document.querySelector('#s3-rustfs-refresh').disabled`).Bool() {
		t.Fatal("Docker readonly did not separate observation from lifecycle mutations")
	}
	page.MustEval(`async()=>{configData.docker.readonly=false;AuraConfigState.init(configData);window.__s3Status.reason='docker_unavailable';window.__s3Status.ready=false;await s3RefreshStatus(s3ConfigView)}`)
	if !page.MustEval(`()=>document.querySelector('#s3-rustfs-recreate').disabled&&document.querySelector('#s3-rustfs-status').textContent.includes('Docker nicht verfügbar')&&!document.querySelector('#s3-rustfs-preflight').disabled`).Bool() {
		t.Fatal("observed Docker failure left lifecycle enabled or displayed stale readiness")
	}
	page.MustEval(`async()=>{delete window.__s3Status.reason;window.__s3Status.ready=true;configData.docker.readonly=true;AuraConfigState.init(configData);await s3RefreshStatus(s3ConfigView)}`)
	if !page.MustEval(`()=>['s3-provider','s3-rustfs-bucket','s3-rustfs-endpoint'].every(id=>document.getElementById(id).getAttribute('aria-label'))&&document.querySelector('[data-path="rustfs.agent_access"]').getAttribute('role')==='switch'&&!!document.querySelector('[data-path="rustfs.agent_access"]').getAttribute('aria-label')`).Bool() {
		t.Fatal("managed S3 controls are missing accessible labels")
	}
	for _, width := range []int{1440, 390} {
		page.MustSetViewport(width, 844, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme}`, theme)
			if !page.MustEval(`()=>[...document.querySelectorAll('[id^="s3-rustfs-"]')].filter(el=>el.tagName==='BUTTON').every(el=>el.scrollWidth<=el.clientWidth+1)`).Bool() {
				t.Fatalf("RustFS buttons clip their labels at width %d in %s: %s", width, theme, page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('[id^="s3-rustfs-"]')].filter(el=>el.tagName==='BUTTON').map(el=>({id:el.id,width:el.clientWidth,scroll:el.scrollWidth,style:getComputedStyle(el).cssText,parent:el.parentElement.className,display:getComputedStyle(el.parentElement).display,wrap:getComputedStyle(el.parentElement).flexWrap,flex:getComputedStyle(el).flex,widthRule:getComputedStyle(el).width})))`).String())
			}
			if !page.MustEval(`()=>{const root=document.getElementById('content');return root.scrollWidth<=root.clientWidth+1}`).Bool() {
				t.Fatalf("S3 settings overflow at width %d in %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("s3-%s-%d.png", theme, width)), page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	page.MustEval(`()=>{
        window.__s3OldStatus=document.querySelector('#s3-rustfs-status');
        window.fetch=(url,options)=>String(url)==='/api/rustfs/status'?new Promise(resolve=>{window.__s3Pending=resolve;window.__s3Signal=options.signal}):window.__s3RealFetch(url,options);
        s3RefreshStatus(s3ConfigView);selectSection('overview');
    }`)
	waitForJSBool(t, page, `()=>window.__s3Signal.aborted&&!window.__s3OldStatus.isConnected`)
	page.MustEval(`async()=>{const before=window.__s3OldStatus.textContent;window.__s3Pending({ok:true,status:200,json:async()=>({state:'ready',provisioned:true})});await new Promise(resolve=>setTimeout(resolve,0));if(window.__s3OldStatus.textContent!==before)throw Error('detached S3 status was repainted')}`)
}
