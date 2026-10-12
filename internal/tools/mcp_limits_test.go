package tools

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMCPWebSocketBlockedWriteAndWaitingNotification(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-release
	}))
	defer srv.Close()
	defer close(release)
	tr, err := newWebSocketMCPTransport(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tr.Notify(ctx, "test", strings.Repeat("x", 8<<20)) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("blocked write succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("write ignored deadline")
	}
	if err := tr.writeMu.Lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer tr.writeMu.Unlock()
	waiter, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if err := tr.Notify(waiter, "test", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestMCPWebSocketOversizeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var req jsonRPCRequest
		if conn.ReadJSON(&req) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{"id": req.ID, "result": strings.Repeat("x", mcpMessageLimit)})
	}))
	defer srv.Close()
	tr, err := newWebSocketMCPTransport(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	if _, err := tr.Send(context.Background(), "test", nil); err == nil {
		t.Fatal("oversized websocket response accepted")
	}
}

func TestMCPStdioWritesAndWaitersRespectCancellation(t *testing.T) {
	for _, notify := range []bool{false, true} {
		t.Run(fmt.Sprint(notify), func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			tr := newStdioMCPTransport(nil, writer, strings.NewReader(""), nil)
			defer tr.Close()
			ctx, cancel := context.WithCancel(context.Background())
			if err := tr.mu.Lock(ctx); err != nil {
				t.Fatal(err)
			}
			tr.mu.Unlock()
			done := make(chan error, 1)
			go func() {
				if notify {
					done <- tr.Notify(ctx, "test", nil)
				} else {
					_, err := tr.Send(ctx, "test", nil)
					done <- err
				}
			}()
			// The first write owns the transport lock while the pipe has no reader.
			deadline := time.Now().Add(time.Second)
			for len(tr.mu.token) == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			waiter, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
			if err := tr.Notify(waiter, "waiter", nil); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("waiter=%v", err)
			}
			stop()
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("write=%v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked write outlived cancellation")
			}
		})
	}
}

func TestMCPReadAndStderrLimits(t *testing.T) {
	for _, limit := range []int{mcpDiscoveryPageLimit, mcpMessageLimit} {
		for _, delta := range []int{-1, 0, 1} {
			input := strings.Repeat("x", limit+delta)
			data, err := readMCPBounded(strings.NewReader(input), limit)
			if delta > 0 {
				if !errors.Is(err, errMCPMessageTooLarge) {
					t.Fatal(err)
				}
			} else if err != nil || len(data) != len(input) {
				t.Fatal("boundary rejected")
			}
		}
	}
	tr := newStdioMCPTransport(nil, nil, strings.NewReader(strings.Repeat("x", mcpMessageLimit+1)), nil)
	if _, err := tr.readResponse(1); !errors.Is(err, errMCPMessageTooLarge) {
		t.Fatal(err)
	}
	b := &safeBuffer{}
	input := strings.Repeat("a", 17<<20)
	if n, err := b.Write([]byte(input)); n != len(input) || err != nil {
		t.Fatal("short stderr write")
	}
	b.Write([]byte("tail"))
	if b.Len() != mcpStderrLimit || !strings.HasSuffix(b.String(), "tail") || cap(b.buf) > mcpStderrLimit {
		t.Fatal("stderr retention exceeded limit")
	}
}

func TestMCPHTTPBoundsDecodedContentAndExpiredSession(t *testing.T) {
	for _, method := range []string{"tools/list", "tools/call"} {
		limit := mcpMessageLimit
		if method == "tools/list" {
			limit = mcpDiscoveryPageLimit
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				return
			}
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			io.WriteString(gz, `{"id":1,"result":"`)
			io.WriteString(gz, strings.Repeat("x", limit))
			io.WriteString(gz, `"}`)
		}))
		tr, err := newHTTPMCPTransport(srv.URL, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tr.Send(context.Background(), method, nil)
		if !errors.Is(err, errMCPMessageTooLarge) {
			t.Fatalf("%s: %v", method, err)
		}
		if _, err = tr.Send(context.Background(), "ping", nil); !errors.Is(err, errMCPTransportClosed) {
			t.Fatalf("oversized connection reused: %v", err)
		}
		tr.Close()
		srv.Close()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer srv.Close()
	tr, err := newHTTPMCPTransport(srv.URL, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	tr.sessionID = "expired-fixture"
	if _, err := tr.Send(context.Background(), "tools/list", nil); !isRetryableMCPTransportError(err) {
		t.Fatalf("session not invalidated: %v", err)
	}
}

func TestMCPSSEEventBoundAndInvalidEventReset(t *testing.T) {
	body := "data: invalid\n\ndata: " + `{"id":1,` + "\ndata: " + `"result":{"tools":[]}}` + "\n\n"
	if _, err := readMCPStreamableHTTPResponse(strings.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	body = strings.Repeat("data: "+strings.Repeat("x", 1<<20)+"\n", 33) + "\n"
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 64<<10), mcpMessageLimit+1)
	if _, _, err := readMCPSSEEvent(scanner); !errors.Is(err, errMCPMessageTooLarge) {
		t.Fatal(err)
	}
}

func TestMCPCatalogTotalLimitAndEmptyArray(t *testing.T) {
	conn := &mcpConn{transport: &disclosureMCPTransport{responses: []string{`{"tools":[]}`}}}
	if err := conn.discoverTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	var pages []string
	for i := 0; i < 5; i++ {
		raw, _ := json.Marshal(map[string]interface{}{"tools": []interface{}{map[string]interface{}{"name": fmt.Sprint(i), "description": strings.Repeat("x", 13<<20)}}, "nextCursor": fmt.Sprint(i)})
		pages = append(pages, string(raw))
	}
	conn.transport = &disclosureMCPTransport{responses: pages}
	if err := conn.discoverTools(context.Background(), nil); !errors.Is(err, errMCPMessageTooLarge) {
		t.Fatal(err)
	}
	if len(conn.tools) != 0 {
		t.Fatal("partial catalog published")
	}
}

func TestMCPSSESetupCancellationAndIndependentLifetime(t *testing.T) {
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := newSSEMCPTransport(ctx, srv.URL, nil, true); done <- err }()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled setup accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("setup ignored cancellation")
	}
	srv.Close()
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: endpoint\ndata: /message\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel = context.WithCancel(context.Background())
	tr, err := newSSEMCPTransport(ctx, srv.URL, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-tr.closed:
		t.Fatal("caller cancellation closed established stream")
	case <-time.After(20 * time.Millisecond):
	}
	tr.Close()
}

func TestMCPExplicitLongDeadlinePreserved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	bounded, stop := mcpRequestContext(ctx)
	defer stop()
	want, _ := ctx.Deadline()
	got, _ := bounded.Deadline()
	if !got.Equal(want) {
		t.Fatal("explicit sandbox deadline shortened")
	}
}
