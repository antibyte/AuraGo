package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/retronet"

	"github.com/gorilla/websocket"
)

type retroNetTestSocket struct {
	t        *testing.T
	conn     *websocket.Conn
	data     strings.Builder
	controls []retronet.Control
}

func dialRetroNet(t *testing.T, base, token, query string) *retroNetTestSocket {
	t.Helper()
	header := http.Header{"Origin": []string{base}}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/desktop/retronet/connect?"+query, header)
	if err != nil {
		t.Fatalf("dial Retro-Net: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &retroNetTestSocket{t: t, conn: conn}
}

// until reads frames until done() holds, failing after timeout.
func (c *retroNetTestSocket) until(timeout time.Duration, done func() bool) {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for !done() {
		_ = c.conn.SetReadDeadline(deadline)
		kind, payload, err := c.conn.ReadMessage()
		if err != nil {
			c.t.Fatalf("socket read: %v (controls %+v, data %q)", err, c.controls, c.data.String())
		}
		if kind == websocket.BinaryMessage {
			c.data.Write(payload)
			continue
		}
		var control retronet.Control
		if err := json.Unmarshal(payload, &control); err != nil {
			c.t.Fatalf("control frame %q: %v", payload, err)
		}
		c.controls = append(c.controls, control)
	}
}

func (c *retroNetTestSocket) control(kind string) (retronet.Control, bool) {
	for _, control := range c.controls {
		if control.Type == kind {
			return control, true
		}
	}
	return retronet.Control{}, false
}

func (c *retroNetTestSocket) has(kind string) func() bool {
	return func() bool {
		_, ok := c.control(kind)
		return ok
	}
}

func (c *retroNetTestSocket) send(kind int, payload string) {
	c.t.Helper()
	if err := c.conn.WriteMessage(kind, []byte(payload)); err != nil {
		c.t.Fatalf("socket write: %v", err)
	}
}

// expectClosed asserts that the server closes the socket after the result.
func (c *retroNetTestSocket) expectClosed() {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, _, err := c.conn.ReadMessage()
		if err == nil {
			continue
		}
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			c.t.Fatal("the server kept the socket open after the result")
		}
		return
	}
}

func waitRetroNetAudit(t *testing.T, env *retroNetTestEnv, action, target string) (map[string]interface{}, string) {
	t.Helper()
	svc, _, err := env.s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		var raw string
		err := svc.DB().QueryRow(`SELECT details_json FROM desktop_audit WHERE action = ? AND target = ? ORDER BY id DESC LIMIT 1`, action, target).Scan(&raw)
		if err == nil {
			var details map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &details); err != nil {
				t.Fatalf("audit details %q: %v", raw, err)
			}
			return details, raw
		}
		if !errors.Is(err, sql.ErrNoRows) || time.Now().After(deadline) {
			t.Fatalf("no %s audit for %s: %v", action, target, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDesktopRetroNetConnectRejectsBeforeUpgrade(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	const sameOrigin = "http://aurago.test:8088"
	connect := "/api/desktop/retronet/connect?entry=" + retroNetTestEntryID
	for _, tc := range []struct {
		name, path, token, origin   string
		upgrade, readonly, disabled bool
		status                      int
		code                        string
	}{
		{"anonymous", connect, "", sameOrigin, true, false, false, http.StatusUnauthorized, "unauthorized"},
		{"read token", connect, env.readToken, sameOrigin, true, false, false, http.StatusForbidden, "desktop_scope_required"},
		{"readonly", connect, env.adminToken, sameOrigin, true, true, false, http.StatusForbidden, "desktop_readonly"},
		{"grant off", connect, env.adminToken, sameOrigin, true, false, true, http.StatusForbidden, "retronet_disabled"},
		{"unknown entry", "/api/desktop/retronet/connect?entry=own-missing01", env.adminToken, sameOrigin, true, false, false, http.StatusNotFound, ""},
		{"missing entry", "/api/desktop/retronet/connect", env.writeToken, sameOrigin, true, false, false, http.StatusNotFound, ""},
		{"foreign origin", connect, env.writeToken, "https://foreign.example", true, false, false, http.StatusForbidden, ""},
		{"missing origin", connect, env.writeToken, "", true, false, false, http.StatusForbidden, ""},
		{"plain GET", connect, env.writeToken, sameOrigin, false, false, false, http.StatusBadRequest, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env.configure(func(vd *config.VirtualDesktopConfig) {
				vd.ReadOnly, vd.RetroNetEnabled = tc.readonly, !tc.disabled
			})
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.Host = "aurago.test:8088"
			if tc.upgrade {
				r.Header.Set("Connection", "Upgrade")
				r.Header.Set("Upgrade", "websocket")
				r.Header.Set("Sec-Websocket-Version", "13")
				r.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			env.mux.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if w.Header().Get("Upgrade") != "" {
				t.Fatal("the handler upgraded a rejected handshake")
			}
			if tc.code != "" && !strings.Contains(w.Body.String(), `"error":"`+tc.code+`"`) {
				t.Fatalf("body %s lacks error %q", w.Body.String(), tc.code)
			}
		})
	}
	if accepted := env.telnet.acceptedCount(); accepted != 0 {
		t.Fatalf("rejected handshakes reached the service %d times", accepted)
	}
}

func TestDesktopRetroNetConnectBridgesTelnetEndToEnd(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	socket := dialRetroNet(t, env.httpServer.URL, env.writeToken, "entry="+retroNetTestEntryID+"&cols=90&rows=28")
	socket.until(5*time.Second, func() bool {
		_, connected := socket.control("connected")
		return connected && strings.Contains(socket.data.String(), "HELLO RETRO")
	})
	connected, _ := socket.control("connected")
	if connected.Protocol != "telnet" || connected.Kind != "world" || connected.Charset != "utf8" {
		t.Fatalf("connected = %+v", connected)
	}
	if echo, ok := socket.control("echo"); !ok || echo.Remote == nil || *echo.Remote {
		t.Fatalf("initial echo control = %+v, %v; want remote:false", echo, ok)
	}
	env.telnet.waitInput(t, retroNetNAWS(90, 28))
	socket.send(websocket.BinaryMessage, "dir\r")
	env.telnet.waitInput(t, []byte("dir\r\n"))
	socket.send(websocket.TextMessage, `{"type":"resize","cols":100,"rows":30}`)
	env.telnet.waitInput(t, retroNetNAWS(100, 30))
	socket.send(websocket.TextMessage, `{"type":"resize","cols":1000,"rows":1}`)
	env.telnet.waitInput(t, retroNetNAWS(400, 5))
	socket.send(websocket.TextMessage, `{"type":"connect","host":"10.0.0.1","port":23}`)

	env.telnet.hangUp()
	socket.until(5*time.Second, socket.has("result"))
	if result, _ := socket.control("result"); result.Code != retronet.CodeNoCarrier || result.Reason != retronet.ReasonRemoteClosed {
		t.Fatalf("result = %+v, want NO CARRIER/remote_closed", result)
	}
	socket.expectClosed()

	attempt, _ := waitRetroNetAudit(t, env, "desktop_retronet_connect", retroNetTestEntryID)
	if attempt["status"] != "attempt" || attempt["path"] != "/api/desktop/retronet/connect" {
		t.Fatalf("connect audit = %v", attempt)
	}
	session, raw := waitRetroNetAudit(t, env, "desktop_retronet_session", retroNetTestEntryID)
	if session["code"] != retronet.CodeNoCarrier || session["reason"] != retronet.ReasonRemoteClosed {
		t.Fatalf("session audit = %v", session)
	}
	if want := net.JoinHostPort("127.0.0.1", strconv.Itoa(env.telnet.port())); session["target"] != want {
		t.Fatalf("session audit target = %v, want %s", session["target"], want)
	}
	for _, key := range []string{"bytes_in", "bytes_out"} {
		if count, _ := session[key].(float64); count <= 0 {
			t.Fatalf("session audit %s = %v", key, session[key])
		}
	}
	if _, ok := session["duration_ms"].(float64); !ok {
		t.Fatalf("session audit lacks duration_ms: %v", session)
	}
	for _, payload := range []string{"HELLO", "dir"} {
		if strings.Contains(raw, payload) {
			t.Fatalf("session audit leaked payload %q: %s", payload, raw)
		}
	}
}

// A hijacked request keeps its context until the handler returns: the
// adapter's closed Events channel must end the session when the browser leaves.
func TestDesktopRetroNetSessionEndsWhenTheBrowserLeaves(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	socket := dialRetroNet(t, env.httpServer.URL, env.writeToken, "entry="+retroNetTestEntryID)
	socket.until(5*time.Second, socket.has("connected"))
	if active := env.s.retroNetManager.Active(); active != 1 {
		t.Fatalf("active sessions = %d, want 1", active)
	}
	_ = socket.conn.Close()
	session, _ := waitRetroNetAudit(t, env, "desktop_retronet_session", retroNetTestEntryID)
	if session["code"] != retronet.CodeNoCarrier || session["reason"] != retronet.ReasonRemoteClosed {
		t.Fatalf("session audit after the browser left = %v", session)
	}
	if active := env.s.retroNetManager.Active(); active != 0 {
		t.Fatalf("active sessions after the browser left = %d, want 0", active)
	}
}

func TestDesktopRetroNetSessionEndsWhenRevoked(t *testing.T) {
	for _, tc := range []struct{ name, reason string }{
		{"grant off", retronet.ReasonDisabled},
		{"readonly", retronet.ReasonDisabled},
		{"token revoked", retronet.ReasonDisabled},
		{"shutdown", retronet.ReasonServerShutdown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newRetroNetTestEnv(t)
			env.saveEntries(t, env.ownTelnetEntry())
			token, meta, err := env.s.TokenManager.Create("retronet session", []string{desktopScopeWrite}, nil)
			if err != nil {
				t.Fatal(err)
			}
			socket := dialRetroNet(t, env.httpServer.URL, token, "entry="+retroNetTestEntryID)
			socket.until(5*time.Second, socket.has("connected"))
			start := time.Now()
			switch tc.name {
			case "grant off":
				env.configure(func(vd *config.VirtualDesktopConfig) { vd.RetroNetEnabled = false })
			case "readonly":
				env.configure(func(vd *config.VirtualDesktopConfig) { vd.ReadOnly = true })
			case "token revoked":
				if err := env.s.TokenManager.Delete(meta.ID); err != nil {
					t.Fatal(err)
				}
			case "shutdown":
				env.shutdown()
			}
			socket.until(3*time.Second, socket.has("result"))
			if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
				t.Fatalf("the session survived %v after revocation", elapsed)
			}
			result, _ := socket.control("result")
			if result.Code != retronet.CodeNoCarrier || result.Reason != tc.reason {
				t.Fatalf("result = %+v, want NO CARRIER/%s", result, tc.reason)
			}
			socket.expectClosed()
		})
	}
}

func TestDesktopRetroNetLimitAnswersBusy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		maxSessions int
		open        int
	}{
		{"test seam limit of one", 1, 1},
		{"default limit of four", 0, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newRetroNetTestEnv(t, func(s *Server) { s.retroNetManager.MaxSessions = tc.maxSessions })
			env.saveEntries(t, env.ownTelnetEntry())
			var open []*retroNetTestSocket
			for i := 0; i < tc.open; i++ {
				socket := dialRetroNet(t, env.httpServer.URL, env.adminToken, "entry="+retroNetTestEntryID)
				socket.until(5*time.Second, socket.has("connected"))
				open = append(open, socket)
			}
			busy := dialRetroNet(t, env.httpServer.URL, env.adminToken, "entry="+retroNetTestEntryID)
			busy.until(5*time.Second, busy.has("result"))
			if result, _ := busy.control("result"); result.Code != retronet.CodeBusy || result.Reason != retronet.ReasonLimit {
				t.Fatalf("over-limit result = %+v, want BUSY/limit", result)
			}
			if _, ok := busy.control("connected"); ok {
				t.Fatal("an over-limit session reported connected")
			}
			busy.expectClosed()
			open[0].send(websocket.BinaryMessage, "still here\r")
			env.telnet.waitInput(t, []byte("still here\r\n"))
		})
	}
}

func TestDesktopRetroNetSessionClosesOnHTTPDrain(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	tracked := httptest.NewServer(env.s.trackHTTP(env.mux))
	defer tracked.Close()
	socket := dialRetroNet(t, tracked.URL, env.writeToken, "entry="+retroNetTestEntryID)
	socket.until(5*time.Second, socket.has("connected"))
	env.s.beginHTTPDrain()
	socket.expectClosed()
	session, _ := waitRetroNetAudit(t, env, "desktop_retronet_session", retroNetTestEntryID)
	if session["code"] != retronet.CodeNoCarrier {
		t.Fatalf("drained session audit = %v", session)
	}
}

// A desktop config change replaces (and closes) the Desktop service while a
// long session runs; the end record must reach the current service.
func TestDesktopRetroNetSessionAuditUsesTheCurrentDesktopService(t *testing.T) {
	env := newRetroNetTestEnv(t)
	env.saveEntries(t, env.ownTelnetEntry())
	before, _, err := env.s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	socket := dialRetroNet(t, env.httpServer.URL, env.writeToken, "entry="+retroNetTestEntryID)
	socket.until(5*time.Second, socket.has("connected"))
	env.configure(func(vd *config.VirtualDesktopConfig) { vd.MaxFileSizeMB = 77 })
	after, _, err := env.s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatal("the desktop config change did not replace the Desktop service")
	}
	env.telnet.hangUp()
	socket.until(5*time.Second, socket.has("result"))
	session, _ := waitRetroNetAudit(t, env, "desktop_retronet_session", retroNetTestEntryID)
	if session["code"] != retronet.CodeNoCarrier || session["reason"] != retronet.ReasonRemoteClosed {
		t.Fatalf("session audit = %v", session)
	}
}
