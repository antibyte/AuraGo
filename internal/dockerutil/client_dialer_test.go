package dockerutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientWithDialerUsesTheDialFunctionAndNegotiatesTheVersion(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			fmt.Fprint(w, `{"ApiVersion":"1.41","MinAPIVersion":"1.24"}`)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	var dials atomic.Int32
	client := NewClientWithDialer("test-engine", func(ctx context.Context) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}, 0, 0)
	defer client.CloseIdleConnections()

	if client.Host() != "test-engine" {
		t.Fatalf("Host() = %q, want the name passed in", client.Host())
	}
	var out struct {
		OK bool `json:"ok"`
	}
	code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, &out)
	if err != nil || code != http.StatusOK || !out.OK {
		t.Fatalf("DoJSON = (%d, %v, %+v), want an answer through the dial function", code, err, out)
	}
	mu.Lock()
	got := fmt.Sprint(paths)
	mu.Unlock()
	if got != "[/v1.41/containers/json]" {
		t.Fatalf("Engine saw %s, want the negotiated /v1.41 path", got)
	}
	if dials.Load() == 0 {
		t.Fatal("the dial function was never used")
	}
}

func TestNewClientWithDialerHonoursTheProbeTimeout(t *testing.T) {
	client := NewClientWithDialer("slow-engine", func(ctx context.Context) (net.Conn, error) {
		<-ctx.Done() // an SSH login that never finishes
		return nil, ctx.Err()
	}, 10*time.Second, 50*time.Millisecond)
	defer client.CloseIdleConnections()

	start := time.Now()
	code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, nil)
	if code != 0 || err == nil {
		t.Fatalf("DoJSON = (%d, %v), want status 0 with an error", code, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("the version probe took %v, want it bounded by the 50ms probe timeout", elapsed)
	}
}

func TestNewClientWithDialerWithoutDialFunctionFailsInsteadOfPanicking(t *testing.T) {
	client := NewClientWithDialer("none", nil, time.Second, 0)
	defer client.CloseIdleConnections()
	if code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, nil); code != 0 || err == nil {
		t.Fatalf("DoJSON = (%d, %v), want status 0 with an error", code, err)
	}
}
