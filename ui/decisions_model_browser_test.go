package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

func TestDecisionsModelAvailabilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{390, 1440} {
			t.Run(fmt.Sprintf("%s-%d", theme, width), func(t *testing.T) {
				page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(60 * time.Second)
				defer page.MustClose()
				page.MustSetViewport(width, 900, 1, width == 390)
				waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
				page.MustEval(`theme=>{
		 document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;
		 configData.decisions={enabled:true,backend:'provider',provider:'fixture-decisions',model:'fixture/latest'};
		 providersCache.push({id:'fixture-decisions',name:'Fixture',type:'openrouter',base_url:'https://openrouter.ai/api/v1',has_api_key:true});
		 window.modelCalls=0;window.inferenceCalls=0;
		 window.modelStatus={...configData.decisions,configured:true,ready:false,state:'configured',model_availability:{state:'unknown',code:'model_unverified'}};
		 const previous=window.fetch;window.fetch=async(url,options)=>{
		  if(url==='/api/decisions/status')return new Response(JSON.stringify(modelStatus));
		  if(String(url).startsWith('/api/decisions/models')){modelCalls++;return new Response(JSON.stringify({models:[
		   {id:'fixture/latest',alias_target:'fixture/version-20261017',canonical_slug:'fixture/version-20261017',context_length:8192,input_modalities:['text','image'],supports_images:true,usable:true,media_limits:{max_images:4,max_image_bytes:307200}},
		   {id:'fixture/broken',context_length:0,usable:false,unavailable_code:'model_context_unverified'}]}));}
		  if(url==='/api/decisions/test'){inferenceCalls++;return new Response('{}');}
		  return previous(url,options);
		 };
		}`, theme)
				page.MustEval(`()=>{document.querySelector('.sidebar-group:has([data-section="decisions"]) .sidebar-group-header').click();document.querySelector('[data-section="decisions"]').click();}`)
				waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('ungeprüft')`)
				if page.MustEval(`()=>modelCalls+inferenceCalls`).Int() != 0 {
					t.Fatal("passive model view made a remote call")
				}
				page.MustEval(`()=>document.querySelector('[data-decisions-load-models]').click()`)
				waitForJSBool(t, page, `()=>!document.querySelector('[data-decisions-pin-model]')?.hidden`)
				if !page.MustEval(`()=>document.querySelector('[value="fixture/broken"]').disabled && document.querySelector('[data-decisions-model-details]').textContent.includes('Details') && document.querySelector('[data-path="decisions.model"]').value==='fixture/latest'`).Bool() {
					t.Fatal("model restriction, loss warning or alias preservation missing")
				}
				page.MustEval(`()=>document.querySelector('[data-decisions-pin-model]').focus()`)
				page.Keyboard.MustType(input.Enter)
				waitForJSBool(t, page, `()=>document.querySelector('[data-path="decisions.model"]').value==='fixture/version-20261017'`)
				if page.MustEval(`()=>inferenceCalls`).Int() != 0 {
					t.Fatal("pinning a model invoked inference")
				}
				page.MustEval(`()=>{modelStatus={...modelStatus,model:'fixture/version-20261017',model_availability:{state:'blocked',code:'model_context_unverified'}};document.dispatchEvent(new Event('aurago:config-saved'));}`)
				waitForJSBool(t, page, `()=>document.querySelector('[data-decisions-status]')?.textContent.includes('Kontextgrenze nicht verifiziert')`)
				if !page.MustEval(`()=>document.querySelector('[data-decisions-action="test-provider"]').disabled`).Bool() {
					t.Fatal("blocked model test remained enabled")
				}
				if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
					t.Fatal("model details overflow")
				}
			})
		}
	}
}
