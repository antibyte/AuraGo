package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"aurago/internal/tools"
)

// handleHomepageStatus returns the status of the homepage dev and web containers.
func handleHomepageStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		dockerHost := s.Cfg.Docker.Host
		workspacePath := s.Cfg.Homepage.WorkspacePath
		webServerPort := s.Cfg.Homepage.WebServerPort
		webServerDomain := s.Cfg.Homepage.WebServerDomain
		allowLocalServer := s.Cfg.Homepage.AllowLocalServer
		homepageEnabled := s.Cfg.Homepage.Enabled
		webServerInternalOnly := s.Cfg.Homepage.WebServerInternalOnly
		s.CfgMu.RUnlock()

		if !homepageEnabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "disabled",
				"dev_container": map[string]interface{}{"running": false, "exists": false},
				"web_container": map[string]interface{}{"running": false, "exists": false},
			})
			return
		}

		cfg := tools.HomepageConfig{
			DockerHost:            dockerHost,
			WorkspacePath:         workspacePath,
			WebServerPort:         webServerPort,
			WebServerDomain:       webServerDomain,
			WebServerInternalOnly: webServerInternalOnly,
			AllowLocalServer:      allowLocalServer,
		}
		result := tools.HomepageStatus(cfg, s.Logger)

		var parsed map[string]interface{}
		if json.Unmarshal([]byte(result), &parsed) == nil {
			enrichHomepageStatusForRequest(parsed, webServerPort, webServerInternalOnly, r)

			// Inject tunnel URL when Cloudflare Tunnel is running.
			if tunnelURL := tools.GetTunnelURL(); tunnelURL != "" {
				parsed["tunnel_url"] = tunnelURL
				if homepageAnyServerRunning(parsed) {
					parsed["preview_url"] = tunnelURL
				}
			}
			enriched, err := json.Marshal(parsed)
			if err == nil {
				result = string(enriched)
			}
		}

		w.Write([]byte(result))
	}
}

func enrichHomepageStatusForRequest(payload map[string]interface{}, webServerPort int, internalOnly bool, r *http.Request) {
	if payload == nil || internalOnly {
		return
	}
	localURL := homepageBrowserURLForRequest(r, webServerPort)
	if localURL == "" {
		return
	}

	if webContainer, ok := homepageStatusObject(payload["web_container"]); ok && homepageStatusRunning(webContainer) {
		webContainer["browser_url"] = localURL
		payload["local_browser_url"] = localURL
		if _, exists := payload["preview_url"]; !exists {
			payload["preview_url"] = localURL
		}
	}
	if pythonServer, ok := homepageStatusObject(payload["python_server"]); ok && homepageStatusRunning(pythonServer) {
		pythonServer["browser_url"] = localURL
		payload["local_browser_url"] = localURL
		if _, exists := payload["preview_url"]; !exists {
			payload["preview_url"] = localURL
		}
	}
}

func homepageBrowserURLForRequest(r *http.Request, webServerPort int) string {
	if requestLooksTailscale(r) {
		return ""
	}
	if webServerPort <= 0 {
		webServerPort = 8080
	}
	return manifestURLWithRequestHost(fmt.Sprintf("http://127.0.0.1:%d", webServerPort), r)
}

func homepageStatusObject(value interface{}) (map[string]interface{}, bool) {
	obj, ok := value.(map[string]interface{})
	return obj, ok
}

func homepageStatusRunning(value map[string]interface{}) bool {
	running, ok := value["running"].(bool)
	return ok && running
}

func homepageAnyServerRunning(payload map[string]interface{}) bool {
	if webContainer, ok := homepageStatusObject(payload["web_container"]); ok && homepageStatusRunning(webContainer) {
		return true
	}
	if pythonServer, ok := homepageStatusObject(payload["python_server"]); ok && homepageStatusRunning(pythonServer) {
		return true
	}
	return false
}

// handleHomepageDetectWorkspace inspects the running homepage dev container and returns
// the host path that is bind-mounted as the workspace, so the UI can auto-fill the field.
func handleHomepageDetectWorkspace(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		dockerHost := s.Cfg.Docker.Host
		s.CfgMu.RUnlock()

		cfg := tools.HomepageConfig{DockerHost: dockerHost}
		w.Write([]byte(tools.HomepageDetectWorkspacePath(cfg, s.Logger)))
	}
}

// handleHomepageTestConnection tests the SFTP/SCP connection for homepage deployment.
func handleHomepageTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
