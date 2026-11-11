package ui

import "testing"

func TestLLMCapabilityNoticeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	page.MustSetDocumentContent(`<!doctype html><html><body><div id="log"></div></body></html>`)
	page.MustEval(`()=>{window._auragoSharedInitialized=true;window.t=key=>key==='common.llm_router.tool_calling_unavailable'?'Keine nativen Werkzeugaufrufe':key;}`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{window.t=key=>key==='common.llm_router.tool_calling_unavailable'?'Keine nativen Werkzeugaufrufe':key;}`)
	if !page.MustEval(`()=>{const log=document.getElementById('log');AuraLLMRouteBadge(log,{turn_id:'turn',model:'model',tool_calling_unavailable:true});return log.textContent.includes('Keine nativen Werkzeugaufrufe') && !!log.querySelector('[role=status]');}`).Bool() {
		t.Fatal("missing durable capability notice")
	}
}
