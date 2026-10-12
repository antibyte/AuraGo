package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestRemoteEnrollmentCreateReturnsOneTimeToken(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/remote/enroll", strings.NewReader(`{"device_name":"agodesk-desktop"}`))
	req.Header.Set("Content-Type", "application/json")
	handleRemoteEnrollmentCreate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		EnrollmentID string `json:"enrollment_id"`
		Token        string `json:"token"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.EnrollmentID == "" || payload.Token == "" || payload.ExpiresAt == "" {
		t.Fatalf("payload missing required fields: %+v", payload)
	}
	enrollment := assertRemoteTokenStoredAsLookupHash(t, s, payload.Token)
	if enrollment.ID != payload.EnrollmentID || enrollment.DeviceName != "agodesk-desktop" || enrollment.Used {
		t.Fatalf("stored enrollment = %+v, response = %+v", enrollment, payload)
	}
}

// assertRemoteTokenStoredAsLookupHash checks that remote_enrollments holds only
// the lookup hash of token and the vault holds its MAC key.
func assertRemoteTokenStoredAsLookupHash(t *testing.T, s *Server, token string) remote.EnrollmentRecord {
	t.Helper()
	enrollment, err := remote.GetEnrollmentByTokenHash(s.RemoteHub.DB(), remote.DeriveEnrollmentLookupHash(token))
	if err != nil {
		t.Fatalf("enrollment not stored under its lookup hash: %v", err)
	}
	for name, value := range map[string]string{
		"raw token":  token,
		"plain hash": hashSHA256(token),
		"MAC key":    remote.DeriveEnrollmentAuthKey(token),
	} {
		var count int
		if err := s.RemoteHub.DB().QueryRow(`SELECT COUNT(*) FROM remote_enrollments WHERE token_hash = ?`, value).Scan(&count); err != nil {
			t.Fatalf("query %s count: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("remote_enrollments must not hold the %s", name)
		}
	}
	key, err := s.Vault.ReadSecret("remote_enroll_key_" + enrollment.ID)
	if err != nil || key != remote.DeriveEnrollmentAuthKey(token) {
		t.Fatalf("vault MAC key for enrollment %s: %v", enrollment.ID, err)
	}
	return enrollment
}

func TestRemoteEnrollmentCreateFailsWhenMACKeyCannotBeStored(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()
	brokenVault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "missing", "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = brokenVault
	s.RemoteHub = remote.NewRemoteHub(s.RemoteHub.DB(), brokenVault, slog.Default())

	for name, serve := range map[string]func() *httptest.ResponseRecorder{
		"create": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			handleRemoteEnrollmentCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/remote/enroll", strings.NewReader(`{"device_name":"x"}`)))
			return rec
		},
		"download": func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
			req.Host = "localhost:8090"
			handleRemoteDownload(s).ServeHTTP(rec, req)
			return rec
		},
	} {
		if rec := serve(); rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d, want 500; body=%s", name, rec.Code, rec.Body.String())
		}
	}
	var count int
	if err := s.RemoteHub.DB().QueryRow(`SELECT COUNT(*) FROM remote_enrollments`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("a token without a stored MAC key must not be created, got %d rows", count)
	}
}

func TestRemoteDownloadTrailerCarriesRawTokenAndStoresLookupHash(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if !strings.HasPrefix(trailer.EnrollToken, "remote_") || trailer.DeviceName != "nas" {
		t.Fatalf("trailer must carry the raw token and device name: %+v", trailer)
	}
	enrollment := assertRemoteTokenStoredAsLookupHash(t, s, trailer.EnrollToken)
	if enrollment.DeviceName != "nas" || enrollment.Used {
		t.Fatalf("stored enrollment = %+v", enrollment)
	}
}

func TestRemoteDownloadUsesTailscaleSupervisorURL(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, func(cfg *config.Config) {
		cfg.Server.Port = 8090
		cfg.RemoteControl.ConnectionMode = "tailscale"
		cfg.RemoteControl.TailscaleAddress = "aurago.tailnet.ts.net"
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if trailer.SupervisorURL != "wss://aurago.tailnet.ts.net/api/remote/ws" {
		t.Fatalf("supervisor_url = %q", trailer.SupervisorURL)
	}
}

func TestRemoteDownloadUsesManualSupervisorURL(t *testing.T) {
	s, cleanup := newRemoteDownloadTestServer(t, func(cfg *config.Config) {
		cfg.RemoteControl.ConnectionMode = "manual"
		cfg.RemoteControl.SupervisorURL = "https://remote.example.com/custom/ws"
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64?name=nas", nil)
	req.Host = "localhost:8090"
	handleRemoteDownload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	trailer, err := remote.ParseBinaryTrailer(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("parse personalized binary trailer: %v", err)
	}
	if trailer.SupervisorURL != "wss://remote.example.com/custom/ws" {
		t.Fatalf("supervisor_url = %q", trailer.SupervisorURL)
	}
}

func TestRemoteDownloadTransportMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, host, proto, want string
		tls, trusted, serverTLS bool
	}{
		{name: "TLS public port", host: "public.example:9443", tls: true, want: "wss://public.example:9443/api/remote/ws"},
		{name: "trusted proxy", host: "public.example:443", proto: "https", trusted: true, want: "wss://public.example:443/api/remote/ws"},
		{name: "forged proxy", host: "public.example", proto: "https"},
		{name: "plaintext", host: "public.example:8090"},
		{name: "loopback still needs TLS", host: "127.0.0.1:8090"},
		{name: "dedicated TLS", host: "aurago.lan:8090", serverTLS: true, want: "wss://aurago.lan:8443/api/remote/ws"},
		{name: "IPv6", host: "[fd00::5]:9443", tls: true, want: "wss://[fd00::5]:9443/api/remote/ws"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.Port = 8090
			cfg.Server.HTTPS.Enabled = tc.serverTLS
			cfg.Server.HTTPS.HTTPSPort = 8443
			req := httptest.NewRequest(http.MethodGet, "/api/remote/download/linux/amd64", nil)
			req.Host = tc.host
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			req.Header.Set("X-Forwarded-Proto", tc.proto)
			if tc.trusted {
				req = req.WithContext(context.WithValue(req.Context(), trustedProxyContextKey{}, true))
			}
			got, err := remoteDownloadSupervisorURL(&Server{Cfg: cfg}, req)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, raw := range []string{"ws://localhost/ws", "http://private.example/ws"} {
		if _, err := normalizeRemoteSupervisorURL(raw, false, 8090); err == nil {
			t.Fatalf("accepted plaintext URL %s", raw)
		}
	}
}

// The global remote_control.allowed_paths is the default for devices without
// their own list; the hub reads it from the config snapshot on every call.
func TestRemoteHubDefaultAllowedPathsFollowsConfigSnapshot(t *testing.T) {
	cfg := &config.Config{}
	cfg.RemoteControl.AllowedPaths = []string{"/srv"}
	cfg.RemoteControl.ReadOnly = true
	cfg.RemoteControl.MaxFileSizeMB = 7
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()

	hub := s.newRemoteHub(nil, nil, slog.Default(), cfg)
	if hub.DefaultAllowedPaths == nil {
		t.Fatal("DefaultAllowedPaths is not wired")
	}
	if got := hub.DefaultAllowedPaths(); !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("DefaultAllowedPaths() = %q, want [/srv]", got)
	}
	if !hub.DefaultReadOnly || hub.MaxFileSizeMB != 7 {
		t.Fatalf("hub defaults = read-only %v, max file size %d; want true, 7", hub.DefaultReadOnly, hub.MaxFileSizeMB)
	}

	next := &config.Config{}
	next.RemoteControl.AllowedPaths = []string{"/data"}
	s.replaceConfigSnapshot(next)
	if got := hub.DefaultAllowedPaths(); !reflect.DeepEqual(got, []string{"/data"}) {
		t.Fatalf("DefaultAllowedPaths() after reload = %q, want [/data]", got)
	}
}

// A reload that changes remote_control.allowed_paths pushes the new default to
// connected devices without their own list; an unchanged list pushes nothing.
func TestReplaceConfigSnapshotPushesChangedDefaultAllowedPaths(t *testing.T) {
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := remote.CreateDevice(db, remote.DeviceRecord{Name: "nas", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.RemoteControl.AllowedPaths = []string{"/srv"}
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()
	s.RemoteHub = s.newRemoteHub(db, nil, slog.Default(), cfg)

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnCh := make(chan *websocket.Conn, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
	}))
	defer wsServer.Close()
	agentConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer agentConn.Close()
	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(5 * time.Second):
		t.Fatal("websocket server connection missing")
	}
	defer serverConn.Close()
	s.RemoteHub.Register(deviceID, &remote.RemoteConnection{Conn: serverConn, DeviceID: deviceID, SharedKey: strings.Repeat("b", 64)})

	unchanged := &config.Config{}
	unchanged.RemoteControl.AllowedPaths = []string{"/srv"}
	s.replaceConfigSnapshot(unchanged)
	changed := &config.Config{}
	changed.RemoteControl.AllowedPaths = []string{"/data"}
	s.replaceConfigSnapshot(changed)

	// The changed list is pushed.
	_ = agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg remote.RemoteMessage
	if err := agentConn.ReadJSON(&msg); err != nil {
		t.Fatalf("read pushed config update: %v", err)
	}
	var update struct {
		AllowedPaths []string `json:"allowed_paths"`
	}
	if msg.Type != remote.MsgConfigUpdate || json.Unmarshal(msg.Payload, &update) != nil || !reflect.DeepEqual(update.AllowedPaths, []string{"/data"}) {
		t.Fatalf("agent received %s %s, want a config update with allowed_paths [/data]", msg.Type, msg.Payload)
	}

	// Reloading the same list again pushes nothing. A push from either
	// unchanged reload would arrive here.
	same := &config.Config{}
	same.RemoteControl.AllowedPaths = []string{"/data"}
	s.replaceConfigSnapshot(same)
	_ = agentConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	err = agentConn.ReadJSON(&msg)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("unchanged reload: agent read = %s %s (err %v), want no frame", msg.Type, msg.Payload, err)
	}
}

func newRemoteDownloadTestServer(t *testing.T, mutate func(*config.Config)) (*Server, func()) {
	t.Helper()

	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll("deploy", 0o755); err != nil {
		t.Fatalf("MkdirAll deploy: %v", err)
	}
	generic, err := remote.BuildPersonalizedBinary([]byte("binary"), remote.BinaryConfig{SupervisorURL: "ws://placeholder", EnrollToken: "placeholder"})
	if err != nil {
		t.Fatalf("BuildPersonalizedBinary fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join("deploy", "aurago-remote_linux_amd64"), generic, 0o644); err != nil {
		t.Fatalf("WriteFile binary: %v", err)
	}

	db, err := remote.InitDB(filepath.Join(tmp, "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmp, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}

	cfg := &config.Config{}
	cfg.Server.Port = 8090
	cfg.Server.HTTPS.Enabled = true
	if mutate != nil {
		mutate(cfg)
	}
	s := &Server{
		Cfg:       cfg,
		Logger:    slog.Default(),
		Vault:     vault,
		RemoteHub: remote.NewRemoteHub(db, vault, slog.Default()),
	}
	cleanup := func() {
		_ = os.Chdir(oldWD)
	}
	return s, cleanup
}
