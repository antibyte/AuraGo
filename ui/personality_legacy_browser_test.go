package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersonalityLegacyReviewAndFeedbackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(35 * time.Second)
	defer func() {
		_, _ = page.Eval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		_ = page.Close()
	}()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
  window.reviewCalls=[];window.legacyConflict=true;window.legacyFailure=false;window.feedbackCalls=[];window.feedbackFailure=false;
  const original=window.fetch;
  window.fetch=async(url,opts)=>{
   if(url==='/api/personality/feedback'){feedbackCalls.push(JSON.parse(opts.body));return new Response('{}',{status:feedbackFailure?410:200});}
   if(url==='/api/personality/legacy/review'){
    const body=JSON.parse(opts.body);reviewCalls.push(body);
    return new Response('{}',{status:legacyConflict&&body.id===2?409:200});
   }
   if(String(url).startsWith('/api/personality/legacy?')){
    if(legacyFailure)return new Response('{}',{status:500});
    const after=Number(new URL(url,location.origin).searchParams.get('after'));
    return new Response(JSON.stringify({entries:after?[]:[
     {id:1,kind:'profile',fingerprint:'profile-fingerprint',payload:{category:'prefs',key:'format',value:'<img src=x onerror="window.legacyXSS=true">'}},
     {id:2,kind:'emotion',fingerprint:'emotion-fingerprint',payload:{description:'Old feeling'}},
     {id:3,kind:'character',fingerprint:'protected-fingerprint',payload:{text:'Old protected note',protected:1}}
    ],next_after:3,has_more:!after}),{headers:{'Content-Type':'application/json'}});
   }
   return original(url,opts);
  };
  window.reviewDraft=JSON.stringify(configData);
 }`)
	page.MustEval(`async()=>{await renderSection('personality')}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('.personality-legacy-entry').length===3`)
	if page.MustEval(`()=>!!document.querySelector('#config-personality-legacy img')||!!window.legacyXSS||document.querySelector('#config-personality-legacy').textContent.includes('personality_legacy.')`).Bool() {
		t.Fatal("unsafe or untranslated legacy text")
	}
	if page.MustEval(`()=>!!document.querySelector('[data-entry-id="2"] [data-action="adopt"]')||!document.querySelector('[data-entry-id="3"] [data-action="discard"]').disabled`).Bool() {
		t.Fatal("old emotions or protection bypassed")
	}
	for _, width := range []int{1280, 390} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		page.MustElement("#config-personality-legacy").MustScrollIntoView()
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
			t.Fatal("legacy review overflows")
		}
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			name := "personality-legacy-desktop.png"
			if width == 390 {
				name = "personality-legacy-mobile.png"
			}
			if err := os.WriteFile(filepath.Join(dir, name), page.MustScreenshot(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustElement(`[data-entry-id="1"] [data-action="adopt"]`).MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('[data-entry-id="1"]')`)
	if !page.MustEval(`()=>reviewCalls.length===1&&reviewCalls[0].fingerprint==='profile-fingerprint'&&reviewCalls[0].action==='adopt'&&JSON.stringify(configData)===reviewDraft`).Bool() {
		t.Fatal("review changed draft or lost concurrency token")
	}
	page.MustElement(`[data-entry-id="2"] [data-action="discard"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#config-personality-legacy [role="status"]').textContent.includes('geändert')`)
	if !page.MustEval(`()=>!!document.querySelector('[data-entry-id="2"]')`).Bool() {
		t.Fatal("stale review hid retained entry")
	}
	page.MustElement(`#config-personality-legacy [data-action="next-page"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#config-personality-legacy [role="status"]').textContent.includes('Keine Einträge')`)
	page.MustEval(`()=>{legacyFailure=true}`)
	page.MustElement(`#config-personality-legacy [data-action="reload"]`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#config-personality-legacy [role="status"]').textContent.includes('fehlgeschlagen')`)

	source := string(mustReadUIFile(t, "js/chat/main/i18n-ui-chrome.js"))
	source = strings.Split(source, "/* ── Auto-resize")[0]
	page.MustEval(`(source)=>{
  const box=document.createElement('div');box.id='feedback-fixture';box.innerHTML='<textarea id="user-input"></textarea><div><button class="mood-btn" data-feedback="positive">👍</button></div>';document.body.append(box);
  window.autoResize=()=>{};new Function(source)();
 }`, source)
	page.MustElement("#feedback-fixture .mood-btn").MustClick()
	waitForJSBool(t, page, `()=>feedbackCalls.length===1&&!document.querySelector('#feedback-fixture .mood-btn').disabled`)
	if !page.MustEval(`()=>feedbackCalls[0].event_id&&Number.isFinite(Date.parse(feedbackCalls[0].occurred_at))&&feedbackCalls[0].type==='positive'`).Bool() {
		t.Fatal("feedback did not send event identity/time")
	}
	page.MustEval(`()=>{feedbackFailure=true}`)
	page.MustElement("#feedback-fixture .mood-btn").MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#personality-feedback-status').textContent.includes('fehlgeschlagen')&&!document.querySelector('#feedback-fixture .mood-btn').disabled`)
}
