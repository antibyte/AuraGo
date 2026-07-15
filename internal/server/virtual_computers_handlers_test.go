package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/virtualcomputers"

	"github.com/gorilla/websocket"
)

func TestVirtualComputersPreviewProxyKeepsTokenServerSide(t *testing.T) {
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/machines/vm-1/web/8080/app/" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("preview ok"))
	}))
	defer upstream.Close()

	s := &Server{Cfg: virtualComputersTestConfig(upstream.URL)}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/web/8080/app/", nil)

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamAuth != "Bearer boring-token" {
		t.Fatalf("upstream auth = %q", upstreamAuth)
	}
	if strings.Contains(rec.Body.String(), "boring-token") {
		t.Fatalf("response leaked token: %s", rec.Body.String())
	}
}

func TestVirtualComputersWebSocketProxyPassesBinary(t *testing.T) {
	var upstreamAuth string
	var upstreamGoal string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		upstreamGoal = r.URL.Query().Get("goal")
		if r.URL.Path != "/v1/machines/vm-1/vnc" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade upstream: %v", err)
			return
		}
		defer conn.Close()
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read upstream: %v", err)
			return
		}
		if mt != websocket.BinaryMessage || string(msg) != "ping" {
			t.Errorf("upstream got mt=%d msg=%q", mt, msg)
			return
		}
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("pong"))
	}))
	defer upstream.Close()

	s := &Server{Cfg: virtualComputersTestConfig(upstream.URL)}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	proxy := httptest.NewServer(mux)
	defer proxy.Close()

	wsURL := "ws" + strings.TrimPrefix(proxy.URL, "http") + "/api/virtual-computers/machines/vm-1/vnc?goal=" + url.QueryEscape("open docs & report")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("ping")); err != nil {
		t.Fatalf("write proxy: %v", err)
	}
	mt, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read proxy: %v", err)
	}
	if mt != websocket.BinaryMessage || string(msg) != "pong" {
		t.Fatalf("proxy returned mt=%d msg=%q", mt, msg)
	}
	if upstreamAuth != "Bearer boring-token" {
		t.Fatalf("upstream auth = %q", upstreamAuth)
	}
	if upstreamGoal != "open docs & report" {
		t.Fatalf("upstream goal = %q", upstreamGoal)
	}
}

func TestVirtualComputersTTYWebSocketPassesBinaryWithWriteAndAdminAccess(t *testing.T) {
	var upstreamAuth string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/machines/vm-1/tty" {
			t.Errorf("upstream path = %s", r.URL.Path)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade upstream: %v", err)
			return
		}
		defer conn.Close()
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read upstream: %v", err)
			return
		}
		if mt != websocket.BinaryMessage || string(msg) != "printf terminal" {
			t.Errorf("upstream got mt=%d msg=%q", mt, msg)
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte("terminal output")); err != nil {
			t.Errorf("write upstream: %v", err)
		}
	}))
	defer upstream.Close()

	s, _, writeToken := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("desktop admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatalf("create admin token: %v", err)
	}
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	s.Cfg.VirtualComputers.AllowAgentTasks = false
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	proxy := httptest.NewServer(mux)
	defer proxy.Close()

	wsURL := "ws" + strings.TrimPrefix(proxy.URL, "http") + "/api/virtual-computers/machines/vm-1/tty"
	for _, tc := range []struct {
		name, token string
	}{
		{name: "write", token: writeToken},
		{name: "admin", token: adminToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{"Authorization": []string{"Bearer " + tc.token}}
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err != nil {
				t.Fatalf("dial proxy: %v", err)
			}
			defer conn.Close()
			if resp.Header.Get("Authorization") != "" {
				t.Fatalf("downstream response leaked authorization header")
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte("printf terminal")); err != nil {
				t.Fatalf("write proxy: %v", err)
			}
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read proxy: %v", err)
			}
			if mt != websocket.BinaryMessage || string(msg) != "terminal output" {
				t.Fatalf("proxy returned mt=%d msg=%q", mt, msg)
			}
		})
	}
	if upstreamAuth != "Bearer boring-token" {
		t.Fatalf("upstream auth = %q", upstreamAuth)
	}
}

func TestVirtualComputersTTYRejectsReadOnlyAndReadAccessBeforeUpstreamDial(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, upstreamURL string) (*Server, string)
	}{
		{
			name: "read only configuration",
			setup: func(t *testing.T, upstreamURL string) (*Server, string) {
				cfg := virtualComputersTestConfig(upstreamURL)
				cfg.VirtualComputers.ReadOnly = true
				return &Server{Cfg: cfg}, ""
			},
		},
		{
			name: "desktop read token",
			setup: func(t *testing.T, upstreamURL string) (*Server, string) {
				s, readToken, _ := testDesktopPermissionServer(t)
				s.Cfg.VirtualComputers = virtualComputersTestConfig(upstreamURL).VirtualComputers
				return s, readToken
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upstreamRequests := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests++
				w.WriteHeader(http.StatusSwitchingProtocols)
			}))
			defer upstream.Close()

			s, token := tc.setup(t, upstream.URL)
			mux := http.NewServeMux()
			registerVirtualComputersRoutes(mux, s)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/tty", nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "websocket")
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if upstreamRequests != 0 {
				t.Fatalf("upstream received %d TTY requests", upstreamRequests)
			}
		})
	}
}

func TestVirtualComputersVNCRejectsReadOnlyBeforeUpstreamDial(t *testing.T) {
	upstreamRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests++
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer upstream.Close()

	cfg := virtualComputersTestConfig(upstream.URL)
	cfg.VirtualComputers.ReadOnly = true
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/vnc", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if upstreamRequests != 0 {
		t.Fatalf("upstream received %d VNC requests in read-only mode", upstreamRequests)
	}
}

func TestVirtualComputersVNCRejectsDesktopReadTokenBeforeUpstreamDial(t *testing.T) {
	upstreamRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests++
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer upstream.Close()

	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/vnc", nil)
	req.Header.Set("Authorization", "Bearer "+readToken)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if upstreamRequests != 0 {
		t.Fatalf("upstream received %d VNC requests for desktop:read token", upstreamRequests)
	}
}

func TestVirtualComputersAgentWebSocketHonorsTaskAndReadonlyGates(t *testing.T) {
	for _, tc := range []struct {
		name      string
		readonly  bool
		allowTask bool
	}{
		{name: "read only", readonly: true, allowTask: true},
		{name: "disabled", allowTask: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := virtualComputersTestConfig("http://127.0.0.1:1")
			cfg.VirtualComputers.ReadOnly = tc.readonly
			cfg.VirtualComputers.AllowAgentTasks = tc.allowTask
			mux := http.NewServeMux()
			registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/agent?goal=work", nil))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestVirtualComputersRESTUsesPinnedBoringdContracts(t *testing.T) {
	requests := make(chan *http.Request, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/machines/vm-1/publish":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "desktop-template" {
				t.Errorf("publish body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"name":"desktop-template"}`))
		case r.URL.Path == "/v1/machines/vm-1/branch":
			if r.URL.Query().Get("count") != "2" {
				t.Errorf("fork count = %q", r.URL.Query().Get("count"))
			}
			_, _ = w.Write([]byte(`{"machines":[{"id":"vm-2"},{"id":"vm-3"}]}`))
		case r.URL.Path == "/v1/machines/vm-1/save":
			if r.URL.Query().Get("volume") != "vol-1" {
				t.Errorf("save volume = %q", r.URL.Query().Get("volume"))
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected upstream request %s", r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	cfg := virtualComputersTestConfig(upstream.URL)
	cfg.VirtualComputers.AllowPublish = true
	cfg.VirtualComputers.AllowVolumes = true
	s := &Server{Cfg: cfg}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)

	cases := []struct {
		path string
		body string
	}{
		{"/api/virtual-computers/machines/vm-1/publish", `{"name":"desktop-template"}`},
		{"/api/virtual-computers/machines/vm-1/fork", `{"count":2}`},
		{"/api/virtual-computers/machines/vm-1/save", `{"volume_id":"vol-1"}`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if got := len(requests); got != len(cases) {
		t.Fatalf("upstream request count = %d", got)
	}
}

func TestVirtualComputersRESTRejectsLegacyForkTTL(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: virtualComputersTestConfig(upstream.URL)})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/machines/vm-1/fork", strings.NewReader(`{"count":1,"ttl_seconds":300}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_argument") {
		t.Fatalf("fork status=%d body=%s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Fatalf("legacy fork reached upstream %d times", calls)
	}
}

func TestVirtualComputersRESTRejectsExecArgsAndClassifiesCapabilityErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"machine has no vsock device"}`))
	}))
	defer upstream.Close()
	s := &Server{Cfg: virtualComputersTestConfig(upstream.URL)}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/machines/vm-1/exec", strings.NewReader(`{"command":"echo","args":["hello"]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("exec args status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/machines/vm-1/screenshot", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "capability_unavailable") {
		t.Fatalf("screenshot status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersRESTAgentTaskHistoryRemainsReadable(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]string{"type": "done", "text": "complete"})
	}))
	defer upstream.Close()
	mgr, err := virtualcomputers.OpenTaskManager(filepath.Join(t.TempDir(), "virtual_computers.db"), slog.Default(), virtualcomputers.TaskManagerOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	defer mgr.Close()
	virtualcomputers.SetDefaultTaskManager(mgr)
	defer virtualcomputers.SetDefaultTaskManager(nil)
	cfg := virtualComputersTestConfig(upstream.URL)
	cfg.VirtualComputers.AllowAgentTasks = true
	s := &Server{Cfg: cfg}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/tasks", bytes.NewBufferString(`{"machine_id":"vm-1","kind":"shell","instruction":"check disk"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	var started struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if started.TaskID == "" || started.Status != virtualcomputers.AgentTaskStatusQueued {
		t.Fatalf("start response = %+v body=%s", started, rec.Body.String())
	}

	cfg.VirtualComputers.ReadOnly = true
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/tasks/"+started.TaskID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersStorageTestUsesConfiguredVaultCredentialsReadOnly(t *testing.T) {
	var method string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.Method != http.MethodHead || r.URL.Path != "/boring-volumes/" {
			t.Errorf("storage request = %s %s", r.Method, r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=storage-access/") {
			t.Errorf("storage authorization missing configured access key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer storage.Close()
	cfg := virtualComputersTestConfig("http://127.0.0.1:1")
	cfg.VirtualComputers.AllowVolumes = true
	cfg.VirtualComputers.Storage.Endpoint = strings.TrimPrefix(storage.URL, "http://")
	cfg.VirtualComputers.Storage.Bucket = "boring-volumes"
	cfg.VirtualComputers.Storage.UseSSL = false
	cfg.VirtualComputers.S3AccessKeyID = "storage-access"
	cfg.VirtualComputers.S3SecretKey = "storage-secret"
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/storage/test", nil))
	if rec.Code != http.StatusOK || method != http.MethodHead {
		t.Fatalf("status=%d method=%s body=%s", rec.Code, method, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "storage-secret") {
		t.Fatalf("storage test leaked secret: %s", rec.Body.String())
	}
}

func TestVirtualComputersEnsureBoringTokenStoresVaultOnly(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	cfg := virtualComputersTestConfig("http://127.0.0.1:8080")
	cfg.VirtualComputers.BoringToken = ""
	s := &Server{Cfg: cfg, Vault: vault}

	token, generated, err := virtualComputersEnsureBoringToken(s, virtualcomputers.FromAuraConfig(cfg))
	if err != nil {
		t.Fatalf("ensure token: %v", err)
	}
	if !generated {
		t.Fatal("expected generated token")
	}
	if !strings.HasPrefix(token, "boring_") {
		t.Fatalf("token prefix = %q", token)
	}
	stored, err := vault.ReadSecret("virtual_computers_boring_token")
	if err != nil {
		t.Fatalf("read vault token: %v", err)
	}
	if stored != token {
		t.Fatalf("stored token mismatch")
	}
	if s.Cfg.VirtualComputers.BoringToken != token {
		t.Fatalf("runtime config token was not updated")
	}
}

func TestParseVirtualComputersSSHTarget(t *testing.T) {
	user, host, port := parseVirtualComputersSSHTarget("root@example.test:2222", 22)
	if user != "root" || host != "example.test" || port != 2222 {
		t.Fatalf("parsed = user=%q host=%q port=%d", user, host, port)
	}
	user, host, port = parseVirtualComputersSSHTarget("[2001:db8::1]:2200", 22)
	if user != "" || host != "2001:db8::1" || port != 2200 {
		t.Fatalf("parsed ipv6 = user=%q host=%q port=%d", user, host, port)
	}
}

func TestVirtualComputersLoopbackListenAddr(t *testing.T) {
	addr, ok := virtualComputersLoopbackListenAddr("http://localhost:18080")
	if !ok || addr != "127.0.0.1:18080" {
		t.Fatalf("loopback addr = %q ok=%v", addr, ok)
	}
	if addr, ok := virtualComputersLoopbackListenAddr("https://example.test:8443"); ok || addr != "" {
		t.Fatalf("public addr should not tunnel, got %q ok=%v", addr, ok)
	}
}

func TestVirtualComputersSetupManagerLocalHostDoesNotRequireSSHSecret(t *testing.T) {
	cfg := virtualcomputers.ToolConfig{
		ControlPlane: virtualcomputers.ControlPlaneConfig{
			Mode:       "local_host",
			InstallDir: "/opt/boring-computers",
			BoringdURL: "http://127.0.0.1:8080",
		},
	}
	manager, err := virtualComputersSetupManager(&Server{}, cfg, "boring-token")
	if err != nil {
		t.Fatalf("setup manager should not require SSH for local_host: %v", err)
	}
	if _, ok := manager.Executor.(virtualcomputers.LocalCommandExecutor); !ok {
		t.Fatalf("executor = %T, want virtualcomputers.LocalCommandExecutor", manager.Executor)
	}
}

func TestVirtualComputersSetupManagerUsesStoredSudoPasswordForLocalHost(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("sudo_password", "vault-sudo-secret"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneLocalHost}}

	manager, err := virtualComputersSetupManager(&Server{Vault: vault}, cfg, "boring-token")
	if err != nil {
		t.Fatalf("virtualComputersSetupManager: %v", err)
	}
	executor, ok := manager.Executor.(virtualcomputers.LocalCommandExecutor)
	if !ok {
		t.Fatalf("executor = %T", manager.Executor)
	}
	if executor.SudoPassword != "vault-sudo-secret" || manager.SudoPassword != "vault-sudo-secret" {
		t.Fatal("local setup did not reuse central sudo_password")
	}
}

func TestVirtualComputersSetupStatusReportsStoredSudoPasswordWithoutLeakingIt(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("sudo_password", "vault-sudo-secret"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	cfg := virtualComputersTestConfig("http://127.0.0.1:1")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	rec := httptest.NewRecorder()

	handleVirtualComputersSetupStatus(&Server{Cfg: cfg, Vault: vault}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["sudo_password_stored"] != true {
		t.Fatalf("sudo_password_stored = %#v", body["sudo_password_stored"])
	}
	if value := body["has_sudo_or_root"]; value != nil {
		hasSudoOrRoot, ok := value.(bool)
		if !ok || !hasSudoOrRoot {
			t.Fatalf("has_sudo_or_root = %#v, want unknown or confirmed passwordless sudo access", value)
		}
	}
	if strings.Contains(rec.Body.String(), "vault-sudo-secret") {
		t.Fatalf("status leaked sudo password: %s", rec.Body.String())
	}
}

func TestVirtualComputersSetupStatusReportsMissingSudoPassword(t *testing.T) {
	cfg := virtualComputersTestConfig("http://127.0.0.1:1")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	rec := httptest.NewRecorder()
	handleVirtualComputersSetupStatus(&Server{Cfg: cfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil))

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["sudo_password_stored"] != false {
		t.Fatalf("sudo_password_stored = %#v", body["sudo_password_stored"])
	}
}

func TestVirtualComputersSetupOptionsCarriesConfiguredBoringdURL(t *testing.T) {
	cfg := virtualcomputers.ToolConfig{
		ControlPlane: virtualcomputers.ControlPlaneConfig{
			BoringdURL: "http://127.0.0.1:18080",
		},
		Storage:       virtualcomputers.StorageConfig{Endpoint: "minio.local:9000", Bucket: "vc", Region: "home-1", UseSSL: true},
		S3AccessKeyID: "access",
		S3SecretKey:   "secret",
	}
	opts := virtualComputersSetupOptions(cfg, "boring-token", virtualComputersSetupRequest{})
	if opts.BoringdURL != "http://127.0.0.1:18080" {
		t.Fatalf("BoringdURL = %q", opts.BoringdURL)
	}
	if opts.S3Endpoint != "minio.local:9000" || opts.S3Bucket != "vc" || opts.S3Region != "home-1" || !opts.S3UseSSL || opts.S3AccessKeyID != "access" || opts.S3SecretKey != "secret" {
		t.Fatalf("storage options = %+v", opts)
	}
}

func TestVirtualComputersSetupStatusIncludesLocalModeMetadata(t *testing.T) {
	cfg := virtualComputersTestConfig("http://127.0.0.1:8080")
	cfg.VirtualComputers.ControlPlane.Mode = "local_host"
	s := &Server{Cfg: cfg}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil)
	handleVirtualComputersSetupStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["mode"] != "local_host" {
		t.Fatalf("mode = %#v, body=%v", body["mode"], body)
	}
	for _, key := range []string{"host_os", "arch", "running_in_docker", "has_kvm", "has_systemd", "has_sudo_or_root"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("status response missing %q: %v", key, body)
		}
	}
}

func TestVirtualComputersSetupStatusLeavesRemoteChecksUnknownWithoutPreflight(t *testing.T) {
	cfg := virtualComputersTestConfig("http://127.0.0.1:8080")
	cfg.VirtualComputers.ControlPlane.Mode = "ssh_host"
	s := &Server{Cfg: cfg}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil)
	handleVirtualComputersSetupStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["mode"] != "ssh_host" {
		t.Fatalf("mode = %#v, body=%v", body["mode"], body)
	}
	for _, key := range []string{"running_in_docker", "has_kvm", "has_systemd", "has_sudo_or_root"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("status response missing %q: %v", key, body)
		}
		if body[key] != nil {
			t.Fatalf("%s = %#v, want null until preflight has run; body=%v", key, body[key], body)
		}
	}
}

func virtualComputersTestConfig(upstreamURL string) *config.Config {
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.VirtualComputers.Provider = "boring_computers"
	cfg.VirtualComputers.ControlPlane.BoringdURL = upstreamURL
	cfg.VirtualComputers.BoringToken = "boring-token"
	cfg.Tools.VirtualComputers.Enabled = true
	return cfg
}
