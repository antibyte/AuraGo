package server

import (
	"context"
	"net/http"
	"strings"
)

const desktopTicketPrefix = "/desktop-ticket/"

type desktopTicketContextKey struct{}

// desktopTicketMiddleware carries short-lived embed credentials in a path
// segment, then removes that segment before routing and access logging.
func desktopTicketMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, desktopTicketPrefix) {
			part := strings.TrimPrefix(r.URL.Path, desktopTicketPrefix)
			separator := strings.IndexByte(part, '/')
			if separator <= 0 || separator > 4096 {
				http.NotFound(w, r)
				return
			}
			token, target := part[:separator], part[separator:]
			if !strings.HasPrefix(target, "/files/desktop/") && !isDesktopEmbedResourcePath(target) {
				http.NotFound(w, r)
				return
			}
			for _, ch := range token {
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
					(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') {
					http.NotFound(w, r)
					return
				}
			}
			clone := r.Clone(context.WithValue(r.Context(), desktopTicketContextKey{}, token))
			clone.URL.Path = target
			clone.URL.RawPath = ""
			clone.RequestURI = target
			if clone.URL.RawQuery != "" {
				clone.RequestURI += "?" + clone.URL.RawQuery
			}
			r = clone
		}
		if r.URL.Query().Has(desktopEmbedTokenParam) &&
			(strings.HasPrefix(r.URL.Path, "/files/desktop/") || isDesktopEmbedResourcePath(r.URL.Path)) {
			http.Error(w, "query embed tokens are no longer accepted", http.StatusUnauthorized)
			return
		}
		if desktopTicketFromRequest(r) != "" {
			w.Header().Set("Referrer-Policy", "no-referrer")
		}
		next.ServeHTTP(w, r)
	})
}

func desktopTicketFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	token, _ := r.Context().Value(desktopTicketContextKey{}).(string)
	return token
}
