package tools

import (
	"bufio"
	"context"
	"errors"
	"io"
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
		if r.Method != http.MethodGet || r.URL.Path != "/containers/builder/archive" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Query().Get("path") {
		case "/work/out":
			_, _ = io.WriteString(w, "tar-bytes")
		case "/work/big":
			_, _ = io.WriteString(w, strings.Repeat("x", 100))
		case "/work/denied":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"proxy refused\nthe call"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
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
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/big", 64); !errors.Is(err, ErrDockerArchiveTooLarge) {
		t.Fatalf("large archive = %v, want ErrDockerArchiveTooLarge", err)
	}
	if _, err := DockerGetArchiveContext(ctx, cfg, "builder", "/work/denied", 64); err == nil || !strings.Contains(err.Error(), "HTTP 403: proxy refused the call") {
		t.Fatalf("refused archive = %v, want a sanitised HTTP 403 message", err)
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
		<-release
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
