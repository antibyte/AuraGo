package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"aurago/internal/config"
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
		cfgSnapshot := *s.Cfg
		s.CfgMu.RUnlock()

		if !cfgSnapshot.Homepage.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "disabled",
				"dev_container": map[string]interface{}{"running": false, "exists": false},
				"web_container": map[string]interface{}{"running": false, "exists": false},
			})
			return
		}

		cfg := tools.HomepageConfig{
			DockerHost:            cfgSnapshot.Docker.Host,
			WorkspacePath:         cfgSnapshot.Homepage.WorkspacePath,
			WebServerPort:         cfgSnapshot.Homepage.WebServerPort,
			WebServerDomain:       cfgSnapshot.Homepage.WebServerDomain,
			WebServerInternalOnly: cfgSnapshot.Homepage.WebServerInternalOnly,
			AllowLocalServer:      cfgSnapshot.Homepage.AllowLocalServer,
		}
		result := tools.HomepageStatus(cfg, s.Logger)

		var parsed map[string]interface{}
		if json.Unmarshal([]byte(result), &parsed) == nil {
			enrichHomepageStatusForRequest(parsed, homepageStatusBrowserURL(s, &cfgSnapshot, r))

			// Inject tunnel URL when Cloudflare Tunnel is running.
			if tunnelURL := tools.GetTunnelURL(); tunnelURL != "" {
				parsed["tunnel_url"] = tunnelURL
				if homepageAnyServerRunning(parsed) {
					if _, exists := parsed["preview_url"]; !exists {
						parsed["preview_url"] = tunnelURL
					}
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

func homepageConfigFromServer(s *Server) tools.HomepageConfig {
	s.CfgMu.RLock()
	cfgSnapshot := *s.Cfg
	s.CfgMu.RUnlock()
	workspacePath := cfgSnapshot.Homepage.WorkspacePath
	if workspacePath == "" {
		workspacePath = filepath.Join(cfgSnapshot.Directories.DataDir, "homepage")
	}
	return tools.HomepageConfig{
		DockerHost:            cfgSnapshot.Docker.Host,
		WorkspacePath:         workspacePath,
		AgentWorkspaceDir:     cfgSnapshot.Directories.WorkspaceDir,
		DataDir:               cfgSnapshot.Directories.DataDir,
		WebServerPort:         cfgSnapshot.Homepage.WebServerPort,
		WebServerDomain:       cfgSnapshot.Homepage.WebServerDomain,
		WebServerInternalOnly: cfgSnapshot.Homepage.WebServerInternalOnly,
		AllowLocalServer:      cfgSnapshot.Homepage.AllowLocalServer,
	}
}

// handleHomepageSites lists managed Homepage projects with current ledger state.
func handleHomepageSites(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.HomepageRegistryDB == nil {
			jsonError(w, "Homepage registry is not enabled or DB not initialized", http.StatusServiceUnavailable)
			return
		}
		sites, err := tools.ListHomepageManagedSites(s.HomepageRegistryDB)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"total":  len(sites),
			"sites":  sites,
		})
	}
}

// handleHomepageSiteByID returns one managed site or reconciles its ledger state.
func handleHomepageSiteByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.HomepageRegistryDB == nil {
			jsonError(w, "Homepage registry is not enabled or DB not initialized", http.StatusServiceUnavailable)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/homepage/sites/")
		reconcile := false
		if strings.HasSuffix(path, "/reconcile") {
			reconcile = true
			path = strings.TrimSuffix(path, "/reconcile")
		}
		id, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
		if err != nil || id <= 0 {
			jsonError(w, "invalid site id", http.StatusBadRequest)
			return
		}
		switch {
		case reconcile && r.Method != http.MethodPost:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		case !reconcile && r.Method != http.MethodGet:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		site, err := tools.GetHomepageManagedSite(s.HomepageRegistryDB, id)
		if err != nil {
			if err == sql.ErrNoRows {
				jsonError(w, "site not found", http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if reconcile {
			state, err := tools.ReconcileHomepageProject(homepageConfigFromServer(s), s.HomepageRegistryDB, site.ProjectDir, s.Logger)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			site, _ = tools.GetHomepageManagedSite(s.HomepageRegistryDB, id)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"site":   site,
				"state":  state,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"site":   site,
		})
	}
}

func enrichHomepageStatusForRequest(payload map[string]interface{}, browserURL string) {
	if payload == nil || browserURL == "" {
		return
	}

	if webContainer, ok := homepageStatusObject(payload["web_container"]); ok && homepageStatusRunning(webContainer) {
		webContainer["browser_url"] = browserURL
		payload["local_browser_url"] = browserURL
		if _, exists := payload["preview_url"]; !exists {
			payload["preview_url"] = browserURL
		}
	}
	if pythonServer, ok := homepageStatusObject(payload["python_server"]); ok && homepageStatusRunning(pythonServer) {
		pythonServer["browser_url"] = browserURL
		payload["local_browser_url"] = browserURL
		if _, exists := payload["preview_url"]; !exists {
			payload["preview_url"] = browserURL
		}
	}
}

func homepageStatusBrowserURL(s *Server, cfg *config.Config, r *http.Request) string {
	if cfg == nil {
		return ""
	}
	if cfg.Tailscale.TsNet.Enabled && cfg.Tailscale.TsNet.ExposeHomepage {
		if s != nil && s.TsNetManager != nil {
			status := s.TsNetManager.GetStatus()
			if status.HomepageServing {
				if host := tsnetStatusHost(status.DNS, status.CertDNS); host != "" {
					return fmt.Sprintf("https://%s:8443", host)
				}
			}
		}
		if requestLooksTailscale(r) {
			if host := requestForwardedHost(r); host != "" {
				return fmt.Sprintf("https://%s:8443", host)
			}
		}
	}
	if tunnelURL := tools.GetTunnelURL(); tunnelURL != "" {
		return tunnelURL
	}
	if cfg.Homepage.WebServerInternalOnly || requestLooksTailscale(r) {
		return ""
	}
	return homepageBrowserURLForRequest(r, cfg.Homepage.WebServerPort)
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

// handleHomepageHistory serves project history entries for the Homepage Studio UI.
// Supports GET (list/search) and DELETE (single entry).
func handleHomepageHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if s.HomepageRegistryDB == nil {
			jsonError(w, "Homepage registry is not enabled or DB not initialized", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			projectID := int64(0)
			if pidStr := r.URL.Query().Get("project_id"); pidStr != "" {
				pid, err := strconv.ParseInt(pidStr, 10, 64)
				if err != nil || pid <= 0 {
					jsonError(w, "invalid project_id", http.StatusBadRequest)
					return
				}
				projectID = pid
			} else {
				projectDir := r.URL.Query().Get("project_dir")
				explicitProjectDir := projectDir != ""
				if projectDir == "" {
					s.CfgMu.RLock()
					projectDir = s.Cfg.Homepage.WorkspacePath
					s.CfgMu.RUnlock()
				}
				if projectDir != "" {
					if proj, projErr := tools.GetProjectByDir(s.HomepageRegistryDB, projectDir); projErr == nil {
						projectID = proj.ID
					}
				}
				if projectID == 0 && !explicitProjectDir {
					if projects, total, listErr := tools.ListProjects(s.HomepageRegistryDB, "", 2, 0); listErr == nil && total == 1 && len(projects) == 1 {
						projectID = projects[0].ID
					}
				}
			}
			// If no project could be resolved, projectID stays 0 and an empty
			// list is returned, which is expected before any project exists.
			if projectID == 0 {
				b, _ := json.Marshal(map[string]interface{}{
					"status":  "success",
					"total":   0,
					"entries": []tools.HomepageHistoryEntry{},
				})
				w.Write(b)
				return
			}

			entryType := r.URL.Query().Get("entry_type")
			query := r.URL.Query().Get("q")
			if query == "" {
				query = r.URL.Query().Get("query")
			}

			limit := 20
			if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
				limit = v
			}
			offset := 0
			if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
				offset = v
			}

			var entries []tools.HomepageHistoryEntry
			var total int
			var dbErr error
			if query != "" {
				entries, total, dbErr = tools.SearchHomepageHistoryEntries(s.HomepageRegistryDB, projectID, query, entryType, nil, limit, offset)
			} else {
				entries, total, dbErr = tools.ListHomepageHistoryEntries(s.HomepageRegistryDB, projectID, entryType, nil, limit, offset)
			}
			if dbErr != nil {
				jsonError(w, dbErr.Error(), http.StatusInternalServerError)
				return
			}

			b, _ := json.Marshal(map[string]interface{}{
				"status":  "success",
				"total":   total,
				"entries": entries,
			})
			w.Write(b)

		case http.MethodDelete:
			id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			if err != nil || id <= 0 {
				jsonError(w, "id is required", http.StatusBadRequest)
				return
			}
			if err := tools.DeleteHomepageHistoryEntry(s.HomepageRegistryDB, id); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message": "History entry deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
