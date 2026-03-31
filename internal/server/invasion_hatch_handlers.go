package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"

	"github.com/gorilla/websocket"
)

// ── Hatch (deploy) an egg to a nest ─────────────────────────────────────────

func handleInvasionNestHatch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "hatch")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Nest not found", "Invasion hatch nest lookup failed", err, "nest_id", id)
			return
		}
		if !nest.Active {
			jsonError(w, "Nest is inactive", http.StatusBadRequest)
			return
		}
		if nest.EggID == "" {
			jsonError(w, "No egg assigned to this nest", http.StatusBadRequest)
			return
		}
		if nest.HatchStatus == "hatching" {
			jsonError(w, "Hatch already in progress", http.StatusConflict)
			return
		}

		egg, err := invasion.GetEgg(s.InvasionDB, nest.EggID)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Assigned egg not found", "Invasion hatch egg lookup failed", err, "nest_id", id, "egg_id", nest.EggID)
			return
		}

		// Mark hatching status
		_ = invasion.UpdateNestHatchStatus(s.InvasionDB, id, "hatching", "")

		// Run deployment in background
		go func() {
			if err := s.deployEgg(nest, egg); err != nil {
				s.Logger.Error("Egg deployment failed", "nest_id", id, "error", err)
				_ = invasion.UpdateNestHatchStatus(s.InvasionDB, id, "failed", err.Error())
			} else {
				s.Logger.Info("Egg deployed successfully", "nest_id", id, "egg_id", egg.ID)
				_ = invasion.UpdateNestHatchStatus(s.InvasionDB, id, "running", "")
				// Fire mission trigger: egg hatched
				if s.MissionManagerV2 != nil {
					s.MissionManagerV2.NotifyInvasionEvent("egg_hatched", id, nest.Name, egg.ID, egg.Name)
				}
			}
		}()

		writeJSON(w, map[string]interface{}{
			"status":  "hatching",
			"nest_id": id,
			"egg_id":  egg.ID,
		})
	}
}

// deployEgg performs the actual deployment of an egg to a nest.
func (s *Server) deployEgg(nest invasion.NestRecord, egg invasion.EggRecord) error {
	// 1. Generate shared key for master↔egg HMAC
	sharedKey, err := invasion.GenerateSharedKey()
	if err != nil {
		return fmt.Errorf("failed to generate shared key: %w", err)
	}

	// 2. Resolve the master URL the egg should connect to
	masterURL := invasion.ResolveMasterURL(s.Cfg, nest)

	// 3. Optionally export vault
	var vaultData []byte
	var eggMasterKey string
	if egg.IncludeVault {
		vaultData, eggMasterKey, err = invasion.ExportVaultForEgg(s.Vault)
		if err != nil {
			return fmt.Errorf("failed to export vault: %w", err)
		}
	} else {
		// Generate a fresh master key for the egg anyway (needed for vault init)
		eggMasterKey, err = invasion.GenerateSharedKey()
		if err != nil {
			return fmt.Errorf("failed to generate egg master key: %w", err)
		}
	}

	// 4. Generate egg config.yaml
	cfgYAML, err := invasion.GenerateEggConfig(s.Cfg, egg, nest, sharedKey, masterURL, eggMasterKey)
	if err != nil {
		return fmt.Errorf("failed to generate egg config: %w", err)
	}

	// 5. Resolve binary path for target architecture
	binaryPath, err := resolveBinaryPath(nest.TargetArch)
	if err != nil {
		return fmt.Errorf("failed to resolve binary: %w", err)
	}

	// 6. Resolve resources.dat path
	exePath, _ := os.Executable()
	installDir := filepath.Dir(exePath)
	resourcesPath := filepath.Join(installDir, "resources.dat")
	if _, err := os.Stat(resourcesPath); os.IsNotExist(err) {
		altResourcesPath := filepath.Join(installDir, "deploy", "resources.dat")
		if _, err := os.Stat(altResourcesPath); err == nil {
			resourcesPath = altResourcesPath
		} else {
			resourcesPath = "" // no resources to transfer
		}
	}

	// 7. Get nest secret from vault
	var secretBytes []byte
	if nest.VaultSecretID != "" {
		secretStr, err := s.Vault.ReadSecret(nest.VaultSecretID)
		if err != nil {
			return fmt.Errorf("failed to read nest secret: %w", err)
		}
		secretBytes = []byte(secretStr)
	}

	// 8. Build deployment payload
	payload := invasion.EggDeployPayload{
		BinaryPath:   binaryPath,
		ConfigYAML:   cfgYAML,
		ResourcesPkg: resourcesPath,
		SharedKey:    sharedKey,
		Permanent:    egg.Permanent,
		IncludeVault: egg.IncludeVault,
		VaultData:    vaultData,
		MasterKey:    eggMasterKey,
	}

	// 9. Get connector and deploy
	connector := invasion.GetConnector(nest)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := connector.Deploy(ctx, nest, secretBytes, payload); err != nil {
		return err
	}

	// 10. Store shared key in vault for WebSocket auth
	if err := s.Vault.WriteSecret("egg_shared_"+nest.ID, sharedKey); err != nil {
		s.Logger.Warn("Failed to store egg shared key in vault", "nest_id", nest.ID, "error", err)
	}

	return nil
}

// resolveBinaryPath finds the correct AuraGo binary for the target architecture.
// Searches in multiple locations: install dir, parent dir, and common paths.
func resolveBinaryPath(targetArch string) (string, error) {
	exePath, _ := os.Executable()
	installDir := filepath.Dir(exePath)

	// Map target architectures to binary names
	binaryMap := map[string][]string{
		"linux/amd64": {"aurago_linux", "aurago"},
		"linux/arm64": {"aurago_linux_arm64", "aurago"},
	}

	binaryNames, ok := binaryMap[targetArch]
	if !ok {
		return "", fmt.Errorf("unsupported target architecture: %s", targetArch)
	}

	// Also check if current running binary can be used (for self-deployment)
	if exePath != "" {
		exePathClean := filepath.Clean(exePath)
		if info, err := os.Stat(exePathClean); err == nil && !info.IsDir() {
			// Current binary exists - check if name matches what we need
			exeName := filepath.Base(exePathClean)
			for _, expectedName := range binaryNames {
				if exeName == expectedName {
					return exePathClean, nil
				}
			}
		}
	}

	// Search paths in order of priority
	searchPaths := []string{
		installDir,                             // Current dir (e.g., /home/aurago/aurago/bin)
		filepath.Join(installDir, ".."),        // Parent dir
		filepath.Join(installDir, "..", "bin"), // Parent/bin
		filepath.Join(installDir, "deploy"),    // deploy subdir
		"/home/aurago/aurago/bin",              // Common install path
		"/home/aurago/aurago",                  // Parent of bin
		"/opt/aurago/bin",
		"/opt/aurago",
		"/usr/local/bin",
	}

	var checkedPaths []string
	var existingButNotExecutable []string

	for _, searchPath := range searchPaths {
		cleanPath := filepath.Clean(searchPath)
		for _, binaryName := range binaryNames {
			fullPath := filepath.Join(cleanPath, binaryName)
			checkedPaths = append(checkedPaths, fullPath)

			info, err := os.Stat(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue // File doesn't exist, try next
				}
				// Permission error - note it but continue
				existingButNotExecutable = append(existingButNotExecutable, fmt.Sprintf("%s (permission denied)", fullPath))
				continue
			}

			if info.IsDir() {
				continue // It's a directory, not a file
			}

			// File exists - check if executable
			if info.Mode()&0111 != 0 {
				return fullPath, nil
			}

			// File exists but isn't executable
			existingButNotExecutable = append(existingButNotExecutable, fmt.Sprintf("%s (not executable)", fullPath))
		}
	}

	// Build detailed error message
	errMsg := fmt.Sprintf("no binary found for %s (searched for: %v)", targetArch, binaryNames)
	if len(existingButNotExecutable) > 0 {
		errMsg += fmt.Sprintf("; found but not usable: %v", existingButNotExecutable)
	}
	errMsg += fmt.Sprintf("; checked paths: %v", checkedPaths)

	return "", fmt.Errorf("%s", errMsg)
}

// ── Stop a running egg ──────────────────────────────────────────────────────

func handleInvasionNestStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "stop")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Nest not found", http.StatusNotFound)
			return
		}

		// Try graceful stop via WebSocket first
		if s.EggHub.IsConnected(id) {
			if err := s.EggHub.SendStop(id); err != nil {
				s.Logger.Warn("Failed to send stop via WebSocket", "nest_id", id, "error", err)
			}
		}

		// Also stop via connector (process/container level)
		var secretBytes []byte
		if nest.VaultSecretID != "" {
			secretStr, _ := s.Vault.ReadSecret(nest.VaultSecretID)
			secretBytes = []byte(secretStr)
		}

		connector := invasion.GetConnector(nest)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := connector.Stop(ctx, nest, secretBytes); err != nil {
			s.Logger.Warn("Connector stop failed", "nest_id", id, "error", err)
		}

		_ = invasion.UpdateNestHatchStatus(s.InvasionDB, id, "stopped", "")

		writeJSON(w, map[string]interface{}{
			"status":  "stopped",
			"nest_id": id,
		})
	}
}

// ── Hatch status ────────────────────────────────────────────────────────────

func handleInvasionNestHatchStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "status")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Nest not found", http.StatusNotFound)
			return
		}

		connected := s.EggHub.IsConnected(id)
		var telemetry interface{} = nil
		if connected {
			if c := s.EggHub.GetConnection(id); c != nil {
				telemetry = c.GetTelemetry()
			}
		}

		writeJSON(w, map[string]interface{}{
			"nest_id":       id,
			"hatch_status":  nest.HatchStatus,
			"last_hatch_at": nest.LastHatchAt,
			"hatch_error":   nest.HatchError,
			"ws_connected":  connected,
			"telemetry":     telemetry,
		})
	}
}

// ── Send secret to a running egg ────────────────────────────────────────────

func handleInvasionNestSendSecret(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "send-secret")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
			jsonError(w, "Invalid request: key and value required", http.StatusBadRequest)
			return
		}

		if !s.EggHub.IsConnected(id) {
			jsonError(w, "No active WebSocket connection to this nest", http.StatusConflict)
			return
		}

		// Read the shared key from vault
		sharedKey, err := s.Vault.ReadSecret("egg_shared_" + id)
		if err != nil {
			jsonError(w, "Failed to retrieve shared key", http.StatusInternalServerError)
			return
		}

		// Encrypt the value with the shared key (not the vault master key!)
		// The egg will decrypt with the same shared key before storing in its vault.
		encrypted, err := bridge.EncryptWithSharedKey([]byte(req.Value), sharedKey)
		if err != nil {
			jsonError(w, "Failed to encrypt secret", http.StatusInternalServerError)
			return
		}

		if err := s.EggHub.SendSecret(id, req.Key, encrypted); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to send secret", "Failed to send invasion secret", err, "nest_id", id)
			return
		}

		writeJSON(w, map[string]string{"status": "sent"})
	}
}

// ── WebSocket upgrade for egg connections ───────────────────────────────────

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// CheckOrigin validated in handleInvasionWebSocket via HMAC auth.
	// Allow all origins here because eggs connect from arbitrary hosts
	// and authenticate via per-nest shared key HMAC in the first message.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleInvasionWebSocket(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil {
			jsonError(w, "Invasion Control is not enabled", http.StatusServiceUnavailable)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			s.Logger.Error("WebSocket upgrade failed", "error", err)
			return
		}

		// Wait for auth message
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			s.Logger.Warn("No auth message received", "error", err)
			conn.Close()
			return
		}
		conn.SetReadDeadline(time.Time{}) // reset deadline

		var authMsg bridge.Message
		if err := json.Unmarshal(data, &authMsg); err != nil || authMsg.Type != bridge.MsgAuth {
			s.Logger.Warn("Invalid auth message from egg")
			conn.Close()
			return
		}

		// Validate egg and nest
		nest, err := invasion.GetNest(s.InvasionDB, authMsg.NestID)
		if err != nil {
			s.Logger.Warn("Auth failed: nest not found", "nest_id", authMsg.NestID)
			conn.Close()
			return
		}

		egg, err := invasion.GetEgg(s.InvasionDB, authMsg.EggID)
		if err != nil {
			s.Logger.Warn("Auth failed: egg not found", "egg_id", authMsg.EggID)
			conn.Close()
			return
		}

		if !egg.Active {
			s.Logger.Warn("Auth failed: egg is inactive", "egg_id", authMsg.EggID)
			conn.Close()
			return
		}

		// Read shared key from vault
		sharedKey, err := s.Vault.ReadSecret("egg_shared_" + nest.ID)
		if err != nil {
			s.Logger.Warn("Auth failed: shared key not found", "nest_id", nest.ID)
			conn.Close()
			return
		}

		// Verify HMAC
		ok, err := bridge.VerifyMessage(authMsg, sharedKey)
		if err != nil || !ok {
			s.Logger.Warn("Auth failed: HMAC mismatch", "nest_id", nest.ID)
			conn.Close()
			return
		}

		// Send ack
		ackMsg, err := bridge.NewMessage(bridge.MsgAck, authMsg.EggID, authMsg.NestID, sharedKey,
			bridge.AckPayload{RefID: authMsg.ID, Success: true, Detail: "authenticated"})
		if err == nil {
			_ = conn.WriteJSON(ackMsg)
		}

		// Register connection
		eggConn := &bridge.EggConnection{
			Conn:          conn,
			EggID:         authMsg.EggID,
			NestID:        authMsg.NestID,
			SharedKey:     sharedKey,
			LastHeartbeat: time.Now(),
			Status:        "connected",
		}
		if err := s.EggHub.Register(nest.ID, eggConn); err != nil {
			s.Logger.Warn("Connection limit reached", "nest_id", nest.ID, "error", err)
			conn.Close()
			return
		}

		// Update nest status to running
		_ = invasion.UpdateNestHatchStatus(s.InvasionDB, nest.ID, "running", "")

		// Block on read loop
		s.EggHub.HandleMessages(eggConn)
	}
}

// ── Send task to a running egg ──────────────────────────────────────────────

func handleInvasionNestSendTask(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "send-task")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Description string `json:"description"`
			Timeout     int    `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
			jsonError(w, "Invalid request: description is required", http.StatusBadRequest)
			return
		}

		if !s.EggHub.IsConnected(id) {
			jsonError(w, "No active WebSocket connection to this nest", http.StatusConflict)
			return
		}

		taskPayload := bridge.TaskPayload{
			TaskID:      fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Description: req.Description,
			Timeout:     req.Timeout,
		}

		if err := s.EggHub.SendTask(id, taskPayload); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to send task", "Failed to send invasion task", err, "nest_id", id, "task_id", taskPayload.TaskID)
			return
		}

		s.Logger.Info("Task sent to egg", "nest_id", id, "task_id", taskPayload.TaskID)
		writeJSON(w, map[string]interface{}{
			"status":  "sent",
			"task_id": taskPayload.TaskID,
			"nest_id": id,
		})
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// extractNestSubID extracts the nest ID from paths like /api/invasion/nests/{id}/{action}.
func extractNestSubID(urlPath, action string) string {
	prefix := "/api/invasion/nests/"
	rest := strings.TrimPrefix(urlPath, prefix)
	rest = strings.TrimSuffix(rest, "/"+action)
	rest = strings.TrimSuffix(rest, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}
