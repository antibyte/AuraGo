package server

import (
	"net/http"
	"strings"
)

const chibiSenseiAPIBase = "/api/desktop/chibi-sensei"

// handleChibiSensei routes /api/desktop/chibi-sensei/*. B1 serves only meta; later batches add
// projects, rows, jobs and the rest.
func handleChibiSensei(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, chibiSenseiAPIBase+"/") {
			writeChibiError(w, http.StatusNotFound, "not_found", "Chibi Sensei endpoint not found.", nil)
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, chibiSenseiAPIBase), "/"), "/")
		switch {
		case len(parts) == 1 && parts[0] == "meta":
			handleChibiMeta(s, w, r)
		default:
			writeChibiError(w, http.StatusNotFound, "not_found", "Chibi Sensei endpoint not found.", nil)
		}
	}
}
