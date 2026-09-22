package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersonalityDynamicsTranslations(t *testing.T) {
	files, err := filepath.Glob("lang/common/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		var labels map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, file), &labels); err != nil {
			t.Fatal(err)
		}
		for _, key := range strings.Fields("title load familiarity friction trend steady recovering strained reset hint disabled unavailable loading reset_done") {
			if labels["personality_dynamics."+key] == "" {
				t.Fatalf("missing %s in %s", key, file)
			}
		}
	}
}

func TestPersonalityDynamicsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(35 * time.Second)
	defer func() {
		_, _ = page.Eval(`()=>window.removeEventListener('beforeunload',handleConfigBeforeUnload)`)
		_ = page.Close()
	}()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`()=>{
		window.dynamicsFixture={enabled:true,mood:'focused',traits:{affinity:.84},dynamics:{revision:40,load:.64,friction:.36,familiarity:.84,trend:'strained'}};
		window.resetCalls=0; window.resetFailure=false;
		const original=window.fetch;
		window.fetch=async(url,opts)=>{
			if(url==='/api/personality/state')return new Response(JSON.stringify(dynamicsFixture),{headers:{'Content-Type':'application/json'}});
			if(url==='/api/personality/dynamics/reset'){
				resetCalls++; if(opts.method!=='POST')throw new Error('wrong method');
				if(resetFailure)return new Response('{}',{status:500});
				dynamicsFixture.dynamics={...dynamicsFixture.dynamics,revision:41,load:0,friction:0,trend:'steady'};
				return new Response(JSON.stringify(dynamicsFixture),{headers:{'Content-Type':'application/json'}});
			}
			return original(url,opts);
		};
		window.personalityDraftBefore=JSON.stringify(configData.personality);
	}`)
	page.MustEval(`async()=>{await renderSection('personality')}`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('#config-personality-dynamics meter').length===3`)
	if page.MustEval(`()=>document.querySelector('#config-personality-dynamics').textContent.includes('personality_dynamics.')`).Bool() {
		t.Fatal("raw translation keys visible")
	}
	for _, width := range []int{1280, 390} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		page.MustElement("#config-personality-dynamics").MustScrollIntoView()
		if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
			t.Fatal("personality config overflows viewport")
		}
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			name := "config-dynamics-desktop.png"
			if width == 390 {
				name = "config-dynamics-mobile.png"
			}
			if err := os.WriteFile(filepath.Join(dir, name), page.MustScreenshot(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustElement(`#config-personality-dynamics button`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#config-personality-dynamics meter[data-dynamic="load"]').value===0`)
	if page.MustEval(`()=>resetCalls!==1 || document.querySelector('meter[data-dynamic="familiarity"]').value!==.84 || JSON.stringify(configData.personality)!==personalityDraftBefore`).Bool() {
		t.Fatal("reset changed config draft or familiarity")
	}
	page.MustEval(`()=>{resetFailure=true;AuraPersonalityDynamics.render(document.querySelector('#config-personality-dynamics'),{...dynamicsFixture,dynamics:{...dynamicsFixture.dynamics,revision:42,load:.6}})}`)
	page.MustElement(`#config-personality-dynamics button`).MustClick()
	waitForJSBool(t, page, `()=>document.querySelector('#config-personality-dynamics [role="status"]').textContent.includes('nicht verfügbar')`)
	if page.MustEval(`()=>document.querySelector('meter[data-dynamic="load"]').value!==.6 || document.querySelector('#config-personality-dynamics button').disabled`).Bool() {
		t.Fatal("failed reset lost state or stayed disabled")
	}
	page.MustEval(`()=>AuraPersonalityDynamics.render(document.querySelector('#config-personality-dynamics'),{...dynamicsFixture,dynamics:{revision:1,load:1,friction:1,familiarity:0,trend:'strained'}})`)
	if page.MustEval(`()=>document.querySelector('meter[data-dynamic="load"]').value!==.6`).Bool() {
		t.Fatal("late poll overwrote newer state")
	}

	// Exercise the Dashboard consumer against its actual card markup and source.
	html := string(mustReadUIFile(t, "dashboard.html"))
	start := strings.Index(html, `<!-- 3. Personality Traits`)
	end := strings.Index(html[start+5:], "<!--")
	if start < 0 || end < 0 {
		t.Fatal("personality card boundary unavailable")
	}
	card := html[start : start+5+end]
	page.MustEval(`(html)=>{document.getElementById('content').innerHTML=html;window.dashSetHidden=(el,hide)=>{if(el)el.classList.toggle('is-hidden',hide)}}`, card)
	if err := page.AddScriptTag("", string(mustReadUIFile(t, "js/dashboard/dashboard-widgets.js"))); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>renderMoodBadge({...dynamicsFixture,dynamics:{...dynamicsFixture.dynamics,revision:45}})`)
	waitForJSBool(t, page, `()=>document.querySelectorAll('#personality-dynamics meter').length===3`)
	page.MustEval(`()=>renderMoodBadge({enabled:false})`)
	if !page.MustEval(`()=>document.getElementById('personality-content').classList.contains('is-hidden')`).Bool() {
		t.Fatal("disabled engine still exposed active dynamics")
	}
}
