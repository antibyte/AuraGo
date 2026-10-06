package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

func TestContainerTerminalRejectsPlainGETBeforeCreatingExec(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()
	defer replaceContainerProtection(containerProtection{})()

	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers/demo/terminal", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("plain GET status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if fake.createCalls != 0 {
		t.Fatalf("plain GET created %d exec sessions; a shell would run with nobody attached", fake.createCalls)
	}
}

// TestContainerTerminalRejectsPlainGETBeforeProtectionLookup pins the placement
// of the upgrade check: a non-WebSocket request is answered before the
// protection lookup, which inspects the target and resolves the Docker
// endpoint, so a plain GET causes no Docker request and no DNS lookup.
func TestContainerTerminalRejectsPlainGETBeforeProtectionLookup(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()
	old := containerProtectionFor
	containerProtectionFor = func(context.Context, *Server, tools.DockerConfig, string) containerProtection {
		t.Fatal("a non-WebSocket terminal request must not consult container protection")
		return containerProtection{}
	}
	t.Cleanup(func() { containerProtectionFor = old })

	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers/demo/terminal", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("plain GET status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if fake.createCalls != 0 {
		t.Fatalf("plain GET created %d exec sessions", fake.createCalls)
	}
}

func TestDesktopStoreTerminalRejectsPlainGETBeforeCreatingExec(t *testing.T) {
	svc, _, _ := testInstalledStoreApp(t, "commandcode", 18080)
	s := testDesktopStoreServerWithService(t, svc)
	fake := &fakeContainerTerminalBackend{running: true}
	defer replaceContainerTerminalBackend(fake)()

	rec := httptest.NewRecorder()
	handleDesktopStoreAppRoute(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/store/apps/commandcode/terminal", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("plain GET status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if fake.createCalls != 0 {
		t.Fatalf("plain GET created %d store exec sessions", fake.createCalls)
	}
}

func TestContainerTerminalSendsEOFBeforeClosingSession(t *testing.T) {
	s := testContainerServer(true, false)
	session := newFakeContainerTerminalSession()
	fake := &fakeContainerTerminalBackend{running: true, session: session}
	defer replaceContainerTerminalBackend(fake)()
	defer replaceContainerProtection(containerProtection{})()

	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", nil)
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("ls\r")); err != nil {
		t.Fatalf("write input: %v", err)
	}
	_ = conn.Close()

	select {
	case <-session.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal session was not closed after the WebSocket closed")
	}
	if got := string(session.writtenBeforeClose()); got != "ls\r\x04" {
		t.Fatalf("bytes before close = %q, want the input followed by one EOF (no interrupt)", got)
	}
}

func TestDesktopStoreTerminalClosesWithoutEOF(t *testing.T) {
	svc, _, _ := testInstalledStoreApp(t, "commandcode", 18080)
	s := testDesktopStoreServerWithService(t, svc)
	session := newFakeContainerTerminalSession()
	fake := &fakeContainerTerminalBackend{running: true, session: session}
	defer replaceContainerTerminalBackend(fake)()

	ts := httptest.NewServer(handleDesktopStoreAppRoute(s))
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/desktop/store/apps/commandcode/terminal", nil)
	if err != nil {
		t.Fatalf("dial store terminal: %v", err)
	}
	_ = conn.Close()

	select {
	case <-session.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("store terminal session was not closed after the WebSocket closed")
	}
	if got := session.writtenBeforeClose(); len(got) != 0 {
		t.Fatalf("store terminal wrote %q before close; its session lifecycle must not change", got)
	}
}
