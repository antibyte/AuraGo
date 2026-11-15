package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopSearch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		query := r.URL.Query()
		rawPath := query.Get("path")
		searchTerm := query.Get("query")
		w.Header().Set("Content-Type", "application/json")

		if !desktopSearchRequestIsPaged(query) {
			files, err := svc.SearchFiles(r.Context(), rawPath, searchTerm)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"files":  files,
			})
			return
		}

		options := desktop.SearchFilesPageOptions{
			Query: searchTerm,
			Type:  query.Get("type"),
		}
		if options.Type == "" {
			options.Type = query.Get("filter")
		}
		if value, ok := query["limit"]; ok && len(value) > 0 && value[0] != "" {
			options.Limit, err = strconv.Atoi(value[0])
			if err != nil {
				jsonError(w, "invalid desktop search limit", http.StatusBadRequest)
				return
			}
		}
		if value, ok := query["offset"]; ok && len(value) > 0 && value[0] != "" {
			options.Offset, err = strconv.Atoi(value[0])
			if err != nil {
				jsonError(w, "invalid desktop search offset", http.StatusBadRequest)
				return
			}
		}
		if options.ModifiedAfter, err = desktopSearchTimeBound(query.Get("modified_after")); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if options.ModifiedBefore, err = desktopSearchTimeBound(query.Get("modified_before")); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if options.MinSize, err = desktopSearchSizeBound(query.Get("min_size")); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if options.MaxSize, err = desktopSearchSizeBound(query.Get("max_size")); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		page, err := svc.SearchFilesPage(r.Context(), rawPath, options)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"files":       page.Files,
			"has_more":    page.HasMore,
			"truncated":   page.Truncated,
			"next_offset": page.NextOffset,
		})
	}
}

func desktopSearchRequestIsPaged(query map[string][]string) bool {
	for _, key := range []string{"limit", "offset", "type", "filter", "modified_after", "modified_before", "min_size", "max_size"} {
		if _, ok := query[key]; ok {
			return true
		}
	}
	return false
}

func desktopSearchTimeBound(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("desktop search dates must use RFC3339")
	}
	return &parsed, nil
}

func desktopSearchSizeBound(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("desktop search sizes must be whole bytes")
	}
	return &parsed, nil
}
