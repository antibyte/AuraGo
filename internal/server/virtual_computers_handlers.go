package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"aurago/internal/virtualcomputers"

	"github.com/gorilla/websocket"
)

var virtualComputersWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return desktopWSUpgrader.CheckOrigin(r)
	},
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
		writeJSON(w, map[string]interface{}{
			"status":        "ok",
			"enabled":       cfg.Enabled,
			"configured":    strings.TrimSpace(cfg.BoringdURL) != "",
			"auto_setup":    cfg.AutoSetup,
			"provider":      cfg.Provider,
			"control_plane": cfg.ControlPlane,
			"tailscale":     virtualComputersTailscaleStatus(s),
		})
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
		virtualComputersRecordSetupState(s, r, "preflight", "pending")
		virtualComputersRecordAction(s, r, "preflight", "setup", "", nil)
		writeJSON(w, map[string]interface{}{
			"status":  "pending",
			"message": "SSH preflight requires the configured credential executor; this build keeps boringd private and validates host requirements before install.",
			"requirements": []string{
				"Ubuntu host",
				"x86_64 architecture",
				"/dev/kvm available",
				"SSH credential stored in AuraGo vault",
			},
		})
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
		virtualComputersRecordSetupState(s, r, "install", "accepted")
		virtualComputersRecordAction(s, r, "install", "setup", "", nil)
		writeJSON(w, map[string]interface{}{
			"status":  "accepted",
			"message": "install/repair orchestration is configured for idempotent SSH execution; boringd remains private behind AuraGo routes",
		})
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
	return virtualcomputers.NewClient(virtualcomputers.ClientConfig{
		BaseURL: cfg.BoringdURL,
		Token:   cfg.BoringToken,
		Timeout: 30 * time.Second,
	})
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
