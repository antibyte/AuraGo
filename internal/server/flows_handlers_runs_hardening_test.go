package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/security"
)

// c18SSEEvent is one parsed server-sent event.
type c18SSEEvent struct {
	name, id, data string
}

// c18ParseSSE splits a text/event-stream body into its events and counts the comments.
func c18ParseSSE(body string) (events []c18SSEEvent, comments int) {
	for _, block := range strings.Split(body, "\n\n") {
		var ev c18SSEEvent
		for _, line := range strings.Split(block, "\n") {
			switch {
			case line == "":
			case strings.HasPrefix(line, ":"):
				comments++
			case strings.HasPrefix(line, "id: "):
				ev.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			}
		}
		if ev.name != "" {
			events = append(events, ev)
		}
	}
	return events, comments
}

// c18Get sends a GET through handleFlows with extra headers.
func c18Get(t *testing.T, s *Server, token, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	return w
}

// c18StartTestRun starts a test run of rec through the API and returns its id.
func c18StartTestRun(t *testing.T, s *Server, token string, rec *flows.FlowRecord, body string) string {
	t.Helper()
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/test", token, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("test run = %d %s", w.Code, w.Body.String())
	}
	return flowsBody(t, w)["run_id"].(string)
}

// c18EventSeqs returns the ids of the "event" events and checks that each carries its seq.
func c18EventSeqs(t *testing.T, events []c18SSEEvent) []int {
	t.Helper()
	var seqs []int
	for _, ev := range events {
		if ev.name != "event" {
			continue
		}
		var data struct {
			Seq int `json:"seq"`
		}
		if err := json.Unmarshal([]byte(ev.data), &data); err != nil {
			t.Fatalf("event data %q: %v", ev.data, err)
		}
		if strconv.Itoa(data.Seq) != ev.id {
			t.Fatalf("event id %q carries seq %d", ev.id, data.Seq)
		}
		seqs = append(seqs, data.Seq)
	}
	return seqs
}

// c18SetHeartbeat shortens the stream heartbeat for one test. Set it before a test server
// starts serving: the handler goroutines then see the value.
func c18SetHeartbeat(t *testing.T, d time.Duration) {
	t.Helper()
	old := flowStreamHeartbeat
	flowStreamHeartbeat = d
	t.Cleanup(func() { flowStreamHeartbeat = old })
}

func TestC18FinishedRunStreamEndsAtOnceAndHonoursLastEventID(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	runID := c18StartTestRun(t, s, token, rec, "")
	waitFlowRunStatus(t, s, token, runID, "success")
	path := "/api/desktop/flows/runs/" + runID + "/events"

	full, _ := c18ParseSSE(c18Get(t, s, token, path, nil).Body.String())
	all := c18EventSeqs(t, full)
	if len(full) < 3 || full[0].name != "snapshot" || full[len(full)-1].name != "end" || len(all) < 3 {
		t.Fatalf("full stream = %+v", full)
	}
	for i, seq := range all {
		if seq != i+1 {
			t.Fatalf("seqs = %v", all)
		}
	}
	final := all[len(all)-1]

	for _, tc := range []struct {
		name   string
		query  string
		header string
		first  int
	}{
		{"Last-Event-ID alone", "", "2", 3},
		{"the larger of after and Last-Event-ID", "?after=1", "3", 4},
		{"after larger than Last-Event-ID", "?after=3", "1", 4},
		{"negative Last-Event-ID", "", "-5", 1},
		{"garbage Last-Event-ID", "", "abc", 1},
		{"Last-Event-ID beyond the bound", "", strconv.Itoa(flowStreamMaxSeq + 1), 1},
		{"garbage after, valid Last-Event-ID", "?after=x", "2", 3},
	} {
		w := c18Get(t, s, token, path+tc.query, map[string]string{"Last-Event-ID": tc.header})
		events, _ := c18ParseSSE(w.Body.String())
		seqs := c18EventSeqs(t, events)
		if w.Code != http.StatusOK || len(seqs) == 0 || seqs[0] != tc.first || events[len(events)-1].name != "end" {
			t.Errorf("%s: %d %+v", tc.name, w.Code, events)
		}
	}

	// A reconnect after the last event gets snapshot and end at once, without a resync loop
	// and without waiting for a heartbeat.
	started := time.Now()
	w := c18Get(t, s, token, path+"?after="+strconv.Itoa(final), nil)
	events, comments := c18ParseSSE(w.Body.String())
	if len(events) != 2 || events[0].name != "snapshot" || events[1].name != "end" || comments != 0 || time.Since(started) > 5*time.Second {
		t.Fatalf("reconnect after the end = %+v (%d comments, %v)", events, comments, time.Since(started))
	}

	// A service whose bus does not know the run (forgotten after its retention, or a
	// restart) answers with the stored snapshot and end.
	c17SwapBridge(t, s, flowMissionBridge{s: s})
	events, _ = c18ParseSSE(c18Get(t, s, token, path, nil).Body.String())
	if len(events) != 2 || events[0].name != "snapshot" || events[1].name != "end" {
		t.Fatalf("stream of a run the bus forgot = %+v", events)
	}
	if s.flowStreams.total != 0 || len(s.flowStreams.runs) != 0 {
		t.Fatalf("finished streams were not released: %+v", s.flowStreams.runs)
	}
}

func TestC18StreamOutlivesTheWriteTimeoutAndEndsOnDrain(t *testing.T) {
	c18SetHeartbeat(t, 40*time.Millisecond)
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, waitFlowJSON)
	runID := c18StartTestRun(t, s, token, rec, "")
	t.Cleanup(func() { s.Flows.Cancel(runID) })

	// The production chain: trackHTTP (drain) around httpstream (write deadline renewal).
	srv := httptest.NewUnstartedServer(s.trackHTTP(newAgentHTTPServer("", http.HandlerFunc(s.handleFlows)).Handler))
	srv.Config.WriteTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/desktop/flows/runs/"+runID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	lines := make(chan string, 4096)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	next := func() (string, bool) {
		select {
		case line, ok := <-lines:
			return line, ok
		case <-time.After(5 * time.Second):
			t.Fatal("the stream went silent")
			return "", false
		}
	}
	opened := time.Now()
	snapshot, heartbeats := false, 0
	for time.Since(opened) < 3*srv.Config.WriteTimeout || heartbeats < 3 {
		line, ok := next()
		if !ok {
			t.Fatalf("the stream ended after %v (snapshot %v, %d heartbeats)", time.Since(opened), snapshot, heartbeats)
		}
		switch {
		case line == "event: snapshot":
			snapshot = true
		case line == ":heartbeat":
			heartbeats++
		case line == "event: end" || line == "event: resync":
			t.Fatalf("the stream of a running run sent %q", line)
		}
	}
	if !snapshot {
		t.Fatal("no snapshot")
	}

	drained := make(chan struct{})
	go func() {
		s.beginHTTPDrain()
		s.httpRequests.Wait()
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("the open run stream held up the drain")
	}
	for {
		if _, ok := next(); !ok {
			break
		}
	}
	if s.flowStreams.total != 0 {
		t.Fatalf("the drained stream was not released: %d", s.flowStreams.total)
	}
}

// c18BlockingWriter is a streaming ResponseWriter whose first Write waits until release is
// closed; blocked is closed when that Write starts.
type c18BlockingWriter struct {
	header  http.Header
	once    sync.Once
	blocked chan struct{}
	release chan struct{}
	mu      sync.Mutex
	buf     bytes.Buffer
}

func newC18BlockingWriter() *c18BlockingWriter {
	return &c18BlockingWriter{header: http.Header{}, blocked: make(chan struct{}), release: make(chan struct{})}
}

func (w *c18BlockingWriter) Header() http.Header { return w.header }
func (w *c18BlockingWriter) WriteHeader(int)     {}
func (w *c18BlockingWriter) Flush()              {}

func (w *c18BlockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.blocked)
		<-w.release
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *c18BlockingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestC18SlowClientGetsResyncNotEnd: the bus drops a subscriber that falls more than 256
// events behind. The stream must then tell the client to reconnect, not that the run ended,
// and the reconnect continues without a gap.
func TestC18SlowClientGetsResyncNotEnd(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, waitFlowJSON)
	runID := c18StartTestRun(t, s, token, rec, "")
	t.Cleanup(func() { s.Flows.Cancel(runID) })
	waitFlowRunStatus(t, s, token, runID, "running")
	path := "/api/desktop/flows/runs/" + runID + "/events"

	w := newC18BlockingWriter()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleFlows(w, r)
	}()
	select {
	case <-w.blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream never wrote")
	}
	// While the client does not read, the run publishes more than the subscriber buffer.
	const first, count = 1000, 300
	bus := s.Flows.Runner().Bus()
	for i := 0; i < count; i++ {
		bus.Publish(flows.RunEvent{Seq: first + i, RunID: runID, Type: flows.EventStepStarted, NodeID: "n_bbbbbbbb", Time: time.Now()})
	}
	close(w.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not end after the bus dropped it")
	}
	events, _ := c18ParseSSE(w.String())
	last := events[len(events)-1]
	seqs := c18EventSeqs(t, events)
	if last.name != "resync" || strings.Contains(w.String(), "event: end") {
		t.Fatalf("a dropped stream must resync, got %+v", last)
	}
	var resync struct {
		After int `json:"after"`
	}
	if err := json.Unmarshal([]byte(last.data), &resync); err != nil || resync.After != seqs[len(seqs)-1] || resync.After >= first+count-1 {
		t.Fatalf("resync = %q (%v), last seq sent %d", last.data, err, seqs[len(seqs)-1])
	}

	// The reconnect with ?after= continues with the next event.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	again := httptest.NewRequest(http.MethodGet, path+"?after="+strconv.Itoa(resync.After), nil).WithContext(ctx)
	again.Header.Set("Authorization", "Bearer "+token)
	rw := httptest.NewRecorder()
	s.handleFlows(rw, again)
	events, _ = c18ParseSSE(rw.Body.String())
	more := c18EventSeqs(t, events)
	if len(more) != first+count-1-resync.After || more[0] != resync.After+1 || more[len(more)-1] != first+count-1 {
		t.Fatalf("reconnect after %d got %d events (%v…)", resync.After, len(more), more[:min(len(more), 3)])
	}
}

func TestC18StreamsAreLimitedPerRunAndInTotal(t *testing.T) {
	s, token := newFlowsTestServer(t)
	path := "/api/desktop/flows/runs/run_c18limit0001/events"
	for i := 0; i < flowStreamsPerRun; i++ {
		if !s.flowStreams.acquire("run_c18limit0001") {
			t.Fatalf("stream %d refused", i)
		}
	}
	if w := c18Get(t, s, token, path, nil); w.Code != http.StatusTooManyRequests || flowsBody(t, w)["code"] != "FLOW_RUN_LIMIT" {
		t.Fatalf("stream beyond the per-run limit = %d %s", w.Code, w.Body.String())
	}
	s.flowStreams.release("run_c18limit0001")
	if w := c18Get(t, s, token, path, nil); w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_RUN_NOT_FOUND" {
		t.Fatalf("stream within the limit of an unknown run = %d %s", w.Code, w.Body.String())
	}
	for i := flowStreamsPerRun - 1; i < flowStreamsTotal; i++ {
		if !s.flowStreams.acquire(fmt.Sprintf("run_c18other%04d", i)) {
			t.Fatalf("stream %d of another run refused", i)
		}
	}
	if w := c18Get(t, s, token, "/api/desktop/flows/runs/run_c18fresh0001/events", nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("stream beyond the total limit = %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < flowStreamsPerRun-1; i++ {
		s.flowStreams.release("run_c18limit0001")
	}
	for i := flowStreamsPerRun - 1; i < flowStreamsTotal; i++ {
		s.flowStreams.release(fmt.Sprintf("run_c18other%04d", i))
	}
	if s.flowStreams.total != 0 || len(s.flowStreams.runs) != 0 {
		t.Fatalf("limiter after all releases = %d %v", s.flowStreams.total, s.flowStreams.runs)
	}
}

func TestC18StreamScrubsQuotedSecretsByValue(t *testing.T) {
	secret := fmt.Sprintf("c18 \"quoted\" \\ secret\t<%d>", time.Now().UnixNano())
	security.RegisterSensitive(secret)
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	body, _ := json.Marshal(map[string]any{"trigger_data": map[string]any{"name": secret}})
	runID := c18StartTestRun(t, s, token, rec, string(body))
	waitFlowRunStatus(t, s, token, runID, "success")
	w := c18Get(t, s, token, "/api/desktop/flows/runs/"+runID+"/events", nil)
	escaped, _ := json.Marshal(secret)
	inner := strings.Trim(string(escaped), `"`)
	if strings.Contains(w.Body.String(), inner) || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("the secret is in the stream: %s", w.Body.String())
	}
	events, _ := c18ParseSSE(w.Body.String())
	redacted := 0
	for _, ev := range events {
		if !json.Valid([]byte(ev.data)) {
			t.Fatalf("%s data is not JSON: %s", ev.name, ev.data)
		}
		if strings.Contains(ev.data, security.RedactedText("")) {
			redacted++
		}
	}
	// The snapshot (trigger data and the step outputs) and the step_finished events.
	if redacted < 2 {
		t.Fatalf("no redaction in the stream: %s", w.Body.String())
	}
}
