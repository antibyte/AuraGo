package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLMRouterLocaleKeys(t *testing.T) {
	var reference map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/common/en.json"), &reference); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var translated map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/common/"+locale+".json"), &translated); err != nil {
			t.Fatal(err)
		}
		for key := range reference {
			if strings.HasPrefix(key, "common.llm_router.") && strings.TrimSpace(translated[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
	}
}

func TestLLMRouterConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	// The config fixture supplies its own translation/runtime globals. Load the
	// production router helpers without redeclaring those unrelated shared globals.
	shared := normalizeAssetText(mustReadUIFile(t, "shared.js"))
	start := strings.Index(shared, "// Content-free router metadata")
	if start < 0 {
		t.Fatal("router helpers missing")
	}
	end := strings.Index(shared[start:], "if (!window._auragoSharedInitialized)")
	if end < 0 {
		t.Fatal("shared initialization boundary missing")
	}
	if err := page.AddScriptTag("", shared[start:start+end]); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{
        window.routerCalls=[];
        const previousFetch=window.fetch;
        window.fetch=async(url,opts)=>{
            if(url==='/api/llm-router/status') return new Response(JSON.stringify({enabled:true,helper_available:true}));
            if(url==='/api/models/catalog') return new Response(JSON.stringify({models:[
                {provider:'openai',id:'gpt-4o-mini'},
                {provider:'agnes',id:'agnes-2.5-flash'},
                {provider:'agnes',id:'agnes-image-2.1-flash'},
                {provider:'agnes',id:'agnes-video-v2.0'}
            ]}));
            if(url==='/api/llm-router/preview') {
                const body=JSON.parse(opts.body); routerCalls.push(body);
                return new Response(JSON.stringify({enabled:true,decision:{turn_id:'preview',area:body.helper?'coding':'',source:body.helper?'helper':'default',reason:'selected',helper_useful:!body.helper,provider:'main',model:'gpt-4o'}}));
            }
            return previousFetch(url,opts);
        };
        providersCache=[
            {id:'main',name:'Main',type:'openai',model:'gpt-4o'},
            {id:'code',name:'Code',type:'openai',model:'gpt-4o-mini'},
            {id:'agnesai',name:'Agnes AI',type:'agnes',model:'agnes-3.0-flash'},
            {id:'agnes-image',name:'Agnes Image',type:'agnes',model:'agnes-image-2.1-flash'},
            {id:'agnes-video',name:'Agnes Video',type:'agnes',model:'agnes-video-v2.0'}
        ];
        configData.llm_router.areas.coding={provider:'code',model:'gpt-4o-mini'};
        AuraConfigState.init(configData);
        await selectSection('llm_router',{scrollBehavior:'auto'});
    }`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('[data-router-settings] fieldset').length===9`)
	if page.MustEval(`()=>AuraConfigState.isDirty()`).Bool() {
		t.Fatal("opening router changed the draft")
	}
	// Agnes chat is selectable even when the saved model is newer than the catalog.
	waitForJSBool(t, page, `()=>!!document.querySelector('[data-router-settings]')._routerCatalog`)
	if !page.MustEval(`()=>[...document.querySelectorAll('[data-path$=".provider"]')].every(select=>{
        const ids=[...select.options].map(option=>option.value);
        return ids.includes('agnesai') && !ids.includes('agnes-image') && !ids.includes('agnes-video');
    })`).Bool() {
		t.Fatal("Agnes chat missing or Agnes media present in the provider choices")
	}
	page.MustEval(`()=>{const p=document.querySelector('[data-path="llm_router.areas.coding.provider"]');p.value='agnesai';p.dispatchEvent(new Event('change',{bubbles:true}));}`)
	if !page.MustEval(`()=>{const model=document.querySelector('[data-path="llm_router.areas.coding.model"]');const ids=[...model.options].map(o=>o.value);
        return model.value==='' && ids.includes('agnes-3.0-flash') && ids.includes('agnes-2.5-flash') && !ids.includes('agnes-image-2.1-flash') && !ids.includes('agnes-video-v2.0');
    }`).Bool() {
		t.Fatal("Agnes chat model choices lost the configured default or included media")
	}
	page.MustEval(`()=>{const m=document.querySelector('[data-path="llm_router.areas.coding.model"]');m.value='agnes-3.0-flash';m.dispatchEvent(new Event('change',{bubbles:true}));}`)
	if !page.MustEval(`async()=>{await saveConfig();const saved=await (await fetch('/api/config')).json();return !AuraConfigState.isDirty() && saved.llm_router.areas.coding.provider==='agnesai' && saved.llm_router.areas.coding.model==='agnes-3.0-flash';}`).Bool() {
		t.Fatal("Agnes router assignment did not persist through Save")
	}
	page.MustEval(`async()=>{await selectSection('llm_router',{scrollBehavior:'auto'});}`)
	if !page.MustEval(`()=>document.querySelector('[data-path="llm_router.areas.coding.provider"]').value==='agnesai' && document.querySelector('[data-path="llm_router.areas.coding.model"]').value==='agnes-3.0-flash'`).Bool() {
		t.Fatal("Agnes assignment was lost after reopening the router")
	}
	page.MustEval(`()=>document.querySelector('[data-path="llm_router.enabled"]').click()`)
	if !page.MustEval(`()=>{const toggle=document.querySelector('[data-path="llm_router.enabled"]');return toggle.getAttribute('aria-checked')==='true' && toggle.nextElementSibling.textContent==='Aktiv' && getComputedStyle(toggle).borderRadius!=='0px';}`).Bool() {
		t.Fatal("router toggle did not update state and visual feedback")
	}
	// Clearing the provider must clear its model in the actual saved patch.
	page.MustEval(`()=>{const p=document.querySelector('[data-path="llm_router.areas.coding.provider"]');p.value='';p.dispatchEvent(new Event('change',{bubbles:true}));AuraConfigState.syncFromDOM();}`)
	if !page.MustEval(`()=>{const p=AuraConfigState.buildPatch();return p.llm_router.areas.coding.provider==='' && p.llm_router.areas.coding.model==='';}`).Bool() {
		t.Fatal("clearing the provider left a stale model")
	}
	page.MustEval(`()=>{const input=document.querySelector('[data-router-settings] textarea');input.value='Mach das passend';input.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-router-settings] .cfg-actions button').click();}`)
	if page.MustEval(`()=>routerCalls.length`).Int() != 0 {
		t.Fatal("preview accepted unsaved settings")
	}
	if !page.MustEval(`async()=>{await saveConfig();const response=await fetch('/api/config');const saved=await response.json();return !AuraConfigState.isDirty() && saved.llm_router.enabled===true && saved.llm_router.areas.coding.provider==='' && saved.llm_router.areas.coding.model==='';}`).Bool() {
		t.Fatal("router clear did not persist through the Save action")
	}
	page.MustEval(`async()=>{await selectSection('llm_router',{scrollBehavior:'auto'});}`)
	page.MustEval(`()=>{const input=document.querySelector('[data-router-settings] textarea');input.value='Mach das passend';input.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-router-settings] .cfg-actions button').click();}`)
	waitForJSBool(t, page, `()=>routerCalls.length===1 && !document.querySelectorAll('[data-router-settings] .cfg-actions button')[1].disabled`)
	if page.MustEval(`()=>routerCalls[0].helper`).Bool() {
		t.Fatal("local preview called helper")
	}
	page.MustEval(`()=>document.querySelectorAll('[data-router-settings] .cfg-actions button')[1].click()`)
	waitForJSBool(t, page, `()=>routerCalls.length===2`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-router-settings] [role="status"]').textContent.includes('Programmierung')`)
	if !page.MustEval(`()=>routerCalls[1].helper`).Bool() {
		t.Fatal("explicit helper preview missing")
	}
	for _, width := range []int{390, 768, 1440} {
		for _, theme := range []string{"dark", "light"} {
			t.Run(fmt.Sprintf("%d/%s", width, theme), func(t *testing.T) {
				page.MustSetViewport(width, 900, 1, width == 390)
				page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;}`, theme)
				page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`)
				if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
					t.Fatal("router settings overflow")
				}
				if dir := os.Getenv("AURAGO_ROUTER_SCREENSHOTS"); dir != "" {
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("router-%d-%s.png", width, theme)))
				}
			})
		}
	}
	// Provider output must stay text, and a repeated event updates one badge.
	if !page.MustEval(`()=>{const c=document.createElement('div');const d={turn_id:'1',area:'coding',source:'rules',reason:'selected',model:'<img src=x onerror=alert(1)>',provider:'test'};AuraLLMRouteBadge(c,d);AuraLLMRouteBadge(c,{...d,actual_model:'fallback'});return c.children.length===1 && !c.querySelector('img') && c.textContent.includes('fallback');}`).Bool() {
		t.Fatal("unsafe or duplicate routing badge")
	}
}
