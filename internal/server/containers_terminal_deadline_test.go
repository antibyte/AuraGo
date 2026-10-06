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

// TestContainerTerminalBoundsAStalledWrite: input the exec stream does not
// accept within containerTerminalWriteTimeout ends the session instead of
// blocking the WebSocket read loop for good.
func TestContainerTerminalBoundsAStalledWrite(t *testing.T) {
	old := containerTerminalWriteTimeout
	containerTerminalWriteTimeout = 100 * time.Millisecond
	t.Cleanup(func() { containerTerminalWriteTimeout = old })
	serverEnd, dockerEnd := net.Pipe()
	t.Cleanup(func() { _ = dockerEnd.Close() })
	t.Cleanup(replaceContainerTerminalBackend(stubTerminalBackend{session: pipeTerminalSession{serverEnd}}))
	t.Cleanup(replaceContainerProtection(containerProtection{}))
	handler := handleContainerAction(testContainerServer(true, false))
	handlerDone := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		handler(w, r)
	}))
	t.Cleanup(ts.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", nil)
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	defer conn.Close()
	// Nobody reads dockerEnd: the write stalls like a hung dockerd stdin.
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("ls\r")); err != nil {
		t.Fatalf("write input: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	var netErr net.Error
	if err == nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		t.Fatalf("read after a stalled write = %v; want the server to end the session within the write timeout", err)
	}
	// The handler goroutine ends too (it leaked before). Waiting for it also
	// orders its read of containerTerminalWriteTimeout before the cleanup
	// restores the variable, which go test -race needs: a hijacked connection
	// is not awaited by ts.Close.
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("the terminal handler did not return after the stalled write")
	}
}

// TestContainerTerminalWriteTimeoutToleratesABusyProgram pins the controller
// decision: the deadline only catches a hung daemon, so pasting into a program
// that reads its input slowly must never end the session.
func TestContainerTerminalWriteTimeoutToleratesABusyProgram(t *testing.T) {
	if containerTerminalWriteTimeout != 5*time.Minute {
		t.Fatalf("containerTerminalWriteTimeout = %v, want 5m", containerTerminalWriteTimeout)
	}
}

func TestDockerTerminalSessionAppliesTheWriteDeadline(t *testing.T) {
	a, b := net.Pipe()
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
