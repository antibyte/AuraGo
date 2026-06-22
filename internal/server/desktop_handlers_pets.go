package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/desktop"
)

func handleDesktopPets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			if !requireDesktopPermission(s, w, r, desktopScopeRead) {
				return
			}
			id := r.URL.Query().Get("id")
			if id != "" {
				pet, err := svc.GetPet(r.Context(), id)
				if err != nil {
					jsonError(w, err.Error(), http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "pet": pet})
				return
			}
			pets, err := svc.ListPets(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			activeID, _ := svc.GetActivePetID(r.Context())
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "pets": pets, "active_pet_id": activeID})

		case http.MethodPost:
			if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
				return
			}
			action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
			switch action {
			case "activate":
				var body struct {
					ID string `json:"id"`
				}
				if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
					jsonError(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
				if err := svc.SetActivePet(r.Context(), body.ID); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
				event := desktop.Event{Type: "pet_changed", Payload: map[string]interface{}{"active_pet_id": body.ID}, CreatedAt: time.Now().UTC()}
				broadcastDesktopEvent(s, hub, event)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "active_pet_id": body.ID})

			case "install":
				if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
					return
				}
				var body struct {
					ID    string            `json:"id"`
					Files map[string]string `json:"files"`
				}
				if err := decodeDesktopJSON(w, r, &body, desktopLargeJSONBodyLimit); err != nil {
					jsonError(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
				// Support direct file map or a base64-encoded data.zip entry.
				if zipB64, ok := body.Files["data.zip"]; ok {
					zipData, err := base64.StdEncoding.DecodeString(zipB64)
					if err != nil {
						jsonError(w, "invalid zip data: "+err.Error(), http.StatusBadRequest)
						return
					}
					if err := svc.InstallPetFromZip(r.Context(), body.ID, bytes.NewReader(zipData), int64(len(zipData))); err != nil {
						jsonError(w, err.Error(), http.StatusBadRequest)
						return
					}
				} else {
					fileBytes := make(map[string][]byte, len(body.Files))
					for k, v := range body.Files {
						fileBytes[k] = []byte(v)
					}
					if err := svc.InstallPet(r.Context(), body.ID, fileBytes); err != nil {
						jsonError(w, err.Error(), http.StatusBadRequest)
						return
					}
				}
				event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "install_pet", "pet_id": body.ID}, CreatedAt: time.Now().UTC()}
				broadcastDesktopEvent(s, hub, event)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "pet_id": body.ID})

			default:
				jsonError(w, "unsupported action", http.StatusBadRequest)
			}

		case http.MethodDelete:
			if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
				return
			}
			id := r.URL.Query().Get("id")
			if err := svc.DeletePet(r.Context(), id); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			event := desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "delete_pet", "pet_id": id}, CreatedAt: time.Now().UTC()}
			broadcastDesktopEvent(s, hub, event)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
