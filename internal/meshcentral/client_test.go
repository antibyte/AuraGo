package meshcentral

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

// testLogger returns a silent logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ── NewClient ───────────────────────────────────────────────────────────────

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty URL", "", true},
		{"no scheme", "mesh.example.com", true},
		{"ftp scheme", "ftp://mesh.example.com", true},
		{"file scheme", "file:///etc/passwd", true},
		{"http URL", "http://mesh.example.com", false},
		{"https URL", "https://mesh.example.com", false},
		{"URL with path", "https://mesh.example.com/path", false},
		{"URL with query", "https://mesh.example.com?foo=bar", false},
		{"URL with trailing slash", "https://mesh.example.com/", false},
		{"URL with port", "https://mesh.example.com:8443", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewClient(tt.url, "admin", "pass", "", false)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.url, err)
			}
		})
	}
}

func TestNewClientStoresAuthParams(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		loginToken string
		insecure   bool
	}{
		{"username password", "admin", "secret", "", false},
		{"login token", "", "tokenpass", "automation", false},
		{"login token with prefix", "", "tokenpass", "~t:automation", false},
		{"insecure flag", "admin", "pass", "", true},
		{"all fields empty", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := NewClient("https://mesh.example.com", tt.username, tt.password, tt.loginToken, tt.insecure)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if c.username != tt.username {
				t.Errorf("username = %q, want %q", c.username, tt.username)
			}
			if c.password != tt.password {
				t.Errorf("password = %q, want %q", c.password, tt.password)
			}
			if c.loginToken != tt.loginToken {
				t.Errorf("loginToken = %q, want %q", c.loginToken, tt.loginToken)
			}
			if c.insecure != tt.insecure {
				t.Errorf("insecure = %v, want %v", c.insecure, tt.insecure)
			}
			if c.pendingReqs == nil {
				t.Error("pendingReqs should be initialized")
			}
			if c.done == nil {
				t.Error("done channel should be initialized")
			}
		})
	}
}

func TestSetLogger(t *testing.T) {
	c, err := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	logger := testLogger()
	c.SetLogger(logger)
	if c.logger != logger {
		t.Error("logger was not set")
	}
}

// ── WebSocket helpers ───────────────────────────────────────────────────────

// wsPair creates a connected WebSocket pair (server + client) using httptest.
func wsPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	serverConn := make(chan *websocket.Conn, 1)

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConn <- ws
	}))

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	server := <-serverConn

	return server, client, func() {
		server.Close()
		client.Close()
		srv.Close()
	}
}

// clientWithWS returns a Client with an active WebSocket connection and running pumps.
func clientWithWS(t *testing.T) (*Client, *websocket.Conn, func()) {
	t.Helper()
	c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	c.SetLogger(testLogger())

	sConn, cConn, cleanup := wsPair(t)

	c.wsMu.Lock()
	c.ws = cConn
	c.wsMu.Unlock()

	go c.readPump()
	go c.pingPump()

	return c, sConn, func() {
		c.Close()
		cleanup()
	}
}

// ── Send ────────────────────────────────────────────────────────────────────

func TestSend_NotConnected(t *testing.T) {
	c, err := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetLogger(testLogger())

	_, err = c.Send(map[string]interface{}{"action": "test"})
	if err == nil {
		t.Fatal("expected error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("error should mention 'not connected', got: %v", err)
	}
}

func TestSend_ReqIDIncrement(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	// Start a goroutine that reads and discards messages so WriteJSON doesn't block.
	go func() {
		for {
			_, _, err := sConn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	reqid1, err := c.Send(map[string]interface{}{"action": "a1"})
	if err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	reqid2, err := c.Send(map[string]interface{}{"action": "a2"})
	if err != nil {
		t.Fatalf("Send 2: %v", err)
	}
	if reqid2 != reqid1+1 {
		t.Fatalf("reqid2 = %d, want %d", reqid2, reqid1+1)
	}
}

func TestSend_InjectsReqIDIntoCommand(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	msgCh := make(chan []byte, 1)
	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			select {
			case msgCh <- msg:
			default:
			}
		}
	}()

	cmd := map[string]interface{}{"action": "serverinfo", "extra": 42}
	reqid, err := c.Send(cmd)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case msg := <-msgCh:
		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if data["action"] != "serverinfo" {
			t.Errorf("action = %v, want serverinfo", data["action"])
		}
		if data["extra"] != float64(42) {
			t.Errorf("extra = %v, want 42", data["extra"])
		}
		gotReqid, ok := data["reqid"].(float64)
		if !ok {
			t.Fatalf("reqid missing or wrong type")
		}
		if int(gotReqid) != reqid {
			t.Errorf("reqid = %d, want %d", int(gotReqid), reqid)
		}
		// Verify the original map was mutated (Send adds reqid in-place)
		if cmd["reqid"] != reqid {
			t.Errorf("cmd reqid = %v, want %d", cmd["reqid"], reqid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

// ── WaitForReq ──────────────────────────────────────────────────────────────

func TestWaitForReq_Timeout(t *testing.T) {
	c, _, cleanup := clientWithWS(t)
	defer cleanup()

	_, err := c.WaitForReq(1, "test", 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error should mention timeout, got: %v", err)
	}

	// Verify pending request was cleaned up
	c.reqsMu.RLock()
	_, exists := c.pendingReqs[1]
	c.reqsMu.RUnlock()
	if exists {
		t.Error("pending request should be cleaned up after timeout")
	}
}

func TestWaitForReq_ResponseDelivery(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	// Server responds to any message with a matching reqid
	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			rid, ok := data["reqid"].(float64)
			if !ok {
				continue
			}
			resp := map[string]interface{}{
				"reqid":  int(rid),
				"action": "test",
				"result": "ok",
			}
			sConn.WriteJSON(resp)
		}
	}()

	reqid, err := c.Send(map[string]interface{}{"action": "test"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	res, err := c.WaitForReq(reqid, "test", 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForReq: %v", err)
	}
	if res["result"] != "ok" {
		t.Errorf("result = %v, want ok", res["result"])
	}

	// Verify pending request was cleaned up
	c.reqsMu.RLock()
	_, exists := c.pendingReqs[reqid]
	c.reqsMu.RUnlock()
	if exists {
		t.Error("pending request should be cleaned up after response")
	}
}

func TestWaitForReq_ClientClosed(t *testing.T) {
	c, _, cleanup := clientWithWS(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(1)
	var waitErr error
	go func() {
		defer wg.Done()
		_, waitErr = c.WaitForReq(99, "test", 5*time.Second)
	}()

	// Give WaitForReq time to register
	time.Sleep(50 * time.Millisecond)
	c.Close()

	wg.Wait()
	if waitErr == nil {
		t.Fatal("expected error after client closed")
	}
	if !strings.Contains(waitErr.Error(), "disconnected") && !strings.Contains(waitErr.Error(), "client closed") {
		t.Fatalf("error should mention disconnect, got: %v", waitErr)
	}
}

func TestWaitForReq_Concurrent(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	// Server echoes back reqid
	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			rid, ok := data["reqid"].(float64)
			if !ok {
				continue
			}
			resp := map[string]interface{}{
				"reqid": int(rid),
				"val":   int(rid) * 10,
			}
			sConn.WriteJSON(resp)
		}
	}()

	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reqid, err := c.Send(map[string]interface{}{"action": "test", "id": id})
			if err != nil {
				t.Errorf("Send %d: %v", id, err)
				return
			}
			res, err := c.WaitForReq(reqid, "test", 2*time.Second)
			if err != nil {
				t.Errorf("WaitForReq %d: %v", id, err)
				return
			}
			if res["val"] != float64(reqid*10) {
				t.Errorf("val = %v, want %d", res["val"], reqid*10)
			}
		}(i)
	}
	wg.Wait()
}

// ── WaitForAction ───────────────────────────────────────────────────────────

func TestWaitForAction(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			rid, ok := data["reqid"].(float64)
			if !ok {
				continue
			}
			resp := map[string]interface{}{
				"reqid":  int(rid),
				"action": "serverinfo",
				"info":   "test",
			}
			sConn.WriteJSON(resp)
		}
	}()

	res, err := c.WaitForAction("serverinfo", 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForAction: %v", err)
	}
	if res["info"] != "test" {
		t.Errorf("info = %v, want test", res["info"])
	}
}

// ── Close ───────────────────────────────────────────────────────────────────

func TestClose_Idempotent(t *testing.T) {
	c, _, cleanup := clientWithWS(t)
	defer cleanup()

	c.Close()
	// Second close should not panic
	c.Close()
	c.Close()
}

func TestClose_CleansUpPendingRequests(t *testing.T) {
	c, _, cleanup := clientWithWS(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(1)
	var waitErr error
	go func() {
		defer wg.Done()
		_, waitErr = c.WaitForReq(1, "test", 5*time.Second)
	}()

	time.Sleep(50 * time.Millisecond)
	c.Close()
	wg.Wait()

	if waitErr == nil {
		t.Fatal("expected error after close")
	}
}

func TestClose_SetsWsToNil(t *testing.T) {
	c, _, cleanup := clientWithWS(t)
	defer cleanup()

	if !c.IsConnected() {
		t.Fatal("should be connected before close")
	}
	c.Close()
	if c.IsConnected() {
		t.Error("should not be connected after close")
	}
}

// ── IsConnected ─────────────────────────────────────────────────────────────

func TestIsConnected(t *testing.T) {
	c, err := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.IsConnected() {
		t.Error("new client should not be connected")
	}

	_, cConn, cleanup := wsPair(t)
	defer cleanup()

	c.wsMu.Lock()
	c.ws = cConn
	c.wsMu.Unlock()

	if !c.IsConnected() {
		t.Error("client should be connected after setting ws")
	}
}

// ── addAuthCookies ──────────────────────────────────────────────────────────

func TestAddAuthCookies(t *testing.T) {
	tests := []struct {
		name       string
		cookies    []*http.Cookie
		sessionID  string
		wantCookie string
	}{
		{
			name:       "auth cookies",
			cookies:    []*http.Cookie{{Name: "meshcom", Value: "abc123"}},
			wantCookie: "meshcom=abc123",
		},
		{
			name:       "multiple cookies",
			cookies:    []*http.Cookie{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}},
			wantCookie: "a=1; b=2",
		},
		{
			name:       "sessionID fallback",
			sessionID:  "sess456",
			wantCookie: "meshcom=sess456",
		},
		{
			name:       "empty cookies and sessionID",
			cookies:    []*http.Cookie{},
			wantCookie: "",
		},
		{
			name:       "nil cookie skipped",
			cookies:    []*http.Cookie{nil, {Name: "x", Value: "y"}},
			wantCookie: "x=y",
		},
		{
			name:       "cookie with empty name skipped",
			cookies:    []*http.Cookie{{Name: "", Value: "skip"}, {Name: "ok", Value: "yes"}},
			wantCookie: "ok=yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
			c.authCookies = tt.cookies
			c.sessionID = tt.sessionID

			h := make(http.Header)
			c.addAuthCookies(h)

			got := h.Get("Cookie")
			if got != tt.wantCookie {
				t.Errorf("Cookie = %q, want %q", got, tt.wantCookie)
			}
		})
	}
}

// ── High-level API (mock server) ────────────────────────────────────────────

// mockMeshServer creates a test server that mimics MeshCentral login + control.ashx.
func mockMeshServer(t *testing.T, wsHandler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html random="nonce123"></html>`))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "meshcom", Value: "session123"})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/control.ashx", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer ws.Close()
		if wsHandler != nil {
			wsHandler(ws)
		}
	})

	return testutil.NewHTTPServer(t, mux)
}

func TestConnect_Success(t *testing.T) {
	srv := mockMeshServer(t, func(ws *websocket.Conn) {
		_ = ws.WriteJSON(map[string]interface{}{
			"action": "serverinfo",
			"serverinfo": map[string]interface{}{
				"serverVersion": "1.2.3",
				"domain":        "",
			},
		})
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetLogger(testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !c.IsConnected() {
		t.Error("should be connected")
	}
	if c.sessionID != "session123" {
		t.Errorf("sessionID = %q, want session123", c.sessionID)
	}
	c.Close()
}

func TestConnectContextHonorsCanceledContext(t *testing.T) {
	srv := mockMeshServer(t, func(ws *websocket.Conn) {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			if data["action"] == "serverinfo" {
				rid, _ := data["reqid"].(float64)
				ws.WriteJSON(map[string]interface{}{
					"reqid":         int(rid),
					"action":        "serverinfo",
					"serverVersion": "1.2.3",
				})
			}
		}
	})
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetLogger(testLogger())
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = c.ConnectContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ConnectContext error = %v, want context.Canceled", err)
	}
	if c.IsConnected() {
		t.Fatal("client should not connect when context is already canceled")
	}
}

func TestConnect_WebSocketHandshakeFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "meshcom", Value: "session123"})
		w.WriteHeader(http.StatusOK)
	})
	// No /control.ashx handler, so WS handshake returns 404
	srv := testutil.NewHTTPServer(t, mux)
	defer srv.Close()

	c, err := NewClient(srv.URL, "admin", "pass", "", true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetLogger(testLogger())

	err = c.Connect()
	if err == nil {
		t.Fatal("expected error when WebSocket endpoint missing")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error should mention 404, got: %v", err)
	}
}

func TestConnect_TokenAuth(t *testing.T) {
	var gotUsername string

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			ct := r.Header.Get("Content-Type")
			if strings.Contains(ct, "form-urlencoded") {
				r.ParseForm()
				gotUsername = r.FormValue("username")
			}
			http.SetCookie(w, &http.Cookie{Name: "meshcom", Value: "session123"})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html random="nonce123"></html>`))
	})
	mux.HandleFunc("/control.ashx", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		_ = ws.WriteJSON(map[string]interface{}{
			"action": "serverinfo",
			"serverinfo": map[string]interface{}{
				"serverVersion": "1.0.0",
				"domain":        "",
			},
		})
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			if data["action"] == "serverinfo" {
				rid, _ := data["reqid"].(float64)
				ws.WriteJSON(map[string]interface{}{
					"reqid":         int(rid),
					"action":        "serverinfo",
					"serverVersion": "1.0.0",
				})
			}
		}
	})
	tokenSrv := testutil.NewHTTPServer(t, mux)
	defer tokenSrv.Close()

	c, err := NewClient(tokenSrv.URL, "", "tokenpass", "automation", true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetLogger(testLogger())

	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if gotUsername != "~t:automation" {
		t.Errorf("token username = %q, want ~t:automation", gotUsername)
	}
	c.Close()
}

func TestListDeviceGroups(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "meshes" {
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "meshes",
					"meshes": []interface{}{
						map[string]interface{}{"name": "group1"},
					},
				})
			}
		}
	}()

	groups, err := c.ListDeviceGroups()
	if err != nil {
		t.Fatalf("ListDeviceGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
}

func TestListDeviceGroupsHandlesActionOnlyResponse(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan error, 1)
	go func() {
		groups, err := c.ListDeviceGroups()
		if err != nil {
			resultCh <- err
			return
		}
		if len(groups) != 1 {
			resultCh <- errors.New("expected one group")
			return
		}
		resultCh <- nil
	}()

	_, msg, err := sConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if data["action"] != "meshes" {
		t.Fatalf("action = %v, want meshes", data["action"])
	}
	if data["responseid"] == "" {
		t.Fatalf("responseid missing from meshes request: %v", data)
	}

	_ = sConn.WriteJSON(map[string]interface{}{
		"action": "meshes",
		"meshes": []interface{}{
			map[string]interface{}{"name": "group1"},
		},
	})

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("ListDeviceGroups: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for action-only meshes response")
	}
}

func TestListDeviceGroups_InvalidResponse(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "meshes" {
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "meshes",
					// missing "meshes" key
				})
			}
		}
	}()

	_, err := c.ListDeviceGroups()
	if err == nil {
		t.Fatal("expected error for invalid response")
	}
}

func TestListDevices(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "nodes" {
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "nodes",
					"nodes": []interface{}{
						map[string]interface{}{"name": "dev1"},
					},
				})
			}
		}
	}()

	devices, err := c.ListDevices("")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1", len(devices))
	}
}

func TestListDevices_WithMeshID(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	var gotMeshID string
	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "nodes" {
				gotMeshID, _ = data["meshid"].(string)
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "nodes",
					"nodes": []interface{}{
						map[string]interface{}{"name": "dev1"},
					},
				})
			}
		}
	}()

	_, err := c.ListDevices("mesh://test")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if gotMeshID != "mesh://test" {
		t.Errorf("meshid = %q, want mesh://test", gotMeshID)
	}
}

func TestListDevices_NodesMap(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "nodes" {
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "nodes",
					"nodes": map[string]interface{}{
						"mesh1": []interface{}{
							map[string]interface{}{"name": "a"},
							map[string]interface{}{"name": "b"},
						},
						"mesh2": []interface{}{
							map[string]interface{}{"name": "c"},
						},
					},
				})
			}
		}
	}()

	devices, err := c.ListDevices("")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 3 {
		t.Fatalf("len(devices) = %d, want 3", len(devices))
	}
}

func TestListDevicesHandlesResponseIDResponse(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan error, 1)
	go func() {
		devices, err := c.ListDevices("mesh//group1")
		if err != nil {
			resultCh <- err
			return
		}
		if len(devices) != 1 {
			resultCh <- errors.New("expected one device")
			return
		}
		resultCh <- nil
	}()

	_, msg, err := sConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	responseID, _ := data["responseid"].(string)
	if data["action"] != "nodes" || responseID == "" {
		t.Fatalf("nodes request missing action/responseid: %v", data)
	}
	if data["meshid"] != "mesh//group1" {
		t.Fatalf("meshid = %v, want mesh//group1", data["meshid"])
	}

	_ = sConn.WriteJSON(map[string]interface{}{
		"action":     "nodes",
		"responseid": responseID,
		"nodes": map[string]interface{}{
			"mesh//group1": []interface{}{
				map[string]interface{}{"_id": "node//dev1", "name": "dev1"},
			},
		},
	})

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("ListDevices: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for responseid nodes response")
	}
}

func TestServerInfoReceivesUnsolicitedServerInfo(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan struct {
		info map[string]interface{}
		err  error
	}, 1)
	go func() {
		info, err := c.ServerInfo()
		resultCh <- struct {
			info map[string]interface{}
			err  error
		}{info: info, err: err}
	}()

	_ = sConn.WriteJSON(map[string]interface{}{
		"action": "serverinfo",
		"serverinfo": map[string]interface{}{
			"serverVersion": "2.0.0",
			"domain":        "",
		},
	})

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("ServerInfo: %v", res.err)
		}
		if res.info["serverVersion"] != "2.0.0" {
			t.Fatalf("serverVersion = %v, want 2.0.0", res.info["serverVersion"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for unsolicited serverinfo")
	}
}

func TestListEventsSendsEventsRequest(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan error, 1)
	go func() {
		events, err := c.ListEvents("", "user//admin", 5)
		if err != nil {
			resultCh <- err
			return
		}
		if len(events) != 1 {
			resultCh <- errors.New("expected one event")
			return
		}
		resultCh <- nil
	}()

	_, msg, err := sConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	responseID, _ := data["responseid"].(string)
	if data["action"] != "events" || responseID == "" {
		t.Fatalf("events request missing action/responseid: %v", data)
	}
	if data["user"] != "user//admin" {
		t.Fatalf("user = %v, want user//admin", data["user"])
	}
	if data["limit"] != float64(5) && data["limit"] != 5 {
		t.Fatalf("limit = %v, want 5", data["limit"])
	}

	_ = sConn.WriteJSON(map[string]interface{}{
		"action":     "events",
		"responseid": responseID,
		"events": []interface{}{
			map[string]interface{}{"action": "nodeconnect"},
		},
	})

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for events response")
	}
}

func TestDeviceInfoCollectsMeshCentralDetailResponses(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan error, 1)
	go func() {
		info, err := c.DeviceInfo("node//dev1")
		if err != nil {
			resultCh <- err
			return
		}
		for _, key := range []string{"nodes", "network", "lastconnect", "sysinfo"} {
			if info[key] == nil {
				resultCh <- errors.New("missing " + key)
				return
			}
		}
		resultCh <- nil
	}()

	seen := map[string]bool{}
	for len(seen) < 4 {
		_, msg, err := sConn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		action, _ := data["action"].(string)
		responseID, _ := data["responseid"].(string)
		seen[action] = true
		switch action {
		case "nodes":
			if data["id"] != "node//dev1" {
				t.Fatalf("nodes id = %v, want node//dev1", data["id"])
			}
			_ = sConn.WriteJSON(map[string]interface{}{
				"action":     "nodes",
				"responseid": responseID,
				"nodes": map[string]interface{}{
					"mesh//group1": []interface{}{
						map[string]interface{}{"_id": "node//dev1", "name": "dev1"},
					},
				},
			})
		case "getnetworkinfo":
			_ = sConn.WriteJSON(map[string]interface{}{"action": "getnetworkinfo", "responseid": responseID, "net": []interface{}{}})
		case "lastconnect":
			_ = sConn.WriteJSON(map[string]interface{}{"action": "lastconnect", "responseid": responseID, "time": float64(123)})
		case "getsysinfo":
			if data["nodeinfo"] != true {
				t.Fatalf("getsysinfo nodeinfo = %v, want true", data["nodeinfo"])
			}
			_ = sConn.WriteJSON(map[string]interface{}{"action": "getsysinfo", "responseid": responseID, "hardware": map[string]interface{}{"cpu": "x"}})
		default:
			t.Fatalf("unexpected device info action %q", action)
		}
	}

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("DeviceInfo: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for device info responses")
	}
}

func TestListDevices_InvalidResponse(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "nodes" {
				rid, _ := data["reqid"].(float64)
				sConn.WriteJSON(map[string]interface{}{
					"reqid":  int(rid),
					"action": "nodes",
					// missing "nodes" key
				})
			}
		}
	}()

	_, err := c.ListDevices("")
	if err == nil {
		t.Fatal("expected error for invalid response")
	}
}

func TestWakeOnLan(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	msg, err := c.WakeOnLan([]string{"node1", "node2"})
	if err != nil {
		t.Fatalf("WakeOnLan: %v", err)
	}
	if msg != "Wake-on-LAN packet sent" {
		t.Errorf("msg = %q, want 'Wake-on-LAN packet sent'", msg)
	}

	_, raw, err := sConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if data["action"] != "wakedevices" {
		t.Fatalf("action = %v, want wakedevices", data["action"])
	}
	gotNodeIDs, _ := data["nodeids"].([]interface{})
	if len(gotNodeIDs) != 2 {
		t.Errorf("nodeids = %v, want 2 items", gotNodeIDs)
	}
}

func TestWakeOnLan_NotConnected(t *testing.T) {
	c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	c.SetLogger(testLogger())
	_, err := c.WakeOnLan([]string{"node1"})
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestPowerAction(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	gotActionTypeCh := make(chan float64, 1)
	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "poweraction" {
				gotActionType, _ := data["actiontype"].(float64)
				gotActionTypeCh <- gotActionType
			}
		}
	}()

	msg, err := c.PowerAction([]string{"node1"}, 4)
	if err != nil {
		t.Fatalf("PowerAction: %v", err)
	}
	if msg != "Power action sent" {
		t.Errorf("msg = %q, want 'Power action sent'", msg)
	}
	select {
	case gotActionType := <-gotActionTypeCh:
		if gotActionType != 4 {
			t.Errorf("actiontype = %v, want 4", gotActionType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for poweraction request")
	}
}

func TestPowerAction_NotConnected(t *testing.T) {
	c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	c.SetLogger(testLogger())
	_, err := c.PowerAction([]string{"node1"}, 3)
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestRunCommand(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	go func() {
		for {
			_, msg, err := sConn.ReadMessage()
			if err != nil {
				return
			}
			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				continue
			}
			action, _ := data["action"].(string)
			if action == "runcommands" {
				responseID, _ := data["responseid"].(string)
				sConn.WriteJSON(map[string]interface{}{
					"action":     "runcommands",
					"type":       "runcommands",
					"responseid": responseID,
					"result":     "OK",
					"output":     "hello",
				})
			}
		}
	}()

	res, err := c.RunCommand("node1", "echo hello")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if res["output"] != "hello" {
		t.Errorf("output = %v, want hello", res["output"])
	}
}

func TestRunCommandSendsMeshCentralRuncommandsPayload(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	resultCh := make(chan error, 1)
	go func() {
		_, err := c.RunCommand("node//dev1", "echo hello")
		resultCh <- err
	}()

	_, msg, err := sConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	responseID, _ := data["responseid"].(string)
	if data["action"] != "runcommands" {
		t.Fatalf("action = %v, want runcommands", data["action"])
	}
	if responseID == "" {
		t.Fatalf("responseid missing from runcommands request: %v", data)
	}
	nodeIDs, _ := data["nodeids"].([]interface{})
	if len(nodeIDs) != 1 || nodeIDs[0] != "node//dev1" {
		t.Fatalf("nodeids = %v, want [node//dev1]", data["nodeids"])
	}
	if data["cmds"] != "echo hello" {
		t.Fatalf("cmds = %v, want echo hello", data["cmds"])
	}

	_ = sConn.WriteJSON(map[string]interface{}{
		"action":     "runcommands",
		"type":       "runcommands",
		"responseid": responseID,
		"result":     "OK",
	})

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("RunCommand: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for runcommands response")
	}
}

func TestRunCommand_NotConnected(t *testing.T) {
	c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	c.SetLogger(testLogger())
	_, err := c.RunCommand("node1", "echo hi")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestShell(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	errCh := make(chan error, 1)
	go func() {
		_, err := c.Shell("node1", "ls")
		errCh <- err
	}()

	msgCh := make(chan []byte, 1)
	go func() {
		_, msg, err := sConn.ReadMessage()
		if err == nil {
			msgCh <- msg
		}
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
			t.Fatalf("Shell error = %v, want unsupported", err)
		}
	case msg := <-msgCh:
		t.Fatalf("Shell should be disabled, but sent WebSocket message: %s", string(msg))
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for Shell unsupported error")
	}
}

func TestShell_NotConnected(t *testing.T) {
	c, _ := NewClient("https://mesh.example.com", "admin", "pass", "", false)
	c.SetLogger(testLogger())
	_, err := c.Shell("node1", "ls")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

// ── readPump routing ────────────────────────────────────────────────────────

func TestReadPump_DeliversByReqID(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	// Pre-register a pending request
	pr := &pendingRequest{
		action: "test",
		ch:     make(chan response, 1),
	}
	c.reqsMu.Lock()
	c.pendingReqs[42] = pr
	c.reqsMu.Unlock()

	// Send response from server side with matching reqid
	sConn.WriteJSON(map[string]interface{}{
		"reqid":  42,
		"action": "test",
		"value":  "matched",
	})

	select {
	case res := <-pr.ch:
		if res.err != nil {
			t.Fatalf("unexpected error: %v", res.err)
		}
		if res.data["value"] != "matched" {
			t.Errorf("value = %v, want matched", res.data["value"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestReadPumpNotifiesPendingRequestsOnDisconnect(t *testing.T) {
	c, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	pr := &pendingRequest{
		action: "test",
		ch:     make(chan response, 1),
	}
	c.reqsMu.Lock()
	c.pendingReqs[7] = pr
	c.reqsMu.Unlock()

	_ = sConn.Close()

	select {
	case res := <-pr.ch:
		if res.err == nil {
			t.Fatal("expected disconnect error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for pending request disconnect notification")
	}
}

func TestReadPump_SkipsNonJSON(t *testing.T) {
	_, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	// Send plain text and binary-like messages that should be skipped
	sConn.WriteMessage(websocket.TextMessage, []byte("not json"))
	sConn.WriteMessage(websocket.TextMessage, []byte("x"))

	// Should not panic; give readPump time to process
	time.Sleep(100 * time.Millisecond)
}

func TestReadPump_SkipsInvalidJSON(t *testing.T) {
	_, sConn, cleanup := clientWithWS(t)
	defer cleanup()

	sConn.WriteMessage(websocket.TextMessage, []byte(`{invalid`))

	// Should not panic
	time.Sleep(100 * time.Millisecond)
}

// ── httpLogin errors ────────────────────────────────────────────────────────

func TestHTTPLogin_NoCookies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := testutil.NewHTTPServer(t, mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL, "admin", "pass", "", true)
	c.SetLogger(testLogger())

	err := c.httpLogin(context.Background(), "admin")
	if err == nil {
		t.Fatal("expected error when no cookies returned")
	}
	if !strings.Contains(err.Error(), "no auth cookies") {
		t.Fatalf("error = %v, want no auth cookies", err)
	}
}

func TestHTTPLogin_BadStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("bad creds"))
	})
	srv := testutil.NewHTTPServer(t, mux)
	defer srv.Close()

	c, _ := NewClient(srv.URL, "admin", "pass", "", true)
	c.SetLogger(testLogger())

	err := c.httpLogin(context.Background(), "admin")
	if err == nil {
		t.Fatal("expected error for bad status")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %v, want 401", err)
	}
}
