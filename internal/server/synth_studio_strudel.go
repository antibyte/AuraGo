package server

import (
	"html"
	"net/http"
)

const synthStudioStrudelPath = "/api/desktop/synth-studio/strudel"

func synthStudioStrudelCSP(origin string) string {
	return "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-eval' " + origin + "/js/vendor/strudel/strudel.js " + origin + "/js/desktop/apps/synth-studio-strudel-player.js blob:; worker-src blob:; connect-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'; form-action 'none'; base-uri 'none'; object-src 'none'"
}

func (s *Server) handleSynthStudioStrudel(w http.ResponseWriter, r *http.Request) {
	if !authenticateDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cfg := s.ConfigSnapshot(); cfg == nil || !cfg.VirtualDesktop.Enabled {
		http.Error(w, "Desktop unavailable", http.StatusServiceUnavailable)
		return
	}
	origin, ok := storymakerRequestOrigin(r)
	if !ok {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Security-Policy", synthStudioStrudelCSP(origin))
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), midi=(), serial=(), usb=(), geolocation=()")
	if r.Method == http.MethodHead {
		return
	}
	o := html.EscapeString(origin)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="strudel-parent" content="` + o + `"></head><body style="margin:0"><button id="sound" type="button" style="width:100%;min-height:36px" hidden>▶</button><script src="` + o + `/js/vendor/strudel/strudel.js?v=` + html.EscapeString(uiBuildVersion) + `"></script><script src="` + o + `/js/desktop/apps/synth-studio-strudel-player.js?v=` + html.EscapeString(uiBuildVersion) + `"></script></body></html>`))
}
