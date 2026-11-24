package desktop

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestRDPTicketStoreRateLimit(t *testing.T) {
	store := NewRDPTicketStore()
	now := time.Now()
	store.now = func() time.Time { return now }

	for i := 0; i < rdpTicketRateLimit; i++ {
		if _, err := store.Issue("dev1"); err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
	}
	if _, err := store.Issue("dev1"); !errors.Is(err, ErrRDPTicketRateLimited) {
		t.Fatalf("11th issue must be rate limited, got %v", err)
	}
	if _, err := store.Issue("dev2"); err != nil {
		t.Fatalf("other devices have their own budget: %v", err)
	}
	now = now.Add(rdpTicketRateWindow + time.Second)
	if _, err := store.Issue("dev1"); err != nil {
		t.Fatalf("budget must recover after the window: %v", err)
	}
}

func TestHandleRDPTicketSecretGatingAndAudit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("rdp-pw", "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	if err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "rdp1", Name: "rdp1", Type: "server", Protocol: "rdp",
		IPAddress: "127.0.0.1", Username: `CORP\bob`, VaultSecretID: "rdp-pw", Tags: []string{}}); err != nil {
		t.Fatal(err)
	}

	type auditCall struct {
		action, device string
		details        map[string]any
	}
	var calls []auditCall
	audit := func(_ *http.Request, action, deviceID string, details map[string]any) {
		calls = append(calls, auditCall{action, deviceID, details})
	}
	post := func(allow func(*http.Request) bool, store *RDPTicketStore) (int, map[string]any) {
		rec := httptest.NewRecorder()
		HandleRDPTicket(db, vault, testLogger, store, allow, audit)(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	code, body := post(func(*http.Request) bool { return true }, NewRDPTicketStore())
	if code != http.StatusOK || body["password"] != "s3cret-value" || body["has_password"] != true {
		t.Fatalf("allowed caller must get the password: %d %v", code, body)
	}
	for name, allow := range map[string]func(*http.Request) bool{
		"denied": func(*http.Request) bool { return false },
		"nil":    nil,
	} {
		code, body = post(allow, NewRDPTicketStore())
		if code != http.StatusOK || body["ticket"] == "" || body["password"] != "" || body["has_password"] != false || body["username"] != "bob" {
			t.Fatalf("%s: caller must get a ticket without password: %d %v", name, code, body)
		}
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 audit calls, got %d", len(calls))
	}
	for i, c := range calls {
		if c.action != "desktop_rdp_ticket_issued" || c.device != "rdp1" {
			t.Fatalf("call %d: %+v", i, c)
		}
		raw, _ := json.Marshal(c.details)
		if strings.Contains(string(raw), "s3cret-value") || strings.Contains(strings.ToLower(string(raw)), "\"password\"") {
			t.Fatalf("audit details leak the secret: %s", raw)
		}
	}
	if calls[0].details["secret_released"] != true || calls[1].details["secret_released"] != false {
		t.Fatalf("secret_released flag wrong: %v / %v", calls[0].details, calls[1].details)
	}

	// Rate limit: 429, no audit entry for the refused request.
	store := NewRDPTicketStore()
	for i := 0; i < rdpTicketRateLimit; i++ {
		if code, _ = post(nil, store); code != http.StatusOK {
			t.Fatalf("issue %d: %d", i, code)
		}
	}
	before := len(calls)
	if code, _ = post(nil, store); code != http.StatusTooManyRequests {
		t.Fatalf("excess issue must be 429, got %d", code)
	}
	if len(calls) != before {
		t.Fatal("a refused ticket must not be audited as issued")
	}
}

// rdpConnect performs one RDCleanPath request against the gateway and returns
// the first response PDU.
func rdpConnect(t *testing.T, srv *httptest.Server, store *RDPTicketStore) (rdCleanPathPDU, *websocket.Conn) {
	t.Helper()
	ws := dialRDPGateway(t, srv)
	ticket, err := store.Issue("rdp1")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := rdCleanPathPDU{Version: 3390, ProxyAuth: ticket, X224ConnectionPDU: rdpTestX224Request}.marshal()
	if err := ws.WriteMessage(websocket.BinaryMessage, req); err != nil {
		t.Fatal(err)
	}
	return readRDPPDU(t, ws), ws
}

func TestHandleRDPProxyCertificatePinning(t *testing.T) {
	fake := newFakeRDPServer(t, rdpTestX224Confirm)
	srv, store, db := rdpGatewayServerDB(t, "127.0.0.1", fake.listener.Addr().(*net.TCPAddr).Port)
	sum := sha256.Sum256(fake.leafDER)
	want := hex.EncodeToString(sum[:])

	if _, found, _ := inventory.GetRDPCertPin(db, "rdp1"); found {
		t.Fatal("no pin before the first connection")
	}
	resp, _ := rdpConnect(t, srv, store)
	if resp.Error.ErrorCode != 0 || len(resp.ServerCertChain) != 1 {
		t.Fatalf("first connect must succeed: %+v", resp.Error)
	}
	if pin, found, err := inventory.GetRDPCertPin(db, "rdp1"); err != nil || !found || pin != want {
		t.Fatalf("pin = %q found=%v err=%v, want %q", pin, found, err, want)
	}

	resp, _ = rdpConnect(t, srv, store)
	if resp.Error.ErrorCode != 0 || len(resp.ServerCertChain) != 1 {
		t.Fatalf("same certificate must pass: %+v", resp.Error)
	}

	// A different certificate than the pinned one: no chain, 16384 / 409.
	if err := inventory.DeleteRDPCertPin(db, "rdp1"); err != nil {
		t.Fatal(err)
	}
	if err := inventory.SetRDPCertPin(db, "rdp1", strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	resp, ws := rdpConnect(t, srv, store)
	if resp.Error.ErrorCode != rdCleanPathErrCertChanged || resp.Error.HTTPStatusCode != http.StatusConflict || len(resp.ServerCertChain) != 0 {
		t.Fatalf("changed certificate must give 16384/409 without chain: %+v chain=%d", resp.Error, len(resp.ServerCertChain))
	}
	_ = ws.Close()
	if pin, _, _ := inventory.GetRDPCertPin(db, "rdp1"); pin != strings.Repeat("ab", 32) {
		t.Fatal("a mismatch must not overwrite the pin")
	}

	// Reset: the next connect pins the current certificate again.
	var audited []string
	reset := HandleRDPPinReset(db, testLogger, func(*http.Request) bool { return true }, func(_ *http.Request, action, deviceID string, _ map[string]any) {
		audited = append(audited, action+":"+deviceID)
	})
	rec := httptest.NewRecorder()
	reset(rec, httptest.NewRequest(http.MethodPost, "/x?device_id=rdp1", nil))
	if rec.Code != http.StatusOK || len(audited) != 1 || audited[0] != "desktop_rdp_pin_reset:rdp1" {
		t.Fatalf("reset: %d audited=%v", rec.Code, audited)
	}
	if _, found, _ := inventory.GetRDPCertPin(db, "rdp1"); found {
		t.Fatal("pin must be gone after reset")
	}
	resp, _ = rdpConnect(t, srv, store)
	if resp.Error.ErrorCode != 0 {
		t.Fatalf("connect after reset must succeed: %+v", resp.Error)
	}
	if pin, found, _ := inventory.GetRDPCertPin(db, "rdp1"); !found || pin != want {
		t.Fatalf("connect after reset must pin again, got %q", pin)
	}

	// Deleting the device drops its pin.
	if err := inventory.DeleteDevice(db, "rdp1"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := inventory.GetRDPCertPin(db, "rdp1"); found {
		t.Fatal("device deletion must drop the pin")
	}
}

func TestHandleRDPPinResetGuards(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	for _, d := range []inventory.DeviceRecord{
		{ID: "rdp1", Name: "rdp1", Type: "server", Protocol: "rdp", IPAddress: "127.0.0.1", Tags: []string{}},
		{ID: "ssh1", Name: "ssh1", Type: "server", Protocol: "ssh", IPAddress: "127.0.0.1", Port: 22, Tags: []string{}},
	} {
		if err := inventory.AddDevice(db, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := inventory.SetRDPCertPin(db, "rdp1", "aa"); err != nil {
		t.Fatal(err)
	}
	called := false
	audit := func(*http.Request, string, string, map[string]any) { called = true }
	do := func(method, query string, allow func(*http.Request) bool) int {
		rec := httptest.NewRecorder()
		HandleRDPPinReset(db, testLogger, allow, audit)(rec, httptest.NewRequest(method, "/x"+query, nil))
		return rec.Code
	}
	yes := func(*http.Request) bool { return true }
	if c := do(http.MethodGet, "?device_id=rdp1", yes); c != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", c)
	}
	if c := do(http.MethodPost, "?device_id=rdp1", func(*http.Request) bool { return false }); c != http.StatusForbidden {
		t.Fatalf("denied caller: %d", c)
	}
	if c := do(http.MethodPost, "?device_id=rdp1", nil); c != http.StatusForbidden {
		t.Fatalf("nil allow: %d", c)
	}
	if _, found, _ := inventory.GetRDPCertPin(db, "rdp1"); !found || called {
		t.Fatal("a refused reset must keep the pin and not audit")
	}
	for _, q := range []string{"", "?device_id=ssh1", "?device_id=missing"} {
		if c := do(http.MethodPost, q, yes); c == http.StatusOK {
			t.Fatalf("query %q must be rejected", q)
		}
	}
}

func TestHandleRDPProxyRemoteClosedSendsCloseFrame(t *testing.T) {
	fake := newFakeRDPServer(t, rdpTestX224Confirm)
	srv, store := rdpGatewayServer(t, "127.0.0.1", fake.listener.Addr().(*net.TCPAddr).Port)
	resp, ws := rdpConnect(t, srv, store)
	if resp.Error.ErrorCode != 0 {
		t.Fatalf("connect: %+v", resp.Error)
	}
	// The fake server hangs up when it receives "bye".
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("bye")); err != nil {
		t.Fatal(err)
	}
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			var ce *websocket.CloseError
			if !errors.As(err, &ce) || ce.Code != websocket.CloseNormalClosure || ce.Text != "remote closed" {
				t.Fatalf("want close 1000 'remote closed', got %v", err)
			}
			return
		}
	}
}

func TestRDPTLSClientConfigPinsTLS12(t *testing.T) {
	cfg := rdpTLSClientConfig()
	if cfg.MinVersion != tls.VersionTLS12 || cfg.MaxVersion != tls.VersionTLS12 {
		t.Fatalf("min/max = %x/%x, want TLS 1.2 both", cfg.MinVersion, cfg.MaxVersion)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("RDP hosts are self-signed; verification is replaced by pinning")
	}
}
