package ui

import "testing"

func TestMCPPermissionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{const original=window.fetch;window.fetch=async(url,opts)=>{
 if(url==='/api/mcp-server/tools')return new Response(JSON.stringify(['query_memory','execute_shell']));
 if(url==='/api/mcp-server/vscode-bridge')return new Response(JSON.stringify({recommended_tools:['ask_aurago','execute_shell'],clients:{}}));
 return original(url,opts);};}`)
	for _, bridge := range []bool{false, true} {
		page.MustEval(`async bridge=>{configData.mcp_server={enabled:true,require_auth:true,vscode_debug_bridge:bridge,allowed_tools:['query_memory']};AuraConfigState.init(configData);await selectSection('mcp_server',{scrollBehavior:'auto'});}`, bridge)
		waitForJSBool(t, page, `()=>document.querySelectorAll('.mcp-tool-cb').length>=2`)
		if !page.MustEval(`()=>document.getElementById('mcp-allowed-tools-state').value==='["query_memory"]' && !document.querySelector('.mcp-tool-cb[value="execute_shell"]').checked`).Bool() {
			t.Fatal("render expanded saved scope")
		}
		page.MustEval(`()=>{document.querySelectorAll('.mcp-tool-cb').forEach(cb=>cb.checked=true);mcpUpdateAllowedTools();}`)
		if !page.MustEval(`()=>JSON.parse(document.getElementById('mcp-allowed-tools-state').value).length===document.querySelectorAll('.mcp-tool-cb').length`).Bool() {
			t.Fatal("select-all encoded as empty")
		}
		page.MustEval(`async()=>{const saved=JSON.parse(document.getElementById('mcp-allowed-tools-state').value);configData.mcp_server.allowed_tools=saved;await selectSection('mcp_server',{scrollBehavior:'auto'});}`)
		waitForJSBool(t, page, `()=>document.querySelectorAll('.mcp-tool-cb').length>=2`)
		if !page.MustEval(`()=>[...document.querySelectorAll('.mcp-tool-cb')].every(cb=>cb.checked)`).Bool() {
			t.Fatal("reload lost selection")
		}
	}
	page.MustEval(`async()=>{configData.mcp_server.allowed_tools=[];await selectSection('mcp_server',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.mcp-tool-cb').length>=2`)
	for i := 0; i < 2; i++ {
		page.MustEval(`()=>mcpToggleVSCodeBridge(document.querySelector('[data-path="mcp_server.vscode_debug_bridge"]'))`)
		waitForJSBool(t, page, `()=>document.querySelectorAll('.mcp-tool-cb').length>=2`)
	}
	if !page.MustEval(`()=>document.getElementById('mcp-allowed-tools-state').value==='[]'`).Bool() {
		t.Fatal("toggle granted tools")
	}
	page.MustEval(`async()=>await mcpApplyBridgePreset(document.getElementById('mcp-apply-preset'))`)
	if !page.MustEval(`()=>JSON.stringify(configData.mcp_server.allowed_tools)==='["ask_aurago","execute_shell"]'`).Bool() {
		t.Fatal("explicit preset not applied")
	}
}
