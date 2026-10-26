package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestPrintGuardLocalesAndLocalAssets(t *testing.T) {
	var reference map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &reference); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for key := range reference {
		if strings.HasPrefix(key, "desktop.store.pg_") {
			keys = append(keys, key)
		}
	}
	if len(keys) != 37 {
		t.Fatalf("PrintGuard locale count: %d", len(keys))
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &values); err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if values[key] == "" || (locale != "en" && len(reference[key]) > 80 && values[key] == reference[key]) {
				t.Fatalf("%s missing native translation for %s", locale, key)
			}
		}
	}
	for _, path := range []string{"img/desktop/store/printguard.png", "img/desktop/store/printguard.LICENSE.txt", "img/desktop/store/printguard.NOTICE.txt"} {
		if body, err := Content.ReadFile(path); err != nil || len(body) == 0 {
			t.Fatalf("missing packaged PrintGuard asset %s: %v", path, err)
		}
	}
}

// Read-only browser smoke against the isolated, reviewed upstream image.
func TestPrintGuardOriginalUIBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := os.Getenv("AURAGO_PRINTGUARD_LIVE_URL")
	if origin == "" {
		t.Skip("requires an isolated PrintGuard container")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil {
		t.Fatal("live browser fixture must use loopback HTTP")
	}
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.Context(ctx).MustPage()
	defer page.Close()
	page.MustEvalOnNewDocument(`window.pgStateMessages=0;const OriginalSocket=WebSocket;window.WebSocket=class extends OriginalSocket{constructor(...args){super(...args);this.addEventListener('message',e=>{try{if(JSON.parse(e.data).event==='state')pgStateMessages++}catch{}})}}`)
	page.MustNavigate(origin).MustWaitLoad()
	page.MustWait(`()=>document.body.innerText.includes('PrintGuard')&&pgStateMessages>0`)
	page.MustEval(`()=>[...document.querySelectorAll('button')].find(b=>/^skip$/i.test(b.textContent.trim()))?.click()`)
	for _, width := range []int{400, 1280} {
		page.MustSetViewport(width, 850, 1, false)
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+2`).Bool() {
			t.Fatal("upstream UI overflows")
		}
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("printguard-original-%d.png", width)))
		}
	}
}

func TestPrintGuardSetupBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-software-store.css"></head><body class="desktop-body" data-theme="standard"><div id="host"></div><script src="/js/desktop/apps/software-store.js"></script><script>
window.requests=[];window.holdConfig=false;window.resolveConfig=null;window.holdTest=false;window.resolveTest=null;window.missingGateway=false;window.installFixture=false;window.missingNetwork=false;window.nativeFixture=false;
const config=()=>({installed:!installFixture,acceleration:'intel',active_acceleration:'intel',docker_network:missingNetwork?'':'aura-net',agent_mode:'read',token_configured:true,agent_verified:true,architecture:'amd64',nvidia_available:true,containerized:!nativeFixture,networks:['aura-net','aura-other'],compute:'intel cpu',cpu_fallback:true,prerequisites:missingGateway?['printguard_gateway']:missingNetwork?['printguard_network']:[]});
const api=(url,opts={})=>{
 if(url==='/api/desktop/store/catalog')return Promise.resolve({catalog:[{id:'printguard',name:'PrintGuard',description:'',icon:'printer'}],installed:installFixture?[]:[{app_id:'printguard',status:'running'}]});
 if(url==='/api/desktop/store/install'&&opts.method==='POST'){requests.push(JSON.parse(opts.body));return Promise.resolve({operation:{id:'fixture-op',app_id:'printguard',type:'install',status:'pending'}})}
 if(url.endsWith('/config')&&opts.method==='PUT'){requests.push(JSON.parse(opts.body));return Promise.resolve({operation:{id:'fixture-op',app_id:'printguard',type:'configure',status:'pending'}})}
 if(url.endsWith('/config'))return holdConfig?new Promise(resolve=>resolveConfig=resolve):Promise.resolve(config());
 if(url.endsWith('/test')){requests.push({test:true,body:opts.body});return holdTest?new Promise(resolve=>resolveTest=resolve):Promise.resolve({status:'ok'})}
 if(url.includes('/operations/'))return Promise.resolve({operation:{id:'fixture-op',app_id:'printguard',type:'configure',status:'succeeded'}});
 return Promise.resolve({});
};
window.mountStore=async locale=>{const lang=await(await fetch('/lang/desktop/'+locale+'.json')).json();window.pgLocale=lang;SoftwareStoreApp.render(document.getElementById('host'),'pg',{api,t:key=>lang[key]||key,esc:value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))});};
</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture").Timeout(90 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustWait(`()=>typeof mountStore==='function'`)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{400, 1280} {
			page.MustSetViewport(width, 850, 1, false)
			page.MustEval(`theme=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light'}`, theme)
			result := page.MustEval(`async()=>{
 const until=async f=>{for(let i=0;i<120;i++){if(f())return;await new Promise(r=>setTimeout(r,25))}throw Error('PrintGuard condition timed out')};
 await mountStore('de');await until(()=>document.querySelector('[data-action=configure-pg]'));
 document.querySelector('[data-action=configure-pg]').click();await until(()=>document.querySelector('.vd-pg-config'));
 const form=document.querySelector('.vd-pg-config'),token=form.elements.api_token;
 if(token.value||token.getAttribute('value'))throw Error('token prefilled');
 if(form.elements.docker_network.value!=='aura-net'||!form.textContent.includes(pgLocale['desktop.store.pg_cpu_fallback']))throw Error('network or CPU fallback missing');
 const rect=form.getBoundingClientRect();if(rect.width>innerWidth||rect.height>innerHeight||form.scrollWidth>form.clientWidth+2)throw Error('dialog overflows');
 const last=form.querySelector('[type=submit]');last.focus();last.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',bubbles:true,cancelable:true}));
 if(document.activeElement!==form.querySelector('select:not([disabled])'))throw Error('focus not trapped');
 form.querySelector('[data-action=test]').click();await until(()=>form.querySelector('[data-pg-status]').textContent===pgLocale['desktop.store.pg_test_ok']);
 if(requests.at(-1).body)throw Error('test sent a draft or mutation');
 holdTest=true;form.querySelector('[data-action=test]').click();await until(()=>resolveTest);
 token.value='synthetic-browser-token-692418';token.dispatchEvent(new Event('input',{bubbles:true}));
 resolveTest({status:'ok'});holdTest=false;resolveTest=null;await new Promise(r=>setTimeout(r,20));
 if(form.querySelector('[data-pg-status]').textContent)throw Error('stale test verified an edited draft');
 if(!form.querySelector('[data-action=test]').disabled)throw Error('test accepts unsaved draft');
 form.elements.agent_mode.value='control';form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
 await until(()=>!document.querySelector('.vd-pg-config'));
 if(token.value||token.isConnected||requests.at(-1).api_token!=='synthetic-browser-token-692418'||requests.at(-1).agent_mode!=='control')throw Error('save/disposal failed');
 await until(()=>document.querySelector('[data-action=configure-pg]:not([disabled])'));
 document.querySelector('[data-action=configure-pg]').click();await until(()=>document.querySelector('.vd-pg-config'));
 const retained=document.querySelector('[name=api_token]');retained.value='synthetic-disposal-token';SoftwareStoreApp.dispose('pg');
 if(retained.value||retained.isConnected||document.querySelector('.vd-modal-backdrop'))throw Error('secret survived app disposal');
 await mountStore('ja');await until(()=>document.querySelector('[data-action=configure-pg]'));
 holdConfig=true;document.querySelector('[data-action=configure-pg]').click();await until(()=>resolveConfig);
 SoftwareStoreApp.dispose('pg');resolveConfig(config());holdConfig=false;resolveConfig=null;await new Promise(r=>setTimeout(r,50));
 if(document.querySelector('.vd-modal-backdrop'))throw Error('late response reopened dialog');
 installFixture=true;missingGateway=true;await mountStore('en');await until(()=>document.querySelector('[data-action=install]'));
 document.querySelector('[data-action=install]').click();await until(()=>document.querySelector('.vd-pg-config'));
 let installForm=document.querySelector('.vd-pg-config');const beforeInstall=requests.length;
 if(!installForm.querySelector('[type=submit]').disabled)throw Error('install enabled without gateway prerequisites');
 const setupLink=installForm.querySelector('a[href="/config#server"]');
 if(!setupLink||setupLink.target!=='_blank'||!setupLink.rel.includes('noopener'))throw Error('missing safe server settings link');
 installForm.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
 if(requests.length!==beforeInstall)throw Error('blocked install sent a request');
 installForm.elements.docker_network.value='aura-other';installForm.elements.docker_network.dispatchEvent(new Event('change',{bubbles:true}));
 if(!installForm.querySelector('[type=submit]').disabled||!installForm.querySelector('[data-pg-status]').textContent.includes(pgLocale['desktop.store.pg_error_gateway']))throw Error('draft cleared gateway blocker');
 missingGateway=false;installForm.querySelector('[data-action=recheck]').click();
 await until(()=>document.querySelector('.vd-pg-config')&&document.querySelector('.vd-pg-config')!==installForm);installForm=document.querySelector('.vd-pg-config');
 if(installForm.querySelector('[type=submit]').disabled)throw Error('install stayed blocked after setup recheck');
 installForm.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await until(()=>!document.querySelector('.vd-pg-config'));
 if(requests.length!==beforeInstall+1||requests.at(-1).app_id!=='printguard'||requests.at(-1).bind_mode!=='local'||requests.at(-1).docker_network!=='aura-net')throw Error('ready install did not start exactly once');
 SoftwareStoreApp.dispose('pg');missingNetwork=true;await mountStore('de');await until(()=>document.querySelector('[data-action=install]'));
 document.querySelector('[data-action=install]').click();await until(()=>document.querySelector('.vd-pg-config'));
 installForm=document.querySelector('.vd-pg-config');
 if(!installForm.querySelector('[type=submit]').disabled)throw Error('install enabled without network choice');
 installForm.elements.docker_network.value='aura-other';installForm.elements.docker_network.dispatchEvent(new Event('change',{bubbles:true}));
 if(installForm.querySelector('[type=submit]').disabled||installForm.querySelector('[data-pg-status]').textContent)throw Error('valid network choice did not clear network blocker');
 SoftwareStoreApp.dispose('pg');missingNetwork=false;nativeFixture=true;missingGateway=true;
 await mountStore('en');await until(()=>document.querySelector('[data-action=install]'));
 document.querySelector('[data-action=install]').click();await until(()=>document.querySelector('.vd-pg-config'));
 installForm=document.querySelector('.vd-pg-config');
 if(installForm.elements.docker_network||!installForm.querySelector('[type=submit]').disabled)throw Error('native gateway blocker missing');
 const nativeLink=installForm.querySelector('a[href]');nativeLink.focus();nativeLink.dispatchEvent(new KeyboardEvent('keydown',{key:'Tab',shiftKey:true,bubbles:true,cancelable:true}));
 if(document.activeElement!==installForm.querySelector('[data-action=recheck]'))throw Error('native install focus not trapped');
 const nativeRect=installForm.getBoundingClientRect();if(nativeRect.width>innerWidth||installForm.scrollWidth>installForm.clientWidth+2)throw Error('blocked install overflows');
 SoftwareStoreApp.dispose('pg');installFixture=false;nativeFixture=false;missingGateway=false;
 return true;
}`)
			if !result.Bool() {
				t.Fatal("PrintGuard browser flow failed")
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				page.MustEval(`async()=>{await mountStore('de')}`)
				page.MustWait(`()=>!!document.querySelector('[data-action=configure-pg]')`)
				page.MustEval(`()=>document.querySelector('[data-action=configure-pg]').click()`)
				page.MustWait(`()=>!!document.querySelector('.vd-pg-config')`)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("printguard-%s-%d.png", theme, width)))
				page.MustEval(`()=>SoftwareStoreApp.dispose('pg')`)
				page.MustEval(`async()=>{installFixture=true;nativeFixture=true;missingGateway=true;await mountStore('en')}`)
				page.MustWait(`()=>!!document.querySelector('[data-action=install]')`)
				page.MustEval(`()=>document.querySelector('[data-action=install]').click()`)
				page.MustWait(`()=>!!document.querySelector('.vd-pg-config')`)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("printguard-install-blocked-%s-%d.png", theme, width)))
				page.MustEval(`()=>{SoftwareStoreApp.dispose('pg');installFixture=false;nativeFixture=false;missingGateway=false}`)
			}
		}
	}
}
