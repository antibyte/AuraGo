package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/i18n"
	"aurago/ui"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const homeDashShortcutShotsDir = `C:\disposable\homedash-desktop-icon\shots`

// TestDesktopHomeDashShortcutLiveBrowser: a HomeDash dashboard as a desktop icon, end to end. The real desktop shell runs with its own
// startup (bootstrap fetch, WebSocket), against the real handlers for the bootstrap, the shortcuts, the WebSocket and HomeDash, all with an
// admin token. Needs AURAGO_RUN_BROWSER_SMOKE=1 and Chrome; HOMEDASH_SHORTCUT_SHOTS=1 also writes screenshots for the visual review.
func TestDesktopHomeDashShortcutLiveBrowser(t *testing.T) {
	browser := personalRadioBrowser(t)
	shortenHomeDashStreamTimers(t)
	env, _ := newDashShortcutEnv(t, false)
	ctx := context.Background()
	// The injected service gets the capability provider getDesktopService would have set (HomeDash requires the "homedash" capability).
	env.desk.SetCapabilityProvider(desktopCapabilities{s: env.s})
	setDesktop := func(theme string, more map[string]string) {
		t.Helper()
		values := map[string]string{"pet.enabled": "false", "desktop.show_widgets": "false", "windows.animations": "false", "windows.restore_session": "false", "appearance.theme": theme}
		for k, v := range more {
			values[k] = v
		}
		if err := env.desk.SetSettings(ctx, values, desktop.SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	setDesktop("standard", nil)

	// Seed through the real API: two normal app icons and the dashboard.
	postShortcut := func(body map[string]any) {
		t.Helper()
		if w := env.post(t, env.admin, body); w.Code != http.StatusOK {
			t.Fatalf("add shortcut %v = %d %s", body, w.Code, w.Body.String())
		}
	}
	postShortcut(map[string]any{"app_id": "files"})
	postShortcut(map[string]any{"app_id": "homedash"})
	createDashboard := func(name string) string {
		t.Helper()
		w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc(name)})
		if w.Code != http.StatusCreated {
			t.Fatalf("create %q = %d %s", name, w.Code, w.Body.String())
		}
		var c hdSaved
		hdDecode(t, w, &c)
		return c.Dashboard.ID
	}
	revisionOf := func(id string) int64 {
		t.Helper()
		var cur hdSaved
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard?id="+id, nil), &cur)
		return cur.Dashboard.Revision
	}
	rename := func(id, name string) {
		t.Helper()
		var got struct {
			Dashboard map[string]any `json:"dashboard"`
		}
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard?id="+id, nil), &got)
		got.Dashboard["name"] = name
		if w := env.do(t, env.admin, http.MethodPut, "dashboard?id="+id, map[string]any{"revision": revisionOf(id), "document": got.Dashboard}); w.Code != http.StatusOK {
			t.Fatalf("rename = %d %s", w.Code, w.Body.String())
		}
	}
	dashboardShortcuts := func(token string) []string {
		t.Helper()
		var ids []string
		for _, sc := range env.bootstrap(t, token).Shortcuts {
			if sc.TargetType == desktop.ShortcutTargetDashboard {
				ids = append(ids, sc.TargetID+"="+sc.Name)
			}
		}
		return ids
	}
	id := createDashboard("Wohnzimmer")

	// The page: the desktop shell with its own startup; only a test hook is added in front of the startup call.
	i18n.Load(ui.Content, slog.Default())
	read := func(p string) string {
		b, err := ui.Content.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;window.hdErrors=[];addEventListener('error',e=>hdErrors.push(e.message));addEventListener('unhandledrejection',e=>hdErrors.push(String(e.reason)));</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/hd-shell.js"></script></body>`, 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell seam missing")
	}
	shell = shell[:cut] + "window.hdTest={state,openApp};" + shell[cut:]
	admin := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+env.admin)
			h.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.Handle(homeDashAPIPrefix, admin(handleHomeDash(env.s)))
	mux.Handle("/api/desktop/bootstrap", admin(handleDesktopBootstrap(env.s)))
	mux.Handle("/api/desktop/shortcuts", admin(handleDesktopShortcuts(env.s)))
	mux.Handle("/api/desktop/ws", admin(handleDesktopWS(env.s)))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(uiTemplateData("de", "desktop")["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/hd-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	origin := httptest.NewServer(mux)
	// Closed after the pages (defers run last in, first out).
	defer origin.Close()

	type view struct {
		page  *rod.Page
		until func(within time.Duration, what, js string, args ...any)
		eval  func(js string, args ...any) string
	}
	openDesktop := func() *view {
		t.Helper()
		page := browser.MustPage().Timeout(3 * time.Minute)
		page.MustSetViewport(1280, 900, 1, false)
		v := &view{page: page}
		v.eval = func(js string, args ...any) string { return page.MustEval(js, args...).String() }
		diag := func() string {
			return page.MustEval(`()=>JSON.stringify({errors:window.hdErrors,icons:[...document.querySelectorAll('.vd-icon')].map(i=>i.dataset.kind+'/'+i.dataset.dashboardId+'/'+i.textContent.trim()),windows:[...document.querySelectorAll('.vd-window')].map(w=>w.dataset.appId+':'+(w.querySelector('[data-hdx-view]')||{dataset:{}}).dataset.hdxView),title:(document.querySelector('.hdx-title')||{}).textContent,notice:(document.querySelector('.hdx-notice')||{}).textContent,toasts:window.hdToasts,ws:window.hdTest&&hdTest.state.ws&&hdTest.state.ws.readyState})`).Str()
		}
		v.until = func(within time.Duration, what, js string, args ...any) {
			t.Helper()
			deadline := time.Now().Add(within)
			for !page.MustEval(js, args...).Bool() {
				if time.Now().After(deadline) {
					t.Fatalf("timed out after %v waiting for %s\n%s", within, what, diag())
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
		page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
		v.until(30*time.Second, "the shell with its WebSocket", `()=>!!window.hdTest&&!!hdTest.state.bootstrap&&hdTest.state.bootstrap.enabled===true&&!!hdTest.state.ws&&hdTest.state.ws.readyState===1&&!!document.querySelector('.vd-icon[data-kind="app"]')`)
		// Toasts live for seconds only; the recorder lets a wait see one that came and went.
		page.MustEval(`()=>{window.hdToasts=[];new MutationObserver(ms=>ms.forEach(m=>m.addedNodes.forEach(n=>{if(n.nodeType===1&&n.classList.contains('hd-toast'))window.hdToasts.push(n.textContent+'@'+(n.closest('.hdx-toasts')?'host':'engine'))}))).observe(document.body,{childList:true,subtree:true})}`)
		return v
	}

	const hdWin = `.vd-window[data-app-id="homedash"]`
	dashIcon := func(id string) string { return `.vd-icon[data-kind="dashboard"][data-dashboard-id="` + id + `"]` }
	count := func(v *view, sel string) int {
		return v.page.MustEval(`(s)=>document.querySelectorAll(s).length`, sel).Int()
	}
	check := func(cond bool, format string, args ...any) {
		t.Helper()
		if !cond {
			t.Fatalf(format, args...)
		}
	}

	v := openDesktop()
	defer v.page.Close()
	page, until := v.page, v.until

	// 1. Open HomeDash, pin the card.
	page.MustEval(`()=>hdTest.openApp('homedash')`)
	until(20*time.Second, "the card of Wohnzimmer", `(id)=>!!document.querySelector('.hdx-home .hdx-card-pin[data-id="'+id+'"]')`, id)
	pin := `.hdx-home .hdx-card-pin[data-id="` + id + `"]`
	check(page.MustElement(pin).MustAttribute("aria-pressed") != nil && *page.MustElement(pin).MustAttribute("aria-pressed") == "false", "pin must start unpressed")
	check(count(v, dashIcon(id)) == 0, "no dashboard icon before pinning")
	page.MustElement(pin).MustClick()
	until(15*time.Second, "the pressed pin", `(s)=>document.querySelector(s).getAttribute('aria-pressed')==='true'`, pin)
	until(15*time.Second, "the desktop icon", `(s)=>{const i=document.querySelector(s);return !!i&&i.querySelector('.vd-icon-label').textContent==='Wohnzimmer'}`, dashIcon(id))
	until(10*time.Second, "the card meta", `(id)=>{const c=document.querySelector('.hdx-card-pin[data-id="'+id+'"]').closest('.hdx-card');return /auf dem Desktop/.test(c.querySelector('.hdx-card-meta').textContent)}`, id)
	until(10*time.Second, "the added toast", `()=>[...document.querySelectorAll('.hdx-toasts .hd-toast')].some(x=>x.textContent==='„Wohnzimmer“ zum Desktop hinzugefügt')`)
	if got := page.MustEval(`(s)=>{const i=document.querySelector(s);return i.getAttribute('aria-label')+'|'+i.dataset.appId+'|'+i.dataset.id}`, dashIcon(id)).Str(); got != "Wohnzimmer||dashboard-"+id {
		t.Fatalf("icon aria-label|appId|id = %q", got)
	}
	// The HomeDash app icon: the same logo image as the shortcut of the HomeDash app itself.
	same := page.MustEval(`(s)=>{const a=document.querySelector(s+' .vd-sprite-icon'),b=document.querySelector('.vd-icon[data-kind="app"][data-app-id="homedash"] .vd-sprite-icon');const sa=a&&a.querySelector('img[data-vd-logo-img]'),sb=b&&b.querySelector('img[data-vd-logo-img]');return !!sa&&!!sb&&sa.getAttribute('src')===sb.getAttribute('src')&&/homedash\.svg/.test(sa.getAttribute('src'))}`, dashIcon(id)).Bool()
	if !same {
		t.Fatalf("the dashboard icon is not the HomeDash app icon: %s", page.MustEval(`(s)=>(document.querySelector(s)||{}).innerHTML+" ## "+[...document.querySelectorAll(".vd-icon")].map(i=>i.dataset.kind+"/"+i.dataset.appId+": "+i.innerHTML).join(" ## ")`, dashIcon(id)).Str())
	}
	check(strings.Join(dashboardShortcuts(env.admin), ",") == id+"=Wohnzimmer", "bootstrap dashboard shortcuts = %v", dashboardShortcuts(env.admin))

	// 2. Close the window, double-click the icon: one HomeDash window, dashboard view, title Wohnzimmer.
	page.MustElement(hdWin + ` .vd-window-button[data-action="close"]`).MustClick()
	until(10*time.Second, "the closed window", `()=>document.querySelectorAll('.vd-window[data-app-id="homedash"]').length===0`)
	icon := page.MustElement(dashIcon(id))
	if err := icon.Click(proto.InputMouseButtonLeft, 2); err != nil {
		t.Fatal(err)
	}
	until(20*time.Second, "HomeDash on the dashboard", `()=>{const w=document.querySelector('.vd-window[data-app-id="homedash"] [data-hdx-view="dashboard"]');return !!w&&(document.querySelector('.hdx-title')||{}).textContent==='Wohnzimmer'}`)
	time.Sleep(300 * time.Millisecond)
	check(count(v, hdWin) == 1, "windows after the double-click = %d, want 1", count(v, hdWin))

	// 3. A second double-click focuses the same window. Another window takes the focus first so that the check means something.
	page.MustEval(`()=>hdTest.openApp('files')`)
	until(10*time.Second, "the files window in front", `()=>{const w=hdTest.state.windows.get(hdTest.state.activeWindowId);return !!w&&w.appId==='files'}`)
	// Windows open on top of the icon column; move them aside so that real clicks reach the icons.
	aside := func() { page.MustEval(`()=>document.querySelectorAll('.vd-window').forEach(w=>{w.style.left='340px'})`) }
	aside()
	if err := page.MustElement(dashIcon(id)).Click(proto.InputMouseButtonLeft, 2); err != nil {
		t.Fatal(err)
	}
	until(10*time.Second, "HomeDash focused again", `()=>{const w=hdTest.state.windows.get(hdTest.state.activeWindowId);return !!w&&w.appId==='homedash'}`)
	time.Sleep(300 * time.Millisecond)
	check(count(v, hdWin) == 1, "windows after the second double-click = %d, want 1", count(v, hdWin))
	aside()
	check(page.MustEval(`()=>(document.querySelector('.hdx-title')||{}).textContent`).Str() == "Wohnzimmer", "the window left the dashboard")

	// 3b. Window reuse. A HomeDash window on the home list takes the dashboard (still one window); a window that shows another dashboard
	// is never navigated away (a second window opens).
	titles := func() string {
		return page.MustEval(`()=>[...document.querySelectorAll('.vd-window[data-app-id="homedash"]')].map(w=>(w.querySelector('[data-hdx-view]')||{dataset:{}}).dataset.hdxView==='home'?'(home)':((w.querySelector('.hdx-title')||{}).textContent||'?')).join('|')`).Str()
	}
	page.MustElement(hdWin + ` .hdx-back`).MustClick()
	until(15*time.Second, "the home list in the only window", `()=>!!document.querySelector('.vd-window[data-app-id="homedash"] [data-hdx-view="home"]')`)
	check(titles() == "(home)", "windows before the reuse = %q", titles())
	aside()
	if err := page.MustElement(dashIcon(id)).Click(proto.InputMouseButtonLeft, 2); err != nil {
		t.Fatal(err)
	}
	until(20*time.Second, "the home window navigated to the dashboard", `()=>{const w=document.querySelector('.vd-window[data-app-id="homedash"] [data-hdx-view="dashboard"]');return !!w&&(document.querySelector('.hdx-title')||{}).textContent==='Wohnzimmer'}`)
	time.Sleep(300 * time.Millisecond)
	check(titles() == "Wohnzimmer", "windows after the double-click on the home list = %q, want one Wohnzimmer window", titles())
	// A second pinned dashboard while the window shows Wohnzimmer: its icon opens a new window.
	idOther := createDashboard("Garten")
	postShortcut(map[string]any{"dashboard_id": idOther})
	until(15*time.Second, "the icon of Garten", `(s)=>!!document.querySelector(s)`, dashIcon(idOther))
	aside()
	if err := page.MustElement(dashIcon(idOther)).Click(proto.InputMouseButtonLeft, 2); err != nil {
		t.Fatal(err)
	}
	until(20*time.Second, "a second window on Garten", `()=>document.querySelectorAll('.vd-window[data-app-id="homedash"]').length===2&&[...document.querySelectorAll('.vd-window[data-app-id="homedash"] .hdx-title')].some(x=>x.textContent==='Garten')`)
	time.Sleep(300 * time.Millisecond)
	check(count(v, hdWin) == 2, "windows with a dashboard open = %d, want 2", count(v, hdWin))
	check(strings.Contains("|"+titles()+"|", "|Wohnzimmer|") && strings.Contains("|"+titles()+"|", "|Garten|"), "titles = %q, want Wohnzimmer and Garten", titles())
	// Clean up: close the Garten window and delete its dashboard (and with it the shortcut).
	page.MustEval(`()=>{const w=[...document.querySelectorAll('.vd-window[data-app-id="homedash"]')].find(x=>((x.querySelector('.hdx-title')||{}).textContent==='Garten'));w.querySelector('.vd-window-button[data-action="close"]').click()}`)
	until(10*time.Second, "the Garten window closed", `()=>document.querySelectorAll('.vd-window[data-app-id="homedash"]').length===1`)
	if w := env.do(t, env.admin, http.MethodDelete, fmt.Sprintf("dashboard?id=%s&revision=%d", idOther, revisionOf(idOther)), nil); w.Code != http.StatusOK {
		t.Fatalf("delete Garten = %d %s", w.Code, w.Body.String())
	}
	until(15*time.Second, "the icon of Garten gone", `(s)=>!document.querySelector(s)`, dashIcon(idOther))
	aside()

	// 4. Context menu of the icon: Open, Remove from desktop, nothing else.
	if err := page.MustElement(dashIcon(id)).Click(proto.InputMouseButtonRight, 1); err != nil {
		t.Fatal(err)
	}
	until(5*time.Second, "the icon context menu", `()=>!!document.querySelector('.vd-context-menu .vd-context-item')`)
	labels := page.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu .vd-context-label')].map(x=>x.textContent).join('|')`).Str()
	check(labels == "Öffnen|Vom Desktop entfernen", "icon context menu = %q, want Öffnen|Vom Desktop entfernen", labels)
	page.MustEval(`()=>document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape'}))`)
	until(5*time.Second, "the closed context menu", `()=>!document.querySelector('.vd-context-menu')`)

	// 5. The Dashboard window menu offers the removal.
	menuItem := hdWin + ` .vd-window-menu[data-menu-id="homedash-dashboard"] .vd-window-menu-item`
	items := page.MustEval(`(s)=>[...document.querySelectorAll(s)].map(x=>x.textContent.trim().replace(/\s+/g,' '))`, menuItem).Arr()
	var texts []string
	for _, it := range items {
		texts = append(texts, it.String())
	}
	check(strings.Contains(strings.Join(texts, "|"), "Vom Desktop entfernen"), "Dashboard menu = %v, want Vom Desktop entfernen", texts)
	check(!strings.Contains(strings.Join(texts, "|"), "Zum Desktop hinzufügen"), "Dashboard menu offers adding too: %v", texts)

	// 6. A rename through the API reaches the icon and the open window without a reload.
	rename(id, "Küche")
	until(15*time.Second, "the renamed icon", `(s)=>{const i=document.querySelector(s);return !!i&&i.querySelector('.vd-icon-label').textContent==='Küche'&&i.getAttribute('aria-label')==='Küche'}`, dashIcon(id))
	until(15*time.Second, "the renamed window title", `()=>(document.querySelector('.hdx-title')||{}).textContent==='Küche'`)
	check(strings.Join(dashboardShortcuts(env.admin), ",") == id+"=Küche", "bootstrap after the rename = %v", dashboardShortcuts(env.admin))

	// 7. Remove through the window menu: icon gone, toast in the dashboard, then the card shows it unpressed. Pin again from the card.
	page.MustElement(hdWin + ` .vd-window-menu-button[data-window-menu="homedash-dashboard"]`).MustClick()
	until(5*time.Second, "the open Dashboard menu", `()=>!!document.querySelector('.vd-window[data-app-id="homedash"] .vd-window-menu.open')`)
	var removeItem *rod.Element
	for _, el := range page.MustElements(hdWin + ` .vd-window-menu.open .vd-window-menu-item`) {
		if strings.Contains(el.MustText(), "Vom Desktop entfernen") {
			removeItem = el
		}
	}
	check(removeItem != nil, "no remove item in the open Dashboard menu")
	removeItem.MustClick()
	until(15*time.Second, "the icon gone", `(s)=>!document.querySelector(s)`, dashIcon(id))
	until(10*time.Second, "the removed toast", `()=>[...document.querySelectorAll('.vd-window[data-app-id="homedash"] .hd-toast')].some(x=>x.textContent==='„Küche“ vom Desktop entfernt')`)
	check(len(dashboardShortcuts(env.admin)) == 0, "stored shortcut survived the removal: %v", dashboardShortcuts(env.admin))
	page.MustElement(hdWin + ` .hdx-back`).MustClick()
	until(15*time.Second, "the card unpressed", `(id)=>{const b=document.querySelector('.hdx-card-pin[data-id="'+id+'"]');return !!b&&b.getAttribute('aria-pressed')==='false'}`, id)
	page.MustElement(pin).MustClick()
	until(15*time.Second, "the pinned again icon", `(s)=>{const i=document.querySelector(s);return !!i&&i.querySelector('.vd-icon-label').textContent==='Küche'}`, dashIcon(id))
	until(10*time.Second, "the card pressed again", `(s)=>document.querySelector(s).getAttribute('aria-pressed')==='true'`, pin)

	// 8. Deleting the open dashboard through the API: icon gone, the window shows the German notice, the bootstrap has no dashboard shortcut.
	page.MustElement(`.hdx-home .hdx-card-open[data-id="` + id + `"]`).MustClick()
	until(15*time.Second, "the dashboard view", `()=>!!document.querySelector('.vd-window[data-app-id="homedash"] [data-hdx-view="dashboard"]')`)
	if w := env.do(t, env.admin, http.MethodDelete, fmt.Sprintf("dashboard?id=%s&revision=%d", id, revisionOf(id)), nil); w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	until(15*time.Second, "the icon gone after the delete", `(s)=>!document.querySelector(s)`, dashIcon(id))
	until(15*time.Second, "the deleted notice", `()=>/Das Dashboard wurde gelöscht/.test((document.querySelector('.hdx-notice')||{}).textContent||'')`)
	check(len(dashboardShortcuts(env.admin)) == 0, "bootstrap after the delete = %v", dashboardShortcuts(env.admin))
	check(len(env.storedDashboardShortcuts(t)) == 0, "stored shortcut survived the delete")

	// 9. A read token never learns about the dashboard shortcut, an admin token does.
	id2 := createDashboard("Büro")
	postShortcut(map[string]any{"dashboard_id": id2})
	check(strings.Join(dashboardShortcuts(env.admin), ",") == id2+"=Büro", "admin bootstrap = %v", dashboardShortcuts(env.admin))
	check(len(dashboardShortcuts(env.read)) == 0, "read token bootstrap shows %v", dashboardShortcuts(env.read))
	until(15*time.Second, "the icon of the second dashboard", `(s)=>!!document.querySelector(s)`, dashIcon(id2))
	if errs := page.MustEval(`()=>hdErrors.join('\n')`).Str(); errs != "" {
		t.Fatalf("page errors:\n%s", errs)
	}

	if os.Getenv("HOMEDASH_SHORTCUT_SHOTS") != "1" {
		return
	}
	// Screenshots for the visual review: three themes, same data. Wohnzimmer is pinned, Garage is not.
	v.page.Close()
	if w := env.do(t, env.admin, http.MethodDelete, fmt.Sprintf("dashboard?id=%s&revision=%d", id2, revisionOf(id2)), nil); w.Code != http.StatusOK {
		t.Fatalf("cleanup delete = %d", w.Code)
	}
	if err := os.MkdirAll(homeDashShortcutShotsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pinned := createDashboard("Wohnzimmer")
	createDashboard("Garage")
	postShortcut(map[string]any{"dashboard_id": pinned})
	for _, theme := range []struct {
		name  string
		extra map[string]string
	}{{"standard", nil}, {"fruity", map[string]string{"appearance.fruity_mode": "light"}}, {"organic", nil}} {
		setDesktop(theme.name, theme.extra)
		sv := openDesktop()
		sp, su := sv.page, sv.until
		shot := func(scene string) {
			t.Helper()
			time.Sleep(250 * time.Millisecond)
			sp.MustScreenshot(filepath.Join(homeDashShortcutShotsDir, theme.name+"-"+scene+".png"))
		}
		sp.MustEval(`()=>document.activeElement&&document.activeElement.blur()`)
		su(15*time.Second, "the icon", `(s)=>!!document.querySelector(s)`, dashIcon(pinned))
		sp.Mouse.MustMoveTo(640, 700)
		shot("desktop-icon")
		if err := sp.MustElement(dashIcon(pinned)).Click(proto.InputMouseButtonRight, 1); err != nil {
			t.Fatal(err)
		}
		su(5*time.Second, "the context menu", `()=>!!document.querySelector('.vd-context-menu .vd-context-item')`)
		shot("icon-context-menu")
		sp.MustEval(`()=>document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape'}))`)
		su(5*time.Second, "the closed context menu", `()=>!document.querySelector('.vd-context-menu')`)
		if err := sp.MustElement(dashIcon(pinned)).Click(proto.InputMouseButtonLeft, 2); err != nil {
			t.Fatal(err)
		}
		su(20*time.Second, "the dashboard view", `()=>!!document.querySelector('.vd-window[data-app-id="homedash"] [data-hdx-view="dashboard"]')`)
		// A global-menu theme (fruity) moves the menu bar out of the window into the top bar; the selector finds it in both places.
		menuBtn := sp.Timeout(15 * time.Second).MustElement(`.vd-window-menu-button[data-window-menu="homedash-dashboard"]`)
		menuBtn.MustClick()
		su(5*time.Second, "the Dashboard menu", `()=>!!document.querySelector('.vd-window-menu.open')`)
		shot("dashboard-menu")
		sp.MustEval(`()=>document.body.click()`)
		sp.MustElement(`.vd-window[data-app-id="homedash"] .hdx-back`).MustClick()
		su(15*time.Second, "the card list", `()=>document.querySelectorAll('.hdx-card-pin').length===2`)
		sp.Mouse.MustMoveTo(5, 5)
		shot("home-list")
		sp.MustElement(`.hdx-home .hdx-card-pin[aria-pressed="true"]`).MustHover()
		shot("home-list-pin-hover")
		sp.MustElement(`.hdx-home .hdx-card-pin[aria-pressed="false"]`).MustClick()
		su(10*time.Second, "the toast", `()=>!!document.querySelector('.hdx-toasts .hd-toast')`)
		shot("home-toast")
		sp.MustElement(`.hdx-home .hdx-card-pin[aria-pressed="true"]:not([data-id="` + pinned + `"])`).MustClick()
		su(10*time.Second, "the unpinned card", `()=>document.querySelectorAll('.hdx-card-pin[aria-pressed="true"]').length===1`)
		sp.Close()
	}
}
