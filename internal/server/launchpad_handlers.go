package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/launchpad"
)

// ═══════════════════════════════════════════════════════════════
// Launchpad Link Handlers
// ═══════════════════════════════════════════════════════════════

func handleListLaunchpadLinks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		category := r.URL.Query().Get("category")
		links, err := launchpad.List(s.LaunchpadDB, category)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "failed to list links", "list launchpad links failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(links)
	}
}

func handleCreateLaunchpadLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var link launchpad.LaunchpadLink
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		id, err := launchpad.Create(s.LaunchpadDB, link)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		link.ID = id
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(link)
	}
}

func handleGetLaunchpadLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/launchpad/links/")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}
		link, err := launchpad.GetByID(s.LaunchpadDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"link not found"}`, http.StatusNotFound)
				return
			}
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "failed to get link", "get launchpad link failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(link)
	}
}

func handleUpdateLaunchpadLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/launchpad/links/")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}
		var link launchpad.LaunchpadLink
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		link.ID = id
		if err := launchpad.Update(s.LaunchpadDB, link); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"link not found"}`, http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteLaunchpadLink(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/launchpad/links/")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}
		iconPath, err := launchpad.Delete(s.LaunchpadDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"link not found"}`, http.StatusNotFound)
				return
			}
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "failed to delete link", "delete launchpad link failed", err)
			return
		}
		// Clean up icon file if present
		if iconPath != "" {
			if err := launchpad.DeleteIcon(s.Cfg.Directories.DataDir, iconPath); err != nil {
				s.Logger.Warn("Failed to delete launchpad icon file", "path", iconPath, "error", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}

// ═══════════════════════════════════════════════════════════════
// Launchpad Category Handlers
// ═══════════════════════════════════════════════════════════════

func handleListLaunchpadCategories(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		cats, err := launchpad.ListCategories(s.LaunchpadDB)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "failed to list categories", "list launchpad categories failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cats)
	}
}

// ═══════════════════════════════════════════════════════════════
// Launchpad Icon Handlers
// ═══════════════════════════════════════════════════════════════

func handleSearchLaunchpadIcons(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		query := r.URL.Query().Get("q")
		results, err := launchpad.SearchIcons(s.LaunchpadDB, query)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "failed to search icons", "launchpad icon search failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func handleDownloadLaunchpadIcon(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.LaunchpadDB == nil {
			jsonError(w, `{"error":"launchpad database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		var req struct {
			ImageURL string `json:"image_url"`
			LinkID   string `json:"link_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if req.ImageURL == "" {
			http.Error(w, `{"error":"image_url is required"}`, http.StatusBadRequest)
			return
		}
		result, err := launchpad.DownloadIcon(s.Cfg.Directories.DataDir, req.ImageURL, req.LinkID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
