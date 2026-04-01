package tools

import (
	"aurago/internal/config"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
)

// InitiateLifeboatHandover sets the agent to busy mode, saves the update plan and state,
// and returns a sentinel string that the agent supervisor uses to trigger the actual process swap.
func InitiateLifeboatHandover(plan string, cfg *config.Config) string {
	// 1. Ensure directories exist
	dataDir := cfg.Directories.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Sprintf("Tool Output: ERROR creating data directory: %v", err)
	}

	SetBusy(true)

	// 2. Save plan to current_plan.md
	p := plan
	if p == "" {
		p = "No specific plan provided. Transitioning to maintenance mode."
	}
	planPath := filepath.Join(dataDir, "current_plan.md")
	if err := writeSensitiveMaintenanceFile(planPath, []byte(p)); err != nil {
		return fmt.Sprintf("Tool Output: ERROR saving plan: %v", err)
	}

	// 3. Save a minimal state.json
	statePath := filepath.Join(dataDir, "state.json")
	stateJSON := `{"status": "updating", "reason": "user_requested_upgrade"}`
	if err := writeSensitiveMaintenanceFile(statePath, []byte(stateJSON)); err != nil {
		return fmt.Sprintf("Tool Output: ERROR saving state: %v", err)
	}

	// 4. Signal Sidecar via TCP
	go func() {
		slog.Info("Signaling Lifeboat Sidecar...", "port", cfg.Maintenance.LifeboatPort)
		addr := fmt.Sprintf("localhost:%d", cfg.Maintenance.LifeboatPort)
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			slog.Error("Failed to signal Lifeboat Sidecar", "addr", addr, "error", err)
			return
		}
		defer conn.Close()

		cmd := map[string]string{"command": "start_operation"}
		data, _ := json.Marshal(cmd)
		data = append(data, '\n')
		if _, err := conn.Write(data); err != nil {
			slog.Error("Failed to write to Lifeboat Sidecar", "error", err)
		} else {
			slog.Info("Successfully signaled Lifeboat Sidecar")
		}
	}()

	return "Maintenance Mode activated. Use 'initiate_handover' only when ready. The Sidecar is now waiting for your signal."
}

func writeSensitiveMaintenanceFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return nil
}
