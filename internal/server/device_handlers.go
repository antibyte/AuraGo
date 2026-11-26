package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/tools"
)

// handleListDevices returns all devices (GET /api/devices).
func handleListDevices(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
			return
		}

		devices, err := inventory.ListAllDevices(s.InventoryDB)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list devices", "Failed to list devices", err)
			return
		}
		if devices == nil {
			devices = []inventory.DeviceRecord{}
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("quick_connect") == "1" {
			requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				list := quickConnectDevices(r.Context(), devices)
				fillQuickConnectFileAccess(s, list)
				json.NewEncoder(w).Encode(list)
			})).ServeHTTP(w, r)
			return
		}
		json.NewEncoder(w).Encode(devices)
	}
}

// handleGetDevice returns a single device by ID (GET /api/devices/{id}).
func handleGetDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			jsonError(w, "device id required", http.StatusBadRequest)
			return
		}

		d, err := inventory.GetDeviceByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(w, "device not found", http.StatusNotFound)
			} else {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load device", "Failed to load device", err, "device_id", id)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
			return
		}

		var body deviceWriteRequest
		if err := decodeJSONRequest(w, r, &body, s); err != nil {
			jsonError(w, "invalid JSON", jsonDecodeStatus(err))
			return
		}
		req := body.DeviceRecord
		if req.Name == "" {
			jsonError(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			req.Type = "generic"
		}
		req.Protocol = normalizeDeviceProtocolForAPI(req.Protocol)
		if !isAllowedDeviceProtocol(req.Protocol) {
			jsonError(w, "protocol must be ssh, vnc, rdp, or none", http.StatusBadRequest)
			return
		}
		if req.Tags == nil {
			req.Tags = []string{}
		}
		req.CredentialID = strings.TrimSpace(req.CredentialID)
		if req.CredentialID != "" {
			if _, err := credentials.GetByID(s.InventoryDB, req.CredentialID); err != nil {
				jsonError(w, "linked credential not found", http.StatusBadRequest)
				return
			}
			req.Username = ""
			req.VaultSecretID = ""
		}

		fileAccess, ok := validateDeviceFileAccess(s, w, body.FileAccess)
		if !ok {
			return
		}

		id, err := inventory.CreateDevice(s.InventoryDB, req.Name, req.Type, req.Protocol, req.IPAddress, req.Port, req.Username, req.VaultSecretID, req.CredentialID, req.Description, req.Tags, req.MACAddress)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create device", "Failed to create device", err, "device_name", req.Name)
			return
		}
		if fileAccess != nil && req.Protocol == inventory.ProtocolRDP {
			if err := inventory.SetDeviceFileAccess(s.InventoryDB, id, *fileAccess); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store file access", "Failed to store file access", err, "device_id", id)
				return
			}
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			jsonError(w, "device id required", http.StatusBadRequest)
			return
		}

		var body deviceWriteRequest
		if err := decodeJSONRequest(w, r, &body, s); err != nil {
			jsonError(w, "invalid JSON", jsonDecodeStatus(err))
			return
		}
		req := body.DeviceRecord
		req.ID = id
		if req.Name == "" {
			jsonError(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.Type == "" {
			req.Type = "generic"
		}
		req.Protocol = normalizeDeviceProtocolForAPI(req.Protocol)
		if !isAllowedDeviceProtocol(req.Protocol) {
			jsonError(w, "protocol must be ssh, vnc, rdp, or none", http.StatusBadRequest)
			return
		}
		if req.Tags == nil {
			req.Tags = []string{}
		}
		existing, err := inventory.GetDeviceByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(w, "device not found", http.StatusNotFound)
			} else {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load device", "Failed to load device", err, "device_id", id)
			}
			return
		}
		req.CredentialID = strings.TrimSpace(req.CredentialID)
		if req.CredentialID != "" {
			if _, err := credentials.GetByID(s.InventoryDB, req.CredentialID); err != nil {
				jsonError(w, "linked credential not found", http.StatusBadRequest)
				return
			}
			req.Username = ""
			req.VaultSecretID = ""
		} else if existing.CredentialID != "" && strings.TrimSpace(req.Username) == "" && strings.TrimSpace(req.VaultSecretID) == "" {
			req.CredentialID = existing.CredentialID
			req.Username = ""
			req.VaultSecretID = ""
		} else if existing.CredentialID == "" {
			// Preserve legacy inline access data for older device entries until they
			// are explicitly migrated to a credential reference.
			req.Username = existing.Username
			req.VaultSecretID = existing.VaultSecretID
		}

		fileAccess, ok := validateDeviceFileAccess(s, w, body.FileAccess)
		if !ok {
			return
		}

		if err := inventory.UpdateDevice(s.InventoryDB, req); err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(w, "device not found", http.StatusNotFound)
			} else {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to update device", "Failed to update device", err, "device_id", id)
			}
			return
		}
		if req.Protocol != inventory.ProtocolRDP {
			_ = inventory.DeleteDeviceFileAccess(s.InventoryDB, id)
		} else if fileAccess != nil {
			if err := inventory.SetDeviceFileAccess(s.InventoryDB, id, *fileAccess); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store file access", "Failed to store file access", err, "device_id", id)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleDeleteDevice deletes a device (DELETE /api/devices/{id}).
func handleDeleteDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			jsonError(w, "inventory database not configured", http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" {
			jsonError(w, "device id required", http.StatusBadRequest)
			return
		}

		if err := inventory.DeleteDevice(s.InventoryDB, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(w, "device not found", http.StatusNotFound)
			} else {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete device", "Failed to delete device", err, "device_id", id)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleMACLookup performs an ARP-table MAC address lookup for a given IP
// (POST /api/tools/mac_lookup).
func handleMACLookup(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			IP string `json:"ip"`
		}
		if err := decodeJSONRequest(w, r, &req, s); err != nil || strings.TrimSpace(req.IP) == "" {
			jsonError(w, "ip field required", jsonDecodeStatus(err))
			return
		}

		result := tools.LookupMACAddress(strings.TrimSpace(req.IP), "")
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(result)); err != nil {
			s.Logger.Debug("failed to write mac_lookup response", "error", err)
		}
	}
}

func normalizeDeviceProtocolForAPI(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		return inventory.ProtocolSSH
	}
	return protocol
}

func isAllowedDeviceProtocol(protocol string) bool {
	switch protocol {
	case inventory.ProtocolSSH, inventory.ProtocolVNC, inventory.ProtocolRDP, inventory.ProtocolNone:
		return true
	default:
		return false
	}
}

// deviceWriteRequest is the body of POST /api/devices and PUT /api/devices/{id}.
// FileAccess lives in a side table; nil leaves a stored setting untouched.
type deviceWriteRequest struct {
	inventory.DeviceRecord
	FileAccess *inventory.FileAccess `json:"file_access"`
}

// validateDeviceFileAccess normalises the optional file access setting and
// checks the override credential. It writes the 400 response itself.
func validateDeviceFileAccess(s *Server, w http.ResponseWriter, in *inventory.FileAccess) (*inventory.FileAccess, bool) {
	if in == nil {
		return nil, true
	}
	fa, err := inventory.NormalizeFileAccess(*in)
	if err != nil {
		jsonError(w, "invalid file_access: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}
	if fa.CredentialID != "" {
		if _, err := credentials.GetByID(s.InventoryDB, fa.CredentialID); err != nil {
			jsonError(w, "file access credential not found", http.StatusBadRequest)
			return nil, false
		}
	}
	return &fa, true
}

// fillQuickConnectFileAccess sets file_access on RDP entries (off when no row).
func fillQuickConnectFileAccess(s *Server, list []quickConnectDevice) {
	stored, err := inventory.ListDeviceFileAccess(s.InventoryDB)
	if err != nil {
		stored = nil
		s.Logger.Warn("Failed to load device file access", "error", err)
	}
	for i := range list {
		if list[i].Protocol != inventory.ProtocolRDP {
			continue
		}
		fa, ok := stored[list[i].ID]
		if !ok {
			fa = inventory.DefaultFileAccess()
		}
		list[i].FileAccess = &fa
	}
}
