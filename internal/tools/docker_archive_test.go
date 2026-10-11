package tools

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newArchiveTestEngine serves /version plus the given handler for every
// versioned path (the /vX.Y prefix is stripped).
func newArchiveTestEngine(t *testing.T, handler http.HandlerFunc) DockerConfig {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45"}`)
			return
		}
		if i := strings.Index(r.URL.Path[1:], "/"); strings.HasPrefix(r.URL.Path, "/v") && i > 0 {
			r.URL.Path = r.URL.Path[i+1:]
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return DockerConfig{Host: "tcp://" + strings.TrimPrefix(srv.URL, "http://")}
}

func TestDockerGetArchiveContext(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(ClearRuntimePermissionsForTest)
	cfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/containers/builder/archive":
		case "/containers/gone/archive":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"No such container: gone"}`)
			return
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.URL.Query().Get("path") {
		case "/work/out":
			_, _ = io.WriteString(w, "tar-bytes")
		case "/work/big":
			_, _ = io.WriteString(w, strings.Repeat("x", 100))
		case "/work/denied":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"proxy refused\nthe call"}`)
		case "/work/long":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"`+strings.Repeat("e", 2000)+`"}`)
		case "/work/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Could not find the file /work/missing in container builder"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "404 page not found")
		}
	})
	ctx := context.Background()
	data, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/out", 64)
	if err != nil || string(data) != "tar-bytes" {
		t.Fatalf("archive = %q, %v", data, err)
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/missing", 64); !errors.Is(err, ErrDockerArchiveNotFound) {
		t.Fatalf("missing path = %v, want ErrDockerArchiveNotFound", err)
	}
	_, err = DockerGetArchiveContext(ctx, cfg, "gone", "/work/out", 64)
	if err == nil || errors.Is(err, ErrDockerArchiveNotFound) || !strings.Contains(err.Error(), "HTTP 404: No such container: gone") {
		t.Fatalf("missing container = %v, want a plain HTTP 404 error", err)
	}
	_, err = DockerGetArchiveContext(ctx, cfg, "builder", "/work/proxy404", 64)
	if err == nil || errors.Is(err, ErrDockerArchiveNotFound) || !strings.Contains(err.Error(), "HTTP 404: 404 page not found") {
		t.Fatalf("other 404 = %v, want a plain HTTP 404 error", err)
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/big", 64); !errors.Is(err, ErrDockerArchiveTooLarge) {
		t.Fatalf("large archive = %v, want ErrDockerArchiveTooLarge", err)
	}
	if data, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/big", 100); err != nil || len(data) != 100 {
		t.Fatalf("archive of exactly maxBytes = %d bytes, %v", len(data), err)
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/big", 99); !errors.Is(err, ErrDockerArchiveTooLarge) {
		t.Fatalf("archive of maxBytes+1 = %v, want ErrDockerArchiveTooLarge", err)
	}
	if data, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/big", math.MaxInt64); err != nil || len(data) != 100 {
		t.Fatalf("archive with maxBytes = MaxInt64 = %d bytes, %v", len(data), err)
	}
	for _, limit := range []int64{0, -1, math.MinInt64} {
		if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/out", limit); err == nil {
			t.Errorf("maxBytes %d was accepted", limit)
		}
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/denied", 64); err == nil || !strings.Contains(err.Error(), "HTTP 403: proxy refused the call") {
		t.Fatalf("refused archive = %v, want a sanitised HTTP 403 message", err)
	}
	_, err = DockerGetArchiveContext(ctx, cfg, "builder", "/work/long", 64)
	if err == nil || !strings.HasSuffix(err.Error(), "...") || len(err.Error()) > 600 {
		t.Fatalf("long Engine message = %v, want it cut with ...", err)
	}
	for _, bad := range []string{"work/out", "/work/../etc"} {
		if _, err := DockerGetArchiveContext(ctx, cfg, "builder", bad, 64); err == nil {
			t.Errorf("path %q was accepted", bad)
		}
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "../x", "/work/out", 64); err == nil {
		t.Error("an unsafe container name was accepted")
	}
}

func TestDockerStreamContextStreamsAndHonoursGates(t *testing.T) {
	release := make(chan struct{})
	cfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done(): // a failed test closes the body; never hang the server
			return
		}
		_, _ = io.WriteString(w, "second\n")
	})
	ctx := context.Background()
	t.Cleanup(ClearRuntimePermissionsForTest)

	ClearRuntimePermissionsForTest()
	if _, err := DockerStreamContext(ctx, cfg, http.MethodGet, "/events", nil, ""); err == nil {
		t.Fatal("a stream without the Docker permission was allowed")
	}
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true, DockerReadOnly: true})
	if _, err := DockerStreamContext(ctx, cfg, http.MethodPost, "/exec/x/start", []byte(`{}`), "application/json"); !errors.Is(err, ErrDockerReadOnly) {
		t.Fatalf("mutation under docker.read_only = %v, want ErrDockerReadOnly", err)
	}

	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	resp, err := DockerStreamContext(ctx, cfg, http.MethodPost, "/exec/x/start", []byte(`{}`), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	got := make(chan string, 1)
	go func() {
		line, _ := reader.ReadString('\n')
		got <- line
	}()
	select {
	case line := <-got:
		if line != "first\n" {
			t.Fatalf("first line = %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first chunk did not arrive before the response ended")
	}
	close(release)
	rest, _ := io.ReadAll(reader)
	if string(rest) != "second\n" {
		t.Fatalf("rest = %q", rest)
	}
}

func TestDockerStreamContextCancelEndsTheRead(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(ClearRuntimePermissionsForTest)
	cfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // the stream stays open until the client goes away
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := DockerStreamContext(ctx, cfg, http.MethodGet, "/events", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != "first\n" {
		t.Fatalf("first line = %q, %v", line, err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(reader)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("read after cancel = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the context did not end the stream read")
	}
}
