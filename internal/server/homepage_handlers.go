package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/tools"
)

// handleHomepageStatus returns the status of the homepage dev and web containers.
func handleHomepageStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		dockerHost := s.Cfg.Docker.Host
		workspacePath := s.Cfg.Homepage.WorkspacePath
		webServerPort := s.Cfg.Homepage.WebServerPort
		webServerDomain := s.Cfg.Homepage.WebServerDomain
		dockerEnabled := s.Cfg.Docker.Enabled
		homepageEnabled := s.Cfg.Homepage.Enabled
		s.CfgMu.RUnlock()

		if !dockerEnabled || !homepageEnabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "disabled",
				"dev_container": map[string]interface{}{"running": false, "exists": false},
				"web_container": map[string]interface{}{"running": false, "exists": false},
			})
			return
		}

		cfg := tools.HomepageConfig{
			DockerHost:      dockerHost,
			WorkspacePath:   workspacePath,
			WebServerPort:   webServerPort,
			WebServerDomain: webServerDomain,
		}
		result := tools.HomepageStatus(cfg, s.Logger)

		w.Write([]byte(result))
	}
}

// handleHomepageTestConnection tests the SFTP/SCP connection for homepage deployment.
func handleHomepageTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// Parse optional override values from body
		var body struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			User     string `json:"user"`
			Password string `json:"password"`
			Path     string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Fall back to saved config
		s.CfgMu.RLock()
		host := body.Host
		if host == "" {
			host = s.Cfg.Homepage.DeployHost
		}
		port := body.Port
		if port == 0 {
			port = s.Cfg.Homepage.DeployPort
		}
		if port == 0 {
			port = 22
		}
		user := body.User
		if user == "" {
			user = s.Cfg.Homepage.DeployUser
		}
		password := body.Password
		if password == "" {
			password = s.Cfg.Homepage.DeployPassword
		}
		deployKey := s.Cfg.Homepage.DeployKey
		deployPath := body.Path
		if deployPath == "" {
			deployPath = s.Cfg.Homepage.DeployPath
		}
		s.CfgMu.RUnlock()

		// Vault fallback
		if s.Vault != nil {
			if password == "" {
				if v, _ := s.Vault.ReadSecret("homepage_deploy_password"); v != "" {
					password = v
				}
			}
			if deployKey == "" {
				if v, _ := s.Vault.ReadSecret("homepage_deploy_key"); v != "" {
					deployKey = v
				}
			}
		}

		deployCfg := tools.HomepageDeployConfig{
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
			Key:      deployKey,
			Path:     deployPath,
		}

		result := tools.HomepageTestConnection(deployCfg, s.Logger)
		w.Write([]byte(result))
	}
}
