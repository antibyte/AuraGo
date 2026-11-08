package server

// The Storymaker player host page (GET|HEAD /api/desktop/storymaker/player) is the document of the
// sandboxed iframe in the Storymaker app. The page has no inline script or style; its two assets
// are named in the CSP, which has no unsafe-inline and no unsafe-eval, and connect-src is 'none':
// code scenes run in workers that cannot reach any network. The sandbox directive has no
// allow-same-origin, so the page has an opaque origin and cannot read the desktop's cookies,
// storage or API.

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// storymakerPlayerHostPattern is the host form the page accepts; the host ends up in a header and
// in markup.
var storymakerPlayerHostPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]{1,5})?$|^\[[0-9A-Fa-f:.]+\](:[0-9]{1,5})?$`)

// StorymakerPlayerCSP is the Content-Security-Policy of the player host page for an origin (spec
// "User interface", Player iframe).
func StorymakerPlayerCSP(origin string) string {
	return "sandbox allow-scripts; default-src 'none'; script-src " + origin + "/js/storymaker/ " + origin + "/js/vendor/storymaker/runtime.js blob:; worker-src blob:; " +
		"img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src " + origin + "/css/storymaker-player.css; " +
		"form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"
}

// StorymakerPlayerPage is the markup of the player host page (contract section 6).
func StorymakerPlayerPage(origin string) string {
	o := html.EscapeString(origin)
	return `<!doctype html><html data-sm-mode="app" data-sm-base="` + o + `"><head><meta charset="utf-8">` +
		`<link rel="stylesheet" href="` + o + `/css/storymaker-player.css"></head>` +
		`<body><canvas id="sm-stage"></canvas><script src="` + o + `/js/storymaker/host.bundle.js"></script></body></html>`
}

// storymakerRequestOrigin returns the origin the browser used for this request: the scheme from
// TLS or a trusted proxy header, the host from the request. The postMessage target of the iframe is
// this origin, so it must be what the browser sees.
func storymakerRequestOrigin(r *http.Request) (string, bool) {
	host := strings.TrimSpace(r.Host)
	if host == "" || !storymakerPlayerHostPattern.MatchString(host) {
		return "", false
	}
	scheme := "http"
	if r.TLS != nil || trustedForwardedRequest(r) && strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: host}).String(), true
}

func handleStorymakerPlayer(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeStorymakerMethodError(w, "GET, HEAD")
		return
	}
	// The page itself needs no workspace; it needs the feature to be on and a signed-in desktop.
	if _, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false); !ok {
		return
	}
	origin, ok := storymakerRequestOrigin(r)
	if !ok {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_host", "The request has no usable Host header.", nil)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", StorymakerPlayerCSP(origin))
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	// securityHeadersMiddleware leaves X-Frame-Options off for this path (allowDesktopIframe); the
	// CSP's frame-ancestors 'self' is the rule, and this header says the same to older browsers.
	h.Set("X-Frame-Options", "SAMEORIGIN")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(StorymakerPlayerPage(origin)))
	}
}
