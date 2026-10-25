package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

// The Docker sidecar restart is the slowest one: the Engine restarts the
// container and the model loads. A caller that gives up during it must return
// at once, a second caller must not queue behind the lock past its own
// deadline, and the restart has to finish for the next call to use, exactly
// once.
func TestDockerLlamaRestartOutlivesCallersAndHoldsTheLockUntilDone(t *testing.T) {
	prefix := "/" + dockerutil.APIVersion
	restartBegun := make(chan struct{}, 4)
	releaseRestart := make(chan struct{})
	openRestart := sync.OnceFunc(func() { close(releaseRestart) })
	var restarts, embedRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == prefix+"/containers/sidecar/restart":
			restarts.Add(1)
			restartBegun <- struct{}{}
			select {
			case <-releaseRestart:
				w.WriteHeader(http.StatusNoContent)
			case <-r.Context().Done():
			}
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/embeddings":
			embedRequests.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"index": 0, "embedding": unitTestVector()}},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(openRestart) // a failed assertion must not leave the handler blocking server.Close
	embedder := &dockerLlamaEmbedder{
		docker:       newDockerAPIClient("tcp://" + strings.TrimPrefix(server.URL, "http://")),
		containerID:  "sidecar",
		baseURL:      server.URL,
		client:       server.Client(),
		needsRestart: true, // the previous request failed: the next call restarts the sidecar
		logger:       discardEmbeddingLogger(),
	}
	texts := []string{"text"}
	embedWithin := func(within time.Duration) error {
		ctx, cancel := context.WithTimeout(context.Background(), within)
		defer cancel()
		begin := time.Now()
		_, err := embedder.Embed(ctx, texts)
		if elapsed := time.Since(begin); elapsed > 3*time.Second {
			t.Errorf("Embed took %s to honor its %s deadline", elapsed, within)
		}
		return err
	}

	// The first caller starts the restart and gives up while it runs.
	if err := embedWithin(100 * time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller that started the restart = %v, want its deadline error", err)
	}
	select {
	case <-restartBegun:
	case <-time.After(5 * time.Second):
		t.Fatal("the Docker restart never began")
	}
	// The restart still owns the embedder: a second caller leaves at its own
	// deadline instead of queueing behind the lock, and starts nothing.
	if err := embedWithin(50 * time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller waiting for the lock = %v, want its deadline error", err)
	}
	if restarts.Load() != 1 || embedder.restarts.remaining() != maxProcessRestarts-1 {
		t.Fatalf("restarts=%d budget=%d, want one restart paid once", restarts.Load(), embedder.restarts.remaining())
	}

	// Once the Engine finishes, the next call uses the restarted sidecar.
	openRestart()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	vectors, err := embedder.Embed(ctx, texts)
	if err != nil || len(vectors) != 1 {
		t.Fatalf("call after the restart = %v, %v; want the restarted sidecar to answer", vectors, err)
	}
	if restarts.Load() != 1 || embedRequests.Load() != 1 || embedder.restarts.remaining() != maxProcessRestarts-1 {
		t.Fatalf("restarts=%d embeds=%d budget=%d, want the restarted sidecar reused", restarts.Load(), embedRequests.Load(), embedder.restarts.remaining())
	}
}

// returnsWithin runs call on its own goroutine and fails the test when it is
// still blocked after bound; the goroutine is left to the test's cleanup.
func returnsWithin(t *testing.T, name string, bound time.Duration, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
	case <-time.After(bound):
		t.Fatalf("%s was still blocked after %s", name, bound)
	}
}

// A restart detached from its caller holds the embedder lock for as long as it
// runs, up to minutes. Close must not wait that long (a shutdown or a backend
// swap would hang behind it): it cancels the start first, so the sidecar is
// removed within seconds. Status must not wait either (config UI polls it).
func TestDockerLlamaCloseCancelsARunningRestartAndStatusDoesNotWaitForIt(t *testing.T) {
	prefix := "/" + dockerutil.APIVersion
	restartBegun := make(chan struct{}, 4)
	releaseRestart := make(chan struct{})
	openRestart := sync.OnceFunc(func() { close(releaseRestart) })
	var restarts, removals atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == prefix+"/containers/sidecar/restart":
			restarts.Add(1)
			restartBegun <- struct{}{}
			// A model load that never ends: only the client giving up stops it.
			select {
			case <-releaseRestart:
				w.WriteHeader(http.StatusNoContent)
			case <-r.Context().Done():
			}
		case r.Method == http.MethodPost && r.URL.Path == prefix+"/containers/sidecar/stop":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == prefix+"/containers/sidecar":
			removals.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(openRestart) // a failed assertion must not leave the handler blocking server.Close
	embedder := &dockerLlamaEmbedder{
		docker:       newDockerAPIClient("tcp://" + strings.TrimPrefix(server.URL, "http://")),
		containerID:  "sidecar",
		baseURL:      server.URL,
		client:       server.Client(),
		needsRestart: true,
		logger:       discardEmbeddingLogger(),
	}

	// The caller starts the restart and gives up; the restart goes on.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := embedder.Embed(ctx, []string{"text"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller that started the restart = %v, want its deadline error", err)
	}
	select {
	case <-restartBegun:
	case <-time.After(5 * time.Second):
		t.Fatal("the Docker restart never began")
	}

	var status Status
	returnsWithin(t, "Status during the restart", 2*time.Second, func() { status = embedder.Status() })
	if status.State != llamaStateRestarting {
		t.Fatalf("Status during the restart = %q, want %q", status.State, llamaStateRestarting)
	}

	var closeErr error
	returnsWithin(t, "Close during the restart", 2*time.Second, func() { closeErr = embedder.Close() })
	if closeErr != nil {
		t.Fatalf("Close = %v", closeErr)
	}
	if removals.Load() != 1 {
		t.Fatalf("sidecar removals = %d, want the sidecar removed once", removals.Load())
	}
	if state := embedder.Status().State; state != "closed" {
		t.Fatalf("Status after Close = %q, want closed", state)
	}
	if _, err := embedder.Embed(context.Background(), []string{"text"}); err == nil || restarts.Load() != 1 {
		t.Fatalf("Embed after Close = %v with %d restarts, want an error and no new restart", err, restarts.Load())
	}
}
