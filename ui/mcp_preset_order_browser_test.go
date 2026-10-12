package ui

import "testing"

func TestMCPPresetIgnoresStaleCatalogBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, staleFailure := range []bool{false, true} {
		page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
		defer page.MustClose()
		waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
		page.MustEval(`()=>{const original=window.fetch;window.pendingMCPCatalogs=[];window.fetch=async(url,opts)=>{
 if(url==='/api/mcp-server/tools')return new Promise(resolve=>window.pendingMCPCatalogs.push(resolve));
 if(url==='/api/mcp-server/vscode-bridge')return new Response(JSON.stringify({recommended_tools:['ask_aurago','execute_shell'],clients:{}}));
 return original(url,opts);};}`)
		page.MustEval(`async()=>{configData.mcp_server={enabled:true,require_auth:true,vscode_debug_bridge:true,allowed_tools:['query_memory']};AuraConfigState.init(configData);await selectSection('mcp_server',{scrollBehavior:'auto'});}`)
		waitForJSBool(t, page, `()=>window.pendingMCPCatalogs.length===1`)
		page.MustEval(`()=>{window.presetFinished=false;mcpApplyBridgePreset(document.getElementById('mcp-apply-preset')).then(()=>window.presetFinished=true);}`)
		waitForJSBool(t, page, `()=>window.pendingMCPCatalogs.length===2`)
		page.MustEval(`()=>window.pendingMCPCatalogs[1](new Response(JSON.stringify(['query_memory','execute_shell','ask_aurago'])))`)
		waitForJSBool(t, page, `()=>window.presetFinished`)
		page.MustEval(`async failure=>{window.pendingMCPCatalogs[0](failure?new Response('',{status:500}):new Response(JSON.stringify(['query_memory','execute_shell','ask_aurago'])));await new Promise(resolve=>setTimeout(resolve,0));}`, staleFailure)
		if !page.MustEval(`()=>{AuraConfigState.syncFromDOM();const selected=AuraConfigState.buildPatch().mcp_server?.allowed_tools;return selected?.includes('execute_shell')&&selected?.includes('ask_aurago')&&document.querySelectorAll('.mcp-tool-cb:checked').length===3;}`).Bool() {
			t.Fatalf("stale catalog overwrote explicit preset or save state (failure=%v)", staleFailure)
		}
	}
}
