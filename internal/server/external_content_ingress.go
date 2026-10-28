package server

import (
	"net/http"

	"aurago/internal/security"
)

func trustedMailIngress(r *http.Request, s *Server) bool {
	return r != nil && s != nil && r.Header.Get("X-Internal-FollowUp") == "true" &&
		r.Header.Get("X-External-Source") == string(security.ExternalEmail) && isValidInternalLoopbackToken(r, s.internalToken)
}
