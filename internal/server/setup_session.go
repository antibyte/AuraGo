package server

import (
	"net/http"
	"strings"
	"time"
)

// setupSessionOriginAllowed applies the same Origin / Sec-Fetch-* rules as
// handleAuthLogin, so the setup-time session is only issued to a request that
// the login form would accept too. Keep both in sync.
func setupSessionOriginAllowed(r *http.Request) bool {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !requestOriginMatches(r, origin) {
		return false
	}
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if r.Header.Get("Origin") == "" && r.Header.Get("Sec-Fetch-Site") == "" &&
		(r.Header.Get("Sec-Fetch-Mode") != "" || r.Header.Get("Sec-Fetch-Dest") != "") {
		return false
	}
	return true
}

// issueSetupSession signs the browser that just set the initial admin password
// in, so the wizard does not end on a login screen. It must be called from
// handleSetupSave (which holds CfgSaveMu), hence it does not go through
// handleAuthLogin. Any failed check only skips the cookie; the save itself
// stays successful. It must run before the response headers are written.
func issueSetupSession(s *Server, w http.ResponseWriter, r *http.Request, hash, secret string) bool {
	if hash == "" || secret == "" || !setupSessionOriginAllowed(r) {
		return false
	}
	current, _ := s.webAuthnAuthSnapshot()
	if !current.Enabled || current.PasswordHash != hash || current.SessionSecret != secret {
		return false
	}
	// A password alone is not a full login once a second factor is required.
	if current.TOTPEnabled && current.TOTPSecret != "" {
		return false
	}
	if !webAuthnPasswordLoginAllowed(current) {
		return false
	}
	timeoutHours := current.TimeoutHours
	if timeoutHours <= 0 {
		timeoutHours = 24
	}
	SetSessionCookie(w, r, secret, time.Duration(timeoutHours)*time.Hour)
	ip, _, _ := adminLoginKeys(s, r)
	s.Logger.Info("[Auth] Session issued after initial setup", "ip", ip)
	return true
}
