package localllm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"
	"unicode"

	"aurago/internal/dockerutil"
)

var testRuntimeImage = "ghcr.io/example/aurago-llm-cuda@sha256:" + strings.Repeat("a", 64)

// pullTestEngine answers DoJSON through respond and serves image pulls from
// client, recording every streaming timeout requested.
type pullTestEngine struct {
	client   *http.Client
	respond  func(method, path string) (int, error)
	timeouts []time.Duration
}

func (engine *pullTestEngine) DoJSON(_ context.Context, method, path string, _, _ any) (int, error) {
	if engine.respond == nil {
		return http.StatusOK, nil
	}
	return engine.respond(method, path)
}

func (engine *pullTestEngine) HTTPClient() *http.Client { return engine.client }

func (engine *pullTestEngine) HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	engine.timeouts = append(engine.timeouts, timeout)
	return engine.client
}

// pullStreamClient answers every request with status and body.
func pullStreamClient(status int, body io.Reader) *http.Client {
	return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(body), Header: make(http.Header)}, nil
	})}
}

func codeOrEmpty(err error) string {
	if err == nil {
		return ""
	}
	return errorCode(err)
}

func TestPullImageRequestsLongStreamingTimeout(t *testing.T) {
	engine := &pullTestEngine{client: pullStreamClient(http.StatusOK, strings.NewReader(`{"status":"Status: Downloaded newer image"}`+"\n"))}
	manager := &Manager{docker: engine}
	if err := manager.pullImage(context.Background(), testRuntimeImage); err != nil {
		t.Fatalf("pullImage() error = %v", err)
	}
	if len(engine.timeouts) != 1 || engine.timeouts[0] != 2*time.Hour {
		t.Fatalf("streaming timeouts = %v, want one 2h pull client", engine.timeouts)
	}
}

func TestPullImageOutlivesShortEngineTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/images/create") {
			t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 6; i++ {
			fmt.Fprintf(w, `{"status":"Downloading","id":"layer","progressDetail":{"current":%d,"total":6}}`+"\n", i)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(60 * time.Millisecond)
		}
		_, _ = io.WriteString(w, `{"status":"Status: Downloaded newer image"}`+"\n")
	}))
	defer server.Close()
	// The Engine client's own timeout is far shorter than the pull.
	manager := &Manager{docker: dockerutil.NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), 50*time.Millisecond)}
	if err := manager.pullImage(context.Background(), testRuntimeImage); err != nil {
		t.Fatalf("pullImage() error = %v, want the pull to outlive the 50ms Engine client timeout", err)
	}
}

func TestPullImageReportsStreamErrorDetail(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"error detail with error", http.StatusOK, `{"status":"Pulling fs layer"}` + "\n" + `{"errorDetail":{"message":"no matching manifest for linux/amd64"},"error":"no matching manifest for linux/amd64"}` + "\n", "no matching manifest for linux/amd64"},
		{"error detail only", http.StatusOK, `{"errorDetail":{"message":"registry denied the pull"}}` + "\n", "registry denied the pull"},
		{"non-2xx carries the engine message", http.StatusNotFound, `{"message":"manifest unknown"}`, "Docker returned 404: manifest unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(tc.status, strings.NewReader(tc.body))}}
			err := manager.pullImage(context.Background(), testRuntimeImage)
			if codeOrEmpty(err) != "pull_image_failed" || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("pullImage() error = %v, want pull_image_failed naming %q", err, tc.want)
			}
		})
	}
	t.Run("detail is one bounded line", func(t *testing.T) {
		body := `{"error":"denied:\u001b[31m token\nexpired ` + strings.Repeat("x", 1000) + `"}` + "\n"
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusOK, strings.NewReader(body))}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if codeOrEmpty(err) != "pull_image_failed" {
			t.Fatalf("pullImage() error = %v, want pull_image_failed", err)
		}
		if text := err.Error(); strings.ContainsAny(text, "\x1b\n") || len(text) > len("pull_image_failed: ")+256 {
			t.Fatalf("pullImage() error = %q, want one line with at most 256 detail bytes", text)
		}
	})
}

func TestPullImageRejectsTruncatedStream(t *testing.T) {
	t.Run("stream ends inside a message", func(t *testing.T) {
		body := strings.NewReader(`{"status":"Downloading"}` + "\n" + `{"status":"Downlo`)
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusOK, body)}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if codeOrEmpty(err) != "pull_image_failed" {
			t.Fatalf("pullImage() error = %v, want pull_image_failed", err)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("pullImage() error = %v, want it to wrap io.ErrUnexpectedEOF", err)
		}
	})
	t.Run("connection lost", func(t *testing.T) {
		body := io.MultiReader(strings.NewReader(`{"status":"Downloading"}`+"\n"), iotest.ErrReader(io.ErrUnexpectedEOF))
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusOK, body)}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if codeOrEmpty(err) != "pull_image_failed" || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("pullImage() error = %v, want pull_image_failed wrapping io.ErrUnexpectedEOF", err)
		}
	})
}

func TestDeleteHelpersClassifyByStatusCode(t *testing.T) {
	notFound := errors.New("Docker API returned 404: No such container: aurago-local-llm")
	cases := []struct {
		name    string
		code    int
		err     error
		volume  bool
		wantErr bool
	}{
		{"container already gone", http.StatusNotFound, notFound, false, false},
		{"volume already gone", http.StatusNotFound, notFound, true, false},
		{"removed", http.StatusNoContent, nil, false, false},
		{"volume in use names an ID containing 404", http.StatusConflict, errors.New("Docker API returned 409: remove aurago-local-llm-key: volume is in use - [ab404cd9e1f2]"), true, true},
		{"unreachable engine on port 4040", 0, errors.New("Docker request: dial tcp 10.0.0.2:4040: connect: connection refused"), false, true},
		{"engine failure", http.StatusInternalServerError, errors.New("Docker API returned 500: driver failed"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{docker: &pullTestEngine{respond: func(string, string) (int, error) { return tc.code, tc.err }}}
			var err error
			if tc.volume {
				err = manager.deleteRuntimeKeyVolume(context.Background())
			} else {
				err = manager.deleteContainer(context.Background(), managedContainerName)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("delete error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRuntimeStopClassifiesByStatusCode(t *testing.T) {
	cases := []struct {
		name     string
		stopCode int
		stopErr  error
		wantFail bool
	}{
		{"already stopped", http.StatusNotModified, errors.New("Docker API returned 304: "), false},
		{"missing container", http.StatusNotFound, errors.New("Docker API returned 404: No such container"), false},
		{"conflict naming an ID containing 404", http.StatusConflict, errors.New("Docker API returned 409: container ab404cd9e1f2 is restarting"), true},
		{"unreachable engine on port 4040", 0, errors.New("Docker request: dial tcp 10.0.0.2:4040: connect: connection refused"), true},
	}
	for _, tc := range cases {
		respond := func(_, path string) (int, error) {
			if strings.Contains(path, "/stop") {
				return tc.stopCode, tc.stopErr
			}
			return http.StatusNotFound, errors.New("Docker API returned 404: No such container")
		}
		t.Run("stop/"+tc.name, func(t *testing.T) {
			manager := &Manager{docker: &pullTestEngine{respond: respond}, runtimeDir: t.TempDir()}
			want := ""
			if tc.wantFail {
				want = "container_stop_failed"
			}
			if got := codeOrEmpty(manager.stop(context.Background(), true)); got != want {
				t.Fatalf("stop() code = %q, want %q", got, want)
			}
		})
		t.Run("cleanup/"+tc.name, func(t *testing.T) {
			manager := &Manager{docker: &pullTestEngine{respond: respond}, runtimeDir: t.TempDir()}
			want := ""
			if tc.wantFail {
				want = "stale_runtime_stop_failed"
			}
			if got := codeOrEmpty(manager.CleanupStaleRuntime(context.Background())); got != want {
				t.Fatalf("CleanupStaleRuntime() code = %q, want %q", got, want)
			}
		})
	}
}

func TestPullImageHardensFailureDetail(t *testing.T) {
	t.Run("non-JSON error body is kept as the detail", func(t *testing.T) {
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusBadGateway, strings.NewReader("<html>bad gateway</html>"))}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if codeOrEmpty(err) != "pull_image_failed" || !strings.Contains(err.Error(), "Docker returned 502: <html>bad gateway</html>") {
			t.Fatalf("pullImage() error = %v, want pull_image_failed naming the 502 body", err)
		}
	})
	t.Run("an error event of only non-printable runes has no detail", func(t *testing.T) {
		// U+0001 and U+001B are controls, U+202E a bidi override, U+200B a
		// zero-width space; none is whitespace, so the event is still an error.
		body := `{"error":"\u0001\u001b\u202e\u200b"}` + "\n"
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusOK, strings.NewReader(body))}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if err == nil || err.Error() != "pull_image_failed" {
			t.Fatalf("pullImage() error = %v, want exactly pull_image_failed", err)
		}
	})
	t.Run("separators and bidi overrides cannot pass", func(t *testing.T) {
		body := `{"error":"denied\u2028token\u2029expired\u202e\u0000end"}` + "\n"
		manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(http.StatusOK, strings.NewReader(body))}}
		err := manager.pullImage(context.Background(), testRuntimeImage)
		if codeOrEmpty(err) != "pull_image_failed" {
			t.Fatalf("pullImage() error = %v, want pull_image_failed", err)
		}
		for _, r := range err.Error() {
			if !unicode.IsPrint(r) {
				t.Fatalf("pullImage() error = %q contains non-printable rune %U", err.Error(), r)
			}
		}
		for _, want := range []string{"denied", "token", "expired", "end"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("pullImage() error = %q, want it to keep %q", err.Error(), want)
			}
		}
	})
}

func TestPullImageCancellationStaysInspectable(t *testing.T) {
	firstLine := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"Downloading","id":"layer"}`+"\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case firstLine <- struct{}{}:
		default:
		}
		// A slow stream: nothing more arrives until the client gives up.
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-firstLine
		cancel()
	}()
	manager := &Manager{docker: dockerutil.NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Minute)}
	err := manager.pullImage(ctx, testRuntimeImage)
	if codeOrEmpty(err) != "pull_image_failed" || !errors.Is(err, context.Canceled) {
		t.Fatalf("pullImage() error = %v, want pull_image_failed wrapping context.Canceled", err)
	}
}

func TestLogPullFailureWritesReasonAndToleratesMissingLogger(t *testing.T) {
	var logged bytes.Buffer
	manager := &Manager{logger: slog.New(slog.NewTextHandler(&logged, nil))}
	manager.logPullFailure(testRuntimeImage, errors.New("pull_image_failed: no matching manifest for linux/amd64"))
	for _, want := range []string{"runtime image pull failed", testRuntimeImage, "no matching manifest for linux/amd64"} {
		if !strings.Contains(logged.String(), want) {
			t.Fatalf("log = %q, want it to contain %q", logged.String(), want)
		}
	}
	// A Manager built without NewManager has no logger; it must fall back to
	// the default logger instead of panicking.
	(&Manager{}).logPullFailure(testRuntimeImage, errors.New("pull_image_failed"))
}

func TestPullFailureTextIsByteStable(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"engine JSON 404", http.StatusNotFound, `{"message":"manifest unknown"}`, "pull_image_failed: Docker returned 404: manifest unknown"},
		{"HTML 502", http.StatusBadGateway, "<html>bad gateway</html>", "pull_image_failed: Docker returned 502: <html>bad gateway</html>"},
		{"blank JSON message falls back to the body", http.StatusInternalServerError, `{"message":"  "}`, `pull_image_failed: Docker returned 500: {"message":"  "}`},
		{"event with separators and NUL", http.StatusOK, `{"error":"denied\u2028token\u0000end"}` + "\n", "pull_image_failed: denied token end"},
		{"long event cut at a rune boundary", http.StatusOK, `{"error":"` + strings.Repeat("a", 255) + `é tail"}` + "\n", "pull_image_failed: " + strings.Repeat("a", 255)},
		{"event of only non-printable runes", http.StatusOK, `{"error":"\u0001\u202e"}` + "\n", "pull_image_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{docker: &pullTestEngine{client: pullStreamClient(tc.status, strings.NewReader(tc.body))}}
			err := manager.pullImage(context.Background(), testRuntimeImage)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("pullImage() = %v, want %q", err, tc.want)
			}
		})
	}
}
