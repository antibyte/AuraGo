package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/cydcreator"
)

type sseFrame struct {
	id, event, data string
	comment         bool
}

// openCYDEvents opens the event stream and parses it on one goroutine; the
// stream is closed when the test ends.
func openCYDEvents(t *testing.T, srv *httptest.Server, token, path string, lastID string) (<-chan sseFrame, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+cydCreatorAPIPrefix+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	closeStream := func() { cancel(); _ = resp.Body.Close() }
	t.Cleanup(closeStream)
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	frames := make(chan sseFrame, 1024)
	go func() {
		defer close(frames)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var cur sseFrame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if cur != (sseFrame{}) {
					frames <- cur
				}
				cur = sseFrame{}
			case strings.HasPrefix(line, ":"):
				cur.comment = true
			case strings.HasPrefix(line, "id: "):
				cur.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return frames, closeStream
}

// readSSE collects frames until stop returns true.
func readSSE(t *testing.T, frames <-chan sseFrame, timeout time.Duration, stop func(sseFrame) bool) []sseFrame {
	t.Helper()
	var out []sseFrame
	deadline := time.After(timeout)
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				t.Fatalf("stream ended after %d frames", len(out))
			}
			out = append(out, f)
			if stop(f) {
				return out
			}
		case <-deadline:
			t.Fatalf("stream timed out after %d frames", len(out))
		}
	}
}

func TestCYDCreatorEventsReplayResumeAndLive(t *testing.T) {
	f := newCYDTestServer(t)
	saved := cydSSEHeartbeat
	cydSSEHeartbeat = 50 * time.Millisecond
	t.Cleanup(func() { cydSSEHeartbeat = saved })
	p := f.createProject(t, "Stream")
	for i := 0; i < 600; i++ {
		f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"i": i})
	}
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close) // registered first, so the streams close before it

	stream, closeStream := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events?after=0", "")
	backlog := readSSE(t, stream, 10*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
	ready := backlog[len(backlog)-1]
	persisted := backlog[:len(backlog)-1]
	if want := cydReadyData(false, persisted[len(persisted)-1].id); ready.id != "" || ready.data != want {
		t.Fatalf("stream_ready = %+v, want no id and %s", ready, want)
	}
	if len(persisted) != 600 {
		t.Fatalf("backlog has %d events, want 600 (pages of 500 without gaps)", len(persisted))
	}
	for i := 1; i < len(persisted); i++ {
		a, _ := strconv.Atoi(persisted[i-1].id)
		b, _ := strconv.Atoi(persisted[i].id)
		if b <= a {
			t.Fatalf("ids not increasing at %d: %s then %s", i, persisted[i-1].id, persisted[i].id)
		}
	}
	resumeFrom := persisted[549].id

	f.svc.EmitEphemeral(p.ID, cydcreator.EventBuildLog, "", "ccb_1", map[string]any{"build_id": "ccb_1", "lines": []string{"hello"}})
	f.svc.Emit(p.ID, cydcreator.EventBuildStatus, "", "ccb_1", map[string]any{"build": map[string]string{"id": "ccb_1"}})
	live := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventBuildStatus })
	var sawLog, sawPing bool
	for _, fr := range live {
		if fr.event == cydcreator.EventBuildLog {
			sawLog = fr.id == "" && strings.Contains(fr.data, `"lines":["hello"]`)
		}
	}
	if !sawLog || live[len(live)-1].id == "" {
		t.Fatalf("live frames = %+v; want an id-less build_log and an identified build_status", live)
	}
	pings := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.comment })
	sawPing = len(pings) > 0 && pings[len(pings)-1].comment
	if !sawPing {
		t.Fatal("no heartbeat comment")
	}
	closeStream()

	stream, _ = openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events?after=10", resumeFrom)
	resumed := readSSE(t, stream, 10*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
	if len(resumed) != 52 || resumed[0].data != `{"i":550}` || resumed[50].event != cydcreator.EventBuildStatus {
		t.Fatalf("resumed backlog = %d frames (first %+v), want 50 changes, the build_status and stream_ready", len(resumed), resumed[0])
	}
	if want := cydReadyData(false, resumed[50].id); resumed[51].data != want {
		t.Fatalf("resumed stream_ready = %+v, want %s (the live loop continues after the build_status)", resumed[51], want)
	}
}

// cydReadyData is the stream_ready data the handler sends: the seq the live
// loop continues after, and reload when the client must reload its state.
func cydReadyData(reload bool, after string) string {
	if reload {
		return `{"reload":true,"after":` + after + `}`
	}
	return `{"after":` + after + `}`
}

func TestCYDCreatorEventsOfAnUnknownProject(t *testing.T) {
	f := newCYDTestServer(t)
	wantCYDError(t, f.do(f.read, http.MethodGet, "projects/ccp_missing/events", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
}

// The tests below pin the SSE requirements of the plan 2 reviews: reload
// after pruning, seq dedupe around the backlog, end of stream on a closed
// subscription and cancellation by the client.

func TestCYDCreatorEventsAskForAReloadWhenTheResumePointWasPruned(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Pruned")
	for i := 0; i < 20; i++ {
		f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"i": i})
	}
	all, err := f.svc.EventsAfter(context.Background(), p.ID, 0, 500)
	if err != nil || len(all) < 20 {
		t.Fatalf("events = %d %v", len(all), err)
	}
	// Simulate the per-project pruning: every event up to cut is gone.
	cut := all[len(all)-11].Seq
	if _, err := cydTestDB(t, f).Exec(`DELETE FROM cc_events WHERE project_id=? AND seq<=?`, p.ID, cut); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close)

	for _, tc := range []struct {
		name, query, lastID string
		reload              bool
		backlog             int
	}{
		{"pruned Last-Event-ID", "", strconv.FormatInt(all[2].Seq, 10), true, 10},
		{"pruned after", "?after=" + strconv.FormatInt(cut, 10), "", true, 10},
		{"kept resume point", "", strconv.FormatInt(all[len(all)-5].Seq, 10), false, 4},
		{"fresh client", "?after=0", "", false, 10},
	} {
		stream, closeStream := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events"+tc.query, tc.lastID)
		frames := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
		ready := frames[len(frames)-1]
		// Every case's backlog ends with the newest event; the live loop continues after it.
		want := cydReadyData(tc.reload, strconv.FormatInt(all[len(all)-1].Seq, 10))
		if ready.id != "" || ready.data != want || len(frames)-1 != tc.backlog {
			t.Errorf("%s: %d backlog frames, stream_ready %+v; want %d and data %s", tc.name, len(frames)-1, ready, tc.backlog, want)
		}
		closeStream()
	}
}

func TestCYDCreatorEventsNeverRepeatOrLoseAPersistedEvent(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Race")
	for i := 0; i < 300; i++ {
		f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"i": i})
	}
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close)

	// Emit while the stream subscribes and replays: an event can then reach
	// both the backlog and the live channel and must be sent once.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"live": i})
		}
	}()
	stream, _ := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events?after=0", "")
	wg.Wait()
	ready := false
	frames := readSSE(t, stream, 10*time.Second, func(fr sseFrame) bool {
		ready = ready || fr.event == cydcreator.EventStreamReady
		return ready
	})
	// Ephemeral events (no seq) are never deduplicated.
	for i := 0; i < 5; i++ {
		f.svc.EmitEphemeral(p.ID, cydcreator.EventBuildLog, "", "ccb_1", map[string]any{"build_id": "ccb_1", "lines": []string{strconv.Itoa(i)}})
	}
	f.svc.Emit(p.ID, cydcreator.EventBuildStatus, "", "ccb_1", map[string]any{"final": true})
	frames = append(frames, readSSE(t, stream, 10*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventBuildStatus })...)

	var stored []int64
	for after := int64(0); ; {
		page, err := f.svc.EventsAfter(context.Background(), p.ID, after, 500)
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range page {
			stored = append(stored, ev.Seq)
			after = ev.Seq
		}
		if len(page) < 500 {
			break
		}
	}
	var sent []int64
	logs := 0
	for _, fr := range frames {
		if fr.event == cydcreator.EventBuildLog {
			logs++
		}
		if fr.id == "" {
			continue
		}
		seq, _ := strconv.ParseInt(fr.id, 10, 64)
		if len(sent) > 0 && seq <= sent[len(sent)-1] {
			t.Fatalf("seq %d after %d: repeated or out of order", seq, sent[len(sent)-1])
		}
		sent = append(sent, seq)
	}
	if len(sent) != len(stored) || sent[0] != stored[0] || sent[len(sent)-1] != stored[len(stored)-1] {
		t.Fatalf("sent %d persisted events, stored %d", len(sent), len(stored))
	}
	if logs != 5 {
		t.Fatalf("ephemeral build_log frames = %d, want 5", logs)
	}
}

func TestCYDCreatorEventsEndWhenTheSubscriptionCloses(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Ends")
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close)
	stream, _ := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events", "")
	readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
	// Deleting the project closes its subscriptions (as Close does for all).
	if err := f.svc.DeleteProject(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-stream:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("the stream did not end after its subscription closed")
		}
	}
}

// flushSignalRecorder reports the first flush (stream_ready is out).
type flushSignalRecorder struct {
	*httptest.ResponseRecorder
	mu      sync.Mutex
	flushed chan struct{}
	once    sync.Once
}

func (r *flushSignalRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(p)
}

func (r *flushSignalRecorder) Flush() {
	r.mu.Lock()
	r.ResponseRecorder.Flush()
	r.mu.Unlock()
	r.once.Do(func() { close(r.flushed) })
}

func TestCYDCreatorEventsStopWhenTheClientGoesAway(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Gone")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, cydCreatorAPIPrefix+"projects/"+p.ID+"/events", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+f.read)
	w := &flushSignalRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.s.handleCYDCreator(w, r)
	}()
	select {
	case <-w.flushed:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream never became ready")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler kept running after the client went away")
	}
	// The subscription was released: a later event finds no subscriber to block.
	f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"after": "gone"})
}

func TestCYDCreatorEventsReloadWhenTheResumePointIsAboveTheNewestEvent(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Future")
	f.svc.Emit(p.ID, cydcreator.EventProjectChanged, "", "", map[string]any{"i": 1})
	newest, err := f.svc.LatestEventSeq(context.Background(), p.ID)
	if err != nil || newest == 0 {
		t.Fatalf("LatestEventSeq = %d, %v", newest, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close)
	for _, tc := range []struct{ name, query, lastID string }{
		{"huge Last-Event-ID", "", "9000000000000000000"},
		{"after above the newest", "?after=" + strconv.FormatInt(newest+1000, 10), ""},
	} {
		current, err := f.svc.LatestEventSeq(context.Background(), p.ID)
		if err != nil {
			t.Fatal(err)
		}
		stream, closeStream := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events"+tc.query, tc.lastID)
		frames := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
		// The stream continues after the newest event, which the reload tells the client.
		if want := cydReadyData(true, strconv.FormatInt(current, 10)); len(frames) != 1 || frames[0].data != want {
			t.Fatalf("%s: frames before live = %+v, want only stream_ready %s", tc.name, frames, want)
		}
		f.svc.Emit(p.ID, cydcreator.EventBuildStatus, "", "ccb_1", map[string]any{"case": tc.name})
		live := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventBuildStatus })
		if got := live[len(live)-1]; got.id == "" || !strings.Contains(got.data, tc.name) {
			t.Fatalf("%s: live build_status = %+v", tc.name, got)
		}
		closeStream()
	}
	for _, lastID := range []string{"-5", "abc", "1e9"} {
		stream, closeStream := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events", lastID)
		frames := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
		all, _ := f.svc.EventsAfter(context.Background(), p.ID, 0, 500)
		if ready := frames[len(frames)-1]; len(all) == 0 || ready.data != cydReadyData(false, strconv.FormatInt(all[len(all)-1].Seq, 10)) || len(frames)-1 != len(all) {
			t.Errorf("Last-Event-ID %q: %d backlog frames, ready %+v; want the whole backlog (%d) without reload", lastID, len(frames)-1, ready, len(all))
		}
		closeStream()
	}
}

func TestCYDCreatorEventsHeadAnswersHeadersOnly(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Head")
	w := f.do(f.read, http.MethodHead, "projects/"+p.ID+"/events", nil, nil)
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") || w.Body.Len() != 0 {
		t.Fatalf("HEAD events = %d %v %q", w.Code, w.Header(), w.Body.String())
	}
	wantCYDError(t, f.do(f.read, http.MethodHead, "projects/ccp_missing/events", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
}

func TestCYDCreatorEventDataStaysOnOneLine(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Lines")
	srv := httptest.NewServer(http.HandlerFunc(f.s.handleCYDCreator))
	t.Cleanup(srv.Close)
	stream, _ := openCYDEvents(t, srv, f.read, "projects/"+p.ID+"/events", "")
	readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventStreamReady })
	f.svc.Emit(p.ID, cydcreator.EventJobStep, "", "", json.RawMessage("{\n  \"text\": \"two\\nlines\"\r\n}"))
	got := readSSE(t, stream, 5*time.Second, func(fr sseFrame) bool { return fr.event == cydcreator.EventJobStep })
	if data := got[len(got)-1].data; data != `{"text":"two\nlines"}` {
		t.Fatalf("data line = %q", data)
	}
}

// cydStallingWriter is a response writer whose client stops reading after
// the stream became ready: every later write fails as a deadline would.
type cydStallingWriter struct {
	header    http.Header
	mu        sync.Mutex
	stalled   bool
	deadlines []time.Duration
	ready     chan struct{}
	once      sync.Once
}

func (w *cydStallingWriter) Header() http.Header { return w.header }
func (w *cydStallingWriter) WriteHeader(int)     {}

func (w *cydStallingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stalled {
		return 0, os.ErrDeadlineExceeded
	}
	return len(p), nil
}

func (w *cydStallingWriter) Flush() {
	w.once.Do(func() {
		w.mu.Lock()
		w.stalled = true
		w.mu.Unlock()
		close(w.ready)
	})
}

func (w *cydStallingWriter) SetWriteDeadline(t time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadlines = append(w.deadlines, time.Until(t))
	return nil
}

func TestCYDCreatorEventsEndWhenTheClientStopsReading(t *testing.T) {
	f := newCYDTestServer(t)
	saved := cydSSEHeartbeat
	cydSSEHeartbeat = 20 * time.Millisecond
	t.Cleanup(func() { cydSSEHeartbeat = saved })
	p := f.createProject(t, "Stall")
	r := httptest.NewRequest(http.MethodGet, cydCreatorAPIPrefix+"projects/"+p.ID+"/events", nil)
	r.Header.Set("Authorization", "Bearer "+f.read)
	w := &cydStallingWriter{header: http.Header{}, ready: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.s.handleCYDCreator(w, r) // the request context is never cancelled
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a client that stopped reading pinned the handler")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.deadlines) < 2 {
		t.Fatalf("write deadlines = %v, want one per frame", w.deadlines)
	}
	for _, d := range w.deadlines {
		if d < cydSSEWriteTimeout-5*time.Second || d > cydSSEWriteTimeout {
			t.Fatalf("write deadline %v from now, want about %v", d, cydSSEWriteTimeout)
		}
	}
}
