package server

import (
	"net/http"

	"aurago/internal/security"
)

// trustedUntrustedProvenance reports whether an authenticated loopback request
// says that the task behind it was queued from a session that had read
// third-party content. The header means nothing without the internal token.
func trustedUntrustedProvenance(r *http.Request, s *Server) bool {
	return r != nil && s != nil && r.Header.Get("X-Internal-FollowUp") == "true" &&
		r.Header.Get("X-Untrusted-Provenance") == "true" && isValidInternalLoopbackToken(r, s.internalToken)
}

func trustedMailIngress(r *http.Request, s *Server) bool {
	return r != nil && s != nil && r.Header.Get("X-Internal-FollowUp") == "true" &&
		r.Header.Get("X-External-Source") == string(security.ExternalEmail) && isValidInternalLoopbackToken(r, s.internalToken)
}
