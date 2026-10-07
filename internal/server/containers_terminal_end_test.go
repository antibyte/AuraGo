package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// endingFakeTerminalSession ends its "shell" on End: it records the call and
// closes the stream, as an exiting shell would.
type endingFakeTerminalSession struct {
	*fakeContainerTerminalSession
	ends atomic.Int32
}

func (s *endingFakeTerminalSession) End(context.Context) error {
	s.ends.Add(1)
	return s.Close()
}

type endingFakeBackend struct{ session *endingFakeTerminalSession }

func (b endingFakeBackend) ContainerRunning(context.Context, tools.DockerConfig, string) (bool, error) {
	return true, nil
}

func (b endingFakeBackend) CreateSession(context.Context, tools.DockerConfig, string, int, int, []string) (containerTerminalSession, error) {
	return b.session, nil
}

func dialEndingTerminal(t *testing.T) (*endingFakeTerminalSession, *websocket.Conn) {
	t.Helper()
	session := &endingFakeTerminalSession{fakeContainerTerminalSession: newFakeContainerTerminalSession()}
	t.Cleanup(replaceContainerTerminalBackend(endingFakeBackend{session: session}))
	t.Cleanup(replaceContainerProtection(containerProtection{}))
	ts := httptest.NewServer(handleContainerAction(testContainerServer(true, false)))
	t.Cleanup(ts.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", nil)
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return session, conn
}

// TestContainerTerminalEndMessageEndsTheShell: {"type":"end"} ends the shell
// through the session's End, types nothing into the terminal, and the
// WebSocket closes once the shell is gone.
func TestContainerTerminalEndMessageEndsTheShell(t *testing.T) {
	session, conn := dialEndingTerminal(t)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"end"}`)); err != nil {
		t.Fatalf("send end: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	var netErr net.Error
	if err == nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		t.Fatalf("read after end = %v; want the server to close the WebSocket", err)
	}
	if got := session.ends.Load(); got != 1 {
		t.Fatalf("End calls = %d, want 1", got)
	}
	if got := session.writtenBeforeClose(); len(got) != 0 {
		t.Fatalf("End typed %q into the terminal; it must signal the process instead", got)
	}
}

// TestContainerTerminalCloseAndOtherMessagesNeverEndTheShell pins that only the
// explicit end message ends the shell: closing the WebSocket, resize and
// unknown control messages leave it running, so tmux and screen survive.
func TestContainerTerminalCloseAndOtherMessagesNeverEndTheShell(t *testing.T) {
	session, conn := dialEndingTerminal(t)
	for _, msg := range []string{`{"type":"resize","cols":100,"rows":40}`, `{"type":"END"}`, `{"type":"ended"}`, `end`, `{"kind":"end"}`} {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("send %s: %v", msg, err)
		}
	}
	select {
	case <-session.resizeCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("resize was not applied")
	}
	_ = conn.Close()
	select {
	case <-session.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("session not closed after the WebSocket closed")
	}
	if got := session.ends.Load(); got != 0 {
		t.Fatalf("End calls = %d, want 0", got)
	}
}

// TestContainerTerminalBinaryEndTextNeverEndsTheShell: typed or pasted input
// arrives as binary frames, so the text {"type":"end"} in the input stream
// reaches the shell as keystrokes and never ends it.
func TestContainerTerminalBinaryEndTextNeverEndsTheShell(t *testing.T) {
	session, conn := dialEndingTerminal(t)
	input := []byte(`{"type":"end"}`)
	if err := conn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("send input: %v", err)
	}
	_ = conn.Close()
	select {
	case <-session.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("session not closed after the WebSocket closed")
	}
	if got := session.ends.Load(); got != 0 {
		t.Fatalf("End calls = %d, want 0 for binary input", got)
	}
	if got := session.writtenBeforeClose(); string(got) != string(input) {
		t.Fatalf("shell input = %q, want %q", got, input)
	}
}

// TestDockerTerminalSessionEndRunsATaggedHangupExec: End starts a detached,
// non-TTY exec that sends SIGHUP to the exec process (PPid 0) tagged with the
// session's value, which is passed as an argument, never inside the script.
func TestDockerTerminalSessionEndRunsATaggedHangupExec(t *testing.T) {
	var created, started string
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/web/exec"):
			created = string(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"end-exec-1"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/exec/end-exec-1/start"):
			started = string(body)
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, `{"message":"unexpected request"}`, http.StatusConflict)
		}
	})
	const nonce = "0123456789abcdef0123456789abcdef"
	session := &dockerContainerTerminalSession{cfg: tools.DockerConfig{Host: host}, execID: "shell-exec", containerID: "web", nonce: nonce}
	if err := session.End(context.Background()); err != nil {
		t.Fatalf("End: %v", err)
	}
	var exec struct {
		Cmd         []string
		Tty         bool
		AttachStdin bool
	}
	if err := json.Unmarshal([]byte(created), &exec); err != nil {
		t.Fatalf("decode exec create %q: %v", created, err)
	}
	if len(exec.Cmd) != 5 || exec.Cmd[0] != "/bin/sh" || exec.Cmd[1] != "-c" || exec.Cmd[3] != "aurago-end-session" || exec.Cmd[4] != nonce {
		t.Fatalf("end exec Cmd = %q", exec.Cmd)
	}
	for _, part := range []string{"PPid:", "kill -HUP", containerTerminalSessionEnv + "=$1"} {
		if !strings.Contains(exec.Cmd[2], part) {
			t.Fatalf("end script misses %q:\n%s", part, exec.Cmd[2])
		}
	}
	if strings.Contains(exec.Cmd[2], nonce) || exec.Tty || exec.AttachStdin {
		t.Fatalf("end exec = %+v: the tag must be an argument and the exec must not take a TTY or stdin", exec)
	}
	var start struct{ Detach, Tty bool }
	if err := json.Unmarshal([]byte(started), &start); err != nil || !start.Detach || start.Tty {
		t.Fatalf("exec start = %q (%v), want a detached start", started, err)
	}
	if err := (&dockerContainerTerminalSession{cfg: tools.DockerConfig{Host: host}, containerID: "web"}).End(context.Background()); err == nil {
		t.Fatal("a session without a tag must not end anything")
	}
}

func TestContainerTerminalExecPayloadTagsEverySession(t *testing.T) {
	a, err := newContainerTerminalNonce()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newContainerTerminalNonce()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(a) {
		t.Fatalf("nonces %q and %q must be distinct 32-hex values", a, b)
	}
	payload := containerTerminalExecPayload([]string{"/bin/sh"}, a)
	if env, _ := payload["Env"].([]string); !slices.Equal(env, []string{"TERM=xterm-256color", containerTerminalSessionEnv + "=" + a}) {
		t.Fatalf("Env = %v", payload["Env"])
	}
	if payload["Tty"] != true || payload["AttachStdin"] != true || !slices.Equal(payload["Cmd"].([]string), []string{"/bin/sh"}) {
		t.Fatalf("payload = %v", payload)
	}
}
