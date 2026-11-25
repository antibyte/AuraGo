package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopHomeDashControlsBrowser drives the real HomeDash app in the desktop shell against the M3 fixture and covers every P1 path:
// first values within 1.5 s and header icons, the home_assistant template flow (empty, down, omitted), switch pending and confirmation,
// keyboard dimming with its 400 ms trailing send, pointer dimming with the 1250 ms throttle, select (segmented and list), number (stepper
// and slider), confirm-required arming (widget confirm, 4 s expiry, a 409), the 8 s timeout rollback with its 6 s cue, error cues and the
// 429 retry, read-only reasons (403 read_only and the desktop policy), unavailable entities, the light sheet with late adoption, and the
// per-source banners. HOMEDASH_M3_SHOTS=1 also writes review screenshots to reports/homedash-m3/. Motion is reduced (windows.animations=false).
func TestDesktopHomeDashControlsBrowser(t *testing.T) {
	fixture := newHomeDashFixture(t)
	fixture.enableM3(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/homedash/": fixture}).CancelTimeout().Timeout(10 * time.Minute)
	// Cleanups run last in, first out: end the fixture's streams before the test server waits for its handlers.
	t.Cleanup(fixture.shutdown)
	page.MustEval(`(words)=>{Object.assign(window.fixtureWords,JSON.parse(words));document.documentElement.lang='de';
		window.fixtureConsole=[];for(const k of ['error','warn']){const o=console[k];console[k]=(...a)=>{window.fixtureConsole.push(k+': '+a.map(String).join(' '));o.apply(console,a);};}}`, readDesktopAssetText(t, "lang/homedash/de.json"))
	shots := os.Getenv("HOMEDASH_M3_SHOTS") == "1"

	// Helpers ---------------------------------------------------------------------------------------------------------------------------------
	word := func(key string) string { return page.MustEval(`(k)=>window.fixtureWords[k]||''`, key).Str() }
	fill := func(key string, vars map[string]string) string {
		s := word(key)
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{{"+k+"}}", v)
		}
		return s
	}
	before := func(key string) string { return strings.SplitN(word(key), "{{", 2)[0] }
	w := func(id string) string { return `[data-hd-wid="` + id + `"]` }
	num := func(v any) float64 { n, _ := homeDashNum(v); return n }
	diag := func() string {
		return page.MustEval(`()=>JSON.stringify({widgets:[...document.querySelectorAll('.hdx-view .hd-widget')].map(x=>({wid:x.dataset.hdWid||x.dataset.kind,status:x.dataset.hdStatus,pending:x.hasAttribute('data-hd-pending'),readonly:x.hasAttribute('data-hd-readonly'),armed:x.hasAttribute('data-hd-armed'),text:(x.querySelector('.hd-state-text')||{}).textContent})),
			banners:[...document.querySelectorAll('.hdx-banner')].map(b=>b.dataset.banner),errors:window.fixtureErrors,console:window.fixtureConsole})`).Str()
	}
	untilGo := func(within time.Duration, what string, cond func() bool) time.Duration {
		t.Helper()
		start := time.Now()
		for {
			if cond() {
				return time.Since(start)
			}
			if time.Since(start) > within {
				t.Fatalf("timed out after %v waiting for %s\n%s", within, what, diag())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	until := func(within time.Duration, what, js string, args ...any) time.Duration {
		t.Helper()
		return untilGo(within, what, func() bool { return page.MustEval(js, args...).Bool() })
	}
	stateText := func(wid string) string {
		return page.MustEval(`(w)=>{const e=document.querySelector('[data-hd-wid="'+w+'"] .hd-state-text');return e?e.textContent:'';}`, wid).Str()
	}
	has := func(wid, attr string) bool {
		return page.MustEval(`(w,a)=>{const e=document.querySelector('[data-hd-wid="'+w+'"]');return !!e&&e.hasAttribute(a);}`, wid, attr).Bool()
	}
	attrOf := func(sel, attr string) string {
		return page.MustEval(`(s,a)=>{const e=document.querySelector(s);return e?String(e.getAttribute(a)):'<missing>';}`, sel, attr).Str()
	}
	liveText := func() string {
		return page.MustEval(`()=>{const l=document.querySelector('.hdx-view .hd-live');return l?l.textContent:'';}`).Str()
	}
	cmdCount := func() int { return len(fixture.commandLog()) }
	cmdsOf := func(base int, entity, command string) []homeDashCommand {
		out := []homeDashCommand{}
		for _, c := range fixture.commandLog()[base:] {
			if c.Entity == entity && c.Command == command {
				out = append(out, c)
			}
		}
		return out
	}
	waitCmds := func(base int, entity, command string, n int, within time.Duration) []homeDashCommand {
		t.Helper()
		var got []homeDashCommand
		untilGo(within, fmt.Sprintf("%d %s command(s) for %s", n, command, entity), func() bool { got = cmdsOf(base, entity, command); return len(got) >= n })
		return got
	}
	noNewCommands := func(base int, why string) {
		t.Helper()
		if got := fixture.commandLog()[base:]; len(got) != 0 {
			t.Fatalf("%s: unexpected commands %+v", why, got)
		}
	}
	confirmed := func(wid string) int { return page.MustEval(`(x)=>(window.__conf||{})[x]||0`, wid).Int() }
	waitConfirmed := func(wid string, was int) {
		t.Helper()
		until(10*time.Second, "confirmation of "+wid, `(x,b)=>((window.__conf||{})[x]||0)>b`, wid, was)
	}
	ready := func(wid string) {
		t.Helper()
		until(10*time.Second, "widget "+wid+" to show its state", `(x)=>{const e=document.querySelector('[data-hd-wid="'+x+'"]');if(!e)return false;e.scrollIntoView({block:'nearest'});const s=e.querySelector('.hd-state-text');return !!s&&s.textContent!=='';}`, wid)
	}
	// Engine bug (reported, not worked around in the engine): the store drops a released page's ids 30 s after it was left (LINGER_MS) and the stream
	// sends that unsubscribe 250 ms later; a page opened inside that 250 ms window loses its entities for good (the server still holds the ids, so no
	// snapshot follows, but the store entries were pruned). The test timeline is fixed, so openPage waits out the 29 to 32 s window after leaving a page.
	pageLeft := map[string]time.Time{}
	openPage := func(id string) {
		t.Helper()
		if cur := page.MustEval(`()=>{const b=document.querySelector('.hd-tab[aria-selected="true"]');return b?b.dataset.page:'';}`).Str(); cur != "" && cur != id {
			pageLeft[cur] = time.Now()
		}
		if at, ok := pageLeft[id]; ok {
			if since := time.Since(at); since > 29*time.Second && since < 32*time.Second {
				t.Logf("waiting %v before reopening page %s (linger expiry window)", (32*time.Second).Round(time.Second)-since.Round(time.Second), id)
				time.Sleep(32*time.Second - since)
			}
		}
		page.MustEval(`(id)=>{const b=document.querySelector('.hd-tab[data-page="'+id+'"]');if(b)b.click();}`, id)
		until(10*time.Second, "page "+id, `(id)=>{const b=document.querySelector('.hd-tab[data-page="'+id+'"]');const ws=[...document.querySelectorAll('.hdx-view .hd-page > .hd-widget')];
			if(!b||b.getAttribute('aria-selected')!=='true'||!ws.length)return false;let ok=true;for(const e of ws){const s=e.querySelector('.hd-state-text');if(!s||s.textContent===''){ok=false;e.scrollIntoView({block:'nearest'});}}return ok;}`, id)
	}
	theme := func(name, mode string) {
		page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();}`, name, mode)
	}
	shot := func(name string) {
		if !shots {
			return
		}
		dir := filepath.Join("..", "reports", "homedash-m3")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustElement(`.vd-window[data-app-id="homedash"]`).MustScreenshot(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	started, current := time.Now(), ""
	var durations []string
	step := func(name string) {
		if current != "" {
			durations = append(durations, fmt.Sprintf("%s: %v", current, time.Since(started).Round(10*time.Millisecond)))
			t.Logf("step %s done in %v", current, time.Since(started).Round(10*time.Millisecond))
		}
		current, started = name, time.Now()
	}

	// 0. First values (D8): header icons and the first gauge value within 1.5 s with 250 ms latency on every route. ------------------------------
	step("0 first values")
	theme("standard", "dark")
	fixture.setLatency(250 * time.Millisecond)
	fixture.seedDashboard(fixture.starter)
	page.MustEval(`()=>auditTest.openApp('homedash')`)
	until(30*time.Second, "the dashboard list", `()=>!!document.querySelector('.hdx-home[data-hdx-mode="list"] .hdx-card-open')`)
	page.MustEval(`()=>{window.__conf={};const v=document.querySelector('.hdx-view');new MutationObserver(recs=>{for(const r of recs){const e=r.target;
		if(e.hasAttribute&&e.hasAttribute('data-hd-confirmed')){const id=e.getAttribute('data-hd-wid')||'?';window.__conf[id]=(window.__conf[id]||0)+1;}}}).observe(v,{subtree:true,attributes:true,attributeFilter:['data-hd-confirmed']});}`)
	elapsed := page.MustEval(`()=>{const t0=performance.now();document.querySelector('.hdx-card-open').click();
		return new Promise(res=>{const tick=()=>{if(document.querySelector('[data-hd-wid="cpu"] .hd-gauge[aria-valuenow]'))res(performance.now()-t0);else if(performance.now()-t0>10000)res(-1);else requestAnimationFrame(tick);};tick();});}`).Num()
	if elapsed < 0 || elapsed > 1500 {
		t.Fatalf("first gauge value after %.0f ms, want within 1500 ms (latency 250 ms per route)", elapsed)
	}
	t.Logf("first value after %.0f ms", elapsed)
	if !page.MustEval(`()=>!!document.querySelector('[data-hd-wid="memory"] .hd-icon svg[data-hd-icon="memory"]')`).Bool() {
		t.Fatal("the memory widget head shows no memory icon")
	}
	fixture.setLatency(0)

	// 1. Template flow: empty, down, then a created dashboard with omitted entities. ---------------------------------------------------------
	step("1 template flow")
	fixture.setHATemplate(true, 0, "empty")
	page.MustEval(`()=>document.querySelector('.hdx-back').click()`)
	until(10*time.Second, "the Home Assistant button after the bootstrap re-read", `()=>{const b=document.querySelector('.hdx-home .hdx-home-head [data-action="create-ha"]');return !!b&&!b.hidden&&!b.closest('[hidden]');}`)
	createHA := func() {
		page.MustEval(`()=>document.querySelector('.hdx-home .hdx-home-head [data-action="create-ha"]').click()`)
	}
	notice := func(want string) {
		t.Helper()
		until(10*time.Second, "the home notice "+want, `(want)=>{const n=document.querySelector('.hdx-home .hdx-notice');return !!n&&!n.hidden&&n.textContent===want;}`, want)
	}
	createHA()
	notice(word("homedash.home.notice_template_empty"))
	fixture.setHATemplate(true, 0, "down")
	createHA()
	notice(fill("homedash.home.notice_source_unavailable", map[string]string{"source": "Home Assistant"}))
	fixture.setHATemplate(true, 2, "")
	createHA()
	until(10*time.Second, "the three pages", `()=>[...document.querySelectorAll('.hdx-view .hd-tab')].map(b=>b.textContent).join('|')==='Kitchen|Living room|`+word("homedash.home.ha_unassigned")+`'`)
	var created map[string]any
	if err := json.Unmarshal([]byte(fixture.body("POST dashboards")), &created); err != nil ||
		created["template"] != "home_assistant" || created["name"] != word("homedash.home.ha_name") || created["unassigned_title"] != word("homedash.home.ha_unassigned") || len(created) != 3 {
		t.Fatalf("home_assistant request body = %s (%v)", fixture.body("POST dashboards"), err)
	}
	haID := fixture.idByName(word("homedash.home.ha_name"))
	if haID == "" {
		t.Fatalf("the fixture holds no dashboard named %q", word("homedash.home.ha_name"))
	}
	until(10*time.Second, "the template_omitted banner", `()=>{const b=document.querySelector('.hdx-banner[data-banner="template_omitted"] .hdx-banner-text');return !!b&&b.textContent.includes('2');}`)
	// The store lingers released ids (the six system ids of the starter), so the stream holds the kitchen ids plus those for a while.
	wantKitchen := []string{"ha:light.kitchen", "ha:select.heater_mode", "ha:switch.coffee"}
	subscribed := time.Now().Add(10 * time.Second)
	for {
		held := fixture.subscribedIDs()
		if slices.Contains(held, wantKitchen[0]) && slices.Contains(held, wantKitchen[1]) && slices.Contains(held, wantKitchen[2]) {
			break
		}
		if time.Now().After(subscribed) {
			t.Fatalf("subscribed ids = %v, want them to include %v", held, wantKitchen)
		}
		time.Sleep(20 * time.Millisecond)
	}
	ready("w1")
	ready("w2")
	ready("w3")
	t.Logf("window and widget geometry: %s", page.MustEval(`()=>JSON.stringify({win:(r=>[r.width,r.height])(document.querySelector('.vd-window[data-app-id="homedash"]').getBoundingClientRect()),
		widgets:[...document.querySelectorAll('.hdx-view .hd-page > .hd-widget')].map(x=>{const r=x.getBoundingClientRect();return [x.dataset.hdWid,Math.round(r.left),Math.round(r.top),Math.round(r.width),Math.round(r.height)];})})`).Str())

	// 2. Toggle: optimistic overlay, pending only after 250 ms, confirmation flash. ----------------------------------------------------------
	step("2 toggle")
	if got := stateText("w2"); got != word("homedash.state.off") {
		t.Fatalf("coffee machine state line = %q, want %q", got, word("homedash.state.off"))
	}
	fixture.setConfirm(homeDashConfirm{delay: 900 * time.Millisecond})
	base := cmdCount()
	checked := page.MustEval(`()=>{const x=document.querySelector('[data-hd-wid="w2"]'),rec={t0:0,pendingAt:null,pendingOffAt:null,confirmedAt:null,confirmedOffAt:null};window.__rec=rec;
		new MutationObserver(()=>{const n=performance.now(),p=x.hasAttribute('data-hd-pending'),c=x.hasAttribute('data-hd-confirmed');
			if(p&&rec.pendingAt===null)rec.pendingAt=n;if(!p&&rec.pendingAt!==null&&rec.pendingOffAt===null)rec.pendingOffAt=n;
			if(c&&rec.confirmedAt===null)rec.confirmedAt=n;if(!c&&rec.confirmedAt!==null&&rec.confirmedOffAt===null)rec.confirmedOffAt=n;}).observe(x,{attributes:true});
		rec.t0=performance.now();x.querySelector('[role=switch]').click();
		return new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(x.querySelector('[role=switch]').getAttribute('aria-checked')))));}`).Str()
	if checked != "true" {
		t.Fatalf("aria-checked one frame after the click = %q, want true (optimistic overlay)", checked)
	}
	cmds := waitCmds(base, "ha:switch.coffee", "turn_on", 1, 5*time.Second)
	if len(cmds) != 1 || cmds[0].RequestID == "" || cmds[0].Confirmed {
		t.Fatalf("toggle commands = %+v, want one turn_on with a request_id and confirmed false", cmds)
	}
	until(10*time.Second, "the confirmation flash to end", `()=>{const r=window.__rec;return r.confirmedOffAt!==null;}`)
	var times map[string]float64
	_ = json.Unmarshal([]byte(page.MustEval(`()=>JSON.stringify(window.__rec)`).Str()), &times)
	pendingAfter := times["pendingAt"] - times["t0"]
	t.Logf("pending after %.0f ms, pending off after %.0f ms, confirmed after %.0f ms, flash %.0f ms", pendingAfter, times["pendingOffAt"]-times["t0"], times["confirmedAt"]-times["t0"], times["confirmedOffAt"]-times["confirmedAt"])
	if times["pendingAt"] == 0 || pendingAfter < 240 || pendingAfter > 700 {
		t.Fatalf("pending appeared %.0f ms after the click, want 240 to 700 ms", pendingAfter)
	}
	if times["pendingOffAt"] < times["pendingAt"]+400 || times["confirmedAt"] == 0 || times["confirmedAt"] < times["pendingAt"] {
		t.Fatalf("pending/confirmed timeline = %v, want pending for the confirmation delay, then confirmed", times)
	}
	if flash := times["confirmedOffAt"] - times["confirmedAt"]; flash < 450 || flash > 900 {
		t.Fatalf("the confirmation flash lasted %.0f ms, want about 600 ms", flash)
	}
	if has("w2", "data-hd-pending") || attrOf(w("w2")+` [role=switch]`, "aria-checked") != "true" || stateText("w2") != word("homedash.state.on") {
		t.Fatalf("coffee machine after confirmation: pending=%v state=%q", has("w2", "data-hd-pending"), stateText("w2"))
	}
	fixture.setConfirm(homeDashConfirm{delay: 80 * time.Millisecond})

	// 3. Dim by keyboard: three arrow keys, one trailing send 400 ms after the last. -----------------------------------------------------------
	step("3 keyboard dimming")
	ready("w1")
	base = cmdCount()
	was := confirmed("w1")
	if got := attrOf(w("w1")+` .hd-slider`, "aria-valuenow"); got != "40" {
		t.Fatalf("kitchen brightness before the keys = %s, want 40", got)
	}
	page.MustElement(w("w1") + ` .hd-slider`).MustFocus()
	firstKey := time.Now()
	for i := 0; i < 3; i++ {
		page.Keyboard.MustType(input.ArrowRight)
	}
	lastKey := time.Now()
	if got := attrOf(w("w1")+` .hd-slider`, "aria-valuenow"); got != "43" {
		t.Fatalf("aria-valuenow right after three ArrowRight = %s, want 43 (local value at once)", got)
	}
	t.Logf("three keys took %v", lastKey.Sub(firstKey))
	cmds = waitCmds(base, "ha:light.kitchen", "brightness", 1, 5*time.Second)
	waitConfirmed("w1", was)
	if cmds = cmdsOf(base, "ha:light.kitchen", "brightness"); len(cmds) != 1 || num(cmds[0].Args["value"]) != 43 {
		t.Fatalf("keyboard dimming commands = %+v, want exactly one brightness 43", cmds)
	}
	if gap := cmds[0].At.Sub(lastKey); gap < 300*time.Millisecond || gap > 1000*time.Millisecond {
		t.Fatalf("the trailing send came %v after the last key, want about 400 ms", gap)
	}

	// 4. Dim by pointer: real mouse drag over 2.6 s, throttle 1250 ms with a leading edge, commit on release. ---------------------------------
	step("4 pointer dimming")
	base = cmdCount()
	was = confirmed("w1")
	geo := page.MustEval(`()=>{const s=document.querySelector('[data-hd-wid="w1"] .hd-slider');s.scrollIntoView({block:'center'});const r=s.querySelector('.hd-slider-track').getBoundingClientRect(),q=s.getBoundingClientRect();
		return JSON.stringify({x:r.left,w:r.width,y:q.top+q.height/2});}`).Str()
	var track struct{ X, W, Y float64 }
	if err := json.Unmarshal([]byte(geo), &track); err != nil || track.W < 80 {
		t.Fatalf("slider geometry = %s, %v", geo, err)
	}
	at := func(f float64) float64 { return track.X + f*track.W }
	page.Mouse.MustMoveTo(at(0.20), track.Y)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	for i := 1; i <= 26; i++ {
		time.Sleep(100 * time.Millisecond)
		page.Mouse.MustMoveTo(at(0.20+0.60*float64(i)/26), track.Y)
		if i == 13 {
			shot("middrag-glass-standard-dark")
		}
	}
	if attrOf(w("w1")+` .hd-slider`, "data-hd-dragging") != "true" {
		t.Fatal("the slider is not dragging before the release")
	}
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	untilGo(5*time.Second, "the brightness commands of the drag", func() bool { return len(cmdsOf(base, "ha:light.kitchen", "brightness")) >= 2 })
	waitConfirmed("w1", was)
	cmds = cmdsOf(base, "ha:light.kitchen", "brightness")
	inter, final := cmds[:len(cmds)-1], cmds[len(cmds)-1]
	if len(inter) < 1 || len(inter) > 3 {
		t.Fatalf("%d intermediate brightness commands during the drag, want 1 to 3: %+v", len(inter), cmds)
	}
	for i := 1; i < len(inter); i++ {
		if gap := inter[i].At.Sub(inter[i-1].At); gap < 1200*time.Millisecond {
			t.Fatalf("intermediate commands %d and %d are %v apart, want at least 1200 ms (throttle 1250 ms)", i-1, i, gap)
		}
	}
	if got := attrOf(w("w1")+` .hd-slider`, "aria-valuenow"); got != fmt.Sprint(num(final.Args["value"])) {
		t.Fatalf("aria-valuenow after release = %s, the final command sent %v", got, final.Args["value"])
	}
	if has("w1", "data-hd-pending") || attrOf(w("w1"), "data-hd-status") != "ok" {
		t.Fatalf("kitchen light after the drag: pending=%v status=%s", has("w1", "data-hd-pending"), attrOf(w("w1"), "data-hd-status"))
	}
	t.Logf("drag sent %d intermediate commands, final value %v", len(inter), final.Args["value"])

	// 5. Select: segmented click, then the list popover by keyboard. -------------------------------------------------------------------------
	step("5 select")
	base = cmdCount()
	was = confirmed("w3")
	page.MustElement(w("w3") + ` [data-hd-option="boost"]`).MustClick()
	cmds = waitCmds(base, "ha:select.heater_mode", "select_option", 1, 5*time.Second)
	if cmds[0].Args["option"] != "boost" {
		t.Fatalf("segmented pick sent %+v, want option boost", cmds[0])
	}
	waitConfirmed("w3", was)
	until(5*time.Second, "boost to be checked", `()=>{const b=document.querySelector('[data-hd-wid="w3"] [data-hd-option="boost"]');return !!b&&b.getAttribute('aria-checked')==='true';}`)
	openPage("unassigned")
	base = cmdCount()
	was = confirmed("w9")
	page.MustElement(w("w9") + ` .hd-pick`).MustClick()
	until(5*time.Second, "the listbox", `()=>{const l=document.querySelector('.hd-listbox');return !!l&&l.getClientRects().length>0&&document.activeElement===l;}`)
	shot("list-open-glass-standard-dark")
	page.Keyboard.MustType(input.ArrowDown)
	page.Keyboard.MustType(input.Enter)
	cmds = waitCmds(base, "ha:input_select.radio", "select_option", 1, 5*time.Second)
	if cmds[0].Args["option"] != "Rock" {
		t.Fatalf("list pick sent %+v, want option Rock", cmds[0])
	}
	waitConfirmed("w9", was)
	until(5*time.Second, "the listbox to close and focus to return to the picker", `()=>!document.querySelector('.hd-listbox')&&document.activeElement===document.querySelector('[data-hd-wid="w9"] .hd-pick');`)

	// 6. Number: stepper plus (one trailing send), slider End. -------------------------------------------------------------------------------
	step("6 number")
	base = cmdCount()
	was = confirmed("w10")
	plus := time.Now()
	page.MustElement(w("w10") + ` .hd-step[data-dir="1"]`).MustClick()
	cmds = waitCmds(base, "ha:number.boiler_target", "set_value", 1, 5*time.Second)
	waitConfirmed("w10", was)
	if cmds = cmdsOf(base, "ha:number.boiler_target", "set_value"); len(cmds) != 1 || num(cmds[0].Args["value"]) != 56 {
		t.Fatalf("stepper commands = %+v, want one set_value 56", cmds)
	}
	if gap := cmds[0].At.Sub(plus); gap < 300*time.Millisecond || gap > 1200*time.Millisecond {
		t.Fatalf("the stepper command came %v after the click, want about 400 ms", gap)
	}
	openPage("living_room")
	base = cmdCount()
	was = confirmed("w6")
	page.MustElement(w("w6") + ` .hd-slider`).MustFocus()
	page.Keyboard.MustType(input.End)
	cmds = waitCmds(base, "ha:input_number.volume", "set_value", 1, 5*time.Second)
	if num(cmds[0].Args["value"]) != 100 {
		t.Fatalf("slider End sent %+v, want set_value 100", cmds[0])
	}
	waitConfirmed("w6", was)

	// 7. Armed confirm: widget confirm, expiry after 4 s, and a 409 that arms. ---------------------------------------------------------------
	step("7 armed confirm")
	page.MustEval(`()=>{window.__w5=document.querySelector('[data-hd-wid="w5"]');}`)
	fixture.setWidgetConfirm(t, haID, "w5", true)
	until(10*time.Second, "the living room page to render again with confirm", `()=>{const e=document.querySelector('[data-hd-wid="w5"]'),s=e&&e.querySelector('.hd-state-text');return !!e&&e!==window.__w5&&!!s&&s.textContent!=='';}`)
	base = cmdCount()
	was = confirmed("w5")
	page.MustElement(w("w5") + ` .hd-push`).MustClick()
	until(5*time.Second, "w5 to arm", `()=>{const e=document.querySelector('[data-hd-wid="w5"]');return !!e&&e.hasAttribute('data-hd-armed');}`)
	if got := stateText("w5"); !strings.HasPrefix(got, before("homedash.state.armed")) {
		t.Fatalf("armed state line = %q, want it to start with %q", got, before("homedash.state.armed"))
	}
	noNewCommands(base, "the first activation of a confirm widget")
	shot("armed-glass-standard-dark")
	page.MustElement(w("w5") + ` .hd-push`).MustClick()
	cmds = waitCmds(base, "ha:scene.movie", "activate", 1, 5*time.Second)
	if !cmds[0].Confirmed {
		t.Fatalf("second activation sent %+v, want confirmed true", cmds[0])
	}
	waitConfirmed("w5", was)
	until(5*time.Second, "w5 to disarm", `()=>{const e=document.querySelector('[data-hd-wid="w5"]');return !!e&&!e.hasAttribute('data-hd-armed');}`)
	base = cmdCount()
	page.MustElement(w("w5") + ` .hd-push`).MustClick()
	until(5*time.Second, "w5 to arm again", `()=>{const e=document.querySelector('[data-hd-wid="w5"]');return !!e&&e.hasAttribute('data-hd-armed');}`)
	armedAt := time.Now()
	until(6*time.Second, "the arming to expire", `()=>{const e=document.querySelector('[data-hd-wid="w5"]');return !!e&&!e.hasAttribute('data-hd-armed');}`)
	if open := time.Since(armedAt); open < 3500*time.Millisecond || open > 5200*time.Millisecond {
		t.Fatalf("the arming lasted %v, want 4 s", open)
	}
	noNewCommands(base, "an arming that expired")
	openPage("unassigned")
	fixture.answerNext(409, map[string]any{"status": "error", "code": "confirmation_required", "message": "this command needs a confirmation"}, 1, "ha:button.doorbell_test")
	base = cmdCount()
	page.MustElement(w("w8") + ` .hd-push`).MustClick()
	cmds = waitCmds(base, "ha:button.doorbell_test", "press", 1, 5*time.Second)
	if cmds[0].Confirmed {
		t.Fatalf("first press sent %+v, want confirmed false", cmds[0])
	}
	until(5*time.Second, "w8 to arm after the 409", `()=>{const e=document.querySelector('[data-hd-wid="w8"]');return !!e&&e.hasAttribute('data-hd-armed');}`)
	was = confirmed("w8")
	page.MustElement(w("w8") + ` .hd-push`).MustClick()
	cmds = waitCmds(base, "ha:button.doorbell_test", "press", 2, 5*time.Second)
	if !cmds[1].Confirmed {
		t.Fatalf("second press sent %+v, want confirmed true", cmds[1])
	}
	waitConfirmed("w8", was)

	// 8. Rollback without confirmation: 8 s timeout, 6 s cue. -------------------------------------------------------------------------------
	step("8 timeout rollback")
	fixture.setConfirm(homeDashConfirm{drop: true})
	base = cmdCount()
	clickAt := time.Now()
	page.MustElement(w("w7") + ` .hd-push`).MustClick()
	until(2*time.Second, "w7 to show on", `()=>{const e=document.querySelector('[data-hd-wid="w7"] [role=switch]');return !!e&&e.getAttribute('aria-checked')==='true';}`)
	waitCmds(base, "ha:input_boolean.vacation_mode", "turn_on", 1, 5*time.Second)
	until(12*time.Second, "the timeout rollback", `()=>{const e=document.querySelector('[data-hd-wid="w7"]');return !!e&&e.dataset.hdStatus==='error';}`)
	if since := time.Since(clickAt); since < 7500*time.Millisecond || since > 10*time.Second {
		t.Fatalf("the rollback came %v after the click, want about 8 s", since)
	}
	if attrOf(w("w7")+` [role=switch]`, "aria-checked") != "false" || stateText("w7") != word("homedash.cue.timeout") || !strings.Contains(liveText(), word("homedash.cue.timeout")) {
		t.Fatalf("after the timeout: aria-checked=%s state=%q live=%q", attrOf(w("w7")+` [role=switch]`, "aria-checked"), stateText("w7"), liveText())
	}
	cueAt := time.Now()
	until(7*time.Second, "the cue to end", `()=>{const e=document.querySelector('[data-hd-wid="w7"]');return !!e&&e.dataset.hdStatus==='ok';}`)
	if cue := time.Since(cueAt); cue < 5500*time.Millisecond || cue > 6800*time.Millisecond {
		t.Fatalf("the cue lasted %v after it was first seen, want 6 s", cue)
	}
	fixture.setConfirm(homeDashConfirm{delay: 80 * time.Millisecond})

	// 9. Error cues and the 429 retry. -------------------------------------------------------------------------------------------------------
	step("9 error cues")
	openPage("kitchen")
	fixture.answerNext(502, map[string]any{"status": "error", "code": "command_failed", "message": "Home Assistant did not accept the command"}, 1, "ha:switch.coffee")
	base = cmdCount()
	page.MustElement(w("w2") + ` [role=switch]`).MustClick()
	waitCmds(base, "ha:switch.coffee", "turn_off", 1, 5*time.Second)
	until(5*time.Second, "the command_failed cue", `(want)=>{const e=document.querySelector('[data-hd-wid="w2"]'),c=e&&e.querySelector('[role=switch]');return !!e&&e.dataset.hdStatus==='error'&&c.getAttribute('aria-checked')==='true'&&e.querySelector('.hd-state-text').textContent===want;}`, word("homedash.cue.command_failed"))
	shot("errorcue-glass-standard-dark")
	fixture.answerNext(503, map[string]any{"status": "error", "code": "source_unavailable", "message": "Home Assistant is not reachable", "source": "ha"}, 1, "ha:switch.coffee")
	page.MustElement(w("w2") + ` [role=switch]`).MustClick()
	wantSource := fill("homedash.cue.source_unavailable", map[string]string{"source": "Home Assistant"})
	until(5*time.Second, "the source_unavailable cue", `(want)=>{const e=document.querySelector('[data-hd-wid="w2"]');return !!e&&e.dataset.hdStatus==='error'&&e.querySelector('.hd-state-text').textContent===want;}`, wantSource)
	fixture.answerNext(429, map[string]any{"status": "error", "code": "rate_limited", "message": "too many commands", "retry_after_ms": 700}, 1, "ha:light.kitchen")
	base = cmdCount()
	was = confirmed("w1")
	page.MustElement(w("w1") + ` .hd-slider`).MustFocus()
	page.Keyboard.MustType(input.End)
	cmds = waitCmds(base, "ha:light.kitchen", "brightness", 2, 8*time.Second)
	if len(cmds) != 2 || cmds[0].RequestID == cmds[1].RequestID || num(cmds[0].Args["value"]) != 100 || num(cmds[1].Args["value"]) != 100 {
		t.Fatalf("429 retry commands = %+v, want two brightness 100 with different request ids", cmds)
	}
	if gap := cmds[1].At.Sub(cmds[0].At); gap < 550*time.Millisecond || gap > 1200*time.Millisecond {
		t.Fatalf("the retry came %v after the rejected send, want about 700 ms", gap)
	}
	waitConfirmed("w1", was)
	if got := attrOf(w("w1")+` .hd-slider`, "aria-valuenow"); got != "100" || attrOf(w("w1"), "data-hd-status") != "ok" || has("w1", "data-hd-pending") {
		t.Fatalf("kitchen light after the retry: aria-valuenow=%s status=%s", got, attrOf(w("w1"), "data-hd-status"))
	}

	// 10. Read-only: the desktop policy first (a 403 lock outranks it until the next entity frame), then a 403 read_only answer. ------------------
	step("10 read-only")
	base = cmdCount()
	policy := func(on bool) {
		fixture.setReadonly(on)
		page.MustEval(`(on)=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:on}}))`, on)
	}
	policy(true)
	// The 6 s cue of step 9 still owns the state line of w2 (host words come first), so its reason shows only once the cue has ended.
	until(8*time.Second, "w2 to be read-only on the read-only desktop", `()=>{const e=document.querySelector('[data-hd-wid="w2"]');return !!e&&e.hasAttribute('data-hd-readonly')&&e.querySelector('.hd-state-text').textContent.includes('`+word("homedash.reason.desktop_readonly")+`');}`)
	page.MustElement(w("w2") + ` [role=switch]`).MustClick()
	until(5*time.Second, "the long desktop reason to be announced", `(want)=>{const l=document.querySelector('.hdx-view .hd-live');return !!l&&l.textContent===want;}`, word("homedash.reason.desktop_readonly_long"))
	noNewCommands(base, "an activation on the read-only desktop")
	if got := attrOf(w("w2")+` [role=switch]`, "aria-disabled"); got == "true" {
		t.Fatalf("a read-only switch carries aria-disabled=%s, read-only never sets it", got)
	}
	shot("readonly-glass-standard-dark")
	policy(false)
	until(5*time.Second, "w2 to leave read-only", `()=>{const e=document.querySelector('[data-hd-wid="w2"]');return !!e&&!e.hasAttribute('data-hd-readonly')&&e.dataset.hdStatus!=='error';}`)
	fixture.answerNext(403, map[string]any{"status": "error", "code": "read_only", "message": "commands are switched off", "blocked_by": "home_assistant.allowed_services"}, 1, "ha:switch.coffee")
	page.MustElement(w("w2") + ` [role=switch]`).MustClick()
	waitCmds(base, "ha:switch.coffee", "turn_off", 1, 5*time.Second)
	until(5*time.Second, "w2 to be read-only after the 403", `(want)=>{const e=document.querySelector('[data-hd-wid="w2"]');return !!e&&e.hasAttribute('data-hd-readonly')&&e.querySelector('.hd-state-text').textContent.includes(want);}`, word("homedash.reason.allowed_services"))
	if attrOf(w("w2"), "data-hd-status") != "ok" || attrOf(w("w2")+` [role=switch]`, "aria-disabled") == "true" || attrOf(w("w2")+` [role=switch]`, "aria-checked") != "true" {
		t.Fatalf("after the 403 read_only: status=%s aria-disabled=%s aria-checked=%s, want ok, not disabled, rolled back to on",
			attrOf(w("w2"), "data-hd-status"), attrOf(w("w2")+` [role=switch]`, "aria-disabled"), attrOf(w("w2")+` [role=switch]`, "aria-checked"))
	}
	if n := len(cmdsOf(base, "ha:switch.coffee", "turn_off")); n != 1 {
		t.Fatalf("%d turn_off commands for the coffee machine, want only the one the 403 answered", n)
	}

	// 11. Unavailable. -----------------------------------------------------------------------------------------------------------------------
	step("11 unavailable")
	fixture.pushHAState("ha:switch.coffee", map[string]any{"value": nil, "available": false, "text": "unavailable"})
	until(5*time.Second, "w2 to be unavailable", `(want)=>{const e=document.querySelector('[data-hd-wid="w2"]');return !!e&&e.querySelector('[role=switch]').getAttribute('aria-disabled')==='true'&&e.querySelector('.hd-state-text').textContent===want;}`, word("homedash.state.unavailable"))

	// 12. Light sheet and late adoption. -----------------------------------------------------------------------------------------------------
	step("12 light sheet")
	const sheetSel = `.hd-sheet[data-kind="moreinfo"][role="dialog"]`
	page.MustEval(`()=>{const s=document.querySelector('[data-hd-wid="w1"] .hd-slider');s.scrollIntoView({block:'center'});s.focus();
		s.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',shiftKey:true,bubbles:true,cancelable:true}));}`)
	until(5*time.Second, "the light sheet with focus inside", `(sel)=>{const d=document.querySelector(sel);return !!d&&d.getClientRects().length>0&&d.contains(document.activeElement);}`, sheetSel)
	fixture.setConfirm(homeDashConfirm{delay: 80 * time.Millisecond, skew: map[string]any{"color_temp_kelvin": 6300}})
	base = cmdCount()
	page.MustEval(`(sel)=>document.querySelector(sel+' .hd-ct').focus()`, sheetSel)
	ctKey := time.Now()
	page.Keyboard.MustType(input.End)
	cmds = waitCmds(base, "ha:light.kitchen", "color_temp", 1, 5*time.Second)
	if num(cmds[0].Args["kelvin"]) != 6502 {
		t.Fatalf("CT End sent %+v, want color_temp kelvin 6502 (grid anchored at min 2202, step 50)", cmds[0])
	}
	until(10*time.Second, "late adoption of the reported 6300 K", `(sel)=>{const c=document.querySelector(sel+' .hd-ct');return !!c&&c.getAttribute('aria-valuenow')==='6300';}`, sheetSel)
	if adopted := time.Since(ctKey); adopted < 7*time.Second {
		t.Fatalf("the skewed colour temperature was adopted after %v, want the 8 s timeout", adopted)
	}
	time.Sleep(time.Until(ctKey.Add(9200 * time.Millisecond)))
	if got := attrOf(sheetSel, "data-hd-status"); got != "ok" || attrOf(sheetSel+` .hd-ct`, "aria-valuenow") != "6300" {
		t.Fatalf("light sheet 9 s after the CT send: status=%s kelvin=%s, want ok and 6300", got, attrOf(sheetSel+` .hd-ct`, "aria-valuenow"))
	}
	fixture.setConfirm(homeDashConfirm{delay: 80 * time.Millisecond})
	page.Keyboard.MustType(input.Escape)
	until(5*time.Second, "the sheet to close and focus to return to the slider", `(sel)=>!document.querySelector(sel)&&document.activeElement===document.querySelector('[data-hd-wid="w1"] .hd-slider');`, sheetSel)
	openPage("living_room")
	base = cmdCount()
	tile := page.MustEval(`()=>{const s=document.querySelector('[data-hd-wid="w4"] .hd-ltile');s.scrollIntoView({block:'center'});const r=s.getBoundingClientRect();return JSON.stringify({x:r.left+r.width/2,y:r.top+r.height/2});}`).Str()
	var centre struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(tile), &centre); err != nil {
		t.Fatal(err)
	}
	page.Mouse.MustMoveTo(centre.X, centre.Y)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	time.Sleep(600 * time.Millisecond) // a 500 ms hold, held for real
	until(5*time.Second, "the sheet after the hold", `(sel)=>{const d=document.querySelector(sel);return !!d&&d.getClientRects().length>0;}`, sheetSel)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	if rows := page.MustEval(`(sel)=>[...document.querySelector(sel).querySelectorAll('.hd-mi-row:not([hidden])')].map(r=>r.dataset.row).join(',')`, sheetSel).Str(); rows != "power" {
		t.Fatalf("sofa lamp sheet rows = %q, want only power", rows)
	}
	noNewCommands(base, "a hold that opens the sheet (the tap must be swallowed)")
	page.Keyboard.MustType(input.Escape)
	until(5*time.Second, "the sofa sheet to close", `(sel)=>!document.querySelector(sel)`, sheetSel)

	// 13. Per-source banners. ----------------------------------------------------------------------------------------------------------------
	step("13 source banners")
	const bannerText = `(kind,src)=>{const b=document.querySelector('.hdx-banner[data-banner="'+kind+'"] .hdx-banner-text');return !!b&&b.textContent.includes(src);}`
	fixture.setSource("ha", "degraded", "unreachable")
	until(5*time.Second, "the source_degraded banner", bannerText, "source_degraded", "Home Assistant")
	fixture.setSource("ha", "unavailable", "catalog_failed")
	until(5*time.Second, "the source_catalog_failed banner", bannerText, "source_catalog_failed", "Home Assistant")
	until(5*time.Second, "the degraded banner to be replaced", `()=>!document.querySelector('.hdx-banner[data-banner="source_degraded"]')`)
	fixture.setSource("ha", "live", "")
	until(5*time.Second, "the source banners to clear", `()=>!document.querySelector('.hdx-banner[data-banner^="source_"]')`)

	// 14. Screenshots (HOMEDASH_M3_SHOTS=1): every page per family in two looks, and the light sheet in Glass and Tactile. ----------------------
	step("14 screenshots")
	if shots {
		for _, look := range [][2]string{{"standard", "dark"}, {"fruity", "light"}} {
			theme(look[0], look[1])
			for _, family := range []string{"glass", "tactile", "minimal", "instrument"} {
				fixture.setFamily(t, haID, family)
				until(10*time.Second, family+" family", `(f)=>{const r=document.querySelector('.hdx-view .hd-root');return !!r&&r.dataset.hdFamily===f&&document.querySelectorAll('.hdx-view .hd-page > .hd-widget').length>0;}`, family)
				for _, pg := range []string{"kitchen", "living_room", "unassigned"} {
					openPage(pg)
					shot(fmt.Sprintf("page-%s-%s-%s-%s", pg, family, look[0], look[1]))
				}
				if family == "glass" || family == "tactile" {
					openPage("kitchen")
					page.MustEval(`()=>{const s=document.querySelector('[data-hd-wid="w1"] .hd-slider');s.focus();s.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',shiftKey:true,bubbles:true,cancelable:true}));}`)
					until(5*time.Second, "the sheet for the screenshot", `(sel)=>!!document.querySelector(sel)`, sheetSel)
					shot(fmt.Sprintf("sheet-%s-%s-%s", family, look[0], look[1]))
					page.Keyboard.MustType(input.Escape)
					until(5*time.Second, "the sheet to close", `(sel)=>!document.querySelector(sel)`, sheetSel)
				}
			}
		}
	}

	// 15. No page errors. ------------------------------------------------------------------------------------------------------------------
	step("15 page errors")
	if errs := page.MustEval(`()=>window.fixtureErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}
	step("done")
	t.Logf("step durations: %s", strings.Join(durations, "; "))
	if logged := page.MustEval(`()=>window.fixtureConsole.join('\n')`).Str(); logged != "" {
		t.Logf("console warnings and errors:\n%s", logged)
	}
}
