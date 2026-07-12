package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
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

	wsURL := "ws" + strings.TrimPrefix(proxy.URL, "http") + "/api/virtual-computers/machines/vm-1/vnc"
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

func virtualComputersTestConfig(upstreamURL string) *config.Config {
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.VirtualComputers.Provider = "boring_computers"
	cfg.VirtualComputers.ControlPlane.BoringdURL = upstreamURL
	cfg.VirtualComputers.BoringToken = "boring-token"
	cfg.Tools.VirtualComputers.Enabled = true
	return cfg
}
