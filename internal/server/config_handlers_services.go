package server

import (
	"aurago/internal/tools"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
)

// handleAnsibleGenerateToken generates a cryptographically secure random token,
// saves it to the vault, and (if the sidecar is already running) recreates the
// container so the new token is active immediately.
func handleAnsibleGenerateToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate a 32-byte cryptographically secure random token
		b := make([]byte, 32)
		if _, err := crand.Read(b); err != nil {
			s.Logger.Error("[Ansible] Failed to generate token", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
			return
		}
		token := hex.EncodeToString(b)

		// Persist to vault
		if s.Vault != nil {
			if err := s.Vault.WriteSecret("ansible_token", token); err != nil {
				s.Logger.Error("[Ansible] Failed to save token to vault", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save token to vault"})
				return
			}
			s.Logger.Info("[Config] Secret saved to vault", "key", "ansible_token")
		}

		// Update live config
		s.CfgMu.Lock()
		s.Cfg.Ansible.Token = token
		cfg := *s.Cfg
		s.CfgMu.Unlock()

		// If sidecar is enabled, recreate the container with the new token
		if cfg.Ansible.Enabled && cfg.Ansible.Mode == "sidecar" {
			inventoryDir := ""
			if cfg.Ansible.DefaultInventory != "" {
				inventoryDir = filepath.Dir(cfg.Ansible.DefaultInventory)
			}
			go tools.ReapplyAnsibleToken(cfg.Docker.Host, tools.AnsibleSidecarConfig{
				Token:         token,
				Timeout:       cfg.Ansible.Timeout,
				Image:         cfg.Ansible.Image,
				ContainerName: cfg.Ansible.ContainerName,
				PlaybooksDir:  cfg.Ansible.PlaybooksDir,
				InventoryDir:  inventoryDir,
				AutoBuild:     cfg.Ansible.AutoBuild,
				DockerfileDir: cfg.Ansible.DockerfileDir,
			}, s.Logger)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"token":  token,
		})
	}
}

// handleOllamaManagedStatus returns the current status of the managed Ollama container.
func handleOllamaManagedStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		dockerHost := s.Cfg.Docker.Host
		managed := s.Cfg.Ollama.ManagedInstance.Enabled
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if !managed {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "disabled"})
			return
		}
		result := tools.OllamaManagedContainerStatus(dockerHost)
		w.Write([]byte(result))
	}
}

// handleOllamaManagedRecreate calls EnsureOllamaManagedRunning to create/start
// the managed Ollama container. This allows the user to recover after the
// container was manually deleted via the container management page.
func handleOllamaManagedRecreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.Ollama.ManagedInstance.Enabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Managed Ollama instance is not enabled."})
			return
		}

		s.Logger.Info("[Config UI] Ollama managed container recreate requested via Web UI")
		go tools.EnsureOllamaManagedRunning(&cfg, s.Logger)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Container creation started in background.",
		})
	}
}
