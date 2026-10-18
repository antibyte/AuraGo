package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"aurago/internal/contacts"
	"aurago/internal/i18n"
	"aurago/internal/memory"
)

// handleContacts handles GET (list) and POST (create) on /api/contacts.
func handleContacts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ContactsDB == nil {
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_not_initialized"), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query().Get("q")
			list, err := contacts.List(s.ContactsDB, query)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, i18n.T(desktopUILanguage(s), "backend.contacts_list_failed"), "Failed to list contacts", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			var c contacts.Contact
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, "failed to read body", http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &c); err != nil {
				jsonError(w, i18n.T(desktopUILanguage(s), "backend.auth_invalid_json"), http.StatusBadRequest)
				return
			}
			id, err := contacts.Create(s.ContactsDB, c)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, i18n.T(desktopUILanguage(s), "backend.contacts_create_failed"), "Failed to create contact", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id})

		default:
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
		}
	}
}

// handleContactByID handles GET, PUT, DELETE on /api/contacts/{id}.
func handleContactByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ContactsDB == nil {
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_not_initialized"), http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/contacts/")
		if id == "" {
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_missing_id"), http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			c, err := contacts.GetByID(s.ContactsDB, id)
			if err != nil {
				jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_not_found"), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(c)

		case http.MethodPut:
			var c contacts.Contact
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, "failed to read body", http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &c); err != nil {
				jsonError(w, i18n.T(desktopUILanguage(s), "backend.auth_invalid_json"), http.StatusBadRequest)
				return
			}
			c.ID = id
			if err := contacts.UpdateContext(r.Context(), s.ContactsDB, c); err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, contacts.ErrNotFound) {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_not_found"), status)
				} else {
					jsonLoggedError(w, s.Logger, status, i18n.T(desktopUILanguage(s), "backend.contacts_update_failed"), "Failed to update contact", err, "contact_id", id)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			deleteMemories := r.URL.Query().Get("delete_memories") == "true"
			if s.ShortTermMem == nil {
				writePrivacyJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "error": "privacy_store_unavailable", "contact_deleted": false})
				return
			}
			result, err := s.ShortTermMem.DeleteContactWithPrivacy(r.Context(), s.ContactsDB, s.KG, id, deleteMemories)
			job := result.Job
			status, code := "deleted", http.StatusOK
			errorCode := ""
			if err != nil && job.DeletionJobID == "" {
				status, code, errorCode = "error", http.StatusInternalServerError, "contact_delete_not_accepted"
				if errors.Is(err, contacts.ErrNotFound) {
					code, errorCode = http.StatusNotFound, "contact_not_found"
				}
			} else if job.DeletionStatus == memory.PrivacyDeletionAborted {
				status, code, errorCode = "error", http.StatusInternalServerError, "contact_delete_aborted"
			} else if err != nil || job.PendingCount > 0 || !result.OutcomeKnown || job.DeletionStatus != memory.PrivacyDeletionCompleted && job.DeletionStatus != memory.PrivacyDeletionRetainedReview {
				status, code = "partial", http.StatusMultiStatus
			}
			// Never return SQLite errors, names, source IDs, or operation tokens.
			writePrivacyJSON(w, code, map[string]any{
				"status": status, "error": errorCode, "contact_deleted": result.ContactDeleted, "contact_outcome_known": result.OutcomeKnown,
				"deletion_job_id": job.DeletionJobID, "deletion_status": job.DeletionStatus,
				"pending_count": job.PendingCount, "deleted_count": job.DeletedCount, "retained_count": job.RetainedCount,
				"deleted_graph_items": job.DeletedCount, "retained_graph_items": job.RetainedCount,
			})

		default:
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
		}
	}
}
