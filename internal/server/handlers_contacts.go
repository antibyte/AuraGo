package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/contacts"
)

// handleContacts handles GET (list) and POST (create) on /api/contacts.
func handleContacts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ContactsDB == nil {
			http.Error(w, `{"error":"contacts database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query().Get("q")
			list, err := contacts.List(s.ContactsDB, query)
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			var c contacts.Contact
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &c); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			id, err := contacts.Create(s.ContactsDB, c)
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id})

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleContactByID handles GET, PUT, DELETE on /api/contacts/{id}.
func handleContactByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ContactsDB == nil {
			http.Error(w, `{"error":"contacts database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/contacts/")
		if id == "" {
			http.Error(w, `{"error":"missing contact id"}`, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			c, err := contacts.GetByID(s.ContactsDB, id)
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(c)

		case http.MethodPut:
			var c contacts.Contact
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &c); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			c.ID = id
			if err := contacts.Update(s.ContactsDB, c); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				http.Error(w, `{"error":"`+err.Error()+`"}`, status)
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
				http.Error(w, `{"error":"`+err.Error()+`"}`, status)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}
