package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAIGatewaySavedProviderDiagnosticsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview")
			defer page.MustClose()
			waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
			page.MustEval(`async theme=>{
            document.documentElement.dataset.theme=theme==='fruity'?'light':'dark';
            window.gatewayCalls=[];
            const previousFetch=window.fetch;
            window.fetch=async(url,options={})=>{
                if(String(url).startsWith('/api/ai-gateway/')) {
                    const body=options.body?JSON.parse(options.body):{};
                    gatewayCalls.push({url:String(url),body});
                    if(window.gatewayHold && body.probe_kind==='generation') {
                        window.gatewayHold=false;window.gatewaySignal=options.signal;
                        await new Promise(resolve=>window.releaseGateway=resolve);
                    }
                    return new Response(JSON.stringify({status:options.method==='POST'?'ok':'configured',route_supported:true,live_status:options.method==='POST'?'ok':undefined,code:options.method==='POST'?(body.probe_kind==='generation'?'model_test_ok':'metadata_probe_ok'):undefined}));
                }
                return previousFetch(url,options);
            };
            await selectSection('ai_gateway',{scrollBehavior:'auto'});
            configData.providers=[{id:'main',name:'Main',type:'openai'},{id:'rest',name:'Cloudflare REST',type:'cloudflare-rest'}];
            configData.llm.provider='main';
            configData.ai_gateway={enabled:true,account_id:'fixture-account',gateway_id:'fixture-gateway',token:'••••••••',log_mode:'metadata_only',metadata:{env:'test'}};
            window.AuraConfigState.init(configData);renderAIGatewaySection();resetDirtySnapshot();await aiGatewayCheckStatus();
        }`, theme)
			if !page.MustEval(`()=>document.querySelectorAll('#ai-gw-provider option').length===2 && !document.querySelector('[data-path="ai_gateway.mode"]') && !document.querySelector('[data-path="ai_gateway.max_attempts"]') && gatewayCalls.every(c=>!c.body.probe_kind) && document.getElementById('ai-gw-status-banner').textContent.includes('not tested')`).Bool() {
				t.Fatal("passive route status or provider choices incorrect")
			}
			page.MustEval(`async()=>{document.getElementById('ai-gw-provider').value='rest';_aiGatewayProviderID='rest';await aiGatewayCheckStatus();await aiGatewayTestConnection();}`)
			if !page.MustEval(`()=>gatewayCalls.at(-1).body.provider_id==='rest' && gatewayCalls.at(-1).body.probe_kind==='metadata' && document.getElementById('ai-gw-status-banner').textContent.includes('model not tested')`).Bool() {
				t.Fatal("metadata test implied a model test or used wrong provider")
			}
			page.MustEval(`()=>aiGatewayTestConnection('generation')`)
			if !page.MustEval(`()=>document.getElementById('ai-gw-status-banner').textContent.includes('Model test successful')`).Bool() {
				t.Fatal("model result overwritten by local status")
			}
			page.MustEval(`()=>{window.gatewayHold=true;window.oldGatewayTest=aiGatewayTestConnection('generation');}`)
			waitForJSBool(t, page, `()=>typeof releaseGateway==='function'`)
			page.MustEval(`async()=>{document.getElementById('ai-gw-provider').value='main';_aiGatewayProviderID='main';await aiGatewayCheckStatus();releaseGateway();await oldGatewayTest;}`)
			if !page.MustEval(`()=>gatewaySignal.aborted && document.getElementById('ai-gw-status-banner').textContent.includes('not tested')`).Bool() {
				t.Fatal("late model result overwrote another provider")
			}
			for _, width := range []int{1280, 390} {
				page.MustSetViewport(width, 844, 1, false)
				if !page.MustEval(`()=>document.documentElement.scrollWidth<=window.innerWidth+2`).Bool() {
					t.Fatal("gateway settings overflow narrow viewport")
				}
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "ai-gateway-"+theme+".png"), page.MustScreenshot(), 0600); err != nil {
					t.Fatal(err)
				}
			}
			page.MustEval(`async()=>{window.beforeGatewayCount=gatewayCalls.length;const field=document.querySelector('[data-path="ai_gateway.account_id"]');field.value='changed';field.dispatchEvent(new Event('input',{bubbles:true}));await aiGatewayTestConnection();}`)
			if !page.MustEval(`()=>gatewayCalls.length===beforeGatewayCount+1 && gatewayCalls.at(-1).body.draft.ai_gateway.account_id==='changed' && !document.getElementById('ai-gw-test-btn').disabled`).Bool() {
				t.Fatal("unsaved gateway edits were not sent with the test")
			}
		})
	}
}

func TestAIGatewayMetadataDraftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview")
			defer page.MustClose()
			waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
			page.MustEval(`async theme=>{
                document.documentElement.dataset.theme=theme==='fruity'?'light':'dark';
                await selectSection('ai_gateway',{scrollBehavior:'auto'});
                configData.ai_gateway={enabled:false,metadata:{env:'prod'}};
                window.AuraConfigState.init(configData);
                renderAIGatewaySection();resetDirtySnapshot();
                const previousFetch=window.fetch;
                window.gatewaySaves=0;
                window.fetch=(url,options={})=>{
                    if(url==='/api/config' && options.method==='PUT') gatewaySaves++;
                    return previousFetch(url,options);
                };
                document.querySelectorAll('[data-ai-gateway-metadata-key]')[1].value='unfinished';
                aiGatewayCollectMetadata();window.AuraConfigState.syncFromDOM();
                await saveConfig();
                await selectSection('overview',{scrollBehavior:'auto'});
                await selectSection('ai_gateway',{scrollBehavior:'auto'});
            }`, theme)
			if !page.MustEval(`()=>gatewaySaves===0 && !window.AuraConfigState.validate().valid && document.querySelectorAll('[data-ai-gateway-metadata-key]')[0].value==='env' && document.querySelectorAll('[data-ai-gateway-metadata-value]')[0].value==='prod' && document.querySelectorAll('[data-ai-gateway-metadata-key]')[1].value==='unfinished'`).Bool() {
				t.Fatal("invalid metadata was saved or lost during section navigation")
			}
			page.MustEval(`()=>{
                document.querySelectorAll('[data-ai-gateway-metadata-key]')[1].value='env';
                document.querySelectorAll('[data-ai-gateway-metadata-value]')[1].value='duplicate';
                aiGatewayCollectMetadata();window.AuraConfigState.syncFromDOM();
                renderAIGatewaySection();
            }`)
			if !page.MustEval(`()=>!window.AuraConfigState.validate().valid && [...document.querySelectorAll('[data-ai-gateway-metadata-value]')].some(el=>el.value==='prod') && [...document.querySelectorAll('[data-ai-gateway-metadata-value]')].some(el=>el.value==='duplicate')`).Bool() {
				t.Fatal("duplicate metadata silently replaced a row")
			}
			page.MustEval(`()=>{
                document.querySelector('[data-ai-gateway-metadata-key]').value='renamed';
                aiGatewayCollectMetadata();
                document.querySelector('[data-ai-gateway-metadata-remove]').click();
                window.AuraConfigState.syncFromDOM();
            }`)
			if !page.MustEval(`()=>window.AuraConfigState.validate().valid && configData.ai_gateway.metadata.env==='duplicate' && !Object.hasOwn(configData.ai_gateway.metadata,'renamed')`).Bool() {
				t.Fatal("removing a renamed metadata row used its old key")
			}
			page.MustEval(`()=>{
                configData.ai_gateway.metadata={a:'1',b:'2',c:'3',d:'4',e:'5',f:'6'};
                renderAIGatewaySection();aiGatewayCollectMetadata();
                window.AuraConfigState.syncFromDOM();
                renderAIGatewaySection();
            }`)
			if !page.MustEval(`()=>!window.AuraConfigState.validate().valid && document.querySelectorAll('[data-ai-gateway-metadata-key]').length===7`).Bool() {
				t.Fatal("metadata limit silently truncated entries")
			}
			page.MustEval(`()=>{document.querySelectorAll('[data-ai-gateway-metadata-remove]')[5].click();window.AuraConfigState.syncFromDOM();}`)
			if !page.MustEval(`()=>window.AuraConfigState.validate().valid && Object.keys(configData.ai_gateway.metadata).length===5`).Bool() {
				t.Fatal("correcting the metadata draft did not restore valid saving")
			}
			page.MustEval(`()=>{configData=window.AuraConfigState.discard();renderAIGatewaySection();}`)
			if !page.MustEval(`()=>!Array.isArray(configData.ai_gateway.metadata) && configData.ai_gateway.metadata.env==='prod' && document.getElementById('ai-gw-meta-error').textContent===''`).Bool() {
				t.Fatal("discard did not restore the saved metadata")
			}
		})
	}
}
