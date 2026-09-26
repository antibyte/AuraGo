package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTregTranslations(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var english map[string]string
	for _, locale := range append([]string{"en"}, locales...) {
		b, err := os.ReadFile(filepath.Join("lang", "config", "treg", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var translations map[string]string
		if err := json.Unmarshal(b, &translations); err != nil {
			t.Fatal(err)
		}
		if locale == "en" {
			english = translations
			continue
		}
		for key, value := range english {
			if translations[key] == "" || translations[key] == value {
				t.Errorf("%s missing translation for %s", locale, key)
			}
		}
	}
}

func TestTregConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer func() {
		page.MustEval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`() => {
        window.tregFetch=window.fetch; window.tregRequests=[]; window.tregFail=false;
        window.fetch=async (url,options={})=>{
            const path=String(url);
            if(path==='/api/vault/secrets' && options.method==='POST'){window.tregTokenStored=true;return new Response(JSON.stringify({status:'ok'}));}
            if(path==='/api/config' && (!options.method||options.method==='GET')){const response=await window.tregFetch(url,options);const data=await response.json();if(window.tregTokenStored)data.treg.token='••••••••';return new Response(JSON.stringify(data),{headers:{'Content-Type':'application/json'}});}
            if (!path.startsWith('/api/treg/')) return window.tregFetch(url,options);
            window.tregRequests.push({path,method:options.method||'GET'});
            if(window.tregFail)return new Response(JSON.stringify({message:'fixture failure'}),{status:503});
            if(path.endsWith('/status'))return new Response(JSON.stringify({status:'disabled',key_present:true}));
            if(path.endsWith('/test-connection'))return new Response(JSON.stringify({status:'ok',data:{balance_micro:2000000}}));
            const endpoint={id:'fixture.catalog',name:'<img src=x onerror=alert(1)>',summary:'Fixture',method:'POST',path:'/v1/fixture',input:{bodyType:'json'},cost:{micro:1000}};
            return new Response(JSON.stringify({status:'ok',data:path.includes('/catalog?')?{results:[endpoint]}:{endpoint,provider:{name:'Fixture'}}}));
        };
    }`)
	page.MustEval(`async()=>{await selectSection('treg');resetDirtySnapshot();}`)
	page.MustElement("#treg-token").MustInput("fixture-organization-token")
	page.MustElement("#treg-save-token").MustClick()
	waitForJSBool(t, page, `()=>window.tregTokenStored && document.getElementById('treg-token').value===''`)
	page.MustElement("#treg-query").MustInput("media")
	page.MustElement("#treg-search").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('#treg-results button').length===1`)
	page.MustElement("#treg-results button").MustClick()
	waitForJSBool(t, page, `()=>!!document.getElementById('treg-permission')`)
	if !page.MustEval(`()=>document.getElementById('treg-permission').value===''&&document.getElementById('treg-approve').disabled&&!document.querySelector('#treg-details img')`).Bool() {
		t.Fatal("implicit grant or catalog HTML injection")
	}
	page.MustEval(`()=>{const select=document.getElementById('treg-permission');select.value='create';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustElement("#treg-approve").MustClick()
	if !page.MustEval(`()=>AuraConfigState.isDirty()&&AuraConfigState.get('treg.allowed_endpoints').length===1`).Bool() {
		t.Fatal("grant bypassed shared draft")
	}
	if !page.MustEval(`()=>document.getElementById('treg-test-btn').getAttribute('aria-disabled')==='true'`).Bool() {
		t.Fatal("connection test enabled on an unsaved draft")
	}
	page.MustEval(`async()=>{await selectSection('server');await selectSection('treg');}`)
	if !page.MustEval(`()=>JSON.parse(document.querySelector('[data-path="treg.allowed_endpoints"]').value).length===1`).Bool() {
		t.Fatal("draft lost on section navigation")
	}
	page.MustEval(`()=>{const cost=document.getElementById('treg-cost');cost.value='0';cost.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustElement(`[data-path="treg.enabled"]`).MustClick()
	if !page.MustEval(`async()=>{await saveConfig();const saved=await(await tregFetch('/api/config')).json();return saved.treg.enabled===true&&saved.treg.max_call_cost_micro===0&&saved.treg.allowed_endpoints[0].operation==='create';}`).Bool() {
		t.Fatal("saved settings differ from draft")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 900, 1, width == 390)
		for _, theme := range []string{"light", "dark"} {
			page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.querySelectorAll('#content details').forEach(item=>item.open=true);}`, theme)
			if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
				t.Errorf("horizontal overflow at %d %s", width, theme)
			}
		}
	}
	page.MustEval(`()=>{const input=document.getElementById('treg-cost');input.focus();}`)
	if !page.MustEval(`()=>document.activeElement.id==='treg-cost'`).Bool() {
		t.Fatal("keyboard focus unavailable")
	}
	page.MustEval(`async()=>{await selectSection('server');await selectSection('treg');}`)
	page.MustElement("#treg-grants button").MustClick()
	if !page.MustEval(`async()=>{await saveConfig();const saved=await(await tregFetch('/api/config')).json();return saved.treg.allowed_endpoints.length===0;}`).Bool() {
		t.Fatal("revocation was not saved")
	}
	page.MustEval(`async()=>{await selectSection('treg');}`)
	waitForJSBool(t, page, `()=>document.getElementById('treg-test-btn').getAttribute('aria-disabled')==='false'`)
	page.MustElement("#treg-test-btn").MustClick()
	waitForJSBool(t, page, `()=>document.getElementById('treg-action-status').textContent.includes('2.000000')`)
	if page.MustEval(`()=>tregRequests.some(item=>item.path.includes('/call/'))`).Bool() {
		t.Fatal("connection test executed a paid endpoint")
	}
	page.MustEval(`()=>{window.tregFail=true;}`)
	page.MustElement("#treg-search").MustClick()
	waitForJSBool(t, page, `()=>document.getElementById('treg-search-status').textContent.includes('fixture failure')`)
	if strings.Contains(page.MustElement("#content").MustText(), "config.treg.") {
		t.Fatal("untranslated treg control")
	}
	page.MustReload()
	waitForJSBool(t, page, `()=>!!document.getElementById('treg-cost')`)
	if !page.MustEval(`()=>document.querySelector('[data-path="treg.enabled"]').classList.contains('on') && document.getElementById('treg-cost').value==='0.000000' && JSON.parse(document.querySelector('[data-path="treg.allowed_endpoints"]').value).length===0`).Bool() {
		t.Fatal("settings changed after reload")
	}
}
