package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	retroGamesTestBrowserFS = "https://archive.org/services/emularity/engine/browserfs.min.js"
	retroGamesTestLoader    = "https://archive.org/services/emularity/config/loader.js"
)

var (
	retroGamesTestNonceSource = regexp.MustCompile(`'nonce-([^']*)'`)
	retroGamesTestScriptTag   = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
	retroGamesTestURLAttr     = regexp.MustCompile(`(?i)\b(src|href|action|poster|srcset|data|formaction)\s*=\s*"([^"]*)"`)
	retroGamesTestSoundPill   = regexp.MustCompile(`(?s)<div id="rg-sound"[^>]*>([^<]*)</div>`)
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

// retroGamesTestNonce returns the nonce of the response's CSP and checks that
// the header is exactly the pinned policy with that nonce.
func retroGamesTestNonce(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	csp := w.Header().Get("Content-Security-Policy")
	m := retroGamesTestNonceSource.FindStringSubmatch(csp)
	if m == nil {
		t.Fatalf("CSP without a nonce: %q", csp)
	}
	nonce := m[1]
	if want := fmt.Sprintf(retroGamesPlayerCSP, nonce); csp != want {
		t.Fatalf("CSP:\n got %q\nwant %q", csp, want)
	}
	raw, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil || len(raw) != 16 {
		t.Fatalf("nonce %q is not 16 base64 bytes: %d %v", nonce, len(raw), err)
	}
	return nonce
}

type retroGamesTestScript struct {
	src   string
	nonce string
	at    int
}

func retroGamesTestScripts(page string) []retroGamesTestScript {
	var scripts []retroGamesTestScript
	attr := func(attrs, name string) (string, bool) {
		m := regexp.MustCompile(`(?i)\b` + name + `="([^"]*)"`).FindStringSubmatch(attrs)
		if m == nil {
			return "", false
		}
		return m[1], true
	}
	for _, loc := range retroGamesTestScriptTag.FindAllStringSubmatchIndex(page, -1) {
		attrs := page[loc[2]:loc[3]]
		src, _ := attr(attrs, "src")
		nonce, _ := attr(attrs, "nonce")
		scripts = append(scripts, retroGamesTestScript{src: src, nonce: nonce, at: loc[0]})
	}
	return scripts
}

func TestRetroGamesPlayerPageHeadersAndNonce(t *testing.T) {
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
	nonce := retroGamesTestNonce(t, w)
	inline := 0
	for _, s := range retroGamesTestScripts(page) {
		if s.src != "" {
			continue
		}
		inline++
		if s.nonce != nonce {
			t.Errorf("inline script at %d has nonce %q, want %q", s.at, s.nonce, nonce)
		}
	}
	if inline != 2 {
		t.Fatalf("inline scripts = %d, want 2 (bootstrap and start)", inline)
	}
	if strings.Count(page, `nonce="`+nonce+`"`) != inline {
		t.Fatalf("the nonce appears %d times, want once per inline script", strings.Count(page, `nonce="`+nonce+`"`))
	}

	w2, _ := h.playerPage(params)
	if other := retroGamesTestNonce(t, w2); other == nonce {
		t.Fatalf("two responses share the nonce %q", nonce)
	}
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

	var external []string
	for _, s := range scripts {
		if s.src != "" {
			external = append(external, s.src)
		}
	}
	if strings.Join(external, " ") != retroGamesTestBrowserFS+" "+retroGamesTestLoader {
		t.Fatalf("external scripts = %q, want BrowserFS then loader.js", external)
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
	for _, forbidden := range []string{"/api/", "/js/", "/css/", "url(", "@import", "location"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("page contains %q", forbidden)
		}
	}
	for _, want := range []string{
		`<canvas id="canvas" tabindex="0">`,
		"width:100% !important",
		"height:100% !important",
		"object-fit:contain",
		"image-rendering:pixelated",
		".emularity-splash-screen",
		".emularity-splash-title",
		".emularity-progress-indicator",
		"new IALoader(canvas, id, { before_run:",
		"hasCustomCSS: true",
		"setMute(",
		"MutationObserver",
		"'Failed'",
		"event.source !== window.parent",
		"data.rg !== 1",
		"rg: 1, type: 'state'",
		"rg: 1, type: 'audio'",
		"DOMContentLoaded",
		"'pointerdown'",
		"'keydown'",
		"AudioContext",
		"webkitAudioContext",
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
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/html; charset=utf-8" || !strings.HasPrefix(w.Header().Get("Content-Security-Policy"), "sandbox allow-scripts allow-pointer-lock;") {
		t.Fatalf("HEAD: %d %v", w.Code, w.Header())
	}
	if w, body := h.do(http.MethodPost, path, h.writeToken); w.Code != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST: %d allow=%q %v", w.Code, w.Header().Get("Allow"), body)
	}
	if w, body := h.do(http.MethodGet, "player/arcade_tron", h.readToken); w.Code != http.StatusNotFound || body["code"] != "not_found" {
		t.Fatalf("player/<id>: %d %v", w.Code, body)
	}
}

// TestRetroGamesPlayerCSPIsolation pins what keeps archive.org's code away
// from AuraGo: an opaque origin, no inline or eval'd script besides the
// nonce'd ones, and network access to archive.org hosts only.
func TestRetroGamesPlayerCSPIsolation(t *testing.T) {
	directives := map[string][]string{}
	for _, part := range strings.Split(fmt.Sprintf(retroGamesPlayerCSP, "TESTNONCE"), ";") {
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
	for _, token := range []string{"'unsafe-inline'", "'unsafe-eval'", "'self'", "'strict-dynamic'", "*", "data:", "blob:", "https:", "http:"} {
		if has("script-src", token) {
			t.Errorf("script-src allows %s", token)
		}
	}
	if !has("script-src", "'nonce-TESTNONCE'") {
		t.Errorf("script-src lacks the nonce")
	}
	if has("connect-src", "'self'") || has("connect-src", "*") || has("connect-src", "https:") {
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
	retroGamesTestNonce(t, w)
	if refused := serve("../x"); refused.Code != http.StatusBadRequest || refused.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("refusal through the middleware: %d %v", refused.Code, refused.Header())
	}
}
