package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/remote"
	"aurago/internal/virtualcomputers"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var virtualComputersWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return desktopWSUpgrader.CheckOrigin(r)
	},
}

var virtualComputersTunnel = struct {
	sync.Mutex
	key   string
	close func()
}{}

type virtualComputersSetupRequest struct {
	SkipDesktop bool `json:"skip_desktop"`
}

type virtualComputersSSHExecutor struct {
	Host   string
	Port   int
	User   string
	Secret []byte
}

func (e virtualComputersSSHExecutor) Run(ctx context.Context, command string) (string, error) {
	return remote.ExecuteRemoteCommand(ctx, e.Host, e.Port, e.User, e.Secret, command)
}

func (e virtualComputersSSHExecutor) RunScript(ctx context.Context, script string) (string, error) {
	return remote.ExecuteRemoteScript(ctx, e.Host, e.Port, e.User, e.Secret, script)
}

func registerVirtualComputersRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/virtual-computers/setup/status", handleVirtualComputersSetupStatus(s))
	mux.HandleFunc("/api/virtual-computers/setup/preflight", handleVirtualComputersSetupPreflight(s))
	mux.HandleFunc("/api/virtual-computers/setup/install", handleVirtualComputersSetupInstall(s))
	mux.HandleFunc("/api/virtual-computers/setup/repair", handleVirtualComputersSetupInstall(s))
	mux.HandleFunc("/api/virtual-computers/status", handleVirtualComputersStatus(s))
	mux.HandleFunc("/api/virtual-computers/templates", handleVirtualComputersTemplates(s))
	mux.HandleFunc("/api/virtual-computers/volumes", handleVirtualComputersVolumes(s))
	mux.HandleFunc("/api/virtual-computers/machines", handleVirtualComputersMachines(s))
	mux.HandleFunc("/api/virtual-computers/machines/", handleVirtualComputersMachine(s))
}

func handleVirtualComputersSetupStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		payload := map[string]interface{}{
			"status":        "ok",
			"enabled":       cfg.Enabled,
			"configured":    strings.TrimSpace(cfg.BoringdURL) != "",
			"auto_setup":    cfg.AutoSetup,
			"provider":      cfg.Provider,
			"control_plane": cfg.ControlPlane,
			"tailscale":     virtualComputersTailscaleStatus(s),
		}
		for key, value := range virtualComputersSetupMetadata(s, cfg, nil) {
			payload[key] = value
		}
		writeJSON(w, payload)
	}
}

func handleVirtualComputersSetupPreflight(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		if !cfg.AutoSetup {
			jsonError(w, "virtual computer auto-setup is disabled", http.StatusForbidden)
			return
		}
		manager, err := virtualComputersSetupManager(s, cfg, "")
		if err != nil {
			virtualComputersRecordSetupState(s, r, "preflight", "failed")
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		result, err := manager.Preflight(ctx)
		if err != nil {
			virtualComputersRecordSetupState(s, r, "preflight", "failed")
			jsonError(w, manager.RedactInstallLog(err.Error()), http.StatusBadGateway)
			return
		}
		state := "unsupported"
		if result.Supported {
			state = "supported"
		}
		virtualComputersRecordSetupState(s, r, "preflight", state)
		virtualComputersRecordAction(s, r, "preflight", "setup", "", map[string]interface{}{"supported": result.Supported})
		payload := map[string]interface{}{
			"status":   state,
			"result":   result,
			"message":  virtualComputersSetupMessage(cfg),
			"ssh_host": cfg.ControlPlane.Host,
		}
		for key, value := range virtualComputersSetupMetadata(s, cfg, result.Checks) {
			payload[key] = value
		}
		writeJSON(w, payload)
	}
}

func handleVirtualComputersSetupInstall(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		if !cfg.AutoSetup {
			jsonError(w, "virtual computer auto-setup is disabled", http.StatusForbidden)
			return
		}
		var req virtualComputersSetupRequest
		if err := decodeOptionalJSON(r, &req); err != nil {
			jsonError(w, "invalid setup request", http.StatusBadRequest)
			return
		}
		token, generatedToken, err := virtualComputersEnsureBoringToken(s, cfg)
		if err != nil {
			virtualComputersRecordSetupState(s, r, "install", "failed")
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		manager, err := virtualComputersSetupManager(s, cfg, token)
		if err != nil {
			virtualComputersRecordSetupState(s, r, "install", "failed")
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		manager.InstallOptions = virtualComputersSetupOptions(cfg, token, req)
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Minute)
		defer cancel()
		status, err := manager.Install(ctx)
		if err != nil {
			virtualComputersRecordSetupState(s, r, "install", "failed")
			virtualComputersRecordAction(s, r, "install_failed", "setup", "", map[string]interface{}{"message": status.Message})
			jsonError(w, status.Message, http.StatusBadGateway)
			return
		}
		state := "installed"
		if !status.Healthy {
			state = "unhealthy"
		}
		virtualComputersRecordSetupState(s, r, "install", state)
		virtualComputersRecordAction(s, r, "install", "setup", "", map[string]interface{}{"healthy": status.Healthy, "token_generated": generatedToken})
		payload := map[string]interface{}{
			"status":           state,
			"setup":            status,
			"token_configured": true,
			"token_generated":  generatedToken,
			"boringd_url":      cfg.BoringdURL,
			"message":          virtualComputersInstalledMessage(cfg),
		}
		for key, value := range virtualComputersSetupMetadata(s, cfg, status.Preflight.Checks) {
			payload[key] = value
		}
		writeJSON(w, payload)
	}
}

func handleVirtualComputersStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		payload, err := client.Status(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "boringd": payload, "tailscale": virtualComputersTailscaleStatus(s)})
	}
}

func handleVirtualComputersMachines(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope := desktopMethodScope(r.Method)
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			machines, err := client.ListMachines(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersSyncMachines(s, r, machines)
			writeJSON(w, map[string]interface{}{"status": "ok", "machines": machines})
		case http.MethodPost:
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req virtualcomputers.LaunchMachineRequest
			if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			machine, err := client.LaunchMachine(r.Context(), req)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersUpsertMachine(s, r, machine)
			virtualComputersRecordAction(s, r, "launch", "machine", machine.ID, map[string]interface{}{"template": machine.Template})
			writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleVirtualComputersMachine(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, tail := splitVirtualComputerMachinePath(r.URL.Path)
		if machineID == "" {
			jsonError(w, "machine id is required", http.StatusBadRequest)
			return
		}
		if strings.HasPrefix(tail, "web/") {
			handleVirtualComputerPreviewProxy(s, machineID, strings.TrimPrefix(tail, "web/")).ServeHTTP(w, r)
			return
		}
		if virtualComputersIsWSChannel(tail) {
			handleVirtualComputerWSProxy(s, machineID, strings.Trim(tail, "/")).ServeHTTP(w, r)
			return
		}

		scope := desktopMethodScope(r.Method)
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		switch strings.Trim(tail, "/") {
		case "":
			switch r.Method {
			case http.MethodGet:
				machine, err := client.GetMachine(r.Context(), machineID)
				if err != nil {
					jsonError(w, err.Error(), http.StatusBadGateway)
					return
				}
				writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
			case http.MethodDelete:
				if !virtualComputersMutationAllowed(s, w) {
					return
				}
				if err := client.DestroyMachine(r.Context(), machineID); err != nil {
					jsonError(w, err.Error(), http.StatusBadGateway)
					return
				}
				virtualComputersDeleteMachine(s, r, machineID)
				virtualComputersRecordAction(s, r, "destroy", "machine", machineID, nil)
				writeJSON(w, map[string]interface{}{"status": "ok"})
			default:
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case "extend":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req struct {
				TTLSeconds int `json:"ttl_seconds"`
			}
			_ = decodeVirtualComputersJSON(w, r, &req)
			machine, err := client.ExtendMachine(r.Context(), machineID, req.TTLSeconds)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersUpsertMachine(s, r, machine)
			virtualComputersRecordAction(s, r, "extend", "machine", machine.ID, map[string]interface{}{"ttl_seconds": machine.TTLSeconds})
			writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
		case "fork":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req struct {
				TTLSeconds int `json:"ttl_seconds"`
			}
			_ = decodeVirtualComputersJSON(w, r, &req)
			machine, err := client.ForkMachine(r.Context(), machineID, req.TTLSeconds)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersUpsertMachine(s, r, machine)
			virtualComputersRecordAction(s, r, "fork", "machine", machine.ID, map[string]interface{}{"source_machine_id": machineID})
			writeJSON(w, map[string]interface{}{"status": "ok", "machine": machine})
		case "exec":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req virtualcomputers.ExecRequest
			if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			result, err := client.Exec(r.Context(), machineID, req)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersRecordAction(s, r, "exec", "machine", machineID, nil)
			writeJSON(w, map[string]interface{}{"status": "ok", "result": result})
		case "screenshot":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			shot, err := client.Screenshot(r.Context(), machineID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "screenshot": shot})
		case "upload":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req struct {
				Path          string `json:"path"`
				Content       string `json:"content"`
				ContentBase64 string `json:"content_base64"`
			}
			if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			content := []byte(req.Content)
			if req.ContentBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(req.ContentBase64)
				if err != nil {
					jsonError(w, "Invalid base64 content", http.StatusBadRequest)
					return
				}
				content = decoded
			}
			payload, err := client.Upload(r.Context(), machineID, req.Path, content)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersRecordAction(s, r, "upload", "machine", machineID, map[string]interface{}{"path": req.Path})
			writeJSON(w, map[string]interface{}{"status": "ok", "result": payload})
		case "download":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			data, contentType, err := client.Download(r.Context(), machineID, r.URL.Query().Get("path"))
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			_, _ = w.Write(data)
		case "publish":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			cfg := virtualComputersConfigSnapshot(s)
			if !cfg.AllowPublish {
				jsonError(w, "publishing virtual computers is disabled", http.StatusForbidden)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			payload, err := client.Publish(r.Context(), machineID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersRecordAction(s, r, "publish", "machine", machineID, nil)
			writeJSON(w, map[string]interface{}{"status": "ok", "result": payload})
		case "save":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !virtualComputersMutationAllowed(s, w) {
				return
			}
			var req struct {
				Name string `json:"name"`
			}
			_ = decodeVirtualComputersJSON(w, r, &req)
			payload, err := client.SaveMachine(r.Context(), machineID, req.Name)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersRecordAction(s, r, "save", "machine", machineID, nil)
			writeJSON(w, map[string]interface{}{"status": "ok", "result": payload})
		default:
			jsonError(w, "Not found", http.StatusNotFound)
		}
	}
}

func handleVirtualComputersTemplates(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		templates, err := client.ListTemplates(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		virtualComputersSyncTemplates(s, r, templates)
		writeJSON(w, map[string]interface{}{"status": "ok", "templates": templates})
	}
}

func handleVirtualComputersVolumes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			volumes, err := client.ListVolumes(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersSyncVolumes(s, r, volumes)
			writeJSON(w, map[string]interface{}{"status": "ok", "volumes": volumes})
		case http.MethodPost:
			if cfg.ReadOnly {
				jsonError(w, "virtual computers are read-only", http.StatusForbidden)
				return
			}
			if !cfg.AllowVolumes {
				jsonError(w, "virtual computer volumes are disabled", http.StatusForbidden)
				return
			}
			var req struct {
				Name      string `json:"name"`
				SizeBytes int64  `json:"size_bytes"`
			}
			if err := decodeVirtualComputersJSON(w, r, &req); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			volume, err := client.CreateVolume(r.Context(), req.Name, req.SizeBytes)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			virtualComputersUpsertVolume(s, r, volume)
			virtualComputersRecordAction(s, r, "create_volume", "volume", volume.ID, nil)
			writeJSON(w, map[string]interface{}{"status": "ok", "volume": volume})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleVirtualComputerPreviewProxy(s *Server, machineID, tail string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		portPart, suffix, ok := strings.Cut(tail, "/")
		if !ok {
			suffix = ""
		}
		port, err := strconv.Atoi(portPart)
		if err != nil {
			jsonError(w, "invalid preview port", http.StatusBadRequest)
			return
		}
		target, err := client.PreviewTargetURL(machineID, port, suffix)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		virtualComputersSetExposure(s, r, virtualcomputers.ExposureRecord{
			MachineID: machineID,
			Channel:   "web:" + strconv.Itoa(port),
			URL:       r.URL.Path,
			Active:    true,
		})
		cfg := virtualComputersConfigSnapshot(s)
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = target.Path
				req.URL.RawQuery = target.RawQuery
				req.Host = target.Host
				if strings.TrimSpace(cfg.BoringToken) != "" {
					req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.BoringToken))
				}
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				jsonError(w, err.Error(), http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	}
}

func handleVirtualComputerWSProxy(s *Server, machineID, channel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope := desktopScopeRead
		if channel == "agent" || channel == "shell-agent" {
			scope = desktopScopeWrite
		}
		if !requireDesktopPermission(s, w, r, scope) {
			return
		}
		client, err := virtualComputersClient(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		upstreamURL, header, err := client.WebSocketURL(machineID, channel)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		virtualComputersSetExposure(s, r, virtualcomputers.ExposureRecord{
			MachineID: machineID,
			Channel:   channel,
			URL:       r.URL.Path,
			Active:    true,
		})
		upstream, _, err := websocket.DefaultDialer.DialContext(r.Context(), upstreamURL, header)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer upstream.Close()

		downstream, err := virtualComputersWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer downstream.Close()

		errCh := make(chan error, 2)
		go copyVirtualComputerWS(upstream, downstream, errCh)
		go copyVirtualComputerWS(downstream, upstream, errCh)
		<-errCh
	}
}

func copyVirtualComputerWS(dst, src *websocket.Conn, errCh chan<- error) {
	for {
		messageType, data, err := src.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if err := dst.WriteMessage(messageType, data); err != nil {
			errCh <- err
			return
		}
	}
}

func virtualComputersClient(s *Server) (*virtualcomputers.Client, error) {
	cfg := virtualComputersConfigSnapshot(s)
	if !cfg.Enabled {
		return nil, fmt.Errorf("virtual computers are disabled")
	}
	if strings.TrimSpace(cfg.BoringdURL) == "" {
		return nil, fmt.Errorf("boringd URL is not configured")
	}
	if err := virtualComputersEnsureControlPlaneAccess(s, cfg); err != nil {
		return nil, err
	}
	return virtualcomputers.NewClient(virtualcomputers.ClientConfig{
		BaseURL: cfg.BoringdURL,
		Token:   cfg.BoringToken,
		Timeout: 30 * time.Second,
	})
}

func virtualComputersEnsureControlPlaneAccess(s *Server, cfg virtualcomputers.ToolConfig) error {
	mode := virtualComputersControlPlaneMode(cfg)
	if mode == virtualcomputers.ControlPlaneLocalHost {
		if strings.TrimSpace(cfg.BoringdURL) == "" {
			return nil
		}
		if !virtualComputersHealthOK(cfg.BoringdURL) {
			return fmt.Errorf("local boringd is not reachable at %s; run Virtual Computers setup install or repair", cfg.BoringdURL)
		}
		return nil
	}
	if mode != virtualcomputers.ControlPlaneSSHHost {
		return nil
	}
	if strings.TrimSpace(cfg.ControlPlane.Host) == "" && strings.TrimSpace(cfg.ControlPlane.CredentialID) == "" {
		return nil
	}
	localAddr, ok := virtualComputersLoopbackListenAddr(cfg.BoringdURL)
	if !ok {
		return nil
	}
	if virtualComputersHealthOK(cfg.BoringdURL) {
		return nil
	}
	executor, err := virtualComputersSSHSetupExecutor(s, cfg)
	if err != nil {
		return fmt.Errorf("virtual computer SSH tunnel setup failed: %w", err)
	}
	key := fmt.Sprintf("%s:%d>%s", executor.Host, executor.Port, localAddr)

	virtualComputersTunnel.Lock()
	defer virtualComputersTunnel.Unlock()
	if virtualComputersTunnel.key == key && virtualComputersTunnel.close != nil {
		if virtualComputersHealthOK(cfg.BoringdURL) {
			return nil
		}
		virtualComputersTunnel.close()
		virtualComputersTunnel.key = ""
		virtualComputersTunnel.close = nil
	}
	if virtualComputersTunnel.close != nil {
		virtualComputersTunnel.close()
		virtualComputersTunnel.key = ""
		virtualComputersTunnel.close = nil
	}
	closeFn, err := startVirtualComputersSSHTunnel(executor, localAddr, "127.0.0.1:8080", s)
	if err != nil {
		return fmt.Errorf("start virtual computer SSH tunnel: %w", err)
	}
	virtualComputersTunnel.key = key
	virtualComputersTunnel.close = closeFn
	return nil
}

func virtualComputersLoopbackListenAddr(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "localhost", "127.0.0.1":
		host = "127.0.0.1"
	case "::1":
	default:
		return "", false
	}
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", false
		}
	}
	return net.JoinHostPort(host, port), true
}

func virtualComputersHealthOK(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/healthz")
	if err != nil {
		return false
	}
	client := http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func startVirtualComputersSSHTunnel(executor virtualComputersSSHExecutor, localAddr, remoteAddr string, s *Server) (func(), error) {
	sshCfg, err := remote.GetSSHConfig(executor.User, executor.Secret)
	if err != nil {
		return nil, err
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", executor.Host, executor.Port), sshCfg)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		client.Close()
		return nil, err
	}
	var once sync.Once
	closeFn := func() {
		once.Do(func() {
			_ = listener.Close()
			_ = client.Close()
		})
	}
	go serveVirtualComputersSSHTunnel(listener, client, remoteAddr, s)
	return closeFn, nil
}

func serveVirtualComputersSSHTunnel(listener net.Listener, client *ssh.Client, remoteAddr string, s *Server) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			return
		}
		go proxyVirtualComputersTunnelConn(localConn, client, remoteAddr, s)
	}
}

func proxyVirtualComputersTunnelConn(localConn net.Conn, client *ssh.Client, remoteAddr string, s *Server) {
	remoteConn, err := client.Dial("tcp", remoteAddr)
	if err != nil {
		if s != nil && s.Logger != nil {
			s.Logger.Warn("[VirtualComputers] SSH tunnel dial failed", "error", err)
		}
		_ = localConn.Close()
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remoteConn, localConn)
		_ = remoteConn.Close()
		_ = localConn.Close()
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(localConn, remoteConn)
		_ = remoteConn.Close()
		_ = localConn.Close()
		done <- struct{}{}
	}()
	<-done
}

func virtualComputersConfigSnapshot(s *Server) virtualcomputers.ToolConfig {
	if s == nil || s.Cfg == nil {
		return virtualcomputers.ToolConfig{}
	}
	s.CfgMu.RLock()
	cfgCopy := *s.Cfg
	s.CfgMu.RUnlock()
	return virtualcomputers.FromAuraConfig(&cfgCopy)
}

func virtualComputersMutationAllowed(s *Server, w http.ResponseWriter) bool {
	cfg := virtualComputersConfigSnapshot(s)
	if cfg.ReadOnly {
		jsonError(w, "virtual computers are read-only", http.StatusForbidden)
		return false
	}
	return true
}

func virtualComputersTailscaleStatus(s *Server) map[string]interface{} {
	out := map[string]interface{}{
		"enabled":    false,
		"serve_http": false,
		"path":       "/virtual-computers",
	}
	if s == nil || s.Cfg == nil {
		return out
	}
	s.CfgMu.RLock()
	tsnetCfg := s.Cfg.Tailscale.TsNet
	out["enabled"] = tsnetCfg.Enabled
	out["serve_http"] = tsnetCfg.ServeHTTP
	out["host"] = tsnetCfg.Hostname
	if tsnetCfg.Enabled && tsnetCfg.ServeHTTP && strings.TrimSpace(tsnetCfg.Hostname) != "" {
		out["url"] = "https://" + strings.TrimSpace(tsnetCfg.Hostname) + "/virtual-computers"
	}
	s.CfgMu.RUnlock()
	return out
}

func virtualComputersControlPlaneMode(cfg virtualcomputers.ToolConfig) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.ControlPlane.Mode))
	if mode == "" {
		return virtualcomputers.ControlPlaneSSHHost
	}
	return mode
}

func virtualComputersSetupMessage(cfg virtualcomputers.ToolConfig) string {
	if virtualComputersControlPlaneMode(cfg) == virtualcomputers.ControlPlaneLocalHost {
		return "Local boring-computers setup requires Ubuntu/Linux with /dev/kvm, systemd, and root or passwordless sudo."
	}
	return "boring-computers setup requires Ubuntu with /dev/kvm and x86_64/amd64 or arm64/aarch64."
}

func virtualComputersInstalledMessage(cfg virtualcomputers.ToolConfig) string {
	if virtualComputersControlPlaneMode(cfg) == virtualcomputers.ControlPlaneLocalHost {
		return "boringd is installed locally and bound to 127.0.0.1:8080; AuraGo keeps the token server-side."
	}
	return "boringd is installed bound to 127.0.0.1:8080 on the control-plane host; AuraGo keeps the token server-side."
}

func virtualComputersSetupMetadata(s *Server, cfg virtualcomputers.ToolConfig, checks map[string]string) map[string]interface{} {
	mode := virtualComputersControlPlaneMode(cfg)
	if len(checks) == 0 && mode == virtualcomputers.ControlPlaneLocalHost {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := (virtualcomputers.LocalCommandExecutor{}).Preflight(ctx); err == nil {
			checks = virtualcomputers.ParsePreflightOutput(out).Checks
		}
	}
	hostOS := strings.ToLower(strings.TrimSpace(checks["HOST_OS"]))
	arch := strings.TrimSpace(checks["ARCH"])
	if mode == virtualcomputers.ControlPlaneLocalHost {
		if hostOS == "" {
			hostOS = runtime.GOOS
		}
		if arch == "" {
			arch = runtime.GOARCH
		}
	}
	return map[string]interface{}{
		"mode":              mode,
		"host_os":           hostOS,
		"arch":              arch,
		"running_in_docker": virtualComputersCheckBool(s, checks, "RUNNING_IN_DOCKER"),
		"has_kvm":           checks["HAS_KVM"] == "1",
		"has_systemd":       checks["HAS_SYSTEMD"] == "1",
		"has_sudo_or_root":  checks["HAS_SUDO_OR_ROOT"] == "1",
	}
}

func virtualComputersCheckBool(s *Server, checks map[string]string, key string) bool {
	if checks[key] == "1" {
		return true
	}
	if key == "RUNNING_IN_DOCKER" && checks[key] == "" && s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		defer s.CfgMu.RUnlock()
		return s.Cfg.Runtime.IsDocker
	}
	return false
}

func virtualComputersIsWSChannel(tail string) bool {
	switch strings.Trim(tail, "/") {
	case "tty", "vnc", "agent", "shell-agent":
		return true
	default:
		return false
	}
}

func splitVirtualComputerMachinePath(path string) (string, string) {
	rest := strings.TrimPrefix(path, "/api/virtual-computers/machines/")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return "", ""
	}
	machineID, tail, _ := strings.Cut(rest, "/")
	return machineID, tail
}

func virtualComputersLedger(s *Server) (*virtualcomputers.Ledger, error) {
	if s == nil || s.Cfg == nil {
		return nil, nil
	}
	s.CfgMu.RLock()
	path := s.Cfg.SQLite.VirtualComputersPath
	s.CfgMu.RUnlock()
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	return virtualcomputers.OpenLedger(path)
}

func virtualComputersWithLedger(s *Server, r *http.Request, fn func(*virtualcomputers.Ledger) error) {
	if fn == nil {
		return
	}
	ledger, err := virtualComputersLedger(s)
	if err != nil {
		virtualComputersLogLedgerError(s, err)
		return
	}
	if ledger == nil {
		return
	}
	defer ledger.Close()
	if err := fn(ledger); err != nil {
		virtualComputersLogLedgerError(s, err)
	}
}

func virtualComputersLogLedgerError(s *Server, err error) {
	if err == nil || s == nil || s.Logger == nil {
		return
	}
	s.Logger.Warn("[VirtualComputers] ledger update failed", "error", err)
}

func decodeOptionalJSON(r *http.Request, dst interface{}) error {
	if r == nil || r.Body == nil || dst == nil {
		return nil
	}
	if r.ContentLength == 0 {
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(dst)
	if err == nil || err == io.EOF {
		return nil
	}
	return err
}

func virtualComputersSetupManager(s *Server, cfg virtualcomputers.ToolConfig, token string) (virtualcomputers.SetupManager, error) {
	executor, err := virtualComputersSetupExecutor(s, cfg)
	if err != nil {
		return virtualcomputers.SetupManager{}, err
	}
	return virtualcomputers.SetupManager{
		Executor:       executor,
		Token:          token,
		InstallOptions: virtualComputersSetupOptions(cfg, token, virtualComputersSetupRequest{}),
	}, nil
}

func virtualComputersSetupOptions(cfg virtualcomputers.ToolConfig, token string, req virtualComputersSetupRequest) virtualcomputers.SetupInstallOptions {
	return virtualcomputers.SetupInstallOptions{
		InstallDir:         cfg.ControlPlane.InstallDir,
		Token:              token,
		AnthropicKey:       cfg.BoringAnthropicKey,
		OpenRouterKey:      cfg.BoringOpenRouterKey,
		S3AccessKeyID:      cfg.S3AccessKeyID,
		S3SecretKey:        cfg.S3SecretKey,
		MaxRunningMachines: cfg.MaxRunningMachines,
		MaxForks:           cfg.MaxForks,
		AllowInternet:      cfg.AllowInternet,
		AllowPersistent:    cfg.AllowPersistent,
		AllowPublish:       cfg.AllowPublish,
		AllowVolumes:       cfg.AllowVolumes,
		SkipDesktop:        req.SkipDesktop,
	}
}

func virtualComputersSetupExecutor(s *Server, cfg virtualcomputers.ToolConfig) (virtualcomputers.CommandExecutor, error) {
	switch virtualComputersControlPlaneMode(cfg) {
	case virtualcomputers.ControlPlaneLocalHost:
		return virtualcomputers.LocalCommandExecutor{}, nil
	case virtualcomputers.ControlPlaneSSHHost:
		return virtualComputersSSHSetupExecutor(s, cfg)
	default:
		return nil, fmt.Errorf("unsupported virtual computer control-plane mode %q", cfg.ControlPlane.Mode)
	}
}

func virtualComputersSSHSetupExecutor(s *Server, cfg virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
	cp := cfg.ControlPlane
	port := cp.SSHPort
	if port <= 0 {
		port = 22
	}
	if strings.TrimSpace(cp.CredentialID) != "" {
		return virtualComputersSetupExecutorFromCredential(s, cp.CredentialID, cp.Host, port)
	}

	user, host, parsedPort := parseVirtualComputersSSHTarget(cp.Host, port)
	if host == "" {
		return virtualComputersSSHExecutor{}, fmt.Errorf("virtual computer control-plane host is required")
	}
	if user == "" {
		user = "root"
	}
	secret := strings.TrimSpace(safeConfigSSHSecret(s))
	if secret == "" && s != nil && s.Vault != nil {
		secret, _ = s.Vault.ReadSecret("virtual_computers_ssh_secret")
	}
	if strings.TrimSpace(secret) == "" {
		return virtualComputersSSHExecutor{}, fmt.Errorf("virtual computer SSH secret is missing; store virtual_computers_ssh_secret in the vault or select an SSH credential")
	}
	return virtualComputersSSHExecutor{Host: host, Port: parsedPort, User: user, Secret: []byte(secret)}, nil
}

func virtualComputersSetupExecutorFromCredential(s *Server, credentialID, fallbackHost string, fallbackPort int) (virtualComputersSSHExecutor, error) {
	if s == nil || s.InventoryDB == nil {
		return virtualComputersSSHExecutor{}, fmt.Errorf("inventory database is not available for SSH credential lookup")
	}
	if s.Vault == nil {
		return virtualComputersSSHExecutor{}, fmt.Errorf("vault is not available for SSH credential lookup")
	}
	cred, err := credentials.GetByID(s.InventoryDB, strings.TrimSpace(credentialID))
	if err != nil {
		return virtualComputersSSHExecutor{}, fmt.Errorf("load SSH credential: %w", err)
	}
	if cred.Type != "" && !strings.EqualFold(cred.Type, "ssh") {
		return virtualComputersSSHExecutor{}, fmt.Errorf("credential %q is type %q, not ssh", cred.Name, cred.Type)
	}
	user := strings.TrimSpace(cred.Username)
	host := strings.TrimSpace(cred.Host)
	if host == "" {
		host = strings.TrimSpace(fallbackHost)
	}
	parsedUser, parsedHost, parsedPort := parseVirtualComputersSSHTarget(host, fallbackPort)
	if parsedUser != "" {
		user = parsedUser
	}
	if parsedHost != "" {
		host = parsedHost
	}
	if user == "" {
		user = "root"
	}
	secretID := strings.TrimSpace(cred.CertificateVaultID)
	if secretID == "" {
		secretID = strings.TrimSpace(cred.PasswordVaultID)
	}
	if secretID == "" {
		return virtualComputersSSHExecutor{}, fmt.Errorf("credential %q has no SSH password or private key stored in the vault", cred.Name)
	}
	secret, err := s.Vault.ReadSecret(secretID)
	if err != nil {
		return virtualComputersSSHExecutor{}, fmt.Errorf("read SSH credential secret: %w", err)
	}
	if strings.TrimSpace(host) == "" {
		return virtualComputersSSHExecutor{}, fmt.Errorf("virtual computer SSH credential has no host")
	}
	return virtualComputersSSHExecutor{Host: host, Port: parsedPort, User: user, Secret: []byte(secret)}, nil
}

func parseVirtualComputersSSHTarget(target string, defaultPort int) (user, host string, port int) {
	port = defaultPort
	if port <= 0 {
		port = 22
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", port
	}
	if before, after, ok := strings.Cut(target, "@"); ok {
		user = strings.TrimSpace(before)
		target = strings.TrimSpace(after)
	}
	if h, p, err := net.SplitHostPort(target); err == nil {
		host = strings.Trim(h, "[]")
		if parsed, parseErr := strconv.Atoi(p); parseErr == nil && parsed > 0 {
			port = parsed
		}
		return user, host, port
	}
	if strings.Count(target, ":") == 1 {
		if h, p, ok := strings.Cut(target, ":"); ok {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				return user, strings.TrimSpace(h), parsed
			}
		}
	}
	return user, strings.Trim(target, "[]"), port
}

func safeConfigSSHSecret(s *Server) string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.VirtualComputers.ControlPlane.SSHSecret
}

func virtualComputersEnsureBoringToken(s *Server, cfg virtualcomputers.ToolConfig) (string, bool, error) {
	if token := strings.TrimSpace(cfg.BoringToken); token != "" {
		return token, false, nil
	}
	if s == nil || s.Vault == nil {
		return "", false, fmt.Errorf("vault is required to create the boringd token")
	}
	raw, err := GenerateRandomHex(32)
	if err != nil {
		return "", false, fmt.Errorf("generate boringd token: %w", err)
	}
	token := "boring_" + raw
	if err := s.Vault.WriteSecret("virtual_computers_boring_token", token); err != nil {
		return "", false, fmt.Errorf("store boringd token in vault: %w", err)
	}
	s.CfgMu.Lock()
	if s.Cfg != nil {
		newCfg := *s.Cfg
		newCfg.VirtualComputers.BoringToken = token
		s.replaceConfigSnapshot(&newCfg)
	}
	s.CfgMu.Unlock()
	return token, true, nil
}

func virtualComputersActor(r *http.Request) string {
	if r == nil {
		return "server"
	}
	if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
		return "desktop_token"
	}
	return "ui"
}

func virtualComputersRecordSetupState(s *Server, r *http.Request, key, value string) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.SetSetupState(r.Context(), key, value)
	})
}

func virtualComputersRecordAction(s *Server, r *http.Request, action, targetType, targetID string, metadata map[string]interface{}) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.RecordAction(r.Context(), virtualcomputers.ActionRecord{
			Actor:      virtualComputersActor(r),
			Action:     action,
			TargetType: targetType,
			TargetID:   targetID,
			Metadata:   metadata,
		})
	})
}

func virtualComputersUpsertMachine(s *Server, r *http.Request, machine virtualcomputers.Machine) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.UpsertMachine(r.Context(), machine)
	})
}

func virtualComputersDeleteMachine(s *Server, r *http.Request, machineID string) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.DeleteMachine(r.Context(), machineID)
	})
}

func virtualComputersSyncMachines(s *Server, r *http.Request, machines []virtualcomputers.Machine) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		for _, machine := range machines {
			if err := ledger.UpsertMachine(r.Context(), machine); err != nil {
				return err
			}
		}
		return nil
	})
}

func virtualComputersSyncTemplates(s *Server, r *http.Request, templates []virtualcomputers.Template) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		for _, template := range templates {
			if err := ledger.UpsertTemplate(r.Context(), template); err != nil {
				return err
			}
		}
		return nil
	})
}

func virtualComputersUpsertVolume(s *Server, r *http.Request, volume virtualcomputers.Volume) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.UpsertVolume(r.Context(), volume)
	})
}

func virtualComputersSyncVolumes(s *Server, r *http.Request, volumes []virtualcomputers.Volume) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		for _, volume := range volumes {
			if err := ledger.UpsertVolume(r.Context(), volume); err != nil {
				return err
			}
		}
		return nil
	})
}

func virtualComputersSetExposure(s *Server, r *http.Request, exposure virtualcomputers.ExposureRecord) {
	virtualComputersWithLedger(s, r, func(ledger *virtualcomputers.Ledger) error {
		return ledger.SetExposure(r.Context(), exposure)
	})
}

func decodeVirtualComputersJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, desktopMediumJSONBodyLimit)
	return json.NewDecoder(r.Body).Decode(dst)
}
