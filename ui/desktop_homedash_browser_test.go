package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDesktopHomeDashBrowser opens HomeDash from the start menu against the M2 contract fixture: empty state, starter dashboard, live values
// and tones, sparkline history, a family change through dashboard_changed, a dropped stream with the stale banner and the reconnect, an
// empty dashboard, deleting it through the confirm sheet, and the read-only desktop. HOMEDASH_M2_SHOTS=1 also writes the lead's review
// screenshots to reports/homedash-m2/.
func TestDesktopHomeDashBrowser(t *testing.T) {
	fixture := newHomeDashFixture(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(5 * time.Minute)
	page.MustEval(`(words)=>Object.assign(window.fixtureWords, JSON.parse(words))`, readDesktopAssetText(t, "lang/homedash/de.json"))
	word := func(key string) string { return page.MustEval(`(k)=>window.fixtureWords[k]||''`, key).Str() }
	shots := os.Getenv("HOMEDASH_M2_SHOTS") == "1"
	theme := func(name, mode string) {
		page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();}`, name, mode)
	}
	shot := func(name string) {
		if !shots {
			return
		}
		dir := filepath.Join("..", "reports", "homedash-m2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		time.Sleep(600 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustElement(`.vd-window[data-app-id="homedash"]`).MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	both := func(name string) {
		if !shots {
			return
		}
		theme("fruity", "light")
		shot(name + "-fruity-light")
		theme("standard", "dark")
		shot(name + "-standard-dark")
	}
	theme("standard", "dark")
	const root = `document.querySelector('.hdx-view .hd-root')`

	// 1. Start menu, category comms.
	page.MustEval(`()=>{if(document.getElementById('vd-start-menu').hidden)document.getElementById('vd-start-button').click();}`)
	page.MustWait(`()=>!document.getElementById('vd-start-menu').hidden&&!!document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]')`)
	page.MustEval(`()=>document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.vd-start-item[data-app-id="homedash"]')`)
	page.MustEval(`()=>document.querySelector('.vd-start-item[data-app-id="homedash"]').click()`)

	// 2. First-run empty state.
	page.MustWait(`()=>!!document.querySelector('.hdx-app[data-hdx-view="home"] .hdx-home[data-hdx-mode="empty"]')`)
	if got := page.MustEval(`()=>document.querySelector('.hdx-empty-title').textContent`).Str(); got != word("homedash.home.empty_title") {
		t.Fatalf("empty state title = %q", got)
	}
	both("empty")

	// 3. Starter dashboard: six widgets in Glass, subscribed on the stream.
	page.MustEval(`()=>document.querySelector('.hdx-empty [data-action="create-starter"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-root .hd-page > .hd-widget').length===6`)
	if body := fixture.body("POST dashboards"); !strings.Contains(body, `"template":"system"`) || !strings.Contains(body, `"name":"`+word("homedash.home.starter_name")+`"`) {
		t.Fatalf("starter request body = %s", body)
	}
	starterID := fixture.idByName(word("homedash.home.starter_name"))
	page.MustWait(`()=>` + root + `.dataset.hdFamily==='glass'&&` + root + `.dataset.hdConn==='live'`)
	fixture.waitSubscribed(t, 6, 10*time.Second)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="memory"] .hd-gauge[aria-valuenow="50"]')`)
	// SCR-M2-1: the uptime tile reads as a duration.
	page.MustWait(`()=>{const n=document.querySelector('[data-hd-wid="uptime"] .hd-num-int');return !!n&&n.textContent!==''&&!/^\d+$/.test(n.textContent);}`)

	// 4. Live values and host tones.
	fixture.pushState("system:cpu", 97)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return w.dataset.hdTone==='crit'&&!!w.querySelector('.hd-gauge[aria-valuenow="97"]');}`)
	fixture.pushState("system:cpu", 42)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return !w.hasAttribute('data-hd-tone')&&!!w.querySelector('.hd-gauge[aria-valuenow="42"]');}`)

	// 5. Sparkline history over the widget's hours option (1 h).
	page.MustWait(`()=>{const s=document.querySelector('[data-hd-wid="net-rx"] svg.hd-spark'),l=s&&s.querySelector('.hd-spark-line');return !!l&&!s.hasAttribute('data-hd-empty')&&(l.getAttribute('d')||'').length>40;}`)
	req := fixture.findRequest(func(r string) bool {
		return strings.Contains(r, "/history?") && strings.Contains(r, "id=system%3Anet_rx")
	})
	u, err := url.ParseRequestURI(strings.TrimPrefix(req, "GET "))
	if err != nil {
		t.Fatalf("history request %q: %v", req, err)
	}
	from, _ := time.Parse(time.RFC3339Nano, u.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339Nano, u.Query().Get("to"))
	if span := to.Sub(from); span < 3595*time.Second || span > 3605*time.Second {
		t.Fatalf("history span = %v, want the widget's 1 h", span)
	}
	// SCR-M2-2: a newer live value extends the line.
	before := page.MustEval(`()=>document.querySelector('[data-hd-wid="net-rx"] .hd-spark-line').getAttribute('d')`).Str()
	fixture.pushState("system:net_rx", 88.5)
	page.MustWait(`(before)=>document.querySelector('[data-hd-wid="net-rx"] .hd-spark-line').getAttribute('d')!==before`, before)
	both("starter-glass")

	// 6. dashboard_changed re-renders in another family; screenshots of every family.
	for _, family := range []string{"tactile", "minimal", "instrument", "glass"} {
		fixture.setFamily(t, starterID, family)
		page.MustWait(`(f)=>`+root+`.dataset.hdFamily===f&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`, family)
		if family != "glass" {
			both("starter-" + family)
		}
	}

	// 7. Stream drop: stale banner after the 5 s grace, then the reconnect clears it.
	fixture.dropStreams(2)
	page.Timeout(20 * time.Second).MustWait(`()=>!!document.querySelector('.hdx-banner[data-banner="offline"]')&&` + root + `.dataset.hdConn==='offline'&&!!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	shot("stale-standard-dark")
	page.Timeout(40 * time.Second).MustWait(`()=>!document.querySelector('.hdx-banner[data-banner="offline"]')&&` + root + `.dataset.hdConn==='live'&&!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	if opened, refused := fixture.counts(); opened < 2 || refused != 2 {
		t.Fatalf("streams opened %d, refused %d; want a reconnect after two refusals", opened, refused)
	}
	fixture.pushState("system:cpu", 55)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="55"]')`)

	// 7b. revoked shutdown: the app reconnects on its own and values flow again.
	openedBefore, _ := fixture.counts()
	fixture.revoke("shutdown")
	page.Timeout(40 * time.Second).MustWait(`()=>` + root + `.dataset.hdConn==='live'&&!document.querySelector('.hdx-banner[data-banner="offline"]')`)
	fixture.waitSubscribed(t, 6, 40*time.Second)
	if opened, _ := fixture.counts(); opened <= openedBefore {
		t.Fatalf("revoked shutdown opened no new stream (%d before, %d after)", openedBefore, opened)
	}
	fixture.pushState("system:cpu", 61)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="61"]')`)

	// 7c. revoked disabled with HomeDash switched off: the disabled screen; re-enabled, "Check again" brings the app back.
	fixture.setDisabled(true)
	fixture.revoke("disabled")
	page.Timeout(40*time.Second).MustWait(`(w)=>{const t=document.querySelector('.hdx-state-title');return !!t&&!t.closest('[hidden]')&&t.textContent===w;}`, word("homedash.disabled.title"))
	fixture.setDisabled(false)
	page.MustEval(`()=>document.querySelector('.hdx-state [data-action="retry"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-app[data-hdx-view="home"] .hdx-home[data-hdx-mode="list"] .hdx-card-open')`)
	page.MustEval(`()=>document.querySelector('.hdx-card-open').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6&&` + root + `.dataset.hdConn==='live'`)
	fixture.waitSubscribed(t, 6, 20*time.Second)
	fixture.pushState("system:cpu", 55)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="55"]')`)

	// 8. Home list, an empty dashboard, delete through the confirm sheet.
	page.MustEval(`()=>document.querySelector('.hdx-back').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===1`)
	page.MustEval(`()=>document.querySelector('.hdx-home-head [data-action="create-empty"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-view .hd-page-empty')`)
	if body := fixture.body("POST dashboards"); !strings.Contains(body, `"document"`) || !strings.Contains(body, `"name":"`+word("homedash.home.empty_name")+`"`) || !strings.Contains(body, `"id":"main"`) {
		t.Fatalf("empty dashboard request body = %s", body)
	}
	shot("empty-dashboard-standard-dark")
	emptyID := fixture.idByName(word("homedash.home.empty_name"))
	page.MustEval(`()=>document.querySelector('.hdx-back').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===2`)
	both("list")
	page.MustEval(`(id)=>document.querySelector('.hdx-card-delete[data-id="'+id+'"]').click()`, emptyID)
	page.MustWait(`()=>{const l=document.querySelector('.hdx-app > .hd-sheet-layer');return !!l&&!l.hidden&&document.activeElement===l.querySelector('[data-action="cancel"]');}`)
	shot("delete-confirm-standard-dark")
	page.MustEval(`()=>document.querySelector('.hdx-app > .hd-sheet-layer [data-action="confirm"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-home[data-hdx-mode="list"] .hdx-card').length===1`)
	if req := fixture.findRequest(func(r string) bool { return strings.HasPrefix(r, "DELETE ") }); !strings.Contains(req, "id="+emptyID) || !strings.Contains(req, "revision=1") {
		t.Fatalf("delete request = %q", req)
	}

	// 9. Read-only desktop: no creation, no delete, a read-only banner on the dashboard.
	fixture.setReadonly(true)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	page.MustWait(`()=>!document.querySelector('.hdx-readonly-note').hidden&&document.querySelector('.hdx-home-head [data-action="create-starter"]').hidden&&!document.querySelector('.hdx-card-delete')`)
	both("readonly-home")
	page.MustEval(`()=>document.querySelector('.hdx-card-open').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-banner[data-banner="readonly"]')&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`)
	both("readonly-dashboard")

	if errs := page.MustEval(`()=>window.fixtureErrors.filter(e=>/homedash|HomeDash|hd-/i.test(e)).join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
}
