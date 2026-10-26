package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
	"aurago/internal/security"
	"aurago/internal/tools"
)

var printGuardEngineRequest = tools.DockerRequestContext
var printGuardProbeTools = tools.ProbeMCPServerTools
var printGuardGatewayProbe = checkPrintGuardGateway
var printGuardGatewayClient = &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// Check the configured preview site without sending a session or token to it.
func checkPrintGuardGateway(ctx context.Context, origin string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+previewLaunchPath+strings.Repeat("0", 64), nil)
	if err != nil {
		return printGuardError("gateway")
	}
	resp, err := printGuardGatewayClient.Do(req)
	if err != nil {
		return printGuardError("gateway")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil || resp.StatusCode != http.StatusUnauthorized || string(body) != "Preview launch expired\n" {
		return printGuardError("gateway")
	}
	return nil
}

var printGuardReadTools = []string{"get_state", "list_printers", "get_printer", "list_monitors", "get_monitor", "list_cameras", "get_camera", "get_camera_frame", "recent_events"}

type printGuardHostStatus struct {
	Networks      []string `json:"networks"`
	Architecture  string   `json:"architecture"`
	Containerized bool     `json:"containerized"`
	NVIDIA        bool     `json:"nvidia_available"`
	Origins       []string `json:"-"`
}

func printGuardError(code string) error { return errors.New("printguard_" + code) }

func printGuardEngineIsLocal(host string, containerized bool) bool {
	if host == "" {
		return true
	}
	if !strings.Contains(host, "://") {
		host = "tcp://" + host
	}
	u, err := url.Parse(host)
	if err != nil || u.User != nil {
		return false
	}
	if u.Scheme == "npipe" || u.Scheme == "unix" {
		return u.Host == "" || u.Host == "."
	}
	if u.Scheme != "tcp" {
		return false
	}
	// The bundled Compose proxy is local to AuraGo's Docker control network.
	if containerized && u.Hostname() == "docker-proxy" && u.Port() == "2375" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
}

func (s *Server) printGuardHost(ctx context.Context) (printGuardHostStatus, error) {
	s.CfgMu.RLock()
	dockerCfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
	containerized := s.Cfg.Runtime.IsDocker
	s.CfgMu.RUnlock()
	result := printGuardHostStatus{Networks: []string{}, Containerized: containerized}
	if !printGuardEngineIsLocal(dockerCfg.Host, containerized) {
		return result, printGuardError("local_engine")
	}
	data, status, err := printGuardEngineRequest(ctx, dockerCfg, http.MethodGet, "/info", "")
	var info struct {
		Architecture string
		OSType       string
		Runtimes     map[string]json.RawMessage
	}
	if err != nil || status != http.StatusOK || json.Unmarshal(data, &info) != nil {
		return result, printGuardError("docker_unavailable")
	}
	result.Architecture = info.Architecture
	if result.Architecture == "x86_64" {
		result.Architecture = "amd64"
	}
	if result.Architecture == "aarch64" {
		result.Architecture = "arm64"
	}
	_, result.NVIDIA = info.Runtimes["nvidia"]
	if info.OSType != "linux" || !slices.Contains([]string{"amd64", "arm64"}, result.Architecture) {
		return result, printGuardError("architecture")
	}
	if containerized {
		name, err := os.Hostname()
		if err != nil {
			return result, printGuardError("network")
		}
		data, status, err = printGuardEngineRequest(ctx, dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", "")
		var own struct {
			NetworkSettings struct{ Networks map[string]json.RawMessage }
		}
		if err != nil || status != http.StatusOK || json.Unmarshal(data, &own) != nil {
			return result, printGuardError("network")
		}
		for network := range own.NetworkSettings.Networks {
			if network != "" && network != "bridge" && network != "host" && network != "none" {
				data, status, err := printGuardEngineRequest(ctx, dockerCfg, http.MethodGet, "/networks/"+url.PathEscape(network), "")
				var details struct{ Internal *bool }
				if err != nil || status != http.StatusOK || json.Unmarshal(data, &details) != nil || details.Internal == nil {
					return result, printGuardError("network")
				}
				// Internal control networks expose privileged sidecars and lack camera egress.
				if *details.Internal {
					continue
				}
				result.Networks = append(result.Networks, network)
			}
		}
		sort.Strings(result.Networks)
		if len(result.Networks) == 0 {
			return result, printGuardError("network")
		}
	}
	return result, nil
}

func (s *Server) printGuardOrigins(r *http.Request) ([]string, error) {
	domain, err := s.configuredPreviewDomain()
	if err != nil || !previewRequestSecure(r) {
		return nil, printGuardError("gateway")
	}
	s.CfgMu.RLock()
	primary := s.Cfg.Server.HTTPS.Domain
	if primary == "" {
		primary = s.Cfg.Server.Host
	}
	authEnabled := s.Cfg.Auth.Enabled
	s.CfgMu.RUnlock()
	if !authEnabled {
		return nil, printGuardError("auth")
	}
	authority := previewRequestAuthority(r)
	u, err := url.Parse("https://" + authority)
	if err != nil || !strings.EqualFold(u.Hostname(), primary) {
		return nil, printGuardError("gateway")
	}
	appAuthority := previewResourceHost(domain, previewResource{kind: "store", id: desktopstore.PrintGuardAppID, port: "web"})
	if u.Port() != "" {
		appAuthority = net.JoinHostPort(appAuthority, u.Port())
	}
	return []string{"https://" + authority, "https://" + appAuthority}, nil
}

func (s *Server) preparePrintGuardDeployment(r *http.Request, profile string, network *string) (printGuardHostStatus, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	info, err := s.printGuardHost(ctx)
	if err != nil {
		return info, err
	}
	info.Origins, err = s.printGuardOrigins(r)
	if err != nil {
		return info, err
	}
	if err := printGuardGatewayProbe(ctx, info.Origins[1]); err != nil {
		return info, err
	}
	if profile != "cpu" && info.Architecture != "amd64" {
		return info, printGuardError("gpu_architecture")
	}
	if profile == "nvidia" && !info.NVIDIA {
		return info, printGuardError("nvidia")
	}
	if info.Containerized {
		if *network == "" && len(info.Networks) == 1 {
			*network = info.Networks[0]
		}
		if !slices.Contains(info.Networks, *network) {
			return info, printGuardError("network")
		}
	} else if *network != "" {
		return info, printGuardError("network")
	}
	return info, nil
}

func handleDesktopStorePrintGuardConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setDesktopStoreNoCacheHeaders(w)
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method == http.MethodPut && rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, "Store unavailable", http.StatusServiceUnavailable)
			return
		}
		status, err := store.PrintGuardConfiguration(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodGet {
			network := status.DockerNetwork
			info, setupErr := s.preparePrintGuardDeployment(r, status.Acceleration, &network)
			issues := []string{}
			if setupErr != nil {
				issues = append(issues, setupErr.Error())
			}
			if status.AgentMode != "off" {
				s.CfgMu.RLock()
				cfg := *s.Cfg
				s.CfgMu.RUnlock()
				if _, _, err := printGuardBindingConfig(&cfg, s.Vault, true); err != nil {
					status.AgentVerified = false
					code := "printguard_not_verified"
					if strings.HasPrefix(err.Error(), "printguard_") {
						code = err.Error()
					}
					issues = append(issues, code)
				}
			}
			compute := ""
			if status.Installed {
				if token, err := store.PrintGuardAPIToken(); err == nil {
					if code, state, err := s.printGuardState(r.Context(), token); err == nil && code == http.StatusOK {
						var payload struct {
							Stats struct {
								InferenceDevice string `json:"inference_device"`
							} `json:"stats"`
						}
						if json.Unmarshal(state, &payload) == nil {
							compute = security.Scrub(payload.Stats.InferenceDevice)
							if len(compute) > 256 {
								compute = ""
							}
						}
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(struct {
				desktopstore.PrintGuardConfigStatus
				printGuardHostStatus
				Prerequisites []string `json:"prerequisites"`
				Compute       string   `json:"compute"`
				CPUFallback   bool     `json:"cpu_fallback"`
			}{status, info, issues, compute, status.ActiveAcceleration != "cpu" && strings.Contains(strings.ToLower(compute), "cpu")})
			return
		}
		var req desktopstore.PrintGuardConfigUpdate
		if err := decodeDesktopJSON(w, r, &req, 16<<10); err != nil {
			jsonError(w, "Invalid configuration JSON", http.StatusBadRequest)
			return
		}
		profile, network := status.Acceleration, status.DockerNetwork
		if req.Acceleration != nil {
			profile = *req.Acceleration
		}
		if req.DockerNetwork != nil {
			network = *req.DockerNetwork
		}
		revoking := (req.ClearAPIToken || req.AgentMode != nil && *req.AgentMode == "off") && profile == status.Acceleration && network == status.DockerNetwork
		if !revoking {
			info, err := s.preparePrintGuardDeployment(r, profile, &network)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			req.Origins = info.Origins
		}
		req.DockerNetwork = &network
		op, err := store.ConfigurePrintGuard(r.Context(), req)
		if err != nil {
			writeDesktopStoreStartError(w, err)
			return
		}
		s.runDesktopStoreOperation(store, op)
		writeDesktopStoreOperationAccepted(w, op)
	}
}

func printGuardBindingConfig(cfg *config.Config, vault config.SecretReader, verifiedOnly bool) (*tools.MCPServerConfig, desktopstore.PrintGuardAgentBinding, error) {
	var binding desktopstore.PrintGuardAgentBinding
	if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
		return nil, binding, printGuardError("mcp_disabled")
	}
	for _, server := range cfg.MCP.Servers {
		if strings.EqualFold(server.Name, desktopstore.PrintGuardMCPName) {
			return nil, binding, printGuardError("mcp_collision")
		}
	}
	dbPath := filepath.Join(desktop.ConfigFromAuraConfig(cfg).DataDir, "desktop_store.db")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var err error
	binding, err = desktopstore.ReadPrintGuardAgentBinding(ctx, dbPath, vault)
	if err != nil {
		return nil, binding, err
	}
	if binding.Mode == "control" && cfg.VirtualDesktop.ReadOnly {
		return nil, binding, printGuardError("readonly")
	}
	if verifiedOnly && !binding.Verified {
		return nil, binding, printGuardError("not_verified")
	}
	if !printGuardEngineIsLocal(cfg.Docker.Host, cfg.Runtime.IsDocker) {
		return nil, binding, printGuardError("local_engine")
	}
	host, port := "127.0.0.1", binding.HostPort
	if cfg.Runtime.IsDocker {
		if binding.DockerNetwork == "" {
			return nil, binding, printGuardError("network")
		}
		host, port = binding.ContainerName, 8000
	}
	allowed := append([]string{}, printGuardReadTools...)
	if binding.Mode == "control" {
		allowed = append(allowed, "control_printer")
	}
	permissions := tools.RuntimePermissionsFromConfig(cfg)
	runtime := &tools.MCPServerConfig{Name: desktopstore.PrintGuardMCPName, Transport: "streamable_http", URL: "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/mcp/", Headers: map[string]string{"Authorization": "Bearer " + binding.Token}, Enabled: true, AllowPrivateNetwork: true, AllowedTools: allowed, ManagedRevision: binding.Signature, ExecutionPermissions: &permissions, MCPEnabled: true}
	security.RegisterSensitive(binding.Token)
	if verifiedOnly {
		runtime.CheckAccess = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			current, err := desktopstore.ReadPrintGuardAgentBinding(ctx, dbPath, vault)
			if err != nil || !current.Verified || current.Signature != binding.Signature {
				return printGuardError("not_verified")
			}
			return nil
		}
	}
	return runtime, binding, nil
}

func (s *Server) printGuardState(ctx context.Context, token string) (int, []byte, error) {
	target, err := s.previewTarget(ctx, previewResource{kind: "store", id: desktopstore.PrintGuardAppID, port: "web"})
	if err != nil {
		return 0, nil, printGuardError("unavailable")
	}
	target.Path = "/api/v1/state"
	target.RawQuery = ""
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		security.RegisterSensitive(token)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, printGuardError("unavailable")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return resp.StatusCode, nil, printGuardError("unavailable")
	}
	return resp.StatusCode, body, nil
}

func (s *Server) verifyPrintGuardConnection(ctx context.Context, store *desktopstore.Service) error {
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	runtime, binding, err := printGuardBindingConfig(&cfg, s.Vault, false)
	if err != nil {
		return err
	}
	if code, _, err := s.printGuardState(ctx, ""); err != nil || code != http.StatusUnauthorized {
		return printGuardError("anonymous")
	}
	if code, _, err := s.printGuardState(ctx, binding.Token); err != nil || code != http.StatusOK {
		return printGuardError("token")
	}
	names, err := printGuardProbeTools(ctx, *runtime, s.Logger)
	if err != nil {
		return printGuardError("mcp_unavailable")
	}
	for _, required := range runtime.AllowedTools {
		if !slices.Contains(names, required) {
			return printGuardError("scope")
		}
	}
	return store.VerifyPrintGuardAgent(ctx, binding)
}

func handleDesktopStorePrintGuardTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setDesktopStoreNoCacheHeaders(w)
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) || rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err == nil {
			_ = store.RevokePrintGuardVerification()
			_ = tools.SetManagedMCPServer(desktopstore.PrintGuardMCPName, nil)
			err = s.verifyPrintGuardConnection(r.Context(), store)
		}
		if err != nil {
			jsonError(w, security.Scrub(err.Error()), http.StatusBadRequest)
			return
		}
		if err := s.syncPrintGuardMCP(); err != nil {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}
}

func (s *Server) syncPrintGuardMCP() error {
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	runtime, _, _ := printGuardBindingConfig(&cfg, s.Vault, true)
	return tools.SetManagedMCPServer(desktopstore.PrintGuardMCPName, runtime)
}

func (s *Server) refreshPrintGuardMCP(store *desktopstore.Service) {
	ctx, cancel := desktopStoreOperationContext(s.ShutdownCh, 20*time.Second)
	defer cancel()
	if store != nil {
		_ = store.RevokePrintGuardVerification()
		_ = s.verifyPrintGuardConnection(ctx, store)
	}
	if err := s.syncPrintGuardMCP(); err != nil && s.Logger != nil {
		s.Logger.Warn("PrintGuard MCP reconciliation failed", "error", err)
	}
}

func (s *Server) startPrintGuardMCP() {
	ctx, cancel := desktopStoreOperationContext(s.ShutdownCh, 20*time.Second)
	defer cancel()
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	if _, _, err := printGuardBindingConfig(&cfg, s.Vault, false); err != nil {
		return
	}
	if store, err := s.getDesktopStoreService(ctx); err == nil {
		s.refreshPrintGuardMCP(store)
	}
}
