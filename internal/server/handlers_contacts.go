package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/contacts"
	"aurago/internal/i18n"
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
			if err := contacts.Update(s.ContactsDB, c); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
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
			if err := contacts.Delete(s.ContactsDB, id); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, i18n.T(desktopUILanguage(s), "backend.contacts_not_found"), status)
				} else {
					jsonLoggedError(w, s.Logger, status, i18n.T(desktopUILanguage(s), "backend.contacts_delete_failed"), "Failed to delete contact", err, "contact_id", id)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, i18n.T(desktopUILanguage(s), "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
		}
	}
}
