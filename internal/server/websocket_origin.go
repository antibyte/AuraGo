package server

import (
	"net/http"
	"strings"
)

func sameOriginOrNoOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	return requestOriginMatches(r, origin)
}
