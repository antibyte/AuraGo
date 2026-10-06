package server

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// pipeTerminalSession is a session on one end of net.Pipe: Write blocks until
// somebody reads the other end, like a stalled Docker daemon stdin.
type pipeTerminalSession struct{ net.Conn }

func (pipeTerminalSession) Resize(context.Context, int, int) error { return nil }

type stubTerminalBackend struct{ session containerTerminalSession }

func (b stubTerminalBackend) ContainerRunning(context.Context, tools.DockerConfig, string) (bool, error) {
	return true, nil
}

func (b stubTerminalBackend) CreateSession(context.Context, tools.DockerConfig, string, int, int, []string) (containerTerminalSession, error) {
	return b.session, nil
}

// startPipeContainerTerminal opens a container terminal whose exec stream is
// one end of net.Pipe, with containerTerminalWriteTimeout set to writeTimeout.
// The test holds the Docker end; handlerDone closes when the handler returns.
// Tests wait for it before the cleanup restores the timeout: a hijacked
// connection is not awaited by ts.Close, and go test -race needs the handler's
// read of the variable ordered before the restore.
func startPipeContainerTerminal(t *testing.T, writeTimeout time.Duration) (conn *websocket.Conn, dockerEnd net.Conn, handlerDone <-chan struct{}) {
	t.Helper()
	old := containerTerminalWriteTimeout
	containerTerminalWriteTimeout = writeTimeout
	t.Cleanup(func() { containerTerminalWriteTimeout = old })
	serverEnd, dockerEnd := net.Pipe()
	t.Cleanup(func() { _ = dockerEnd.Close() })
	t.Cleanup(replaceContainerTerminalBackend(stubTerminalBackend{session: pipeTerminalSession{serverEnd}}))
	t.Cleanup(replaceContainerProtection(containerProtection{}))
	handler := handleContainerAction(testContainerServer(true, false))
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		handler(w, r)
	}))
	t.Cleanup(ts.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", nil)
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, dockerEnd, done
}

func waitForContainerTerminalHandler(t *testing.T, handlerDone <-chan struct{}, after string) {
	t.Helper()
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("the terminal handler did not return after %s", after)
	}
}

// TestContainerTerminalBoundsAStalledWrite: input the exec stream does not
// accept within containerTerminalWriteTimeout ends the session instead of
// blocking the WebSocket read loop for good.
func TestContainerTerminalBoundsAStalledWrite(t *testing.T) {
	conn, _, handlerDone := startPipeContainerTerminal(t, 100*time.Millisecond)
	// Nobody reads the Docker end: the write stalls like a hung dockerd stdin.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("ls\r")); err != nil {
		t.Fatalf("write input: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	var netErr net.Error
	if err == nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		t.Fatalf("read after a stalled write = %v; want the server to end the session within the write timeout", err)
	}
	// The handler goroutine ends too (it leaked before).
	waitForContainerTerminalHandler(t, handlerDone, "the stalled write")
}

// TestContainerTerminalRearmsTheWriteDeadlinePerWrite: every input write gets
// a fresh deadline, so a session that stays idle longer than the timeout
// still accepts input. A deadline set once per session would refuse the
// second write.
func TestContainerTerminalRearmsTheWriteDeadlinePerWrite(t *testing.T) {
	const timeout = 200 * time.Millisecond
	conn, dockerEnd, handlerDone := startPipeContainerTerminal(t, timeout)
	send := func(input string) {
		t.Helper()
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte(input)); err != nil {
			t.Fatalf("write %q: %v", input, err)
		}
		_ = dockerEnd.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 16)
		n, err := dockerEnd.Read(buf)
		if err != nil || string(buf[:n]) != input {
			t.Fatalf("exec stdin received %q, %v; want %q", buf[:n], err, input)
		}
	}

	send("a")
	time.Sleep(3 * timeout)
	send("b")

	_ = conn.Close()
	waitForContainerTerminalHandler(t, handlerDone, "the browser closed")
}

// TestContainerTerminalWriteTimeoutStaysFiveMinutes pins the controller
// decision: the deadline only catches a hung daemon, so pasting into a program
// that reads its input slowly must never end the session.
func TestContainerTerminalWriteTimeoutStaysFiveMinutes(t *testing.T) {
	if containerTerminalWriteTimeout != 5*time.Minute {
		t.Fatalf("containerTerminalWriteTimeout = %v, want 5m", containerTerminalWriteTimeout)
	}
}

func TestDockerTerminalSessionAppliesTheWriteDeadline(t *testing.T) {
	a, b := net.Pipe()
	// Without the deadline the write would block for good: fail instead.
	stop := time.AfterFunc(2*time.Second, func() { _ = a.Close() })
	defer stop.Stop()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	session := &dockerContainerTerminalSession{stream: &bufferedDockerRawConn{Conn: a, reader: bufio.NewReader(a)}}
	if err := session.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	_, err := session.Write([]byte("x"))
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("write into a stalled stream = %v, want a timeout", err)
	}
}
