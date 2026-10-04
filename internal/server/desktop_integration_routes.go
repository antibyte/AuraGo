package server

import (
	"net/http"
	"path"
	"strings"
)

// Shared integrations retain their own handlers and authorization. Only these
// explicit server-owned routes can also be entered through Desktop policy.
var desktopIntegrationRoots = map[string]bool{
	"3d-printers": true, "agent": true, "appointments": true, "bluetooth": true,
	"budget": true, "cheatsheets": true, "contacts": true, "containers": true,
	"credentials": true, "daemons": true, "dashboard": true, "devices": true,
	"go2rtc": true, "homepage": true, "integrations": true, "invasion": true,
	"knowledge-graph": true, "launchpad": true, "meshcore": true, "missions": true,
	"openscad": true, "operational-issues": true, "people": true, "personalities": true,
	"providers": true, "radio-browser": true, "realtime-speech": true, "sip": true,
	"speech-lab": true, "todos": true, "webhooks": true,
}

func desktopIntegrationHandler(s *Server, routes http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/api/desktop/integrations/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		root, _, _ := strings.Cut(rel, "/")
		if !desktopIntegrationRoots[root] || strings.ContainsAny(rel, "\\%\x00") || path.Clean(rel) != strings.TrimSuffix(rel, "/") {
			http.NotFound(w, r)
			return
		}
		if !requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopIntegrationOperation(r.Method, rel)) {
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/api/" + rel
		clone.URL.RawPath = ""
		clone.RequestURI = clone.URL.RequestURI()
		routes.ServeHTTP(w, clone)
	}
}

func desktopIntegrationOperation(method, route string) desktopOperation {
	if method == http.MethodPost && strings.HasPrefix(route, "missions/v2/") && strings.HasSuffix(route, "/cancel") {
		return desktopStop
	}
	if method == http.MethodPost && strings.HasPrefix(route, "sip/calls/") && strings.HasSuffix(route, "/hangup") {
		return desktopStop
	}
	if method == http.MethodDelete && (strings.HasPrefix(route, "realtime-speech/sessions/") || strings.HasPrefix(route, "realtime-speech/actions/")) {
		return desktopStop
	}
	if method == http.MethodPost && strings.HasPrefix(route, "realtime-speech/actions/") && strings.HasSuffix(route, "/cancel") {
		return desktopStop
	}
	if strings.HasPrefix(route, "sip/browser-media") || strings.HasPrefix(route, "realtime-speech/headset") {
		return desktopExecute
	}
	return desktopMethodOperation(method)
}
