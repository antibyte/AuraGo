package desktop

import (
	"net/http"
	"net/url"
	"strings"
)

func sameHostWebSocketOrigin(r *http.Request) bool {
	if r == nil {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// SameHostWebSocketOrigin is the strict same-host Origin check for WebSocket
// bridges outside this package; an empty Origin is refused.
func SameHostWebSocketOrigin(r *http.Request) bool { return sameHostWebSocketOrigin(r) }
