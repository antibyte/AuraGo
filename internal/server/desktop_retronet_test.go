package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/retronet"
	"aurago/internal/security"
)

const retroNetTestEntryID = "own-fakebbs01"

// retroNetTestEnv is a Desktop-enabled server with auth, three tokens, the
// Retro-Net routes plus /api/desktop/settings, and a fake Telnet service.
type retroNetTestEnv struct {
	s                                 *Server
	mux                               *http.ServeMux
	httpServer                        *httptest.Server
	telnet                            *retroNetFakeTelnet
	shutdown                          context.CancelFunc
	adminToken, writeToken, readToken string
}

// newRetroNetTestEnv builds the environment. seed functions run before the
// HTTP server starts, so they may adjust the pre-seeded engine race-free.
func newRetroNetTestEnv(t *testing.T, seed ...func(*Server)) *retroNetTestEnv {
	t.Helper()
	s := newDesktopFilesystemTestServer(t)
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	env := &retroNetTestEnv{s: s}
	for _, grant := range []struct {
		token *string
		scope string
	}{
		{&env.adminToken, desktopScopeAdmin},
		{&env.writeToken, desktopScopeWrite},
		{&env.readToken, desktopScopeRead},
	} {
		raw, _, err := tokens.Create("retronet "+grant.scope, []string{grant.scope}, nil)
		if err != nil {
			t.Fatalf("create %s token: %v", grant.scope, err)
		}
		*grant.token = raw
	}
	s.Vault, s.TokenManager = vault, tokens
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "retronet-session-secret"
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.VirtualDesktop.RetroNetEnabled = true
	integrationCtx, shutdown := context.WithCancel(context.Background())
	t.Cleanup(shutdown)
	s.integrationCtx, env.shutdown = integrationCtx, shutdown
	env.telnet = startRetroNetFakeTelnet(t)
	network := retroNetTestNetwork{port: env.telnet.port()}
	// Test seam: (*Server).retroNet keeps pre-seeded values and only fills in
	// what is missing (here: the first-contact host-key callback).
	s.retroNetManager = &retronet.Manager{Dialer: network.dialer()}
	s.retroNetStatus = &retronet.StatusProber{Dialer: network.dialer(), Timeout: time.Second}
	for _, apply := range seed {
		apply(s)
	}
	env.mux = http.NewServeMux()
	registerDesktopRetroNetRoutes(env.mux, s)
	env.mux.HandleFunc("/api/desktop/settings", handleDesktopSettings(s))
	// httptest.Server.Close does not wait for hijacked (WebSocket) handlers.
	// Wait for them before the Desktop service and temp dirs go away: a session
	// end audits through the current service and may reopen it.
	var handlers sync.WaitGroup
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() { handlers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Retro-Net handlers still running at cleanup")
		}
	})
	env.httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.Add(1)
		defer handlers.Done()
		env.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(env.httpServer.Close)
	return env
}

func (e *retroNetTestEnv) configure(change func(vd *config.VirtualDesktopConfig)) {
	e.s.CfgMu.Lock()
	change(&e.s.Cfg.VirtualDesktop)
	e.s.CfgMu.Unlock()
}

func (e *retroNetTestEnv) serve(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func (e *retroNetTestEnv) directory(t *testing.T, token string) retroNetDirectoryResponse {
	t.Helper()
	rec := e.serve(t, http.MethodGet, "/api/desktop/retronet/directory", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("directory status=%d body=%s", rec.Code, rec.Body.String())
	}
	var directory retroNetDirectoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &directory); err != nil {
		t.Fatalf("decode directory: %v", err)
	}
	return directory
}

func (e *retroNetTestEnv) saveEntries(t *testing.T, entries ...string) {
	t.Helper()
	svc, _, err := e.s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}
	document := `{"version":1,"entries":[` + strings.Join(entries, ",") + `]}`
	if err := svc.SetSetting(context.Background(), retronet.EntriesSetting, document, desktop.SourceUser); err != nil {
		t.Fatalf("store own entries: %v", err)
	}
}

// ownTelnetEntry is a world/utf8 Telnet entry whose host resolves (through
// retroNetTestNetwork) to the fake service on loopback.
func (e *retroNetTestEnv) ownTelnetEntry() string {
	return fmt.Sprintf(`{"id":%q,"name":"Fake BBS","protocol":"telnet","host":"bbs.retronet.test","port":%d,"kind":"world","charset":"utf8"}`, retroNetTestEntryID, e.telnet.port())
}

// retroNetTestNetwork resolves every host to loopback and connects only to the
// fake Telnet port; every other port fails at once like a refused connection,
// so status probes of the catalog never leave the machine.
type retroNetTestNetwork struct{ port int }

func (n retroNetTestNetwork) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, nil
}

func (n retroNetTestNetwork) dial(ctx context.Context, network, address string) (net.Conn, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil || port != strconv.Itoa(n.port) {
		return nil, errors.New("retronet test network: connection refused")
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
}

func (n retroNetTestNetwork) dialer() retronet.Dialer {
	return retronet.Dialer{Resolver: n, Dial: n.dial, Timeout: 2 * time.Second, AllowRestricted: true}
}

// retroNetFakeTelnet asks for NAWS, greets, and records every byte it receives.
type retroNetFakeTelnet struct {
	listener net.Listener
	mu       sync.Mutex
	conns    []net.Conn
	accepted int
	input    []byte
}

func startRetroNetFakeTelnet(t *testing.T) *retroNetFakeTelnet {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fake := &retroNetFakeTelnet{listener: listener}
	go fake.serve()
	t.Cleanup(fake.close)
	return fake
}

func (f *retroNetFakeTelnet) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns = append(f.conns, conn)
		f.accepted++
		f.mu.Unlock()
		go f.handle(conn)
	}
}

func (f *retroNetFakeTelnet) handle(conn net.Conn) {
	_, _ = conn.Write([]byte{255, 253, 31}) // IAC DO NAWS
	_, _ = conn.Write([]byte("HELLO RETRO\r\n"))
	buffer := make([]byte, 4096)
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			f.mu.Lock()
			f.input = append(f.input, buffer[:n]...)
			f.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (f *retroNetFakeTelnet) port() int { return f.listener.Addr().(*net.TCPAddr).Port }

func (f *retroNetFakeTelnet) acceptedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accepted
}

// hangUp closes every accepted connection, like a service hanging up.
func (f *retroNetFakeTelnet) hangUp() {
	f.mu.Lock()
	conns := f.conns
	f.conns = nil
	f.mu.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (f *retroNetFakeTelnet) close() {
	_ = f.listener.Close()
	f.hangUp()
}

func (f *retroNetFakeTelnet) waitInput(t *testing.T, want []byte) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		found := bytes.Contains(f.input, want)
		seen := append([]byte(nil), f.input...)
		f.mu.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake Telnet service never received %q; got %q", want, seen)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// retroNetNAWS is IAC SB NAWS <cols> <rows> IAC SE (no 255 bytes in the test sizes).
func retroNetNAWS(cols, rows int) []byte {
	return []byte{255, 250, 31, byte(cols >> 8), byte(cols), byte(rows >> 8), byte(rows), 255, 240}
}

func TestDesktopRetroNetRoutesRequireWriteScopeAndGrant(t *testing.T) {
	env := newRetroNetTestEnv(t)
	const directory, status = "/api/desktop/retronet/directory", "/api/desktop/retronet/status"
	for _, tc := range []struct {
		name, method, path, token string
		readonly, disabled        bool
		status                    int
		code                      string
	}{
		{"anonymous", http.MethodGet, directory, "", false, false, http.StatusUnauthorized, "unauthorized"},
		{"read token", http.MethodGet, directory, env.readToken, false, false, http.StatusForbidden, "desktop_scope_required"},
		{"read token status", http.MethodPost, status, env.readToken, false, false, http.StatusForbidden, "desktop_scope_required"},
		{"write token", http.MethodGet, directory, env.writeToken, false, false, http.StatusOK, ""},
		{"grant off", http.MethodGet, directory, env.adminToken, false, true, http.StatusForbidden, "retronet_disabled"},
		{"grant off status", http.MethodPost, status, env.adminToken, false, true, http.StatusForbidden, "retronet_disabled"},
		{"readonly directory", http.MethodGet, directory, env.adminToken, true, false, http.StatusForbidden, "retronet_disabled"},
		{"readonly status", http.MethodPost, status, env.adminToken, true, false, http.StatusForbidden, "desktop_readonly"},
		{"directory method", http.MethodPost, directory, env.adminToken, false, false, http.StatusMethodNotAllowed, ""},
		{"status method", http.MethodGet, status, env.adminToken, false, false, http.StatusMethodNotAllowed, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env.configure(func(vd *config.VirtualDesktopConfig) {
				vd.ReadOnly, vd.RetroNetEnabled = tc.readonly, !tc.disabled
			})
			rec := env.serve(t, tc.method, tc.path, tc.token, "")
			if rec.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.code != "" && !strings.Contains(rec.Body.String(), `"error":"`+tc.code+`"`) {
				t.Fatalf("body %s lacks error %q", rec.Body.String(), tc.code)
			}
		})
	}
}

func TestDesktopRetroNetDirectoryListsCatalogThenOwnEntries(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	catalog := retronet.DefaultCatalog()
	for _, tc := range []struct {
		name, token string
		canEdit     bool
	}{
		{"admin", env.adminToken, true},
		{"write token", env.writeToken, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.serve(t, http.MethodGet, "/api/desktop/retronet/directory", tc.token, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"entries", "status", "stale", "can_edit"} {
				if _, ok := raw[key]; !ok {
					t.Fatalf("directory JSON lacks %q: %s", key, rec.Body.String())
				}
			}
			if len(raw) != 4 {
				t.Fatalf("directory JSON has unexpected keys: %s", rec.Body.String())
			}
			directory := env.directory(t, tc.token)
			if len(directory.Entries) != len(catalog)+1 {
				t.Fatalf("entries = %d, want %d catalog + 1 own", len(directory.Entries), len(catalog))
			}
			first := directory.Entries[0]
			if first.ID != catalog[0].ID || first.Own || first.DescriptionKey == "" {
				t.Fatalf("first entry = %+v, want the first catalog entry", first)
			}
			last := directory.Entries[len(directory.Entries)-1]
			if last.ID != retroNetTestEntryID || !last.Own || last.Category != retronet.CategoryOwn {
				t.Fatalf("last entry = %+v, want the own entry", last)
			}
			if len(directory.Status) != len(directory.Entries) {
				t.Fatalf("status has %d entries, want %d", len(directory.Status), len(directory.Entries))
			}
			switch directory.Status[last.ID].State {
			case "online", "offline", "unknown":
			default:
				t.Fatalf("own status = %+v", directory.Status[last.ID])
			}
			if directory.CanEdit != tc.canEdit {
				t.Fatalf("can_edit = %v, want %v", directory.CanEdit, tc.canEdit)
			}
		})
	}
}

func TestDesktopRetroNetStatusRefreshUsesTheGuardedDialer(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	rec := env.serve(t, http.MethodPost, "/api/desktop/retronet/status", env.writeToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status map[string]retronet.Status `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Status[retroNetTestEntryID].State; got != "online" {
		t.Fatalf("own entry state = %q, want online (%s)", got, rec.Body.String())
	}
	if got := body.Status["telehack"].State; got != "offline" {
		t.Fatalf("telehack state = %q, want offline through the refusing test network", got)
	}
}

func TestDesktopRetroNetOwnEntriesSaveThroughDesktopSettings(t *testing.T) {
	env := newRetroNetTestEnv(t)
	put := func(token, value string) *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]string{"key": retronet.EntriesSetting, "value": value})
		if err != nil {
			t.Fatal(err)
		}
		return env.serve(t, http.MethodPut, "/api/desktop/settings", token, string(body))
	}
	valid := `{"version":1,"entries":[` + env.ownTelnetEntry() + `]}`
	if rec := put(env.writeToken, valid); rec.Code != http.StatusForbidden {
		t.Fatalf("write token saved own entries: %d %s", rec.Code, rec.Body.String())
	}
	if rec := put(env.adminToken, valid); rec.Code != http.StatusOK {
		t.Fatalf("valid own entries rejected: %d %s", rec.Code, rec.Body.String())
	}
	for _, host := range []string{"192.168.1.10", "127.0.0.1", "10.0.0.5"} {
		rec := put(env.adminToken, strings.Replace(valid, "bbs.retronet.test", host, 1))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid desktop setting value for retronet.entries") {
			t.Fatalf("private literal %s was not rejected: %d %s", host, rec.Code, rec.Body.String())
		}
	}
	entries := env.directory(t, env.adminToken).Entries
	if last := entries[len(entries)-1]; last.ID != retroNetTestEntryID || last.Host != "bbs.retronet.test" {
		t.Fatalf("directory after saves = %+v", last)
	}
}

func TestDesktopRetroNetHostKeyCallbackPersistsFirstContactKey(t *testing.T) {
	var production Server
	manager, prober := production.retroNet()
	if manager == nil || prober == nil || manager.OnHostKeyAccepted == nil || manager.Dialer.AllowRestricted || prober.Dialer.AllowRestricted {
		t.Fatal("production Retro-Net wiring must use the guarded dialer and persist host keys")
	}

	env := newRetroNetTestEnv(t)
	env.saveEntries(t, `{"id":"own-sshgame01","name":"SSH Game","protocol":"ssh","host":"game.retronet.test","port":2222,"user":"guest"}`)
	manager, prober = env.s.retroNet()
	if manager != env.s.retroNetManager || prober != env.s.retroNetStatus {
		t.Fatal("the lazy initializer replaced the pre-seeded test engine")
	}
	if manager.OnHostKeyAccepted == nil {
		t.Fatal("first-contact host keys are not wired to the desktop service")
	}
	fingerprint := "SHA256:" + strings.Repeat("Q", 43)
	for _, unmarked := range []context.Context{context.Background(), withRetroNetHostKeyPersistence(context.Background(), false)} {
		if err := manager.OnHostKeyAccepted(unmarked, "own-sshgame01", fingerprint); err != nil {
			t.Fatalf("a skipped store reported %v", err)
		}
	}
	if entries := env.directory(t, env.adminToken).Entries; entries[len(entries)-1].HostKey != "" {
		t.Fatal("a session without the admin mark pinned a host key")
	}
	if err := manager.OnHostKeyAccepted(withRetroNetHostKeyPersistence(context.Background(), true), "own-sshgame01", fingerprint); err != nil {
		t.Fatalf("store host key: %v", err)
	}
	entries := env.directory(t, env.adminToken).Entries
	if last := entries[len(entries)-1]; last.ID != "own-sshgame01" || last.HostKey != fingerprint {
		t.Fatalf("own SSH entry after first contact = %+v", last)
	}
	if err := manager.OnHostKeyAccepted(withRetroNetHostKeyPersistence(context.Background(), true), "own-sshgame01", "SHA256:"+strings.Repeat("R", 43)); err == nil {
		t.Fatal("a stored host key was overwritten")
	}
}

// retroNetLogBuffer is a concurrency-safe log sink for slog.
type retroNetLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *retroNetLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *retroNetLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestDesktopRetroNetHostKeyStoreLogsFailuresAndAnnouncesSettings(t *testing.T) {
	logs := &retroNetLogBuffer{}
	env := newRetroNetTestEnv(t, func(s *Server) { s.Logger = slog.New(slog.NewTextHandler(logs, nil)) })
	env.saveEntries(t, `{"id":"own-sshgame01","name":"SSH Game","protocol":"ssh","host":"game.retronet.test","port":2222,"user":"guest"}`)
	_, hub, err := env.s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	events, unsubscribe, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	manager, _ := env.s.retroNet()

	fingerprint := "SHA256:" + strings.Repeat("Q", 43)
	if err := manager.OnHostKeyAccepted(withRetroNetHostKeyPersistence(context.Background(), true), "own-sshgame01", fingerprint); err != nil {
		t.Fatalf("store host key: %v", err)
	}
	select {
	case event := <-events:
		payload, _ := event.Payload.(map[string]interface{})
		settings, _ := payload["settings"].(map[string]string)
		if event.Type != "desktop_changed" || payload["operation"] != "set_settings" || !strings.Contains(settings[retronet.EntriesSetting], fingerprint) {
			t.Fatalf("host-key event = %+v, want desktop_changed/set_settings with the stored entries", event)
		}
		writer := httptest.NewRequest(http.MethodGet, "/api/desktop/ws", nil)
		writer.Header.Set("Authorization", "Bearer "+env.writeToken)
		if _, delivered := filterDesktopEvent(env.s, writer, event); delivered {
			t.Fatal("the entries event reaches non-admin clients")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("storing a host key announced no settings change")
	}
	if strings.Contains(logs.String(), "own-sshgame01") {
		t.Fatalf("a successful store was logged as a failure: %s", logs.String())
	}

	rejected := "SHA256:" + strings.Repeat("R", 43)
	if err := manager.OnHostKeyAccepted(withRetroNetHostKeyPersistence(context.Background(), true), "own-sshgame01", rejected); err == nil {
		t.Fatal("a stored host key was overwritten")
	}
	logged := logs.String()
	if !strings.Contains(logged, "own-sshgame01") || !strings.Contains(logged, "already has a host key") {
		t.Fatalf("the failed store was not logged with entry and error: %q", logged)
	}
	if strings.Contains(logged, rejected) || strings.Contains(logged, fingerprint) {
		t.Fatalf("the failure log carries a fingerprint: %q", logged)
	}
	select {
	case event := <-events:
		t.Fatalf("a failed store announced %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}
