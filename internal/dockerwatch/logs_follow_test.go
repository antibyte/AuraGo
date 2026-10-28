package dockerwatch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

// logsHTTPHosts serves one Engine over HTTP that has the container "web" (a TTY
// container) and records the query of its log request.
type logsHTTPHosts struct {
	client *dockerutil.Client
	mu     sync.Mutex
	query  url.Values
	hits   int
}

func newLogsHTTPHosts(t *testing.T, logBody string) *logsHTTPHosts {
	t.Helper()
	h := &logsHTTPHosts{}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.hits++
		h.mu.Unlock()
		switch {
		case r.URL.Path == "/version":
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`)
		case strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_, _ = io.WriteString(w, `{"Config":{"Tty":true}}`)
		case strings.HasSuffix(r.URL.Path, "/containers/web/logs"):
			h.mu.Lock()
			h.query = r.URL.Query()
			h.mu.Unlock()
			_, _ = io.WriteString(w, logBody)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"no such container"}`)
		}
	}))
	t.Cleanup(server.Close)
	h.client = dockerutil.NewClient("tcp://"+server.Listener.Addr().String(), 5*time.Second)
	t.Cleanup(h.client.CloseIdleConnections)
	return h
}

func (h *logsHTTPHosts) Host(context.Context, string) (dockerhost.Host, error) {
	return dockerhost.Host{}, nil
}

func (h *logsHTTPHosts) Client(context.Context, string) (*dockerutil.Client, error) {
	return h.client, nil
}

func TestFollowContainerStreamsTheEnginesLogThroughTheHub(t *testing.T) {
	body := "2026-10-28T07:00:01.000000000Z first\r\n2026-10-28T07:00:02.000000000Z second\n2026-10-28T07:00:03.000000000Z third\n"
	hosts := newLogsHTTPHosts(t, body)
	hub := New(hosts, Options{})
	sink := &logsSink{}
	since := logsBase.Add(-time.Hour)
	opened := 0
	opt := FollowOptions{Tail: 50, Since: since, Opened: func() {
		if len(sink.lines()) != 0 {
			t.Error("Opened came after the first line")
		}
		opened++
	}}

	err := hub.FollowContainer(context.Background(), "h1", "web", opt, sink.out, sink.notice)
	if err != nil {
		t.Fatalf("FollowContainer: %v", err)
	}
	if opened != 1 {
		t.Fatalf("Opened was called %d times, want once", opened)
	}
	if got := sink.texts(); len(got) != 3 || got[0] != "first" || got[1] != "second" || got[2] != "third" {
		t.Fatalf("lines = %v, want the three lines without timestamps", got)
	}
	if first := sink.lines()[0]; first.S != "out" || !first.T.Equal(logsBase.Add(time.Second)) || first.Svc != "" || first.Ctr != "" {
		t.Fatalf("first entry = %+v, want a plain entry (no service or container on a single stream)", first)
	}
	if len(sink.batches) != 1 {
		t.Fatalf("%d batches, want the three lines in one", len(sink.batches))
	}
	hosts.mu.Lock()
	q := hosts.query
	hosts.mu.Unlock()
	if q.Get("follow") != "1" || q.Get("tail") != "50" || q.Get("timestamps") != "1" || q.Get("since") == "" {
		t.Fatalf("log query = %v, want follow=1, tail=50, timestamps=1 and since", q)
	}
	// The stream slot is free again: all of them can be taken.
	for i := 0; i < MaxStreamsPerHost; i++ {
		release, err := hub.Acquire("h1")
		if err != nil {
			t.Fatalf("slot %d after the follow ended: %v", i, err)
		}
		defer release()
	}
}

func TestFollowContainerNeedsAFreeStreamSlot(t *testing.T) {
	hosts := newLogsHTTPHosts(t, "")
	hub := New(hosts, Options{})
	for i := 0; i < MaxStreamsPerHost; i++ {
		release, err := hub.Acquire("h1")
		if err != nil {
			t.Fatal(err)
		}
		defer release()
	}
	opt := FollowOptions{Tail: 10, Opened: func() { t.Error("Opened was called without a stream") }}
	err := hub.FollowContainer(context.Background(), "h1", "web", opt, (&logsSink{}).out, nil)
	if !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("FollowContainer without a free slot = %v, want ErrTooManyStreams", err)
	}
	hosts.mu.Lock()
	hits := hosts.hits
	hosts.mu.Unlock()
	if hits != 0 {
		t.Fatalf("the Engine got %d requests although no slot was free", hits)
	}
}

func TestFollowContainerReportsEngineAnswersUnchanged(t *testing.T) {
	hosts := newLogsHTTPHosts(t, "")
	hub := New(hosts, Options{})
	opt := FollowOptions{Opened: func() { t.Error("Opened was called although the Engine refused the request") }}
	err := hub.FollowContainer(context.Background(), "h1", "gone", opt, (&logsSink{}).out, nil)
	if dockerhost.StatusOf(err) != http.StatusNotFound {
		t.Fatalf("unknown container: %v (status %d), want the Engine's 404 as a *StatusError", err, dockerhost.StatusOf(err))
	}
}

func TestFollowContainerMapsOptionsAndStopsWithTheReceiverOrTheContext(t *testing.T) {
	fe := newLogsFakeEngine()
	fe.history["web"] = []dockerhost.LogLine{logsLine(1, "out", "one"), logsLine(2, "err", "two"), logsLine(3, "out", "three")}
	env := fe.env()

	// Tail 0 and no Since: no history, only new lines. A Since without a Tail asks for every line since then.
	for _, tc := range []struct {
		opt  FollowOptions
		want logsRead
	}{
		{FollowOptions{}, logsRead{Tail: 0, Follow: true}},
		{FollowOptions{Tail: 99999}, logsRead{Tail: MaxLogTail, Follow: true}},
		{FollowOptions{Since: logsBase}, logsRead{Tail: MaxLogTail, Since: logsBase, Follow: true}},
		{FollowOptions{Tail: 2, Since: logsBase}, logsRead{Tail: 2, Since: logsBase, Follow: true}},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- env.followContainer(ctx, "h1", "web", tc.opt, (&logsSink{}).out, nil) }()
		logsWaitFor(t, "the read", func() bool { return len(fe.liveCalls("web")) > 0 })
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("FollowContainer after cancel = %v, want context.Canceled", err)
		}
		got := fe.liveCalls("web")
		if got[len(got)-1].read != tc.want {
			t.Fatalf("options %+v became %+v, want %+v", tc.opt, got[len(got)-1].read, tc.want)
		}
		fe.mu.Lock()
		fe.calls = nil
		fe.mu.Unlock()
	}
	if fe.slotsInUse() != 0 {
		t.Fatalf("%d stream slots still held after the follows ended", fe.slotsInUse())
	}

	// A receiver that fails ends the follow with its error.
	boom := errors.New("client gone")
	sink := &logsSink{outErr: boom}
	err := env.followContainer(context.Background(), "h1", "web", FollowOptions{Tail: 10}, sink.out, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("FollowContainer with a failing receiver = %v, want its error", err)
	}
	if fe.slotsInUse() != 0 {
		t.Fatalf("slots in use = %d, want 0", fe.slotsInUse())
	}
}
