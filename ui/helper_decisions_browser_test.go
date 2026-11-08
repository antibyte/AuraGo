package ui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHelperDecisionsLocaleKeys(t *testing.T) {
	keys := []string{"helper_title", "helper_help", "helper_router", "helper_rag", "helper_turn", "helper_pending", "helper_testing", "helper_slow", "helper_failed", "helper_off", "helper_stats"}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/config/"+locale+".json"), &values); err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values["config.decisions."+key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
	}
}

func TestHelperDecisionsConfigBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"dark", "light"} {
		page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(60 * time.Second)
		t.Cleanup(func() { _ = page.Close() })
		waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
		page.MustEval(`theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.documentElement.dataset.desktopTheme=theme==='light'?'fruity':'standard';}`, theme)
		page.MustEval(`()=>{
			window.helperMutations=[];
			const previous=window.fetch;
			window.fetch=async(url,options)=>{
				const path=new URL(typeof url==='string'?url:url.url,location.origin).pathname;
				if(path.startsWith('/api/decisions/') && options?.method && options.method!=='GET')window.helperMutations.push(path);
				if(path==='/api/decisions/status')return new Response(JSON.stringify({enabled:true,ready:true,installed:true,state:'ready',backend:'local',consumers:[],helper_profiles:[
					{id:'router',enabled:true,available:false,code:'helper_testing',samples:2,probe_calls:3,last_response_ms:250},
					{id:'helper_rag',enabled:true,available:true,code:'available',samples:5,probe_calls:6,last_response_ms:450,offloaded_evaluations:7,helper_calls_avoided:1},
					{id:'helper_turn',enabled:true,available:false,code:'helper_slow',samples:0,probe_calls:1,last_response_ms:950}
				]}));
				return previous(url,options);
			};
		}`)
		page.MustElement(`.sidebar-group:has([data-section="decisions"]) .sidebar-group-header`).MustClick()
		page.MustElement(`[data-section="decisions"]`).MustClick()
		page.MustEval(`async()=>{await selectSection('decisions',{scrollBehavior:'auto'});}`)
		waitForJSBool(t, page, `()=>document.querySelector('[data-helper-decision-profile="helper_rag"]')?.textContent.includes('Helper calls avoided 1')`)
		for _, size := range [][2]int{{1440, 900}, {390, 844}} {
			page.MustSetViewport(size[0], size[1], 1, false)
			if !page.MustEval(`()=>{
				const card=document.querySelector('[data-helper-decision-profiles]');
				return card.querySelectorAll('[data-helper-decision-profile]').length===3 && !card.querySelector('[data-path],button,[role="switch"]')
					&& card.textContent.includes('Performance test running') && card.textContent.includes('Too slow; Helper handles the work')
					&& card.scrollWidth<=card.clientWidth+1 && window.helperMutations.length===0;
			}`).Bool() {
				t.Fatal("profile states, layout or passive behavior failed")
			}
		}
		page.MustClose()
	}
}
