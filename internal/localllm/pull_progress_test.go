package localllm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

func setPullBudget(t *testing.T, baseline, window time.Duration) {
	t.Helper()
	oldBaseline, oldWindow := imagePullBaseline, imagePullStallWindow
	imagePullBaseline, imagePullStallWindow = baseline, window
	t.Cleanup(func() { imagePullBaseline, imagePullStallWindow = oldBaseline, oldWindow })
}

// pullStallServer serves the version probe and hands the pull to stream. The
// Engine client's own timeout (50 ms) is far below every pull here.
func pullStallServer(t *testing.T, stream func(w http.ResponseWriter, r *http.Request)) *Manager {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		stream(w, r)
	}))
	t.Cleanup(server.Close)
	return &Manager{docker: dockerutil.NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), 50*time.Millisecond)}
}

func writePullLine(w http.ResponseWriter, line string) {
	_, _ = io.WriteString(w, line+"\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func TestPullImageContinuesPastBaselineWhileProgressing(t *testing.T) {
	setPullBudget(t, 100*time.Millisecond, 600*time.Millisecond)
	manager := pullStallServer(t, func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 40; i++ { // about 800 ms, eight times the baseline
			writePullLine(w, fmt.Sprintf(`{"status":"Downloading","id":"layer","progressDetail":{"current":%d,"total":40}}`, i))
			time.Sleep(20 * time.Millisecond)
		}
		writePullLine(w, `{"status":"Status: Downloaded newer image"}`)
	})
	if err := manager.pullImage(context.Background(), testRuntimeImage); err != nil {
		t.Fatalf("pullImage() = %v, want a progressing pull to outlive the baseline", err)
	}
}

func TestPullImageFailsWhenProgressStallsAfterBaseline(t *testing.T) {
	setPullBudget(t, 100*time.Millisecond, 300*time.Millisecond)
	manager := pullStallServer(t, func(w http.ResponseWriter, r *http.Request) {
		// An Engine that keeps re-sending the same layer state while the registry stalls.
		for {
			writePullLine(w, `{"status":"Downloading","id":"layer","progressDetail":{"current":7,"total":40}}`)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	})
	started := time.Now()
	err := manager.pullImage(context.Background(), testRuntimeImage)
	if codeOrEmpty(err) != "pull_image_failed" || !strings.Contains(err.Error(), "no new pull progress") {
		t.Fatalf("pullImage() = %v, want pull_image_failed naming the stall", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("pullImage() = %v; a stall must not look like an intended cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("stall detected after %s, want shortly after baseline plus window", elapsed)
	}
}

func TestPullImageWithoutProgressStillFailsAtBaseline(t *testing.T) {
	setPullBudget(t, 150*time.Millisecond, 150*time.Millisecond)
	manager := pullStallServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	})
	started := time.Now()
	err := manager.pullImage(context.Background(), testRuntimeImage)
	if codeOrEmpty(err) != "pull_image_failed" || time.Since(started) > 3*time.Second {
		t.Fatalf("pullImage() = %v after %s, want pull_image_failed at the baseline", err, time.Since(started))
	}
}

// Pins the old fixed bound from the other side: a pull whose Engine never
// answers at all (no status line) fails at the baseline too, as the 2 h client
// timeout did. As in production, the window is shorter than the baseline.
func TestPullImageWithoutResponseFailsAtBaseline(t *testing.T) {
	setPullBudget(t, 150*time.Millisecond, 100*time.Millisecond)
	manager := pullStallServer(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // no status line, no headers
	})
	started := time.Now()
	err := manager.pullImage(context.Background(), testRuntimeImage)
	if codeOrEmpty(err) != "pull_image_failed" || errors.Is(err, context.Canceled) || time.Since(started) > 3*time.Second {
		t.Fatalf("pullImage() = %v after %s, want pull_image_failed at the baseline", err, time.Since(started))
	}
}

func TestPullImageNeverStallsInsideBaseline(t *testing.T) {
	setPullBudget(t, 3*time.Second, 20*time.Millisecond)
	manager := pullStallServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(400 * time.Millisecond) // silent for twenty stall windows
		writePullLine(w, `{"status":"Status: Downloaded newer image"}`)
	})
	if err := manager.pullImage(context.Background(), testRuntimeImage); err != nil {
		t.Fatalf("pullImage() = %v, want no stall rule inside the baseline", err)
	}
}

// Pins: the production budget keeps today's two hours unconditional.
func TestPullImageBaselineKeepsTodaysTwoHours(t *testing.T) {
	if imagePullBaseline != 2*time.Hour || imagePullStallWindow != 15*time.Minute {
		t.Fatalf("pull budget = %s baseline, %s stall window; want 2h and 15m", imagePullBaseline, imagePullStallWindow)
	}
}

// Pins: the pull never mutates the shared Engine client it copies.
func TestPullImageLeavesTheSharedClientTimeout(t *testing.T) {
	client := dockerutil.NewClient("tcp://127.0.0.1:1", 50*time.Millisecond)
	before := client.HTTPClient().Timeout
	manager := &Manager{docker: client}
	_ = manager.pullImage(context.Background(), testRuntimeImage)
	if after := client.HTTPClient().Timeout; after != before {
		t.Fatalf("shared client timeout = %s after the pull, want %s", after, before)
	}
}

func TestPullProgressCountsOnlyNewStates(t *testing.T) {
	start := time.Unix(1000, 0)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	progress := newPullProgress(start)
	feed := func(line string, s int) { progress.observe([]byte(line+"\n"), at(s)) }
	expect := func(s int) {
		t.Helper()
		if got := progress.lastProgress(); !got.Equal(at(s)) {
			t.Fatalf("last progress = %s, want %s", got, at(s))
		}
	}
	layerA1 := `{"status":"Downloading","id":"a","progressDetail":{"current":1,"total":9}}`
	feed(layerA1, 1)
	expect(1)
	feed(layerA1, 2) // identical state: no progress
	expect(1)
	feed(`not json`, 3)
	expect(1)
	feed(`{"status":"Downloading","id":"b","progressDetail":{"current":1,"total":9}}`, 4) // new layer
	expect(4)
	feed(`{"status":"Downloading","id":"a","progressDetail":{"current":2,"total":9}}`, 5)
	expect(5)
	feed(`{"status":"Status: Downloaded newer image"}`, 6)
	expect(6)
	progress.observe([]byte(`{"status":"Extracting","id":"a",`), at(7)) // half a line
	expect(6)
	progress.observe([]byte(`"progressDetail":{"current":1,"total":9}}`+"\n"), at(8))
	expect(8)
}

func TestPullProgressSkipsOverlongLines(t *testing.T) {
	start := time.Unix(1000, 0)
	progress := newPullProgress(start)
	long := `{"status":"Downloading","id":"x","progress":"` + strings.Repeat("=", dockerutil.MaxJSONMessageLine) + `"}`
	progress.observe([]byte(long[:len(long)/2]), start.Add(time.Second))
	progress.observe([]byte(long[len(long)/2:]+"\n"), start.Add(2*time.Second))
	if got := progress.lastProgress(); !got.Equal(start) {
		t.Fatalf("last progress = %s after an overlong line, want it unchanged", got)
	}
	progress.observe([]byte(`{"status":"Downloading","id":"x","progressDetail":{"current":1,"total":2}}`+"\n"), start.Add(3*time.Second))
	if got := progress.lastProgress(); !got.Equal(start.Add(3 * time.Second)) {
		t.Fatalf("last progress = %s, want the line after the overlong one to count", got)
	}
}
