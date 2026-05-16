package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

func TestContainerTerminalRejectsDockerDisabledBeforeUpgrade(t *testing.T) {
	s := testContainerServer(false, false)
	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket while Docker was disabled")
	}
}

func TestContainerTerminalRejectsDockerReadOnlyBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, true)
	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket while Docker was read-only")
	}
}

func TestContainerUpdateRejectsDockerReadOnly(t *testing.T) {
	s := testContainerServer(true, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/containers/demo/update", nil)

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestContainerTerminalRejectsStoppedContainerBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: false}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()

	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket for a stopped container")
	}
	if fake.createCalls != 0 {
		t.Fatalf("terminal session was created %d times for stopped container", fake.createCalls)
	}
}

func TestContainerTerminalRejectsCrossOriginBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: true}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()

	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")
	req.Header.Set("Origin", "http://evil.example")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded cross-origin websocket")
	}
	if fake.createCalls != 0 {
		t.Fatalf("terminal session was created %d times for rejected origin", fake.createCalls)
	}
}

func TestContainerTerminalResizeControlCallsBackend(t *testing.T) {
	s := testContainerServer(true, false)
	session := newFakeContainerTerminalSession()
	fake := &fakeContainerTerminalBackend{running: true, session: session}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()

	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/api/containers/demo/terminal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial terminal websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{"type": "resize", "cols": 120, "rows": 32}); err != nil {
		t.Fatalf("write resize control: %v", err)
	}

	select {
	case got := <-session.resizeCalls:
		if got.cols != 120 || got.rows != 32 {
			t.Fatalf("resize = %+v, want cols=120 rows=32", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resize call")
	}
}

func testContainerServer(dockerEnabled, dockerReadOnly bool) *Server {
	cfg := &config.Config{}
	cfg.Docker.Enabled = dockerEnabled
	cfg.Docker.ReadOnly = dockerReadOnly
	return &Server{Cfg: cfg, Logger: slog.Default()}
}

func newContainerTerminalUpgradeRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func replaceContainerTerminalBackend(next containerTerminalBackend) func() {
	old := activeContainerTerminalBackend
	activeContainerTerminalBackend = next
	return func() {
		activeContainerTerminalBackend = old
	}
}

type fakeContainerTerminalBackend struct {
	running     bool
	session     *fakeContainerTerminalSession
	createCalls int
}

func (f *fakeContainerTerminalBackend) ContainerRunning(ctx context.Context, cfg tools.DockerConfig, containerID string) (bool, error) {
	return f.running, nil
}

func (f *fakeContainerTerminalBackend) CreateSession(ctx context.Context, cfg tools.DockerConfig, containerID string, cols, rows int) (containerTerminalSession, error) {
	f.createCalls++
	if f.session == nil {
		f.session = newFakeContainerTerminalSession()
	}
	return f.session, nil
}

type terminalResizeCall struct {
	cols int
	rows int
}

type fakeContainerTerminalSession struct {
	closeOnce   sync.Once
	closed      chan struct{}
	resizeCalls chan terminalResizeCall
}

func newFakeContainerTerminalSession() *fakeContainerTerminalSession {
	return &fakeContainerTerminalSession{
		closed:      make(chan struct{}),
		resizeCalls: make(chan terminalResizeCall, 4),
	}
}

func (s *fakeContainerTerminalSession) Read(p []byte) (int, error) {
	<-s.closed
	return 0, io.EOF
}

func (s *fakeContainerTerminalSession) Write(p []byte) (int, error) {
	return len(p), nil
}

func (s *fakeContainerTerminalSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	return nil
}

func (s *fakeContainerTerminalSession) Resize(ctx context.Context, cols, rows int) error {
	s.resizeCalls <- terminalResizeCall{cols: cols, rows: rows}
	return nil
}
