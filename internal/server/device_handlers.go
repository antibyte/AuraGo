package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
)

// handleListDevices returns all devices (GET /api/devices).
func handleListDevices(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		devices, err := inventory.ListAllDevices(s.InventoryDB)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if devices == nil {
			devices = []inventory.DeviceRecord{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(devices)
	}
}

// handleGetDevice returns a single device by ID (GET /api/devices/{id}).
func handleGetDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			http.Error(w, `{"error":"device id required"}`, http.StatusBadRequest)
			return
		}

		d, err := inventory.GetDeviceByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(d)
	}
}

// handleCreateDevice creates a new device (POST /api/devices).
func handleCreateDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		var req inventory.DeviceRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			req.Type = "generic"
		}
		if req.Tags == nil {
			req.Tags = []string{}
		}
		if strings.TrimSpace(req.CredentialID) != "" {
			if _, err := credentials.GetByID(s.InventoryDB, req.CredentialID); err != nil {
				http.Error(w, `{"error":"linked credential not found"}`, http.StatusBadRequest)
				return
			}
			req.Username = ""
			req.VaultSecretID = ""
		}

		id, err := inventory.CreateDevice(s.InventoryDB, req.Name, req.Type, req.IPAddress, req.Port, req.Username, req.VaultSecretID, req.CredentialID, req.Description, req.Tags, req.MACAddress)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

// handleUpdateDevice updates an existing device (PUT /api/devices/{id}).
func handleUpdateDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			http.Error(w, `{"error":"device id required"}`, http.StatusBadRequest)
			return
		}

		var req inventory.DeviceRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		req.ID = id
		if req.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			req.Type = "generic"
		}
		if req.Tags == nil {
			req.Tags = []string{}
		}
		existing, err := inventory.GetDeviceByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}
		if strings.TrimSpace(req.CredentialID) != "" {
			if _, err := credentials.GetByID(s.InventoryDB, req.CredentialID); err != nil {
				http.Error(w, `{"error":"linked credential not found"}`, http.StatusBadRequest)
				return
			}
			req.Username = ""
			req.VaultSecretID = ""
		} else if existing.CredentialID == "" {
			// Preserve legacy inline access data for older device entries until they
			// are explicitly migrated to a credential reference.
			req.Username = existing.Username
			req.VaultSecretID = existing.VaultSecretID
		}

		if err := inventory.UpdateDevice(s.InventoryDB, req); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleDeleteDevice deletes a device (DELETE /api/devices/{id}).
func handleDeleteDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			http.Error(w, `{"error":"device id required"}`, http.StatusBadRequest)
			return
		}

		if err := inventory.DeleteDevice(s.InventoryDB, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
