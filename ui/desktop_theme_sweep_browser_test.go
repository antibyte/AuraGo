package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// The theme sweep drives the real desktop shell through every registry theme at a wide and a narrow
// viewport and checks what the registry promises: the published body attributes, the shell layout, the
// colour scheme, the reachable windows and the Settings theme picker. It builds the shell CSS and scripts
// from the part lists in scripts/build-ui-bundles.js on every request, so a stale generated bundle can
// never hide a regression.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopTheme -timeout 30m
//	AURAGO_THEME_SWEEP=fruity,crazy limits the sweep to some registry ids (Fruity runs in both modes).
//
// Screenshots land in reports/desktop-theme-sweep/<id>[-light|-dark]-<width>.png.

// themeSweepSeam lists the shell functions exposed to the test as window.auditTest.
var themeSweepSeam = []string{
	"state", "api", "openApp", "closeWindow", "focusWindow", "applyDesktopSettings", "refreshSpacesForViewport",
	"renderIcons", "renderStartApps", "renderStartButtonIcon", "renderTaskbar",
	"desktopThemes", "desktopThemeById", "currentDesktopTheme", "settingValue",
	"applyWindowSnap", "toggleMaximizeWindow", "minimizeWindow", "switchSpace", "modalDialog",
}

// themeSweepWideMinWidth is the first viewport width that is not a compact layout.
const themeSweepWideMinWidth = 821

// themeSweepNetworkQuiet is how long the booted fixture desktop has to stay off the network before a session is
// handed out. The shell hydrates the pet catalog and the settings from the server on boot, and the pet runtime
// polls for the bootstrap every 100 ms and hydrates again when it has it, so the last boot request follows
// the one before within about 100 ms.
const themeSweepNetworkQuiet = 500 * time.Millisecond

// ---------------------------------------------------------------------------------------------------
// Settings API double.

// themeSweepStore answers the desktop settings API for the browser: it validates PUT bodies like the
// server does, merges them and replies with all settings, as internal/server/desktop_handlers_settings.go.
// Every reply, a GET included, carries the live settings as they are when the reply is built.
type themeSweepStore struct {
	mu       sync.Mutex
	settings map[string]string
	accepted []string // "key=value" of every PUT the page sent and the store took
	rejected []string
}

// themeSweepSeed is the factory settings plus the switches that keep the fixture page quiet.
func themeSweepSeed() map[string]string {
	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{
		"windows.restore_session": "false", "windows.animations": "false", "pet.enabled": "false",
		"phone_gadget.enabled": "false", "desktop.show_widgets": "false",
	} {
		settings[key] = value
	}
	return settings
}

func newThemeSweepStore() *themeSweepStore {
	return &themeSweepStore{settings: themeSweepSeed()}
}

func (s *themeSweepStore) snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make(map[string]string, len(s.settings))
	for key, value := range s.settings {
		copied[key] = value
	}
	return copied
}

func (s *themeSweepStore) value(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings[key]
}

// set writes settings from the test side, without validation.
func (s *themeSweepStore) set(updates map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range updates {
		s.settings[key] = value
	}
}

// reset restores the seed and forgets the PUT history.
func (s *themeSweepStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = themeSweepSeed()
	s.accepted, s.rejected = nil, nil
}

// rejections lists the PUTs the store refused; the shell only shows a toast for those.
func (s *themeSweepStore) rejections() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.rejected...)
}

// writesTo lists the values the page PUT for key, in order.
func (s *themeSweepStore) writesTo(key string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var values []string
	for _, entry := range s.accepted {
		if value, found := strings.CutPrefix(entry, key+"="); found {
			values = append(values, value)
		}
	}
	return values
}

func themeSweepAllowedValues(key string) (values []string, known bool) {
	for _, def := range desktop.DesktopSettingDefinitions() {
		if def.Key == key {
			return def.Values, true
		}
	}
	return nil, false
}

// validate mirrors desktop.validateDesktopSetting for the keys the theme picker writes.
func (s *themeSweepStore) validate(key, value string) error {
	allowed, known := themeSweepAllowedValues(key)
	if !known {
		return fmt.Errorf("unsupported desktop setting %s", key)
	}
	if len(allowed) > 0 {
		if !desktopThemeContains(allowed, value) {
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		return nil
	}
	if key == "appearance.wallpaper_by_space" && strings.TrimSpace(value) != "" {
		var bySpace map[string]string
		if err := json.Unmarshal([]byte(value), &bySpace); err != nil {
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		wallpapers, _ := themeSweepAllowedValues("appearance.wallpaper")
		for space, wallpaper := range bySpace {
			if (space != "1" && space != "2" && space != "3") || !desktopThemeContains(wallpapers, wallpaper) {
				return fmt.Errorf("invalid desktop setting value for %s", key)
			}
		}
	}
	return nil
}

func (s *themeSweepStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, message string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		var body struct {
			Key      string            `json:"key"`
			Value    string            `json:"value"`
			Settings map[string]string `json:"settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(http.StatusBadRequest, "Invalid JSON")
			return
		}
		updates := body.Settings
		if updates == nil {
			updates = map[string]string{body.Key: body.Value}
		}
		for key, value := range updates {
			if err := s.validate(key, value); err != nil {
				s.mu.Lock()
				s.rejected = append(s.rejected, fmt.Sprintf("%s=%q: %v", key, value, err))
				s.mu.Unlock()
				fail(http.StatusBadRequest, err.Error())
				return
			}
		}
		s.set(updates)
		s.mu.Lock()
		for key, value := range updates {
			s.accepted = append(s.accepted, key+"="+value)
		}
		s.mu.Unlock()
	default:
		fail(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": s.snapshot()})
}

// ---------------------------------------------------------------------------------------------------
// Shell assembled from the bundle part lists.

func themeSweepNormalize(text string) string {
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	return strings.TrimRight(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), " \t\n")
}

// themeSweepConcat joins the part files like the bundle script does. It reads the files on every call, so
// an edit shows up on the next page load.
func themeSweepConcat(parts []string, javascript bool) (string, error) {
	var bundle strings.Builder
	for _, part := range parts {
		data, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(part)))
		if err != nil {
			return "", fmt.Errorf("read bundle part %s: %w", part, err)
		}
		if javascript {
			bundle.WriteString("\n;\n/* " + part + " */\n")
		} else {
			bundle.WriteString("\n/* " + part + " */\n")
		}
		bundle.WriteString(themeSweepNormalize(string(data)) + "\n")
	}
	return bundle.String(), nil
}

// themeSweepSpliceSeam puts the window.auditTest seam in front of the startup call of the desktop shell.
func themeSweepSpliceSeam(shell string) (string, error) {
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		return "", fmt.Errorf("startup seam missing: no `    ensureDesktopRadialMenuAnchor();` in the desktop shell")
	}
	for _, name := range themeSweepSeam {
		declared := regexp.MustCompile(`(?m)^\s*(?:(?:async\s+)?function\s+` + name + `\s*\(|(?:const|let|var)\s+` + name + `\b)`)
		if !declared.MatchString(shell) {
			return "", fmt.Errorf("the desktop shell declares no %s for the test seam", name)
		}
	}
	return shell[:cut] + "window.auditTest={" + strings.Join(themeSweepSeam, ",") + "};" + shell[cut:], nil
}

type themeSweepServer struct {
	url   string
	store *themeSweepStore
}

// newThemeSweepServer serves the desktop fixture page and the settings double.
func newThemeSweepServer(t *testing.T) *themeSweepServer {
	t.Helper()
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	words := readDesktopAssetText(t, "lang/desktop/de.json")
	html = strings.Replace(html, "</body>", `<script>window.fixtureWords=`+words+`;window.t=(k,v)=>{let s=fixtureWords[k]||k;for(const [key,value] of Object.entries(v||{}))s=s.replaceAll('{{'+key+'}}',value).replaceAll('{'+key+'}',value);return s;};window.fixtureErrors=[];addEventListener('error',e=>fixtureErrors.push(e.message));addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));window.WebSocket=class extends EventTarget{close(){}};</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/theme-sweep-shell.js"></script></body>`, 1)

	// The part lists come from the bundle script once; the part files are read on every request.
	shellParts := desktopBundleParts(t, "ui/js/desktop/bundles/main.bundle.js")
	styleParts := desktopBundleParts(t, "ui/css/desktop-shell.bundle.css")
	effectParts := desktopBundleParts(t, "ui/js/desktop/bundles/desktop-theme-fx.bundle.js")
	shell := func() (string, error) {
		text, err := themeSweepConcat(shellParts, true)
		if err != nil {
			return "", err
		}
		return themeSweepSpliceSeam(text)
	}
	styles := func() (string, error) { return themeSweepConcat(styleParts, false) }
	effects := func() (string, error) { return themeSweepConcat(effectParts, true) }
	for name, build := range map[string]func() (string, error){"the desktop shell": shell, "the shell styles": styles, "the theme effects": effects} {
		if _, err := build(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	store := newThemeSweepStore()
	serve := func(contentType string, build func() (string, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, err := build()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprint(w, body)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", serve("text/html; charset=utf-8", func() (string, error) { return html, nil }))
	mux.HandleFunc("/theme-sweep-shell.js", serve("text/javascript", shell))
	mux.HandleFunc("/css/desktop-shell.bundle.css", serve("text/css", styles))
	mux.HandleFunc("/js/desktop/bundles/desktop-theme-fx.bundle.js", serve("text/javascript", effects))
	mux.Handle("/api/desktop/settings", store)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/desktop/bootstrap" {
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{}, "widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{}, "workspace": map[string]interface{}{"readonly": false}, "settings": store.snapshot()})
		} else {
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &themeSweepServer{url: server.URL, store: store}
}

// ---------------------------------------------------------------------------------------------------
// Browser session.

type themeSweepSession struct {
	page  *rod.Page
	store *themeSweepStore

	mu      sync.Mutex
	console []string // console and exception messages of the page, for failure reports
}

// newThemeSweepSession opens the fixture desktop in a fresh headless browser. reducedMotion is the emulated
// prefers-reduced-motion value ("reduce" or "no-preference"): the OS animation setting changes the headless
// default, so the media feature is always pinned.
func newThemeSweepSession(t *testing.T, reducedMotion string) *themeSweepSession {
	return newThemeSweepSessionWithBrowser(t, reducedMotion, nil)
}

func newThemeSweepSessionWithBrowser(t *testing.T, reducedMotion string, tweak func(*launcher.Launcher)) *themeSweepSession {
	t.Helper()
	requirePrecisionBrowserSmoke(t)
	server := newThemeSweepServer(t)
	page := newSmokeBrowserWith(t, tweak).MustPage()
	t.Cleanup(func() { _ = page.Close() })
	s := &themeSweepSession{page: page, store: server.store}
	if err := (proto.RuntimeEnable{}).Call(page); err != nil {
		t.Fatalf("enable the runtime events: %v", err)
	}
	go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		var parts []string
		for _, arg := range e.Args {
			text := arg.Value.String()
			if arg.Value.Nil() || text == "" {
				text = arg.Description
			}
			parts = append(parts, text)
		}
		s.logConsole(string(e.Type) + ": " + strings.Join(parts, " "))
	}, func(e *proto.RuntimeExceptionThrown) {
		text := e.ExceptionDetails.Text
		if e.ExceptionDetails.Exception != nil {
			text += " " + e.ExceptionDetails.Exception.Description
		}
		s.logConsole("exception: " + text)
	})()
	s.setViewport(t, 1280)
	media := proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: reducedMotion}}}
	if err := media.Call(page); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	// Armed before the navigation, because rod only counts the requests that start after this call.
	networkIdle := page.WaitRequestIdle(themeSweepNetworkQuiet, nil, nil, nil)
	if err := page.Timeout(60 * time.Second).Navigate(server.url + "/fixture"); err != nil {
		t.Fatalf("open the fixture desktop: %v", err)
	}
	if err := page.Timeout(60 * time.Second).WaitLoad(); err != nil {
		t.Fatalf("load the fixture desktop: %v", err)
	}
	s.waitFor(t, "the desktop shell to start", 30*time.Second, `() => !!(window.auditTest && auditTest.state.ws && auditTest.state.bootstrap && document.body.dataset.theme)`)
	// The boot is not over yet: its settings request may still be in flight, and its answer would land on
	// top of whatever the test pushed in the meantime (see pushSettings).
	s.waitForNetworkIdle(t, networkIdle)
	reduced := reducedMotion == "reduce"
	if got := s.eval(t, `() => matchMedia('(prefers-reduced-motion: reduce)').matches`).Value.Bool(); got != reduced {
		t.Fatalf("emulated prefers-reduced-motion %q, but matchMedia(reduce) is %v", reducedMotion, got)
	}
	return s
}

func (s *themeSweepSession) logConsole(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.console = append(s.console, message)
}

// pageState describes the desktop for a failure report; it never fails the test itself.
func (s *themeSweepSession) pageState() string {
	state := "unavailable"
	if result, err := s.page.Timeout(5 * time.Second).Eval(`() => JSON.stringify({
		theme: document.body.dataset.theme, animations: document.body.dataset.animations,
		storedTheme: window.auditTest ? auditTest.settingValue('appearance.theme') : null,
		fxHost: typeof window.AuraDesktopThemeFx, fxActive: window.AuraDesktopThemeFx ? AuraDesktopThemeFx.activeId() : null,
		fxLayers: document.querySelectorAll('.vd-theme-fx-layer').length,
		fx: window.AuraDesktopThemeFx?.inspect?.(),
		reducedMotion: matchMedia('(prefers-reduced-motion: reduce)').matches,
		windows: window.auditTest ? [...auditTest.state.windows.values()].map(w => w.appId) : null,
		errors: window.fixtureErrors,
		fxResource: performance.getEntriesByType('resource').filter(e => e.name.includes('theme-fx')).map(e => e.name + ' status=' + e.responseStatus + ' ms=' + Math.round(e.duration))
	})`); err == nil {
		state = result.Value.Str()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("page %s, console %q", state, s.console)
}

func (s *themeSweepSession) eval(t testing.TB, js string, args ...interface{}) *proto.RuntimeRemoteObject {
	t.Helper()
	result, err := s.page.Timeout(30*time.Second).Eval(js, args...)
	if err != nil {
		t.Fatalf("browser script failed: %v\n%s", err, js)
	}
	return result
}

func (s *themeSweepSession) evalInto(t testing.TB, out interface{}, js string, args ...interface{}) {
	t.Helper()
	if err := s.eval(t, js, args...).Value.Unmarshal(out); err != nil {
		t.Fatalf("decode the browser script result: %v\n%s", err, js)
	}
}

// waitFor polls a page predicate until it is truthy; it fails the test after timeout.
func (s *themeSweepSession) waitFor(t testing.TB, what string, timeout time.Duration, js string, args ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		result, err := s.page.Timeout(10*time.Second).Eval(js, args...)
		if err == nil && result.Value.Bool() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s (last script error: %v); %s", timeout, what, err, s.pageState())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitForNetworkIdle runs the wait function of Page.WaitRequestIdle, which ends once the page has been off the
// network for themeSweepNetworkQuiet, and fails the test when that takes more than 30 seconds.
func (s *themeSweepSession) waitForNetworkIdle(t testing.TB, idle func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		idle()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("the fixture desktop was still busy on the network 30s after it started; %s", s.pageState())
	}
}

// waitForStore polls the settings double until predicate holds.
func (s *themeSweepSession) waitForStore(t testing.TB, what string, timeout time.Duration, predicate func(*themeSweepStore) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !predicate(s.store) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s; settings now: theme=%q icon_theme=%q wallpaper=%q wallpaper_by_space=%q, rejected PUTs: %v",
				timeout, what, s.store.value("appearance.theme"), s.store.value("appearance.icon_theme"), s.store.value("appearance.wallpaper"), s.store.value("appearance.wallpaper_by_space"), s.store.rejections())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *themeSweepSession) setViewport(t testing.TB, width int) {
	t.Helper()
	if err := s.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: width, Height: 900, DeviceScaleFactor: 1, Mobile: false}); err != nil {
		t.Fatalf("set the %d px viewport: %v", width, err)
	}
}

// settle waits for style and layout to flush after a change.
func (s *themeSweepSession) settle(t testing.TB) {
	t.Helper()
	s.eval(t, `async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); }`)
}

func (s *themeSweepSession) closeAllWindows(t testing.TB) {
	t.Helper()
	s.eval(t, `() => { const a = auditTest; [...a.state.windows.keys()].forEach(id => a.closeWindow(id)); }`)
	s.waitFor(t, "all windows to close", 10*time.Second, `() => auditTest.state.windows.size === 0 && !document.querySelector('.vd-window')`)
}

// pushSettings copies the store into the page settings (in place, so every holder sees them) and re-applies
// them the way a saved setting would: attributes, taskbar, icons and the start menu.
//
// The shell hydrates its settings from GET /api/desktop/settings and merges the answer over
// state.bootstrap.settings whenever it lands, however old the snapshot it carries. The session constructor waits
// for the boot requests, so none is left in flight when a test pushes; copying once more on the next frame
// covers one that still slips in right behind the push.
func (s *themeSweepSession) pushSettings(t testing.TB) {
	t.Helper()
	s.eval(t, `(snapshot) => {
		const a = auditTest;
		const copy = () => {
			const settings = a.state.bootstrap.settings;
			for (const key of Object.keys(settings)) delete settings[key];
			Object.assign(settings, snapshot);
		};
		copy();
		a.applyDesktopSettings();
		a.refreshSpacesForViewport();
		a.renderIcons();
		a.renderStartButtonIcon();
		a.renderStartApps();
		requestAnimationFrame(copy);
	}`, s.store.snapshot())
	s.settle(t)
}

// openApps opens the apps in order and waits until every window has rendered its content.
func (s *themeSweepSession) openApps(t testing.TB, ids ...string) {
	t.Helper()
	s.eval(t, `async (ids) => {
		await Promise.all(ids.map(id => AuraDesktopModules.loadAppAssets(id)));
		ids.forEach(id => auditTest.openApp(id));
	}`, ids)
	s.waitFor(t, fmt.Sprintf("the windows %v to render", ids), 20*time.Second, `(ids) => ids.every(id => {
		const content = document.querySelector('.vd-window[data-app-id="' + id + '"] .vd-window-content');
		return !!content && content.children.length > 0;
	})`, ids)
	s.settle(t)
}

func (s *themeSweepSession) screenshot(t testing.TB, path string) {
	t.Helper()
	time.Sleep(150 * time.Millisecond) // let the wallpaper and icon images paint
	png, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		t.Fatalf("screenshot %s: %v", path, err)
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------------------------------
// The sweep.

type themeSweepCombo struct {
	theme desktopThemeDescriptor
	mode  string // fruity light/dark; empty for themes without a scheme setting
}

func (c themeSweepCombo) label() string {
	if c.mode == "" {
		return c.theme.ID
	}
	return c.theme.ID + "-" + c.mode
}

func (c themeSweepCombo) wantScheme() string {
	if c.theme.SchemeSetting != "" {
		return c.mode
	}
	return c.theme.Scheme
}

func themeSweepCombinations(t *testing.T, themes []desktopThemeDescriptor) []themeSweepCombo {
	t.Helper()
	filter := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("AURAGO_THEME_SWEEP"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			filter[id] = true
		}
	}
	for id := range filter {
		found := false
		for _, theme := range themes {
			found = found || theme.ID == id
		}
		if !found {
			t.Fatalf("AURAGO_THEME_SWEEP lists %q, which is no registry theme", id)
		}
	}
	var combos []themeSweepCombo
	for _, theme := range themes {
		if len(filter) > 0 && !filter[theme.ID] {
			continue
		}
		if theme.SchemeSetting != "" {
			combos = append(combos, themeSweepCombo{theme, "light"}, themeSweepCombo{theme, "dark"})
		} else {
			combos = append(combos, themeSweepCombo{theme, ""})
		}
	}
	if len(combos) == 0 {
		t.Fatal("the theme sweep has no combination to run")
	}
	return combos
}

type themeSweepRect struct {
	Left, Top, Right, Bottom, Width, Height float64
}

type themeSweepMeasure struct {
	Theme       string
	Shell       string
	Scheme      string
	Polish      string
	ColorScheme string
	AppBg       string
	Bar         *struct {
		Hidden bool
		Rect   themeSweepRect
	}
	TaskbarApps bool
	Dock        bool
	ActiveApp   string
	ActiveRect  *themeSweepRect
	WindowApps  []string
	Viewport    struct{ Width, Height float64 }
	CardIDs     []string
	Pressed     []string
	Errors      []string
}

const themeSweepMeasureJS = `() => {
	const a = auditTest, body = document.body, style = getComputedStyle(body);
	const rect = el => { const r = el.getBoundingClientRect(); return { Left: r.left, Top: r.top, Right: r.right, Bottom: r.bottom, Width: r.width, Height: r.height }; };
	const bar = document.getElementById('vd-global-bar');
	const apps = document.getElementById('vd-taskbar-apps');
	const active = a.state.windows.get(a.state.activeWindowId);
	const settings = document.querySelector('.vd-window[data-app-id="settings"]');
	const cards = settings ? [...settings.querySelectorAll('.vd-theme-card')] : [];
	return {
		Theme: body.dataset.theme || '', Shell: body.dataset.shell || '', Scheme: body.dataset.scheme || '', Polish: body.dataset.polish || '',
		ColorScheme: style.colorScheme, AppBg: style.getPropertyValue('--vd-theme-app-bg').trim(),
		Bar: bar ? { Hidden: bar.hidden, Rect: rect(bar) } : null,
		TaskbarApps: !!apps, Dock: !!apps && apps.classList.contains('vd-dock'),
		ActiveApp: active ? active.appId : '', ActiveRect: active ? rect(active.element) : null,
		WindowApps: [...a.state.windows.values()].map(w => w.appId),
		Viewport: { Width: innerWidth, Height: innerHeight },
		CardIDs: cards.map(card => card.dataset.desktopTheme || ''),
		Pressed: cards.filter(card => card.getAttribute('aria-pressed') === 'true').map(card => card.dataset.desktopTheme || ''),
		Errors: window.fixtureErrors.slice()
	};
}`

// themeSweepRealErrors drops the benign ResizeObserver notice browsers raise on layout loops.
func themeSweepRealErrors(errs []string) []string {
	var real []string
	for _, message := range errs {
		if !strings.Contains(message, "ResizeObserver loop") {
			real = append(real, message)
		}
	}
	return real
}

func (s *themeSweepSession) sweep(t *testing.T, combo themeSweepCombo, width int, themes []desktopThemeDescriptor, screenshotDir string) {
	t.Helper()
	theme := combo.theme
	mode := combo.mode
	if mode == "" {
		mode = "light"
	}
	s.closeAllWindows(t)
	s.setViewport(t, width)
	s.store.set(map[string]string{
		"appearance.theme":       theme.ID,
		"appearance.icon_theme":  theme.IconTheme,
		"appearance.fruity_mode": mode,
		"windows.animations":     "false",
	})
	s.pushSettings(t)
	s.openApps(t, "calculator", "settings")
	s.screenshot(t, filepath.Join(screenshotDir, fmt.Sprintf("%s-%d.png", combo.label(), width)))

	var got themeSweepMeasure
	s.evalInto(t, &got, themeSweepMeasureJS)
	barShown := got.Bar != nil && !got.Bar.Hidden
	t.Logf("data-theme=%s data-shell=%s data-scheme=%s data-polish=%s color-scheme=%s app-bg=%s global-bar=%v dock=%v active=%s%+v cards=%d pressed=%v",
		got.Theme, got.Shell, got.Scheme, got.Polish, got.ColorScheme, got.AppBg, barShown, got.Dock, got.ActiveApp, got.ActiveRect, len(got.CardIDs), got.Pressed)
	if got.Theme != theme.ID {
		t.Errorf("body data-theme is %q, want %q", got.Theme, theme.ID)
	}
	if got.Shell != theme.Shell {
		t.Errorf("body data-shell is %q, want %q", got.Shell, theme.Shell)
	}
	if got.Scheme != combo.wantScheme() {
		t.Errorf("body data-scheme is %q, want %q", got.Scheme, combo.wantScheme())
	}
	if want := map[bool]string{true: "on", false: "off"}[theme.GlassPolish]; got.Polish != want {
		t.Errorf("body data-polish is %q, want %q", got.Polish, want)
	}
	if got.ColorScheme != combo.wantScheme() {
		t.Errorf("computed color-scheme is %q, want %q", got.ColorScheme, combo.wantScheme())
	}
	if got.AppBg == "" {
		t.Errorf("--vd-theme-app-bg resolves to nothing on the body")
	}

	wantBar := theme.GlobalMenu && width >= themeSweepWideMinWidth
	switch {
	case wantBar && (got.Bar == nil || got.Bar.Hidden):
		t.Errorf("#vd-global-bar must be present and shown for %s at %d px, got %+v", theme.ID, width, got.Bar)
	case wantBar && (got.Bar.Rect.Width <= 0 || got.Bar.Rect.Height <= 0):
		t.Errorf("#vd-global-bar is not hidden but has no size: %+v", got.Bar.Rect)
	case !wantBar && got.Bar != nil && !got.Bar.Hidden:
		t.Errorf("#vd-global-bar must not show for %s at %d px", theme.ID, width)
	}

	if !got.TaskbarApps {
		t.Errorf("#vd-taskbar-apps is missing")
	} else if got.Dock != (theme.Shell == "dock") {
		t.Errorf("#vd-taskbar-apps has the vd-dock class: %v, want %v for shell %q", got.Dock, theme.Shell == "dock", theme.Shell)
	}

	for _, app := range []string{"calculator", "settings"} {
		if !desktopThemeContains(got.WindowApps, app) {
			t.Errorf("the %s window did not open (windows: %v)", app, got.WindowApps)
		}
	}
	if got.ActiveRect == nil {
		t.Errorf("no active window")
	} else if r, vw, vh := got.ActiveRect, got.Viewport.Width, got.Viewport.Height; r.Width <= 0 || r.Height <= 0 || r.Left < -1 || r.Top < -1 || r.Right > vw+1 || r.Bottom > vh+1 {
		t.Errorf("the active %s window %+v is not inside the %.0fx%.0f viewport", got.ActiveApp, *r, vw, vh)
	}

	var ids []string
	for _, candidate := range themes {
		ids = append(ids, candidate.ID)
	}
	if len(got.CardIDs) != len(themes) {
		t.Errorf("the Settings window shows %d .vd-theme-card elements %v, want %d %v", len(got.CardIDs), got.CardIDs, len(ids), ids)
	}
	if len(got.Pressed) != 1 || got.Pressed[0] != theme.ID {
		t.Errorf("aria-pressed=true is on %v, want exactly [%s]", got.Pressed, theme.ID)
	}
	if errs := themeSweepRealErrors(got.Errors); len(errs) > 0 {
		t.Errorf("page errors: %v", errs)
	}
	s.eval(t, `() => { window.fixtureErrors.length = 0; }`)
}

func TestDesktopThemeSweepBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	themes := parseDesktopThemeRegistry(t)
	combos := themeSweepCombinations(t, themes)
	dir := filepath.Join("..", "reports", "desktop-theme-sweep")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := newThemeSweepSession(t, "no-preference")

	for _, combo := range combos {
		for _, width := range []int{1280, 420} {
			combo, width := combo, width
			t.Run(fmt.Sprintf("%s-%d", combo.label(), width), func(t *testing.T) {
				s.sweep(t, combo, width, themes, dir)
			})
		}
	}

	// The picker flows run on a fresh 1280 px desktop whose only window is Settings. overrides are applied to
	// the factory settings before the page renders them.
	freshSettings := func(t *testing.T, overrides map[string]string) {
		t.Helper()
		s.closeAllWindows(t)
		s.setViewport(t, 1280)
		s.store.reset()
		s.store.set(overrides)
		s.pushSettings(t)
		s.openApps(t, "settings")
		s.waitFor(t, "the theme cards", 10*time.Second, `() => document.querySelectorAll('.vd-window[data-app-id="settings"] .vd-theme-card').length > 0`)
	}
	const cardSelector = `.vd-window[data-app-id="settings"] .vd-theme-card[data-desktop-theme="%s"]`
	card := func(t *testing.T, id string) *rod.Element {
		t.Helper()
		element, err := s.page.Timeout(15 * time.Second).Element(fmt.Sprintf(cardSelector, id))
		if err != nil {
			t.Fatalf("find the %s card: %v", id, err)
		}
		return element
	}
	click := func(t *testing.T, id string) {
		t.Helper()
		element := card(t, id)
		if err := element.ScrollIntoView(); err != nil {
			t.Fatalf("scroll the %s card into view: %v", id, err)
		}
		if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
			t.Fatalf("click the %s card: %v", id, err)
		}
		s.waitFor(t, "the "+id+" theme to apply", 10*time.Second, `(id) => document.body.dataset.theme === id`, id)
	}
	press := func(t *testing.T, id string, key input.Key) {
		t.Helper()
		if err := card(t, id).Focus(); err != nil {
			t.Fatalf("focus the %s card: %v", id, err)
		}
		if err := s.page.Keyboard.Type(key); err != nil {
			t.Fatalf("press the key on the %s card: %v", id, err)
		}
		s.waitFor(t, "the "+id+" theme to apply", 10*time.Second, `(id) => document.body.dataset.theme === id`, id)
		// Saving re-renders Settings and replaces every card, so focus has to be handed back.
		s.waitFor(t, "focus to stay on the "+id+" card", 5*time.Second, `(id) => {
			const el = document.activeElement;
			return !!el && el.matches('.vd-theme-card[data-desktop-theme="' + id + '"]');
		}`, id)
	}
	saved := func(t *testing.T, what string, want map[string]string) {
		t.Helper()
		s.waitForStore(t, what, 10*time.Second, func(store *themeSweepStore) bool {
			for key, value := range want {
				if store.value(key) != value {
					return false
				}
			}
			return true
		})
		if rejected := s.store.rejections(); len(rejected) > 0 {
			t.Errorf("the settings API refused: %v", rejected)
		}
	}

	t.Run("Enter picks a theme, pairs its icons and keeps focus", func(t *testing.T) {
		freshSettings(t, nil)
		press(t, "black_gloss", input.Enter)
		saved(t, "Black Gloss and its WhiteSur icons to be saved", map[string]string{"appearance.theme": "black_gloss", "appearance.icon_theme": "whitesur"})
	})

	t.Run("Space picks a theme and swaps the icons back", func(t *testing.T) {
		freshSettings(t, map[string]string{"appearance.theme": "black_gloss", "appearance.icon_theme": "whitesur"})
		press(t, "cyberpunk", input.Space)
		saved(t, "Cyberpunk and its Papirus icons to be saved", map[string]string{"appearance.theme": "cyberpunk", "appearance.icon_theme": "papirus"})
	})

	t.Run("wallpaper follows an untouched theme", func(t *testing.T) {
		freshSettings(t, nil)
		click(t, "amiga")
		saved(t, "the wallpaper to follow the Amiga theme", map[string]string{"appearance.theme": "amiga", "appearance.wallpaper": "slate", "appearance.wallpaper_by_space": "{}"})
		s.waitFor(t, "the desktop to show the Amiga wallpaper", 5*time.Second, `() => document.body.dataset.wallpaper === 'slate'`)
	})

	t.Run("wallpaper follows from one theme suggestion to the next", func(t *testing.T) {
		freshSettings(t, map[string]string{"appearance.theme": "amiga", "appearance.icon_theme": "papirus", "appearance.wallpaper": "slate"})
		click(t, "cyberpunk")
		saved(t, "the wallpaper to follow from Amiga to Cyberpunk", map[string]string{"appearance.theme": "cyberpunk", "appearance.wallpaper": "city_rain"})
	})

	t.Run("a wallpaper the user picked stays", func(t *testing.T) {
		freshSettings(t, map[string]string{"appearance.wallpaper": "ember"})
		click(t, "crazy")
		saved(t, "the Crazy theme to be saved", map[string]string{"appearance.theme": "crazy"})
		time.Sleep(600 * time.Millisecond) // a wrong hand-over would be a PUT right behind the theme
		if got := s.store.value("appearance.wallpaper"); got != "ember" {
			t.Errorf("the wallpaper became %q, want the user's ember", got)
		}
		if writes := append(s.store.writesTo("appearance.wallpaper"), s.store.writesTo("appearance.wallpaper_by_space")...); len(writes) > 0 {
			t.Errorf("the picker wrote wallpaper settings %v although the user chose the wallpaper", writes)
		}
		if got := s.eval(t, `() => document.body.dataset.wallpaper`).Value.Str(); got != "ember" {
			t.Errorf("the desktop shows the wallpaper %q, want ember", got)
		}
	})
}

// ---------------------------------------------------------------------------------------------------
// Reduced motion.

func TestDesktopCrazyThemeHonoursReducedMotionBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	skin := readDesktopThemeSource(t, "css/desktop-theme-crazy.css")
	skinAnimates := strings.Contains(skin, "@keyframes")

	runs := []struct {
		name          string
		reducedMotion string
		animations    string
		wantEffects   bool
	}{
		{"reduced motion with animations on", "reduce", "true", false},
		{"animations switched off", "no-preference", "false", false},
		{"positive control", "no-preference", "true", true},
	}
	for _, run := range runs {
		run := run
		t.Run(run.name, func(t *testing.T) {
			s := newThemeSweepSession(t, run.reducedMotion)
			s.store.set(map[string]string{"appearance.theme": "crazy", "windows.animations": run.animations})
			s.pushSettings(t)
			s.openApps(t, "calculator")

			if run.wantEffects {
				s.waitFor(t, "the Crazy effects layer (the effects bundle loads lazily)", 20*time.Second,
					`() => !!document.querySelector('.vd-theme-fx-layer') && !!window.AuraDesktopThemeFx && AuraDesktopThemeFx.activeId() === 'crazy'`)
				if skinAnimates {
					s.waitFor(t, "a running animation (the Crazy skin declares @keyframes)", 10*time.Second,
						`() => document.getAnimations().some(animation => animation.playState === 'running')`)
				}
				return
			}

			time.Sleep(1500 * time.Millisecond)
			var got struct {
				Layers   int
				ActiveFx string
				Running  []string
			}
			s.evalInto(t, &got, `() => ({
				Layers: document.querySelectorAll('.vd-theme-fx-layer').length,
				ActiveFx: window.AuraDesktopThemeFx ? AuraDesktopThemeFx.activeId() : '',
				Running: document.getAnimations().filter(animation => animation.playState === 'running').map(animation => {
					const target = animation.effect && animation.effect.target;
					const owner = target ? (target.id ? '#' + target.id : '') + (typeof target.className === 'string' && target.className ? '.' + target.className.trim().split(/\s+/).join('.') : '') || target.tagName : '?';
					return (animation.animationName || animation.transitionProperty || animation.constructor.name) + ' on ' + owner;
				})
			})`)
			if got.Layers != 0 || got.ActiveFx != "" {
				t.Errorf("%d .vd-theme-fx-layer elements (active effect %q), want none", got.Layers, got.ActiveFx)
			}
			if len(got.Running) != 0 {
				t.Errorf("%d animations still run: %v", len(got.Running), got.Running)
			}
		})
	}
}
