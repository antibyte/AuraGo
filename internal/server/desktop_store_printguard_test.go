package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func printGuardServerFixture(t *testing.T, port int) (*Server, *desktopstore.Service) {
	t.Helper()
	oldProbe := printGuardGatewayProbe
	printGuardGatewayProbe = func(context.Context, string) error { return nil }
	t.Cleanup(func() { printGuardGatewayProbe = oldProbe })
	oldEngine := printGuardEngineRequest
	printGuardEngineRequest = func(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
		return []byte(`{"Architecture":"amd64","OSType":"linux"}`), 200, nil
	}
	t.Cleanup(func() { printGuardEngineRequest = oldEngine })
	root := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("42", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := desktopstore.NewService(desktopstore.Config{DBPath: filepath.Join(root, "desktop_store.db"), WorkspaceDir: filepath.Join(root, "workspace"), Docker: &serverStoreDockerAdapter{}, Secrets: vault, PortAllocator: serverFixedPorts(port), PortProbe: func(context.Context, string, int) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	op, err := store.StartInstall(context.Background(), desktopstore.InstallRequest{AppID: desktopstore.PrintGuardAppID, BindMode: "local", AllowedOrigins: []string{"https://aura.example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	s := testDesktopStoreServerWithService(t, store)
	s.Logger = slog.Default()
	s.Vault = vault
	s.Cfg.Directories.DataDir = root
	s.Cfg.Agent.AllowMCP, s.Cfg.MCP.Enabled = true, true
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Cfg.Auth.Enabled, s.Cfg.Auth.SessionSecret = true, "printguard-test-session-secret"
	s.Cfg.Server.HTTPS.Domain, s.Cfg.Server.PreviewDomain = "aura.example.org", "apps.example.net"
	return s, store
}

func printGuardOwnerRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, "https://aura.example.org"+path, strings.NewReader(body))
	r.Header.Set("Origin", "https://aura.example.org")
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue("printguard-test-session-secret", time.Now().Add(time.Hour))})
	return r
}

func configurePrintGuardFixture(t *testing.T, store *desktopstore.Service, mode, token string) {
	t.Helper()
	op, err := store.ConfigurePrintGuard(context.Background(), desktopstore.PrintGuardConfigUpdate{AgentMode: &mode, APIToken: token, Origins: []string{"https://aura.example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPrintGuardRoutesRequireAdminAndCSRF(t *testing.T) {
	s, read, write := testDesktopPermissionServer(t)
	s.Logger = slog.Default()
	for _, item := range []struct{ method, suffix string }{{"GET", "config"}, {"PUT", "config"}, {"POST", "test"}} {
		for _, token := range []string{"", read, write} {
			r := httptest.NewRequest(item.method, "https://aura.example.org/api/desktop/store/apps/printguard/"+item.suffix, bytes.NewBufferString(`{}`))
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			authMiddleware(s, handleDesktopStoreAppRoute(s)).ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Fatalf("%s %s admitted non-admin: %d", item.method, item.suffix, w.Code)
			}
		}
	}
	for _, path := range []string{"config", "test"} {
		method := "PUT"
		if path == "test" {
			method = "POST"
		}
		r := httptest.NewRequest(method, "https://aura.example.org/api/desktop/store/apps/printguard/"+path, strings.NewReader(`{}`))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", "https://foreign.example")
		w := httptest.NewRecorder()
		authMiddleware(s, handleDesktopStoreAppRoute(s)).ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-origin %s admitted: %d", path, w.Code)
		}
	}
}

func TestPrintGuardGatewayIsMandatoryAndHostProfilesAreBounded(t *testing.T) {
	s, _ := printGuardServerFixture(t, 18000)
	old := printGuardEngineRequest
	t.Cleanup(func() { printGuardEngineRequest = old })
	architecture, runtime := "amd64", true
	printGuardEngineRequest = func(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error) {
		return []byte(`{"Architecture":"` + architecture + `","OSType":"linux","Runtimes":` + map[bool]string{true: `{"nvidia":{}}`, false: `{}`}[runtime] + `}`), 200, nil
	}
	network := ""
	if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "intel", &network); err != nil {
		t.Fatal(err)
	}
	architecture = "arm64"
	if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "intel", &network); err == nil {
		t.Fatal("arm64 accepted GPU image")
	}
	architecture, runtime = "amd64", false
	if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "nvidia", &network); err == nil {
		t.Fatal("NVIDIA without toolkit accepted")
	}
	s.Cfg.Docker.Host = "tcp://192.168.1.45:2375"
	if _, err := s.printGuardHost(context.Background()); err == nil {
		t.Fatal("remote daemon accepted")
	}
	s.Cfg.Docker.Host = ""
	for _, validGateway := range []bool{false, true} {
		s.Cfg.Server.PreviewDomain = ""
		if validGateway {
			s.Cfg.Server.PreviewDomain = "apps.example.net"
		}
		w := httptest.NewRecorder()
		handleDesktopStoreOpenURL(s, "printguard").ServeHTTP(w, printGuardOwnerRequest("GET", "/api/desktop/store/apps/printguard/open-url", ""))
		if !validGateway && w.Code == http.StatusOK {
			t.Fatal("fell back to direct app URL")
		}
		if validGateway && (w.Code != http.StatusOK || !strings.Contains(w.Body.String(), ".apps.example.net")) {
			t.Fatalf("forced isolated launch missing: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPrintGuardMCPAuthenticationScopesAndRevocation(t *testing.T) {
	const token = "synthetic-printguard-server-token-981473"
	var anonymous atomic.Bool
	var requestsMu sync.Mutex
	var methods []string
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		methods = append(methods, r.Method+" "+r.URL.Path)
		requestsMu.Unlock()
		if !anonymous.Load() && r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stats":{"inference_device":"intel cpu"}}`))
	}))
	defer hub.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(hub.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	s, store := printGuardServerFixture(t, port)
	configurePrintGuardFixture(t, store, "read", token)
	old := printGuardProbeTools
	t.Cleanup(func() { printGuardProbeTools = old })
	probeCalls := 0
	printGuardProbeTools = func(context.Context, tools.MCPServerConfig, *slog.Logger) ([]string, error) {
		probeCalls++
		return append(append([]string{}, printGuardReadTools...), "heat_printer", "start_print", "remove_printer"), nil
	}
	anonymous.Store(true)
	if err := s.verifyPrintGuardConnection(context.Background(), store); err == nil || probeCalls != 0 {
		t.Fatal("anonymous hub or wrong token accepted")
	}
	anonymous.Store(false)
	if err := s.verifyPrintGuardConnection(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := printGuardBindingConfig(s.Cfg, s.Vault, true)
	if err != nil || slices.Contains(cfg.AllowedTools, "control_printer") || slices.Contains(cfg.AllowedTools, "heat_printer") || cfg.CheckAccess() != nil {
		t.Fatalf("read grant not enforced: %v", err)
	}
	configurePrintGuardFixture(t, store, "control", "")
	if cfg.CheckAccess() == nil {
		t.Fatal("token grant change did not revoke existing connection")
	}
	if err := s.verifyPrintGuardConnection(context.Background(), store); err == nil {
		t.Fatal("read token enabled control")
	}
	printGuardProbeTools = func(context.Context, tools.MCPServerConfig, *slog.Logger) ([]string, error) {
		return append(append([]string{}, printGuardReadTools...), "control_printer", "heat_printer"), nil
	}
	if err := s.verifyPrintGuardConnection(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	control, _, err := printGuardBindingConfig(s.Cfg, s.Vault, true)
	if err != nil || !slices.Contains(control.AllowedTools, "control_printer") || slices.Contains(control.AllowedTools, "heat_printer") {
		t.Fatalf("control scope widened: %v", err)
	}
	op, err := store.StartAppOperation(context.Background(), "printguard", desktopstore.OperationStop, desktopstore.OperationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if control.CheckAccess() == nil {
		t.Fatal("pending stop kept connection authorized")
	}
	if err := store.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	requestsMu.Lock()
	defer requestsMu.Unlock()
	for _, method := range methods {
		if method != "GET /api/v1/state" {
			t.Fatalf("test performed a printer mutation: %s", method)
		}
	}
}

func TestPrintGuardSharedNetworkSelection(t *testing.T) {
	s, _ := printGuardServerFixture(t, 18000)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = "tcp://docker-proxy:2375"
	old := printGuardEngineRequest
	t.Cleanup(func() { printGuardEngineRequest = old })
	networks := `{"aura-net":{},"aura-second":{},"docker-control":{},"bridge":{}}`
	printGuardEngineRequest = func(_ context.Context, _ tools.DockerConfig, method, path, body string) ([]byte, int, error) {
		if method != http.MethodGet || body != "" {
			t.Error("host preflight mutated Docker")
		}
		if path == "/info" {
			return []byte(`{"Architecture":"amd64","OSType":"linux"}`), 200, nil
		}
		if strings.HasPrefix(path, "/networks/") {
			return []byte(`{"Internal":` + strconv.FormatBool(path == "/networks/docker-control") + `}`), 200, nil
		}
		return []byte(`{"NetworkSettings":{"Networks":` + networks + `}}`), 200, nil
	}
	for _, name := range []string{"", "foreign", "bridge", "host", "docker-control"} {
		if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "cpu", &name); err == nil {
			t.Fatalf("unselected/foreign network accepted: %q", name)
		}
	}
	name := "aura-second"
	if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "cpu", &name); err != nil {
		t.Fatal(err)
	}
	networks = `{"aura-net":{},"bridge":{}}`
	name = ""
	if _, err := s.preparePrintGuardDeployment(printGuardOwnerRequest("GET", "/", ""), "cpu", &name); err != nil || name != "aura-net" {
		t.Fatalf("sole own network was not selected: %q %v", name, err)
	}
}

func TestPrintGuardEngineEndpointsStayLocal(t *testing.T) {
	for _, endpoint := range []string{"remote.example:2375", "tcp://remote.example:2375", "tcp://192.0.2.10:2375", "http://127.0.0.1:2375", "tcp://user@localhost:2375", "npipe://remote/pipe/docker_engine"} {
		if printGuardEngineIsLocal(endpoint, false) || printGuardEngineIsLocal(endpoint, true) {
			t.Fatalf("remote/unsupported endpoint accepted: %s", endpoint)
		}
	}
	for _, endpoint := range []string{"", "unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine", "127.0.0.1:2375", "tcp://[::1]:2375"} {
		if !printGuardEngineIsLocal(endpoint, false) {
			t.Fatalf("local endpoint refused: %s", endpoint)
		}
	}
	if printGuardEngineIsLocal("tcp://docker-proxy:2375", false) || !printGuardEngineIsLocal("tcp://docker-proxy:2375", true) {
		t.Fatal("bundled proxy is not scoped to containerized AuraGo")
	}
}

func TestPrintGuardGatewayChecksTLSAndRoutingWithoutCredentials(t *testing.T) {
	code, body := http.StatusUnauthorized, "Preview launch expired\n"
	var count atomic.Int32
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || !strings.HasPrefix(r.URL.Path, previewLaunchPath) {
			t.Error("gateway probe sent a credential or used the wrong route")
		}
		if code == http.StatusFound {
			w.Header().Set("Location", "/redirected")
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	defer gateway.Close()
	if checkPrintGuardGateway(context.Background(), gateway.URL) == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	old := printGuardGatewayClient
	t.Cleanup(func() { printGuardGatewayClient = old })
	printGuardGatewayClient = gateway.Client()
	printGuardGatewayClient.CheckRedirect = old.CheckRedirect
	if err := checkPrintGuardGateway(context.Background(), gateway.URL); err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct {
		code int
		body string
	}{{200, "Preview launch expired\n"}, {401, "unrelated auth proxy"}, {302, ""}} {
		code, body = value.code, value.body
		before := count.Load()
		if checkPrintGuardGateway(context.Background(), gateway.URL) == nil || count.Load() != before+1 {
			t.Fatal("wrong gateway or redirected probe accepted")
		}
	}
}

func TestPrintGuardCanRevokeWithBrokenGateway(t *testing.T) {
	s, store := printGuardServerFixture(t, 18000)
	configurePrintGuardFixture(t, store, "read", "synthetic-pg-revocation-284063")
	s.Cfg.Server.PreviewDomain = ""
	w := httptest.NewRecorder()
	handleDesktopStorePrintGuardConfig(s).ServeHTTP(w, printGuardOwnerRequest("PUT", "/api/desktop/store/apps/printguard/config", `{"clear_api_token":true}`))
	if w.Code != http.StatusAccepted {
		t.Fatalf("revocation required working gateway: %d %s", w.Code, w.Body.String())
	}
	var accepted struct {
		Operation desktopstore.Operation `json:"operation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	status, err := store.PrintGuardConfiguration(context.Background())
	if err != nil || status.TokenConfigured || status.AgentMode != "off" {
		t.Fatalf("removal did not immediately revoke access: %+v %v", status, err)
	}
	for i := 0; i < 100; i++ {
		if _, err := desktopstore.ReadPrintGuardAgentBinding(context.Background(), filepath.Join(s.Cfg.Directories.DataDir, "desktop_store.db"), s.Vault); err == nil {
			t.Fatal("removed token still authorizes the bridge")
		}
		op, err := store.Operation(context.Background(), accepted.Operation.ID)
		if err == nil && op.Status == desktopstore.OperationSucceeded {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("revocation operation did not finish")
}

func TestPrintGuardDesktopGrantChangeReconcilesMCP(t *testing.T) {
	s := cydConfigServer(t, "agent:\n  allow_mcp: true\nmcp:\n  enabled: true\nvirtual_desktop:\n  enabled: true\n  readonly: false\n")
	oldInit, oldStop := initExternalMCPManager, shutdownExternalMCPManager
	t.Cleanup(func() { initExternalMCPManager, shutdownExternalMCPManager = oldInit, oldStop })
	starts, stops := 0, 0
	shutdownExternalMCPManager = func() uint64 { stops++; return uint64(stops) }
	initExternalMCPManager = func([]tools.MCPServerConfig, *slog.Logger, uint64) *tools.MCPManager { starts++; return nil }
	for _, body := range []string{`{"virtual_desktop":{"readonly":true}}`, `{"virtual_desktop":{"enabled":false}}`} {
		before := stops
		w := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body)))
		if w.Code != http.StatusOK || stops != before+1 || starts != stops {
			t.Fatalf("Desktop grant did not revoke and rebuild MCP: code=%d starts=%d stops=%d", w.Code, starts, stops)
		}
	}
}
