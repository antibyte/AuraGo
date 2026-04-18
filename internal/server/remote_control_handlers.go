package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/remote"

	"github.com/gorilla/websocket"
)

// ── WebSocket handler ───────────────────────────────────────────────────────

var remoteUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func handleRemoteWebSocket(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		wsConn, err := remoteUpgrader.Upgrade(w, r, nil)
		if err != nil {
			s.Logger.Error("Remote WebSocket upgrade failed", "error", err)
			return
		}

		// Read the first message — must be auth
		wsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, data, err := wsConn.ReadMessage()
		wsConn.SetReadDeadline(time.Time{})
		if err != nil {
			s.Logger.Warn("Remote: no auth message received", "error", err)
			wsConn.Close()
			return
		}

		var msg remote.RemoteMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.Logger.Warn("Remote: invalid auth message", "error", err)
			wsConn.Close()
			return
		}

		if msg.Type != remote.MsgAuth {
			s.Logger.Warn("Remote: first message must be auth", "type", msg.Type)
			wsConn.Close()
			return
		}

		if err := s.RemoteHub.HandleEnrollment(wsConn, msg); err != nil {
			s.Logger.Debug("Remote enrollment result", "info", err)
			// Connection may already be closed by HandleEnrollment
			return
		}

		// Find the connection that was just registered
		var auth remote.AuthPayload
		_ = json.Unmarshal(msg.Payload, &auth)
		deviceID := auth.DeviceID
		if deviceID == "" {
			// For new enrollments, the device ID was sent in the auth response
			// The connection should now be registered under the new device ID
			// We need to find it
			for _, id := range s.RemoteHub.ConnectedDevices() {
				conn := s.RemoteHub.GetConnection(id)
				if conn != nil && conn.Conn == wsConn {
					deviceID = id
					break
				}
			}
		}

		conn := s.RemoteHub.GetConnection(deviceID)
		if conn == nil {
			// Not registered (pending approval, rejected, etc.)
			return
		}

		// Block on message handling
		s.RemoteHub.HandleMessages(conn)
	}
}

// ── REST API handlers ───────────────────────────────────────────────────────

func handleRemoteDevices(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			devices, err := remote.ListDevices(s.RemoteHub.DB())
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load devices", "Failed to list remote devices", err)
				return
			}
			// Enrich with connection status
			type deviceView struct {
				remote.DeviceRecord
				IsConnected bool `json:"is_connected"`
			}
			views := make([]deviceView, len(devices))
			for i, d := range devices {
				views[i] = deviceView{
					DeviceRecord: d,
					IsConnected:  s.RemoteHub.IsConnected(d.ID),
				}
			}
			writeJSON(w, views)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleRemoteDevice(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}
		// Extract device ID from URL: /api/remote/devices/{id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/remote/devices/"), "/")
		deviceID := parts[0]
		if deviceID == "" {
			jsonError(w, "missing device ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			device, err := remote.GetDevice(s.RemoteHub.DB(), deviceID)
			if err != nil {
				jsonError(w, "device not found", http.StatusNotFound)
				return
			}
			writeJSON(w, device)

		case http.MethodPut:
			var update struct {
				Name         *string  `json:"name"`
				ReadOnly     *bool    `json:"read_only"`
				AllowedPaths []string `json:"allowed_paths"`
				Tags         []string `json:"tags"`
			}
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				jsonError(w, "invalid request body", http.StatusBadRequest)
				return
			}

			device, err := remote.GetDevice(s.RemoteHub.DB(), deviceID)
			if err != nil {
				jsonError(w, "device not found", http.StatusNotFound)
				return
			}

			if update.Name != nil {
				device.Name = *update.Name
			}
			if update.ReadOnly != nil {
				device.ReadOnly = *update.ReadOnly
			}
			if update.AllowedPaths != nil {
				device.AllowedPaths = update.AllowedPaths
			}
			if update.Tags != nil {
				device.Tags = update.Tags
			}

			if err := remote.UpdateDevice(s.RemoteHub.DB(), device); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to update device", "Failed to update remote device", err, "device_id", deviceID)
				return
			}

			// Push config update to connected device
			if s.RemoteHub.IsConnected(deviceID) {
				ro := device.ReadOnly
				maxFileSizeMB := s.Cfg.RemoteControl.MaxFileSizeMB
				_ = s.RemoteHub.SendConfigUpdate(deviceID, remote.ConfigUpdatePayload{
					ReadOnly:      &ro,
					AllowedPaths:  device.AllowedPaths,
					MaxFileSizeMB: &maxFileSizeMB,
				})
			}

			writeJSON(w, device)

		case http.MethodDelete:
			// Revoke if connected
			if s.RemoteHub.IsConnected(deviceID) {
				_ = s.RemoteHub.SendRevoke(deviceID)
			}
			if err := remote.DeleteDevice(s.RemoteHub.DB(), deviceID); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete device", "Failed to delete remote device", err, "device_id", deviceID)
				return
			}
			// Clean up vault secret
			_ = s.Vault.DeleteSecret("remote_shared_key_" + deviceID)
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleRemoteDeviceApprove(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		deviceID := extractRemoteDeviceID(r.URL.Path, "/approve")
		if err := s.RemoteHub.ApproveDevice(deviceID); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Unable to approve device", "Failed to approve remote device", err, "device_id", deviceID)
			return
		}
		writeJSON(w, map[string]string{"status": "approved"})
	}
}

func handleRemoteDeviceReject(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		deviceID := extractRemoteDeviceID(r.URL.Path, "/reject")
		if err := s.RemoteHub.RejectDevice(deviceID); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Unable to reject device", "Failed to reject remote device", err, "device_id", deviceID)
			return
		}
		writeJSON(w, map[string]string{"status": "rejected"})
	}
}

func handleRemoteDeviceRevoke(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		deviceID := extractRemoteDeviceID(r.URL.Path, "/revoke")
		if s.RemoteHub.IsConnected(deviceID) {
			_ = s.RemoteHub.SendRevoke(deviceID)
		}
		_ = remote.UpdateDeviceStatus(s.RemoteHub.DB(), deviceID, "revoked")
		writeJSON(w, map[string]string{"status": "revoked"})
	}
}

func handleRemoteEnrollmentCreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			DeviceName string `json:"device_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Generate enrollment token using the security package pattern
		rawToken, err := generateRemoteToken()
		if err != nil {
			jsonError(w, "token generation failed", http.StatusInternalServerError)
			return
		}

		tokenHash := hashSHA256(rawToken)
		expires := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)

		enrollID, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
			TokenHash:  tokenHash,
			DeviceName: req.DeviceName,
			ExpiresAt:  expires,
		})
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Enrollment creation failed", "Failed to create remote enrollment", err)
			return
		}

		writeJSON(w, map[string]string{
			"enrollment_id": enrollID,
			"token":         rawToken,
			"expires_at":    expires,
		})
	}
}

func handleRemoteDownload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		// Rate-limit unauthenticated download requests to prevent enrollment token
		// flooding (max 5 downloads per 10 minutes per IP).
		clientIP := ClientIP(r, s.Cfg.Server.HTTPS.BehindProxy)
		if IsLockedOut(clientIP) {
			jsonError(w, "Too many download requests — please wait before trying again", http.StatusTooManyRequests)
			return
		}
		RecordFailedLogin(clientIP, 5, 10)

		// Parse /api/remote/download/{os}/{arch}?token=...&name=...
		path := strings.TrimPrefix(r.URL.Path, "/api/remote/download/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			jsonError(w, "invalid path, expected /api/remote/download/{os}/{arch}", http.StatusBadRequest)
			return
		}
		targetOS := parts[0]
		targetArch := parts[1]

		// Validate OS/Arch
		validPlatforms := map[string]bool{
			"linux/amd64": true, "linux/arm64": true,
			"darwin/amd64": true, "darwin/arm64": true,
			"windows/amd64": true, "windows/arm64": true,
		}
		platform := targetOS + "/" + targetArch
		if !validPlatforms[platform] {
			jsonError(w, "Unsupported platform", http.StatusBadRequest)
			return
		}

		// Find generic binary
		ext := ""
		if targetOS == "windows" {
			ext = ".exe"
		}
		binaryName := fmt.Sprintf("aurago-remote_%s_%s%s", targetOS, targetArch, ext)
		binaryPath := filepath.Join("deploy", binaryName)

		genericBinary, err := os.ReadFile(binaryPath)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Requested binary is not available", "Remote binary not available", err, "binary", binaryName)
			return
		}

		// Generate enrollment token for this download
		rawToken, err := generateRemoteToken()
		if err != nil {
			jsonError(w, "token generation failed", http.StatusInternalServerError)
			return
		}

		tokenHash := hashSHA256(rawToken)
		deviceName := r.URL.Query().Get("name")
		expires := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)

		_, err = remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
			TokenHash:  tokenHash,
			DeviceName: deviceName,
			ExpiresAt:  expires,
		})
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Enrollment creation failed", "Failed to create download enrollment", err, "platform", platform)
			return
		}

		// Determine supervisor URL.
		// Priority: explicit bridge_address (non-localhost) > Host header > localhost fallback.
		// We skip bridge_address when it points to localhost/127.0.0.1 because that value
		// is used for the lifeboat IPC TCP bridge — it is meaningless for remote clients.
		var supervisorURL string
		bridgeAddr := s.Cfg.Server.BridgeAddress
		bridgeIsLocal := bridgeAddr == "" ||
			strings.HasPrefix(bridgeAddr, "localhost:") ||
			strings.HasPrefix(bridgeAddr, "127.0.0.1:")
		if !bridgeIsLocal {
			supervisorURL = fmt.Sprintf("ws://%s/api/remote/ws", bridgeAddr)
		} else if host := r.Host; host != "" {
			// r.Host contains the hostname (and port) the client used to reach this server.
			// Strip any existing port and re-attach the configured server port so the
			// WebSocket URL is always correct regardless of proxies or port forwarding.
			scheme := "ws"
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "wss"
			}
			hostname := host
			if h, _, err := net.SplitHostPort(host); err == nil {
				hostname = h
			}
			supervisorURL = fmt.Sprintf("%s://%s:%d/api/remote/ws", scheme, hostname, s.Cfg.Server.Port)
		} else {
			supervisorURL = fmt.Sprintf("ws://localhost:%d/api/remote/ws", s.Cfg.Server.Port)
		}

		// Build personalized binary with trailer
		personalBinary, err := remote.BuildPersonalizedBinary(genericBinary, remote.BinaryConfig{
			SupervisorURL: supervisorURL,
			EnrollToken:   rawToken,
			DeviceName:    deviceName,
		})
		if err != nil {
			jsonError(w, "binary personalization failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, binaryName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(personalBinary)))
		w.Write(personalBinary)
	}
}

func handleRemoteAuditLog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.RemoteHub == nil {
			jsonError(w, "Remote Control not available", http.StatusServiceUnavailable)
			return
		}

		deviceID := r.URL.Query().Get("device_id")
		entries, err := remote.ListAuditLog(s.RemoteHub.DB(), deviceID, 100)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load audit log", "Failed to list remote audit log", err, "device_id", deviceID)
			return
		}
		writeJSON(w, entries)
	}
}

func handleRemotePlatforms(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type platformInfo struct {
			OS        string `json:"os"`
			Arch      string `json:"arch"`
			Available bool   `json:"available"`
			FileName  string `json:"file_name"`
		}

		platforms := []struct{ os, arch string }{
			{"linux", "amd64"}, {"linux", "arm64"},
			{"darwin", "amd64"}, {"darwin", "arm64"},
			{"windows", "amd64"}, {"windows", "arm64"},
		}

		var result []platformInfo
		for _, p := range platforms {
			ext := ""
			if p.os == "windows" {
				ext = ".exe"
			}
			name := fmt.Sprintf("aurago-remote_%s_%s%s", p.os, p.arch, ext)
			path := filepath.Join("deploy", name)
			_, err := os.Stat(path)
			result = append(result, platformInfo{
				OS:        p.os,
				Arch:      p.arch,
				Available: err == nil,
				FileName:  name,
			})
		}
		writeJSON(w, result)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func extractRemoteDeviceID(urlPath, suffix string) string {
	// /api/remote/devices/{id}/{suffix}
	path := strings.TrimPrefix(urlPath, "/api/remote/devices/")
	path = strings.TrimSuffix(path, suffix)
	path = strings.TrimSuffix(path, "/")
	return path
}

func generateRemoteToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "remote_" + hex.EncodeToString(b), nil
}

func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
