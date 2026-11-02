package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDockerTerminalRevocationClosesConnectionWithoutEndingShell(t *testing.T) {
	for _, policy := range []string{"read_only", "disabled", "host"} {
		t.Run(policy, func(t *testing.T) {
			s := testContainerServer(true, false)
			session := newFakeContainerTerminalSession()
			defer replaceContainerTerminalBackend(&fakeContainerTerminalBackend{running: true, session: session})()
			defer replaceContainerProtection(containerProtection{})()
			ts := httptest.NewServer(handleContainerAction(s))
			defer ts.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			next := *s.ConfigSnapshot()
			switch policy {
			case "read_only":
				next.Docker.ReadOnly = true
			case "disabled":
				next.Docker.Enabled = false
			case "host":
				next.Docker.Host = "unix:///changed.sock"
			}
			s.CfgMu.Lock()
			s.replaceConfigSnapshot(&next)
			s.CfgMu.Unlock()
			select {
			case <-session.closed:
			case <-time.After(3 * time.Second):
				t.Fatal("revoked session stayed open")
			}
			if got := session.writtenBeforeClose(); len(got) != 0 {
				t.Fatalf("revocation sent shell input: %q", got)
			}
		})
	}
}

func TestDockerTerminalRejectsStaleHostAndDisabledWrites(t *testing.T) {
	for _, tc := range []struct {
		enabled, readonly bool
		host              string
		status            int
	}{
		{false, false, "", 503}, {true, true, "", 403}, {true, false, "old-host", 409},
	} {
		s := testContainerServer(tc.enabled, tc.readonly)
		w := httptest.NewRecorder()
		done, admitted := s.admitDockerTerminal(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.host)
		if admitted {
			done()
			t.Fatal("unauthorized terminal admitted")
		}
		if w.Code != tc.status {
			t.Fatalf("status=%d want=%d", w.Code, tc.status)
		}
	}
}
