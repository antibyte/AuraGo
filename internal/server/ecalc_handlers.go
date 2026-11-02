package server

import (
	"net/http"
	"strings"

	"aurago/internal/ecalc"
)

const ecalcCatalogPath = "/api/ecalc/catalog"

func registerECalcRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc(ecalcCatalogPath, handleECalcCatalog(s))
}

// handleECalcCatalog serves GET /api/ecalc/catalog?lang=xx: the embedded ECalc catalog
// with names and documents resolved for one language (unknown or empty means English).
// The route is read-only and has no state or config switch. It is gated like the other
// desktop app routes (session, desktop read scope). The ETag is the one LocalizedJSON
// computed for the body, and a matching If-None-Match answers 304 without a body.
func handleECalcCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		catalog, err := ecalc.Default()
		if err != nil {
			s.Logger.Error("The ECalc catalog could not be loaded", "error", err)
			http.Error(w, "ECalc catalog unavailable", http.StatusInternalServerError)
			return
		}
		body, etag, err := catalog.LocalizedJSON(r.URL.Query().Get("lang"))
		if err != nil {
			s.Logger.Error("The ECalc catalog could not be localized", "error", err)
			http.Error(w, "ECalc catalog unavailable", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("ETag", etag)
		h.Set("Cache-Control", "no-cache")
		if ecalcETagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// ecalcETagMatches reports whether an If-None-Match header names etag. It compares
// weakly, as RFC 9110 prescribes for If-None-Match (a W/ prefix on either side is
// ignored), and "*" matches any tag.
func ecalcETagMatches(header, etag string) bool {
	if strings.TrimSpace(header) == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == want {
			return true
		}
	}
	return false
}
