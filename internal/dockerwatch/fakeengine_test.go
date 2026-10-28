package dockerwatch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

var reStatsPath = regexp.MustCompile(`^/v1\.\d+/containers/([^/]+)/stats$`)

// fakeEngine is an httptest Docker Engine with the routes the hub uses:
// /version, /info, /containers/json, /events (a feed the test controls),
// /containers/{id}/stats and /system/df.
type fakeEngine struct {
	server *httptest.Server

	mu         sync.Mutex
	api        string // reported ApiVersion
	containers string // body of /containers/json
	info       string
	df         string
	infoCalls  int
	listCalls  int
	dfCalls    int

	eventsStatus int  // non-zero: /events answers this status
	eventsHang   bool // /events never answers
	eventsEnd    bool // /events answers 200 and ends the stream at once
	eventConns   int
	feeds        []*eventFeed

	stats        map[string]func(call int) (int, string)
	statsCalls   map[string]int
	statsQueries []string
	statsDelay   time.Duration
	inflight     int
	maxInflight  int
}

type eventFeed struct {
	lines chan string
	drop  chan struct{}
}

func newFakeEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{
		api:        "1.45",
		containers: "[]",
		info:       `{"Name":"nas","NCPU":4,"MemTotal":8589934592}`,
		df:         `{"LayersSize":0,"Images":null,"Containers":null,"Volumes":null,"BuildCache":null}`,
		stats:      map[string]func(int) (int, string){},
		statsCalls: map[string]int{},
	}
	fe.server = testutil.NewHTTPServer(t, http.HandlerFunc(fe.serve))
	t.Cleanup(fe.server.Close)
	return fe
}

func (fe *fakeEngine) client(t *testing.T) *dockerutil.Client {
	t.Helper()
	c := dockerutil.NewClient("tcp://127.0.0.1:"+strconv.Itoa(fe.server.Listener.Addr().(*net.TCPAddr).Port), 5*time.Second)
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func (fe *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/version"):
		fe.mu.Lock()
		api := fe.api
		fe.mu.Unlock()
		fmt.Fprintf(w, `{"Version":"27.3.1","ApiVersion":%q,"MinAPIVersion":"1.24"}`, api)
	case strings.HasSuffix(path, "/info"):
		fe.mu.Lock()
		fe.infoCalls++
		body := fe.info
		fe.mu.Unlock()
		fmt.Fprint(w, body)
	case strings.HasSuffix(path, "/containers/json"):
		fe.mu.Lock()
		fe.listCalls++
		body := fe.containers
		fe.mu.Unlock()
		fmt.Fprint(w, body)
	case strings.HasSuffix(path, "/system/df"):
		fe.mu.Lock()
		fe.dfCalls++
		body := fe.df
		fe.mu.Unlock()
		fmt.Fprint(w, body)
	case strings.HasSuffix(path, "/events"):
		fe.serveEvents(w, r)
	case reStatsPath.MatchString(path):
		fe.serveStats(w, r, reStatsPath.FindStringSubmatch(path)[1])
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"page not found"}`)
	}
}

func (fe *fakeEngine) serveEvents(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	fe.eventConns++
	status, hang, end := fe.eventsStatus, fe.eventsHang, fe.eventsEnd
	fe.mu.Unlock()
	switch {
	case hang:
		<-r.Context().Done()
		return
	case status != 0:
		w.WriteHeader(status)
		fmt.Fprint(w, `{"message":"events are not available"}`)
		return
	}
	// The feed is registered before the client can see the answer, so a test
	// that has seen the stream open can always write to it.
	feed := &eventFeed{lines: make(chan string, 64), drop: make(chan struct{})}
	if !end {
		fe.mu.Lock()
		fe.feeds = append(fe.feeds, feed)
		fe.mu.Unlock()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flusher := w.(http.Flusher)
	flusher.Flush()
	if end {
		return
	}
	for {
		select {
		case line := <-feed.lines:
			io.WriteString(w, line+"\n")
			flusher.Flush()
		case <-feed.drop:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (fe *fakeEngine) serveStats(w http.ResponseWriter, r *http.Request, id string) {
	fe.mu.Lock()
	fe.statsCalls[id]++
	call := fe.statsCalls[id]
	fe.statsQueries = append(fe.statsQueries, r.URL.RawQuery)
	gen := fe.stats[id]
	delay := fe.statsDelay
	fe.inflight++
	fe.maxInflight = max(fe.maxInflight, fe.inflight)
	fe.mu.Unlock()
	defer func() {
		fe.mu.Lock()
		fe.inflight--
		fe.mu.Unlock()
	}()
	if delay > 0 {
		time.Sleep(delay)
	}
	if gen == nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"No such container"}`)
		return
	}
	status, body := gen(call)
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

func (fe *fakeEngine) set(f func(fe *fakeEngine)) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	f(fe)
}

func (fe *fakeEngine) conns() int {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return fe.eventConns
}

func (fe *fakeEngine) calls(id string) int {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return fe.statsCalls[id]
}

// send writes a line on the newest open events feed.
func (fe *fakeEngine) send(t *testing.T, lines ...string) {
	t.Helper()
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if len(fe.feeds) == 0 {
		t.Fatal("no events feed is open")
	}
	feed := fe.feeds[len(fe.feeds)-1]
	for _, line := range lines {
		feed.lines <- line
	}
}

// dropFeed ends the newest events connection from the server side.
func (fe *fakeEngine) dropFeed(t *testing.T) {
	t.Helper()
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if len(fe.feeds) == 0 {
		t.Fatal("no events feed is open")
	}
	close(fe.feeds[len(fe.feeds)-1].drop)
}

// fakeHosts satisfies Hosts over fake engines.
type fakeHosts struct {
	mu      sync.Mutex
	clients map[string]*dockerutil.Client
	down    map[string]bool
}

func newFakeHosts() *fakeHosts {
	return &fakeHosts{clients: map[string]*dockerutil.Client{}, down: map[string]bool{}}
}

func (f *fakeHosts) add(t *testing.T, id string, fe *fakeEngine) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[id] = fe.client(t)
}

func (f *fakeHosts) setDown(id string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[id] = down
}

func (f *fakeHosts) Host(_ context.Context, id string) (dockerhost.Host, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.clients[id]; !ok {
		return dockerhost.Host{}, dockerhost.ErrHostNotFound
	}
	return dockerhost.Host{ID: id, Name: id}, nil
}

func (f *fakeHosts) Client(_ context.Context, id string) (*dockerutil.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down[id] {
		return nil, fmt.Errorf("dial tcp 127.0.0.1:1: connect: connection refused")
	}
	c, ok := f.clients[id]
	if !ok {
		return nil, dockerhost.ErrHostNotFound
	}
	return c, nil
}

// testClock is a manually advanced clock for Options.Now.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock {
	return &testClock{t: time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// testEnv is a hub over one fake host "h1" with short timings.
type testEnv struct {
	hub   *Hub
	hosts *fakeHosts
	fe    *fakeEngine
	clock *testClock
}

func newTestEnv(t *testing.T, tune func(*Options)) *testEnv {
	t.Helper()
	fe := newFakeEngine(t)
	hosts := newFakeHosts()
	hosts.add(t, "h1", fe)
	clock := newTestClock()
	opts := Options{
		Now:            clock.Now,
		StatsInterval:  15 * time.Millisecond,
		HostInterval:   40 * time.Millisecond,
		PollInterval:   15 * time.Millisecond,
		RetryPaused:    250 * time.Millisecond,
		IdleStop:       60 * time.Millisecond,
		ConnectTimeout: 2 * time.Second,
		Backoff:        []time.Duration{5 * time.Millisecond, 10 * time.Millisecond},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if tune != nil {
		tune(&opts)
	}
	hub := New(hosts, opts)
	t.Cleanup(hub.Close)
	return &testEnv{hub: hub, hosts: hosts, fe: fe, clock: clock}
}

// attach opens a client and subscribes it.
func (e *testEnv) attach(t *testing.T, sub Subscription) *Client {
	t.Helper()
	c, err := e.hub.Attach()
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	t.Cleanup(c.Close)
	if _, err := c.Set(sub); err != nil {
		t.Fatalf("Set: %v", err)
	}
	return c
}

// next waits for the next message of a client.
func next(t *testing.T, c *Client, timeout time.Duration) Message {
	t.Helper()
	select {
	case m, ok := <-c.Messages():
		if !ok {
			t.Fatal("the client's stream is closed")
		}
		return m
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a message")
		return Message{}
	}
}

// waitFor reads messages until pick returns true for one; the others are skipped.
func waitFor(t *testing.T, c *Client, what string, pick func(Message) bool) Message {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case m, ok := <-c.Messages():
			if !ok {
				t.Fatalf("the stream closed while waiting for %s", what)
			}
			if pick(m) {
				return m
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func isStatus(state string) func(Message) bool {
	return func(m Message) bool {
		s, ok := m.Data.(HostStatus)
		return m.Event == MsgHostStatus && ok && s.State == state
	}
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
