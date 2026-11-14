package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

const mailReceiptsRoute = "/api/mail-notifications/receipts"

func handleMailReceipts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		cfg := s.ConfigSnapshot()
		if r.URL.Path == mailReceiptsRoute && r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == mailReceiptsRoute && r.Method == http.MethodGet {
			limit := 50
			if raw := r.URL.Query().Get("limit"); raw != "" {
				var err error
				limit, err = strconv.Atoi(raw)
				if err != nil {
					jsonError(w, "invalid page size", http.StatusBadRequest)
					return
				}
			}
			page, err := tools.ListMailReceipts(r.Context(), cfg.Directories.DataDir, r.URL.Query().Get("channel"), r.URL.Query().Get("cursor"), limit)
			if err != nil {
				writeMailReceiptError(w, err)
				return
			}
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, mailReceiptsRoute+"/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || parts[1] != "resolve" {
			jsonError(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, bearer := bearerCredential(r.Header.Get("Authorization")); !bearer && !checkCSRFOriginWithPolicy(r, true) {
			jsonError(w, "invalid request origin", http.StatusForbidden)
			return
		}
		var body struct {
			Revision   string `json:"revision"`
			Resolution string `json:"resolution"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			jsonError(w, "invalid resolution", http.StatusBadRequest)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			jsonError(w, "invalid resolution", http.StatusBadRequest)
			return
		}
		if err := tools.ResolveMailReceipt(r.Context(), cfg.Directories.DataDir, parts[0], body.Revision, body.Resolution); err != nil {
			writeMailReceiptError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeMailReceiptError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tools.ErrMailReceiptRequest):
		jsonError(w, "invalid receipt request", http.StatusBadRequest)
	case errors.Is(err, tools.ErrMailReceiptConflict):
		jsonError(w, "receipt changed or attempt active; refresh before resolving", http.StatusConflict)
	case errors.Is(err, tools.ErrMailReceiptNotFound):
		jsonError(w, "receipt not found; refresh before resolving", http.StatusNotFound)
	default:
		// SQLite/transport errors must never become a metadata-content side channel.
		jsonError(w, "mail receipt store unavailable", http.StatusInternalServerError)
	}
}
