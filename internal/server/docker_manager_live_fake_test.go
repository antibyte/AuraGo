package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerwatch"
)

// ── fake live hub ───────────────────────────────────────────────────────────

// dmFakeLiveClient is one stream client of the fake hub.
type dmFakeLiveClient struct {
	id       string
	messages chan dockerwatch.Message

	mu       sync.Mutex
	sub      dockerwatch.Subscription
	sets     int
	setErr   error
	accepted dockerwatch.Accepted
	closed   chan struct{}
	once     sync.Once
}

func newDMFakeLiveClient() *dmFakeLiveClient {
	return &dmFakeLiveClient{id: strings.Repeat("ab", 16), messages: make(chan dockerwatch.Message, 16), closed: make(chan struct{})}
}

func (c *dmFakeLiveClient) ID() string                           { return c.id }
func (c *dmFakeLiveClient) Messages() <-chan dockerwatch.Message { return c.messages }
func (c *dmFakeLiveClient) Close()                               { c.once.Do(func() { close(c.closed) }) }

func (c *dmFakeLiveClient) Set(sub dockerwatch.Subscription) (dockerwatch.Accepted, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	if c.setErr != nil {
		return dockerwatch.Accepted{}, c.setErr
	}
	c.sub = sub
	return c.accepted, nil
}

func (c *dmFakeLiveClient) subscription() dockerwatch.Subscription {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sub
}

// dmFollowCall is what a fake follower was asked to do.
type dmFollowCall struct {
	host, ref string
	sel       dockerwatch.ProjectSelector
	opt       dockerwatch.FollowOptions
}

// dmFakeLive stands in for the hub in the handler tests. A follower runs the
// function the test sets; by default it reports the stream open and ends.
type dmFakeLive struct {
	mu sync.Mutex

	client    *dmFakeLiveClient
	attachErr error

	acquireErr error
	acquired   int
	released   int

	follow func(ctx context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error
	calls  []dmFollowCall

	usage     dockerhost.DiskUsage
	usageAt   time.Time
	cached    bool
	dfErr     error
	dfRefresh []bool
}

func newDMFakeLive() *dmFakeLive { return &dmFakeLive{client: newDMFakeLiveClient()} }

func (l *dmFakeLive) Attach() (dmLiveClient, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.attachErr != nil {
		return nil, l.attachErr
	}
	return l.client, nil
}

func (l *dmFakeLive) Client(id string) (dmLiveClient, bool) {
	if id != l.client.id {
		return nil, false
	}
	return l.client, true
}

func (l *dmFakeLive) Acquire(string) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.acquireErr != nil {
		return nil, l.acquireErr
	}
	l.acquired++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.released++
	}, nil
}

func (l *dmFakeLive) DiskUsage(_ context.Context, _ string, refresh bool) (dockerhost.DiskUsage, time.Time, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dfRefresh = append(l.dfRefresh, refresh)
	return l.usage, l.usageAt, l.cached, l.dfErr
}

func (l *dmFakeLive) run(ctx context.Context, call dmFollowCall, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
	l.mu.Lock()
	l.calls = append(l.calls, call)
	follow := l.follow
	l.mu.Unlock()
	if follow == nil {
		if call.opt.Opened != nil {
			call.opt.Opened()
		}
		return nil
	}
	return follow(ctx, call, out, notice)
}

func (l *dmFakeLive) FollowContainer(ctx context.Context, host, ref string, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
	return l.run(ctx, dmFollowCall{host: host, ref: ref, opt: opt}, out, notice)
}

func (l *dmFakeLive) FollowProject(ctx context.Context, host string, sel dockerwatch.ProjectSelector, opt dockerwatch.FollowOptions, out func(dockerwatch.LogBatch) error, notice func(dockerwatch.Notice) error) error {
	return l.run(ctx, dmFollowCall{host: host, sel: sel, opt: opt}, out, notice)
}

func (l *dmFakeLive) lastCall(t *testing.T) dmFollowCall {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) == 0 {
		t.Fatal("no follower was started")
	}
	return l.calls[len(l.calls)-1]
}

func (l *dmFakeLive) slots() (acquired, released int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.acquired, l.released
}

var _ dmLive = (*dmFakeLive)(nil)

// ── fixture ─────────────────────────────────────────────────────────────────

// dmLiveFixture is the Docker Manager server of newDMFixture (host registry,
// fake Engines) with the fake hub and the fake stacks service routed in, served
// over HTTP so that event streams can be read while they are open.
type dmLiveFixture struct {
	*dmFixture
	live     *dmFakeLive
	stacks   *dmFakeStacks
	noStacks bool
	readOnly bool
	srv      *httptest.Server
}

func newDMLiveFixture(t *testing.T) *dmLiveFixture {
	t.Helper()
	f := &dmLiveFixture{dmFixture: newDMFixture(t), live: newDMFakeLive(), stacks: newDMFakeStacks()}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dm := &dmRequest{s: f.s, w: w, r: r, readOnly: f.readOnly, live: f.live}
		if !f.noStacks {
			dm.stacks = f.stacks
		}
		dm.route(dockerManagerSegments(r.URL.Path))
	}))
	t.Cleanup(f.srv.Close)
	t.Cleanup(f.srv.CloseClientConnections)
	return f
}

// dmStream is an open event stream response. Reads are bounded by the context
// of the request, so a missing frame fails the test instead of hanging it.
type dmStream struct {
	t      *testing.T
	resp   *http.Response
	reader *bufio.Reader
	cancel context.CancelFunc
}

// dmFrame is one event of a stream: its name and data, or a comment line.
type dmFrame struct {
	event, data, comment string
}

func (f dmFrame) json(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(f.data), &out); err != nil {
		t.Fatalf("data of event %q is not a JSON object: %q", f.event, f.data)
	}
	return out
}

// open sends a request to the fixture's server and returns the response.
func (f *dmLiveFixture) open(method, path string, body io.Reader) *dmStream {
	f.t.Helper()
	return dmOpenStream(f.t, method, f.srv.URL+"/api/docker-manager"+path, body, nil)
}

// dmOpenStream sends a request and returns the open response; the stream is
// closed when the test ends.
func dmOpenStream(t *testing.T, method, url string, body io.Reader, headers map[string]string) *dmStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("%s %s: %v", method, url, err)
	}
	st := &dmStream{t: t, resp: resp, reader: bufio.NewReader(resp.Body), cancel: cancel}
	t.Cleanup(st.close)
	return st
}

func (s *dmStream) close() {
	s.cancel()
	_ = s.resp.Body.Close()
}

// next reads the next frame (an event with its data, or a comment line).
func (s *dmStream) next() dmFrame {
	s.t.Helper()
	var frame dmFrame
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			s.t.Fatalf("stream ended while waiting for a frame: %v (partial %+v)", err, frame)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if frame != (dmFrame{}) {
				return frame
			}
		case strings.HasPrefix(line, ": "):
			frame.comment = strings.TrimPrefix(line, ": ")
		case strings.HasPrefix(line, "event: "):
			frame.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			frame.data = strings.TrimPrefix(line, "data: ")
		}
	}
}

// nextEvent skips comment lines and returns the next event.
func (s *dmStream) nextEvent() dmFrame {
	s.t.Helper()
	for {
		if frame := s.next(); frame.event != "" {
			return frame
		}
	}
}

// expectEnd checks that the stream has ended (EOF).
func (s *dmStream) expectEnd() {
	s.t.Helper()
	if line, err := s.reader.ReadString('\n'); err != io.EOF {
		s.t.Fatalf("stream = %q, %v; want the end of the stream", line, err)
	}
}

// body reads the whole (non-streaming) answer as JSON.
func (s *dmStream) body() map[string]any {
	s.t.Helper()
	raw, err := io.ReadAll(s.reader)
	if err != nil {
		s.t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		s.t.Fatalf("answer %q is not JSON: %v", raw, err)
	}
	return out
}

// expectError checks a JSON error answer.
func (s *dmStream) expectError(status int, code string) map[string]any {
	s.t.Helper()
	body := s.body()
	if s.resp.StatusCode != status || body["status"] != "error" || body["code"] != code {
		s.t.Fatalf("answer = %d %v, want %d error %q", s.resp.StatusCode, body, status, code)
	}
	return body
}

// withHeartbeat shortens the idle heartbeat of the event streams for a test.
func withHeartbeat(t *testing.T, d time.Duration) {
	t.Helper()
	old := dmSSEHeartbeat
	dmSSEHeartbeat = d
	t.Cleanup(func() { dmSSEHeartbeat = old })
}
