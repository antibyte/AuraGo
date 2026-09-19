package ui

import (
	"fmt"
	"testing"
)

func TestConfigDisclosureSettingsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	if page.MustEval(`()=>('_effective_tool_policy' in configData) || ('_config_migrations' in AuraConfigState.snapshot().draft) || !effectiveToolPolicy || configMigrationNotices.length!==1`).Bool() {
		t.Fatal("response diagnostics leaked into editable config or were lost")
	}
	page.MustEval(`()=>{window.originalFixtureFetch=window.fetch;window.fetch=async (url,opts)=>{
		if(url==='/api/mcp-server/tools') return new Response(JSON.stringify(['docker','filesystem']),{headers:{'Content-Type':'application/json'}});
		if(url==='/api/personality') return new Response('{}',{headers:{'Content-Type':'application/json'}});
		return originalFixtureFetch(url,opts);
	}}`)
	page.MustEval(`async()=>{configData.agent.output_compression.enabled=false;AuraConfigState.init(configData);await selectSection('output_compression',{scrollBehavior:'auto'});}`)
	if !page.MustEval(`()=>!!document.querySelector('[data-path="agent.output_compression.reversible.enabled"]')`).Bool() {
		t.Fatal("archive controls hidden while compression is disabled")
	}
	page.MustEval(`async()=>{await selectSection('memory_analysis',{scrollBehavior:'auto'});}`)
	if page.MustEval(`()=>[...document.querySelectorAll('#content [data-path]')].some(el=>/memory_analysis\.(enabled|preset|real_time|query_expansion|llm_reranking|unified_memory_block|effectiveness_tracking|weekly_reflection)$/.test(el.dataset.path))`).Bool() {
		t.Fatal("obsolete memory switches still rendered")
	}
	page.MustEval(`async()=>{await selectSection('skill_manager',{scrollBehavior:'auto'});}`)
	if !page.MustEval(`()=>!!document.querySelector('[data-path="tools.skill_manager.readonly"]') && !document.querySelector('[data-path^="skill_manager."]')`).Bool() {
		t.Fatal("skill manager writes obsolete root")
	}
	page.MustEval(`async()=>{configData.mcp_server={enabled:true,require_auth:true,allowed_tools:[]};AuraConfigState.init(configData);await selectSection('mcp_server',{scrollBehavior:'auto'});}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.mcp-tool-cb').length===2`)
	if page.MustEval(`()=>[...document.querySelectorAll('.mcp-tool-cb')].some(el=>el.checked)`).Bool() {
		t.Fatal("empty allowlist displayed as all allowed")
	}
	page.MustEval(`()=>{document.querySelectorAll('.mcp-tool-cb').forEach(el=>el.checked=true);mcpUpdateAllowedTools();}`)
	if page.MustEval(`()=>configData.mcp_server.allowed_tools.length`).Int() != 2 {
		t.Fatal("select all did not save explicit names")
	}
	page.MustEval(`async()=>{AuraConfigState.set('agent.max_tool_guides',7);configData=AuraConfigState.snapshot().draft;await selectSection('prompts_editor',{scrollBehavior:'auto'});persState.editName='neutral';await persActivate();}`)
	if !page.MustEval(`()=>AuraConfigState.get('agent.max_tool_guides')===7 && AuraConfigState.isDirty() && AuraConfigState.get('personality.core_personality',{saved:true})==='neutral'`).Bool() {
		t.Fatal("personality activation overwrote unrelated draft or canonical state")
	}
	for _, section := range []string{"optimizations", "output_compression", "memory_analysis", "skill_manager", "mcp_server", "prompts_editor", "danger_zone"} {
		page.MustEval(`async key=>{configData.agent.output_compression.enabled=true;await selectSection(key,{scrollBehavior:'auto'});}`, section)
		for _, width := range []int{390, 768, 1024, 1440, 1920} {
			for _, theme := range []string{"dark", "light"} {
				for _, density := range []string{"comfortable", "compact"} {
					t.Run(fmt.Sprintf("%s/%d/%s/%s", section, width, theme, density), func(t *testing.T) {
						page.MustSetViewport(width, 900, 1, width == 390)
						page.MustEval(`(theme,density)=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;AuraPrecisionWorkspace.setDensity(density);}`, theme, density)
						page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
						if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1 || !!document.querySelector('#content .cfg-error-state')`).Bool() {
							t.Fatal("configuration page overflow or render failure")
						}
					})
				}
			}
		}
	}
}
