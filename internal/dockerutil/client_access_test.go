package dockerutil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientDialUsesTheClientsOwnTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	}))
	defer server.Close()
	addr := server.Listener.Addr().String()

	var dials atomic.Int32
	dialed := NewClientWithDialer("custom", func(ctx context.Context) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}, 0, 0)
	defer dialed.CloseIdleConnections()
	plain := NewClient("tcp://"+addr, time.Second)
	defer plain.CloseIdleConnections()

	for name, c := range map[string]*Client{"dialer": dialed, "endpoint": plain} {
		conn, err := c.Dial(context.Background())
		if err != nil {
			t.Fatalf("%s: Dial: %v", name, err)
		}
		if _, err := io.WriteString(conn, "GET /ping HTTP/1.1\r\nHost: docker\r\nConnection: close\r\n\r\n"); err != nil {
			t.Fatalf("%s: write: %v", name, err)
		}
		status, err := bufio.NewReader(conn).ReadString('\n')
		conn.Close()
		if err != nil || status != "HTTP/1.1 200 OK\r\n" {
			t.Fatalf("%s: first line = %q, %v, want an HTTP 200 answer over the raw connection", name, status, err)
		}
	}
	if dials.Load() != 1 {
		t.Fatalf("the caller's dial function ran %d times, want once (for Dial)", dials.Load())
	}
}

func TestClientDialWithoutDialFunctionFailsInsteadOfPanicking(t *testing.T) {
	client := NewClientWithDialer("none", nil, time.Second, 0)
	defer client.CloseIdleConnections()
	if conn, err := client.Dial(context.Background()); err == nil {
		conn.Close()
		t.Fatal("Dial succeeded without a dial function")
	}
	var missing *Client
	if _, err := missing.Dial(context.Background()); err == nil {
		t.Fatal("Dial on a nil client succeeded")
	}
}

func TestClientAPIVersionReturnsTheNegotiatedVersionAndProbesOnce(t *testing.T) {
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			probes.Add(1)
			fmt.Fprint(w, `{"ApiVersion":"1.41","MinAPIVersion":"1.24"}`)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	client := NewClient("tcp://"+server.Listener.Addr().String(), 2*time.Second)
	defer client.CloseIdleConnections()

	version, err := client.APIVersion(context.Background())
	if err != nil || version != "1.41" {
		t.Fatalf("APIVersion = %q, %v, want 1.41 without the v prefix", version, err)
	}
	if code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, nil); err != nil || code != http.StatusOK {
		t.Fatalf("DoJSON = %d, %v", code, err)
	}
	if again, _ := client.APIVersion(context.Background()); again != "1.41" || probes.Load() != 1 {
		t.Fatalf("APIVersion = %q after %d probes, want the cached 1.41 after one probe", again, probes.Load())
	}
}

func TestClientAPIVersionReportsAFailedProbe(t *testing.T) {
	client := NewClientWithDialer("down", func(ctx context.Context) (net.Conn, error) {
		return nil, fmt.Errorf("dial tcp 127.0.0.1:1: connect: connection refused")
	}, time.Second, 0)
	defer client.CloseIdleConnections()
	if version, err := client.APIVersion(context.Background()); err == nil || version != "" {
		t.Fatalf("APIVersion = %q, %v, want an error", version, err)
	}
}

func TestStreamingHTTPClientHasNoTotalTimeoutAndSharesTheTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			fmt.Fprint(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`)
			return
		}
		fmt.Fprint(w, "first ")
		w.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		fmt.Fprint(w, "second")
	}))
	defer server.Close()
	client := NewClient("tcp://"+server.Listener.Addr().String(), 50*time.Millisecond)
	defer client.CloseIdleConnections()

	streaming := client.StreamingHTTPClient()
	if streaming.Timeout != 0 {
		t.Fatalf("Timeout = %v, want none", streaming.Timeout)
	}
	if streaming.Transport != client.HTTPClient().Transport {
		t.Fatal("the streaming client must reuse the Docker transport")
	}
	if client.HTTPClient().Timeout != 50*time.Millisecond {
		t.Fatalf("the shared client's timeout changed to %v", client.HTTPClient().Timeout)
	}

	read := func(hc *http.Client) (string, error) {
		req, _ := http.NewRequest(http.MethodGet, Endpoint("events"), nil)
		resp, err := hc.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}
	if body, err := read(client.HTTPClient()); err == nil {
		t.Fatalf("the 50ms client read %q without an error, want its total timeout to cut the slow body", body)
	}
	if body, err := read(streaming); err != nil || body != "first second" {
		t.Fatalf("streaming read = %q, %v, want the whole slow body", body, err)
	}
	var missing *Client
	if missing.StreamingHTTPClient() != nil {
		t.Fatal("StreamingHTTPClient on a nil client should be nil")
	}
}
