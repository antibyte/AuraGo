package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/memory"
)

func handlePrivacyDeletionCollection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/privacy-deletions" {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Privacy deletion store unavailable", http.StatusServiceUnavailable)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		jobs, err := s.ShortTermMem.ListPrivacyDeletionJobs(r.Context(), limit)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list privacy deletions", "Failed to list privacy deletions", err)
			return
		}
		writePrivacyJSON(w, http.StatusOK, map[string]any{"status": "ok", "jobs": jobs})
	}
}

func handlePrivacyDeletionItem(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ShortTermMem == nil {
			jsonError(w, "Privacy deletion store unavailable", http.StatusServiceUnavailable)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/privacy-deletions/")
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "retry" {
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			job, err := s.ShortTermMem.RetryPrivacyDeletionJob(r.Context(), parts[0], time.Now().UTC())
			if errors.Is(err, memory.ErrPrivacyDeletionJobNotFound) {
				jsonError(w, "deletion_job_not_found", http.StatusNotFound)
				return
			}
			if errors.Is(err, memory.ErrPrivacyDeletionJobNotRetryable) {
				jsonError(w, "deletion_job_not_retryable", http.StatusConflict)
				return
			}
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to retry privacy deletion", "Failed to retry privacy deletion", err)
				return
			}
			writePrivacyJSON(w, http.StatusAccepted, map[string]any{
				"status": "ok", "deletion_job_id": job.DeletionJobID, "deletion_status": job.DeletionStatus,
				"pending_count": job.PendingCount, "deleted_count": job.DeletedCount, "retained_count": job.RetainedCount,
			})
			return
		}
		if len(parts) != 1 || parts[0] == "" || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		job, err := s.ShortTermMem.GetPrivacyDeletionJob(r.Context(), parts[0])
		if errors.Is(err, memory.ErrPrivacyDeletionJobNotFound) {
			jsonError(w, "deletion_job_not_found", http.StatusNotFound)
			return
		}
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read privacy deletion", "Failed to read privacy deletion", err)
			return
		}
		writePrivacyJSON(w, http.StatusOK, map[string]any{"status": "ok", "job": job})
	}
}

func writePrivacyJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
