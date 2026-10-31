package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/gorilla/websocket"
)

func TestCodeStudioOperationBindsCurrentDockerHostAndContainer(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	routes := newCodeStudioHandlers(s)
	first := routes
	_, id, err := first.codeContainer(context.Background(), true)
	if err != nil || id != "code-1" {
		t.Fatalf("first acquisition: id=%s err=%v", id, err)
	}
	firstHost := first.docker.(codeStudioDockerAdapter).cfg.Host
	// A route registered before the host switch must use the newly owned
	// service and its transport together. No request is sent to either host.
	s.CfgMu.Lock()
	s.Cfg.Docker.Host = "tcp://new-docker-host:2375"
	s.CfgMu.Unlock()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	svc.CodeContainer().SetDockerClient(&fakeServerCodeContainerDocker{
		containers:    []desktop.CodeDockerContainer{{ID: "code-2", Names: []string{"/aurago-code-studio"}}},
		inspectByName: map[string]desktop.CodeDockerInspect{"code-2": {ID: "code-2", State: desktop.CodeDockerState{Running: true}, NetworkMode: "none"}},
	})
	second := routes
	_, id, err = second.codeContainer(context.Background(), true)
	if err != nil || id != "code-2" {
		t.Fatalf("second acquisition: id=%s err=%v", id, err)
	}
	if got := second.docker.(codeStudioDockerAdapter).cfg.Host; got != s.Cfg.Docker.Host {
		t.Fatalf("stale adapter: %q", got)
	}
	if first.docker.(codeStudioDockerAdapter).cfg.Host != firstHost || routes.docker != nil {
		t.Fatal("acquisition changed another request's transport")
	}
}

func TestCodeStudioStatusUsesLiveReadOnlyPolicy(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := newCodeStudioHandlers(s)
	for _, readOnly := range []bool{true, false} {
		s.CfgMu.Lock()
		s.Cfg.VirtualDesktop.ReadOnly = readOnly
		s.CfgMu.Unlock()
		s.DesktopService.SetReadOnly(readOnly)
		rec := httptest.NewRecorder()
		h.handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/code-studio/status", nil))
		var response struct {
			CodeStudio struct {
				ReadOnly bool `json:"read_only"`
				Sessions int  `json:"active_terminal_sessions"`
			} `json:"code_studio"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || rec.Code != 200 || response.CodeStudio.ReadOnly != readOnly || response.CodeStudio.Sessions != 0 {
			t.Fatalf("live status: %s (%v)", rec.Body.String(), err)
		}
	}
}

func TestCodeStudioTerminalUpgradeFailureReleasesLease(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: &recordingCodeStudioDockerAPI{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/code-studio/terminal", nil)
	req.Header.Set("Origin", "http://example.com")
	h.handleTerminal(rec, req)
	if rec.Code == http.StatusSwitchingProtocols {
		t.Fatal("invalid request upgraded")
	}
	if status := s.DesktopService.CodeContainer().Status(context.Background()); status.ActiveTerminalSessions != 0 {
		t.Fatalf("failed upgrade leaked %d leases", status.ActiveTerminalSessions)
	}
}

func TestCodeStudioTerminalConnectionsReleaseLeases(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	h := codeStudioHandlers{server: s, docker: fakeCodeStudioDockerAPI{}}
	srv := httptest.NewServer(http.HandlerFunc(h.handleTerminal))
	defer srv.Close()
	dial := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/code-studio/terminal", http.Header{"Origin": []string{srv.URL}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	count := func(want int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if s.DesktopService.CodeContainer().Status(context.Background()).ActiveTerminalSessions == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("terminal lease count did not reach %d", want)
	}
	first, second := dial(), dial()
	count(2)
	_ = first.Close()
	count(1)
	_ = second.Close()
	count(0)
}
