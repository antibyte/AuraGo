package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		EggPort:      egg.EggPort,
		Permanent:    egg.Permanent,
		IncludeVault: egg.IncludeVault,
		VaultData:    vaultData,
		MasterKey:    eggMasterKey,
	}

	// 9. Create deployment history record
	binaryHash := hashFile(binaryPath)
	configHash := hashBytes(cfgYAML)
	var deployID string
	if s.InvasionDB != nil {
		deployID, _ = invasion.CreateDeployment(s.InvasionDB, nest.ID, egg.ID, nest.DeployMethod, binaryHash, configHash)
	}

	// 10. Get connector and deploy
	connector := invasion.GetConnector(nest)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := connector.Deploy(ctx, nest, secretBytes, payload); err != nil {
		if deployID != "" {
			_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "failed")
		}
		return err
	}

	if deployID != "" {
		_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "deployed")
	}

	// 11. Health check with brief stabilization delay
	time.Sleep(3 * time.Second)
	if err := connector.HealthCheck(ctx, nest, secretBytes); err != nil {
		s.Logger.Warn("Health check failed after deploy, attempting auto-rollback", "nest_id", nest.ID, "error", err)
		if deployID != "" {
			_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "failed")
		}
		if rbErr := connector.Rollback(ctx, nest, secretBytes); rbErr != nil {
			s.Logger.Error("Auto-rollback failed", "nest_id", nest.ID, "error", rbErr)
			if deployID != "" {
				_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "failed")
			}
			return fmt.Errorf("deploy health check failed (%w) and rollback also failed: %v", err, rbErr)
		}
		if deployID != "" {
			_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "rolled_back")
		}
		return fmt.Errorf("deploy health check failed, rolled back: %w", err)
	}

	if deployID != "" {
		_ = invasion.UpdateDeploymentStatus(s.InvasionDB, deployID, "verified")
	}

	// 12. Store shared key in vault for WebSocket auth
	if err := s.Vault.WriteSecret("egg_shared_"+nest.ID, sharedKey); err != nil {
		s.Logger.Warn("Failed to store egg shared key in vault", "nest_id", nest.ID, "error", err)
	}

	return nil
}

// hashFile returns the SHA-256 hex hash of a file using streaming to avoid
// loading the entire (potentially large) binary into memory.
// Returns empty string on error.
func hashFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// hashBytes returns the SHA-256 hex hash of a byte slice.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
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
	// Eggs and remote agents do not send Origin. Browser-originated upgrades
	// must be same-origin before the post-upgrade HMAC auth is considered.
	CheckOrigin: sameOriginOrNoOrigin,
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

		// Persist task in DB before sending
		if s.InvasionDB != nil {
			eggID := ""
			if conn := s.EggHub.GetConnection(id); conn != nil {
				eggID = conn.EggID
			}
			taskID, err := invasion.CreateTask(s.InvasionDB, id, eggID, req.Description, req.Timeout)
			if err != nil {
				s.Logger.Warn("Failed to persist task", "error", err)
			} else {
				taskPayload.TaskID = taskID
			}
		}

		if err := s.EggHub.SendTask(id, taskPayload); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to send task", "Failed to send invasion task", err, "nest_id", id, "task_id", taskPayload.TaskID)
			return
		}

		// Mark as sent
		if s.InvasionDB != nil {
			_ = invasion.UpdateTaskStatus(s.InvasionDB, taskPayload.TaskID, "sent", "", "")
		}

		s.Logger.Info("Task sent to egg", "nest_id", id, "task_id", taskPayload.TaskID)
		writeJSON(w, map[string]interface{}{
			"status":  "sent",
			"task_id": taskPayload.TaskID,
			"nest_id": id,
		})
	}
}

// ── Key Rotation API ────────────────────────────────────────────────────────

func handleInvasionNestRotateKey(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "rotate-key")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		if !s.EggHub.IsConnected(id) {
			jsonError(w, "No active WebSocket connection to this nest", http.StatusConflict)
			return
		}

		// Generate new key
		newKey, err := invasion.GenerateSharedKey()
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to generate key", "Key generation failed", err, "nest_id", id)
			return
		}

		// Send rekey to egg via hub (encrypts with current key, rotates)
		if err := s.EggHub.SendRekey(id, newKey); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to send rekey", "Rekey message failed", err, "nest_id", id)
			return
		}

		// Update vault with new key
		if err := s.Vault.WriteSecret("egg_shared_"+id, newKey); err != nil {
			s.Logger.Error("Failed to update vault after rekey — key mismatch possible!", "nest_id", id, "error", err)
			jsonError(w, "Key rotated on egg but vault update failed — check logs", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("Shared key rotated successfully", "nest_id", id)
		writeJSON(w, map[string]interface{}{
			"status":  "rotated",
			"nest_id": id,
		})
	}
}

// ── Task history API ────────────────────────────────────────────────────────

func handleInvasionNestTasks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "tasks")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		tasks, err := invasion.GetTasksByNest(s.InvasionDB, id, 100)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to get tasks", "Failed to query invasion tasks", err, "nest_id", id)
			return
		}
		if tasks == nil {
			tasks = []invasion.TaskRecord{}
		}
		writeJSON(w, tasks)
	}
}

func handleInvasionTask(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/invasion/tasks/")
		id = strings.TrimSuffix(id, "/")
		if id == "" {
			jsonError(w, "Missing task ID", http.StatusBadRequest)
			return
		}

		task, err := invasion.GetTaskByID(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Task not found", http.StatusNotFound)
			return
		}
		writeJSON(w, task)
	}
}

// ── Rollback API ────────────────────────────────────────────────────────────

func handleInvasionNestRollback(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "rollback")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Nest not found", http.StatusNotFound)
			return
		}

		var secretBytes []byte
		if nest.VaultSecretID != "" {
			secretStr, _ := s.Vault.ReadSecret(nest.VaultSecretID)
			secretBytes = []byte(secretStr)
		}

		connector := invasion.GetConnector(nest)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := connector.Rollback(ctx, nest, secretBytes); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Rollback failed", "Invasion rollback failed", err, "nest_id", id)
			return
		}

		// Mark the last deployment as rolled back
		if last, err := invasion.GetLastSuccessfulDeployment(s.InvasionDB, id); err == nil {
			_ = invasion.UpdateDeploymentStatus(s.InvasionDB, last.ID, "rolled_back")
		}

		_ = invasion.UpdateNestHatchStatus(s.InvasionDB, id, "running", "")
		s.Logger.Info("Rollback completed", "nest_id", id)

		writeJSON(w, map[string]interface{}{
			"status":  "rolled_back",
			"nest_id": id,
		})
	}
}

// ── Deployment history API ──────────────────────────────────────────────────

func handleInvasionNestDeployments(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "deployments")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		deployments, err := invasion.GetDeploymentHistory(s.InvasionDB, id, 50)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to get deployment history", "Invasion deployment history query failed", err, "nest_id", id)
			return
		}
		if deployments == nil {
			deployments = []invasion.DeploymentRecord{}
		}
		writeJSON(w, deployments)
	}
}

// ── Safe Config Reconfigure API ─────────────────────────────────────────────

// handleInvasionNestSafeReconfigure applies a safe config patch to a running egg.
// POST /api/invasion/nests/{id}/safe-reconfigure
// Body: SafeConfigPatch JSON
func handleInvasionNestSafeReconfigure(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "safe-reconfigure")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		// Parse patch
		var patch invasion.SafeConfigPatch
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			jsonError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate patch
		if err := invasion.ValidateSafeConfigPatch(patch); err != nil {
			jsonError(w, "Validation failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Get nest and egg
		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Nest not found", http.StatusNotFound)
			return
		}
		if nest.EggID == "" {
			jsonError(w, "No egg assigned to this nest", http.StatusBadRequest)
			return
		}
		egg, err := invasion.GetEgg(s.InvasionDB, nest.EggID)
		if err != nil {
			jsonError(w, "Egg not found", http.StatusNotFound)
			return
		}

		// Generate current config (same as deployEgg)
		sharedKey, _ := s.Vault.ReadSecret("egg_shared_" + nest.ID)
		if sharedKey == "" {
			jsonError(w, "Shared key not found — re-hatch required", http.StatusConflict)
			return
		}
		masterURL := invasion.ResolveMasterURL(s.Cfg, nest)
		eggMasterKey, _ := s.Vault.ReadSecret("egg_master_key_" + nest.ID)
		if eggMasterKey == "" {
			eggMasterKey, _ = invasion.GenerateSharedKey()
		}

		cfgYAML, err := invasion.GenerateEggConfig(s.Cfg, egg, nest, sharedKey, masterURL, eggMasterKey)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to generate config", "Safe reconfigure config generation failed", err, "nest_id", id)
			return
		}

		// Apply patch
		patchedYAML, err := invasion.ApplySafeConfigPatch(cfgYAML, patch)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to apply patch", "Safe reconfigure patch failed", err, "nest_id", id)
			return
		}

		// Create revision record
		patchJSON, _ := invasion.PatchToJSON(patch)
		configHash := invasion.HashConfigYAML(patchedYAML)
		revID, err := invasion.CreateSafeConfigRevision(s.InvasionDB, nest.ID, nest.EggID, patchJSON, configHash, "safe_reconfigure")
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create revision", "Safe reconfigure revision creation failed", err, "nest_id", id)
			return
		}

		// Set desired config rev on nest
		if err := invasion.UpdateNestConfigRev(s.InvasionDB, nest.ID, revID, nest.AppliedConfigRev); err != nil {
			s.Logger.Warn("Failed to update nest desired_config_rev", "nest_id", id, "error", err)
		}

		// Mark revision as applying
		_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, revID, "applying", "")

		// Get nest secret for connector
		var secretBytes []byte
		if nest.VaultSecretID != "" {
			secretStr, err := s.Vault.ReadSecret(nest.VaultSecretID)
			if err != nil {
				_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, revID, "failed", "failed to read nest secret")
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read nest secret", "Safe reconfigure secret read failed", err, "nest_id", id)
				return
			}
			secretBytes = []byte(secretStr)
		}

		// Execute reconfigure via connector
		connector := invasion.GetConnector(nest)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := connector.Reconfigure(ctx, nest, secretBytes, patchedYAML); err != nil {
			_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, revID, "failed", err.Error())
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Reconfigure failed: "+err.Error(), "Safe reconfigure connector failed", err, "nest_id", id)
			return
		}

		// Health check
		time.Sleep(3 * time.Second)
		if err := connector.HealthCheck(ctx, nest, secretBytes); err != nil {
			_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, revID, "failed", "health check after reconfigure failed: "+err.Error())
			s.Logger.Warn("Health check failed after safe reconfigure", "nest_id", id, "error", err)
			jsonError(w, "Reconfigure applied but health check failed: "+err.Error(), http.StatusConflict)
			return
		}

		// Mark as applied
		_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, revID, "applied", "")
		_ = invasion.UpdateNestConfigRev(s.InvasionDB, nest.ID, revID, revID)

		writeJSON(w, map[string]interface{}{
			"status":      "applied",
			"revision_id": revID,
			"config_hash": configHash,
			"nest_id":     id,
		})
	}
}

// handleInvasionNestConfigHistory returns the safe config revision history for a nest.
// GET /api/invasion/nests/{id}/config-history?limit=20
func handleInvasionNestConfigHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "config-history")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := fmt.Sscanf(l, "%d", &limit); err != nil || parsed != 1 {
				limit = 20
			}
		}

		revs, err := invasion.ListSafeConfigRevisions(s.InvasionDB, id, limit)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list config history", "Config history query failed", err, "nest_id", id)
			return
		}
		if revs == nil {
			revs = []invasion.SafeConfigRevision{}
		}
		writeJSON(w, revs)
	}
}

// handleInvasionNestConfigRollback rolls back to a previous safe config revision.
// POST /api/invasion/nests/{id}/config-rollback
// Body: {"revision_id": "..."}
func handleInvasionNestConfigRollback(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := extractNestSubID(r.URL.Path, "config-rollback")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}

		var req struct {
			RevisionID string `json:"revision_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.RevisionID == "" {
			jsonError(w, "Missing revision_id", http.StatusBadRequest)
			return
		}

		// Get the target revision
		rev, err := invasion.GetSafeConfigRevision(s.InvasionDB, req.RevisionID)
		if err != nil {
			jsonError(w, "Revision not found: "+err.Error(), http.StatusNotFound)
			return
		}
		if rev.NestID != id {
			jsonError(w, "Revision does not belong to this nest", http.StatusBadRequest)
			return
		}
		if rev.Status != "applied" {
			jsonError(w, "Can only roll back to an applied revision", http.StatusBadRequest)
			return
		}

		// Get nest and egg
		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonError(w, "Nest not found", http.StatusNotFound)
			return
		}
		if nest.EggID == "" {
			jsonError(w, "No egg assigned to this nest", http.StatusBadRequest)
			return
		}
		egg, err := invasion.GetEgg(s.InvasionDB, nest.EggID)
		if err != nil {
			jsonError(w, "Egg not found", http.StatusNotFound)
			return
		}

		// Re-generate config at the target revision's patch state
		// We re-apply the target revision's patch to a fresh config
		patch, err := invasion.JSONToPatch(rev.PatchJSON)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to parse revision patch", "Config rollback patch parse failed", err, "nest_id", id)
			return
		}

		sharedKey, _ := s.Vault.ReadSecret("egg_shared_" + nest.ID)
		if sharedKey == "" {
			jsonError(w, "Shared key not found — re-hatch required", http.StatusConflict)
			return
		}
		masterURL := invasion.ResolveMasterURL(s.Cfg, nest)
		eggMasterKey, _ := s.Vault.ReadSecret("egg_master_key_" + nest.ID)
		if eggMasterKey == "" {
			eggMasterKey, _ = invasion.GenerateSharedKey()
		}

		cfgYAML, err := invasion.GenerateEggConfig(s.Cfg, egg, nest, sharedKey, masterURL, eggMasterKey)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to generate config", "Config rollback config generation failed", err, "nest_id", id)
			return
		}

		patchedYAML, err := invasion.ApplySafeConfigPatch(cfgYAML, patch)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to apply patch", "Config rollback patch apply failed", err, "nest_id", id)
			return
		}

		// Create a rollback revision record
		patchJSON, _ := invasion.PatchToJSON(patch)
		configHash := invasion.HashConfigYAML(patchedYAML)
		rollbackRevID, err := invasion.CreateSafeConfigRevision(s.InvasionDB, nest.ID, nest.EggID, patchJSON, configHash, "rollback")
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create rollback revision", "Config rollback revision creation failed", err, "nest_id", id)
			return
		}

		_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, rollbackRevID, "applying", "")

		// Get nest secret
		var secretBytes []byte
		if nest.VaultSecretID != "" {
			secretStr, err := s.Vault.ReadSecret(nest.VaultSecretID)
			if err != nil {
				_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, rollbackRevID, "failed", "failed to read nest secret")
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read nest secret", "Config rollback secret read failed", err, "nest_id", id)
				return
			}
			secretBytes = []byte(secretStr)
		}

		// Execute reconfigure via connector
		connector := invasion.GetConnector(nest)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := connector.Reconfigure(ctx, nest, secretBytes, patchedYAML); err != nil {
			_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, rollbackRevID, "failed", err.Error())
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Rollback reconfigure failed: "+err.Error(), "Config rollback connector failed", err, "nest_id", id)
			return
		}

		// Health check
		time.Sleep(3 * time.Second)
		if err := connector.HealthCheck(ctx, nest, secretBytes); err != nil {
			_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, rollbackRevID, "failed", "health check after rollback failed: "+err.Error())
			jsonError(w, "Rollback applied but health check failed: "+err.Error(), http.StatusConflict)
			return
		}

		// Mark rollback as applied, mark old current as rolled_back
		_ = invasion.UpdateSafeConfigRevisionStatus(s.InvasionDB, rollbackRevID, "applied", "")
		_ = invasion.UpdateNestConfigRev(s.InvasionDB, nest.ID, rollbackRevID, rollbackRevID)

		writeJSON(w, map[string]interface{}{
			"status":               "rolled_back",
			"rollback_revision_id": rollbackRevID,
			"target_revision_id":   req.RevisionID,
			"config_hash":          configHash,
			"nest_id":              id,
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
