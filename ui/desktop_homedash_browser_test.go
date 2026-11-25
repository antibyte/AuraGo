package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestDesktopHomeDashBrowser opens HomeDash from the start menu against the M2 contract fixture: empty state, starter dashboard, live values
// and tones, sparkline history, a family change through dashboard_changed, a dropped stream with the stale banner and the reconnect, an
// empty dashboard, deleting it through the confirm sheet, and the read-only desktop (live switch and a fresh load). HOMEDASH_M2_SHOTS=1 also
// writes the lead's review screenshots to reports/homedash-m2/.
func TestDesktopHomeDashBrowser(t *testing.T) {
	fixture := newHomeDashFixture(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(5 * time.Minute)
	// Cleanups run last in, first out: end the fixture's streams before the test server waits for its handlers.
	t.Cleanup(fixture.shutdown)
	// Production sets <html lang>; relative times and units follow it.
	page.MustEval(`(words)=>{Object.assign(window.fixtureWords, JSON.parse(words));document.documentElement.lang='de';}`, readDesktopAssetText(t, "lang/homedash/de.json"))
	word := func(key string) string { return page.MustEval(`(k)=>window.fixtureWords[k]||''`, key).Str() }
	shots := os.Getenv("HOMEDASH_M2_SHOTS") == "1"
	theme := func(name, mode string) {
		page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();}`, name, mode)
	}
	shotOf := func(selector, name string) {
		if !shots {
			return
		}
		dir := filepath.Join("..", "reports", "homedash-m2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		time.Sleep(600 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustElement(selector).MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	shot := func(name string) { shotOf(`.vd-window[data-app-id="homedash"]`, name) }
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
	const offlineBanner = `document.querySelector('.hdx-banner[data-banner="offline"]')`
	const uptimeSeconds = 7215 // system:uptime in ui/testdata/homedash-m2/system-entities.json

	// 1. Start menu, category comms (screenshots of the open menu with the HomeDash entry in both looks).
	openMenu := func() {
		page.MustEval(`()=>{if(document.getElementById('vd-start-menu').hidden)document.getElementById('vd-start-button').click();}`)
		page.MustWait(`()=>!document.getElementById('vd-start-menu').hidden&&!!document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]')`)
		page.MustEval(`()=>document.querySelector('#vd-start-rail .vd-start-category[data-category="comms"]').click()`)
		page.MustWait(`()=>!!document.querySelector('.vd-start-item[data-app-id="homedash"]')`)
	}
	openMenu()
	if shots {
		theme("fruity", "light")
		openMenu()
		shotOf(`#vd-start-menu`, "startmenu-fruity-light")
		theme("standard", "dark")
		openMenu()
		shotOf(`#vd-start-menu`, "startmenu-standard-dark")
	}
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
	if starterID == "" {
		t.Fatalf("the fixture holds no dashboard named %q", word("homedash.home.starter_name"))
	}
	page.MustWait(`()=>` + root + `.dataset.hdFamily==='glass'&&` + root + `.dataset.hdConn==='live'`)
	fixture.waitSubscribed(t, 0, 6, 10*time.Second)
	wantIDs := []string{"system:cpu", "system:disk", "system:memory", "system:net_rx", "system:net_tx", "system:uptime"}
	if got := fixture.subscribedIDs(); !slices.Equal(got, wantIDs) {
		t.Fatalf("subscribed ids = %v, want the six starter ids %v", got, wantIDs)
	}
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="memory"] .hd-gauge[aria-valuenow="50"]')`)
	// SCR-M2-1: the uptime tile reads as the formatted duration of the fixture's 7215 s.
	wantUptime := page.MustEval(`(s)=>HomeDashEngine.format.duration(s,document.documentElement.lang)`, uptimeSeconds).Str()
	if wantUptime == "" || strings.Trim(wantUptime, "0123456789") == "" {
		t.Fatalf("expected uptime text %q is not a formatted duration", wantUptime)
	}
	page.MustWait(`(want)=>{const n=document.querySelector('[data-hd-wid="uptime"] .hd-num-int');return !!n&&n.textContent!==''&&n.textContent===want;}`, wantUptime)
	if got := page.MustEval(`()=>document.querySelector('[data-hd-wid="uptime"] .hd-num-int').textContent`).Str(); got != wantUptime {
		t.Fatalf("uptime tile = %q, want %q", got, wantUptime)
	}

	// 4. Live values and host tones.
	fixture.pushState("system:cpu", 97)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return w.dataset.hdTone==='crit'&&!!w.querySelector('.hd-gauge[aria-valuenow="97"]');}`)
	fixture.pushState("system:cpu", 42)
	page.MustWait(`()=>{const w=document.querySelector('[data-hd-wid="cpu"]');return !w.hasAttribute('data-hd-tone')&&!!w.querySelector('.hd-gauge[aria-valuenow="42"]');}`)

	// 5. Sparkline history over the widget's hours option (1 h): loaded first, then a live value moves the line's last point.
	page.MustWait(`()=>{const s=document.querySelector('[data-hd-wid="net-rx"] svg.hd-spark'),l=s&&s.querySelector('.hd-spark-line');return !!l&&!s.hasAttribute('data-hd-empty')&&((l.getAttribute('d')||'').match(/[LC]/g)||[]).length>=20;}`)
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
	page.MustWait(`(before)=>{const l=document.querySelector('[data-hd-wid="net-rx"] .hd-spark-line');if(!l)return false;const d=l.getAttribute('d')||'',last=s=>(s.match(/-?\d+(?:\.\d+)?/g)||[]).slice(-2).join(',');return d!==before&&last(d)!==last(before);}`, before)
	both("starter-glass")

	// 6. dashboard_changed re-renders in another family; screenshots of every family.
	for _, family := range []string{"tactile", "minimal", "instrument", "glass"} {
		fixture.setFamily(t, starterID, family)
		page.MustWait(`(f)=>`+root+`.dataset.hdFamily===f&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`, family)
		if family != "glass" {
			both("starter-" + family)
		}
	}

	// 7. Stream drop: nothing shows inside the 5 s grace; then the stale banner; the reconnect clears it.
	fixture.dropStreams(2)
	time.Sleep(2 * time.Second)
	if early := page.MustEval(`()=>!` + offlineBanner + `&&!document.querySelector('[data-hd-stale="true"]')`).Bool(); !early {
		t.Fatal("the offline banner or a stale widget showed inside the 5 s grace after the stream dropped")
	}
	page.Timeout(20 * time.Second).MustWait(`()=>!!` + offlineBanner + `&&` + root + `.dataset.hdConn==='offline'&&!!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	shot("stale-standard-dark")
	page.Timeout(40 * time.Second).MustWait(`()=>!` + offlineBanner + `&&` + root + `.dataset.hdConn==='live'&&!document.querySelector('[data-hd-wid="cpu"][data-hd-stale="true"]')`)
	if opened, refused := fixture.counts(); opened < 2 || refused != 2 {
		t.Fatalf("streams opened %d, refused %d; want a reconnect after two refusals", opened, refused)
	}
	fixture.pushState("system:cpu", 55)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="55"]')`)

	// 7b. revoked shutdown: the client leaves live, reconnects on its own with a new stream that is subscribed again, and values flow.
	openedBefore, _ := fixture.counts()
	fixture.revoke("shutdown")
	page.Timeout(40 * time.Second).MustWait(`()=>` + root + `.dataset.hdConn!=='live'`)
	page.Timeout(40 * time.Second).MustWait(`()=>` + root + `.dataset.hdConn==='live'&&!` + offlineBanner)
	fixture.waitSubscribed(t, openedBefore, 6, 40*time.Second)
	if opened, _ := fixture.counts(); opened <= openedBefore {
		t.Fatalf("revoked shutdown opened no new stream (%d before, %d after)", openedBefore, opened)
	}
	fixture.pushState("system:cpu", 61)
	page.MustWait(`()=>!!document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow="61"]')`)

	// 7c. revoked disabled with HomeDash switched off: the disabled screen; re-enabled, "Check again" brings the app back.
	openedBefore, _ = fixture.counts()
	fixture.setDisabled(true)
	fixture.revoke("disabled")
	page.Timeout(40*time.Second).MustWait(`(w)=>{const t=document.querySelector('.hdx-state-title');return !!t&&!t.closest('[hidden]')&&t.textContent===w;}`, word("homedash.disabled.title"))
	fixture.setDisabled(false)
	page.MustEval(`()=>document.querySelector('.hdx-state [data-action="retry"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-app[data-hdx-view="home"] .hdx-home[data-hdx-mode="list"] .hdx-card-open')`)
	page.MustEval(`()=>document.querySelector('.hdx-card-open').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6&&` + root + `.dataset.hdConn==='live'`)
	fixture.waitSubscribed(t, openedBefore, 6, 20*time.Second)
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
	if emptyID == "" {
		t.Fatalf("the fixture holds no dashboard named %q", word("homedash.home.empty_name"))
	}
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

	// 9. Read-only desktop: no creation, no delete, a read-only banner on the dashboard, and not a single write request from the app.
	const readonlyHome = `()=>{const h=document.querySelector('.hdx-home[data-hdx-mode="list"]');if(!h||h.querySelector('.hdx-readonly-note').hidden)return false;
		const create=[...h.querySelectorAll('[data-action="create-starter"],[data-action="create-empty"]')];
		return create.length>=2&&create.every(b=>b.hidden)&&!h.querySelector('.hdx-card-delete');}`
	mutationsAtSwitch := fixture.mutations()
	fixture.setReadonly(true)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
	page.MustWait(readonlyHome)
	both("readonly-home")
	page.MustEval(`()=>document.querySelector('.hdx-card-open').click()`)
	page.MustWait(`()=>!!document.querySelector('.hdx-banner[data-banner="readonly"]')&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length===6`)
	both("readonly-dashboard")

	// 9b. A fresh load while the desktop is read-only: close the window and reopen it; the policy event is not sent again, so bootstrap decides.
	page.MustEval(`()=>{const w=[...auditTest.state.windows.values()].find(x=>x.appId==='homedash');return auditTest.closeWindow(w.id);}`)
	page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="homedash"]')`)
	page.MustEval(`()=>auditTest.openApp('homedash')`)
	page.MustWait(`()=>!!document.querySelector('.hdx-app[data-hdx-view="home"]')`)
	page.MustWait(readonlyHome)
	if got := fixture.mutations(); got != mutationsAtSwitch {
		t.Fatalf("the read-only desktop sent %d POST/DELETE requests (%d before the switch, %d after)", got-mutationsAtSwitch, mutationsAtSwitch, got)
	}

	if errs := page.MustEval(`()=>window.fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
}
