package server

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const (
	retroGamesTestBrowserFS = "https://archive.org/services/emularity/engine/browserfs.min.js"
	retroGamesTestLoader    = "https://archive.org/services/emularity/config/loader.js"
)

var (
	retroGamesTestScriptElement = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	retroGamesTestURLAttr       = regexp.MustCompile(`(?i)\b(src|href|action|poster|srcset|data|formaction)\s*=\s*"([^"]*)"`)
	retroGamesTestSoundPill     = regexp.MustCompile(`(?s)<div id="rg-sound"[^>]*>([^<]*)</div>`)
)

func retroGamesPlayerPath(params url.Values) string {
	return "player?" + params.Encode()
}

// playerPage fetches the player page and fails the test unless it is a 200
// HTML answer.
func (h *retroGamesHarness) playerPage(params url.Values) (*httptest.ResponseRecorder, string) {
	h.t.Helper()
	w, body := h.do(http.MethodGet, retroGamesPlayerPath(params), h.readToken)
	if w.Code != http.StatusOK {
		h.t.Fatalf("GET player %v: %d %v %s", params, w.Code, body, w.Body.String())
	}
	return w, w.Body.String()
}

type retroGamesTestScript struct {
	attrs string
	src   string
	body  string
	at    int
}

func retroGamesTestScripts(page string) []retroGamesTestScript {
	var scripts []retroGamesTestScript
	for _, loc := range retroGamesTestScriptElement.FindAllStringSubmatchIndex(page, -1) {
		attrs := page[loc[2]:loc[3]]
		src := ""
		if m := regexp.MustCompile(`(?i)\bsrc="([^"]*)"`).FindStringSubmatch(attrs); m != nil {
			src = m[1]
		}
		scripts = append(scripts, retroGamesTestScript{attrs: attrs, src: src, body: page[loc[4]:loc[5]], at: loc[0]})
	}
	return scripts
}

func retroGamesTestHashSource(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// retroGamesTestCSP checks that the response's CSP is the pinned policy and
// that its script hashes are those of the inline scripts the page serves.
func retroGamesTestCSP(t *testing.T, w *httptest.ResponseRecorder, page string) {
	t.Helper()
	csp := w.Header().Get("Content-Security-Policy")
	if csp != retroGamesPlayerCSP {
		t.Fatalf("CSP:\n got %q\nwant %q", csp, retroGamesPlayerCSP)
	}
	var hashes []string
	for _, s := range retroGamesTestScripts(page) {
		if s.src == "" {
			hashes = append(hashes, retroGamesTestHashSource(s.body))
		}
	}
	if want := fmt.Sprintf(retroGamesPlayerCSPFormat, strings.Join(hashes, " ")); csp != want {
		t.Fatalf("the CSP's script hashes do not match the served inline scripts:\n got %q\nwant %q", csp, want)
	}
}

func TestRetroGamesPlayerPageHeadersAndScriptHashes(t *testing.T) {
	h := newRetroGamesHarness(t)
	params := url.Values{"id": {"msdos_Prince_of_Persia_1990"}}
	w, page := h.playerPage(params)
	for name, want := range map[string]string{
		"Content-Type":                 "text/html; charset=utf-8",
		"Cache-Control":                "no-store",
		"Referrer-Policy":              "no-referrer",
		"X-Content-Type-Options":       "nosniff",
		"Cross-Origin-Resource-Policy": "same-origin",
		"X-Frame-Options":              "SAMEORIGIN",
	} {
		if got := w.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	retroGamesTestCSP(t, w, page)

	// The inline scripts are the static constants, byte for byte: no template
	// value reaches them, so their hashes never change per response.
	var inline []string
	for _, s := range retroGamesTestScripts(page) {
		if s.src == "" {
			inline = append(inline, s.body)
			if strings.TrimSpace(s.attrs) != "" {
				t.Errorf("inline script with attributes %q", s.attrs)
			}
		}
	}
	if len(inline) != 2 || inline[0] != retroGamesPlayerBootstrap || inline[1] != retroGamesPlayerStart {
		t.Fatalf("inline scripts are not the static bootstrap and start scripts: %d", len(inline))
	}
	for _, script := range []string{retroGamesPlayerBootstrap, retroGamesPlayerStart} {
		if strings.Contains(strings.ToLower(script), "</script") || strings.Contains(script, "{{") {
			t.Fatalf("an inline script must not contain </script or template actions")
		}
	}
	if strings.Contains(page, "nonce") {
		t.Fatalf("the page still carries a nonce")
	}

	w2, page2 := h.playerPage(url.Values{"id": {"arcade_tron"}, "muted": {"1"}, "sound": {"Klick für Ton"}})
	retroGamesTestCSP(t, w2, page2)
}

func TestRetroGamesPlayerPageStructure(t *testing.T) {
	h := newRetroGamesHarness(t)
	_, page := h.playerPage(url.Values{"id": {"arcade_tron"}})

	const base = `<base href="https://archive.org/">`
	head := strings.Index(page, "<head>")
	if head < 0 || !strings.HasPrefix(strings.TrimLeft(page[head+len("<head>"):], " \r\n\t"), base) {
		t.Fatalf("<base> is not the first element of <head>:\n%s", page)
	}
	scripts := retroGamesTestScripts(page)
	if len(scripts) == 0 || strings.Index(page, base) > scripts[0].at {
		t.Fatalf("<base> must come before every script")
	}
	if !strings.Contains(page, "<html>") || strings.Contains(page, "lang=") {
		t.Errorf("<html> must not claim a language")
	}

	// Bootstrap, BrowserFS, loader.js, start.
	if len(scripts) != 4 || scripts[0].src != "" || scripts[1].src != retroGamesTestBrowserFS || scripts[2].src != retroGamesTestLoader || scripts[3].src != "" {
		t.Fatalf("script order = %+v", scripts)
	}

	// Self-contained: the only URLs are archive.org's, nothing on the AuraGo
	// host (relative URLs would resolve against <base> anyway).
	for _, m := range retroGamesTestURLAttr.FindAllStringSubmatch(page, -1) {
		switch m[2] {
		case "https://archive.org/", retroGamesTestBrowserFS, retroGamesTestLoader:
		default:
			t.Errorf("unexpected URL attribute %s=%q", m[1], m[2])
		}
	}
	for _, forbidden := range []string{
		"/api/", "/js/", "/css/", "url(", "@import",
		"location",   // the page trusts only server-rendered values
		"object-fit", // the canvas box itself is sized to the picture
		".setMute(",  // archive.org's mute falls back to eval; the page mutes with a gain
	} {
		if strings.Contains(page, forbidden) {
			t.Errorf("page contains %q", forbidden)
		}
	}

	// The bootstrap installs the hooks before archive.org's scripts run.
	for _, want := range []string{
		"AudioContext", "webkitAudioContext", "'statechange'",
		"AudioNode.prototype.connect", "AudioNode.prototype.disconnect", "AudioDestinationNode", "createGain",
		"instanceof HTMLScriptElement", "'securitypolicyviolation'", "'script-src-elem'", "'worker-src'",
	} {
		if !strings.Contains(retroGamesPlayerBootstrap, want) {
			t.Errorf("bootstrap lacks %q", want)
		}
	}
	for _, want := range []string{
		"new IALoader(canvas, id, { before_run:",
		"hasCustomCSS: true",
		"MutationObserver",
		"/^(Failed|Invalid media|Non-system disk)/",
		"event.source !== window.parent",
		"data.rg !== 1",
		"rg: 1, type: 'state'",
		"rg: 1, type: 'audio'",
		"Promise.allSettled",
		"DOMContentLoaded",
		"'pointerdown'",
		"'keydown'",
		"ResizeObserver",
		"'important'",
	} {
		if !strings.Contains(retroGamesPlayerStart, want) {
			t.Errorf("start script lacks %q", want)
		}
	}
	for _, want := range []string{
		`<canvas id="canvas" tabindex="0">`,
		"image-rendering:pixelated",
		".emularity-splash-screen",
		".emularity-splash-title",
		".emularity-progress-indicator",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if !regexp.MustCompile(`<div id="rg-sound"[^>]*\bhidden\b[^>]*>`).MatchString(page) {
		t.Errorf("the sound pill must start hidden")
	}
}

func TestRetroGamesPlayerPageIdentifierAndMuted(t *testing.T) {
	h := newRetroGamesHarness(t)
	_, page := h.playerPage(url.Values{"id": {"Some.Game_v1-2"}, "muted": {"1"}})
	if !strings.Contains(page, `<meta name="rg-id" content="Some.Game_v1-2">`) {
		t.Fatalf("identifier meta missing:\n%s", page)
	}
	if !strings.Contains(page, `<meta name="rg-muted" content="1">`) {
		t.Fatalf("muted=1 did not reach the page")
	}
	for _, muted := range []string{"", "0", "true", "yes", "11"} {
		_, page := h.playerPage(url.Values{"id": {"arcade_tron"}, "muted": {muted}})
		if !strings.Contains(page, `<meta name="rg-muted" content="0">`) {
			t.Fatalf("muted=%q must start unmuted", muted)
		}
	}
}

func TestRetroGamesPlayerRejectsInvalidIdentifiers(t *testing.T) {
	h := newRetroGamesHarness(t)
	for _, id := range []string{"", ".hidden", "-x", "../x", "a/b", "a b", `a"b`, "<script>", strings.Repeat("a", 101)} {
		w, body := h.do(http.MethodGet, retroGamesPlayerPath(url.Values{"id": {id}}), h.readToken)
		if w.Code != http.StatusBadRequest || body["code"] != "invalid_identifier" {
			t.Fatalf("id %q: %d %v", id, w.Code, body)
		}
	}
	if w, body := h.do(http.MethodGet, "player", h.readToken); w.Code != http.StatusBadRequest || body["code"] != "invalid_identifier" {
		t.Fatalf("no id: %d %v", w.Code, body)
	}
}

func TestRetroGamesPlayerSoundLabel(t *testing.T) {
	h := newRetroGamesHarness(t)
	pill := func(sound string, set bool) string {
		t.Helper()
		params := url.Values{"id": {"arcade_tron"}}
		if set {
			params.Set("sound", sound)
		}
		_, page := h.playerPage(params)
		m := retroGamesTestSoundPill.FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("sound %q: no pill text in\n%s", sound, page)
		}
		return m[1]
	}
	if got := pill("", false); got != "Click for sound" {
		t.Fatalf("default label = %q", got)
	}
	if got := pill(" \t\r\n ", true); got != "Click for sound" {
		t.Fatalf("blank label = %q", got)
	}
	if got := pill("Klick für Ton", true); got != "Klick für Ton" {
		t.Fatalf("label = %q", got)
	}
	if got := pill("<b>bold</b> & \"q\"", true); got != "&lt;b&gt;bold&lt;/b&gt; &amp; &#34;q&#34;" {
		t.Fatalf("label was not escaped: %q", got)
	}
	if got := pill("  So\x00und\x1b \x7fon\u0085  ", true); got != "Sound on" {
		t.Fatalf("control characters were not stripped: %q", got)
	}
	long := pill("  "+strings.Repeat("é", 70)+"  ", true)
	if utf8.RuneCountInString(long) != 60 || strings.Trim(long, "é") != "" {
		t.Fatalf("label was not capped at 60 runes: %d %q", utf8.RuneCountInString(long), long)
	}
}

func TestRetroGamesPlayerGates(t *testing.T) {
	h := newRetroGamesHarness(t)
	path := retroGamesPlayerPath(url.Values{"id": {"arcade_tron"}})

	h.s.Cfg.RetroGames.Enabled = false
	if w, body := h.do(http.MethodGet, path, h.readToken); w.Code != http.StatusServiceUnavailable || body["code"] != "disabled" {
		t.Fatalf("disabled: %d %v", w.Code, body)
	}
	h.s.Cfg.RetroGames.Enabled = true
	h.s.Cfg.VirtualDesktop.Enabled = false
	if w, body := h.do(http.MethodGet, path, h.readToken); w.Code != http.StatusServiceUnavailable || body["code"] != "desktop_unavailable" {
		t.Fatalf("desktop off: %d %v", w.Code, body)
	}
	h.s.Cfg.VirtualDesktop.Enabled = true

	// The same refusals as the other read routes.
	for _, credential := range []string{"", "not-a-token"} {
		player, playerBody := h.doAs(http.MethodGet, path, credential)
		status, statusBody := h.doAs(http.MethodGet, "status", credential)
		if player.Code != status.Code || player.Code < 400 || fmt.Sprint(playerBody) != fmt.Sprint(statusBody) {
			t.Fatalf("credential %q: player %d %v, status %d %v", credential, player.Code, playerBody, status.Code, statusBody)
		}
		if player.Header().Get("Content-Security-Policy") != "" {
			t.Fatalf("credential %q: a refusal must not look like the page", credential)
		}
	}
	if w, _ := h.doAs(http.MethodGet, path, "session"); w.Code != http.StatusOK {
		t.Fatalf("session: %d", w.Code)
	}

	w, _ := h.do(http.MethodHead, path, h.readToken)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/html; charset=utf-8" || w.Header().Get("Content-Security-Policy") != retroGamesPlayerCSP {
		t.Fatalf("HEAD: %d %v", w.Code, w.Header())
	}
	if w, body := h.do(http.MethodPost, path, h.writeToken); w.Code != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST: %d allow=%q %v", w.Code, w.Header().Get("Allow"), body)
	}
	if w, body := h.do(http.MethodGet, "player/arcade_tron", h.readToken); w.Code != http.StatusNotFound || body["code"] != "not_found" {
		t.Fatalf("player/<id>: %d %v", w.Code, body)
	}
}

// TestRetroGamesPlayerCSPIsolation pins what protects AuraGo's origin from
// archive.org's code: an opaque origin (no access to AuraGo's cookies,
// storage, DOM or API responses), script only from the two hashed static
// inline scripts and archive.org, and connections to archive.org hosts only.
// It does not make the page's own content secret, and it does not stop
// archive.org's code (same realm as the inline scripts) from posting any
// {rg: 1} message to the app or navigating the frame itself.
func TestRetroGamesPlayerCSPIsolation(t *testing.T) {
	directives := map[string][]string{}
	for _, part := range strings.Split(retroGamesPlayerCSP, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		name := strings.ToLower(fields[0])
		if _, dup := directives[name]; dup {
			t.Fatalf("duplicate directive %q", name)
		}
		directives[name] = fields[1:]
	}
	has := func(directive, token string) bool {
		for _, v := range directives[directive] {
			if strings.EqualFold(v, token) {
				return true
			}
		}
		return false
	}
	if got := strings.Join(directives["sandbox"], " "); got != "allow-scripts allow-pointer-lock" || has("sandbox", "allow-same-origin") {
		t.Fatalf("sandbox = %q", got)
	}
	for _, token := range []string{"'unsafe-inline'", "'unsafe-eval'", "'unsafe-hashes'", "'strict-dynamic'", "*", "data:", "blob:", "https:", "http:"} {
		if has("script-src", token) {
			t.Errorf("script-src allows %s", token)
		}
	}
	hashes := 0
	for _, v := range directives["script-src"] {
		if strings.HasPrefix(v, "'nonce-") {
			t.Errorf("script-src has a nonce: %s", v)
		}
		if strings.HasPrefix(v, "'sha256-") {
			hashes++
		}
	}
	if hashes != 2 || !has("script-src", retroGamesTestHashSource(retroGamesPlayerBootstrap)) || !has("script-src", retroGamesTestHashSource(retroGamesPlayerStart)) {
		t.Errorf("script-src must allow exactly the two inline scripts by hash: %q", directives["script-src"])
	}
	// Only frame-ancestors may name AuraGo's origin.
	for directive := range directives {
		if directive != "frame-ancestors" && has(directive, "'self'") {
			t.Errorf("%s allows 'self'", directive)
		}
	}
	if has("connect-src", "*") || has("connect-src", "https:") {
		t.Errorf("connect-src = %q", directives["connect-src"])
	}
	if got := strings.Join(directives["default-src"], " "); got != "'none'" {
		t.Errorf("default-src = %q", got)
	}
	for _, directive := range []string{"form-action", "object-src"} {
		if got := strings.Join(directives[directive], " "); got != "'none'" {
			t.Errorf("%s = %q", directive, got)
		}
	}
	if got := strings.Join(directives["frame-ancestors"], " "); got != "'self'" {
		t.Errorf("frame-ancestors = %q", got)
	}
	// Every host the page may load code from or talk to is archive.org's.
	for directive, values := range directives {
		for _, v := range values {
			if !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "http://") {
				continue
			}
			u, err := url.Parse(strings.Replace(v, "*.", "wildcard.", 1))
			if err != nil || u.Scheme != "https" || (u.Host != "archive.org" && !strings.HasSuffix(u.Host, ".archive.org")) {
				t.Errorf("%s allows %q", directive, v)
			}
		}
	}
}

// TestRetroGamesPlayerFramingSurvivesSecurityHeaders runs the route behind the
// global security middleware: it sends X-Frame-Options: DENY and its own CSP for
// every path, which the page answer must replace so the desktop can frame it.
// Refusals keep the middleware's headers.
func TestRetroGamesPlayerFramingSurvivesSecurityHeaders(t *testing.T) {
	h := newRetroGamesHarness(t)
	handler := securityHeadersMiddleware(http.HandlerFunc(h.s.handleRetroGames), false, false)
	serve := func(id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, retroGamesPrefix+retroGamesPlayerPath(url.Values{"id": {id}}), nil)
		r.Header.Set("Authorization", "Bearer "+h.readToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := serve("arcade_tron")
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("page through the middleware: %d %v", w.Code, w.Header())
	}
	retroGamesTestCSP(t, w, w.Body.String())
	if refused := serve("../x"); refused.Code != http.StatusBadRequest || refused.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("refusal through the middleware: %d %v", refused.Code, refused.Header())
	}
}

// TestRetroGamesPlayerSessionThroughAuthMiddleware follows the real iframe
// navigation: the desktop's session cookie, through authMiddleware and the
// security headers, gets the framable page; without it the refusal stays
// unframable.
func TestRetroGamesPlayerSessionThroughAuthMiddleware(t *testing.T) {
	h := newRetroGamesHarness(t)
	mux := http.NewServeMux()
	registerRetroGamesRoutes(mux, h.s)
	handler := securityHeadersMiddleware(authMiddleware(h.s, mux), false, false)
	serve := func(session bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, retroGamesPrefix+retroGamesPlayerPath(url.Values{"id": {"arcade_tron"}, "muted": {"1"}}), nil)
		r.Header.Set("Sec-Fetch-Dest", "iframe")
		r.Header.Set("Sec-Fetch-Mode", "navigate")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("Referer", "http://example.com/desktop")
		if session {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(h.s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := serve(true)
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("session navigation: %d %v", w.Code, w.Header())
	}
	retroGamesTestCSP(t, w, w.Body.String())
	if !strings.Contains(w.Body.String(), `<meta name="rg-muted" content="1">`) {
		t.Fatalf("session navigation did not render the page")
	}
	anonymous := serve(false)
	if anonymous.Code != http.StatusUnauthorized || anonymous.Header().Get("X-Frame-Options") != "DENY" || anonymous.Header().Get("Content-Security-Policy") == retroGamesPlayerCSP {
		t.Fatalf("anonymous navigation: %d %v", anonymous.Code, anonymous.Header())
	}
}
