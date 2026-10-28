package server

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func dmTermWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// dmTermServeWithOptions runs a fake session behind a WebSocket served with opts.
func dmTermServeWithOptions(t *testing.T, opts containerTerminalServeOptions) (*websocket.Conn, *fakeContainerTerminalSession) {
	t.Helper()
	session := newFakeContainerTerminalSession()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := containerTerminalUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serveContainerTerminalSessionWith(r.Context(), conn, session, opts)
	}))
	t.Cleanup(ts.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, session
}

func TestServeContainerTerminalSessionWithBoundsTheBrowsersFrames(t *testing.T) {
	conn, session := dmTermServeWithOptions(t, containerTerminalServeOptions{ReadLimit: 64})
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("0123456789")); err != nil {
		t.Fatal(err)
	}
	dmTermWaitFor(t, "the small frame", func() bool {
		session.mu.Lock()
		defer session.mu.Unlock()
		return string(session.written) == "0123456789"
	})
	if err := conn.WriteMessage(websocket.BinaryMessage, make([]byte, 200)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("a frame over the read limit did not end the session")
	}
	if got := session.writtenBeforeClose(); string(got) != "0123456789" {
		t.Fatalf("the session got %q, want only the frame within the limit", got)
	}
}

func TestServeContainerTerminalSessionWithPingsAndEndsWhenTheBrowserGoesSilent(t *testing.T) {
	// A browser that reads answers every ping with a pong: the session outlives several pong waits.
	conn, session := dmTermServeWithOptions(t, containerTerminalServeOptions{PingInterval: 20 * time.Millisecond, PongWait: 300 * time.Millisecond})
	var pings atomic.Int32
	conn.SetPingHandler(func(data string) error {
		pings.Add(1)
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(900 * time.Millisecond) // three pong waits
	if pings.Load() < 10 {
		t.Fatalf("the browser saw %d pings in 900 ms, want about 40", pings.Load())
	}
	select {
	case <-session.closed:
		t.Fatal("a browser that answers pings lost its session")
	default:
	}

	// A browser that never reads never answers: the session ends after one pong wait.
	silent, silentSession := dmTermServeWithOptions(t, containerTerminalServeOptions{PingInterval: 20 * time.Millisecond, PongWait: 150 * time.Millisecond})
	_ = silent
	select {
	case <-silentSession.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("a silent browser kept its session open")
	}
}
